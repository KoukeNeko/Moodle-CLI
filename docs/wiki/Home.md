# Moodle CLI handbook

Moodle CLI is an independent command-line client for learners, educators, administrators, scripts,
and agents. It prefers
Moodle's official Web Service API, uses read-only fallbacks when a site exposes fewer functions, and
verifies the final state of assignment submissions.

[繁體中文](Home-zh-TW)

## Start here

1. [Install a release or build from source](Installation).
2. Add a site and choose an [authentication method](Authentication-and-Sites).
3. Explore the [command guide](Commands).
4. For automation, read the [JSON contract](JSON-Contract).

## Design promises

- Configuration contains site and account metadata; credentials live in the operating-system
  keychain.
- The CLI checks capabilities reported by the active site instead of assuming them from a version
  number.
- HTML fallback is read-only. Unknown Web Service functions are treated as writes.
- A write with an unknown outcome is reported as ambiguous and is never blindly retried.
- Human output and the versioned JSON contract are explicit, separate interfaces.

## Verified environment

The Docker-backed integration suite currently verifies Moodle 4.5.12, 5.1.7, and 5.2.3. Its
long-lived fixture represents ten academic years rather than a pristine demonstration site. See
[Development and testing](Development-and-Testing) for exact commands and coverage.

Moodle CLI is not affiliated with Moodle or Moodle HQ. “Moodle” is a trademark of Moodle Pty Ltd.
