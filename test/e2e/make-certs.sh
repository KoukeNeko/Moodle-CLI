#!/usr/bin/env bash
# 產生測試站用的本地 CA 與伺服器憑證。
#
# 為什麼要自己簽一個 CA，而不是在工具上加一個「略過憑證檢查」的旗標：
# QR 登入與 SSO 都需要 HTTPS 才驗得到，但那種旗標一旦存在，就會有人在正式環境
# 用它。改成讓測試信任一個只存在於本機的 CA——Go 認 SSL_CERT_FILE，所以驗證流程
# 完全走真正的 TLS 驗證路徑，產品程式碼一行都不用改。
#
# 產出的檔案不入版控：私鑰不該進 git，而且每台機器自己產一份就好。
set -euo pipefail

DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)/tls
mkdir -p "$DIR"

if [ -f "$DIR/ca.crt" ] && [ -f "$DIR/server.crt" ]; then
  # 已經有了就不重簽：重簽會讓已經在跑的容器與已匯出的 SSL_CERT_FILE 對不上。
  exit 0
fi

# CA。有效期短是刻意的：這是測試憑證，不該在誰的機器上留三年。
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout "$DIR/ca.key" -out "$DIR/ca.crt" \
  -subj "/CN=moodle-cli test CA" \
  -addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
  -addext "keyUsage=critical,keyCertSign,cRLSign" 2>/dev/null

# 伺服器憑證。SAN 同時涵蓋 localhost 與 127.0.0.1：Moodle 會用 wwwroot 裡的主機名
# 產生自己的連結，而測試從哪一個名字連進來不一定相同。
openssl req -newkey rsa:2048 -nodes \
  -keyout "$DIR/server.key" -out "$DIR/server.csr" \
  -subj "/CN=localhost" 2>/dev/null
openssl x509 -req -in "$DIR/server.csr" -days 365 \
  -CA "$DIR/ca.crt" -CAkey "$DIR/ca.key" -CAcreateserial \
  -out "$DIR/server.crt" \
  -extfile <(printf 'subjectAltName=DNS:localhost,IP:127.0.0.1\nextendedKeyUsage=serverAuth\n') 2>/dev/null

rm -f "$DIR/server.csr" "$DIR/ca.srl"
chmod 600 "$DIR"/*.key
echo "[certs] 已產生 $DIR/{ca.crt,server.crt,server.key}"
