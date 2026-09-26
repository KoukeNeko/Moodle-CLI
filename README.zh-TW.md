<h1 align="center">Moodle CLI</h1>

<p align="center">
  <strong>供學習者、教職員與管理者使用的獨立 Moodle 命令列客戶端。</strong><br>
  給人閱讀的終端輸出，以及給 script、CI 與 agent 使用的版本化 contract。
</p>

<p align="center">
  <a href="#相容性"><img alt="Verified with Moodle 4.5.12, 5.1.7, and 5.2.3" src="https://img.shields.io/badge/MOODLE-4.5.12%20%7C%205.1.7%20%7C%205.2.3-FF8B00?style=for-the-badge&logo=moodle&logoColor=white"></a>
  <a href="#相容性"><img alt="Linux, macOS, and Windows" src="https://img.shields.io/badge/PLATFORMS-LINUX%20%7C%20MACOS%20%7C%20WINDOWS-5C6BC0?style=for-the-badge"></a>
  <a href="#適合自動化的穩定-contract"><img alt="JSON contract version 1" src="https://img.shields.io/badge/JSON%20CONTRACT-V1-009688?style=for-the-badge&logo=json&logoColor=white"></a>
</p>

<p align="center">
  <a href="README.md">English</a> · <strong>繁體中文</strong>
</p>

<p align="center">
  <a href="#快速開始">快速開始</a>
  · <a href="https://github.com/KoukeNeko/Moodle-CLI/wiki/Home-zh-TW">使用手冊</a>
  · <a href="https://koukeneko.github.io/Moodle-CLI/">驗證儀表板</a>
  · <a href="test/e2e/README.md">測試環境</a>
  · <a href="docs/architecture.md">架構</a>
</p>

```sh
moodle course list
moodle calendar upcoming
moodle participant list --param courseid=42
moodle assignment submissions --param 'assignmentids=[17]'
moodle ws describe core_course_get_contents
```

Moodle CLI 連線到你指定的 Moodle 站台，優先使用官方 Web Service API；站台沒有開放所需函式時，
才回退到唯讀 AJAX 或 HTML adapter。它處理課程、參與者、分組、作業、行事曆、成績、論壇、完成度、
學分負荷與檔案，也不假設每個角色或 Moodle 安裝都具備相同能力。

同一支 binary 同時服務人與程式。一般輸出保持好讀；`--json` 輸出穩定的 `schema_version: 1`
envelope，命令提供 JSON Schema，錯誤則有固定 exit code。同一套 use case 也提供 MCP server；除非明確
啟用，會寫入的工具不會出現在工具清單。

## 與眾不同之處

### 依站台實際能力登入

Moodle CLI 支援既有 token、Moodle 帳密、登入 QR 資料、瀏覽器 session，以及 Moodle mobile launch
流程。Linux 桌面可以用每位使用者自己的 D-Bus URL handler，開啟平常使用的瀏覽器並自動接回結果；
學校 SSO、passkey 與 MFA 都留在瀏覽器內完成。macOS 可明確匯入既有 Safari session；自動 callback
handler 仍只支援 Linux，Windows 目前使用手動 callback 方式。

匯入瀏覽器 session 必須明確執行：`moodle auth import-browser` 可在 macOS Safari、Firefox 或
Chromium 系瀏覽器中尋找指定站台的 session，不會成為登入時暗中發生的副作用。Safari 的 cookie 格式
並非 Apple 公開介面，macOS 也可能拒絕讀取，因此屬於盡力支援；不必為了登入改裝 Firefox。
目前也支援 Firefox session snapshot 與 Chromium 的 Linux fallback encryption；由 macOS Keychain、
Windows DPAPI、Linux secret service（`v11`）或 app-bound encryption（`v20`）保管 key 的 Chromium
cookie 會被明確拒絕，不嘗試繞過瀏覽器保護。

### 真的交作業，不只是上傳檔案

作業提交完整執行 Moodle 的學生流程：上傳檔案、儲存 submission、在作業要求時另外送出評分，最後再
向 Moodle 回讀狀態。HTTP 成功不等於作業已交件。如果寫入回應遺失、又無法確認最終狀態，命令會用
exit code `12` 與 `ambiguous` outcome 結束，不會猜測或盲目重送。

### 適合自動化的穩定 contract

```sh
moodle version --json
moodle commands --json
moodle schema assignment.submit
moodle schema assignment submit --json
moodle course list --json
moodle assignment list --current --json --fields name,due_date --no-input
moodle ws list --version v52 --effect write --json
moodle ws describe core_course_update_courses --json
moodle ws call core_course_get_contents --params-json '{"courseid":42}'
```

所有 JSON 回應共用同一個版本化 envelope。穩定的錯誤類別各自對應 exit code：成功 `0`、內部錯誤
`1`、用法 `2`、設定 `3`、認證 `4`、權限 `5`、找不到 `6`、驗證 `7`、衝突 `8`、不可用 `9`、
網路 `10`、上游 `11`、寫入結果不明 `12`。呼叫者自己按下 Ctrl-C 是 `130`，刻意不佔用表格位置：中斷
命令是一個決定，不是失敗的方式。程式應依結構化 code 分支，不比對英文訊息。

給無人看管的呼叫者：`--fields` 只輸出會讀的欄位；`--no-input` 讓所有提示都變成錯誤，而不是卡住的
行程；`moodle schema <命令>` 回報每個命令的 `safety`（`read`、`local` 或 `write`）與 `idempotency`，
以及輸入與輸出的 JSON Schema，讓 agent 自己判斷能不能執行。Wiki 的
[自動化範例](https://github.com/KoukeNeko/Moodle-CLI/wiki/Automation-Recipes-zh-TW)附有可以直接交給
agent 的規則。

Typed `ws` registry 由拋棄式 Moodle 4.5.12、5.1.7、5.2.3 站台直接產生，目前是 780 個函式的聯集
（各版 759／761／755）。每個版本的參數與回傳 JSON Schema、transport、effect、capability、deprecated
與外部依賴都分開保留；執行時還會再核對目前 token 的 service 是否真的暴露該函式。

### 角色依 capability，不依人物假設

這支 CLI 不只給學生使用。Moodle 會依 system、category、course、activity、group 與 override context
決定帳號能做什麼，因此同一支 binary 可供站台管理員、manager、course creator、editing teacher、
non-editing teacher、student、一般 authenticated user 與自訂角色使用。命令不從角色名稱猜權限，而是
檢查目前憑證暴露的函式，再由 Moodle 在真正的 context 執行 capability 判斷。

高階寫入會在 `moodle commands --json` 明確標成 write，要求 `--yes` 或支援 `--dry-run`，不重試泛用
寫入，且一定服從全域 `--read-only`。第三方 plugin 函式仍由明確標示為 untyped 的 `api call` 處理。

### 對寫入保持保守

- 官方 Web Service 函式由人工審查過的 safety registry 分類；未知函式一律視為寫入，絕不自動重試。
- Typed 與高階寫入的 `--dry-run` 會驗證並顯示計畫，但不送出寫入。
- `--read-only` 會把寫入命令從命令樹移除。
- `moodle mcp serve` 預設唯讀，必須明確加上 `--allow-write`。
- HTML fallback 只讀；無法證明提交語意時，CLI 會拒絕執行。

## 快速開始

### 安裝

```sh
# macOS 或 Linux，任何 shell——會以 checksums.txt 驗證下載內容
curl -fsSL https://raw.githubusercontent.com/KoukeNeko/Moodle-CLI/main/scripts/install.sh | sh

# Windows（PowerShell）
irm https://raw.githubusercontent.com/KoukeNeko/Moodle-CLI/main/scripts/install.ps1 | iex
```

或使用套件管理器：

```sh
# macOS 或 Linux（Homebrew）
brew tap KoukeNeko/tap
brew install koukeneko/tap/moodle-cli

# Windows（Scoop）
scoop bucket add koukeneko https://github.com/KoukeNeko/scoop-bucket
scoop install koukeneko/moodle-cli

# Debian、Ubuntu、Fedora、RHEL、Alpine——Releases 頁面提供 .deb、.rpm、.apk
sudo dpkg -i moodle-cli_<version>_linux_amd64.deb
```

```sh
moodle version
moodle shell-completion zsh > "${fpath[1]}/_moodle"   # bash、zsh、fish、powershell
```

[Releases 頁面](https://github.com/KoukeNeko/Moodle-CLI/releases)另有 Linux、macOS、Windows 的
amd64 與 arm64 archive。安裝腳本會以 `checksums.txt` 核對 archive，digest 不符就拒絕安裝；手動下載
請自行核對。Stable release 通過 macOS 簽章與 notarization 驗證後才更新 Homebrew 與 Scoop，新 tag
可能需要幾分鐘才會出現。`scripts/uninstall.sh` 與 `scripts/uninstall.ps1` 會移除安裝的內容，加上
`--purge` 則連設定與已存憑證一併移除。詳見[安裝手冊](https://github.com/KoukeNeko/Moodle-CLI/wiki/Installation-zh-TW)。

### 從原始碼建置

請使用 Go 1.26 以上版本：

```sh
git clone https://github.com/KoukeNeko/Moodle-CLI.git
cd Moodle-CLI
make build
./bin/moodle version
make install
```

最快的開始方式是讓它去問站台支援什麼，再由你選擇：

```sh
moodle setup https://moodle.example.edu
```

也可以分步進行——登記站台並檢查可用登入方式：

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
# macOS：匯入已在 Safari 登入的 session
moodle auth import-browser --site school --browser safari --store
```

憑證存在作業系統 keychain；設定檔只保存站台與帳號中繼資料，不保存 token 或 browser session。
CI 的一次性執行可以用 `MOODLE_WS_TOKEN` 或 `MOODLE_SESSION`，完全不落地保存。完全沒有 keychain 的
機器可以主動選用權限 `0600` 的檔案：

```sh
moodle --credential-store file auth login --site school --token-stdin
```

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

moodle participant list --param courseid=2
moodle enrolment methods --param courseid=2
moodle enrolment add --params-json '{"enrolments":[{"roleid":5,"userid":7,"courseid":2}]}' --dry-run
moodle group create --params-json '{"groups":[{"courseid":2,"name":"Lab A"}]}' --dry-run
moodle assignment submissions --param 'assignmentids=[42]'
moodle workload validate --require-minimum

moodle ws list --version v52 --component mod_assign
moodle ws describe mod_assign_save_grade
moodle ws call core_enrol_get_users_courses --param userid=4
moodle api call local_example_function --params-json '{}'  # 第三方／未登錄
moodle mcp serve                      # 唯讀工具
moodle mcp serve --allow-write        # 明確開放寫入工具
```

完整介面請執行 `moodle <command> --help`、閱讀自動產生的[完整命令參考](https://github.com/KoukeNeko/Moodle-CLI/wiki/Command-Reference-zh-TW)，
或在[功能覆蓋](https://github.com/KoukeNeko/Moodle-CLI/wiki/Feature-Coverage-zh-TW)查閱全部 780 個 core function。

## 安全邊界

這支程式會讀取憑證並連線到伺服器，因此邊界必須明確：

- 只連線到目前 profile 設定的 Moodle 站台。
- 只存取 Moodle CLI 自己建立的 keychain 項目。完全沒有 keychain 的機器（headless Linux、SSH、WSL、
  容器）可以主動選用權限 `0600` 的檔案：`--credential-store file`；不會自動退回檔案。
- 只有明確執行 browser import 才讀取指定 profile；只回傳或保存指定站台的 cookie，絕不寫入 log。
- `auth logout` 只刪本機憑證，不撤銷 Moodle token，因為同一個 token 可能也供 Moodle mobile app 使用。
- 沒有遙測、自動更新、提權、process injection 或憑證匯出。

完整 threat boundary 與 callback 設計見[安全模型](https://github.com/KoukeNeko/Moodle-CLI/wiki/Security-Model-zh-TW)。

## 相容性

[COMPATIBILITY.zh-TW.md](COMPATIBILITY.zh-TW.md) 是完整矩陣：哪些 Moodle 版本、平台、登入方式與
憑證儲存已驗證、預期可用或未測試，每一項都附依據。以下為摘要：

Docker 完整測試只宣稱以下實際驗證過的版本：

| Moodle | 驗證版本 | 情境 |
| --- | --- | --- |
| 4.5 LTS | 4.5.12 | 標準站、限制 Web Service 站、十個學年 |
| 5.1 | 5.1.7 | 標準站、限制 Web Service 站、十個學年 |
| 5.2 | 5.2.3 | 標準站、限制 Web Service 站、十個學年 |

小型十年情境建立 10 個年度 cohort、30 位學生、20 份作業、60 次 submission、八門封存課程與兩門進行中
課程，並涵蓋 calendar event、缺成績、零分、草稿、已交件、逾期與跨年度資料匯流，專門找出只在全新
示範站才成立的假設。

獨立的 PostgreSQL scale profile 會建立 50,000 位合成學生、跨 20 學期的 1,000 門正式課、一門含
50,000 人的零學分 orientation 課，以及 237 萬筆選課資料。SQL control plane 另行驗證大學生每學期
21 學分、研究生每學期 6 學分；CLI 再讀回代表性 workload 與完整 50 頁參與者，記錄 latency、peak
RSS、HTTP requests 與磁碟。Image bootstrap 完成後，Moodle scale 容器不能連公網。Moodle 5.2.3
自身要求 PostgreSQL 16，因此使用最低相容版本，不繞過環境檢查。

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
make moodle-matrix V=v52
make moodle-scale V=v52
make moodle-down V=v52
```

Architecture test 會強制 package 邊界：feature package 擁有自己的 interface；`moodle` 與 `webread` 是
adapter；`bootstrap` 只是 composition root；CLI 與 MCP 都不能直接碰 HTTP。詳見
[docs/architecture.md](docs/architecture.md) 與[開發手冊](https://github.com/KoukeNeko/Moodle-CLI/wiki/Development-and-Testing-zh-TW)。

## 專案狀態

專案正在積極開發。Release workflow 設定為產生 checksum archive 與 SBOM，以 Developer ID 簽署並 notarize
兩個 macOS binary，在真正的 macOS runner 驗證後才公開 release，替 checksum 加上 keyless workflow
signature，並更新公開的 [Homebrew tap](https://github.com/KoukeNeko/homebrew-tap) 與
[Scoop bucket](https://github.com/KoukeNeko/scoop-bucket)。目前公開版本以
[release 紀錄](https://github.com/KoukeNeko/Moodle-CLI/releases)為準。Windows Authenticode 尚未設定，
因此 Windows 可能顯示 SmartScreen 警告。

<p>
  <a href="https://github.com/KoukeNeko/Moodle-CLI/actions/workflows/ci.yml"><img alt="CI status" src="https://img.shields.io/github/actions/workflow/status/KoukeNeko/Moodle-CLI/ci.yml?branch=main&style=for-the-badge&logo=githubactions&logoColor=white&label=CI"></a>
  <a href="https://go.dev/"><img alt="Go 1.26 or newer" src="https://img.shields.io/badge/GO-1.26%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
  <a href="LICENSE"><img alt="MIT license" src="https://img.shields.io/badge/LICENSE-MIT-4CAF50?style=for-the-badge&logo=github"></a>
</p>

## 授權與商標

[MIT](LICENSE) © 2026 KoukeNeko。

Moodle CLI 是獨立專案，與 Moodle 或 Moodle HQ 沒有隸屬、認可或贊助關係。「Moodle」是 Moodle Pty Ltd
的商標。本專案不包含 Moodle 原始碼。
