//go:build windows

package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
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

	nativeWindowCmdMu sync.Mutex
	nativeWindowCmd   *exec.Cmd
)

const attachParentProcess = ^uintptr(0)

// EnsureConsole 根据参数在必要时分配或挂载控制台
func EnsureConsole(enable bool) {
	// 无论是否显式开启，都安装控制台底层处理函数
	InstallConsoleCtrlHandler()

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

// CloseNativeWindow 关闭当前由本程序拉起的 Edge 独立 App 战术小窗口
func CloseNativeWindow() {
	nativeWindowCmdMu.Lock()
	defer nativeWindowCmdMu.Unlock()

	if nativeWindowCmd != nil && nativeWindowCmd.Process != nil {
		log.Printf("🖥️ 正在关闭 UI 独立战术窗口 (PID: %d)...", nativeWindowCmd.Process.Pid)
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

	appArg := fmt.Sprintf("--app=%s", url)
	windowSizeArg := "--window-size=999,666"

	cmd := exec.Command(
		edgePath,
		appArg,
		windowSizeArg,
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
