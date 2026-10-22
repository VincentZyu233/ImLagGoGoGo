//go:build linux

package ui

import (
	"os/exec"
)

func EnsureConsole(_ bool) {}

func OpenNativeWindow(url string) error {
	// Linux 优先尝试 xdg-open
	return exec.Command("xdg-open", url).Start()
}
