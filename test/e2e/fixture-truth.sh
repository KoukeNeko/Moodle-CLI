#!/usr/bin/env bash
# 問站台的資料庫，而不是問被測的 CLI。
#
#   test/e2e/fixture-truth.sh submittable grad1
#   test/e2e/fixture-truth.sh forums 2
#   test/e2e/fixture-truth.sh readable grad1 2
#
# 第二個參數起原樣傳給 fixture-truth.php。容器名可用 E2E_CONTAINER 覆寫。
set -euo pipefail

CONTAINER="${E2E_CONTAINER:-moodle-cli-e2e-v52-std-1}"
REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)

docker cp "$REPO_DIR/test/e2e/fixture-truth.php" "$CONTAINER:/fixture-truth.php" >/dev/null
docker exec "$CONTAINER" php /fixture-truth.php "$@"
