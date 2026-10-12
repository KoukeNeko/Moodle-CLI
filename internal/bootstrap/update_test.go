package bootstrap

import "testing"

func TestTheReleaseCheckRunsOnlyWhereAPersonIsReadingAndHasNotSaidNo(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	nothing := func(string) string { return "" }

	for _, tc := range []struct {
		name   string
		env    func(string) string
		person bool
		want   bool
	}{
		{"a person at a terminal", nothing, true, true},
		{"stderr is a pipe or a file", nothing, false, false},
		{"the opt-out is set", func(name string) string {
			if name == "MOODLE_CLI_NO_UPDATE_CHECK" {
				return "1"
			}
			return ""
		}, true, false},
		{"automation", func(name string) string {
			if name == "CI" {
				return "true"
			}
			return ""
		}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := updateCheck("0.3.0", tc.env, tc.person) != nil; got != tc.want {
				t.Errorf("check built = %v, want %v", got, tc.want)
			}
		})
	}
}
