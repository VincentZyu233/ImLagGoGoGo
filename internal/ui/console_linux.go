//go:build !windows

package ui

import "sync"

var (
	shutdownMu        sync.Mutex
	shutdownCallbacks []func(reason string)
)

// RegisterShutdownHook 注册进程退出时的回调钩子
func RegisterShutdownHook(fn func(reason string)) {
	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	shutdownCallbacks = append(shutdownCallbacks, fn)
}

// TriggerShutdown 手动或通过信号统一触发优雅退出
func TriggerShutdown(reason string) {
	shutdownMu.Lock()
	cbs := make([]func(string), len(shutdownCallbacks))
	copy(cbs, shutdownCallbacks)
	shutdownMu.Unlock()

	for _, cb := range cbs {
		cb(reason)
	}
}

// InstallConsoleCtrlHandler 在非 Windows 平台为空实现
func InstallConsoleCtrlHandler() {}
