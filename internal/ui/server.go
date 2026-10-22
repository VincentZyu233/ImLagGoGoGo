package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/VincentZyuApps/ImLagGoGoGo/internal/config"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/corpus"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/driver"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/gsi"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/state"
)

//go:embed assets/*
var assetsFS embed.FS

type UIServer struct {
	cfg     *config.Config
	corpus  *corpus.CorpusManager
	gsiSrv  *gsi.Server
	engine  *state.Engine
	driver  driver.Driver

	listener net.Listener
	port     int
	mu       sync.RWMutex

	lastExcuse     string
	lastExcuseTime string
	deathCount     int
}

func NewUIServer(
	cfg *config.Config,
	cm *corpus.CorpusManager,
	gs *gsi.Server,
	eng *state.Engine,
	drv driver.Driver,
) *UIServer {
	s := &UIServer{
		cfg:    cfg,
		corpus: cm,
		gsiSrv: gs,
		engine: eng,
		driver: drv,
	}

	eng.OnDeath(func(event state.DeathEvent) {
		s.mu.Lock()
		s.lastExcuse = event.Message
		s.lastExcuseTime = event.Timestamp.Format("15:04:05")
		s.deathCount++
		s.mu.Unlock()
	})

	return s
}

func (s *UIServer) Start() (int, error) {
	// 在 127.0.0.1 上寻找一个随机可用端口作为管理界面
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listen ui server: %w", err)
	}

	s.listener = l
	s.port = l.Addr().(*net.TCPAddr).Port

	sub, _ := fs.Sub(assetsFS, "assets")
	mux := http.NewServeMux()

	// 静态文件服务 (内嵌 index.html)
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// API 路由
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/corpus", s.handleCorpus)
	mux.HandleFunc("/api/test-say", s.handleTestSay)
	mux.HandleFunc("/api/install-gsi", s.handleInstallGSI)
	mux.HandleFunc("/api/choose-cs2-path", s.handleChooseCS2Path)
	mux.HandleFunc("/api/open-messages", s.handleOpenMessages)
	mux.HandleFunc("/api/reload-corpus", s.handleReloadCorpus)
	mux.HandleFunc("/api/open-url", s.handleOpenURL)

	go func() {
		_ = http.Serve(l, mux)
	}()

	return s.port, nil
}

func (s *UIServer) Port() int {
	return s.port
}

func (s *UIServer) handleState(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	last := s.gsiSrv.LastPayload()
	player := "未在对局"
	health := 0
	mapName := "无活动地图"

	if last != nil && last.Player != nil {
		player = last.Player.Name
		if last.Player.State != nil {
			health = last.Player.State.Health
		}
	}
	if last != nil && last.Map != nil {
		mapName = last.Map.Name
	}

	cs2Path, _ := s.driver.LocateCS2Path()

	res := map[string]any{
		"connected":      s.gsiSrv.IsConnected(),
		"port":           s.gsiSrv.ActualPort(),
		"player":         player,
		"health":         health,
		"map":            mapName,
		"deathCount":     s.deathCount,
		"cs2Path":        cs2Path,
		"lastExcuse":     s.lastExcuse,
		"lastExcuseTime": s.lastExcuseTime,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *UIServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.cfg)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			SteamID       string `json:"steamid"`
			BehaviorMode  string `json:"behavior_mode"`
			SelectionMode string `json:"selection_mode"`
			Channel       string `json:"channel"`
			DeathDelayMs  int    `json:"death_delay_ms"`
			CloseAction   string `json:"close_action"`
			AutoStart     bool   `json:"auto_start"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			s.cfg.Player.SteamID = req.SteamID
			s.cfg.Mode.BehaviorMode = req.BehaviorMode
			s.cfg.Corpus.SelectionMode = req.SelectionMode
			s.cfg.Chat.Channel = req.Channel
			if req.DeathDelayMs > 0 {
				s.cfg.Chat.DeathDelayMs = req.DeathDelayMs
			}
			s.cfg.UI.CloseAction = req.CloseAction
			s.cfg.UI.AutoStart = req.AutoStart

			_ = s.cfg.Save()
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (s *UIServer) handleCorpus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.corpus.GetAllMessages())
}

func (s *UIServer) handleTestSay(w http.ResponseWriter, r *http.Request) {
	go func() {
		time.Sleep(3 * time.Second)
		_, _ = s.engine.ManualTrigger()
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func (s *UIServer) handleInstallGSI(w http.ResponseWriter, r *http.Request) {
	err := s.driver.InstallGSIConfig(s.gsiSrv.ActualPort())
	res := map[string]any{"success": err == nil}
	if err != nil {
		res["error"] = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *UIServer) handleChooseCS2Path(w http.ResponseWriter, r *http.Request) {
	path, err := s.driver.ChooseCS2PathViaDialog()
	res := map[string]any{"success": err == nil, "path": path}
	if err != nil {
		res["error"] = err.Error()
	} else {
		s.cfg.UI.CS2Path = path
		_ = s.cfg.Save()
		_ = s.driver.InstallGSIConfig(s.gsiSrv.ActualPort())
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *UIServer) handleOpenMessages(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS == "windows" {
		_ = exec.Command("notepad.exe", s.cfg.Corpus.FilePath).Start()
	}
	w.WriteHeader(http.StatusOK)
}

func (s *UIServer) handleReloadCorpus(w http.ResponseWriter, r *http.Request) {
	_ = s.corpus.Reload()
	w.WriteHeader(http.StatusOK)
}

func (s *UIServer) handleOpenURL(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url != "" && runtime.GOOS == "windows" {
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
	w.WriteHeader(http.StatusOK)
}
