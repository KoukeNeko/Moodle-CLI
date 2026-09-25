# 完整命令參考

[English](Command-Reference) · [繁體中文](Command-Reference-zh-TW)

本頁由 `moodle commands --json` 自動產生；每個公開命令都必須出現在此。位置參數以 synopsis 為準；網站、角色或 capability 不允許時，Moodle 會回傳明確錯誤。

## 全域旗標

| Flag | 說明 |
| --- | --- |
| `--json` | 將版本化 JSON contract 寫到 stdout。 |
| `--pretty` | 縮排 JSON；與 --json 一起使用。 |
| `--read-only` | 隱藏並拒絕所有可能變更 Moodle 的命令。 |
| `--backend auto|ws-only` | 允許自動 fallback，或將本次執行限制為 Web Services。 |

## 命令 (106)

### `moodle api`

直接操作站台 Web Service 函式。

- 用法: `moodle api [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle api call`

直接操作站台 Web Service 函式：呼叫函式。

- 用法: `moodle api call <function> [flags]`
- JSON 輸出 kind: `api.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--allow-write` | 允許呼叫已知寫入函式 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle api functions`

直接操作站台 Web Service 函式：列出帳號可呼叫的函式。

- 用法: `moodle api functions [flags]`
- JSON 輸出 kind: `api.functions`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--match` | 只保留名稱包含此文字的項目 |
| `--site` | 指定 Moodle 站台 |
| `--unreviewed` | 只顯示尚未審查的函式 |
| `--writes` | 只顯示可能變更資料的函式 |

### `moodle assignment`

查看、提交與管理作業。

- 用法: `moodle assignment [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle assignment extend`

查看、提交與管理作業：設定繳交展延期限。

- 用法: `moodle assignment extend [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle assignment grade`

查看、提交與管理作業：評分。

- 用法: `moodle assignment grade [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle assignment list`

查看、提交與管理作業：列出項目。

- 用法: `moodle assignment list [flags]`
- JSON 輸出 kind: `assignment.list`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--course` | 限制或指定課程 |
| `--current` | only courses running now: started, and not yet past their end date |
| `--site` | 指定 Moodle 站台 |

### `moodle assignment lock`

查看、提交與管理作業：鎖定。

- 用法: `moodle assignment lock [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle assignment reveal-identities`

查看、提交與管理作業：揭露匿名評分身分。

- 用法: `moodle assignment reveal-identities [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle assignment revert`

查看、提交與管理作業：退回草稿。

- 用法: `moodle assignment revert [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle assignment show`

查看、提交與管理作業：顯示詳細資料。

- 用法: `moodle assignment show <assignment-id|url> [flags]`
- JSON 輸出 kind: `assignment.show`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--site` | 指定 Moodle 站台 |

### `moodle assignment status`

查看、提交與管理作業：顯示目前狀態。

- 用法: `moodle assignment status <assignment-id|url> [flags]`
- JSON 輸出 kind: `assignment.status`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--site` | 指定 Moodle 站台 |

### `moodle assignment submissions`

查看、提交與管理作業：列出作業繳交。

- 用法: `moodle assignment submissions [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle assignment submit`

查看、提交與管理作業：提交作業並回讀狀態。

- 用法: `moodle assignment submit <assignment-id|url> <file>... [flags]`
- JSON 輸出 kind: `assignment.submit`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--accept-statement` | record that you accept this assignment's submission statement |
| `--account` | 指定代為操作的帳號 |
| `--draft` | save the work without handing it in for grading |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle assignment unlock`

查看、提交與管理作業：解除鎖定。

- 用法: `moodle assignment unlock [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle auth`

登入 Moodle、管理憑證並檢查目前身分。

- 用法: `moodle auth [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle auth handler-status`

登入 Moodle、管理憑證並檢查目前身分：檢查瀏覽器登入 handler。

- 用法: `moodle auth handler-status [flags]`
- JSON 輸出 kind: `auth.handler`
- 資料效果: **唯讀**

### `moodle auth import-browser`

登入 Moodle、管理憑證並檢查目前身分：匯入既有瀏覽器 session。

- 用法: `moodle auth import-browser [flags]`
- JSON 輸出 kind: `auth.import_browser`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--browser` | 選擇 safari、firefox 或 chromium；未指定時搜尋現有 profile |
| `--cookie-name` | 站台若改過 session cookie 名稱，可在此指定（預設 MoodleSession） |
| `--list-profiles` | 只列出此電腦找到的瀏覽器 profile |
| `--profile` | 指定瀏覽器 profile 目錄或 Safari cookie 檔案 |
| `--site` | 指定 Moodle 站台 |
| `--store` | 驗證後將 session 存入作業系統鑰匙圈 |

### `moodle auth import-session`

登入 Moodle、管理憑證並檢查目前身分：從隱藏輸入匯入並驗證瀏覽器 session。

- 用法: `moodle auth import-session [flags]`
- JSON 輸出 kind: `auth.login`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--cookie-name` | 站台若改過 session cookie 名稱，可在此指定（預設 MoodleSession） |
| `--site` | 指定 Moodle 站台 |
| `--stdin` | 從標準輸入讀取單一 session cookie；未指定時使用隱藏輸入提示 |

### `moodle auth login`

登入 Moodle、管理憑證並檢查目前身分：登入並安全保存憑證。

- 用法: `moodle auth login [flags]`
- JSON 輸出 kind: `auth.login`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--callback` | a pasted <scheme>://token=... callback URL (manual method) |
| `--method` | login method: token, password, qr, mobilelaunch, browser-session or manual |
| `--passport` | the passport used to start the login, so the callback can be verified |
| `--password-stdin` | read the password from stdin |
| `--qr` | the decoded content of a login QR code (qr method) |
| `--session-cookie` | a session your browser already holds, as MoodleSession=… (browser-session method) |
| `--site` | 指定 Moodle 站台 |
| `--token` | an existing web service token |
| `--token-stdin` | read the token from stdin |
| `--username` | Moodle username (password method) |

### `moodle auth logout`

登入 Moodle、管理憑證並檢查目前身分：刪除本機保存的憑證。

- 用法: `moodle auth logout [flags]`
- JSON 輸出 kind: `auth.logout`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--site` | 指定 Moodle 站台 |

### `moodle auth methods`

登入 Moodle、管理憑證並檢查目前身分：列出可用方法。

- 用法: `moodle auth methods [flags]`
- JSON 輸出 kind: `auth.methods`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--site` | 指定 Moodle 站台 |

### `moodle auth register-handler`

登入 Moodle、管理憑證並檢查目前身分：安裝瀏覽器登入 handler。

- 用法: `moodle auth register-handler [flags]`
- JSON 輸出 kind: `auth.handler`
- 資料效果: **寫入**

### `moodle auth status`

登入 Moodle、管理憑證並檢查目前身分：顯示目前狀態。

- 用法: `moodle auth status [flags]`
- JSON 輸出 kind: `auth.status`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--site` | 指定 Moodle 站台 |

### `moodle auth unregister-handler`

登入 Moodle、管理憑證並檢查目前身分：移除瀏覽器登入 handler。

- 用法: `moodle auth unregister-handler [flags]`
- JSON 輸出 kind: `auth.handler`
- 資料效果: **寫入**

### `moodle calendar`

查看與管理行事曆事件及待辦事項。

- 用法: `moodle calendar [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle calendar create`

查看與管理行事曆事件及待辦事項：建立。

- 用法: `moodle calendar create [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle calendar delete`

查看與管理行事曆事件及待辦事項：刪除。

- 用法: `moodle calendar delete [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle calendar upcoming`

查看與管理行事曆事件及待辦事項：List deadlines and to-dos, overdue work included。

- 用法: `moodle calendar upcoming [flags]`
- JSON 輸出 kind: `calendar.upcoming`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--days` | how many days ahead to look; 0 for everything Moodle offers |
| `--limit` | maximum number of events to return |
| `--site` | 指定 Moodle 站台 |

### `moodle calendar update`

查看與管理行事曆事件及待辦事項：更新。

- 用法: `moodle calendar update [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle commands`

列出每個 CLI 命令、旗標、輸出 kind 與寫入屬性，供文件、script 與 agent 使用。

- 用法: `moodle commands [flags]`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `-h, --help` | help for commands |

### `moodle completion`

查看及更新課程／活動完成狀態。

- 用法: `moodle completion [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle completion activity`

查看及更新課程／活動完成狀態：顯示活動完成狀態。

- 用法: `moodle completion activity [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle completion course`

查看及更新課程／活動完成狀態：顯示課程完成狀態。

- 用法: `moodle completion course [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle completion mark`

查看及更新課程／活動完成狀態：手動標記完成。

- 用法: `moodle completion mark [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle course`

依 Moodle 角色允許的範圍讀取與管理課程。

- 用法: `moodle course [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle course contents`

依 Moodle 角色允許的範圍讀取與管理課程：顯示課程章節、活動與檔案。

- 用法: `moodle course contents [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle course create`

依 Moodle 角色允許的範圍讀取與管理課程：建立。

- 用法: `moodle course create [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle course delete`

依 Moodle 角色允許的範圍讀取與管理課程：刪除。

- 用法: `moodle course delete [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle course list`

依 Moodle 角色允許的範圍讀取與管理課程：列出項目。

- 用法: `moodle course list [flags]`
- JSON 輸出 kind: `course.list`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--current` | only courses running now: started, and not yet past their end date |
| `--cursor` | continue a previous listing |
| `--limit` | maximum number of courses to return |
| `--site` | 指定 Moodle 站台 |

### `moodle course search`

依 Moodle 角色允許的範圍讀取與管理課程：搜尋。

- 用法: `moodle course search [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle course show`

依 Moodle 角色允許的範圍讀取與管理課程：顯示詳細資料。

- 用法: `moodle course show [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle course update`

依 Moodle 角色允許的範圍讀取與管理課程：更新。

- 用法: `moodle course update [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle doctor`

診斷站台、登入、backend 與 capability，並說明不可用的原因。

- 用法: `moodle doctor [flags]`
- JSON 輸出 kind: `doctor`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--site` | 指定 Moodle 站台 |

### `moodle enrolment`

查看選課方法並管理使用者選課。

- 用法: `moodle enrolment [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle enrolment add`

查看選課方法並管理使用者選課：新增。

- 用法: `moodle enrolment add [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle enrolment methods`

查看選課方法並管理使用者選課：列出可用方法。

- 用法: `moodle enrolment methods [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle enrolment remove`

查看選課方法並管理使用者選課：移除。

- 用法: `moodle enrolment remove [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle enrolment update`

查看選課方法並管理使用者選課：更新。

- 用法: `moodle enrolment update [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle file`

下載 Moodle 所保存的檔案。

- 用法: `moodle file [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle file download`

下載 Moodle 所保存的檔案：下載。

- 用法: `moodle file download <url> [flags]`
- JSON 輸出 kind: `file.download`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--as` | save under this name instead of the one the site suggests |
| `--dir` | directory to save into (default: the working directory) |
| `--force` | replace a file that is already there |
| `--site` | 指定 Moodle 站台 |

### `moodle forum`

讀取與管理討論區、主題及貼文。

- 用法: `moodle forum [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle forum create`

讀取與管理討論區、主題及貼文：建立。

- 用法: `moodle forum create [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle forum delete`

讀取與管理討論區、主題及貼文：刪除。

- 用法: `moodle forum delete [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle forum discussions`

讀取與管理討論區、主題及貼文：列出討論主題。

- 用法: `moodle forum discussions <forum-id|url> [flags]`
- JSON 輸出 kind: `forum.discussions`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--site` | 指定 Moodle 站台 |

### `moodle forum edit`

讀取與管理討論區、主題及貼文：Edit a discussion post。

- 用法: `moodle forum edit [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle forum favourite`

讀取與管理討論區、主題及貼文：加入最愛。

- 用法: `moodle forum favourite [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle forum list`

讀取與管理討論區、主題及貼文：列出項目。

- 用法: `moodle forum list [flags]`
- JSON 輸出 kind: `forum.list`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--course` | 限制或指定課程 |
| `--site` | 指定 Moodle 站台 |

### `moodle forum lock`

讀取與管理討論區、主題及貼文：鎖定。

- 用法: `moodle forum lock [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle forum pin`

讀取與管理討論區、主題及貼文：置頂。

- 用法: `moodle forum pin [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle forum read`

讀取與管理討論區、主題及貼文：讀取。

- 用法: `moodle forum read <discussion-id|url> [flags]`
- JSON 輸出 kind: `forum.thread`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--site` | 指定 Moodle 站台 |

### `moodle forum reply`

讀取與管理討論區、主題及貼文：回覆。

- 用法: `moodle forum reply [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle forum subscribe`

讀取與管理討論區、主題及貼文：訂閱。

- 用法: `moodle forum subscribe [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle forum unfavourite`

讀取與管理討論區、主題及貼文：移出最愛。

- 用法: `moodle forum unfavourite [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle forum unlock`

讀取與管理討論區、主題及貼文：解除鎖定。

- 用法: `moodle forum unlock [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle forum unpin`

讀取與管理討論區、主題及貼文：取消置頂。

- 用法: `moodle forum unpin [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle forum unsubscribe`

讀取與管理討論區、主題及貼文：取消訂閱。

- 用法: `moodle forum unsubscribe [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle grade`

讀取與管理成績及成績簿分類。

- 用法: `moodle grade [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle grade category-create`

讀取與管理成績及成績簿分類：建立成績簿分類。

- 用法: `moodle grade category-create [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle grade list`

讀取與管理成績及成績簿分類：列出項目。

- 用法: `moodle grade list [flags]`
- JSON 輸出 kind: `grade.list`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--course` | 限制或指定課程 |
| `--site` | 指定 Moodle 站台 |

### `moodle grade overview`

讀取與管理成績及成績簿分類：顯示總覽。

- 用法: `moodle grade overview [flags]`
- JSON 輸出 kind: `grade.overview`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--site` | 指定 Moodle 站台 |

### `moodle grade update`

讀取與管理成績及成績簿分類：更新。

- 用法: `moodle grade update [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle group`

管理課程群組及成員。

- 用法: `moodle group [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle group create`

管理課程群組及成員：建立。

- 用法: `moodle group create [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle group delete`

管理課程群組及成員：刪除。

- 用法: `moodle group delete [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle group list`

管理課程群組及成員：列出項目。

- 用法: `moodle group list [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle group member-add`

管理課程群組及成員：加入群組成員。

- 用法: `moodle group member-add [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle group member-remove`

管理課程群組及成員：移除群組成員。

- 用法: `moodle group member-remove [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle group update`

管理課程群組及成員：更新。

- 用法: `moodle group update [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle mcp`

以 Model Context Protocol 將 Moodle 提供給 agent。

- 用法: `moodle mcp [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle mcp serve`

以 Model Context Protocol 將 Moodle 提供給 agent：在 stdin/stdout 執行 MCP server。

- 用法: `moodle mcp serve [flags]`
- JSON 輸出 kind: `mcp.serve`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--allow-write` | 允許呼叫已知寫入函式 |
| `--site` | 指定 Moodle 站台 |

### `moodle participant`

搜尋與查看課程參與者。

- 用法: `moodle participant [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle participant list`

搜尋與查看課程參與者：列出項目。

- 用法: `moodle participant list [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle participant search`

搜尋與查看課程參與者：搜尋。

- 用法: `moodle participant search [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle participant show`

搜尋與查看課程參與者：顯示詳細資料。

- 用法: `moodle participant show [flags]`
- JSON 輸出 kind: `workflow.call`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle quiz`

See your quizzes and how your attempts went。

- 用法: `moodle quiz [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle quiz list`

quiz：列出項目。

- 用法: `moodle quiz list [flags]`
- JSON 輸出 kind: `quiz.list`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--course` | 限制或指定課程 |
| `--current` | only courses running now: started, and not yet past their end date |
| `--site` | 指定 Moodle 站台 |

### `moodle quiz show`

quiz：顯示詳細資料。

- 用法: `moodle quiz show <quiz-id|url> [flags]`
- JSON 輸出 kind: `quiz.show`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--site` | 指定 Moodle 站台 |

### `moodle resolve`

解析 Moodle URL 所指向的資源種類與識別碼。

- 用法: `moodle resolve <url> [flags]`
- JSON 輸出 kind: `resolve`
- 資料效果: **唯讀**

### `moodle schema`

列出或輸出指定 JSON response kind 的 schema。

- 用法: `moodle schema [kind] [flags]`
- 資料效果: **唯讀**

### `moodle site`

管理 CLI 已知的 Moodle 站台與學務設定。

- 用法: `moodle site [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle site academic`

管理 CLI 已知的 Moodle 站台與學務設定：管理學務欄位設定。

- 用法: `moodle site academic [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle site academic configure`

管理 CLI 已知的 Moodle 站台與學務設定：設定欄位對應與最低學分。

- 用法: `moodle site academic configure [flags]`
- JSON 輸出 kind: `site.academic.configure`
- 資料效果: **寫入**

| Flag | 說明 |
| --- | --- |
| `--credits-field` | course custom-field shortname containing credits |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--graduate-minimum` | minimum graduate credits per term |
| `--level-field` | course custom-field shortname containing undergraduate or graduate |
| `--site` | 指定 Moodle 站台 |
| `--term-field` | course custom-field shortname containing the academic term |
| `--undergraduate-minimum` | minimum undergraduate credits per term |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle site add`

登記一個 Moodle 站台。

- 用法: `moodle site add <name> <url> [flags]`
- JSON 輸出 kind: `site.add`
- 資料效果: **唯讀**

### `moodle site inspect`

檢查此帳號在站台可用的 capability 與 external functions。

- 用法: `moodle site inspect [flags]`
- JSON 輸出 kind: `site.inspect`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--functions` | include the full function list |
| `--site` | 指定 Moodle 站台 |

### `moodle site list`

管理 CLI 已知的 Moodle 站台與學務設定：列出項目。

- 用法: `moodle site list [flags]`
- JSON 輸出 kind: `site.list`
- 資料效果: **唯讀**

### `moodle site remove`

移除站台設定及其本機憑證。

- 用法: `moodle site remove <name> [flags]`
- JSON 輸出 kind: `site.remove`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--yes` | 確認執行 Moodle 寫入 |

### `moodle site use`

管理 CLI 已知的 Moodle 站台與學務設定：設為預設站台。

- 用法: `moodle site use <name> [flags]`
- JSON 輸出 kind: `site.use`
- 資料效果: **唯讀**

### `moodle version`

輸出 CLI 版本、commit 與建置時間。

- 用法: `moodle version [flags]`
- 資料效果: **唯讀**

### `moodle workload`

依課程自訂欄位計算並驗證每學期學分。

- 用法: `moodle workload [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle workload show`

依學期顯示課程、學分、學制及適用最低學分。

- 用法: `moodle workload show [flags]`
- JSON 輸出 kind: `workload.show`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--site` | 指定 Moodle 站台 |
| `--term` | only this academic term |

### `moodle workload validate`

驗證各學期是否符合大學部或研究所最低學分。

- 用法: `moodle workload validate [flags]`
- JSON 輸出 kind: `workload.validate`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--require-minimum` | 未達最低學分時回傳 validation exit code |
| `--site` | 指定 Moodle 站台 |
| `--term` | only this academic term |

### `moodle ws`

檢視並呼叫具型別的 Moodle core Web Service registry。

- 用法: `moodle ws [command]`
- 類型：命令群組，請選擇子命令
- 資料效果: **唯讀**

### `moodle ws call`

檢視並呼叫具型別的 Moodle core Web Service registry：呼叫函式。

- 用法: `moodle ws call <function> [flags]`
- JSON 輸出 kind: `ws.call`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--account` | 指定代為操作的帳號 |
| `--allow-write` | 允許呼叫已知寫入函式 |
| `--dry-run` | 驗證並顯示操作，但不送出 |
| `--param` | 加入一個 name=value 參數，可重複 |
| `--params-json` | 以 JSON object 提供巢狀參數 |
| `--site` | 指定 Moodle 站台 |

### `moodle ws describe`

檢視並呼叫具型別的 Moodle core Web Service registry：顯示版本化 schema、effect 與需求。

- 用法: `moodle ws describe <function> [flags]`
- JSON 輸出 kind: `ws.describe`
- 資料效果: **唯讀**

### `moodle ws list`

離線列出 Moodle 4.5、5.1、5.2 core external functions 聯集。

- 用法: `moodle ws list [flags]`
- JSON 輸出 kind: `ws.list`
- 資料效果: **唯讀**

| Flag | 說明 |
| --- | --- |
| `--component` | only this Moodle component |
| `--credential` | only functions that issue or change credentials |
| `--deprecated` | only functions deprecated in at least one version |
| `--destructive` | only destructive writes |
| `--effect` | only read or write functions |
| `--match` | 只保留名稱包含此文字的項目 |
| `--version` | 指定 registry 的 Moodle 版本 |
