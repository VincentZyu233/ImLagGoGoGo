//go:build windows

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

var (
	kernel32Dll         = syscall.NewLazyDLL("kernel32.dll")
	procAllocConsole    = kernel32Dll.NewProc("AllocConsole")
	procAttachConsole   = kernel32Dll.NewProc("AttachConsole")
	procGetConsoleWindow = kernel32Dll.NewProc("GetConsoleWindow")
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

// OpenNativeWindow 打开无边框/独立 App 桌面窗口
func OpenNativeWindow(url string) error {
	// Windows 10/11 默认自带 Edge
	edgePath := `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`
	if _, err := os.Stat(edgePath); err != nil {
		edgePath = `C:\Program Files\Microsoft\Edge\Application\msedge.exe`
	}

	appArg := fmt.Sprintf("--app=%s", url)
	cmd := exec.Command(edgePath, appArg, "--window-size=780,550")
	return cmd.Start()
}
