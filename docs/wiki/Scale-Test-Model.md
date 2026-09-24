# Ten-year scale test model

[繁體中文](Scale-Test-Model-zh-TW) · [Home](Home)

The scale profile is a separate PostgreSQL 16 environment. Moodle 5.2.3 rejects PostgreSQL 15, so
the fixture follows Moodle's real minimum instead of bypassing its environment check.

## Deterministic truth

| Measure | Value |
| --- | ---: |
| Academic years / terms | 10 / 20 |
| Students | 50,000 (40,000 undergraduate; 10,000 graduate) |
| Academic course instances | 1,000 (50 per term) |
| Academic enrolments | 2,320,000 |
| Orientation enrolments | 50,000 |
| Total enrolment facts | 2,370,000 |
| Undergraduate load | 7 × 3 credits = 21 per term, for 8 terms |
| Graduate load | 2 × 3 credits = 6 per term, for 4 terms |

A zero-credit orientation course contains all 50,000 students and forces participant pagination.
Course custom fields hold credits, academic level, and term. The fixture also preserves repeated
names across years, repeats, suspension, archives, groups, availability, zero grades, missing grades,
drafts, overdue work, cross-role membership, and permission overrides.

## Independence and isolation

The seed is deterministic and convergent. The control plane queries Moodle's PostgreSQL tables
directly; the CLI is the system under test and does not share the control-plane counting logic.
Destructive function recipes must use disposable small fixtures; the full recipe harness is still
pending and no destructive matrix runs on the large site. After bootstrap,
the scale Moodle container is disconnected from public egress.

## Gates

- Seed ≤ 180 minutes; complete job ≤ 240 minutes
- One CLI operation ≤ 120 seconds; peak RSS ≤ 1 GiB
- Generated database plus Moodle data ≤ 8 GiB
- Record p50, p95, maximum latency, RSS, REST request count, and disk use
- Record runner identity/image so results from different hardware are never compared accidentally

The current harness enforces the absolute limits above and a linear REST-request budget. The planned
25% regression gate against the last five comparable successful runs is not yet enforced; the
dashboard must not present a historical baseline until that aggregation exists.

Run it with `make moodle-scale V=v52`. Public results contain synthetic summaries only; raw redacted
evidence is retained as a 90-day Actions artifact.
