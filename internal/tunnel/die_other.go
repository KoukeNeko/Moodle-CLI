//go:build !linux

package tunnel

import "os/exec"

// dieWithParent has no equivalent here: neither macOS nor Windows can tell a
// child to stop when its parent is gone. The guard still stops cloudflared when
// the host is killed; only the guard itself being killed, as well, would leave
// it running.
func dieWithParent(*exec.Cmd) {}
