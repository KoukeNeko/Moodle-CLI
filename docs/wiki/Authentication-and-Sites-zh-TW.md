# 登入與站台

[English](Authentication-and-Sites) · [首頁](Home-zh-TW)

## 新增與選擇站台

```sh
moodle site add school https://moodle.example.edu
moodle site list
moodle site use school
moodle doctor
```

站台設定只記錄 URL、帳號與目前選擇，不包含 token 或 browser session。同一站台可以保存多個帳號，
並以 `--account` 選擇。

## 探測登入方式

```sh
moodle auth methods --site school
```

結果明確區分 `available`、`unavailable` 與 `unknown`。連不到站台時不會被誤報成每種登入都被關閉。

| 方法 | 適用情況 |
| --- | --- |
| `token` | 已有 Moodle Web Service token。 |
| `password` | 站台允許以 Moodle 原生帳密登入 mobile services。 |
| `qr` | 已解碼 Moodle app 登入 QR code。 |
| `mobilelaunch` | Linux 已裝 callback handler，且站台開啟 mobile services。 |
| `browser-session` | 手上有新的 `MoodleSession`，要交換為 token。 |
| `manual` | 在瀏覽器登入後貼回 callback URL；所有平台可用。 |

## Linux Browser SSO

```sh
moodle auth register-handler
moodle auth handler-status
moodle auth login --site school --method mobilelaunch
```

註冊只影響目前使用者。Browser callback 走 D-Bus，不出現在 process command line。登入 transaction 有
時間限制、只能使用一次、綁定站台 canonical URL，回傳 token 也會先驗證再保存。

## 匯入既有瀏覽器 session

```sh
moodle auth import-browser --site school --list-profiles
moodle auth import-browser --site school --store
```

支援 Firefox session snapshot 與 Chromium 的 Linux fallback encryption。由 macOS Keychain、Windows
DPAPI、Linux secret service（`v11`）或 app-bound encryption（`v20`）保護的 Chromium cookie 會被拒絕，
不嘗試繞過。Browser profile 內含許多站台的憑證，因此匯入必須由使用者明確執行。程式只回傳或保存相符的
Moodle cookie，但尋找過程必然需要解析瀏覽器儲存內容。關閉瀏覽器或從網頁登出後，該 session 可能立即失效。

## 非互動模式

請從 stdin 讀 secret，不要放在 command-line argument：

```sh
printf '%s' "$TOKEN" | moodle auth login --site school --token-stdin
printf '%s' "$PASSWORD" | moodle auth login --site school \
  --method password --username student --password-stdin
```

一次性 process 可用 `MOODLE_WS_TOKEN` 或 `MOODLE_SESSION` 覆寫已存憑證，不寫入磁碟或 keychain。

## 登出

```sh
moodle auth status
moodle auth logout
```

登出只刪本機副本，不撤銷 Moodle token，因為 Moodle 可能把同一個 token 給 mobile app 使用。需要從
server 失效時，請到 Moodle 的 Security keys 頁面撤銷。
