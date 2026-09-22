# 一個只給測試用的 OS keychain（Secret Service）。
#
# 為什麼需要它：憑證只進 keychain、設定檔絕不存憑證是這個工具的硬規則，而
# `moodle auth login` 是唯一把憑證寫進去的地方。headless 機器上沒有
# org.freedesktop.secrets，那條路就會停在「無法寫入 keychain」——規則最重要
# 的那一半在測試裡等於沒驗到。
#
# 金鑰圈用空密碼解鎖：這是拋棄式的測試容器，不對外開放，裡面不會有真的憑證。
FROM ubuntu:26.04

RUN apt-get update -qq \
 && apt-get install -y -qq --no-install-recommends gnome-keyring dbus gosu \
 && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
