# 相容性矩陣

*[English](COMPATIBILITY.md) · **繁體中文***

更新日期：2026-09-26

下表每一個「已驗證」都有實際跑過的依據：CI job、本專案的測試，或對真實站台量測的紀錄。其餘一律寫明狀態。

## Moodle 伺服器

| 伺服器 | 狀態 | 依據與限制 |
| --- | --- | --- |
| Moodle 4.5.12 LTS | 已驗證 | 每次發版跑 Docker E2E：標準站台、Web Service 受限站台、十年 fixture。`make moodle-up V=v45`。 |
| Moodle 5.1.7 | 已驗證 | 同一套，`V=v51`。 |
| Moodle 5.2.3 | 已驗證 | 同一套，`V=v52`。PostgreSQL 規模測試在此執行（5 萬學生、1000 門課）。 |
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
| 側欄區塊連結的活動 | 不算成該課程自己的活動 | 是 |

## 作業系統與架構

發版 archive 全為純 Go（`CGO_ENABLED=0`）：

- macOS `amd64`、`arm64`
- Linux `amd64`、`arm64`（另有 `.deb`、`.rpm`、`.apk`）
- Windows `amd64`、`arm64`

| 平台 | 狀態 | 限制 |
| --- | --- | --- |
| Linux | 編譯、單元測試與完整 Docker 測試 | 唯一支援自動瀏覽器 callback handler 的平台。 |
| macOS | CI 編譯與單元測試 | 沒有自動 callback：請用 `auth import-browser`、`auth import-session`，或 `--method manual`／`qr`。Binary 有 Developer ID 簽章與 Apple 公證，並在發布前於真實 macOS 主機對可下載的 archive 驗證。 |
| Windows | CI 編譯與單元測試 | 沒有自動 callback。Chrome、Edge、Brave v20 的 cookie store 無 cgo 無法讀取，因此不支援瀏覽器匯入，請用 `qr` 或 `manual`。**未經 Authenticode 簽章**，SmartScreen 可能警告。 |

Docker Moodle 測試只在 Linux 執行，因此上面與站台互動的行為都是在 Linux 上量測的。macOS 與 Windows
編譯同一份程式並執行同一套單元與契約測試。

建置設定是朝可重現寫的——build date 與檔案時間戳取自 commit 而非發版時間，`-trimpath` 也不留建置者
路徑——但 CI 目前還沒有重建某個 tag 並比對 digest，所以可重現性是設定的性質，而不是量測到的事實。
macOS archive 本質上無法可重現：公證需要安全時間戳。

## 憑證儲存

| 位置 | 狀態 | 說明 |
| --- | --- | --- |
| macOS Keychain | 預設使用；CI 未涵蓋 | 託管 runner 沒有已解鎖的 keychain，往返由記憶體儲存與人工檢查覆蓋，不是自動化。 |
| Windows Credential Manager | 預設使用；CI 未涵蓋 | 同上。 |
| Linux Secret Service（GNOME Keyring、KWallet） | 預設使用；CI 未涵蓋 | 2026-09-26 以 GNOME Keyring 人工驗證。 |
| 檔案（主動選用） | POSIX 上已驗證 | `--credential-store file` 或 `preferences.credential_store`，給沒有 keychain 的機器：headless Linux、SSH、WSL、容器。永不自動啟用。在 Linux 與 macOS 以 `0600` 建立、目錄 `0700`。**Windows 沒有權限位元**（Go 只對應唯讀屬性），該平台依靠 `%APPDATA%` 既有的 ACL 保護，而且那裡預設本來就是 Credential Manager。 |
| `MOODLE_WS_TOKEN`／`MOODLE_SESSION` | 已驗證 | 只在單一 process 有效，不寫入磁碟。 |

## 登入方式

| 方式 | 狀態 | 說明 |
| --- | --- | --- |
| 既有 web service token | 已驗證 | `auth login --token-stdin`。 |
| Moodle 帳號密碼 | 已驗證 | 僅在站台自己顯示登入表單時可用。 |
| QR 登入碼 | 真實站台未驗證 | 已實作並有單元測試。Moodle 要求 HTTPS，而 Docker 測試站台是 HTTP，因此沒有端到端驗證。這是唯一會送出 `MoodleMobile` User-Agent 的請求。 |
| 瀏覽器 SSO 自動 callback | 端到端未驗證 | Handler 註冊與 callback transaction 在 Linux 有測試；完整交換需要有 identity provider 的 HTTPS 站台，目前沒有。ADR-0004 有追蹤。 |
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
