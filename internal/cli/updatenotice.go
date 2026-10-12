package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/update"
)

// updateGrace is how long a command that has already finished waits for the
// answer. Most commands spend longer than this talking to Moodle, so the
// question is usually answered by then; this is for the ones that do not, and
// it is paid at most once a day.
var updateGrace = time.Second

// updateNotice looks for a newer release while a command runs and says so once
// the command has succeeded. It writes to stderr and only there: the notice is
// for a person, and stdout stays what the command printed.
type updateNotice struct {
	// newer is nil when the check is off.
	newer   func(context.Context) (update.Release, bool)
	current string
	out     io.Writer

	// found and cancel exist from begin until end.
	found  chan *update.Release
	cancel context.CancelFunc
}

// begin starts the question, unless this run is not one a person is reading.
func (n *updateNotice) begin(cmd *cobra.Command, quiet bool) {
	if n.newer == nil || quiet || withoutNotice(cmd) {
		return
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, n.cancel = context.WithCancel(ctx)
	n.found = make(chan *update.Release, 1)
	go func() {
		if release, ok := n.newer(ctx); ok {
			n.found <- &release
			return
		}
		n.found <- nil
	}()
}

// end says what the question found, if it has been answered by now.
func (n *updateNotice) end() {
	if n.found == nil {
		return
	}
	defer n.cancel()
	timer := time.NewTimer(updateGrace)
	defer timer.Stop()
	select {
	case release := <-n.found:
		if release == nil {
			return
		}
		fmt.Fprintf(n.out, "moodle %s is available (this is %s): %s\n"+
			"To stop these checks, set %s=1.\n",
			strings.TrimPrefix(release.Tag, "v"), strings.TrimPrefix(n.current, "v"),
			release.URL, update.EnvDisable)
	case <-timer.C:
	}
}

// withoutNotice reports the commands that are never followed by a notice: the
// ones whose output a program reads (completion candidates and scripts), the
// long-running servers other programs talk to, and help, whose text is itself
// the diagnostic.
func withoutNotice(cmd *cobra.Command) bool {
	if !cmd.HasParent() {
		// The bare command only prints its help and fails.
		return true
	}
	top := cmd
	for top.Parent().HasParent() {
		top = top.Parent()
	}
	switch top.Name() {
	case "mcp", "help", "completion", "shell-completion",
		cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
		return true
	}
	return false
}
