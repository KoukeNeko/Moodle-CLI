#!/usr/bin/env bash
# 對測試站套用或還原一個範圍突變。
#
#   test/e2e/mutate.sh list
#   test/e2e/mutate.sh apply suspend-enrolment [v52-std]
#   test/e2e/mutate.sh revert suspend-enrolment
#
# 每一種突變都保留物件本身，只改一個讓端點不再回傳它的維度。
set -euo pipefail

WHAT="${1:?usage: mutate.sh list|apply|revert [name] [service]}"
if [ "$WHAT" = list ]; then
  NAME=""; SERVICE="${2:-v52-std}"
else
  NAME="${2:?usage: mutate.sh $WHAT <name> [service]}"; SERVICE="${3:-v52-std}"
fi

REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
CONTAINER=$(docker compose --project-name moodle-cli-e2e \
  --file "$REPO_DIR/test/e2e/docker-compose.yml" ps -q "$SERVICE")
[ -n "$CONTAINER" ] || { echo "$SERVICE 沒有在跑" >&2; exit 2; }

docker cp "$REPO_DIR/test/e2e/mutate.php" "$CONTAINER:/mutate.php" >/dev/null
docker exec "$CONTAINER" php /mutate.php "$WHAT" ${NAME:+"$NAME"}
