# Development and testing

[繁體中文](Development-and-Testing-zh-TW) · [Home](Home)

## Local verification

```sh
make test          # unit, contract, and architecture tests
make test-race     # the same packages under the race detector
make lint          # gofmt check and go vet
make verify        # all of the above, then a production-style build
```

Tests use `-count=1` because the architecture suite reads package sources at runtime, an input Go's
test cache cannot infer.

## Docker Moodle lab

Supported selectors are `v45`, `v51`, and `v52`:

```sh
make moodle-up V=v52
make moodle-decade V=v52
make moodle-down V=v52
make moodle-purge V=v52
```

Each version provisions a normal site and a restricted variant with Mobile Web Services disabled.
The full end-to-end suite exercises 250+ command invocations, expected failures, contract envelopes,
read fallbacks, identity separation, assignment submission, download safety, and semantic assertions.

`make moodle-decade` is the aging test. Its deterministic fixture contains:

- 10 academic-year cohorts and courses;
- 30 student identities;
- 20 assignments and 60 submissions;
- eight archived and two active courses;
- submitted work, drafts, overdue work, missing grades, and a real zero grade;
- calendar events and records converging across years.

The suite asserts exact counts and a 3,287-day data span, so silently dropping archived records or
confusing zero with missing cannot pass.

## Architecture

This is a modular monolith. Feature packages define the backend interfaces they consume. `moodle`
and `webread` implement adapters; `authmethod/*` implements login strategies; `bootstrap` wires the
application; CLI and MCP call feature use cases and do not perform HTTP.

Architecture tests enforce import direction. New behavior should be added to the smallest owning
feature instead of creating a shared god package. Read [docs/architecture.md in the repository](https://github.com/KoukeNeko/Moodle-CLI/blob/main/docs/architecture.md)
and the ADRs before changing package boundaries.

## CI and release

GitHub Actions runs tests on the supported Go line and the next line, lint and coverage, builds on
Linux/macOS/Windows, validates the JSON contract from the compiled binary, runs the ten-year Docker
scenario and exposed read-function role matrix against all verified Moodle versions, and builds a
snapshot release package. Push and pull-request runs do not receive release-signing secrets.

Before a `v*` tag can invoke GoReleaser, its own `make verify` and three-version Moodle smoke/role
matrix gates must pass. Apple signing secrets are only used by the subsequent release job; stable
tags also require `HOMEBREW_TAP_TOKEN` before a draft is created. Archives contain the binary,
README, and license, with
checksums, SBOMs, and a keyless checksum signature. Both macOS binaries are Developer ID-signed and
submitted to Apple before they are archived. A separate macOS runner opens the downloadable archives,
verifies `codesign` and Gatekeeper acceptance, and only then publishes the draft. Stable releases then
update `KoukeNeko/homebrew-tap`. Windows Authenticode is not configured.

No public release exists yet, so the workflow is distribution-ready but its Apple credentials have
not been proven against a real tagged artifact. The first tag is the final end-to-end validation.
