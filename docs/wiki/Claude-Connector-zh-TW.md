# Claude connector（host 模式）

[English](Claude-Connector) · [首頁](Home-zh-TW)

`moodle mcp host` 以 HTTPS 提供與 `moodle mcp serve` 相同的工具。網頁版、手機 App 與 Cowork 裡的 Claude
透過以 URL 新增的 connector 使用 Moodle，所以 CLI 不必跑在 Claude 所在的機器上。

## Connector 與 MCP

Connector 是 Claude 對這項功能的稱呼，背後一定是 MCP。遠端的 connector 是以 URL 新增的 MCP server；本機的
connector 是 desktop extension，Claude Desktop 也把它列在 connector 裡。

| 名詞 | 是什麼 | 在哪裡執行 | 本專案的命令 |
| --- | --- | --- | --- |
| MCP | 協定：以 JSON-RPC 2.0 訊息傳遞工具呼叫 | — | — |
| 本機（stdio）server | 透過 stdin／stdout 的 MCP | 你的電腦，由用戶端啟動 | `moodle mcp serve` |
| 遠端 server，在 Claude 裡顯示為 connector | 透過 Streamable HTTP 的 MCP | 任何可由 HTTPS 連到的地方；Claude 從 Anthropic 的伺服器連出 | `moodle mcp host` |
| Desktop extension（MCPB） | 為 Claude Desktop 打包的本機 MCP server | 你的電腦 | — |
| Messages API 的 MCP connector | API 參數，把遠端 server 附加到單次請求 | — | — |

claude.ai、Claude Desktop、手機與 Cowork 是從 Anthropic 的伺服器連出，不是從你的裝置連出。位於 `localhost`
或內網的 server 沒有 tunnel 就連不到。Claude Code 則是從你的電腦連出。

## 驗證狀態

2026-10-12，對本機 Moodle 4.5.12 測試站實測：

- 瀏覽器中的 claude.ai：新增自訂 connector、完成登入、列出 14 個工具，並在核准後呼叫其中兩個來回答問題。
- Claude Code 2.1.267（loopback）：登入與連線檢查。
- 官方 MCP Inspector 與獨立的 OAuth 客戶端，經 Cloudflare quick tunnel。

同一天也在 Linux 上以 cloudflared 2026.9.0、用最終版程式實測了由 host 自己啟動 quick tunnel（不帶 `--public-url`）：tunnel
成功啟動，host 在公告位址前確認公開位址通到自己，公開位址回應探索文件，host 停止時 tunnel 一併停止且沒有留下程序。以 `kill -9` 強制終止 host 時，守衛與 `cloudflared` 在兩秒內一併結束。在家目錄放一份
`ingress:` 指向另一個本機服務的 `config.yml` 時，tunnel 仍然通到 host，而那個服務沒有收到任何請求。「tunnel 被終止時 host 也停止」
是用較早版本的程式實測的，最終版由使用假 cloudflared 的測試覆蓋。經由 host 自己啟動的 tunnel 在 claude.ai 的登入沒有重做；
claude.ai 的實測用的是手動啟動的 tunnel。

尚未實測：macOS、Windows、Claude Desktop、手機 App、Cowork、以 Team 或 Enterprise Owner 身分新增 connector，以及 host 重啟後
Claude 對該 connector 的顯示。疑難排解表中，未標示「實際觀察」的列，都是根據程式碼與 Claude 文件推論。

## 連線如何運作

```text
Claude ──HTTPS──▶ tunnel 或 reverse proxy ──▶ moodle mcp host（127.0.0.1）──▶ Moodle
```

1. Claude 呼叫 `POST /mcp`。Host 回 `401`，並指向自己的探索文件。
2. Claude 讀取該文件（`/.well-known/oauth-protected-resource/mcp`）與
   `/.well-known/oauth-authorization-server`，再到 `/register` 自行註冊。
3. Claude 在你的瀏覽器開啟 `/authorize`。頁面列出應用程式、站台、存取範圍（唯讀或讀寫）以及瀏覽器將返回的位置，
   並要求輸入 host passphrase。
4. 你按下允許後，Claude 以 PKCE（S256）到 `/token` 換取 access token（1 小時）與 refresh token
   （30 天，每次使用就換新）。
5. Claude 帶著 `Authorization: Bearer …` 呼叫 `/mcp`。每次連線都先以協定修訂版 2026-07-28 送出
   `server/discover`；host 回 `400` 拒絕，Claude 便退回以 2025-06-18 執行 `initialize`。日誌裡每次連線都會出現一次
   `POST /mcp 400`，屬於預期行為（實際觀察）。

Host 同時是授權伺服器與資源伺服器，且只放行一個人：知道 passphrase 的那位。

## 設定

1. 在 host 所在的機器照常登入。沒有 keychain 的機器可用 `MOODLE_WS_TOKEN` 或 `--credential-store file`，見
   [登入與站台](Authentication-and-Sites-zh-TW)。
2. 建立只有你能讀的 passphrase 檔，然後啟動 host。沒有自己的位址時，只要已安裝
   [`cloudflared`](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/)，
   這樣就夠了：

   ```sh
   umask 077
   openssl rand -base64 24 > ~/.moodle-host-passphrase
   moodle mcp host --passphrase-file ~/.moodle-host-passphrase
   ```

   沒有 `--public-url` 時，host 會自己啟動 Cloudflare quick tunnel。它監聽一個空閒的 loopback 埠，等到 tunnel
   能傳送流量（最多可能要一分鐘），確認該位址通到 host，再印出 connector URL，例如
   `https://example-words.trycloudflare.com/mcp`。這個位址是隨機的、每次啟動都會換，在 host 停止前任何人都連得到：
   知道位址的人都能開啟登入頁，而擋在他們與 access token 之間的只有 passphrase。Cloudflare 會轉送所有流量，也不對
   quick tunnel 提供可用性保證，所以這適合試用。Tunnel 停止時，host 也會停止。

   執行 `cloudflared` 時使用 host 自己的設定檔，所以你的 `config.yml` 無法把 tunnel 導向別處；你的 `TUNNEL_*` 與
   `MOODLE_*` 環境變數也不會傳給它。它需要能連出到 Cloudflare（埠 7844，TCP 與 UDP），而且 Cloudflare 可能限制同一位址
   建立 quick tunnel 的次數：幾分鐘內建立約五個後，就出現過錯誤 1015。使用 quick tunnel 時，`--listen` 必須是 loopback
   位址。

   優先用檔案，不要用 `MOODLE_HOST_PASSPHRASE`：其他 process 讀得到它，也可能留在 shell 歷史；絕對不要把
   passphrase 放在命令列。兩者都沒有時，會產生一組只用於本次執行的 passphrase 並印在 stderr，所以不要把 stderr 導到
   共用或會保留的日誌，例如 systemd journal 或容器日誌。自訂的 passphrase 至少 12 個字元。
3. 要穩定的位址，請用具名 tunnel，或用終止 HTTPS 並轉送給 host 的 reverse proxy，再傳入該位址：

   ```sh
   moodle mcp host --public-url https://moodle-mcp.example.com \
     --passphrase-file ~/.moodle-host-passphrase
   ```

   使用 `--public-url` 時，除非用 `--listen` 指定，host 會監聽 `127.0.0.1:8787`，也不會啟動 tunnel。
4. 在 claude.ai 開啟 **Customize > Connectors**，選 **Add > Add custom connector**，輸入名稱與 host 印出的 connector URL
   （以 `/mcp` 結尾），選 **Continue**。Claude 會偵測出 **Sign in now** 與 **Register automatically**，保持不變並選
   **Add**。接著選 **Connect**，在開啟的頁面輸入 passphrase，再選 **Allow**。

   在對話中，connector 位於 **+ > Connectors**。實測時 Claude 在每次工具呼叫前都會詢問，選項為 **Allow once**、
   **Always allow** 與 **Decline**。

Team 與 Enterprise 方案由 Owner 在 **Organization settings > Connectors** 新增，各成員再以 passphrase 自行連線。
每位連線的成員都以同一個 Moodle 帳號行事，passphrase 是共用祕密，收回存取的唯一方法是停止 host。

Claude Code 用 `claude mcp add --transport http moodle <connector URL>` 註冊，再用
`claude mcp login moodle` 或在 Claude Code 內執行 `/mcp` 啟動相同的登入流程。Claude Code 跑在你的電腦上，所以
不用 tunnel，loopback 位址就能用：

```sh
moodle mcp host --public-url http://localhost:8787
claude mcp add --transport http moodle http://localhost:8787/mcp
```

`--listen` 應維持在 loopback。有 `--public-url` 時預設 `127.0.0.1:8787`，沒有時預設一個空閒的埠。`--public-url` 是不含路徑的
origin，必須是 `https`；唯一例外是 `localhost`、`127.0.0.1` 與 `::1` 可用 `http`，而這只有同一台機器上的 Claude Code
連得到。

`--allow-write` 才提供寫入工具，與 `mcp serve` 相同。其中包含一個通用的寫入工具，可呼叫該帳號有權執行的任何寫入函式
（發文、編輯、刪除），部分變更無法復原。課程內容由其他人（包含其他學生）撰寫並會送到 Claude，所以請讓 Claude 對寫入
工具維持「先詢問」，並在核准前看過參數。

## 控制項

- 呼叫 `/mcp` 需要有效的 access token。Query string 裡的 token 不被接受。
- Passphrase 以固定時間比較；十分鐘內輸錯五次，所有人的登入都會鎖到視窗結束，連正確的 passphrase 也一樣。知道位址的
  人可以藉此一直鎖住登入，但這不會洩漏 passphrase。
- 註冊是開放的：伺服器沒有提供 client metadata document 時，Claude 會自行註冊。Client id 不授予任何權限。Redirect
  必須是 `https`，或 loopback 位址上的 `http`，且必須與註冊時完全相符；loopback 的 redirect 可以使用任何埠。
- 登入頁對任何知道位址的人開放。它列出站台、存取範圍與瀏覽器返回的位置，但不列帳號。應用程式的名稱由應用程式自行決定、
  未經驗證，所以要看的是返回位置；它以瀏覽器解析的方式顯示，因此長得像的字母會顯示成 `xn--` 文字。
- 必須使用 PKCE S256。授權碼只能用一次、五分鐘內有效。Refresh token 只能用一次。再次使用已用過的授權碼或 refresh token
  會結束那次登入，由它發出的一切都會失效；但若在 refresh 之後十秒內，則視為兩個請求同時競爭，不會結束登入。
- Client id 與進行中的登入是「簽章」而非「儲存」，所以未驗證的請求不會佔用記憶體。
- Access token 與 refresh token 只存在記憶體並以雜湊保存。Host 停止就結束所有登入，並使 connector 的註冊失效。
- 對 `/mcp` 帶有外來 `Origin` 標頭的請求會被拒絕，避免網頁碰到跑在 loopback 的 host。Anthropic 的請求不帶 `Origin`
  （實際觀察）。
- 預設唯讀：沒有 `--allow-write` 就不提供寫入工具。
- Stderr 的日誌記錄每個請求的 method、跳脫後的路徑、狀態與耗時，以及每次工具呼叫的名稱、結果與耗時；絕不記錄
  query string、參數或 token。

## 限制

- 一個 host 只服務一個站台、一個帳號。任何通過的人都以該帳號行事，且 host 執行期間持有它的 Moodle 憑證。
- Tunnel 供應商會終止 HTTPS，看得到流量；Anthropic 也看得到：工具結果（包含成績與繳交內容）會送給 Claude。
- 工具呼叫一次只處理一個，因為底層 session 不能安全共用。用戶端斷線不會取消呼叫；停止 host 會，且有五秒寬限期。
- 回應是純 JSON。Host 不開事件串流、不保留 session、不接受批次請求，支援 2025-06-18 及更早的修訂版。
- 在 claude.ai 與 Desktop，Claude 限制單次工具結果約 150,000 字元、單次工具呼叫 240 秒。前面的 proxy 可能更早切斷：
  origin 超過 125 秒沒回應時，Cloudflare 會回 `524`，只有 Enterprise 方案能調高。排隊等待輪到的呼叫也算在內。
- Anthropic 的伺服器來自 `160.79.104.0/21`，以位址限制存取時要留意（實際觀察：`160.79.106.164`）。
- Quick tunnel 每次啟動都換位址、沒有可用性保證，且在 host 停止前對整個網際網路開放。所以沒有 `--public-url` 的重啟，
  會讓舊的 connector 指向一個已不存在的位址。
- `cloudflared` 在守衛（guard）底下執行；守衛是 `moodle` 的第二個副本，host 因任何原因結束（包含被強制終止）時，它都會停止
  cloudflared。只有守衛也被殺掉，cloudflared 才會留下；在 Linux 上，那種情況核心也會停止它。守衛在程序清單中顯示為
  `moodle --moodle-tunnel-guard …`。
- 登入不會跨重啟保留。任何重啟（包含為了加上 `--allow-write`，或因而產生新的 passphrase）都會使 connector 失效：
  請在 Claude 移除後重新新增。
- Refresh token 只能用一次，所以重送一次回應已遺失的 refresh 的用戶端，必須重新登入。若重送發生在十秒之後，整個登入都會結束，而不只是那個 token。
- Host 執行期間 Moodle 憑證若過期，工具結果會說明需要由人做什麼。儲存的 web service token 不必重啟就會重新讀取；
  瀏覽器 session（無論是儲存的或來自 `MOODLE_SESSION`）與 `MOODLE_WS_TOKEN` 只在 host 啟動時讀一次，所以要換成新值就得
  重啟，而重啟會讓 connector 登出（見上）。

## 疑難排解

| 現象 | 原因 |
| --- | --- |
| 「cannot start a Cloudflare quick tunnel」 | 沒有安裝 `cloudflared`、Cloudflare 正在限制 quick tunnel（錯誤 1015）、網路擋住它（埠 7844），或 tunnel 在 60 秒內沒有就緒。錯誤訊息會附上它最後的輸出。請安裝它，或傳入 `--public-url` 並自備 tunnel 或 proxy。 |
| 「the Cloudflare quick tunnel does not lead to this host」 | 該位址有回應，但回的不是這個 host 的登入 metadata，表示有東西把 tunnel 導向別處。Host 不會公告它。請傳入 `--public-url`。 |
| 「the Cloudflare quick tunnel stopped」 | Host 執行期間 `cloudflared` 結束了。請重新啟動 host 取得新位址，並重新新增 connector。 |
| 「a Cloudflare quick tunnel needs --listen on a loopback address」 | `--listen` 指定了萬用位址或網路位址。請用 `127.0.0.1`，或傳入 `--public-url`。 |
| 每次連線開頭出現一次 `POST /mcp 400` | Claude 對較新協定修訂版的探測，屬預期行為（實際觀察）。 |
| 「Couldn't reach the MCP server」 | Tunnel 或 proxy 已停止、防火牆擋了 Anthropic 的範圍、host 在沒有 `--public-url` 的情況下重啟而換了位址，或位址不是剛好以一個 `/mcp` 結尾：只輸入位址、`/mcp/` 或 `/mcp/mcp` 都會得到 `404`，日誌顯示 `POST / 404`、`POST /mcp/ 404` 或 `POST /mcp/mcp 404`。 |
| 「Authorization with the MCP server failed」 | `--public-url` 與輸入 Claude 的位址不同。探索文件必須與 connector URL 完全一致。 |
| 「This link is not valid」 | 新增 connector 之後 host 重啟了，或登入使用的返回位置不是該用戶端註冊過的。請移除 connector 再新增一次。 |
| 「Too many attempts」 | 十分鐘內輸錯五次 passphrase。等視窗結束。 |
| 工具不見了 | Host 是唯讀。以 `--allow-write` 重啟，再移除 connector 後重新新增。 |
| `/mcp` 回 `403` | 請求帶有與公開位址不同的 `Origin` 標頭。 |

參考：Model Context Protocol 的[授權](https://modelcontextprotocol.io/specification/2025-06-18/basic/authorization)與
[傳輸](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)規格（2025-06-18）、
[2026-07-28 的向下相容規則](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning#backward-compatibility-with-initialization-based-versions)，
以及 Claude 關於[建置](https://claude.com/docs/connectors/building)與
[新增](https://claude.com/docs/connectors/custom/add-unlisted) connector 的文件。
