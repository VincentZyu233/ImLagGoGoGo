//go:build windows

package ui

import (
	"log"
	"sync"
	"syscall"
)

var (
	procSetConsoleCtrlHandler = kernel32Dll.NewProc("SetConsoleCtrlHandler")

	shutdownMu           sync.Mutex
	shutdownCallbacks    []func(reason string)
	ctrlHandlerInstalled bool
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

// InstallConsoleCtrlHandler 注册底层 Win32 控制台控制处理函数
// 直接捕获 CTRL_C_EVENT, CTRL_BREAK_EVENT, CTRL_CLOSE_EVENT 等，解决 AttachConsole 后 Go 信号监听失效的问题
func InstallConsoleCtrlHandler() {
	if ctrlHandlerInstalled {
		return
	}
	ctrlHandlerInstalled = true

	cb := syscall.NewCallback(func(ctrlType uint32) uintptr {
		switch ctrlType {
		case 0: // CTRL_C_EVENT
			log.Println("🛑 [控制台] 捕获到 Ctrl+C 中断信号，准备终止对局引擎...")
			TriggerShutdown("Console Ctrl+C")
			return 1
		case 1: // CTRL_BREAK_EVENT
			log.Println("🛑 [控制台] 捕获到 Ctrl+Break 中断信号，准备终止对局引擎...")
			TriggerShutdown("Console Ctrl+Break")
			return 1
		case 2: // CTRL_CLOSE_EVENT
			log.Println("🛑 [控制台] 控制台窗口被用户关闭，退出后台引擎...")
			TriggerShutdown("Console Window Closed")
			return 1
		case 5, 6: // CTRL_LOGOFF_EVENT, CTRL_SHUTDOWN_EVENT
			TriggerShutdown("System Shutdown")
			return 1
		}
		return 0
	})

	procSetConsoleCtrlHandler.Call(cb, 1)
}
