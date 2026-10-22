package state

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/VincentZyuApps/ImLagGoGoGo/internal/config"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/corpus"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/driver"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/gsi"
)

type DeathEvent struct {
	Timestamp   time.Time
	PlayerName  string
	SteamID     string
	MapName     string
	Round       int
	Message     string
	Channel     string
	TriggeredAt time.Time
}

type Engine struct {
	cfg    *config.Config
	corpus *corpus.CorpusManager
	driver driver.Driver

	mu           sync.RWMutex
	paused       bool
	lastHealth   int
	lastRound    int
	lastDeathAt  time.Time
	roundTrigger bool // 当前回合是否已经狡辩过，防止多次触发

	deathHistory []DeathEvent
	onDeathEvent func(event DeathEvent)
}

func NewEngine(cfg *config.Config, cm *corpus.CorpusManager, drv driver.Driver) *Engine {
	return &Engine{
		cfg:          cfg,
		corpus:       cm,
		driver:       drv,
		lastHealth:   100,
		deathHistory: make([]DeathEvent, 0),
	}
}

func (e *Engine) OnDeath(callback func(event DeathEvent)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onDeathEvent = callback
}

// TogglePause 切换静音/暂停状态
func (e *Engine) TogglePause() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.paused = !e.paused
	return e.paused
}

func (e *Engine) IsPaused() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.paused
}

// ProcessPayload 处理 GSI 推送的数据快照
func (e *Engine) ProcessPayload(payload *gsi.GSIPayload) {
	if payload == nil || payload.Player == nil || payload.Player.State == nil {
		return
	}

	// 1. 检查是否开启了静音 / 模式是否允许自动触发
	e.mu.Lock()
	if e.paused {
		e.mu.Unlock()
		return
	}
	behaviorMode := strings.ToLower(e.cfg.Mode.BehaviorMode)
	if behaviorMode == "cfg" {
		// 纯手动 CFG 模式下，GSI 自动触发不生效
		e.mu.Unlock()
		return
	}
	e.mu.Unlock()

	// 2. 验证是否为目标玩家本人 (SteamID 优先，昵称备选)
	if !e.isTargetPlayer(payload.Player) {
		return
	}

	currHealth := payload.Player.State.Health
	currRound := 0
	currMap := "unknown"
	if payload.Map != nil {
		currRound = payload.Map.Round
		currMap = payload.Map.Name
	}

	e.mu.Lock()
	// 新回合重置触发锁
	if currRound != e.lastRound {
		e.lastRound = currRound
		e.roundTrigger = false
	}

	prevHealth := e.lastHealth
	e.lastHealth = currHealth

	// 核心判定: 上次存活 (HP > 0) 且 本次死亡 (HP == 0)，并且当前回合尚未触发过，且冷却间隔大于 3 秒
	isDying := (prevHealth > 0 && currHealth == 0)
	if !isDying || e.roundTrigger || time.Since(e.lastDeathAt) < 3*time.Second {
		e.mu.Unlock()
		return
	}

	// 锁定当前回合与时间戳
	e.roundTrigger = true
	e.lastDeathAt = time.Now()
	playerName := payload.Player.Name
	steamID := payload.Player.SteamID
	channel := e.cfg.Chat.Channel
	delayMs := e.cfg.Chat.DeathDelayMs
	checkForeground := e.cfg.Chat.CheckForeground
	e.mu.Unlock()

	// 3. 异步执行延迟缓冲并注入借口
	go func() {
		// 死亡延迟缓冲 (默认 555ms)
		if delayMs > 0 {
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}

		// 校验前台窗口 (如果开启了窗口检测)
		if checkForeground && e.driver != nil && !e.driver.IsCS2Foreground() {
			return
		}

		// 从洗牌池提取一条借口
		msg, _, _ := e.corpus.Next()

		// 决定聊天按键
		chatKey := "y"
		if strings.ToLower(channel) == "team" {
			chatKey = "u"
		}

		// 执行键盘模拟注入
		if e.driver != nil {
			_ = e.driver.SendChatMessage(chatKey, msg, e.cfg.Chat.KeyIntervalMs)
		}

		// 记录事件与上报回调
		event := DeathEvent{
			Timestamp:   time.Now(),
			PlayerName:  playerName,
			SteamID:     steamID,
			MapName:     currMap,
			Round:       currRound,
			Message:     msg,
			Channel:     channel,
			TriggeredAt: time.Now(),
		}

		e.mu.Lock()
		e.deathHistory = append(e.deathHistory, event)
		if len(e.deathHistory) > 50 {
			e.deathHistory = e.deathHistory[1:]
		}
		cb := e.onDeathEvent
		e.mu.Unlock()

		if cb != nil {
			cb(event)
		}
	}()
}

// ManualTrigger 手动一键触发狡辩 (供测试按钮或绑定快捷键调用)
func (e *Engine) ManualTrigger() (string, error) {
	msg, _, _ := e.corpus.Next()
	chatKey := "y"
	if strings.ToLower(e.cfg.Chat.Channel) == "team" {
		chatKey = "u"
	}

	if e.driver != nil {
		if err := e.driver.SendChatMessage(chatKey, msg, e.cfg.Chat.KeyIntervalMs); err != nil {
			return msg, fmt.Errorf("send chat message: %w", err)
		}
	}

	return msg, nil
}

func (e *Engine) isTargetPlayer(player *gsi.PlayerSection) bool {
	if player == nil {
		return false
	}

	targetSteamID := strings.TrimSpace(e.cfg.Player.SteamID)
	// 若配置了 SteamID，强制优先进行严格匹配
	if targetSteamID != "" {
		return strings.EqualFold(player.SteamID, targetSteamID)
	}

	// 否则通过配置的昵称白名单逐一比对
	if len(e.cfg.Player.PlayerNames) == 0 {
		// 若任何配置都没填，默认认为是本人 (开箱即用)
		return true
	}

	for _, name := range e.cfg.Player.PlayerNames {
		if strings.TrimSpace(name) == "" {
			continue
		}
		if strings.EqualFold(player.Name, name) {
			return true
		}
	}

	return false
}

// GetRecentDeaths 返回最近的死亡记录
func (e *Engine) GetRecentDeaths() []DeathEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()

	res := make([]DeathEvent, len(e.deathHistory))
	copy(res, e.deathHistory)
	return res
}
