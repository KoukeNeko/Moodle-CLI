#!/usr/bin/env bash
# Run the small Moodle decade fixture in CI and long-haul. All three SQLite
# versions have intermittently failed a first fixture write; the precise
# database cause is not yet confirmed. Retry only that exact failure once
# after discarding disposable volumes.
set -euo pipefail

VERSION="${1:-v52}"
REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$REPO_DIR"

case "$VERSION" in
  v45 | v51 | v52) ;;
  *) echo "unknown version: $VERSION" >&2; exit 2 ;;
esac

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT
mkdir -p test/e2e/artifacts

for ATTEMPT in 1 2; do
  LOG="$WORKDIR/decade-$ATTEMPT.log"
  if make moodle-decade V="$VERSION" > "$LOG" 2>&1; then
    # Keep routine GitHub logs compact; the emitted fixture and redacted
    # report artifacts contain the detailed evidence.
    tail -35 "$LOG"
    exit 0
  fi

  tail -60 "$LOG" >&2
  # seed-faculty deliberately redacts Moodle's exception message, so its
  # dml_write_exception class is the only safe signature available in CI.
  # Retrying once is bounded; a deterministic schema/fixture bug still fails.
  # Hosted runners need not have ripgrep; grep is available in the base image.
  if [ "$ATTEMPT" -ne 1 ] || ! grep -Eq \
      '!!! Error writing to database !!!|\[seed-faculty\] dml_write_exception code=0 cause=none causecode=none' \
      "$LOG"; then
    exit 1
  fi

  echo "Moodle $VERSION SQLite write failed; rebuilding disposable fixture once" >&2
  printf 'version=%s\nattempts=2\nfirst_error=disposable_fixture_database_write\n' "$VERSION" \
    > "test/e2e/artifacts/decade-retry-$VERSION.txt"
  make moodle-purge V="$VERSION"
  make moodle-up V="$VERSION"
done
