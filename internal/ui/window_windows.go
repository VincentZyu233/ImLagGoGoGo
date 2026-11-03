//go:build windows

package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32Dll               = syscall.NewLazyDLL("kernel32.dll")
	procAllocConsole          = kernel32Dll.NewProc("AllocConsole")
	procAttachConsole         = kernel32Dll.NewProc("AttachConsole")
	procGetConsoleWindow      = kernel32Dll.NewProc("GetConsoleWindow")
	procGetConsoleProcessList = kernel32Dll.NewProc("GetConsoleProcessList")

	user32Dll                    = syscall.NewLazyDLL("user32.dll")
	procGetSystemMetrics         = user32Dll.NewProc("GetSystemMetrics")
	procSetWindowPos             = user32Dll.NewProc("SetWindowPos")
	procEnumWindows              = user32Dll.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32Dll.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible          = user32Dll.NewProc("IsWindowVisible")
	procGetClassName             = user32Dll.NewProc("GetClassNameW")
	procGetWindowText            = user32Dll.NewProc("GetWindowTextW")
	procShowWindow               = user32Dll.NewProc("ShowWindow")
	procPostMessageW             = user32Dll.NewProc("PostMessageW")

	nativeWindowCmdMu sync.Mutex
	nativeWindowCmd   *exec.Cmd

	activeWindowHwndMu sync.Mutex
	activeWindowHwnd   uintptr
)

const (
	attachParentProcess = ^uintptr(0)
	swHide              = 0
	swShow              = 5
	wmClose             = 0x0010
)

// IsLaunchedFromTerminal 判断当前是否由已有的终端（PowerShell / CMD）启动
func IsLaunchedFromTerminal() bool {
	var pids [2]uint32
	ret, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), 2)
	return ret > 1
}

// HideConsoleWindow 隐藏当前进程的控制台黑框
func HideConsoleWindow() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd != 0 {
		procShowWindow.Call(hwnd, uintptr(swHide))
	}
}

// EnsureConsole 智能感知并管理控制台窗口
func EnsureConsole(enable bool) {
	// 无论是否显式开启，都安装 Win32 底层控制台事件拦截器
	InstallConsoleCtrlHandler()

	fromTerminal := IsLaunchedFromTerminal()

	if !enable && !fromTerminal {
		// 桌面双击启动且用户未指定开启控制台：第一毫秒隐去黑框
		HideConsoleWindow()
		return
	}

	// 如果来自已有终端或者显式请求开启，重定向标准输出确保日志直接进入终端
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		ret, _, _ := procAttachConsole.Call(attachParentProcess)
		if ret == 0 {
			procAllocConsole.Call()
		}
	}

	os.Stdout, _ = os.OpenFile("CONOUT$", os.O_WRONLY, 0644)
	os.Stderr, _ = os.OpenFile("CONOUT$", os.O_WRONLY, 0644)
	os.Stdin, _ = os.OpenFile("CONIN$", os.O_RDONLY, 0644)
}

// OpenDefaultBrowser 使用操作系统默认浏览器打开 WebUI 页面
func OpenDefaultBrowser(url string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	return cmd.Start()
}

// CloseNativeWindow 关闭当前由本程序拉起的 Edge 独立 App 战术小窗口
func CloseNativeWindow() {
	activeWindowHwndMu.Lock()
	hwnd := activeWindowHwnd
	activeWindowHwnd = 0
	activeWindowHwndMu.Unlock()

	// 1. 发送 Win32 原生 WM_CLOSE 消息给窗口句柄
	if hwnd != 0 {
		procPostMessageW.Call(hwnd, wmClose, 0, 0)
	}

	nativeWindowCmdMu.Lock()
	defer nativeWindowCmdMu.Unlock()

	// 2. 终止持有的独立 Edge 子进程
	if nativeWindowCmd != nil && nativeWindowCmd.Process != nil {
		log.Printf("🖥️ 正在终止 UI 独立战术沙箱进程 (PID: %d)...", nativeWindowCmd.Process.Pid)
		_ = nativeWindowCmd.Process.Kill()
		nativeWindowCmd = nil
	}
}

// OpenNativeWindow 打开精简的独立 App 桌面窗口
func OpenNativeWindow(url string) error {
	nativeWindowCmdMu.Lock()
	if nativeWindowCmd != nil && nativeWindowCmd.Process != nil {
		_ = nativeWindowCmd.Process.Kill()
		nativeWindowCmd = nil
	}

	edgePath := `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`
	if _, err := os.Stat(edgePath); err != nil {
		edgePath = `C:\Program Files\Microsoft\Edge\Application\msedge.exe`
	}

	// 隔离 Edge Profile，打破 Chromium 全局单例委托机制，确保作为当前进程真正的子进程运行在 Job Object 中
	profileDir := `D:\temp\gemini\ImLagGoGoGo\edge_profile`
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		profileDir = filepath.Join(os.TempDir(), "imlag_edge_profile")
		_ = os.MkdirAll(profileDir, 0755)
	}

	appArg := fmt.Sprintf("--app=%s", url)
	userDataArg := fmt.Sprintf("--user-data-dir=%s", profileDir)
	windowSizeArg := "--window-size=999,666"

	cmd := exec.Command(
		edgePath,
		appArg,
		userDataArg,
		windowSizeArg,
		"--no-first-run",
		"--no-default-browser-check",
	)

	if err := cmd.Start(); err != nil {
		nativeWindowCmdMu.Unlock()
		return err
	}

	nativeWindowCmd = cmd
	nativeWindowCmdMu.Unlock()

	// 显式将 Edge 子进程加入全局 Job Object，实现父进程退出时操作系统内核级强杀
	if cmd.Process != nil {
		AssignProcessToJob(cmd.Process)
	}

	// 异步监听 Edge 窗口关闭事件
	go func() {
		_ = cmd.Wait()
		nativeWindowCmdMu.Lock()
		if nativeWindowCmd == cmd {
			nativeWindowCmd = nil
		}
		nativeWindowCmdMu.Unlock()
	}()

	// 异步居中调整并捕获 HWND
	go centerAndResizeWindow(999, 666)

	return nil
}

// centerAndResizeWindow 屏幕居中并将窗口精确锁定为 width x height
func centerAndResizeWindow(width, height int) {
	screenWidth, _, _ := procGetSystemMetrics.Call(0)  // SM_CXSCREEN
	screenHeight, _, _ := procGetSystemMetrics.Call(1) // SM_CYSCREEN
	if screenWidth == 0 || screenHeight == 0 {
		return
	}

	x := (int(screenWidth) - width) / 2
	y := (int(screenHeight) - height) / 2

	// 在 3 秒内轮询查找 Edge 独立的 App 窗口
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		var foundHwnd uintptr

		cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
			vis, _, _ := procIsWindowVisible.Call(hwnd)
			if vis == 0 {
				return 1 // continue
			}

			var titleBuf [256]uint16
			procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&titleBuf[0])), 256)
			title := syscall.UTF16ToString(titleBuf[:])

			if title == "IMLAG GOGOGO" || title == "ImLagGoGoGo" || (len(title) >= 5 && title[:5] == "ImLag") {
				foundHwnd = hwnd
				return 0 // stop
			}
			return 1
		})

		procEnumWindows.Call(cb, 0)

		if foundHwnd != 0 {
			activeWindowHwndMu.Lock()
			activeWindowHwnd = foundHwnd
			activeWindowHwndMu.Unlock()

			// SWP_SHOWWINDOW = 0x0040
			procSetWindowPos.Call(
				foundHwnd,
				0,
				uintptr(x),
				uintptr(y),
				uintptr(width),
				uintptr(height),
				0x0040,
			)
			return
		}
	}
}
