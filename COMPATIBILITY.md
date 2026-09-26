# Compatibility matrix

***English** · [繁體中文](COMPATIBILITY.zh-TW.md)*

Updated: 2026-09-26

Every "Verified" row below is backed by something that runs: a CI job, a test in this repository, or
a measurement recorded against a real site. Anything else says so.

## Moodle server

| Server | Status | Evidence and limits |
| --- | --- | --- |
| Moodle 4.5.12 LTS | Verified | Docker E2E on every release: a standard site, a site with Web Services restricted, and a ten-year fixture. `make moodle-up V=v45`. |
| Moodle 5.1.7 | Verified | Same suite, `V=v51`. |
| Moodle 5.2.3 | Verified | Same suite, `V=v52`. The PostgreSQL scale profile runs here (50,000 students, 1,000 courses). |
| Other Moodle 4.5.x, 5.1.x, 5.2.x | Expected compatible | Same API family. Run `moodle doctor` first: the CLI checks the functions a site actually exposes rather than trusting a version number. |
| Moodle 4.4 and earlier | Unverified | The typed `ws` registry has no snapshot for them, so `moodle ws` refuses and names `moodle api call` instead. The plain read commands may work. |
| Moodle 5.0 | Unverified | Between two verified releases and likely fine; nothing measures it. |

The typed registry is a 780-function union generated from disposable installations of the three
verified versions (759/761/755 respectively), keeping each version's own parameter and return schema.

## What a site's configuration decides

A Moodle can be configured so that a feature is unreachable, whatever its version. `moodle doctor`
reports which route answers for each feature on the site you are signed in to.

| Site state | Effect | Verified |
| --- | --- | --- |
| Mobile web services on, token issued | Every feature, including handing work in | Yes, all three versions |
| Mobile web services off, browser session only | Reads only. Courses over AJAX; assignments, quizzes, grades and forums from the site's own pages. `meta.missing` names every field the route cannot see. Submitting refuses rather than guessing. | Yes: the `v45`/`v51`/`v52` restricted-site scenarios, and reads measured against a real university site (Moodle 4.5, SSO, no token) on 2026-09-25 |
| A course that hides its grade report | `grade list` reports `permission_denied` with Moodle's own reason | Yes, measured on a real site |
| An activity linked from a side block | Not counted as the course's own | Yes |

## Operating systems and architectures

Release archives are pure Go (`CGO_ENABLED=0`) for:

- macOS `amd64`, `arm64`
- Linux `amd64`, `arm64` — also `.deb`, `.rpm` and `.apk`
- Windows `amd64`, `arm64`

| Platform | Status | Limits |
| --- | --- | --- |
| Linux | Builds, unit tests and the full Docker suite | The only platform with the automatic browser callback handler. |
| macOS | Builds and unit tests in CI | No automatic callback: sign in with `auth import-browser`, `auth import-session`, or `--method manual`/`qr`. Binaries carry a Developer ID signature and Apple notarization, checked against the downloadable archive on a real macOS host before a release is published. |
| Windows | Builds and unit tests in CI | No automatic callback. Chrome, Edge and Brave v20 cookie stores cannot be read without cgo, so browser import is unavailable there; use `qr` or `manual`. **Not Authenticode-signed**, so SmartScreen may warn. |

The Docker-backed Moodle suite runs on Linux only, so the site-facing behaviour above is measured
there. macOS and Windows compile the same code and run the same unit and contract tests.

The build is written to be reproducible — the build date and file timestamps come from the commit,
not from release time, and `-trimpath` keeps the builder's paths out — but nothing in CI yet rebuilds
a tag and compares digests, so reproducibility is a property of the configuration rather than a
measured fact. macOS archives cannot be reproducible in any case: notarization needs a secure
timestamp.

## Credential storage

| Where | Status | Notes |
| --- | --- | --- |
| macOS Keychain | Used by default; not exercised in CI | A hosted runner has no unlocked keychain, so the round trip is covered by an in-memory store and by manual checks, not automatically. |
| Windows Credential Manager | Used by default; not exercised in CI | As above. |
| Linux Secret Service (GNOME Keyring, KWallet) | Used by default; not exercised in CI | Verified by hand against GNOME Keyring on 2026-09-26. |
| A file, opt-in | Verified on POSIX | `--credential-store file` or `preferences.credential_store`, for a machine with no keychain: headless Linux, SSH, WSL, a container. Never automatic. Created `0600` in a `0700` directory on Linux and macOS. **Windows has no mode bits** — Go maps all but the read-only attribute to nothing — so there the file is protected by the ACL `%APPDATA%` already carries, and Credential Manager is the default store anyway. |
| `MOODLE_WS_TOKEN` / `MOODLE_SESSION` | Verified | One process, nothing written to disk. |

## Authentication

| Method | Status | Notes |
| --- | --- | --- |
| An existing web service token | Verified | `auth login --token-stdin`. |
| Moodle username and password | Verified | Only where the site shows its own login form. |
| QR login code | Unverified against a real site | Implemented and unit-tested. Moodle requires HTTPS for it and the Docker test sites are HTTP, so it is untested end to end. The only request that sends a `MoodleMobile` User-Agent. |
| Browser SSO with automatic callback | Unverified end to end | The handler registration and callback transaction are tested on Linux; the full exchange needs an HTTPS site with an identity provider, which nothing here has. ADR-0004 tracks it. |
| Manual callback paste | Verified | Every platform. |
| Importing a browser session (Firefox, Safari, Chromium) | Parsers verified against captured stores | Windows Chrome/Edge/Brave v20 stores are unreadable; Safari needs Full Disk Access, and `auth import-session` is the route when the on-disk cookie is absent. |
| Pasting a session cookie | Verified against a real site | `auth import-session`, hidden input or `--stdin`. Measured on Moodle 4.5 behind SSO, 2026-09-25. |

## Interfaces

| Interface | Status | Notes |
| --- | --- | --- |
| JSON contract v1 | Verified | Every response kind has an embedded JSON Schema, asserted in tests. Exit codes are fixed. |
| `moodle schema <command>` | Verified | `safety` and `idempotency` per command, for unattended callers. |
| MCP server | Verified | `moodle mcp serve`, read-only unless `--allow-write`. It has no quiz tools yet. |
| Shell completion | Verified by tests in this repository | bash, zsh, fish, PowerShell, through `moodle shell-completion`. The installers' own round trip runs in a workflow triggered when they change. |

Moodle CLI is not affiliated with Moodle or Moodle HQ. "Moodle" is a trademark of Moodle Pty Ltd.
