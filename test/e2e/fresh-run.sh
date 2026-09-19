#!/usr/bin/env bash
# 從全新的世界跑一次完整 e2e：丟掉 volume → 重裝 → 佈建 → 跑 → 比對。
#
#   test/e2e/fresh-run.sh            # 會先問一次
#   test/e2e/fresh-run.sh --yes      # 不問
#   test/e2e/fresh-run.sh --yes v45  # 指定版本（預設 v52）
#
# 為什麼要有這一支：full-run.sh 會改變它自己依賴的狀態——第 10 節把挑中的作業
# 交出去，於是連跑第二次的前提就不一樣了。加了把關之後不會再靜默錄壞，但
# 「跑完站台就不一樣了」這件事本身還在。唯一乾淨的解法是每一輪都從同一個起點開始。
#
# 這裡的隔離單位是 **volume**，不是資料庫：這些站台用 sqlite3，而
# $CFG->dataroot 就是 /var/www/moodledata——也就是那個 named volume。
# 資料庫檔案與上傳的檔案在同一個 volume 裡，所以兩者不可能各自回到不同的時間點。
# （Moodle 自己的 Behat 每個 scenario 重置 DB **加** dataroot，就是為了這件事。）
set -euo pipefail

YES=0
[ "${1:-}" = "--yes" ] && { YES=1; shift; }
VERSION="${1:-v52}"

REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$REPO_DIR"

if [ "$YES" != 1 ]; then
  echo "這會刪除 $VERSION 的 volume（站台上的所有資料都會不見）。"
  printf '繼續？[y/N] '
  read -r answer
  case "$answer" in y | Y | yes | YES) ;; *) echo "取消了。" >&2; exit 1 ;; esac
fi

echo "==> 1/5 丟掉舊的 volume"
./scripts/moodle-env.sh down "$VERSION" --purge

echo "==> 2/5 重新安裝並佈建（首次安裝要幾分鐘）"
./scripts/moodle-env.sh up "$VERSION"
./test/e2e/seed-masters.sh "${VERSION}-std"

echo "==> 3/5 建置"
make build

# full-run.sh 的連接埠預設是 v52 的。不把版本的連接埠傳進去，就會變成
# 「佈建了這個版本、卻對另一個版本跑測」——而且兩邊都活著的時候不會報錯，
# 只會安靜地量錯站台。第一次寫這支就是這樣壞的。
case "$VERSION" in
  v45) export STD_PORT=8451 NOWS_PORT=8452 ;;
  v51) export STD_PORT=8511 NOWS_PORT=8512 ;;
  v52) export STD_PORT=8521 NOWS_PORT=8522 ;;
  *) echo "不認得的版本：$VERSION" >&2; exit 2 ;;
esac

echo "==> 4/5 跑完整 e2e（std=$STD_PORT nows=$NOWS_PORT）"
./test/e2e/full-run.sh --nows

echo "==> 5/5 跟上一輪比對"
# 這是全新的站台，所以第一次跑沒有東西可以比——那不是失敗。
if ! ./test/e2e/compare-run.sh; then
  echo
  echo "結束碼有差異。若這是這個版本的第一輪，那是預期的；" >&2
  echo "否則請把上面列出來的每一條都看過。" >&2
  exit 1
fi
