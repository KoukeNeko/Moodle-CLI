package callback

// This package is groundwork, and nothing calls it yet.
//
// It exists because the browser login Moodle offers ends with the site
// redirecting to "<scheme>://token=<payload>", which means registering a URL
// handler with the operating system. That handler runs as a separate process
// and has to pass a credential back to the one that is waiting, and the
// address it carries can be produced by anything on the machine — so the
// parts below are the ones worth being careful about, and the ones that can
// be checked without a desktop:
//
//   - a login is answerable once, expires, and cannot be answered by another;
//   - the channel it comes back through is reachable only by this user;
//   - a callback is refused unless it is shaped exactly as Moodle sends one.
//
// What is missing is registration itself, per platform, and the last step of
// the path: browser to operating system to handler. None of that can be
// verified on a machine with no desktop, no browser, and neither macOS nor
// Windows. Writing it from specifications and calling it done is how a
// credential channel ends up untested, so it is left undone and written down
// instead — see TODO.md, Phase 5.
