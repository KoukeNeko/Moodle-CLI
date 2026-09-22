# JSON contract

[繁體中文](JSON-Contract-zh-TW) · [Home](Home)

Add `--json` explicitly when output is consumed by a program. Piping does not silently change the
format.

```sh
moodle version --json
moodle course list --json
moodle commands --json
moodle schema assignment.submit
```

## Envelope

Successful responses carry a stable top-level `schema_version`, `kind`, `data`, and `meta`:

```json
{
  "schema_version": 1,
  "kind": "version",
  "data": {},
  "meta": {"source": "local"}
}
```

Errors use `kind: "error"` and structured fields such as `code`, `reason`, `outcome`, and
`retryable`. Consumers must branch on these fields rather than matching the English `message`.
`reason` is intentionally open-ended; tolerate values added by later versions.

## Exit codes

| Exit | Error code | Meaning |
| ---: | --- | --- |
| 0 | — | success |
| 1 | `internal` | local invariant or unexpected internal failure |
| 2 | `usage` | invalid command or arguments |
| 3 | `configuration` | missing or invalid local configuration |
| 4 | `authentication` | rejected or expired credential |
| 5 | `permission_denied` | authenticated but not permitted |
| 6 | `not_found` | requested resource does not exist |
| 7 | `validation` | input or returned state failed validation |
| 8 | `conflict` | server state conflicts with the operation |
| 9 | `unavailable` | capability or local facility is unavailable |
| 10 | `network` | transport failure |
| 11 | `upstream` | Moodle returned an invalid or failed response |
| 12 | any code, `outcome: ambiguous` | a write may already have happened |

Ambiguous outcome takes precedence over the underlying error class. Do not automatically retry exit
12. Read the affected Moodle object and decide from its current state.

## Schema and command discovery

`moodle schema` lists embedded response schemas. `moodle schema <kind>` prints one schema.
`moodle commands --json` describes the current command tree, including whether a command mutates
state. This makes the binary itself the discovery source for shells and agents.

Changing the meaning or shape of contract v1 requires a new schema version. Human wording and
diagnostic details are not stable API.
