#!/usr/bin/env bash
# 把四學期碩士班的情境資料佈建到一個已就緒的測試站。
#
#   make moodle-up V=v52
#   test/e2e/seed-masters.sh                    # 預設 v52-std
#   test/e2e/seed-masters.sh v52-nows           # 也可以指定別的服務
#
# 可重複執行：每一項都先查再建。
set -euo pipefail

SERVICE="${1:-v52-std}"
REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
CONTAINER=$(docker compose --project-name moodle-cli-e2e \
  --file "$REPO_DIR/test/e2e/docker-compose.yml" ps -q "$SERVICE")
[ -n "$CONTAINER" ] || { echo "$SERVICE 沒有在跑" >&2; exit 2; }

docker cp "$REPO_DIR/test/e2e/seed-masters.php" "$CONTAINER:/seed-masters.php" >/dev/null
docker exec "$CONTAINER" php /seed-masters.php
