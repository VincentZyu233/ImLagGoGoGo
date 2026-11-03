//go:build !windows

package ui

import "os"

// InitJobObject 在非 Windows 平台为空实现
func InitJobObject() {}

// AssignProcessToJob 在非 Windows 平台为空实现
func AssignProcessToJob(process *os.Process) {}
