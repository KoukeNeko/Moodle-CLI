# Moodle CLI v0.2.0 — correct dates, page-route reads, quizzes, and an agent-ready surface

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
