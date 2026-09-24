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
    if [ "$ATTEMPT" -gt 1 ]; then
      printf 'version=%s\nattempts=%s\nfirst_error=Error writing to database\n' \
        "$VERSION" "$ATTEMPT" > "test/e2e/artifacts/decade-retry-$VERSION.txt"
    fi
    exit 0
  fi

  tail -60 "$LOG" >&2
  if [ "$ATTEMPT" -ne 1 ] || ! rg -q '!!! Error writing to database !!!' "$LOG"; then
    exit 1
  fi

  echo "Moodle $VERSION SQLite write failed; rebuilding disposable fixture once" >&2
  printf 'version=%s\nattempts=2\nfirst_error=Error writing to database\n' "$VERSION" \
    > "test/e2e/artifacts/decade-retry-$VERSION.txt"
  make moodle-purge V="$VERSION"
  make moodle-up V="$VERSION"
done
