# Moodle CLI v0.1.3 — Safari session fallback

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
