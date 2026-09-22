#!/bin/sh
# 把 session bus 開在綁定掛載進來的目錄上，主機才連得到。
#
# 容器之間不能共用抽象 socket，但 unix socket 可以跨綁定掛載使用，所以 bus
# 直接住在 /run/keyring（＝主機的 test/e2e/run）。
set -eu

SHARED=/run/keyring
E2E_HOST_UID=${E2E_HOST_UID:-0}
E2E_HOST_GID=${E2E_HOST_GID:-0}

case "$E2E_HOST_UID:$E2E_HOST_GID" in
  *[!0-9:]*) echo "E2E_HOST_UID and E2E_HOST_GID must be numeric" >&2; exit 2 ;;
esac

# dbus-daemon resolves its own numeric UID through the password database before
# it starts. Hosted runners commonly use an ID that does not exist in the base
# image, so create only the missing identity (numeric gosu still does the drop).
if ! getent group "$E2E_HOST_GID" >/dev/null; then
  groupadd --gid "$E2E_HOST_GID" moodle-e2e
fi
if ! getent passwd "$E2E_HOST_UID" >/dev/null; then
  useradd --uid "$E2E_HOST_UID" --gid "$E2E_HOST_GID" \
    --home-dir "/tmp/home-$E2E_HOST_UID" --no-create-home moodle-e2e
fi

mkdir -p "$SHARED"
rm -f "$SHARED/bus"

# D-Bus session bus 只接受與 daemon 相同 UID 的 EXTERNAL 認證。開發機通常以
# root 跑 Docker，但 GitHub hosted runner 不是；若容器仍用 root，主機看得到
# socket 卻會在第一次讀寫時被 reset。兩個 daemon 因此都降權成主機使用者。
export XDG_RUNTIME_DIR="/tmp/xdg-$E2E_HOST_UID"
export HOME="/tmp/home-$E2E_HOST_UID"
mkdir -p "$XDG_RUNTIME_DIR" "$HOME"
chown "$E2E_HOST_UID:$E2E_HOST_GID" "$XDG_RUNTIME_DIR" "$HOME"
chmod 700 "$XDG_RUNTIME_DIR"

gosu "$E2E_HOST_UID:$E2E_HOST_GID" \
  dbus-daemon --session --address="unix:path=$SHARED/bus" \
  --nofork --nopidfile --print-address &
DBUS_PID=$!

# 等 bus 起來，否則 gnome-keyring-daemon 會連不上。
i=0
while [ ! -S "$SHARED/bus" ]; do
  i=$((i + 1))
  [ "$i" -gt 100 ] && { echo "session bus 沒有起來" >&2; exit 1; }
  sleep 0.1
done

export DBUS_SESSION_BUS_ADDRESS="unix:path=$SHARED/bus"

# 空密碼解鎖：測試用的拋棄式金鑰圈，不該有人把真的憑證放進來。
#
# 用 --foreground 再自己放到背景，而不是 --daemonize：daemonize 會讓它脫離
# 這層 shell，死掉時沒有人知道，症狀是稍後才出現的「Message recipient
# disconnected from message bus」——憑證寫不進去，卻看不出原因。留在前景、
# 由下面的 wait 顧著，它一死 shell 就結束、容器結束，restart 政策會把它拉起來。
printf '\n' | gosu "$E2E_HOST_UID:$E2E_HOST_GID" \
  gnome-keyring-daemon --unlock --components=secrets --foreground >/dev/null &
KEYRING_PID=$!

echo "keychain 就緒：$DBUS_SESSION_BUS_ADDRESS"
trap 'kill "$DBUS_PID" "$KEYRING_PID" 2>/dev/null || true' EXIT INT TERM
# 監看真正提供 Secret Service 的程序；它一死就讓容器重啟，而不是留下只有
# D-Bus socket、實際不能存取秘密的半活狀態。
wait "$KEYRING_PID"
