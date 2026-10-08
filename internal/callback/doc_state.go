package callback

// Browser callbacks arrive through D-Bus on Linux or Apple Events on macOS.
// Both handlers forward the URI over a private per-user Unix socket. Windows
// registration remains unavailable. A callback must answer a live, single-use
// transaction; the authentication method verifies its token with Moodle before
// storing it. The token never travels in a handler command-line argument.
