package windows

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

type CS2Locator struct {
	customPath string
}

func NewCS2Locator(customPath string) *CS2Locator {
	return &CS2Locator{customPath: customPath}
}

// LocateCS2Path 自动寻找 CS2 游戏安装目录
func (l *CS2Locator) LocateCS2Path() (string, error) {
	// 1. 如果用户手动配置了路径且有效，优先使用
	if l.customPath != "" {
		if l.isValidCS2Path(l.customPath) {
			return l.customPath, nil
		}
	}

	// 2. 从注册表读取 Steam 安装路径
	steamPath, err := l.getSteamPathFromRegistry()
	if err != nil {
		return "", fmt.Errorf("detect steam path: %w", err)
	}

	// 3. 从 libraryfolders.vdf 扫描所有磁盘库目录
	libraryFolders := l.parseSteamLibraries(steamPath)

	possibleRelPaths := []string{
		filepath.Join("steamapps", "common", "Counter-Strike Global Offensive"),
		filepath.Join("steamapps", "common", "Counter-Strike 2"),
	}

	for _, lib := range libraryFolders {
		for _, rel := range possibleRelPaths {
			candidate := filepath.Join(lib, rel)
			if l.isValidCS2Path(candidate) {
				return candidate, nil
			}
		}
	}

	return "", fmt.Errorf("CS2 installation directory not found in Steam libraries")
}

func (l *CS2Locator) isValidCS2Path(p string) bool {
	if p == "" {
		return false
	}
	vpkPath := filepath.Join(p, "game", "csgo", "pak01_dir.vpk")
	_, err := os.Stat(vpkPath)
	return err == nil
}

func (l *CS2Locator) getSteamPathFromRegistry() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()

	val, _, err := k.GetStringValue("SteamPath")
	if err != nil {
		return "", err
	}
	return filepath.Clean(val), nil
}

func (l *CS2Locator) parseSteamLibraries(steamPath string) []string {
	libs := []string{steamPath}
	vdfPath := filepath.Join(steamPath, "steamapps", "libraryfolders.vdf")

	f, err := os.Open(vdfPath)
	if err != nil {
		return libs
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, `"path"`) {
			parts := strings.Split(line, `"`)
			if len(parts) >= 4 {
				path := parts[3]
				path = strings.ReplaceAll(path, `\\`, `\`)
				if _, err := os.Stat(path); err == nil {
					libs = append(libs, filepath.Clean(path))
				}
			}
		}
	}

	return libs
}

// ChooseCS2PathViaDialog 弹出 Windows 系统文件夹选择器兜底手动指定
func (l *CS2Locator) ChooseCS2PathViaDialog() (string, error) {
	script := `
Add-Type -AssemblyName System.Windows.Forms
$dialog = New-Object System.Windows.Forms.FolderBrowserDialog
$dialog.Description = "请选择 Counter-Strike 2 (CS2) 游戏根目录"
$dialog.ShowNewFolderButton = $false
if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
    Write-Output $dialog.SelectedPath
}
`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("folder dialog execution error: %w", err)
	}

	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return "", fmt.Errorf("user canceled folder selection")
	}

	if !l.isValidCS2Path(selected) {
		return selected, fmt.Errorf("选中的目录不是有效的 CS2 游戏目录 (未找到 game/csgo/pak01_dir.vpk)")
	}

	return selected, nil
}
