# Verification report contract

[繁體中文](Report-Contract-zh-TW) · [Home](Home)

The GitHub Pages dashboard separates inventory from execution evidence. It never converts an absent
artifact into a pass.

## Views

- **Overview** — supported versions, registry union, scale truth, and executed role-cell count.
- **Function coverage** — version, component, effect, transport, recipe status, and registry result.
- **Role matrix** — role/domain counts for passed, expected denied, expected unavailable, failed, and not run.
- **Scale** — truth counts, credit invariants, latency, RSS, REST calls, and disk observations.
- **Runs** — commit, run, generation time, runner identity/image, and result status.

Filters are available for version, component, effect, and result. `registry-covered`, `passed`,
`partial`, and `not-run` are deliberately distinct states.
The `recipe` column says `not-implemented` until an executable fixture binding and assertion exists;
the generated parameter schema alone is not a test recipe.

## Role matrix JSONL

`test/reports/role-matrix.jsonl` contains one executed CLI cell per line. Every object must contain
`version`, `role`, `function`, and `outcome`; `domain` is optional and otherwise derives from the
function namespace. The only valid outcomes are `passed`, `expected_denied`,
`expected_unavailable`, and `failed`. `skip` is invalid and makes the report build fail.

When the artifact is absent, the dashboard emits explicit `not-run` placeholders. When it is
present, rows are aggregated by version, runtime role, and domain, while every unexecuted
role/function cell remains in an explicit `not_run` denominator. A partial artifact therefore cannot
make a whole role look complete. Duplicate cells fail the report build. Matching function rows
change from `registry-covered` to `executed` or `failed`; expected denial and unavailability remain
separate authorization results rather than being relabelled as passes.

`test/reports/runtime-roles-<version>.json` supplies the denominator before function cells exist. If
present, dashboard placeholders use the principals discovered from that Moodle runtime, including
custom/plugin roles, instead of the standard-role fallback. Executed cells must name a principal in
the matching inventory. A malformed inventory, duplicate role, count mismatch, or missing site
administrator fails the report build rather than silently reverting to the fallback.

## Retention and redaction

Pages currently publishes the latest synthetic snapshot. Historical trend accumulation is a pending
milestone and must not be inferred from the single-row Runs view. Full JSONL, JUnit, transcript,
and container diagnostics are Actions artifacts retained for 90 days. Neither surface may contain
tokens, cookies, passwords, callback URLs, authorization headers, or unredacted request secrets.

The report builder reads generated registries and test artifacts, publishes exact source lineage,
and still builds after a test failure so failed evidence remains inspectable. `make report-site`
rebuilds the same static dashboard locally. `make report-data-test` verifies absent, observed, and
invalid/no-skip evidence behavior in CI.
