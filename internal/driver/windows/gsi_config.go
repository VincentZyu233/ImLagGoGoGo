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
	return &GsiConfigWriter{
		locator: locator,
	}
}

// InstallGSIConfig 自动向 CS2 的 cfg 目录写入 gamestate_integration_imlag.cfg
func (w *GsiConfigWriter) InstallGSIConfig(listenPort int) error {
	cs2Path, err := w.locator.LocateCS2Path()
	if err != nil {
		return fmt.Errorf("locate cs2 path: %w", err)
	}

	cfgDir := filepath.Join(cs2Path, "game", "csgo", "cfg")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return fmt.Errorf("create cfg directory: %w", err)
	}

	cfgPath := filepath.Join(cfgDir, "gamestate_integration_imlag.cfg")

	content := fmt.Sprintf(`"ImLag GSI Integration"
{
    "uri"           "http://127.0.0.1:%d/"
    "timeout"       "5.0"
    "buffer"        "0.1"
    "throttle"      "0.2"
    "heartbeat"     "30.0"
    "data"
    {
        "provider"              "1"
        "map"                   "1"
        "round"                 "1"
        "player_id"             "1"
        "player_state"          "1"
        "player_weapons"        "1"
        "player_match_stats"    "1"
    }
}
`, listenPort)

	return os.WriteFile(cfgPath, []byte(content), 0644)
}

// WriteSelectorCfg 生成方案 A 所需的 CS2 原生 say 脚本轮换体系 (支持全服与队内双通道轮换)
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

	total := min(len(messages), 150)

	// 1. 生成全局 say 文件 (imlag_say_global_X.cfg)
	for i := 0; i < total; i++ {
		cfgName := fmt.Sprintf("imlag_say_global_%d.cfg", i+1)
		fullPath := filepath.Join(cfgDir, cfgName)
		safeMsg := strings.ReplaceAll(messages[i], `"`, `'`)
		data := fmt.Sprintf("// ImLag Global Say %d\nsay \"%s\"\n", i+1, safeMsg)
		_ = os.WriteFile(fullPath, []byte(data), 0644)
	}

	// 2. 生成队内 say 文件 (imlag_say_team_X.cfg)
	for i := 0; i < total; i++ {
		cfgName := fmt.Sprintf("imlag_say_team_%d.cfg", i+1)
		fullPath := filepath.Join(cfgDir, cfgName)
		safeMsg := strings.ReplaceAll(messages[i], `"`, `'`)
		data := fmt.Sprintf("// ImLag Team Say %d\nsay_team \"%s\"\n", i+1, safeMsg)
		_ = os.WriteFile(fullPath, []byte(data), 0644)
	}

	// 3. 生成全局选择器循环链 (imlag_say_global_selector.cfg)
	var globalLines []string
	globalLines = append(globalLines, "// ImLag Global Selector")
	for i := 1; i <= total; i++ {
		next := i%total + 1
		globalLines = append(globalLines, fmt.Sprintf("alias imlag_global_say_%d \"exec imlag_say_global_%d; alias imlag_do_global_say imlag_global_say_%d\"", i, i, next))
	}
	globalLines = append(globalLines, "alias imlag_do_global_say imlag_global_say_1")
	_ = os.WriteFile(filepath.Join(cfgDir, "imlag_say_global_selector.cfg"), []byte(strings.Join(globalLines, "\n")+"\n"), 0644)

	// 4. 生成队内选择器循环链 (imlag_say_team_selector.cfg)
	var teamLines []string
	teamLines = append(teamLines, "// ImLag Team Selector")
	for i := 1; i <= total; i++ {
		next := i%total + 1
		teamLines = append(teamLines, fmt.Sprintf("alias imlag_team_say_%d \"exec imlag_say_team_%d; alias imlag_do_team_say imlag_team_say_%d\"", i, i, next))
	}
	teamLines = append(teamLines, "alias imlag_do_team_say imlag_team_say_1")
	_ = os.WriteFile(filepath.Join(cfgDir, "imlag_say_team_selector.cfg"), []byte(strings.Join(teamLines, "\n")+"\n"), 0644)

	// 5. 挂载到 autoexec.cfg
	globalKey := "k"
	teamKey := "l"
	if len(bindKeys) > 0 && strings.TrimSpace(bindKeys[0]) != "" {
		globalKey = strings.TrimSpace(bindKeys[0])
	}
	if len(bindKeys) > 1 && strings.TrimSpace(bindKeys[1]) != "" {
		teamKey = strings.TrimSpace(bindKeys[1])
	}

	w.injectAutoexec(cfgDir, globalKey, teamKey)

	return nil
}

func (w *GsiConfigWriter) injectAutoexec(cfgDir string, globalKey, teamKey string) {
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
		if !inBlock && !strings.Contains(line, "imlag_say_") && !strings.Contains(line, "imlag_do_") {
			filtered = append(filtered, line)
		}
	}

	filtered = append(filtered, "")
	filtered = append(filtered, tagStart)
	filtered = append(filtered, "// Managed automatically by ImLagGoGoGo")
	filtered = append(filtered, "exec imlag_say_global_selector")
	filtered = append(filtered, "exec imlag_say_team_selector")
	if globalKey != "" {
		filtered = append(filtered, fmt.Sprintf("bind \"%s\" \"imlag_do_global_say\"", strings.ToLower(globalKey)))
	}
	if teamKey != "" {
		filtered = append(filtered, fmt.Sprintf("bind \"%s\" \"imlag_do_team_say\"", strings.ToLower(teamKey)))
	}
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
