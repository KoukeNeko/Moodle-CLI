//go:build windows

package tunnel

import "os"

// alive says whether a process exists. Signal 0 is not available here, so the
// process is opened instead.
func alive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer process.Release()
	return true
}
