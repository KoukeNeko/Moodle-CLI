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
| `manual` | 在瀏覽器登入後，若瀏覽器顯示 callback URL，將它貼回 CLI。 |

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
# macOS：沿用 Safari，不需要安裝 Firefox。
moodle auth import-browser --site school --browser safari --store
```

支援 macOS Safari、Firefox session snapshot 與 Chromium 的 Linux fallback encryption。Safari 的
cookie 檔格式並非 Apple 公開介面，macOS 也可能拒絕存取；程式會明確回報，不會暗中換用別的瀏覽器。
Safari 畫面已登入，但目前的 session 不一定會存進磁碟 cookie 檔；找不到 `MoodleSession` 不代表未登入。
若系統拒絕，請先衡量是否願意讓終端機讀取瀏覽器資料，再調整 macOS 隱私權設定。由 macOS Keychain、Windows
DPAPI、Linux secret service（`v11`）或 app-bound encryption（`v20`）保護的 Chromium cookie 會被拒絕，
不嘗試繞過。Browser profile 內含許多站台的憑證，因此匯入必須由使用者明確執行。程式只回傳或保存相符的
Moodle cookie，但尋找過程必然需要解析瀏覽器儲存內容。關閉瀏覽器或從網頁登出後，該 session 可能立即失效。

如果 Safari 顯示已登入，`import-browser` 卻找不到 `MoodleSession`，不必繼續調整磁碟權限，也不必安裝其他
瀏覽器。必要時先到 **Safari → 設定 → 進階** 開啟開發者功能，再在 Moodle 分頁選 **開發 → 顯示網頁檢閱器**；
於 **儲存空間 → Cookie** 選 Moodle 網域，只複製 `MoodleSession` 的 **值**，然後執行：

```sh
moodle auth import-session --site school
```

在不顯示輸入內容的提示下貼上值並按 Return。CLI 會先向該站台驗證，再存入系統鑰匙圈，不會印出值。
透過安全管線輸入時可加 `--stdin`；不要把值放在命令參數、shell 歷史、截圖、聊天室或 issue。
此方法不需要「完整磁碟存取權」。[Apple 的 Safari 開發者工具說明](https://support.apple.com/en-ca/guide/safari/sfri20948/mac)。

## 非互動模式

請從 stdin 讀 secret，不要放在 command-line argument：

```sh
printf '%s' "$TOKEN" | moodle auth login --site school --token-stdin
printf '%s' "$PASSWORD" | moodle auth login --site school \
  --method password --username student --password-stdin
```

一次性 process 可用 `MOODLE_WS_TOKEN` 或 `MOODLE_SESSION` 覆寫已存憑證，不寫入磁碟或 keychain。

## 沒有 keychain 的機器

Headless Linux、SSH 連線、WSL 與多數容器都沒有 Secret Service，作業系統沒有地方存憑證。登入時會回報
這件事，並給兩條路：環境變數只撐一次執行，或改用檔案長期保存。

```sh
moodle --credential-store file auth login --site school --token-stdin
moodle --credential-store file auth status
```

不想每次都帶旗標，就把選擇寫進設定：

```yaml
preferences:
  credential_store: file
```

不會自動切換到檔案。檔案是設定檔旁的 `credentials.json`，以 `0600` 建立、目錄 `0700`，`auth login`
會印出路徑。這隔離同機的其他帳號，但擋不住以同一使用者身分執行的程式。`moodle auth logout` 會移除該
筆憑證，檔案空了也會一併刪除。

## 登出

```sh
moodle auth status
moodle auth logout
```

登出只刪本機副本，不撤銷 Moodle token，因為 Moodle 可能把同一個 token 給 mobile app 使用。需要從
server 失效時，請到 Moodle 的 Security keys 頁面撤銷。
