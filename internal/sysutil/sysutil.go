// Package sysutil contains small utilities related to the operating system, such as detecting whether the process runs with sufficient privileges to edit the hosts file.
package sysutil

import (
	"os"
	"runtime"

	"github.com/ezerevello/focusguard/internal/hostsfile"
)

// IsElevated attempts to determine if the current process has permissions to write to the hosts file (root on Linux/macOS, Administrator on Windows).
// It is a heuristic based on attempting to open the file for writing.
func IsElevated() bool {
	path := hostsfile.Path()
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// OS returns a human-readable name of the operating system.
func OS() string {
	switch runtime.GOOS {
		case "windows":
			return "windows"
		case "linux":
			return "linux"
		case "darwin":
			return "macos"
		default:
			return runtime.GOOS
	}
}
