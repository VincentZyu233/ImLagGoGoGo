package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/VincentZyuApps/ImLagGoGoGo/assets"
	"github.com/pelletier/go-toml/v2"
)

const Version = "0.1.0"

type Config struct {
	Player  PlayerConfig  `toml:"player"`
	Mode    ModeConfig    `toml:"mode"`
	Chat    ChatConfig    `toml:"chat"`
	Network NetworkConfig `toml:"network"`
	Corpus  CorpusConfig  `toml:"corpus"`
	UI      UIConfig      `toml:"ui"`

	// 运行时动态配置，不持久化
	ConfigPath string `toml:"-"`
}

type PlayerConfig struct {
	SteamID     string   `toml:"steamid"`
	PlayerNames []string `toml:"player_names"`
}

type ModeConfig struct {
	BehaviorMode string `toml:"behavior_mode"` // "both", "auto", "cfg"
}

type ChatConfig struct {
	Channel         string `toml:"channel"`          // "all", "team"
	DeathDelayMs    int    `toml:"death_delay_ms"`   // 默认 555ms
	KeyIntervalMs   int    `toml:"key_interval_ms"`  // 默认 60ms
	CheckForeground bool   `toml:"check_foreground"` // 默认 true
	ToggleHotkey    string `toml:"toggle_hotkey"`    // 默认 "Ctrl+Shift+S"
}

type NetworkConfig struct {
	GSIPort int `toml:"gsi_port"` // 默认 4000
	MaxPort int `toml:"max_port"` // 默认 4999
}

type CorpusConfig struct {
	FilePath      string `toml:"file_path"`      // "Messages.txt"
	SelectionMode string `toml:"selection_mode"` // "shuffle", "random"
	WatchFile     bool   `toml:"watch_file"`     // 默认 true
	Encoding      string `toml:"encoding"`       // "auto", "utf-8", "gbk"
}

type UIConfig struct {
	Theme       string `toml:"theme"`        // "cs2-panorama"
	CloseAction string `toml:"close_action"` // "ask", "minimize", "exit"
	AutoStart   bool   `toml:"auto_start"`   // 默认 false
	CS2Path     string `toml:"cs2_path"`     // 自定义 CS2 根路径 (可选)
}

func DefaultConfig() Config {
	return Config{
		Player: PlayerConfig{
			SteamID:     "",
			PlayerNames: []string{},
		},
		Mode: ModeConfig{
			BehaviorMode: "both",
		},
		Chat: ChatConfig{
			Channel:         "all",
			DeathDelayMs:    555,
			KeyIntervalMs:   60,
			CheckForeground: true,
			ToggleHotkey:    "Ctrl+Shift+S",
		},
		Network: NetworkConfig{
			GSIPort: 4000,
			MaxPort: 4999,
		},
		Corpus: CorpusConfig{
			FilePath:      "Messages.txt",
			SelectionMode: "shuffle",
			WatchFile:     true,
			Encoding:      "auto",
		},
		UI: UIConfig{
			Theme:       "system",
			CloseAction: "ask",
			AutoStart:   false,
			CS2Path:     "",
		},
	}
}

// LoadOrCreate 加载配置，若不存在则尝试从 config.example.toml 复制或生成默认配置
func LoadOrCreate(path string) (*Config, error) {
	if path == "" {
		exeDir, err := os.Executable()
		if err != nil {
			path = "config.toml"
		} else {
			path = filepath.Join(filepath.Dir(exeDir), "config.toml")
		}
	}

	cfg := DefaultConfig()
	cfg.ConfigPath = path

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 优先使用内嵌的 assets 模板
		initialBytes := assets.DefaultConfigTOML
		if len(initialBytes) == 0 {
			marshaled, err := toml.Marshal(cfg)
			if err != nil {
				return nil, fmt.Errorf("marshal default config: %w", err)
			}
			initialBytes = marshaled
		}

		if err := os.WriteFile(path, initialBytes, 0644); err != nil {
			return nil, fmt.Errorf("write initial config to %s: %w", path, err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal toml: %w", err)
	}

	// 字段归一化与兜底
	if cfg.Network.GSIPort <= 0 {
		cfg.Network.GSIPort = 4000
	}
	if cfg.Network.MaxPort < cfg.Network.GSIPort {
		cfg.Network.MaxPort = 4999
	}
	if cfg.Chat.DeathDelayMs <= 0 {
		cfg.Chat.DeathDelayMs = 555
	}
	if cfg.Chat.KeyIntervalMs <= 0 {
		cfg.Chat.KeyIntervalMs = 60
	}
	if cfg.Corpus.FilePath == "" {
		cfg.Corpus.FilePath = "Messages.txt"
	}
	if !filepath.IsAbs(cfg.Corpus.FilePath) {
		cfg.Corpus.FilePath = filepath.Join(filepath.Dir(path), cfg.Corpus.FilePath)
	}
	if cfg.Corpus.SelectionMode == "" {
		cfg.Corpus.SelectionMode = "shuffle"
	}

	return &cfg, nil
}

// Save 保存当前配置到文件
func (c *Config) Save() error {
	if c.ConfigPath == "" {
		return fmt.Errorf("config path is empty")
	}

	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(c.ConfigPath, data, 0644)
}
