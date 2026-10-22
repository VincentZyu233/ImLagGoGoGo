//go:build windows

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32Dll          = syscall.NewLazyDLL("kernel32.dll")
	procAllocConsole     = kernel32Dll.NewProc("AllocConsole")
	procAttachConsole    = kernel32Dll.NewProc("AttachConsole")
	procGetConsoleWindow = kernel32Dll.NewProc("GetConsoleWindow")

	user32Dll                    = syscall.NewLazyDLL("user32.dll")
	procGetSystemMetrics         = user32Dll.NewProc("GetSystemMetrics")
	procSetWindowPos             = user32Dll.NewProc("SetWindowPos")
	procEnumWindows              = user32Dll.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32Dll.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible          = user32Dll.NewProc("IsWindowVisible")
)

const attachParentProcess = ^uintptr(0)

// EnsureConsole 根据参数在必要时分配或挂载控制台
func EnsureConsole(enable bool) {
	if !enable {
		return
	}

	// 优先尝试依附到父进程控制台
	ret, _, _ := procAttachConsole.Call(attachParentProcess)
	if ret == 0 {
		// 独立分配控制台
		procAllocConsole.Call()
	}

	// 重新定向标准输入输出
	os.Stdout, _ = os.OpenFile("CONOUT$", os.O_WRONLY, 0644)
	os.Stderr, _ = os.OpenFile("CONOUT$", os.O_WRONLY, 0644)
	os.Stdin, _ = os.OpenFile("CONIN$", os.O_RDONLY, 0644)
}

// OpenNativeWindow 打开独立 App 桌面窗口，并强制执行独立沙盒与居中锁定
func OpenNativeWindow(url string) error {
	// Windows 10/11 默认自带 Edge
	edgePath := `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`
	if _, err := os.Stat(edgePath); err != nil {
		edgePath = `C:\Program Files\Microsoft\Edge\Application\msedge.exe`
	}

	// 1. 独立数据目录沙盒 (防止被正在运行的系统 Edge 劫持并恢复拉长的窗口高度)
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = os.TempDir()
	}
	profileDir := filepath.Join(localAppData, "ImLagGoGoGo", "ui_profile")
	_ = os.MkdirAll(profileDir, 0755)

	appArg := fmt.Sprintf("--app=%s", url)
	dataDirArg := fmt.Sprintf("--user-data-dir=%s", profileDir)
	windowSizeArg := "--window-size=760,500"

	cmd := exec.Command(
		edgePath,
		appArg,
		dataDirArg,
		windowSizeArg,
		"--no-first-run",
		"--no-default-browser-check",
	)

	if err := cmd.Start(); err != nil {
		return err
	}

	// 2. 异步在后台精准居中并校准窗口大小
	if cmd.Process != nil {
		go centerAndResizeWindow(cmd.Process.Pid, 760, 500)
	}

	return nil
}

// centerAndResizeWindow 屏幕居中并将窗口精确锁定为 width x height
func centerAndResizeWindow(targetPid int, width, height int) {
	screenWidth, _, _ := procGetSystemMetrics.Call(0)  // SM_CXSCREEN
	screenHeight, _, _ := procGetSystemMetrics.Call(1) // SM_CYSCREEN
	if screenWidth == 0 || screenHeight == 0 {
		return
	}

	x := (int(screenWidth) - width) / 2
	y := (int(screenHeight) - height) / 2

	// 在 3 秒内轮询查找 Edge 创建的主窗口并进行居中调整
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		var foundHwnd uintptr

		cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
			vis, _, _ := procIsWindowVisible.Call(hwnd)
			if vis == 0 {
				return 1 // continue
			}

			var wndPid uint32
			procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&wndPid)))
			if int(wndPid) == targetPid {
				foundHwnd = hwnd
				return 0 // stop enum
			}
			return 1
		})

		procEnumWindows.Call(cb, 0)

		if foundHwnd != 0 {
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
