package windows

import (
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
)

const (
	processQueryLimitedInformation = 0x1000
)

type CS2Tracker struct{}

func NewCS2Tracker() *CS2Tracker {
	return &CS2Tracker{}
}

// IsCS2Foreground 判断当前处于前台活跃状态的窗口是否属于 cs2.exe
func (t *CS2Tracker) IsCS2Foreground() bool {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return false
	}

	hProcess, _, _ := procOpenProcess.Call(
		uintptr(processQueryLimitedInformation),
		0,
		uintptr(pid),
	)
	if hProcess == 0 {
		return false
	}
	defer procCloseHandle.Call(hProcess)

	var buf [1024]uint16
	size := uint32(len(buf))
	ret, _, _ := procQueryFullProcessImageNameW.Call(
		hProcess,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return false
	}

	exePath := syscall.UTF16ToString(buf[:size])
	exeName := strings.ToLower(filepath.Base(exePath))

	return exeName == "cs2.exe"
}

// IsCS2Running 判断系统中是否有 cs2.exe 进程正在运行
func (t *CS2Tracker) IsCS2Running() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32Snapshot := kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW := kernel32.NewProc("Process32FirstW")
	procProcess32NextW := kernel32.NewProc("Process32NextW")

	const th32csSnapProcess = 0x00000002
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snapshot == uintptr(syscall.InvalidHandle) {
		return false
	}
	defer procCloseHandle.Call(snapshot)

	type processEntry32W struct {
		Size              uint32
		Usage             uint32
		ProcessID         uint32
		DefaultHeapID     uintptr
		ModuleID          uint32
		Threads           uint32
		ParentProcessID   uint32
		PriClassBase      int32
		Flags             uint32
		ExeFile           [260]uint16
	}

	var pe processEntry32W
	pe.Size = uint32(unsafe.Sizeof(pe))

	ret, _, _ := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&pe)))
	for ret != 0 {
		name := strings.ToLower(syscall.UTF16ToString(pe.ExeFile[:]))
		if name == "cs2.exe" {
			return true
		}
		ret, _, _ = procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&pe)))
	}

	return false
}
