<h1 align="center">Moodle CLI</h1>

<p align="center">
  <strong>為學生設計的獨立 Moodle 命令列客戶端。</strong><br>
  給人閱讀的終端輸出，以及給 script、CI 與 agent 使用的版本化 contract。
</p>

<p align="center">
  <a href="https://github.com/KoukeNeko/Moodle-CLI/actions/workflows/ci.yml"><img alt="CI status" src="https://img.shields.io/github/actions/workflow/status/KoukeNeko/Moodle-CLI/ci.yml?branch=main&style=for-the-badge&logo=githubactions&logoColor=white&label=CI"></a>
  <a href="#相容性"><img alt="Verified with Moodle 4.5.12, 5.1.7, and 5.2.3" src="https://img.shields.io/badge/MOODLE-4.5.12%20%7C%205.1.7%20%7C%205.2.3-FF8B00?style=for-the-badge&logo=moodle&logoColor=white"></a>
  <a href="https://go.dev/"><img alt="Go 1.26 or newer" src="https://img.shields.io/badge/GO-1.26%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
  <a href="LICENSE"><img alt="MIT license" src="https://img.shields.io/badge/LICENSE-MIT-4CAF50?style=for-the-badge&logo=github"></a>
</p>

<p align="center">
  <a href="README.md">English</a> · <strong>繁體中文</strong>
</p>

<p align="center">
  <a href="#快速開始">快速開始</a>
  · <a href="https://github.com/KoukeNeko/Moodle-CLI/wiki/Home-zh-TW">使用手冊</a>
  · <a href="test/e2e/README.md">測試環境</a>
  · <a href="docs/architecture.md">架構</a>
</p>

```sh
moodle course list
moodle calendar upcoming
moodle assignment show 42
moodle assignment submit 42 report.pdf --dry-run
```

Moodle CLI 連線到你指定的 Moodle 站台，優先使用官方 Web Service API；站台沒有開放所需函式時，
才回退到唯讀 AJAX 或 HTML adapter。它處理課程、作業、行事曆、成績、論壇與檔案，也不假設每個
Moodle 安裝都具備相同能力。

同一支 binary 同時服務人與程式。一般輸出保持好讀；`--json` 輸出穩定的 `schema_version: 1`
envelope，命令提供 JSON Schema，錯誤則有固定 exit code。同一套 use case 也提供 MCP server；除非明確
啟用，會寫入的工具不會出現在工具清單。

## 與眾不同之處

### 依站台實際能力登入

Moodle CLI 支援既有 token、Moodle 帳密、登入 QR 資料、瀏覽器 session，以及 Moodle mobile launch
流程。Linux 桌面可以用每位使用者自己的 D-Bus URL handler，開啟平常使用的瀏覽器並自動接回結果；
學校 SSO、passkey 與 MFA 都留在瀏覽器內完成。macOS 與 Windows 目前使用手動 callback 方式。

匯入瀏覽器 session 必須明確執行：`moodle auth import-browser` 只在 Firefox 或 Chromium 系瀏覽器
profile 中尋找指定站台的 session，不會成為登入時暗中發生的副作用。

### 真的交作業，不只是上傳檔案

作業提交完整執行 Moodle 的學生流程：上傳檔案、儲存 submission、在作業要求時另外送出評分，最後再
向 Moodle 回讀狀態。HTTP 成功不等於作業已交件。如果寫入回應遺失、又無法確認最終狀態，命令會用
exit code `12` 與 `ambiguous` outcome 結束，不會猜測或盲目重送。

### 適合自動化的穩定 contract

```sh
moodle version --json
moodle commands --json
moodle schema assignment.submit
moodle course list --json
```

所有 JSON 回應共用同一個版本化 envelope。穩定的錯誤類別各自對應 exit code：成功 `0`、內部錯誤
`1`、用法 `2`、設定 `3`、認證 `4`、權限 `5`、找不到 `6`、驗證 `7`、衝突 `8`、不可用 `9`、
網路 `10`、上游 `11`、寫入結果不明 `12`。程式應依結構化 code 分支，不比對英文訊息。

### 對寫入保持保守

- 官方 Web Service 函式由人工審查過的 safety registry 分類；未知函式一律視為寫入，絕不自動重試。
- 作業提交的 `--dry-run` 會解析並顯示計畫，但不送出寫入。
- `--read-only` 會把寫入命令從命令樹移除。
- `moodle mcp serve` 預設唯讀，必須明確加上 `--allow-write`。
- HTML fallback 只讀；無法證明提交語意時，CLI 會拒絕執行。

## 快速開始

目前還沒有正式 release。請用 Go 1.26 以上版本從原始碼建置：

```sh
git clone https://github.com/KoukeNeko/Moodle-CLI.git
cd Moodle-CLI
make build
./bin/moodle version
make install
```

登記站台並檢查可用登入方式：

```sh
moodle site add school https://moodle.example.edu
moodle auth methods --site school
```

Linux 桌面只需安裝一次 callback handler，之後由一般瀏覽器完成 SSO：

```sh
moodle auth register-handler
moodle auth login --site school --method mobilelaunch
```

所有平台都能使用不需 desktop handler 的手動 callback：

```sh
moodle auth login --site school --method manual
```

其他明確登入方式：

```sh
printf '%s' "$TOKEN" | moodle auth login --site school --token-stdin
printf '%s' "$PASSWORD" | moodle auth login --site school \
  --method password --username student --password-stdin
moodle auth import-browser --site school --list-profiles
moodle auth import-browser --site school --store
```

憑證存在作業系統 keychain；設定檔只保存站台與帳號中繼資料，不保存 token 或 browser session。
CI 的一次性執行可以用 `MOODLE_WS_TOKEN` 或 `MOODLE_SESSION`，完全不落地保存。

## 日常命令

```sh
moodle doctor                         # 說明這個帳號能做什麼
moodle course list                    # 已選課程
moodle calendar upcoming              # 逾期與即將到期事項
moodle grade overview                 # 各課程總分
moodle grade list --course 2          # 單一課程成績簿

moodle assignment list
moodle assignment show 42             # ID 與貼上的 Moodle URL 都能用
moodle assignment status 42
moodle assignment submit 42 report.pdf --dry-run
moodle assignment submit 42 report.pdf --yes
moodle assignment submit 42 report.pdf --draft --yes

moodle forum list
moodle forum discussions 7
moodle forum read 19
moodle file download 'https://moodle.example.edu/pluginfile.php/...'
moodle resolve 'https://moodle.example.edu/mod/assign/view.php?id=42'

moodle api functions --match assign
moodle api call core_enrol_get_users_courses --param userid=4
moodle mcp serve                      # 唯讀工具
moodle mcp serve --allow-write        # 明確開放寫入工具
```

完整介面請執行 `moodle <command> --help`，或閱讀[命令手冊](https://github.com/KoukeNeko/Moodle-CLI/wiki/Commands-zh-TW)。

## 安全邊界

這支程式會讀取憑證並連線到伺服器，因此邊界必須明確：

- 只連線到目前 profile 設定的 Moodle 站台。
- 只存取 Moodle CLI 自己建立的 keychain 項目。
- 只有明確執行 browser import 才讀取指定 profile；只回傳或保存指定站台的 cookie，絕不寫入 log。
- `auth logout` 只刪本機憑證，不撤銷 Moodle token，因為同一個 token 可能也供 Moodle mobile app 使用。
- 沒有遙測、自動更新、提權、process injection 或憑證匯出。

完整 threat boundary 與 callback 設計見[安全模型](https://github.com/KoukeNeko/Moodle-CLI/wiki/Security-Model-zh-TW)。

## 相容性

Docker 完整測試只宣稱以下實際驗證過的版本：

| Moodle | 驗證版本 | 情境 |
| --- | --- | --- |
| 4.5 LTS | 4.5.12 | 標準站、限制 Web Service 站、十個學年 |
| 5.1 | 5.1.7 | 標準站、限制 Web Service 站、十個學年 |
| 5.2 | 5.2.3 | 標準站、限制 Web Service 站、十個學年 |

十年情境建立 10 個年度 cohort、30 位學生、20 份作業、60 次 submission、八門封存課程與兩門進行中
課程，並涵蓋 calendar event、缺成績、零分、草稿、已交件、逾期與跨年度資料匯流，專門找出只在全新
示範站才成立的假設。

Release build 目標為 Linux、macOS、Windows 的 amd64 與 arm64。現在只有 Linux 實作自動 browser
callback handler；CLI 與手動登入路徑會在三種作業系統建置與測試。

## 開發

```sh
make test
make test-race
make lint
make verify

make moodle-up V=v52
make moodle-decade V=v52
make moodle-down V=v52
```

Architecture test 會強制 package 邊界：feature package 擁有自己的 interface；`moodle` 與 `webread` 是
adapter；`bootstrap` 只是 composition root；CLI 與 MCP 都不能直接碰 HTTP。詳見
[docs/architecture.md](docs/architecture.md) 與[開發手冊](https://github.com/KoukeNeko/Moodle-CLI/wiki/Development-and-Testing-zh-TW)。

## 專案狀態

專案正在積極開發，尚未發布第一個正式版本。上述 source build 與已測命令可使用；安裝套件、程式碼簽章
與 notarization 尚未提供。未來的 release archive 會由 tag 觸發 GitHub Actions，附 checksum、SBOM 與
keyless signature。

## 授權與商標

[MIT](LICENSE) © 2026 KoukeNeko。

Moodle CLI 是獨立專案，與 Moodle 或 Moodle HQ 沒有隸屬、認可或贊助關係。「Moodle」是 Moodle Pty Ltd
的商標。本專案不包含 Moodle 原始碼。
