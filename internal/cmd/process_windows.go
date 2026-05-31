//go:build windows

package cmd

import (
	"math"
	"syscall"

	"golang.org/x/sys/windows"
)

const processStillActive = 259

// newSysProcAttrForDetach returns SysProcAttr for Windows.
// CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW detaches the child from the
// parent's console group without flashing a visible console window.
func newSysProcAttrForDetach() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: 0x00000200 | 0x08000000, // CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW
	}
}

// isProcessRunning checks if a process with the given PID exists.
func isProcessRunning(pid int) bool {
	if pid <= 0 || pid > math.MaxUint32 {
		return false
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() {
		_ = windows.CloseHandle(handle)
	}()

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}

	return exitCode == processStillActive
}
