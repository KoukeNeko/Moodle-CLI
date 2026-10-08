# 相容性矩陣

*[English](COMPATIBILITY.md) · **繁體中文***

更新日期：2026-10-09

下表每一個「已驗證」都有實際跑過的依據：CI job、本專案的測試，或對真實站台量測的紀錄。其餘一律寫明狀態。

## Moodle 伺服器

| 伺服器 | 狀態 | 依據與限制 |
| --- | --- | --- |
| Moodle 4.5.12 LTS | 已驗證 | 每次發版跑 Docker E2E：標準站台、Web Service 受限站台、十年 fixture。`make moodle-up V=v45`。 |
| Moodle 5.1.7 | 已驗證 | 同一套，`V=v51`。 |
| Moodle 5.2.3 | 已驗證 | 同一套，`V=v52`。PostgreSQL 規模測試在此執行（5 萬學生、1000 門課）。 |
| Moodle 4.5.3 | 單一真實站台的讀取與登入已驗證 | 一個 SSO 後的大學站台，學生 token 可用 465 個函式：`site inspect`、課程、作業、測驗、成績、論壇讀取，以及 session 換 token。不是 Docker 測試套件，所以只代表一個站台而非矩陣。2026-09-26。 |
| 其他 4.5.x、5.1.x、5.2.x | 預期相容 | 同一個 API 家族。請先跑 `moodle doctor`：CLI 檢查站台實際暴露哪些函式，不依版本號推測。 |
| Moodle 4.4 及更早 | 未驗證 | Typed `ws` registry 沒有對應快照，`moodle ws` 會拒絕並指向 `moodle api call`。一般讀取命令可能可用。 |
| Moodle 5.0 | 未驗證 | 位於兩個已驗證版本之間，很可能沒問題，但沒有任何量測。 |

Typed registry 是三個已驗證版本（各 759／761／755 個函式）產生的 780 函式聯集，各版本的參數與回傳
schema 分開保留。

## 站台設定決定的部分

不論版本，Moodle 都可以被設定成某個功能無法使用。`moodle doctor` 會回報目前登入的站台上，每個功能由
哪條路線回答。

| 站台狀態 | 影響 | 是否驗證 |
| --- | --- | --- |
| Mobile web service 開啟、可取得 token | 所有功能，包含繳交作業 | 是，三個版本 |
| Mobile web service 關閉、只有 browser session | 只能讀取。課程走 AJAX；作業、測驗、成績、論壇讀站台自己的頁面。`meta.missing` 會列出該路線讀不到的每個欄位。繳交會拒絕而不是猜測。 | 是：`v45`／`v51`／`v52` 的受限站台情境，以及 2026-09-25 對真實大學站台（Moodle 4.5、SSO、無 token）量測的讀取結果 |
| 課程不對學生開放成績報告 | `grade list` 回報 `permission_denied`，並引用 Moodle 自己的原因 | 是，在真實站台量測 |
| 被 availability 限制擋住的活動 | Moodle 回 HTTP 200，每個被拒的活動各一個 warning，所以清單看起來是完整的。`meta.partial` 為 true，並回報數量。 | 是：某課程 15 份作業中有 11 份對學生隱藏，Moodle 4.5.3，2026-09-26 |
| 成績項目的名稱或分數含標記 | 表格顯示文字；JSON 原樣保留 Moodle 自己的呈現 | 是：multilang 名稱與 scale 的圖示，Moodle 4.5.3 |
| 側欄區塊連結的活動 | 不算成該課程自己的活動 | 是 |

## 作業系統與架構

發版 archive 全為純 Go（`CGO_ENABLED=0`）：

- macOS `amd64`、`arm64`
- Linux `amd64`、`arm64`（另有 `.deb`、`.rpm`、`.apk`）
- Windows `amd64`、`arm64`

| 平台 | 狀態 | 限制 |
| --- | --- | --- |
| Linux | 編譯、單元測試與完整 Docker 測試 | 自動瀏覽器 callback 使用已註冊的 per-user D-Bus handler。 |
| macOS | CI 編譯與單元測試；本機原生 callback 已驗證 | `mobilelaunch` 首次登入會安裝 per-user Apple Event handler。2026-10-09 已驗證原生 URL 傳遞，以及 eCourse2 SSO、token 與課程讀取完整流程。Binary 有 Developer ID 簽章與 Apple 公證，並在發布前於真實 macOS 主機對可下載的 archive 驗證。 |
| Windows | CI 編譯與單元測試 | 沒有自動 callback。Chrome、Edge、Brave v20 的 cookie store 無 cgo 無法讀取，因此不支援瀏覽器匯入，請用 `qr` 或 `manual`。**未經 Authenticode 簽章**，SmartScreen 可能警告。 |

Docker Moodle 測試只在 Linux 執行。macOS 與 Windows 編譯同一份程式並執行同一套單元與契約測試；
macOS 瀏覽器 callback 另有上述本機原生測試與真實站台驗證。

建置設定是朝可重現寫的——build date 與檔案時間戳取自 commit 而非發版時間，`-trimpath` 也不留建置者
路徑——但 CI 目前還沒有重建某個 tag 並比對 digest，所以可重現性是設定的性質，而不是量測到的事實。
macOS archive 本質上無法可重現：公證需要安全時間戳。

## 憑證儲存

| 位置 | 狀態 | 說明 |
| --- | --- | --- |
| macOS Keychain | 預設使用；CI 未涵蓋 | 託管 runner 沒有已解鎖的 keychain，測試使用記憶體儲存。2026-10-09 已在本機以 eCourse2 驗證 token 保存，以及另一個 CLI process 重複使用。 |
| Windows Credential Manager | 預設使用；CI 未涵蓋 | 同上。 |
| Linux Secret Service（GNOME Keyring、KWallet） | 預設使用；CI 未涵蓋 | 2026-09-26 以 GNOME Keyring 人工驗證。需要已解鎖的 collection：在沒有桌面 session 的 SSH 環境下由 D-Bus 啟動的 keyring 會回報自己是鎖住的，錯誤訊息會說明這件事。 |
| 檔案（主動選用） | POSIX 上已驗證 | `--credential-store file` 或 `preferences.credential_store`，給沒有 keychain 的機器：headless Linux、SSH、WSL、容器。永不自動啟用。在 Linux 與 macOS 以 `0600` 建立、目錄 `0700`。**Windows 沒有權限位元**（Go 只對應唯讀屬性），該平台依靠 `%APPDATA%` 既有的 ACL 保護，而且那裡預設本來就是 Credential Manager。 |
| `MOODLE_WS_TOKEN`／`MOODLE_SESSION` | 已驗證 | 只在單一 process 有效，不寫入磁碟。 |

## 登入方式

| 方式 | 狀態 | 說明 |
| --- | --- | --- |
| 既有 web service token | 已驗證 | `auth login --token-stdin`。 |
| Moodle 帳號密碼 | 已驗證 | 僅在站台自己顯示登入表單時可用。 |
| QR 登入碼 | 真實站台未驗證 | 已實作並有單元測試。Moodle 要求 HTTPS，而 Docker 測試站台是 HTTP，因此沒有端到端驗證。Moodle 把這個碼直接放在使用者自己的個人資料頁，藏在「檢視 QR code」按鈕後面，而且**不會**顯示給站台管理員——見 `admin/tool/mobile/lib.php`。這是唯一會送出 `MoodleMobile` User-Agent 的請求。 |
| 用瀏覽器 session 換 token | 真實站台已驗證 | `auth login --method browser-session`。貼一次新的 `MoodleSession` cookie 就換成一般的 web service token，且不隨原 session 過期。2026-09-26 在 SSO 後的 Moodle 4.5.3 量測。 |
| 瀏覽器 SSO 自動 callback | macOS 單一真實站台端到端已驗證 | 2026-10-09 已以 `mobilelaunch` 完成 eCourse2 SSO、自動 callback、token 驗證與保存，以及後續 Web Service 課程讀取。原生 Apple Event 傳遞有本機主動啟用的測試；Linux handler 註冊與 callback transaction 有測試，但完整 Linux SSO 往返仍未驗證。eCourse2 token 到期時間仍未知。 |
| 手動貼上 callback | 已驗證 | 所有平台。 |
| 匯入瀏覽器 session（Firefox、Safari、Chromium） | Parser 以擷取的真實 store 驗證 | Windows 的 Chrome／Edge／Brave v20 無法讀取；Safari 需要完全磁碟存取權，若磁碟上沒有 cookie 則走 `auth import-session`。 |
| 貼上 session cookie | 真實站台已驗證 | `auth import-session`，隱藏輸入或 `--stdin`。2026-09-25 在 SSO 後的 Moodle 4.5 量測。 |

## 介面

| 介面 | 狀態 | 說明 |
| --- | --- | --- |
| JSON contract v1 | 已驗證 | 每個 response kind 都有內嵌 JSON Schema，並在測試中斷言。Exit code 固定。 |
| `moodle schema <命令>` | 已驗證 | 每個命令的 `safety` 與 `idempotency`，供無人看管的呼叫者判斷。 |
| MCP server | 已驗證 | `moodle mcp serve`，除非 `--allow-write` 否則唯讀。目前還沒有測驗相關工具。 |
| Shell 補全 | 由本專案測試驗證 | bash、zsh、fish、PowerShell，透過 `moodle shell-completion`。安裝腳本本身的往返由變更時觸發的 workflow 執行。 |

Moodle CLI 與 Moodle 及 Moodle HQ 無關。「Moodle」是 Moodle Pty Ltd 的商標。
