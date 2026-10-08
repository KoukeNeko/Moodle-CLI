# 登入與站台

[English](Authentication-and-Sites) · [首頁](Home-zh-TW)

## 從這裡開始

一個命令就會向站台詢問它支援什麼、說明哪個方式適合它與原因、列出所有方式與其狀態，
並執行你選的那一個：

```sh
moodle setup https://moodle.example.edu
```

這需要終端機。腳本或 agent 請用下面的 `site add` 與 `auth login --method …`，
它們的每一種憑證都可以從 stdin 傳入。

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
| `mobilelaunch` | macOS，或已裝 callback handler 的 Linux 桌面；站台開啟 mobile services。 |
| `browser-session` | 手上有新的 `MoodleSession`，要交換為 token。 |
| `manual` | 在瀏覽器登入後，若瀏覽器顯示 callback URL，將它貼回 CLI。 |

## Harness 如何選擇登入方式

Harness 要同時考慮站台支援、自己的執行環境，以及已持有的憑證。站台登記完成後，先查驗要使用的
帳號與登入方式：

```sh
moodle auth status --site school --account student --json --no-input
moodle auth methods --site school --json --no-input
```

憑證有效就直接用它執行後續命令，不必再次呼叫 `auth login`。注入的 `MOODLE_WS_TOKEN` 或
`MOODLE_SESSION` 優先於已保存的憑證，因此要確認回傳身分符合預期使用者。Browser session
可用於支援的唯讀 AJAX／HTML 功能；Web Service 操作，包括繳交作業，需要 token。

`auth methods` 的 `data[]` 每筆包含 `name`、`description`、`availability`、`reason`。
`available` 只代表探測認為方法可能可用，不代表 harness 已有必要輸入、桌面瀏覽器，或有人能完成
SSO／MFA。`unknown` 代表無法確認，不代表停用。決策使用 `name` 與 `availability`；description
和 reason 是說明文字，不是穩定的 code。目前回應沒有無人值守能力或必要輸入的欄位，請依序套用下表：

| Harness 已有的條件 | 採用方式 | 此時是否需要人或瀏覽器？ |
| --- | --- | --- |
| 可用的已存憑證或環境變數憑證 | 直接重複使用，不必登入。 | 不需要，但必須能存取憑證儲存區。 |
| 要保存的既有 Web Service token | `auth login --method token --token-stdin`。 | 不需要，從 stdin 傳入 token。 |
| Moodle 原生帳密，且站台支援 | `auth login --method password --username student --password-stdin`。 | 不需要，密碼走 stdin；學校 SSO 帳密不等同原生帳密。 |
| SSO、macOS／Linux 桌面瀏覽器，且有人能接手登入 | `auth login --method mobilelaunch`；Linux 需先註冊 handler。 | 需要人在瀏覽器完成登入。 |
| 新的登入 QR 解碼內容，且站台支援 | `auth login --method qr --qr-stdin`。 | 交換時不需要，但取得 QR 資料需要先在瀏覽器登入。 |
| 明確提供的新 browser session，且站台支援 | `auth login --method browser-session --session-cookie-stdin`，嘗試交換 token。 | 交換時不需要，但 Moodle 可能拒絕。 |
| 有人能取得 mobile launch callback URL | `auth login --method manual`。 | 需要人開啟印出的網址，再貼回 callback。 |
| 已明確授權匯入 session，且只需唯讀功能 | `auth import-browser --store` 或 `auth import-session --stdin`。 | 需要既有的已登入 session；匯入不會核發 token。 |
| Headless、使用 SSO，且沒有上述憑證 | 停止登入工作，回報需要人完成登入並提供憑證。 | 需要；CLI 尚無 device-code 或遠端 callback 交接流程。 |

上述命令加上 `--site school`，並將 `student` 換成要使用的帳號名稱或使用者名稱。無人值守的
憑證交換也要加 `--json --no-input`。Secret 不放進 argv、log 或聊天；沒有 keychain 的機器
在保存憑證時要明確指定 `--credential-store file`，或改由每個 process 注入憑證。

判斷條件後明確指定 `--method`；省略可能會自動選到 mobile launch，而明確指定的方法即使探測
結果不確定也會嘗試執行。`--json` 只控制輸出格式。目前 `mobilelaunch --no-input` 會跳過 Enter
提示，但仍開啟瀏覽器並等待 callback，並不是 headless 登入模式；未安排瀏覽器登入時，無人值守的
harness 不應呼叫它。遇到認證 exit code `4`，停止依賴登入的工作並安排更新憑證；設定或憑證儲存區
錯誤（`3`）、網路錯誤（`10`）則分別處理。沒有人能接手時，不要反覆嘗試 SSO。

eCourse2 在 Mac 上已驗證的互動登入方式是 `mobilelaunch`。完成一次瀏覽器登入後，同一使用者執行的
harness 若能存取鑰匙圈，就可重複使用已存 token；另一台 headless 主機需要另外配置憑證。

## macOS 與 Linux Browser SSO

macOS 執行 `moodle auth login --site school --method mobilelaunch`，按 Enter 開啟瀏覽器。
首次登入會在 `~/Applications` 安裝目前使用者的登入 handler，之後重複使用。完成 SSO 後，
若瀏覽器詢問，允許開啟登入 handler；CLI 自動接收、驗證並保存 token，不需要複製 callback 網址。

Linux 先註冊一次 handler：

```sh
moodle auth register-handler
moodle auth handler-status
moodle auth login --site school --method mobilelaunch
```

註冊只影響目前使用者。Linux callback 走 D-Bus；macOS 走 Apple Event 與匿名 stdin pipe，
都不把 token 放進 process command line，且共用私人的 Unix socket。登入 transaction 有
時間限制、只能使用一次、綁定站台 canonical URL，回傳 token 也會先驗證再保存。

## Token 有效期限與重新登入

學校 OAuth／SSO 在瀏覽器內完成身分驗證，再由 Moodle 透過 mobile launch 回傳 Web Service token，
CLI 保存並重複使用它。學校 OAuth 的 callback 回到 Moodle；mobile launch 的 callback 才把
Web Service token 交給 CLI。兩者是不同步驟，CLI 不會取得 OAuth refresh token。

Moodle 5.2 新核發的 mobile Web Service token 預設有效 **12 週（84 天）**，由管理員的
`tokenduration` 設定控制。這只是預設值，不能保證特定站台或 token 也適用。
參見 [Moodle 的 token 期限設定](https://github.com/moodle/moodle/blob/MOODLE_502_STABLE/public/admin/settings/security.php)。

期限從 token 建立時間算起。之後重新登入時，Moodle 可能回傳仍有效的既有 token，因此不一定會重新
取得完整的 84 天；持續呼叫 API 也不會延長期限。
參見 [Moodle 的 token 核發邏輯](https://github.com/moodle/moodle/blob/MOODLE_502_STABLE/public/lib/external/classes/util.php)。

CLI 目前會確認憑證是否可用，不會回報建立時間或到期日。2026-10-09 的 eCourse2
（`ecourse2.ccu.edu.tw`）驗證已成功完成瀏覽器登入與課程讀取，但 mobile launch callback
及已查詢的站台／mobile 設定 API 都未提供這顆 token 的實際到期時間。
不能用這次登入時間或 Moodle 預設值推算它的到期日。

使用自動 mobile launch 的站台，可檢查要使用的帳號；token 到期或遭撤銷後重新登入：

```sh
moodle auth status --site school --account student
moodle auth login --site school --account student --method mobilelaunch
moodle auth status --site school --account student
```

將 `student` 換成 `moodle site list` 列出的帳號名稱。macOS 或已設定 handler 的 Linux
桌面按 Enter 後完成瀏覽器登入，CLI 會自動接回並驗證 token。目前沒有自動 refresh；無人值守的
工作在 token 到期後，需要先完成互動登入才能恢復。其他登入方式則明確指定原本的方法重新執行。

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
