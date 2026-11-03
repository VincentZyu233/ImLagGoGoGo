//go:build linux

package ui

import (
	"os/exec"
	"sync"
)

var (
	linuxWindowCmdMu sync.Mutex
	linuxWindowCmd   *exec.Cmd
)

func EnsureConsole(_ bool) {}
func IsLaunchedFromTerminal() bool { return true }
func HideConsoleWindow() {}

// OpenDefaultBrowser 在 Linux 上使用默认浏览器打开
func OpenDefaultBrowser(url string) error {
	return exec.Command("xdg-open", url).Start()
}

// CloseNativeWindow 关闭在 Linux 上拉起的窗口
func CloseNativeWindow() {
	linuxWindowCmdMu.Lock()
	defer linuxWindowCmdMu.Unlock()
	if linuxWindowCmd != nil && linuxWindowCmd.Process != nil {
		_ = linuxWindowCmd.Process.Kill()
		linuxWindowCmd = nil
	}
}

// OpenNativeWindow 在 Linux 上优先尝试呼出 Chromium 独立窗口，若无则回退到 xdg-open
func OpenNativeWindow(url string) error {
	linuxWindowCmdMu.Lock()
	defer linuxWindowCmdMu.Unlock()

	for _, bin := range []string{"google-chrome", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(bin); err == nil {
			appArg := "--app=" + url
			cmd := exec.Command(path, appArg, "--window-size=999,666")
			if err := cmd.Start(); err == nil {
				linuxWindowCmd = cmd
				return nil
			}
		}
	}
	return exec.Command("xdg-open", url).Start()
}
