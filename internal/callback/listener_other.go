//go:build !unix

package callback

import (
	"context"
	"net"
	"time"
)

// Listen and Receive keep the broker's contract complete on non-Unix builds.
// A supported Windows implementation will replace these with a named pipe;
// until then the platform adapter refuses explicitly instead of breaking the
// whole binary at compile time.
func Listen(string) (net.Listener, error) { return nil, unsupported() }

func Receive(context.Context, net.Listener, time.Duration) (string, error) {
	return "", unsupported()
}
