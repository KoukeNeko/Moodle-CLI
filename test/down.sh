#!/usr/bin/env bash
# 停掉一個版本的測試站（保留資料 volume）。加 --purge 連資料一起刪。
set -euo pipefail
PROFILE="${1:?usage: down.sh <v45|v51|v52> [--purge]}"
cd "$(dirname "$0")"
if [ "${2:-}" = "--purge" ]; then
  docker compose --profile "$PROFILE" down -v
else
  docker compose --profile "$PROFILE" down
fi
