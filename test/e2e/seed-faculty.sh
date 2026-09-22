#!/usr/bin/env bash
# 佈建一位教師十年的授課史，以及其他角色的真實狀態。
#
#   test/e2e/seed-masters.sh && test/e2e/seed-faculty.sh
#
# 要先跑 seed-masters.sh：prof1、mgr1、cc1 是那一份建的，這一份只補他們的歷史。
# 可重複執行。
set -euo pipefail

SERVICE="${1:-v52-std}"
REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
CONTAINER=$(docker compose --project-name moodle-cli-e2e \
  --file "$REPO_DIR/test/e2e/docker-compose.yml" ps -q "$SERVICE")
[ -n "$CONTAINER" ] || { echo "$SERVICE 沒有在跑" >&2; exit 2; }

docker cp "$REPO_DIR/test/e2e/seed-faculty.php" "$CONTAINER:/seed-faculty.php" >/dev/null
docker exec "$CONTAINER" php /seed-faculty.php
