//go:build linux

package tunnel

import (
	"os/exec"
	"syscall"
)

// dieWithParent asks the kernel to stop cloudflared if the guard goes away
// without cleaning up. The guard is what stops cloudflared when the host is
// killed; this covers the guard itself being killed, which on other systems
// nothing does.
func dieWithParent(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}
