package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/update"
)

var newRelease = update.Release{
	Tag: "v0.4.0",
	URL: "https://github.com/KoukeNeko/Moodle-CLI/releases/tag/v0.4.0",
}

// runChecked runs the CLI with a release check and returns what it printed, the
// exit code and how many times the check was asked.
func runChecked(t *testing.T, check func(context.Context) (update.Release, bool), args ...string) (stdout, stderr string, code int, asked int32) {
	t.Helper()
	var calls atomic.Int32
	var out, errOut bytes.Buffer
	app := New(
		BuildInfo{Version: "0.3.0", Commit: "abc123", BuildDate: "2026-09-18T00:00:00Z"},
		Streams{Out: &out, Err: &errOut},
		Deps{
			ConfigPath: filepath.Join(t.TempDir(), "config.yaml"),
			NewerRelease: func(ctx context.Context) (update.Release, bool) {
				calls.Add(1)
				return check(ctx)
			},
		},
	)
	code = app.Execute(context.Background(), args)
	return out.String(), errOut.String(), code, calls.Load()
}

func found(context.Context) (update.Release, bool) { return newRelease, true }

func TestANewerReleaseIsMentionedOnStderrAfterTheCommand(t *testing.T) {
	stdout, stderr, code, _ := runChecked(t, found, "version")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.HasPrefix(stdout, "moodle 0.3.0 (") || strings.Contains(stdout, "0.4.0") {
		t.Errorf("stdout = %q; the notice must not reach it", stdout)
	}
	for _, want := range []string{"moodle 0.4.0 is available (this is 0.3.0)", newRelease.URL, update.EnvDisable} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want it to contain %q", stderr, want)
		}
	}
}

func TestNothingIsSaidWhenThereIsNothingNewer(t *testing.T) {
	_, stderr, _, asked := runChecked(t, func(context.Context) (update.Release, bool) { return update.Release{}, false }, "version")

	if asked != 1 || stderr != "" {
		t.Errorf("asked %d time(s), stderr = %q; want one question and silence", asked, stderr)
	}
}

func TestTheCheckIsOffWithoutOne(t *testing.T) {
	var out, errOut bytes.Buffer
	app := New(BuildInfo{Version: "0.3.0"}, Streams{Out: &out, Err: &errOut},
		Deps{ConfigPath: filepath.Join(t.TempDir(), "config.yaml")})

	if code := app.Execute(context.Background(), []string{"version"}); code != 0 || errOut.Len() != 0 {
		t.Errorf("exit code = %d, stderr = %q; want 0 and nothing", code, errOut.String())
	}
}

func TestNoPersonIsReadingSoNothingIsAskedOrSaid(t *testing.T) {
	for name, args := range map[string][]string{
		"json":                  {"--json", "version"},
		"no-input":              {"--no-input", "version"},
		"a command that failed": {"course", "list"},
		"help":                  {"help", "version"},
		"the bare command":      {},
		"completion candidates": {"__complete", "ver"},
		"completion script":     {"shell-completion", "bash"},
		"the agent server":      {"mcp", "serve"},
	} {
		t.Run(name, func(t *testing.T) {
			_, stderr, _, _ := runChecked(t, found, args...)

			if strings.Contains(stderr, "is available") {
				t.Errorf("stderr = %q; want no notice", stderr)
			}
		})
	}
}

func TestTheQuestionIsNotAskedWhereItsAnswerWouldNotBeShown(t *testing.T) {
	// Asking costs a request, and the answer is dropped on the floor: do
	// neither.
	for name, args := range map[string][]string{
		"json":                  {"--json", "version"},
		"no-input":              {"--no-input", "version"},
		"help":                  {"help", "version"},
		"the bare command":      {},
		"completion candidates": {"__complete", "ver"},
		"completion script":     {"shell-completion", "bash"},
		"the agent server":      {"mcp", "serve"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, _, asked := runChecked(t, found, args...); asked != 0 {
				t.Errorf("the check was asked %d time(s); want none", asked)
			}
		})
	}
}

func TestASlowAnswerDoesNotHoldTheCommandBack(t *testing.T) {
	grace := updateGrace
	updateGrace = 50 * time.Millisecond
	t.Cleanup(func() { updateGrace = grace })

	abandoned := make(chan struct{})
	slow := func(ctx context.Context) (update.Release, bool) {
		<-ctx.Done()
		close(abandoned)
		return update.Release{}, false
	}

	started := time.Now()
	_, stderr, code, _ := runChecked(t, slow, "version")

	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Errorf("the command took %s", elapsed)
	}
	if code != 0 || stderr != "" {
		t.Errorf("exit code = %d, stderr = %q; want 0 and nothing", code, stderr)
	}
	select {
	case <-abandoned:
	case <-time.After(5 * time.Second):
		t.Error("the question was not called off once the command was done")
	}
}

func TestAnAnswerThatArrivesInTimeIsNotWaitedForLonger(t *testing.T) {
	grace := updateGrace
	updateGrace = time.Hour
	t.Cleanup(func() { updateGrace = grace })

	started := time.Now()
	_, stderr, _, _ := runChecked(t, found, "version")

	if time.Since(started) > 10*time.Second {
		t.Error("the command waited out the grace period for an answer it already had")
	}
	if !strings.Contains(stderr, "is available") {
		t.Errorf("stderr = %q, want the notice", stderr)
	}
}
