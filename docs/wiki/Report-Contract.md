# Verification report contract

[繁體中文](Report-Contract-zh-TW) · [Home](Home)

The GitHub Pages dashboard separates inventory from execution evidence. It never converts an absent
artifact into a pass.

## Views

- **Overview** — supported versions, registry union, scale truth, and executed role-cell count.
- **Function coverage** — version, component, effect, transport, recipe status, and registry result.
- **Role matrix** — role/domain counts for passed, expected denied, expected unavailable, and failed.
- **Scale** — truth counts, credit invariants, latency, RSS, REST calls, and disk observations.
- **Runs** — commit, run, generation time, runner identity/image, and result status.

Filters are available for version, component, effect, and result. `registry-covered`, `passed`,
`partial`, and `not-run` are deliberately distinct states.

## Retention and redaction

Pages currently publishes the latest synthetic snapshot. Historical trend accumulation is a pending
milestone and must not be inferred from the single-row Runs view. Full JSONL, JUnit, transcript,
and container diagnostics are Actions artifacts retained for 90 days. Neither surface may contain
tokens, cookies, passwords, callback URLs, authorization headers, or unredacted request secrets.

The report builder reads generated registries and test artifacts, publishes exact source lineage,
and still builds after a test failure so failed evidence remains inspectable. `make report-site`
rebuilds the same static dashboard locally.
