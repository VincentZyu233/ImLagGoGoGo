package windows

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type GsiConfigWriter struct {
	locator *CS2Locator
}

func NewGsiConfigWriter(locator *CS2Locator) *GsiConfigWriter {
	return &GsiConfigWriter{locator: locator}
}

// InstallGSIConfig 向 CS2 安装/更新 gamestate_integration_imlag.cfg
func (w *GsiConfigWriter) InstallGSIConfig(listenPort int) error {
	cs2Path, err := w.locator.LocateCS2Path()
	if err != nil {
		return fmt.Errorf("locate cs2: %w", err)
	}

	cfgDir := filepath.Join(cs2Path, "game", "csgo", "cfg")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return fmt.Errorf("create cfg directory: %w", err)
	}

	cfgPath := filepath.Join(cfgDir, "gamestate_integration_imlag.cfg")

	content := fmt.Sprintf(`"ImLag GSI Integration v3.0"
{
    "uri" "http://127.0.0.1:%d"
    "timeout" "5.0"
    "buffer"  "0.1"
    "throttle" "0.1"
    "heartbeat" "10.0"
    "data"
    {
        "provider"            "1"
        "map"                 "1"
        "round"               "1"
        "player_id"           "1"
        "player_state"        "1"
        "previously"          "1"
    }
}
`, listenPort)

	return os.WriteFile(cfgPath, []byte(content), 0644)
}

// WriteSelectorCfg 生成游戏内手动轮换 say 脚本，并自动绑定到 autoexec.cfg
func (w *GsiConfigWriter) WriteSelectorCfg(messages []string, bindKeys []string) error {
	if len(messages) == 0 {
		return nil
	}

	cs2Path, err := w.locator.LocateCS2Path()
	if err != nil {
		return fmt.Errorf("locate cs2: %w", err)
	}

	cfgDir := filepath.Join(cs2Path, "game", "csgo", "cfg")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return fmt.Errorf("create cfg directory: %w", err)
	}

	maxCfg := min(len(messages), 100)
	for i := 0; i < maxCfg; i++ {
		nextIndex := (i + 1) % maxCfg + 1
		cfgName := fmt.Sprintf("imlag_say_%d.cfg", i+1)
		fullPath := filepath.Join(cfgDir, cfgName)

		safeMsg := strings.ReplaceAll(messages[i], `"`, `'`)
		data := fmt.Sprintf("say \"%s\"\nalias imlag_random_say \"exec imlag_say_%d.cfg\"\n", safeMsg, nextIndex)
		_ = os.WriteFile(fullPath, []byte(data), 0644)
	}

	// 生成选择器入口
	selectorPath := filepath.Join(cfgDir, "imlag_say_selector.cfg")
	selectorContent := "imlag_random_say\n"
	_ = os.WriteFile(selectorPath, []byte(selectorContent), 0644)

	// 挂载到 autoexec.cfg
	if len(bindKeys) > 0 {
		w.injectAutoexec(cfgDir, bindKeys)
	}

	return nil
}

func (w *GsiConfigWriter) injectAutoexec(cfgDir string, bindKeys []string) {
	autoexecPath := filepath.Join(cfgDir, "autoexec.cfg")
	var lines []string
	if data, err := os.ReadFile(autoexecPath); err == nil {
		lines = strings.Split(string(data), "\n")
	}

	var filtered []string
	inBlock := false
	const tagStart = "// --- ImLag Auto-Bind Start ---"
	const tagEnd = "// --- ImLag Auto-Bind End ---"

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == tagStart {
			inBlock = true
			continue
		}
		if trimmed == tagEnd {
			inBlock = false
			continue
		}
		if !inBlock && !strings.Contains(line, "exec imlag_say_selector") {
			filtered = append(filtered, line)
		}
	}

	filtered = append(filtered, "")
	filtered = append(filtered, tagStart)
	filtered = append(filtered, "// Managed automatically by ImLagGoGoGo")
	for _, k := range bindKeys {
		k = strings.TrimSpace(k)
		if k != "" {
			filtered = append(filtered, fmt.Sprintf("bind \"%s\" \"exec imlag_say_selector\"", k))
		}
	}
	filtered = append(filtered, "alias imlag_random_say \"exec imlag_say_1.cfg\"")
	filtered = append(filtered, tagEnd)
	filtered = append(filtered, "host_writeconfig")
	filtered = append(filtered, "")

	_ = os.WriteFile(autoexecPath, []byte(strings.Join(filtered, "\n")), 0644)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
