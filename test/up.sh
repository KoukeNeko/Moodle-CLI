#!/usr/bin/env bash
# 起一個版本的測試 Moodle（標準站 + 關閉 Mobile Web Services 的變體站），
# 等待就緒後自動佈建。
#
#   ./up.sh v52        # 起 Moodle 5.2
#   ./up.sh v51
#   ./up.sh v45
#   ./down.sh v52      # 停掉（保留資料）
#
# 一次只起一個版本：每個站台約 300–500MB，六個一起會吃爆 8GB 的機器。
set -euo pipefail

PROFILE="${1:?usage: up.sh <v45|v51|v52>}"
cd "$(dirname "$0")"

case "$PROFILE" in
  v45) STD_PORT=8451; NOWS_PORT=8452 ;;
  v51) STD_PORT=8511; NOWS_PORT=8512 ;;
  v52) STD_PORT=8521; NOWS_PORT=8522 ;;
  *) echo "unknown profile: $PROFILE (expected v45|v51|v52)" >&2; exit 2 ;;
esac

echo "==> 啟動 $PROFILE"
docker compose --profile "$PROFILE" up -d

for svc in "${PROFILE}-std" "${PROFILE}-nows"; do
  cid=$(docker compose ps -q "$svc")
  echo "==> 等待 $svc 就緒（首次安裝 Moodle 需數分鐘）"
  until [ "$(docker inspect -f '{{.State.Health.Status}}' "$cid" 2>/dev/null)" = healthy ]; do
    sleep 5
  done
done

./seed.sh "$(docker compose ps -q "${PROFILE}-std")"  std
./seed.sh "$(docker compose ps -q "${PROFILE}-nows")" nows

cat <<EOF

就緒：
  標準站（Web Services 開啟）  http://localhost:$STD_PORT
  變體站（Mobile WS 關閉）     http://localhost:$NOWS_PORT

帳號：admin/Admin123!  teacher1/Teacher123!  student1/Student123!

取得 token：
  curl -s http://localhost:$STD_PORT/login/token.php \\
    -d username=student1 -d password=Student123! -d service=moodle_mobile_app
EOF
