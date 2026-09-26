# Automation recipes

[繁體中文](Automation-Recipes-zh-TW) · [Home](Home)

Working examples of driving Moodle from something other than a keyboard. Each one leans on the
[JSON contract](JSON-Contract): `--json` for a stable shape, `--fields` to keep the payload to what
is read, `--no-input` so nothing waits for a prompt, and exit codes to decide what happened. Add
`--read-only` (or set `MOODLE_CLI_READ_ONLY=1`) to anything that only reads: a mistake then cannot
reach the site.

## 1. This term's deadlines, in order

```sh
moodle assignment list --current --json --fields name,course_short_name,due_date --no-input --read-only |
  jq -r 'if (.meta.missing | index("due_date")) then error("this route cannot see due dates") else . end
         | .data[] | select(.due_date != null)
         | [.due_date, .course_short_name, .name] | @tsv' |
  sort
```

Two details worth copying. `--current` keeps courses that have started and not yet ended, so last
year's assignments stay out. And the check on `meta.missing` is what makes `select(.due_date != null)`
safe: on a route that could not read deadlines, a `null` means "not known", and filtering it out
would silently drop work that is due. When `due_date` is not in `meta.missing`, a `null` really is an
assignment without a deadline.

Dates are RFC 3339 in UTC; convert them for display, not for comparison.

## 2. Branch on the exit code, not the message

```sh
if out=$(moodle grade overview --json --no-input --read-only); then
  printf '%s\n' "$out" | jq -r '.data[] | "\(.course_id)\t\(.display)"'
else
  case $? in
    4) echo "signed out: sign in again" >&2 ;;
    5) echo "this account may not read grades" >&2 ;;
    10) echo "network trouble; safe to retry a read" >&2 ;;
    130) echo "stopped by the user" >&2; exit 130 ;;
    *) printf '%s\n' "$out" | jq -r '.error.message' >&2 ;;
  esac
  exit 1
fi
```

With `--json` the error envelope arrives on stdout too, so one stream carries whatever happened. The
full table is in [JSON contract](JSON-Contract#exit-codes). The code that matters most to anything
that writes is `12`: the write may already have happened, so read the object back before deciding.
`130` is the caller's own Ctrl-C — pass it on rather than reporting it as a failure.

## 3. Let an agent inspect, rehearse, then commit

An agent should not have to guess whether a command is safe to run unattended. Ask the binary:

```sh
moodle schema assignment submit --json --no-input | jq '.data | {safety, idempotency}'
```

```json
{"safety": "write", "idempotency": "non_idempotent"}
```

`read` commands are free to run, `local` ones change only this machine, and `write` needs a
decision. `non_idempotent` means a failed attempt must not simply be repeated. `moodle commands
--json` gives the same two fields for every command at once, and `schema` answers even under
`--read-only`, which hides the writes it withholds.

Rehearse before handing work in. `--dry-run` reads the assignment and your current submission and
lays out the steps a real run would take — including whether handing in is a separate step on this
assignment — and sends nothing. Submitting needs a web service token; a browser session can read
assignments but not hand work in.

```sh
moodle assignment submit 1436182 hw1.tar.gz --dry-run --json --no-input
```

Then commit, and read the state back if the outcome was not confirmed:

```sh
if moodle assignment submit 1436182 hw1.tar.gz --yes --json --no-input > result.json; then
  jq '.data' result.json
else
  case $? in
    12) moodle assignment status 1436182 --json --no-input ;;  # never resend blindly
    130) moodle assignment status 1436182 --json --no-input ;;  # Ctrl-C after sending is 12, not 130
    *)  jq -r '.error.message' result.json >&2; exit 1 ;;
  esac
fi
```

`assignment submit` reports what Moodle says afterwards, not what it sent: a submission that is
still a draft is reported as one.

## 4. Rules to give your agent

Put these in whatever your agent reads before it works — `AGENTS.md`, `CLAUDE.md`, a system prompt:

```text
When using the moodle CLI:
- Always pass --json --no-input. Add --read-only unless the task is to change something.
- Use --fields for only the fields you read; an unknown name is refused with the real ones.
- Before running a command you have not used, run `moodle schema <command> --json`.
  Run safety "read" freely; ask before safety "write".
- Before reading a null as "none", check meta.missing: a field listed there was not readable.
- Branch on exit codes and error.code, never on message text. Never retry exit 12:
  read the object back instead. Exit 130 is the user's Ctrl-C; stop, do not retry.
```

An agent that speaks the Model Context Protocol can use `moodle mcp serve` instead, which returns
structured results without the shell in between.
