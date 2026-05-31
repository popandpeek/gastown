//go:build !windows

package cmd

import "syscall"

// newSysProcAttrForDetach returns SysProcAttr that detaches the child from
// the parent's process group so it survives the caller's exit.
func newSysProcAttrForDetach() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// isProcessRunning checks if a process with the given PID exists.
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}

	// EPERM means process exists but we don't have permission to signal it.
	return err == syscall.EPERM
}
