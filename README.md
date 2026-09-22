<h1 align="center">Moodle CLI</h1>

<p align="center">
  <strong>An independent Moodle command-line client for students.</strong><br>
  Readable terminal output for people, and a versioned contract for scripts, CI, and agents.
</p>

<p align="center">
  <a href="https://github.com/KoukeNeko/Moodle-CLI/actions/workflows/ci.yml"><img alt="CI status" src="https://img.shields.io/github/actions/workflow/status/KoukeNeko/Moodle-CLI/ci.yml?branch=main&style=for-the-badge&logo=githubactions&logoColor=white&label=CI"></a>
  <a href="#compatibility"><img alt="Verified with Moodle 4.5.12, 5.1.7, and 5.2.3" src="https://img.shields.io/badge/MOODLE-4.5.12%20%7C%205.1.7%20%7C%205.2.3-FF8B00?style=for-the-badge&logo=moodle&logoColor=white"></a>
  <a href="https://go.dev/"><img alt="Go 1.26 or newer" src="https://img.shields.io/badge/GO-1.26%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
  <a href="LICENSE"><img alt="MIT license" src="https://img.shields.io/badge/LICENSE-MIT-4CAF50?style=for-the-badge&logo=github"></a>
</p>

<p align="center">
  <strong>English</strong> · <a href="README.zh-TW.md">繁體中文</a>
</p>

<p align="center">
  <a href="#getting-started">Getting started</a>
  · <a href="https://github.com/KoukeNeko/Moodle-CLI/wiki">Handbook</a>
  · <a href="test/e2e/README.md">Test lab</a>
  · <a href="docs/architecture.md">Architecture</a>
</p>

```sh
moodle course list
moodle calendar upcoming
moodle assignment show 42
moodle assignment submit 42 report.pdf --dry-run
```

Moodle CLI talks to the Moodle site you configure, preferring its official Web Service API and
falling back to read-only AJAX or HTML adapters when a site does not expose the required function.
It handles courses, assignments, calendars, grades, forums, and files without pretending that every
Moodle installation has the same capabilities.

One binary serves people and programs. Human output stays readable; `--json` emits a stable
`schema_version: 1` envelope, commands expose JSON Schema, and failures have fixed exit codes. The
same use cases also back an MCP server whose write tools are absent unless explicitly enabled.

## What makes it different

### Sign in the way the site allows

Moodle CLI supports an existing token, Moodle username/password, login QR data, a browser session,
and Moodle's mobile launch flow. On Linux desktops, the mobile flow can open the normal browser and
receive the result through a per-user D-Bus URL handler. Institutional SSO, passkeys, and MFA remain
inside the browser where they belong. macOS and Windows currently use the manual callback method.

Browser-session import is explicit: `moodle auth import-browser` looks for the requested site's
session in Firefox or a Chromium-family profile. It never runs as a hidden side effect of login.

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
```

Every JSON response uses the same versioned envelope. Stable error classes map to distinct process
exit codes: success `0`, internal `1`, usage `2`, configuration `3`, authentication `4`, permission
`5`, not found `6`, validation `7`, conflict `8`, unavailable `9`, network `10`, upstream `11`, and
ambiguous write outcome `12`. Programs branch on structured codes, never English messages.

### Deliberately conservative writes

- Official Web Service functions are classified by a reviewed safety registry. Unknown functions
  are treated as writes and are never retried automatically.
- `--dry-run` makes assignment submission resolve and describe its plan without sending a write.
- `--read-only` removes write commands from the command tree.
- `moodle mcp serve` is read-only by default; `--allow-write` must be explicit.
- HTML fallback is read-only. If Moodle cannot prove a submission's semantics, the CLI refuses it.

## Getting started

No release has been published yet. Build the current source with Go 1.26 or newer:

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

moodle api functions --match assign
moodle api call core_enrol_get_users_courses --param userid=4
moodle mcp serve                      # read-only tools
moodle mcp serve --allow-write        # opt in to write tools
```

Run `moodle <command> --help`, or use the [command handbook](https://github.com/KoukeNeko/Moodle-CLI/wiki/Commands)
for the complete surface.

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

The decade scenario creates 10 yearly cohorts, 30 students, 20 assignments, 60 submissions, eight
archived courses, two active courses, calendar events, missing grades, zero grades, drafts, submitted
work, overdue work, and cross-year convergence. It is intended to catch assumptions that only work
on a fresh demonstration site.

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
make moodle-down V=v52
```

The package boundaries are enforced by architecture tests. Feature packages own their interfaces;
the `moodle` and `webread` packages are adapters; `bootstrap` is only the composition root; CLI and
MCP never reach into HTTP directly. See [docs/architecture.md](docs/architecture.md) and the
[development handbook](https://github.com/KoukeNeko/Moodle-CLI/wiki/Development-and-Testing).

## Project status

The project is under active development and has not made its first public release. Source builds and
the tested command surface above are usable; packaging, code signing, and notarization are not yet
available. Release archives, when introduced, will be produced by the tagged GitHub Actions workflow
with checksums, SBOMs, and keyless signatures.

## License and trademarks

[MIT](LICENSE) © 2026 KoukeNeko.

Moodle CLI is an independent project. It is not affiliated with, endorsed by, or sponsored by Moodle
or Moodle HQ. “Moodle” is a trademark of Moodle Pty Ltd. No Moodle source code is included.
