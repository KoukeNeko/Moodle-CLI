# Command guide

[繁體中文](Commands-zh-TW) · [Home](Home)

Use `moodle <command> --help` as the authoritative flag reference. IDs shown below can often be
replaced by a pasted Moodle URL.

## Sites and authentication

```sh
moodle site add <name> <url>
moodle site list
moodle site use <name>
moodle site remove <name>

moodle auth methods
moodle auth login [--method token|password|qr|mobilelaunch|browser-session|manual]
moodle auth status
moodle auth logout
moodle auth import-browser [--list-profiles|--store]
moodle auth register-handler         # Linux only
moodle auth handler-status
moodle auth unregister-handler
```

## Student reads

```sh
moodle doctor
moodle course list
moodle calendar upcoming
moodle grade overview
moodle grade list --course <id|url>
moodle assignment list
moodle assignment show <id|url>
moodle assignment status <id|url>
moodle forum list
moodle forum discussions <id|url>
moodle forum read <id|url>
moodle file download <url>
moodle resolve <url>
```

`doctor` explains the available backend and missing capability instead of assuming every site has
the same functions. Forum reads do not mark discussions as read.

## Assignment submission

```sh
moodle assignment submit <id|url> report.pdf --dry-run
moodle assignment submit <id|url> report.pdf --yes
moodle assignment submit <id|url> report.pdf --draft --yes
```

The non-draft path uploads, saves, performs the separate “submit for grading” step when required, and
reads the state back. `--draft` deliberately stops before handing work in. The JSON field `handed_in`
is the final truth reported by Moodle.

## API escape hatch

```sh
moodle api functions --match assign
moodle api call core_enrol_get_users_courses --param userid=4
```

The raw call still passes through the safety policy. Unreviewed functions are treated as writes and
are not retried. Use it to reach an official function that has no high-level command, not to bypass
the safety model.

## Automation and agents

```sh
moodle version --json
moodle commands --json
moodle schema
moodle schema assignment.submit
moodle mcp serve
moodle mcp serve --allow-write
moodle course list --read-only
```

`--read-only` removes write commands. MCP is read-only unless `--allow-write` is explicitly present.
See [JSON contract](JSON-Contract) before consuming output in a program.
