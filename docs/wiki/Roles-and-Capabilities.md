# Roles and capabilities

[繁體中文](Roles-and-Capabilities-zh-TW) · [Home](Home)

Moodle CLI is not a student-only client. It deliberately does not translate a role name into
authority. Moodle evaluates capabilities at system, category, course, activity, group, and override
contexts, so two users with the same role name may legitimately receive different results.

## Target identities for the runtime matrix

- Site administrator (an identity outside the role table)
- `manager`, `coursecreator`, `editingteacher`, `teacher`, `student`, `guest`, `user`, and `frontpage`
- Every runtime custom/plugin role
- A fixture role derived from an archetype
- A fixture role with no archetype

The generated registry and command scenarios exist today, but the exhaustive runtime
role-by-function harness has not emitted an artifact yet. The public dashboard therefore reports
these cells as `not-run`, never as passed or skipped.

Once that harness is complete, every matrix cell must execute through the CLI. Its only terminal
outcomes will be `passed`, `expected_denied`, `expected_unavailable`, and `failed`. A role that cannot
see a function must prove that boundary with an explicit denial or unavailable result.

`make moodle-decade V=<version>` now discovers roles from the running Moodle database and writes
`test/reports/runtime-roles-<version>.json`. The fixture includes every installed runtime role, a
distinct site administrator principal, declared context levels, credential kind, and the real context
assignment used by the test. It also installs two canaries: `matrixteacher`, which inherits the
`teacher` archetype, and `matrixblank`, which has no archetype. This proves that later matrix stages
cannot be implemented as a hard-coded list of the eight standard shortnames. This inventory is an
input to the full role/function harness; inventory alone is not reported as function execution.

`make moodle-role-preflight V=v52` then executes the harmless typed
`core_webservice_get_site_info` read through the CLI for every password principal. Guest executes an
explicit no-credential CLI path and must be unavailable. The redacted
`test/reports/role-preflight-<version>.jsonl` contains no token, password, username, or Moodle
response. It proves credential and transport readiness only; it is not counted as complete
role/function coverage.

## Three different gates

1. **Registry support** says a core function exists in one of the supported Moodle versions.
2. **Service exposure** says the active credential's external service offers the function.
3. **Capability evaluation** is Moodle's decision for the real object and context used by the call.

The CLI never treats registry presence as permission. `moodle ws describe` documents requirements;
`moodle site inspect` reports service exposure; the operation itself remains Moodle's authority.

## Write safety

High-level writes and typed `ws` writes are marked in `moodle commands --json`, support `--dry-run`,
require an explicit write opt-in, and obey global `--read-only`. Generic writes are not retried. A
lost response that cannot be reconciled returns exit `12` (`ambiguous`) instead of being repeated.

See [Complete command reference](Command-Reference) and [Feature coverage](Feature-Coverage).
