# JSON contract

[繁體中文](JSON-Contract-zh-TW) · [Home](Home)

Add `--json` explicitly when output is consumed by a program. Piping does not silently change the
format.

```sh
moodle version --json
moodle course list --json
moodle assignment list --current --json --fields name,due_date --no-input
moodle commands --json
moodle schema assignment.submit
moodle schema assignment submit --json
```

For something that runs unattended — a script, a CI job, an agent — use `--json` with `--no-input`,
and `--fields` for only the fields you read. [Automation recipes](Automation-Recipes) has worked
examples.

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

## Partial answers

`meta.source` says which route answered: `ws` (a web service token), `ajax` (a browser session), or
`html` (the site's own pages). They do not see the same amount. When a route could not read a
field, `meta.partial` is `true` and `meta.missing` names the field, and its value is `null`. A
`null` whose name is *not* in `meta.missing` is an answer: an assignment with no due date, a quiz
with no closing time. Check `meta.missing` before reading a `null` as "none".

## Field selection

`--fields` keeps only the named top-level fields of `data` — of each row in a list, or of the one
object. It works only with `--json`. The envelope and `meta` stay whole. A name the response does
not have is refused with exit 2, and the error's `hint` lists the names it does have; the list comes
from the published schema, so a typo is caught even when the list is empty.

```sh
moodle quiz list --current --json --fields name,closes_at
```

A narrowed response is a projection: it no longer carries every field the kind's schema requires.

## Never waiting for input

`--no-input` guarantees that nothing stops to ask. Where a command would prompt — a pasted
callback, a hidden session cookie, a password, a confirmation before submitting — it fails with
exit 2 instead, and the hint names the flag that supplies the answer (`--callback`, `--stdin`,
`--password-stdin`, `--yes`). Without a terminal the prompts already refuse; `--no-input` makes the
same true when one is attached.

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
| 130 | `network`, `reason: interrupted` | the caller stopped the command |

Ambiguous outcome takes precedence over the underlying error class. Do not automatically retry exit
12. Read the affected Moodle object and decide from its current state.

Exit `130` follows the shell convention of 128 plus the signal number rather than taking a place in
the table, because stopping a command is a decision rather than a way it failed. Its `code` stays
`network` — the closed set has no value for it — and `reason` is `interrupted`. An interrupt that cut
off a **write** after the request was sent reports `12` instead: Moodle does not undo a write because
the client stopped listening, so the state has to be read back.

## Schema and command discovery

`moodle schema` lists embedded response schemas. `moodle schema <kind>` prints one schema.
`moodle schema <command>` — for example `moodle schema assignment submit --json` — describes one
command:

```json
{
  "schema_version": 1,
  "kind": "command.schema",
  "data": {
    "command": "assignment submit",
    "kind": "assignment.submit",
    "safety": "write",
    "idempotency": "non_idempotent",
    "input_schema": {"...": "its flags by name, positional arguments as args"},
    "output_schema": {"...": "the assignment.submit schema"}
  },
  "meta": {"source": "local"}
}
```

`safety` is what running the command can change: `read` changes nothing, `local` changes only this
machine (configuration, the keychain, a downloaded file), and `write` can change something on
Moodle. `idempotency` says whether a failed or unconfirmed run may simply be repeated; every write
is `non_idempotent`, because Moodle functions carry no idempotency key. An agent runs `read` freely,
weighs `local`, and needs a decision for `write`.

`moodle commands --json` gives the same `safety` and `idempotency` for every command at once,
alongside `mutates` (whether `--read-only` withholds the command). This makes the binary itself the
discovery source for shells and agents; nothing has to be scraped from `--help`.

A single word that names a published kind is read as the kind. The one-word commands that share a
kind's name — `version`, `doctor`, `resolve`, `commands` — are all `read`; `moodle commands --json`
describes them.

## Global flags

| Flag | Purpose |
| --- | --- |
| `--json` | Emit the versioned JSON contract |
| `--fields` | Keep only these data fields (requires `--json`) |
| `--no-input` | Never prompt; fail where a command would wait for an answer |
| `--pretty` | Indent the JSON |
| `--verbose` / `-v` | Print redacted HTTP requests and responses to stderr |
| `--read-only` | Refuse every call that can change anything on the site; also `MOODLE_CLI_READ_ONLY` |
| `--backend` | Which routes may answer: `auto` or `ws-only` |

Changing the meaning or shape of contract v1 requires a new schema version. Human wording and
diagnostic details are not stable API.
