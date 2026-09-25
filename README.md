<h1 align="center">Moodle CLI</h1>

<p align="center">
  <strong>An independent Moodle command-line client for learners, educators, and administrators.</strong><br>
  Readable terminal output for people, and a versioned contract for scripts, CI, and agents.
</p>

<p align="center">
  <a href="#compatibility"><img alt="Verified with Moodle 4.5.12, 5.1.7, and 5.2.3" src="https://img.shields.io/badge/MOODLE-4.5.12%20%7C%205.1.7%20%7C%205.2.3-FF8B00?style=for-the-badge&logo=moodle&logoColor=white"></a>
  <a href="#compatibility"><img alt="Linux, macOS, and Windows" src="https://img.shields.io/badge/PLATFORMS-LINUX%20%7C%20MACOS%20%7C%20WINDOWS-5C6BC0?style=for-the-badge"></a>
  <a href="#a-contract-fit-for-automation"><img alt="JSON contract version 1" src="https://img.shields.io/badge/JSON%20CONTRACT-V1-009688?style=for-the-badge&logo=json&logoColor=white"></a>
</p>

<p align="center">
  <strong>English</strong> · <a href="README.zh-TW.md">繁體中文</a>
</p>

<p align="center">
  <a href="#getting-started">Getting started</a>
  · <a href="https://github.com/KoukeNeko/Moodle-CLI/wiki">Handbook</a>
  · <a href="https://koukeneko.github.io/Moodle-CLI/">Verification dashboard</a>
  · <a href="test/e2e/README.md">Test lab</a>
  · <a href="docs/architecture.md">Architecture</a>
</p>

```sh
moodle course list
moodle calendar upcoming
moodle participant list --param courseid=42
moodle assignment submissions --param 'assignmentids=[17]'
moodle ws describe core_course_get_contents
```

Moodle CLI talks to the Moodle site you configure, preferring its official Web Service API and
falling back to read-only AJAX or HTML adapters when a site does not expose the required function.
It handles courses, participants, groups, assignments, calendars, grades, forums, completion,
academic workload, and files without pretending that every role or Moodle installation has the
same capabilities.

One binary serves people and programs. Human output stays readable; `--json` emits a stable
`schema_version: 1` envelope, commands expose JSON Schema, and failures have fixed exit codes. The
same use cases also back an MCP server whose write tools are absent unless explicitly enabled.

## What makes it different

### Sign in the way the site allows

Moodle CLI supports an existing token, Moodle username/password, login QR data, a browser session,
and Moodle's mobile launch flow. On Linux desktops, the mobile flow can open the normal browser and
receive the result through a per-user D-Bus URL handler. Institutional SSO, passkeys, and MFA remain
inside the browser where they belong. On macOS, an existing Safari session can be imported explicitly;
the automatic callback handler is still Linux-only. Windows currently uses the manual callback method.

Browser-session import is explicit: `moodle auth import-browser` looks for the requested site's
session in Safari on macOS, Firefox, or a Chromium-family profile. It never runs as a hidden side
effect of login. Safari's cookie store is undocumented and access may be denied by macOS; it is a
best-effort alternative to the manual callback, not a reason to install another browser. Firefox
session snapshots and Chromium's Linux fallback encryption are supported; Chromium cookies
whose keys are held by macOS Keychain, Windows DPAPI, a Linux secret service (`v11`), or app-bound
encryption (`v20`) are refused with an actionable error rather than worked around.

### Submit work, not merely upload it

Assignment submission follows Moodle's complete student workflow: upload files, save the
submission, submit it for grading when the assignment requires that extra step, then read the state
back from Moodle. A successful HTTP response is not treated as proof that the work was handed in.
If the response to a write is lost and the result cannot be reconciled, the command exits `12` with
an `ambiguous` outcome instead of guessing or sending the write again.

### A contract fit for automation

```sh
moodle version --json
moodle commands --json
moodle schema assignment.submit
moodle course list --json
moodle ws list --version v52 --effect write --json
moodle ws describe core_course_update_courses --json
moodle ws call core_course_get_contents --params-json '{"courseid":42}'
```

Every JSON response uses the same versioned envelope. Stable error classes map to distinct process
exit codes: success `0`, internal `1`, usage `2`, configuration `3`, authentication `4`, permission
`5`, not found `6`, validation `7`, conflict `8`, unavailable `9`, network `10`, upstream `11`, and
ambiguous write outcome `12`. Programs branch on structured codes, never English messages.

The typed `ws` registry is generated from disposable installations of Moodle 4.5.12, 5.1.7, and
5.2.3. It currently describes a 780-function union (759/761/755 functions respectively), retaining
each version's parameter and return JSON Schema, transport, effect, capability, deprecation, and
external-dependency metadata. A token's actual service exposure is checked again at runtime.

### Roles are capabilities, not personas

The CLI is not student-only. Moodle decides what an account may do from its system, category,
course, activity, group, and override context. The same binary therefore serves site
administrators, managers, course creators, editing and non-editing teachers, students, ordinary
authenticated users, and custom roles. Commands do not infer authority from a role name: they
inspect the functions exposed to the active credential and let Moodle enforce the relevant
capability in its real context.

Read commands remain useful across roles. Reviewed high-level write commands are identified as
writes in `moodle commands --json`, require `--yes` (or support `--dry-run`), never retry a generic
write, and honor the process-wide `--read-only` guard. Third-party plugin functions remain
available through the explicitly untyped `api call` escape hatch.

### Deliberately conservative writes

- Official Web Service functions are classified by a reviewed safety registry. Unknown functions
  are treated as writes and are never retried automatically.
- `--dry-run` validates typed and high-level writes and describes the plan without sending it.
- `--read-only` removes write commands from the command tree.
- `moodle mcp serve` is read-only by default; `--allow-write` must be explicit.
- HTML fallback is read-only. If Moodle cannot prove a submission's semantics, the CLI refuses it.

## Getting started

Once a stable version appears on the [Releases page](https://github.com/KoukeNeko/Moodle-CLI/releases),
install it using the package manager for your platform. Until then, use the source build below:

```sh
# macOS or Linux (Homebrew)
brew tap KoukeNeko/tap
brew install koukeneko/tap/moodle-cli

# Windows (Scoop)
scoop bucket add koukeneko https://github.com/KoukeNeko/scoop-bucket
scoop install koukeneko/moodle-cli

moodle version
```

The Releases page also provides direct Linux,
macOS, and Windows downloads for amd64 and arm64. Match your archive against `checksums.txt`
before running it. Homebrew and Scoop entries are updated after a stable release passes its
macOS signature and notarization checks; a newly published tag may take a few minutes to appear
in the package repositories. See [installation details](https://github.com/KoukeNeko/Moodle-CLI/wiki/Installation).

To build the current source instead, use Go 1.26 or newer:

```sh
git clone https://github.com/KoukeNeko/Moodle-CLI.git
cd Moodle-CLI
make build
./bin/moodle version
```

Install it to `~/.local/bin`:

```sh
make install
```

Register a site and inspect its available login methods:

```sh
moodle site add school https://moodle.example.edu
moodle auth methods --site school
```

For a Linux desktop, install the callback handler once and let the normal browser complete SSO:

```sh
moodle auth register-handler
moodle auth login --site school --method mobilelaunch
```

For every platform, the manual callback flow works without a registered desktop handler:

```sh
moodle auth login --site school --method manual
```

Other explicit methods include:

```sh
printf '%s' "$TOKEN" | moodle auth login --site school --token-stdin
printf '%s' "$PASSWORD" | moodle auth login --site school \
  --method password --username student --password-stdin
moodle auth import-browser --site school --list-profiles
moodle auth import-browser --site school --store
# On macOS, use the Safari session you already signed in with:
moodle auth import-browser --site school --browser safari --store
```

Credentials are kept in the operating-system keychain. Configuration stores site and account
metadata, never tokens or browser sessions. For an ephemeral CI run, use `MOODLE_WS_TOKEN` or
`MOODLE_SESSION` instead of persisting a credential.

## Everyday commands

```sh
moodle doctor                         # explain what this account can do
moodle course list                    # enrolled courses
moodle calendar upcoming              # overdue and upcoming work
moodle grade overview                 # course totals
moodle grade list --course 2          # one course's gradebook

moodle assignment list
moodle assignment show 42             # IDs and pasted Moodle URLs both work
moodle assignment status 42
moodle assignment submit 42 report.pdf --dry-run
moodle assignment submit 42 report.pdf --yes
moodle assignment submit 42 report.pdf --draft --yes

moodle forum list
moodle forum discussions 7
moodle forum read 19
moodle file download 'https://moodle.example.edu/pluginfile.php/...'
moodle resolve 'https://moodle.example.edu/mod/assign/view.php?id=42'

moodle participant list --param courseid=2
moodle enrolment methods --param courseid=2
moodle enrolment add --params-json '{"enrolments":[{"roleid":5,"userid":7,"courseid":2}]}' --dry-run
moodle group create --params-json '{"groups":[{"courseid":2,"name":"Lab A"}]}' --dry-run
moodle assignment submissions --param 'assignmentids=[42]'
moodle workload validate --require-minimum

moodle ws list --version v52 --component mod_assign
moodle ws describe mod_assign_save_grade
moodle ws call core_enrol_get_users_courses --param userid=4
moodle api call local_example_function --params-json '{}'  # third-party/unregistered
moodle mcp serve                      # read-only tools
moodle mcp serve --allow-write        # opt in to write tools
```

Run `moodle <command> --help`, use the generated [complete command reference](https://github.com/KoukeNeko/Moodle-CLI/wiki/Command-Reference),
or inspect all 780 core functions in [feature coverage](https://github.com/KoukeNeko/Moodle-CLI/wiki/Feature-Coverage).

## Security boundary

This program reads credentials and connects to a server, so its boundary should be explicit:

- It contacts only the Moodle site configured for the active profile.
- It accesses only credentials stored under Moodle CLI's own keychain entries.
- Browser import reads the selected browser profile only after an explicit command, returns or
  stores only the named site's cookie, and never logs it.
- `auth logout` removes the local credential but does not revoke the Moodle token, because the same
  token may also belong to Moodle's mobile app.
- It has no telemetry, updater, privilege escalation, process injection, or credential export.

See the [security model](https://github.com/KoukeNeko/Moodle-CLI/wiki/Security-Model) for the full
threat boundary and callback design.

## Compatibility

The full Docker-backed suite is verified against exactly these versions:

| Moodle | Verified version | Scenario |
| --- | --- | --- |
| 4.5 LTS | 4.5.12 | standard site, restricted Web Service site, ten academic years |
| 5.1 | 5.1.7 | standard site, restricted Web Service site, ten academic years |
| 5.2 | 5.2.3 | standard site, restricted Web Service site, ten academic years |

The small decade scenario creates 10 yearly cohorts, 30 students, 20 assignments, 60 submissions, eight
archived courses, two active courses, calendar events, missing grades, zero grades, drafts, submitted
work, overdue work, and cross-year convergence. It is intended to catch assumptions that only work
on a fresh demonstration site.

The separate PostgreSQL scale profile creates 50,000 synthetic students, 1,000 academic course
instances across 20 terms, a 50,000-participant orientation course, and 2.37 million enrolment
facts. SQL control-plane assertions independently verify 21 undergraduate credits and 6 graduate
credits per term; the CLI then reads representative workloads and all 50 participant pages while
recording latency, peak RSS, HTTP requests, and disk use. The scale container has no public egress
after its image bootstrap completes. Moodle 5.2.3 requires PostgreSQL 16, so the profile uses that
minimum instead of bypassing Moodle's environment check.

Release builds target Linux, macOS, and Windows on amd64 and arm64. Linux is the only platform with
the automatic browser callback handler today; the CLI and manual authentication paths build and test
on all three operating systems.

## Development

```sh
make test
make test-race
make lint
make verify

make moodle-up V=v52
make moodle-decade V=v52
make moodle-matrix V=v52
make moodle-scale V=v52
make moodle-down V=v52
```

The package boundaries are enforced by architecture tests. Feature packages own their interfaces;
the `moodle` and `webread` packages are adapters; `bootstrap` is only the composition root; CLI and
MCP never reach into HTTP directly. See [docs/architecture.md](docs/architecture.md) and the
[development handbook](https://github.com/KoukeNeko/Moodle-CLI/wiki/Development-and-Testing).

## Project status

The project is under active development. The release workflow is configured to produce checksummed archives and SBOMs, sign
and notarize both macOS binaries with Developer ID, verify them on a real macOS runner before
publication, sign the checksums with a keyless workflow signature, and update the public
[Homebrew tap](https://github.com/KoukeNeko/homebrew-tap) and
[Scoop bucket](https://github.com/KoukeNeko/scoop-bucket). Check the
[release history](https://github.com/KoukeNeko/Moodle-CLI/releases) for the latest published
version. Windows Authenticode signing is not configured, so Windows may display a SmartScreen warning.

<p>
  <a href="https://github.com/KoukeNeko/Moodle-CLI/actions/workflows/ci.yml"><img alt="CI status" src="https://img.shields.io/github/actions/workflow/status/KoukeNeko/Moodle-CLI/ci.yml?branch=main&style=for-the-badge&logo=githubactions&logoColor=white&label=CI"></a>
  <a href="https://go.dev/"><img alt="Go 1.26 or newer" src="https://img.shields.io/badge/GO-1.26%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
  <a href="LICENSE"><img alt="MIT license" src="https://img.shields.io/badge/LICENSE-MIT-4CAF50?style=for-the-badge&logo=github"></a>
</p>

## License and trademarks

[MIT](LICENSE) © 2026 KoukeNeko.

Moodle CLI is an independent project. It is not affiliated with, endorsed by, or sponsored by Moodle
or Moodle HQ. “Moodle” is a trademark of Moodle Pty Ltd. No Moodle source code is included.
