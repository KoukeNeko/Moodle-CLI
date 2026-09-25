# 命令指南

[English](Commands) · [首頁](Home-zh-TW)

完整 flag 以 `moodle <command> --help` 為準。以下多數 ID 都可以直接換成從 Moodle 貼來的 URL。

自動產生的[完整命令參考](Command-Reference-zh-TW)逐一說明所有命令與旗標；
[功能覆蓋](Feature-Coverage-zh-TW)列出三版 registry 的每個 core function。

## 站台與登入

```sh
moodle site add <名稱> <網址>
moodle site list
moodle site use <名稱>
moodle site remove <名稱>

moodle auth methods
moodle auth login [--method token|password|qr|mobilelaunch|browser-session|manual]
moodle auth status
moodle auth logout
moodle auth import-browser [--list-profiles|--store]
moodle auth import-browser --browser safari --store   # macOS Safari
moodle auth import-session --site <name>              # Safari 磁碟檔找不到 session 時隱藏輸入貼上
moodle auth register-handler         # 只支援 Linux
moodle auth handler-status
moodle auth unregister-handler
```

## 依角色 capability 的讀取功能

```sh
moodle doctor
moodle course list
moodle calendar upcoming
moodle grade overview
moodle grade list --course <id|url>
moodle assignment list
moodle assignment show <id|url>
moodle assignment status <id|url>
moodle forum list
moodle forum discussions <id|url>
moodle forum read <id|url>
moodle file download <url>
moodle resolve <url>
```

`doctor` 會說明可用 backend 與缺少的 capability，不假設每個站台都有相同函式。論壇讀取不會把文章
標示為已讀。

## 交作業

```sh
moodle assignment submit <id|url> report.pdf --dry-run
moodle assignment submit <id|url> report.pdf --yes
moodle assignment submit <id|url> report.pdf --draft --yes
```

非草稿流程會上傳、儲存、在需要時執行獨立的「submit for grading」，最後回讀狀態。`--draft` 明確停在
尚未交件的狀態；JSON 的 `handed_in` 才是 Moodle 回報的最終事實。

## Typed core service 與 plugin escape hatch

```sh
moodle api functions --match assign
moodle api call core_enrol_get_users_courses --param userid=4
moodle ws list --version v52
moodle ws describe core_course_update_courses
```

Raw call 仍會經過 safety policy。未審查函式視為寫入且不重試；它是用來操作尚未有高階命令的官方函式，
不是繞過安全模型。

## 自動化與 agent

```sh
moodle version --json
moodle commands --json
moodle schema
moodle schema assignment.submit
moodle mcp serve
moodle mcp serve --allow-write
moodle course list --read-only
```

`--read-only` 會移除寫入命令。MCP 除非明確加上 `--allow-write`，否則維持唯讀。程式使用輸出前請閱讀
[JSON contract](JSON-Contract-zh-TW)。
