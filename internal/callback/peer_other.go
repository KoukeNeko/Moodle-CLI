//go:build unix && !linux

package callback

import "net"

// PeerIsSelf cannot be answered here yet.
//
// SO_PEERCRED is Linux's; the others have LOCAL_PEERCRED and getpeereid,
// which differ enough to be worth writing against a real system rather than
// from documentation.
//
// It reports the check as unavailable rather than returning true. Those are
// different things: the caller may well decide to carry on — the boundary is
// the runtime directory's mode and this was always the second line — but it
// should decide that knowingly, not be told a check passed that never ran.
func PeerIsSelf(net.Conn) (bool, error) { return false, ErrPeerCheckUnavailable }
