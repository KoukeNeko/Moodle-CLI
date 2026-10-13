# Moodle CLI v0.4.0 — Claude connector host, new-release notice and assignment overview

## Added

```sh
moodle mcp host
moodle mcp host --public-url https://moodle-mcp.example.com
```

Serves the MCP tools over Streamable HTTP, so Claude can use Moodle as a custom connector without
the CLI running on the machine Claude runs on. With no `--public-url` the host starts a Cloudflare
quick tunnel itself (`cloudflared` must be installed): it binds a free loopback port first and points
the tunnel at that listener, waits until the tunnel carries traffic, checks that the public address
answers with the host's own sign-in metadata before announcing it, and stops when the tunnel does.
`cloudflared` runs with a configuration file of the host's own, because a `config.yml` of the user's
with `ingress:` rules otherwise replaces `--url` and publishes another service, and without `MOODLE_*`
and `TUNNEL_*` environment variables. A quick tunnel requires a loopback `--listen`. `cloudflared` runs
under a guard, a second copy of `moodle` started with a hidden argument, which stops it when the host
exits for any reason: the host holds the write end of the guard's input, the operating system closes
it when the host dies, and the guard stops `cloudflared` on end of input. Verified with `kill -9` on
Linux; macOS and Windows use the same mechanism but were not tried. The host is its own OAuth 2.1 authorization server:
discovery documents, dynamic client registration, PKCE (S256), rotating refresh tokens, and a sign-in
page that asks the owner for a host passphrase and names the application, site, access, and return
address. Read-only unless `--allow-write`, exactly as for `mcp serve`.

Protections: the host listens on loopback by default (`127.0.0.1:8787`, or a free port without
`--public-url`), and a quick tunnel requires a loopback `--listen`; with `--public-url` any `--listen`
is accepted, since a proxy on another machine or a container needs one. Constant-time passphrase
comparison, with the check, the comparison and the recording of a miss as one step so a burst of
guesses cannot get past the global limit of five wrong tries per ten minutes, a 12-character minimum for a chosen passphrase, exact redirect matching
(HTTPS or loopback only, any port on loopback), signed client ids and sign-in requests so
unauthenticated calls store nothing, tokens held as hashes in memory only, every sign-in ended when
a spent authorization code or refresh token is used again (a second use within ten seconds of a
refresh is taken for two requests racing and only fails), a refusal of foreign
`Origin` headers on `/mcp`, and a log that never records a query string, an argument, or a token.
The sign-in page is open to anyone who knows the address, so it omits the account, shows the return
address as punycode, and says the application's name is not verified.

The host logs each tool call's name, outcome, and duration on stderr, so someone running it with
`--allow-write` can see what was done. A dropped connection does not cancel a call in flight;
stopping the host does, after a five-second grace period.

`moodle commands` now classes `mcp host` as `local`, not `read`: it starts a program, binds a port, and
without `--public-url` publishes the account. An empty `--public-url=` is an error rather than a
request for a quick tunnel. Interrupting the host while the tunnel comes up exits `130`.

New flags: `--listen` (default `127.0.0.1:8787` with `--public-url`, a free port without), `--public-url`
(optional), `--passphrase-file`. New environment variable: `MOODLE_HOST_PASSPHRASE`.

## Changed

The MCP server now answers through one transport-independent path that stdio and HTTP share. Tool
calls are serialised, because the session behind them is not safe to share. Behaviour that differs
from before for `mcp serve`:

- A `tools/call` with no `id` is not run. It would be a write nobody can see the outcome of.
- The server stops reading when a reply cannot be written, where it used to keep running requests
  whose answers could not be delivered.
- The text part of a tool result is compact JSON, about 73% smaller on the largest answers, so
  results stay under the roughly 150,000 characters Claude accepts. `structuredContent` is unchanged.
  The `ws_list` description now says an unfiltered call is very large.
- The server's instructions no longer name `mcp serve` when telling an agent that writing needs a
  restart, and the credential hint says a credential supplied through `MOODLE_WS_TOKEN`, and a
  browser session whether stored or from `MOODLE_SESSION`, is only replaced by restarting. A stored
  web service token is still read again without one.
- `moodle ws call`, the MCP `ws_read` tool and `moodle mcp host` now treat ten core functions as
  writes, because each records something although Moodle declares it read: `tool_mobile_get_content`
  (it runs whichever plugin's mobile view it is given, and mod_attendance's can record the student's
  own attendance), the seven `core_competency_*_viewed` functions, `core_blog_view_entries` and
  `mod_lti_view_lti`. They now need `--allow-write`, are never retried, are refused under
  `--read-only`, and a read-only MCP session, including a remote client of `mcp host`, can no longer
  reach them through `ws_read`. The registry snapshots were regenerated from Moodle 4.5.12, 5.1.7 and
  5.2.3 with an explicit override list in `test/e2e/export-ws-registry.php`; only those ten entries
  changed. `core_get_fragment` is still a read: it dispatches to any component's fragment callback,
  which is a maintainer decision this change does not make.

## Fixed

- Values in page requests are now URL-encoded. A course or activity reference containing `&`, `#`, a
  space or `%` used to be pasted into the query string of a request that carries the browser session,
  so it could add parameters or change the request; one with a space produced a malformed request.
- `auth import-browser` now finds Chrome Beta, Dev and Canary, listed as "Chrome Beta", "Chrome Dev"
  and "Chrome Canary". A Chromium-family profile with no cookie database is no longer listed or
  chosen as the default, so an emptied `Chromium/Default` no longer hides the profile that holds the
  session. Only a missing database hides a profile: if it cannot be looked up for another reason,
  such as permission denied, the profile stays listed so an explicit import reports the real error.
  The "several browser profiles" hint now says `--profile` takes a path. On macOS, Chromium-family
  cookies protected by the Keychain are still refused.
- `--verbose` no longer prints credentials a site answers with. The trace masked what was sent but
  quoted the first 120 characters of whatever came back, so `login/token.php` and the QR exchange
  printed the web service token they had just issued and most of the private token, and
  `tool_mobile_get_autologin_key` its key. The same excerpt appears in the hint when a reply cannot
  be read and in the message for an error status, so those showed the values without the flag, and
  an MCP client received them. The excerpt now masks any JSON member or `name=value` pair named
  `token`, `privatetoken`, `wstoken`, `qrloginkey`, `key`, `autologinurl`, `userprivateaccesskey`,
  `sesskey`, `passport` or `password` before it is cut, so a secret that straddles the limit is hidden
  whole. A request parameter named `key` or `autologinurl` is now redacted in the trace too, and a
  calendar or forum link carrying one of these names is no longer shown.
- `moodle api call` no longer demands `--allow-write` for `tool_mobile_get_plugins_supporting_mobile`
  and `core_calendar_get_calendar_events`, which only read. `tool_mobile_get_content` is on the
  reviewed list as a write, so it is reported as reviewed rather than unreviewed.

## Assignment overview, and quizzes over MCP

```sh
moodle assignment overview --current --json
```

Lists assignments with the signed-in account's submission state in one command; `--course` and
`--current` narrow it. The state is read once per assignment, four at a time. A status that could not
be read comes back as a `null` submission with a `status_error` and `meta.partial`, never as an item
that was not submitted. New kind `assignment.overview`, with its schema.

The MCP server gains three read tools: `assignment_overview`, `quiz_list` and `quiz_show`. When the
account is signed in with a browser session it now also falls back to the page and AJAX routes for
courses, assignments and quizzes, as the CLI does, over both `mcp serve` and `mcp host`. That session
is read once, when the server starts, so renewing it takes a restart, and the expired-session hint
says so. `assignment overview` was run read-only against one real university site; the new MCP tools
were tested against a fake site and have not been run against a real one.

## New-release notice

At an interactive terminal, a command that succeeds prints one notice on stderr when a newer release
exists, and nothing else changes: nothing is downloaded or installed. At most once a day the program
sends github.com a `HEAD` request for the repository's `releases/latest` link and reads where it
redirects, the same question the install script asks, and keeps the answer in `update-check.json` in
the user cache directory. The request carries the user agent `moodle-cli/<version>` and no account,
site, or credential. It is not made with `--json` or `--no-input`, for `mcp`, completion, or help,
when stderr is not a terminal, when `CI` is set, in a build that is not a release, or when
`MOODLE_CLI_NO_UPDATE_CHECK` is set to anything but `0`, `false`, `no`, or `off`. A command waits
at most one second for the answer; an attempt that fails or is abandoned still counts as the day's.

The README and the security model said the program contacts only the configured Moodle site and has
no updater; they now name this request. The notice is a person's note, so it goes to stderr and the
envelope is unchanged.

## Documentation and limitations

New wiki page, [Claude connector](docs/wiki/Claude-Connector.md): what a connector is relative to MCP,
how the connection works, setup with a Cloudflare quick tunnel, controls, limits, and troubleshooting.

Verified on 2026-10-12 against Moodle 4.5.12: the host's own quick tunnel with cloudflared 2026.9.0 on
Linux and the final code (up, address checked before it is announced, discovery documents served
through the public address, stops with the host leaving nothing behind; with a `config.yml` whose
`ingress:` points at another local service the tunnel still reached the host and that service
received no requests; the host stopping when the tunnel is killed was run with an earlier version, and
the final code is covered by a test with a stand-in for cloudflared; a claude.ai sign-in through a
host-started tunnel was not repeated), claude.ai in the browser (custom connector added, signed
in, 14 tools listed, two tool calls answered), Claude Code 2.1.267 on loopback, the official MCP
Inspector through a Cloudflare quick tunnel, and an independent client for the full sign-in. Not
tried: Claude Desktop, the phone apps, Cowork, adding the connector as a Team or Enterprise Owner,
and a connector whose host has restarted.

Claude opens each connection with a `server/discover` request under revision 2026-07-28 and falls
back to `initialize` when the host answers `400`, so every connection logs one `POST /mcp 400`. The
host speaks 2025-06-18 and older. Sign-ins do not survive a restart and the connector then has to be
removed and added again; one host serves one account; a refresh token works once, and using a spent one again ends the sign-in; responses are plain
JSON with no event stream, session, or batching.

## Contract

`schema_version` stays `1`. One new kind was added, `assignment.overview`; no field was added to an existing
kind. `moodle version` is unchanged.

# Moodle CLI v0.3.0 — automatic browser sign-in on macOS

## Added

```sh
moodle auth login --site school --method mobilelaunch
```

On macOS, press Enter to open the normal browser, complete institutional SSO, and let the CLI
receive, verify, and save Moodle's Web Service token automatically. First sign-in installs a
per-user URL handler in `~/Applications` using macOS's built-in AppleScript compiler; Xcode and
manual callback copying are not required. Linux retains its registered D-Bus callback handler.

The macOS helper receives an Apple Event and passes the callback through an anonymous stdin pipe
and a private Unix socket. Token-bearing URLs are kept out of handler process arguments and
temporary files. Registration preserves unrelated applications, supports repeated registration,
and restores the previous helper if an update fails. The locally compiled helper is signed ad hoc;
the distributed CLI retains Developer ID signing and Apple notarization.

## Documentation and limitations

The English and Traditional Chinese README and handbook now explain the separate OAuth and mobile
callbacks, token lifetime, signing in again, and choosing authentication for an agent harness.
Harness guidance covers reusing credentials, explicit login methods, secure stdin/environment
inputs, desktop browser sign-in, and headless credential provisioning.

An eCourse2 browser sign-in and subsequent Web Service course listing were verified on 2026-10-09.
Its actual token expiry was not exposed by the APIs checked; Moodle's default token duration is
not a confirmed expiry for that site. The CLI has no automatic token refresh or remote/device-code
login flow. Windows automatic callbacks remain unsupported. `mobilelaunch --no-input` skips the
Enter prompt but still requires a browser sign-in and callback, so it is not a headless login mode.

## Contract

`schema_version` stays `1`; no fields were added to the JSON contract.

## v0.2.1 — one command to sign in, and honest gaps

Driven by signing in to a real university Moodle from scratch: an SSO site whose
session had expired, where every route that worked needed something pasted.

## Added

```sh
moodle setup https://moodle.example.edu
```

One command asks the site what it supports, says what its configuration implies,
lists every login method with its status, names the one that suits the site and
why, and runs the choice. Two commands were correct and unhelpful: `site add`
succeeds whatever the site turns out to be, and `auth login` then chooses
automatically — which skips every method needing a paste, and on an SSO site
that is every method that works. A student's introduction to the tool was
"no login method can run without more information".

The suggestion reads the site's own configuration rather than ranking methods in
the abstract. Measured against a real SSO site, it recommends `browser-session`,
which is what worked there — and unlike `auth import-session`, that trades the
session for a token which does not expire with it.

`auth login` now reads the QR content and the browser session from stdin, with
`--qr-stdin` and `--session-cookie-stdin`. Only the token and password could
arrive that way; the rest had to travel in argv, where `ps` shows them to anyone
on the machine and the shell keeps them in history. Passing two stdin flags is
refused rather than signing in with the second credential empty.

## Fixed

`browser-session` only read a flag, so choosing it from a menu failed with a
message naming that flag. It now prompts, with the input hidden: a session cookie
outlives the command, so echoing it would leave it in the terminal's scrollback.

The grade table printed Moodle's own markup. Measured on Moodle 4.5.3: the report
returns the item name as stored, so a multilang name arrived as one span per
language, and a scale's mark arrived wrapped in the icon the web UI draws. Both
were unreadable and both wrecked the column widths. The JSON still passes
Moodle's rendering through unchanged.

An assignment listing said it was incomplete without saying what was short.
Moodle answers an activity this account may not read with a warning and a
complete HTTP 200 — measured on a real site, 11 of a course's 15 assignments
withheld from a student while the listing reported four. The count is now
reported, because a module id for an activity you cannot open is not something
to act on.

## Contract

`schema_version` stays `1`, and nothing was added to it.

ADR-0003 §8 promised that adding a field was compatible and that consumers must
ignore unknown ones. The schemas never allowed it: 35 of 36 set
`additionalProperties: false` from the first release, so any addition failed
validation. The text now matches the implementation rather than the schemas being
loosened — a closed schema is what catches a consumer's typo, and `--fields`
refuses an unknown name from the same source. Inside v1, what may be added is a
new `kind`, a new `reason` value and a new enum value; a note meant for a person
goes to stderr. A test asserts every published object stays closed.

## v0.2.0 — correct dates, page-route reads, quizzes, and an agent-ready surface

This release was driven by running the previous one against a real university Moodle: a
Moodle 4.5 site behind SSO that issues no web service token, where every read has to come from the
AJAX endpoint or the site's own pages.

## Fixed

Dates in the human tables were sliced off the UTC timestamp, so a Taiwanese term starting on
1 August read as 31 July and any deadline before 08:00 local time moved a day. Tables also counted a
CJK character as one terminal column, which pushed every later column out of line on a site with
Chinese course names.

On a token-less site, assignments came back with no name, no course and no deadline while `meta`
claimed a complete answer — against ADR-0001 §6. The page routes now read the name, course,
description and attachments from the activity's own page and the deadline from the calendar's month
view as a number rather than prose, list a forum's threads, and name in `meta.missing` every field
they cannot see. `file download` works with a browser session, `site inspect` no longer reports a
valid session as expired, and typed calls say a web service token is needed instead of complaining
about a missing registry snapshot. A site-wide forum linked from a side block is no longer counted
as every course's own.

Ctrl-C was reported as a network error, which told a script the user's own decision was transport
trouble worth retrying. It is now exit `130`, deliberately outside the code table. An interrupt that
cut off a **write** after the request was sent is exit `12`, ambiguous: Moodle does not undo a write
because the client stopped listening, and reporting a clean failure invites a second submission.

Tab completion never worked. Cobra skips its own generator when a command called `completion`
exists, which Moodle's activity completion does, and the hidden `__complete` wrote its candidates to
stderr because Cobra's output is aimed there to keep help off stdout.

## Added

```sh
moodle quiz list --current
moodle quiz show 1436146
moodle assignment list --current --json --fields name,due_date --no-input
moodle schema assignment submit --json
moodle shell-completion zsh > "${fpath[1]}/_moodle"
moodle --credential-store file auth login --site school --token-stdin
```

`quiz list` and `quiz show` read quizzes, and with a token the attempts scaled to the quiz's own
maximum and the best grade. `--current` keeps only courses that have started and not yet ended, on
`course`, `assignment` and `quiz list`.

For anything unattended: `--fields` keeps the payload to the fields actually read and refuses an
unknown name with the real ones; `--no-input` turns every prompt into an error instead of a hung
process; and `moodle schema <command>` reports each command's `safety` (`read`, `local` or `write`)
and `idempotency` alongside its input and output JSON Schema. `moodle commands --json` carries the
same two fields for the whole surface. The wiki gains an automation recipes page with worked
examples and rules to hand an agent.

`-v` finally switches on the redacted HTTP trace that already existed and was tested. A machine with
no keychain — headless Linux, SSH, WSL, a container — can keep credentials in a `0600` file with
`--credential-store file`, which is never automatic; ADR-0004 records the amended decision.

`scripts/install.sh` and `scripts/install.ps1` verify the archive against the release's
`checksums.txt` and refuse a digest that does not match; the uninstallers remove exactly what was
installed and leave a hand-made completion alone. Releases now also carry `.deb`, `.rpm` and `.apk`,
which install the shell completions. [COMPATIBILITY.md](COMPATIBILITY.md) is a new matrix of what is
verified, by what evidence, and what is untested.

## Contract

`schema_version` stays `1`. Additions only: the `command.schema` kind, `safety` and `idempotency` on
`commands`, `quiz.list` and `quiz.show`, and exit `130` with `reason: "interrupted"` — whose `code`
remains `network`, because that set is closed. `meta.partial` and `meta.missing` are now populated by
the page routes that previously left them empty, so a consumer that already reads them sees the
truth where it used to see silence.

## v0.1.3 — Safari session fallback

Safari may show a signed-in Moodle page while its on-disk cookie file contains no
`MoodleSession`. This release stops treating that snapshot as the only Safari route:

```sh
moodle auth import-session --site school
```

Copy only the active `MoodleSession` value from Safari Web Inspector and paste it at the
hidden terminal prompt. The CLI verifies the session with the selected Moodle site before
storing it in the OS keychain; the value is never placed in a command argument or echoed.
`--stdin` is available for a secure pipe. This fallback does not need Full Disk Access or
another browser. The existing Safari file import remains available when the cookie is
present there. macOS automatic callback handling is still not implemented.

Login hints, README, and English/Traditional Chinese Wiki now explain this limitation
and route. Tests cover accepted input, rejection, no secret echo, JSON output, and
architectural boundaries.

## v0.1.2 — Safari session import on macOS

This release lets macOS users import an existing Safari Moodle session without installing Firefox:

```sh
moodle auth import-browser --site school --browser safari --store
```

The command reads only when explicitly requested, selects the named site's session cookie, verifies
the session with Moodle before storing it in the OS keychain, and reports missing access or an
unrecognized Safari cookie format clearly. Safari's cookie store is not a public Apple API, so this
remains best-effort and may require macOS privacy permission. The automatic mobile-launch callback
handler is still Linux-only; macOS login hints now say so and point to Safari import instead of
suggesting `register-handler`.

The README and English/Traditional Chinese Wiki describe the new `--browser` selector and its
limitations. Other browser import methods and the JSON contract are unchanged.

## v0.1.1 — first public release

Moodle CLI is an independent command-line client for learners, educators, administrators, scripts,
and agents. `v0.1.0` was a failed, unpublished release attempt; `v0.1.1` is the first public release.

## Highlights

- Read courses, participants, groups, assignments, grades, forums, calendars, completion, workload,
  and files; use reviewed high-level writes when the active Moodle account has permission.
- Inspect and call the typed registry of 780 core Web Service functions across Moodle 4.5.12, 5.1.7,
  and 5.2.3. Unknown and third-party functions remain available through `moodle api call`.
- Use human-readable output or the versioned `--json` contract. MCP tools are read-only by default;
  write tools require an explicit opt-in.
- Log in with a site-supported token, credentials, browser session, or mobile/browser flow. The
  automatic browser callback handler is currently Linux-only; manual callback works on all platforms.
- Verify behavior against Docker-backed Moodle 4.5.12, 5.1.7, and 5.2.3 test sites, including
  ten-year academic scenarios. The separate scale fixture models 50,000 students and 1,000 courses.

## Install

macOS or Linux with Homebrew:

```sh
brew tap KoukeNeko/tap
brew install koukeneko/tap/moodle-cli
```

Windows with Scoop:

```powershell
scoop bucket add koukeneko https://github.com/KoukeNeko/scoop-bucket
scoop install koukeneko/moodle-cli
```

Or download the archive for your OS and CPU below. Compare it with `checksums.txt`; verify
`checksums.txt.sigstore.json` with Cosign and the release workflow's GitHub Actions identity.
The macOS binaries are Developer ID-signed and notarized. Windows binaries are not
Authenticode-signed, so SmartScreen may warn.

See the [installation handbook](https://github.com/KoukeNeko/Moodle-CLI/wiki/Installation),
[繁體中文安裝手冊](https://github.com/KoukeNeko/Moodle-CLI/wiki/Installation-zh-TW), and
[complete command reference](https://github.com/KoukeNeko/Moodle-CLI/wiki/Command-Reference).
