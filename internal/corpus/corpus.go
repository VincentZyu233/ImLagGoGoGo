package corpus

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/VincentZyuApps/ImLagGoGoGo/assets"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type SelectionMode string

const (
	ModeShuffle SelectionMode = "shuffle"
	ModeRandom  SelectionMode = "random"
)

type CorpusManager struct {
	filePath      string
	selectionMode SelectionMode
	encoding      string

	mu           sync.RWMutex
	messages     []string
	deck         []string // 洗牌池中的剩余借口
	currentIndex int

	stopWatcher chan struct{}
}

func NewCorpusManager(filePath string, mode SelectionMode, encoding string) (*CorpusManager, error) {
	if filePath == "" {
		filePath = "Messages.txt"
	}
	if mode == "" {
		mode = ModeShuffle
	}
	if encoding == "" {
		encoding = "auto"
	}

	cm := &CorpusManager{
		filePath:      filePath,
		selectionMode: mode,
		encoding:      encoding,
		stopWatcher:   make(chan struct{}),
	}

	if err := cm.ensureFileExists(); err != nil {
		return nil, fmt.Errorf("ensure corpus file: %w", err)
	}

	if err := cm.Reload(); err != nil {
		return nil, fmt.Errorf("initial load corpus: %w", err)
	}

	return cm, nil
}

func (cm *CorpusManager) ensureFileExists() error {
	if _, err := os.Stat(cm.filePath); err == nil {
		return nil
	}

	// 优先从内嵌 assets 释放默认词库
	if len(assets.DefaultMessagesTXT) > 0 {
		return os.WriteFile(cm.filePath, assets.DefaultMessagesTXT, 0644)
	}

	// 兜底生成默认神句
	defaultMessages := []string{
		"我卡了，刚掉帧直接掉到1帧",
		"鼠标突然断联飞天上了",
		"耳机没电了，完全听不见脚步",
		"刚才屏幕突然黑了一下，显示器掉驱动了",
		"不是哥们，你这提前枪开了透视吧？",
		"我家猫刚才跳键盘上踩了空格",
		"键盘拔键器卡住了，大狙开不出镜",
		"我网线被室友拔了，高ping战士附体",
		"鼠标垫太滑了，甩枪甩到太平洋去了",
		"刚才输入法切出中文候选框了",
	}

	content := strings.Join(defaultMessages, "\n") + "\n"
	return os.WriteFile(cm.filePath, []byte(content), 0644)
}

// Reload 重新加载词库，支持编码检测与去除空行
func (cm *CorpusManager) Reload() error {
	raw, err := os.ReadFile(cm.filePath)
	if err != nil {
		return fmt.Errorf("read corpus file: %w", err)
	}

	// 智能转码
	utf8Text, err := cm.decodeToUTF8(raw)
	if err != nil {
		return fmt.Errorf("decode text: %w", err)
	}

	lines := strings.Split(utf8Text, "\n")
	var validMessages []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 忽略空行与以 # 或 // 开头的注释行
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		validMessages = append(validMessages, trimmed)
	}

	if len(validMessages) == 0 {
		validMessages = append(validMessages, "我卡了 (词库为空，请在 Messages.txt 中添加语句)")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.messages = validMessages
	cm.reshuffleLocked()

	return nil
}

// Next 获取下一条借口语句
func (cm *CorpusManager) Next() (string, int, int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	total := len(cm.messages)
	if total == 0 {
		return "我卡了", 0, 0
	}

	if cm.selectionMode == ModeRandom {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		msg := cm.messages[r.Intn(total)]
		return msg, 1, total
	}

	// 洗牌池模式 (Shuffle Deck)
	if cm.currentIndex >= len(cm.deck) {
		cm.reshuffleLocked()
	}

	msg := cm.deck[cm.currentIndex]
	cm.currentIndex++
	remaining := len(cm.deck) - cm.currentIndex

	return msg, remaining, total
}

func (cm *CorpusManager) reshuffleLocked() {
	deck := make([]string, len(cm.messages))
	copy(deck, cm.messages)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	cm.deck = deck
	cm.currentIndex = 0
}

func (cm *CorpusManager) decodeToUTF8(data []byte) (string, error) {
	// 去除 UTF-8 BOM
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		data = data[3:]
	}

	switch strings.ToLower(cm.encoding) {
	case "utf-8", "utf8":
		return string(data), nil
	case "gbk", "gb2312", "gb18030":
		reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(reader)
		return buf.String(), err
	case "auto":
		if utf8.Valid(data) {
			return string(data), nil
		}
		// 自动尝试 GBK 转码
		reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GB18030.NewDecoder())
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(reader); err == nil {
			return buf.String(), nil
		}
		// 最终兜底使用带有替换符的 UTF-8
		return strings.ToValidUTF8(string(data), ""), nil
	default:
		return string(data), nil
	}
}

// GetStats 返回词库总数与当前洗牌池剩余数量
func (cm *CorpusManager) GetStats() (total int, remaining int) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return len(cm.messages), len(cm.deck) - cm.currentIndex
}

// GetAllMessages 返回当前加载的所有词句
func (cm *CorpusManager) GetAllMessages() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	res := make([]string, len(cm.messages))
	copy(res, cm.messages)
	return res
}
