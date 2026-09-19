package callback

import (
	"regexp"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Scheme is the URL scheme this tool asks the desktop to send it.
//
// Not "moodle", "moodlecli" or "login". A registered scheme is a name in a
// namespace nobody owns — any application may claim the same one — so a
// generic name is a collision waiting to happen, and claiming one somebody
// else is using would take their links.
//
// Never "moodlemobile" in particular. A site can set forcedurlscheme to that,
// and claiming it would be taking the official Moodle app's callbacks.
const Scheme = "moodle-cli-auth"

// MobileScheme is the one this must never claim, named so the check below can
// say why rather than refusing anonymously.
const MobileScheme = "moodlemobile"

// schemePattern is Moodle's own rule, copied from launch.php. A scheme it
// will not redirect to is one there is no point registering.
var schemePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9\-+.]*$`)

// CheckScheme reports whether a scheme may be registered.
func CheckScheme(scheme string) error {
	if !schemePattern.MatchString(scheme) {
		return errs.New(errs.CodeUsage,
			"Moodle will not redirect to a scheme named "+scheme).
			WithHint("it has to start with a letter and hold only letters, " +
				"digits, and the characters . + -")
	}
	if scheme == MobileScheme {
		return errs.New(errs.CodeUsage,
			"that is the Moodle app's own scheme").
			WithHint("a site can send its students to the official app with it; " +
				"taking it here would take those logins away from the app")
	}
	return nil
}
