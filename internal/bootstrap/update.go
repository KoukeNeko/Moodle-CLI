package bootstrap

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/update"
)

// updateCheck builds the release check the commands run, or nil where there
// should be none. person says whether stderr is a terminal, since the notice it
// produces is for someone reading it. Nothing is asked of the network unless
// all of: a person is there, the environment has not said no, and there is a
// place to remember the answer.
func updateCheck(version string, getenv func(string) string, person bool) func(context.Context) (update.Release, bool) {
	if !person || update.Disabled(getenv) {
		return nil
	}
	path, err := update.DefaultCachePath()
	if err != nil {
		return nil
	}
	return (&update.Checker{Current: version, CachePath: path}).Newer
}
