#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
PROJECT="$ROOT/report-site"

"$ROOT/scripts/build-report-data.py"

# Codex's installed data-app compiler gives authors a verified, dependency-free
# build. A normal checkout (including GitHub Actions) uses the pinned lockfile.
COMPILER=$(find /root/.codex/plugins/cache -path '*/data-analytics/*/scripts/data-app.mjs' -print 2>/dev/null | sort -V | tail -n 1 || true)
if [[ -n "$COMPILER" ]]; then
  exec node "$COMPILER" build --project-dir "$PROJECT" --separate-data
fi

cd "$PROJECT"
npm ci
npm run build
