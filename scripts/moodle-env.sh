#!/usr/bin/env bash
# 測試用 Moodle 環境的單一進入點。慣例沿用 KoukeNeko/taiga-cli 的
# scripts/test-integration.sh：固定 project name、compose --wait、失敗時倒出 log。
#
#   scripts/moodle-env.sh up     v52      # 起容器 → 等就緒 → 佈建
#   scripts/moodle-env.sh down   v52      # 停掉（保留資料）
#   scripts/moodle-env.sh down   v52 --purge
#   scripts/moodle-env.sh status
#
# 一次只起一個版本：每個站約 300–500MB。
set -euo pipefail

REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
COMPOSE_FILE="$REPO_DIR/test/e2e/docker-compose.yml"
PROJECT_NAME="moodle-cli-e2e"

dc() { docker compose --project-name "$PROJECT_NAME" --file "$COMPOSE_FILE" "$@"; }

ports_for() {
  case "$1" in
    v45) echo "8451 8452" ;;
    v51) echo "8511 8512" ;;
    v52) echo "8521 8522" ;;
    # HTTPS 站只有一個，沒有變體站。
    tls) echo "8443 -" ;;
    *) echo "unknown version: $1 (expected v45|v51|v52|tls)" >&2; return 2 ;;
  esac
}

cmd_up() {
  local ver="${1:?usage: moodle-env.sh up <v45|v51|v52|tls>}"
  read -r std_port nows_port <<<"$(ports_for "$ver")"

  if [ "$ver" = "tls" ]; then
    # 憑證要先有，nginx 才起得來。
    "$REPO_DIR/test/e2e/make-certs.sh"
  fi

  echo "==> 啟動 $ver（首次安裝 Moodle 需數分鐘）"
  # --wait 會等到 compose 的 healthcheck 通過才返回，不必自己輪詢。
  if ! dc --profile "$ver" up --detach --wait; then
    echo "ERROR: 容器未能就緒，以下是 log：" >&2
    dc --profile "$ver" logs --no-color --tail 50 >&2
    exit 1
  fi

  if [ "$ver" = "tls" ]; then
    "$REPO_DIR/test/e2e/seed.sh" "$(dc ps -q v52-tls)" std
    cat <<EOF

就緒：
  HTTPS 站  https://localhost:$std_port

帳號：admin/Admin123!  teacher1/Teacher123!  student1/Student123!

憑證是本機自簽的，要讓工具信任它就設這個環境變數（不需要任何略過檢查的旗標）：
  export SSL_CERT_FILE=$REPO_DIR/test/e2e/tls/ca.crt
EOF
    return
  fi

  "$REPO_DIR/test/e2e/seed.sh" "$(dc ps -q "${ver}-std")"  std
  "$REPO_DIR/test/e2e/seed.sh" "$(dc ps -q "${ver}-nows")" nows

  cat <<EOF

就緒：
  標準站（Web Services 開啟）  http://localhost:$std_port
  變體站（Mobile WS 關閉）     http://localhost:$nows_port

帳號：admin/Admin123!  teacher1/Teacher123!  student1/Student123!

取得 token：
  curl -s http://localhost:$std_port/login/token.php \\
    -d username=student1 -d password=Student123! -d service=moodle_mobile_app
EOF
}

cmd_down() {
  local ver="${1:?usage: moodle-env.sh down <v45|v51|v52> [--purge]}"
  ports_for "$ver" >/dev/null
  if [ "${2:-}" = "--purge" ]; then
    dc --profile "$ver" down --volumes --remove-orphans
  else
    dc --profile "$ver" down --remove-orphans
  fi
}

cmd_status() {
  dc ps --all
}

case "${1:-}" in
  up)     shift; cmd_up "$@" ;;
  down)   shift; cmd_down "$@" ;;
  status) cmd_status ;;
  *) echo "usage: moodle-env.sh {up|down|status} [v45|v51|v52]" >&2; exit 2 ;;
esac
