//go:build windows

package ui

import (
	"fmt"
	"os"
	"os/exec"
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
	procGetClassName             = user32Dll.NewProc("GetClassNameW")
	procGetWindowText            = user32Dll.NewProc("GetWindowTextW")
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

// OpenDefaultBrowser 使用操作系统默认浏览器打开 WebUI 页面
func OpenDefaultBrowser(url string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	return cmd.Start()
}

// OpenNativeWindow 打开精简的独立 App 桌面窗口
func OpenNativeWindow(url string) error {
	edgePath := `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`
	if _, err := os.Stat(edgePath); err != nil {
		edgePath = `C:\Program Files\Microsoft\Edge\Application\msedge.exe`
	}

	appArg := fmt.Sprintf("--app=%s", url)
	windowSizeArg := "--window-size=999,666"

	cmd := exec.Command(
		edgePath,
		appArg,
		windowSizeArg,
	)

	if err := cmd.Start(); err != nil {
		return err
	}

	// 异步居中调整
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
