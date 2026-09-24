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

Until the full matrix is emitted, each `role-preflight-<version>.jsonl` contributes its one executed
`core_webservice_get_site_info` CLI cell per principal. The dashboard labels its recipe
`credential-preflight` and keeps all other function cells in the `not_run` denominator. Malformed
preflight outcomes or exits fail the report build.

The separate `role-matrix-service-<version>.jsonl` fragments contain CLI-confirmed service
unavailability for password principals. The builder merges them with preflight cells, rejects
duplicate role/function pairs, and keeps every remaining cell in `not_run`.

`role-matrix-read-<version>.jsonl` contains executed, curated no-argument reads for the same
password principals. Before counting a result, the builder checks the function against the
versioned registry and the checked-in recipe manifest. Only a matching CLI exit and one of
`passed`, `expected_denied`, or `expected_unavailable` is accepted; response bodies and credentials
are excluded. Guest, other reads, and writes remain in `not_run` until their own recipes execute.

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

Pushes build the dashboard UI without replacing published long-haul evidence. Scheduled long-haul
runs and explicit publish dispatches deploy the dashboard after downloading their own redacted
`test/reports/` artifacts. The report build rejects artifacts extracted under a duplicate
`test/reports/reports/` directory.

To republish the latest report code against an existing long-haul result, dispatch **Verification
dashboard** with that run's numeric `evidence_run_id`. It downloads the run's redacted artifacts,
shows the test commit separately from the report commit, and does not repeat the 50k seed. The
`runner` column refers to the scale runner when recorded in its summary; older summaries explicitly
say when that runner name was not captured.

An optional `supplemental_run_id` lets the same rebuild add a later role-matrix recipe artifact
without rerunning the scale seed. The supplemental artifact is extracted separately, and its run ID
and commit appear in the Runs view. For each Moodle version, a missing primary runtime-role
inventory or credential preflight is filled from the supplemental run; a version present in both
runs keeps the primary evidence, and conflicting runtime principals fail the build. Recipe
fragments from both runs are combined with duplicate cells rejected. The builder also rejects
missing or malformed recipe evidence.

The report builder reads generated registries and test artifacts, publishes exact source lineage,
and still builds after a test failure so failed evidence remains inspectable. `make report-site`
rebuilds the same static dashboard locally. `make report-data-test` verifies absent, observed, and
invalid/no-skip evidence behavior in CI.
