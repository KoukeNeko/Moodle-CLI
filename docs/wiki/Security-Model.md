# Security model

[繁體中文](Security-Model-zh-TW) · [Home](Home)

Moodle CLI handles authentication material, talks to a university service, and can submit work. Its
security boundary is intentionally narrow and visible.

## Credentials

- Tokens and browser sessions are stored in the operating-system keychain under Moodle CLI's own
  entries. The configuration file contains metadata only.
- A machine with no keychain — headless Linux, SSH, WSL, a container — can keep them in a file
  instead, with `--credential-store file` or `preferences.credential_store: file`. Nothing switches
  to a file on its own: the file is `credentials.json` beside the configuration, created `0600` in a
  `0700` directory, and `auth login` reports the path. Mode `0600` keeps other accounts on the
  machine out and nothing else — it does not protect against anything running as this user, which is
  equally true of an unlocked desktop keychain. Windows has no mode bits, so there the file relies on
  the ACL `%APPDATA%` already carries; Credential Manager is the default on that platform anyway.
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

## Browser callback handlers

The automatic mobile-launch callback uses per-user desktop and D-Bus service files. D-Bus activation
keeps the token-bearing callback out of `/proc/<pid>/cmdline`. A callback is accepted only when it:

1. matches a live, unexpired login transaction;
2. carries the expected site hash and passport proof;
3. has not already been claimed; and
4. contains a token the target Moodle site accepts.

On macOS, a per-user application receives the URL as an Apple Event and sends it to the CLI through an
anonymous stdin pipe. It never writes the token to a temporary file or a process argument. The CLI
forwards it through the same private Unix socket and transaction checks as Linux. The locally compiled
application is signed ad hoc; the release binary retains its separate release signing.
Windows automatic callbacks remain unsupported.

## Network and writes

- Requests go only to the configured Moodle origin. Credentials are not forwarded cross-origin.
- Official Web Service functions use a reviewed mutation/retry registry. Unknown functions are
  classified as writes and never retried.
- AJAX and HTML adapters are read-only.
- Assignment writes are reconciled by reading the resulting state. If the state remains unknown, the
  outcome is `ambiguous` and automatic retry is forbidden.
- Request throttling and Moodle's `Retry-After` are respected.

## Host mode

`moodle mcp host` is the one command that accepts network connections. It listens on the address
given by `--listen`, which should be loopback, and is meant to sit behind a tunnel or proxy that
terminates HTTPS. Without `--public-url` it starts a Cloudflare quick tunnel itself, which publishes it
at a random address until it stops. It binds the port first and points the tunnel at that listener,
runs `cloudflared` with a configuration file of its own and without `MOODLE_*` and `TUNNEL_*`
environment variables, runs it under a guard process that stops it if the host is killed, and checks that the public address
reaches it before announcing it. A client reaches the tools only with an access token issued after the owner enters
the host passphrase on a page that names the site, the access being granted, and where the browser
returns. Sign-ins are held in memory and end when the process stops. While it runs, the host holds
the account's Moodle credential, and anyone it has admitted acts as that account. The tunnel provider
and Anthropic can see the traffic, and course content written by other people reaches Claude, which
matters most when writing is on. The log records each tool call's name, outcome, and duration, never
its arguments. [Claude connector](Claude-Connector) lists every control and limit.

## Out of scope

The CLI cannot protect a compromised operating system, an unlocked user keychain, a malicious
Moodle server, or a browser profile readable by another local process. It provides no telemetry,
automatic updater, privilege escalation, process injection, or remote credential backup.

Report a suspected vulnerability privately to the repository owner before opening a public issue
when disclosure would expose users or credentials.
