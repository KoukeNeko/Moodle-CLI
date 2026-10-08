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
