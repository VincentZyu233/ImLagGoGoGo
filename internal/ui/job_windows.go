//go:build windows

package ui

import (
	"log"
	"os"
	"syscall"
	"unsafe"
)

var (
	procCreateJobObjectW         = kernel32Dll.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = kernel32Dll.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = kernel32Dll.NewProc("AssignProcessToJobObject")
	procGetCurrentProcess        = kernel32Dll.NewProc("GetCurrentProcess")
)

const (
	JobObjectExtendedLimitInformation = 9
	JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x00002000

	processTerminate = 0x0001
	processSetQuota  = 0x0100
)

type jobobjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobobjectExtendedLimitInformation struct {
	BasicLimitInformation jobobjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

var globalJobObject uintptr

// InitJobObject 创建并配置当前进程所属的 Windows Job Object。
// 设置 JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE，使当前进程终止或崩溃时，
// 操作系统内核级自动销毁该 Job 内所有子进程（如 Edge 战术小窗口），杜绝孤儿进程残留。
func InitJobObject() {
	job, _, err := procCreateJobObjectW.Call(0, 0)
	if job == 0 {
		log.Printf("⚠️ 创建 Job Object 失败: %v", err)
		return
	}

	var info jobobjectExtendedLimitInformation
	info.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

	ret, _, err := procSetInformationJobObject.Call(
		job,
		uintptr(JobObjectExtendedLimitInformation),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	if ret == 0 {
		log.Printf("⚠️ 配置 Job Object 限制失败: %v", err)
		return
	}

	curProc, _, _ := procGetCurrentProcess.Call()
	ret, _, err = procAssignProcessToJobObject.Call(job, curProc)
	if ret == 0 {
		log.Printf("⚠️ 将主进程分配至 Job Object 失败: %v", err)
		return
	}

	globalJobObject = job
}

// AssignProcessToJob 将指定进程句柄显式加入 Job Object
func AssignProcessToJob(process *os.Process) {
	if globalJobObject == 0 || process == nil {
		return
	}
	procHandle, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(process.Pid))
	if err != nil {
		return
	}
	defer syscall.CloseHandle(procHandle)
	procAssignProcessToJobObject.Call(globalJobObject, uintptr(procHandle))
}
