#!/usr/bin/env bash
# Lifecycle for the PostgreSQL-backed scale site. Only one version may run at
# a time; the GitHub workflow also serialises jobs at repository scope.
set -euo pipefail

REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
COMPOSE_FILE="$REPO_DIR/test/scale/docker-compose.yml"
PROJECT_NAME="moodle-cli-scale"

select_version() {
  case "$1" in
    v45) export SCALE_MOODLE_TAG=v4.5.12 SCALE_PORT=9451 ;;
    v51) export SCALE_MOODLE_TAG=v5.1.7  SCALE_PORT=9511 ;;
    v52) export SCALE_MOODLE_TAG=v5.2.3  SCALE_PORT=9521 ;;
    *) echo "unknown version: $1 (expected v45|v51|v52)" >&2; exit 2 ;;
  esac
}

dc() { docker compose --project-name "$PROJECT_NAME" --file "$COMPOSE_FILE" "$@"; }

case "${1:-}" in
  up)
    select_version "${2:-v52}"
    "$REPO_DIR/scripts/runner-preflight.sh"
    dc up --detach --wait
    container=$(dc ps -q moodle)
    docker network disconnect "${PROJECT_NAME}_bootstrap" "$container" 2>/dev/null || true
    "$REPO_DIR/test/e2e/seed.sh" "$container" std
    printf 'scale site ready: http://127.0.0.1:%s\n' "$SCALE_PORT"
    ;;
  down)
    select_version "${2:-v52}"
    if [ "${3:-}" = "--purge" ]; then
      dc down --volumes --remove-orphans
    else
      dc down --remove-orphans
    fi
    ;;
  status)
    dc ps --all
    ;;
  *)
    echo "usage: moodle-scale-env.sh {up|down|status} [v45|v51|v52] [--purge]" >&2
    exit 2
    ;;
esac
