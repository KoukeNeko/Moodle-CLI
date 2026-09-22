# 測試用 Moodle

本機測試站。用 Docker，每個版本一個**標準站**（Web Services 開啟）
與一個**變體站**（Mobile Web Services 關閉）。

## 用法

從 repo 根目錄：

```bash
make moodle-up    V=v52   # 起 Moodle 5.2（標準站 + 變體站），等就緒後自動佈建
make moodle-up    V=v51   # Moodle 5.1
make moodle-up    V=v45   # Moodle 4.5 LTS
make moodle-down  V=v52   # 停掉，保留資料
make moodle-purge V=v52   # 停掉並刪除資料
make moodle-status        # 列出目前的測試站
```

`make` 只是薄薄一層；實際進入點是 [`scripts/moodle-env.sh`](../../scripts/moodle-env.sh)
（固定 project name `moodle-cli-e2e`、`docker compose --wait`、失敗時倒出 log），
慣例沿用 [KoukeNeko/taiga-cli](https://github.com/KoukeNeko/taiga-cli) 的
`scripts/test-integration.sh`。

**一次只起一個版本。** 每個站約 300–500MB，六個一起會吃爆 8GB 的機器。

首次啟動要等 Moodle 安裝（數分鐘）；之後資料存在 volume，重啟很快。

## 站台

| 版本 | 標準站 | 變體站（Mobile WS 關閉） |
|---|---|---|
| 4.5 LTS (`v4.5.12`) | http://localhost:8451 | http://localhost:8452 |
| 5.1 (`v5.1.7`) | http://localhost:8511 | http://localhost:8512 |
| 5.2 (`v5.2.3`) | http://localhost:8521 | http://localhost:8522 |

帳號：`admin` / `Admin123!`、`teacher1` / `Teacher123!`、
`student1` / `Student123!`、`student2` / `Student123!`

（交件是不可逆的，所以準備了兩個學生；驗收腳本還會自己再建臨時帳號。）

## 佈建內容

- 課程 `CS204`（Operating Systems），teacher1 為教師、student1 與 student2 為學生。
- 三個作業，對應三種不同的提交流程：

  | 作業 | `submissiondrafts` | `requiresubmissionstatement` | 意義 |
  |---|---|---|---|
  | A1 direct submit | 0 | 0 | 儲存即算提交 |
  | A2 submit button | 1 | 0 | 需要按「提交評分」 |
  | A3 statement | 1 | 1 | 需要同意提交聲明 |

  三個作業都啟用了 file 與 online text 兩個繳交外掛，因為真實課程通常就是這樣，
  而且 Moodle 一次 `save_submission` 會寫入所有啟用的外掛——只送檔案、不送
  online text，會把學生在瀏覽器裡打的字存成空的。這個行為必須測得到。

- 三個作業都有截止日：A1 七天後、A2 十四天後（另有 21 天的 cutoff）、
  **A3 已經過期一天但沒有 cutoff**——Moodle 仍然收件，只是標記遲交。沒有逾期的
  資料，行事曆的 `overdue` 分支驗不到。
- 一個 `type=general` 的「Q&A 討論區」，內含一則教師發的討論與一則學生回覆。
  moosh 建出來的是 `type=news`（公告），學生不能在那裡發言，驗不到真正的討論串。
  名稱故意含 `&`，因為 Moodle 兩個函式對它的跳脫處理不一致。
- A1 掛了一個說明附件 `rubric.txt`：沒有附件，下載流程與 `introattachments`
  的欄位形狀都驗不到。
- student1 的 A1 有一筆 85/100 的成績與評語：沒有已評分的資料，成績功能連
  「已評分」和「未評分」都分不出來，等於沒驗到。
- 標準站：`enablewebservices`、`enablemobilewebservice`、`rest` 協定、mobile service 已啟用，
  且「已驗證使用者」角色已授予 `webservice/rest:use`。

## HTTPS 站（QR 登入、SSO）

```bash
make moodle-up V=tls                 # 產生本機 CA + 起 Moodle 與 TLS 終端
test/e2e/accept-qrlogin.sh
```

QR 登入與 autologin 需要 HTTPS——Moodle 對 http 直接回 `httpsrequired`，所以那些
路徑在上面的站台上驗不到。憑證由 `make-certs.sh` 產生一個只存在於本機的 CA，測試
設 `SSL_CERT_FILE` 去信任它。**工具本身沒有、也不該有「略過憑證檢查」的旗標**：
那種旗標一旦存在就會有人在正式環境用它，而這裡走的是真正的 TLS 驗證路徑
（驗收腳本第一條就是確認不給 CA 時連線會被拒絕）。

`tls/` 不入版控：私鑰不該進 git，每台機器自己產一份。

## 測試用的 OS keychain

憑證只進 OS keychain、設定檔永不存憑證是這個工具的硬規則，而 `moodle auth login`
是唯一會寫憑證的地方。headless 主機上沒有 `org.freedesktop.secrets`，那條路會停在
「無法寫入 keychain」——規則最重要的那一半在測試裡等於沒驗到。

`keyring` 服務補上這一塊：D-Bus session bus 開在 `test/e2e/run/bus`（綁定掛載，
所以主機連得到；容器之間不能共用抽象 socket，unix socket 可以）。它沒有 profile，
因為每個版本都需要，而且只有約 60MB。

金鑰圈以**空密碼**解鎖：拋棄式容器、不對外開放、裡面不會有真的憑證。

```bash
export DBUS_SESSION_BUS_ADDRESS=unix:path=$PWD/test/e2e/run/bus
./bin/moodle auth login --method token --token …
./bin/moodle course list          # 不必再帶任何環境變數
```

`gnome-keyring-daemon` 以前景模式跑，由 entrypoint 的 `wait` 顧著：它一死 shell
就結束、容器結束，`restart: unless-stopped` 把它拉起來。healthcheck 問的是
「`org.freedesktop.secrets` 有主嗎」，不是「bus 活著嗎」——金鑰圈死掉時 bus 還在，
只檢查 bus 會讓容器一直顯示 healthy。

## 十年資料生命週期

專案不只測「剛開完的新站」。十年情境把一位教師連續十屆的同名課程、
每屆不同的學生、封存與現行課程、學生六年學習歷程，以及跨角色資料疊在同一座站上。

```bash
make moodle-up V=v52
make moodle-decade V=v52
```

驗收會同時問兩邊：容器內 Moodle 資料庫是 control plane，CLI JSON 契約是受測系統。
目前固定情境包含：

- 10 屆同名但不同 ID 的課程（8 門封存、2 門現行）。
- 30 名跨屆學生、20 份作業與 60 筆提交，防止清單截斷或同名去重。
- 學生→研究生→助教的跨角色帳號，另含教師、類別管理者、開課者與零選課帳號。
- 重修與同名課、停權選課、隱藏課、他人課程的權限拒絕。
- 草稿、已交、遲交、截止後拒收、需提交聲明、零分與未評分。
- 檔案附件、線上文字、討論串、行事曆時間視窗與歷史成績。

`decade-run.sh` 可重複執行；fixture 會收斂到當年應有的「最近兩年」狀態，
不會因跨年重跑而讓舊課程永遠留在現行區。

## 學生與助教生命週期：全功能逐字紀錄

```bash
make moodle-up V=v52
test/e2e/seed-masters.sh                  # 四學期的情境資料（可重複執行）
test/e2e/full-run.sh                      # 標準站
test/e2e/full-run.sh --nows               # 連 Mobile WS 關閉的變體站也跑
```

情境是一位四學期的碩士生，**後兩學期同時是大學部課程的助教**——同一個帳號在不同
課程有不同角色。單一功能的測試不會碰到那種組合，而那正是會出錯的地方：助教在
CS1001 看得到別人的繳交，但沒有 `mod/assign:viewownsubmissionsummary`，Moodle 於是
完全不回 `lastattempt` 那個鍵。

八個標準角色裡，這裡跑到六個：`student`（含大學部與研究生）、`teacher`（助教）、
`editingteacher`、`user`（零選課的新帳號）、`manager`、`coursecreator`，外加站台管理員。

另外兩個**刻意不跑**，因為它們不是能帶著憑證登入的身分：

- `guest` 是未登入的訪客。這支工具一律帶著憑證發問，所以那個情境對應的是「完全沒有
  憑證」，見第 20 節。
- `frontpage` 只決定登入者在站台首頁看到什麼，而這裡的每個命令都以課程為範圍。

紀錄寫到 `test/e2e/logs/<時間>/`：

- `transcript.log` — 每個命令的完整命令列、stdout、stderr 與結束碼
- `summary.tsv` — `結束碼 → 命令` 的清單，用來核對每一個非零結束碼是不是刻意的

憑證經過 `redact()`：這些站台的密碼是公開的，但「紀錄裡不該出現 token」這件事本身
不該因為站台是測試站就破例。

腳本**不判斷對錯**，它產生的是可讀的證據；判斷留給讀的人與上面的驗收腳本。

## 瀏覽器 fixture

`internal/browser` 讀 Firefox 自己寫的檔案，所以 fixture 必須由 Firefox 產生。
怎麼在沒有桌面的機器上做到（headless 就夠），見 [firefox.md](firefox.md)。

## 語意範圍與範圍突變

欄位稽核（`scripts/audit-optional-fields.sh`）查的是「欄位在不在」。
另一類缺陷是「這支函式在回傳 `[]` 之前搜尋的宇宙是什麼」——空回應能不能支撐
一句全稱否定。那一份表在 [semantic-scope.md](semantic-scope.md)。

對應的工具保留物件本身、只動一個縮減維度：

```sh
test/e2e/mutate.sh list
test/e2e/mutate.sh apply  suspend-enrolment
test/e2e/mutate.sh revert suspend-enrolment
```

受測的性質**不是**「端點應該還是要回傳這個物件」——通常它正確地不該回傳。
而是：端點不再回傳一個確實存在的物件時，CLI 不得把那個觀測強化成更廣的
不存在宣稱。

## 交作業流程的驗收

```bash
make moodle-up V=v52
test/e2e/accept-assignment.sh 8521 "$(docker compose --project-name moodle-cli-e2e \
  --file test/e2e/docker-compose.yml ps -q v52-std)"
```

它會建一個全新的學生（交件不可逆），然後對三種作業設定各跑一遍：dry run 不留下痕跡、
A1 存檔即提交、A2 存成草稿後仍是草稿、A3 沒有 `--accept-statement` 就拒絕，
以及重複交件回報衝突（exit 8）。

## 取得 token 並試打 API

```bash
TOKEN=$(curl -s http://localhost:8521/login/token.php \
  -d username=student1 -d password='Student123!' -d service=moodle_mobile_app \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["token"])')

curl -s http://localhost:8521/webservice/rest/server.php \
  -d wstoken=$TOKEN -d wsfunction=core_webservice_get_site_info -d moodlewsrestformat=json
```

## 架設過程踩到的坑（都已在設定檔處理）

1. **`REVERSEPROXY=true` 是必要的。** 容器內 nginx 聽 8080，對外映射到別的 port。
   沒有它，Moodle 用 `SERVER_PORT`(8080) 比對 `wwwroot` 的 port，`/login/token.php`
   直接回 `requirecorrectaccess`（"Invalid url or port"）。
2. **`enablemobilewebservice=1` 不會啟用 mobile service 那一列。**
   `admin/cli/cfg.php` 只寫設定值；還要把 `external_services` 裡 `moodle_mobile_app`
   的 `enabled` 設成 1，否則 token 請求回 `servicenotavailable`。
3. **「已驗證使用者」角色預設沒有 `webservice/rest:use`。** 少了它，token 發得出來，
   但呼叫 REST 會得到 `accessexception`。
4. **image 內建的 blueprint runner 不能用。** 它在 5.2 的 `public/` 佈局下會去找
   `/var/www/html/public/admin/cli/cfg.php`（實際在 `/var/www/html/admin/cli/cfg.php`），
   而且不支援建立作業。所以改用 `seed.sh` + `seed.php`。
5. **`moosh activity-add` 的 `-o` 選項不會套用到 assign 設定欄位。**
   三個作業的參數由 `seed.php` 用 Moodle API 設定。
6. **moosh 會把除錯 backtrace 印到 stderr**（它強制開發者除錯模式；例如
   noemailever 的提示）。那是雜訊不是錯誤，`seed.sh` 把 stderr 導到
   `/tmp/moodle-seed-<container>.log`，並以實際查詢結果作為成功判準。
7. **改設定後要清快取**，否則 PHP opcache 與 Moodle 的服務／權限快取會讓修改看似無效。
8. **反向代理不可以轉發客戶端的 Host。** Moodle 在 `reverseproxy` 模式下要求 Host
   指向伺服器的**內部**名稱；收到與 `wwwroot` 相同的 Host 時，它判定有人繞過 proxy
   直連，丟出 `reverseproxyabused`。另外 `REVERSEPROXY` 只管 host/port，scheme 要
   靠 `SSLPROXY`，少了它 AJAX 呼叫會回 `unsupportedredirect`。
9. **SQLite 上 moosh 會撞 `mdl_sessions.sid` 的唯一鍵。** moosh 反覆啟動會累積 session 列，
   之後的呼叫全部失敗（Moodle 4.5 實測會讓 `course-enrol`、`activity-add` 整批無聲失敗，
   最後只建出空課程）。`seed.sh` 因此在每次 moosh 前跑 `admin/cli/kill_all_sessions.php`。
10. **`moosh course-enrol` 會只做一半。** Moodle 4.5 實測：`user_enrolments` 寫進去了，
   `role_assignments` 卻沒有，而且 moosh 仍然回非零。那樣的站台學生連 `mod/assign:view`
   都沒有，作業列表是空的，但每張表看起來都「有資料」，極難查。選課因此改由
   `seed.php` 用 Moodle API 做，驗證關卡也改看角色指派數而不是選課數。
11. **選課起始日不能是「現在」。** Moodle 只認已經開始的選課，剛好在這一秒開始的會被
    當成還沒生效，佈建完立刻查就會看到一門課都沒有。`seed.php` 一律往前挪一天。
12. **`assign.nosubmissions` 要歸零。** moosh 建作業時沒有啟用任何繳交外掛，那一列就被
    標成「不收繳交」。這時 `mod_assign_get_submission_status` 回的是 `nopermission`，
    而不是 `submissionsenabled=false`——訊息完全指向錯誤的方向（權限），查很久。
13. **nginx 的健康檢查要用 `127.0.0.1`，不是 `localhost`。** 容器裡 `localhost` 先解析到
    `::1`，而 nginx 只聽 IPv4，於是 wget 拿到 Connection refused、`tls-proxy` 永遠顯示
    unhealthy。一直顯示 unhealthy 的健康檢查比沒有還糟：它教人忽略那個欄位，而 QR 登入
    其實是好的。

## 已驗證

2026-09-18 實測，三個版本皆端到端通過：

| 版本 | 標準站 | 變體站 | `get_site_info` 函式數 |
|---|---|---|---|
| 4.5.12 LTS | ✅ token + 課程 + 三種作業提交 | ✅ 回 `enablewsdescription` | 437 |
| 5.1.7 | ✅ token + 課程 + 三種作業提交 | ✅ 回 `enablewsdescription` | 431 |
| 5.2.3 | ✅ token + 課程 + 三種作業提交 | ✅ 回 `enablewsdescription` | 429 |

「三種作業提交」是 `accept-assignment.sh` 在乾淨佈建上跑完全綠，不只是能列出作業。

> 同樣的種子資料，三個版本開放的函式數量都不同（437／431／429）。這正是
> 「看函式清單、不看版本號」這個做法的實證。

## 限制

- 這些站是 **HTTP**。QR 登入與 autologin 需要 HTTPS（Moodle 會擋 `httpsrequired`），
  所以 **SSO 相關的登入方式無法在這個矩陣上測**，要另外準備 HTTPS 站台
  。
- SQLite 僅供測試；Moodle 官方不建議正式環境使用。
- 尚未加入 SSO（OAuth2）站台，Phase 5 需要時再補。
