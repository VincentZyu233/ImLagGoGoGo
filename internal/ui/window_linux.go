//go:build linux

package ui

import (
	"os/exec"
)

func EnsureConsole(_ bool) {}

// OpenDefaultBrowser 在 Linux 上使用默认浏览器打开
func OpenDefaultBrowser(url string) error {
	return exec.Command("xdg-open", url).Start()
}

// OpenNativeWindow 在 Linux 上优先尝试呼出 Chromium 独立窗口，若无则回退到 xdg-open
func OpenNativeWindow(url string) error {
	for _, bin := range []string{"google-chrome", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(bin); err == nil {
			appArg := "--app=" + url
			return exec.Command(path, appArg, "--window-size=999,666").Start()
		}
	}
	return exec.Command("xdg-open", url).Start()
}
