# 測試用 Moodle

本機測試站。用 Docker，每個版本一個
**標準站**（Web Services 開啟）與一個**變體站**（Mobile Web Services 關閉）。

## 用法

```bash
./up.sh v52      # 起 Moodle 5.2（標準站 + 變體站），等就緒後自動佈建
./up.sh v51      # Moodle 5.1
./up.sh v45      # Moodle 4.5 LTS
./down.sh v52            # 停掉，保留資料
./down.sh v52 --purge    # 停掉並刪除資料
```

**一次只起一個版本。** 每個站約 300–500MB，六個一起會吃爆 8GB 的機器。

首次啟動要等 Moodle 安裝（數分鐘）；之後資料存在 volume，重啟很快。

## 站台

| 版本 | 標準站 | 變體站（Mobile WS 關閉） |
|---|---|---|
| 4.5 LTS (`v4.5.12`) | http://localhost:8451 | http://localhost:8452 |
| 5.1 (`v5.1.7`) | http://localhost:8511 | http://localhost:8512 |
| 5.2 (`v5.2.3`) | http://localhost:8521 | http://localhost:8522 |

帳號：`admin` / `Admin123!`、`teacher1` / `Teacher123!`、`student1` / `Student123!`

## 佈建內容

- 課程 `CS204`（Operating Systems），teacher1 為教師、student1 為學生。
- 三個作業，對應三種不同的提交流程：

  | 作業 | `submissiondrafts` | `requiresubmissionstatement` | 意義 |
  |---|---|---|---|
  | A1 direct submit | 0 | 0 | 儲存即算提交 |
  | A2 submit button | 1 | 0 | 需要按「提交評分」 |
  | A3 statement | 1 | 1 | 需要同意提交聲明 |

- 標準站：`enablewebservices`、`enablemobilewebservice`、`rest` 協定、mobile service 已啟用，
  且「已驗證使用者」角色已授予 `webservice/rest:use`。

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
8. **SQLite 上 moosh 會撞 `mdl_sessions.sid` 的唯一鍵。** moosh 反覆啟動會累積 session 列，
   之後的呼叫全部失敗（Moodle 4.5 實測會讓 `course-enrol`、`activity-add` 整批無聲失敗，
   最後只建出空課程）。`seed.sh` 因此在每次 moosh 前跑 `admin/cli/kill_all_sessions.php`。

## 已驗證

2026-09-18 實測，三個版本皆端到端通過：

| 版本 | 標準站 | 變體站 | `get_site_info` 函式數 |
|---|---|---|---|
| 4.5.12 LTS | ✅ token + 課程 + 三種作業 | ✅ 回 `enablewsdescription` | 437 |
| 5.1.7 | ✅ token + 課程 + 三種作業 | ✅ 回 `enablewsdescription` | 431 |
| 5.2.3 | ✅ token + 課程 + 三種作業 | ✅ 回 `enablewsdescription` | 429 |

> 同樣的種子資料，三個版本開放的函式數量都不同（437／431／429）。這正是
> 「看函式清單、不看版本號」這個做法的實證。

## 限制

- 這些站是 **HTTP**。QR 登入與 autologin 需要 HTTPS（Moodle 會擋 `httpsrequired`），
  所以 **SSO 相關的登入方式無法在這個矩陣上測**，要另外準備 HTTPS 站台
  （待確認項）。
- SQLite 僅供測試；Moodle 官方不建議正式環境使用。
- 尚未加入 SSO（OAuth2）站台，Phase 5 需要時再補。
