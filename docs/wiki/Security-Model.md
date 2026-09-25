# Security model

[繁體中文](Security-Model-zh-TW) · [Home](Home)

Moodle CLI handles authentication material, talks to a university service, and can submit work. Its
security boundary is intentionally narrow and visible.

## Credentials

- Tokens and browser sessions are stored in the operating-system keychain under Moodle CLI's own
  entries. The configuration file contains metadata only.
- `MOODLE_WS_TOKEN` and `MOODLE_SESSION` supply an ephemeral credential for one process and take
  precedence over stored credentials.
- Secrets should enter through stdin or environment variables, not command-line arguments.
- `auth logout` deletes the local copy. It does not revoke the Moodle token because that token may be
  shared with the official mobile app.

## Browser access

Browser profile access happens only after `auth import-browser`. The command searches Safari on
macOS, Firefox, or Chromium-family storage for the requested host. Other cookies may be parsed
because browsers store them together, but only the matching Moodle session is returned or stored;
none are logged. An explicit `--browser safari` does not inspect other browser profiles.

Not finding a cookie means only that it was absent from the readable browser snapshot. It does not
prove the user is signed out.

`auth import-session` does not read any browser profile. It accepts one cookie at a hidden terminal
prompt, or from an explicitly requested stdin pipe, verifies it with the chosen Moodle site, then
stores it in the OS keychain. The value is not accepted as a command-line argument or echoed in
diagnostics. The person running the command must copy the cookie from their own browser; it must
never be sent in a screenshot, chat, or issue.

## Linux callback handler

The automatic mobile-launch callback uses per-user desktop and D-Bus service files. D-Bus activation
keeps the token-bearing callback out of `/proc/<pid>/cmdline`. A callback is accepted only when it:

1. matches a live, unexpired login transaction;
2. carries the expected site hash and passport proof;
3. has not already been claimed; and
4. contains a token the target Moodle site accepts.

macOS and Windows do not install a partial or unverified handler; they return an explicit unsupported
error and direct the user to the manual flow.

## Network and writes

- Requests go only to the configured Moodle origin. Credentials are not forwarded cross-origin.
- Official Web Service functions use a reviewed mutation/retry registry. Unknown functions are
  classified as writes and never retried.
- AJAX and HTML adapters are read-only.
- Assignment writes are reconciled by reading the resulting state. If the state remains unknown, the
  outcome is `ambiguous` and automatic retry is forbidden.
- Request throttling and Moodle's `Retry-After` are respected.

## Out of scope

The CLI cannot protect a compromised operating system, an unlocked user keychain, a malicious
Moodle server, or a browser profile readable by another local process. It provides no telemetry,
automatic updater, privilege escalation, process injection, or remote credential backup.

Report a suspected vulnerability privately to the repository owner before opening a public issue
when disclosure would expose users or credentials.
