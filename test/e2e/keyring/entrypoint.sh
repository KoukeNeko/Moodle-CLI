#!/bin/sh
# 把 session bus 開在綁定掛載進來的目錄上，主機才連得到。
#
# 容器之間不能共用抽象 socket，但 unix socket 可以跨綁定掛載使用，所以 bus
# 直接住在 /run/keyring（＝主機的 test/e2e/run）。
set -eu

SHARED=/run/keyring
mkdir -p "$SHARED"
rm -f "$SHARED/bus"

# dbus 要求 XDG_RUNTIME_DIR 只有自己能寫，所以另外放，不跟共用的 socket 同一個目錄。
export XDG_RUNTIME_DIR=/tmp/xdg
mkdir -p "$XDG_RUNTIME_DIR"
chmod 700 "$XDG_RUNTIME_DIR"

dbus-daemon --session --address="unix:path=$SHARED/bus" \
            --nofork --nopidfile --print-address &

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
printf '\n' | gnome-keyring-daemon --unlock --components=secrets --foreground >/dev/null &

echo "keychain 就緒：$DBUS_SESSION_BUS_ADDRESS"
wait
