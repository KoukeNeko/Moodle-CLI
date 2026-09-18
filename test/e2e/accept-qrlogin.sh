#!/usr/bin/env bash
# QR 登入的真站驗收。需要 HTTPS 站：Moodle 對 http 直接回 httpsrequired。
#
#   make moodle-up V=tls
#   test/e2e/accept-qrlogin.sh
set -euo pipefail

REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
BIN="$REPO_DIR/bin/moodle"
[ -x "$BIN" ] || { echo "先 make build" >&2; exit 2; }

SITE=${SITE:-https://localhost:8443}
export SSL_CERT_FILE="$REPO_DIR/test/e2e/tls/ca.crt"
[ -f "$SSL_CERT_FILE" ] || { echo "沒有測試憑證，先 make moodle-up V=tls" >&2; exit 2; }

CONTAINER=$(docker compose --project-name moodle-cli-e2e \
  --file "$REPO_DIR/test/e2e/docker-compose.yml" ps -q v52-tls)
[ -n "$CONTAINER" ] || { echo "HTTPS 站沒有在跑" >&2; exit 2; }

# 產生一組真的 QR 登入 key。Moodle 沒有對外的介面可以要一組，所以從容器內產生，
# 就像使用者在瀏覽器裡看到 QR 圖那樣。
QRGEN=$(mktemp)
cat > "$QRGEN" <<'PHP'
<?php
define('CLI_SCRIPT', true);
require('/var/www/html/config.php');
$u = $DB->get_record('user', ['username' => 'student1'], '*', MUST_EXIST);
\core\session\manager::set_user($u);
$key = \tool_mobile\api::get_qrlogin_key(get_config('tool_mobile'));
echo "moodlemobile://{$CFG->wwwroot}?qrlogin={$key}&userid={$u->id}\n";
PHP
# docker cp 會保留權限，而 mktemp 產出的是 600；容器裡跑的是 nobody，讀不到。
chmod 644 "$QRGEN"
docker cp "$QRGEN" "$CONTAINER:/tmp/qrgen.php" >/dev/null
rm -f "$QRGEN"
qr() { docker exec "$CONTAINER" php /tmp/qrgen.php | tail -1; }

fail=0
check() { # check <說明> <預期子字串> <實際>
  case "$3" in
    *"$2"*) echo "  ok   $1" ;;
    *) echo "  FAIL $1: 沒看到 \"$2\"，實際是：$3"; fail=1 ;;
  esac
}

MOODLE_CLI_CONFIG=$(mktemp -d)/config.yaml
export MOODLE_CLI_CONFIG
"$BIN" site add tls "$SITE" >/dev/null

echo "== 憑證驗證是真的走 TLS =="
# 不給 CA 就必須失敗。工具裡沒有、也不該有「略過憑證檢查」的旗標。
out=$(SSL_CERT_FILE=/dev/null "$BIN" doctor 2>&1 || true)
check "不信任的 CA 會被拒絕" "certificate" "$out"

echo "== QR 登入 =="
CODE=$(qr)
out=$("$BIN" auth login --method qr --qr "$CODE" 2>&1 || true)
# 這台機器沒有 keychain 時會停在儲存那一步，但交換本身已經成功。
case "$out" in
  *keychain*) echo "  ok   QR 交換成功（keychain 不可用，停在儲存）" ;;
  *"Signed in"*|*"signed in"*) echo "  ok   QR 登入完成" ;;
  *) echo "  FAIL QR 登入：$out"; fail=1 ;;
esac

echo "== 用過的 key 不能再用 =="
out=$("$BIN" auth login --method qr --qr "$CODE" 2>&1 || true)
check "重用會被拒絕" "key" "$out"
check "並說明是單次使用" "single use" "$out"

echo "== 站台要求 Moodle app 的身分 =="
# 這一條驗的是程式碼裡的主張：那個 User-Agent 是必要的，而且只用在這一個請求。
KEY=$(qr | sed 's/.*qrlogin=\([^&]*\).*/\1/')
out=$(curl -fsS --cacert "$SSL_CERT_FILE" -A "moodle-cli/test" \
  "$SITE/lib/ajax/service-nologin.php?info=tool_mobile_get_tokens_for_qr_login" \
  -d "[{\"index\":0,\"methodname\":\"tool_mobile_get_tokens_for_qr_login\",\"args\":{\"qrloginkey\":\"$KEY\",\"userid\":4}}]" || true)
check "一般 UA 會被站台拒絕" "apprequired" "$out"

[ $fail -eq 0 ] && echo "全部通過"
exit $fail
