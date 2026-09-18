#!/usr/bin/env bash
# 套用或還原權限／可見性的量測 fixture（獨立分組、可用性限制）。
#
#   test/e2e/seed-permissions.sh apply            # 預設 v52-std
#   test/e2e/seed-permissions.sh revert
#   test/e2e/seed-permissions.sh apply v52-nows   # 也可以指定別的服務
#
# 刻意不放進 seed-masters.sh：這裡的每一項都會讓某個帳號少看到東西，
# 混進全功能 e2e 就等於讓那一輪的期望輸出取決於限制條件。
# 兩邊都可重複執行；revert 只刪這支腳本自己建的東西。
set -euo pipefail

MODE="${1:-apply}"
case "$MODE" in
  apply|revert) ;;
  *) echo "用法：$0 [apply|revert] [服務名稱]" >&2; exit 2 ;;
esac

SERVICE="${2:-v52-std}"
REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
CONTAINER=$(docker compose --project-name moodle-cli-e2e \
  --file "$REPO_DIR/test/e2e/docker-compose.yml" ps -q "$SERVICE")
[ -n "$CONTAINER" ] || { echo "$SERVICE 沒有在跑" >&2; exit 2; }

docker cp "$REPO_DIR/test/e2e/seed-permissions.php" "$CONTAINER:/seed-permissions.php" >/dev/null
docker exec "$CONTAINER" php /seed-permissions.php "$MODE"
