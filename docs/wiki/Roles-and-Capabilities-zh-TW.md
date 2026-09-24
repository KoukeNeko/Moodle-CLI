# 角色與 capability

[English](Roles-and-Capabilities) · [首頁](Home-zh-TW)

Moodle CLI 並非只供學生使用，也不把角色名稱直接翻譯成權限。Moodle 會在 system、category、course、
activity、group 與 override context 評估 capability；因此角色名稱相同的兩個帳號，得到不同結果可能完全
正確。

## Runtime matrix 的目標身分

- 站台管理員（不是 role table 內的角色）
- `manager`、`coursecreator`、`editingteacher`、`teacher`、`student`、`guest`、`user`、`frontpage`
- runtime 動態發現的所有自訂／plugin 角色
- 一個由 archetype 衍生的 fixture 角色
- 一個沒有 archetype 的 fixture 角色

目前已具備產生式 registry 與代表性命令情境，但完整的 runtime「角色 × 函式」harness 尚未產生
artifact，因此公開 dashboard 將這些 cell 明確標為 `not-run`，不會算成 passed 或 skip。

Harness 完成後，每個 cell 都必須經 CLI 執行；終態只能是 `passed`、`expected_denied`、
`expected_unavailable`、`failed`。角色看不到某個函式時，也要用明確拒絕或 unavailable 證明邊界。

`make moodle-decade V=<version>` 現在會從執行中的 Moodle 資料庫動態發現角色，並寫出
`test/reports/runtime-roles-<version>.json`。fixture 包含所有已安裝 runtime role、獨立的站台管理員
principal、可指派 context levels、credential kind，以及測試實際採用的 context assignment。
另外固定安裝兩個 canary：繼承 `teacher` archetype 的 `matrixteacher`，以及沒有 archetype 的
`matrixblank`。這能證明後續矩陣不能把八個標準 shortname 寫死。此 inventory 是完整
role/function harness 的輸入；只有 inventory 不會被宣稱為函式已執行。

接著 `make moodle-role-preflight V=v52` 會替每個 password principal 實際透過 CLI 執行無副作用的
typed `core_webservice_get_site_info`；guest 必須在明確的無憑證 CLI 路徑得到 unavailable。
去敏後的 `test/reports/role-preflight-<version>.jsonl` 不含 token、密碼、username 或 Moodle response。
它只證明 credential 與 transport 已可供後續 harness 使用，不計為完整 role/function 覆蓋。

長途測試的 `moodle-service-matrix` 階段會讓每個可登入 principal，逐一透過 typed CLI 呼叫
未暴露於官方 mobile service 的 core function。每個 cell 必須回傳 exit 9 與
`unavailable/capability`；service gate 在參數驗證或 Moodle 函式呼叫前執行。
`role-matrix-service-<version>.jsonl` 是部分矩陣證據；guest 與已暴露函式仍列在未執行分母。

## 三層不同的 gate

1. **Registry 支援**：core function 存在於至少一個受支援 Moodle 版本。
2. **Service exposure**：目前憑證所屬 external service 有暴露函式。
3. **Capability 評估**：Moodle 對此次呼叫的真實物件與 context 作最後判斷。

CLI 不會把 registry 收錄誤當成權限。`moodle ws describe` 記錄需求，`moodle site inspect` 顯示 service
exposure，實際操作仍以 Moodle 的判斷為準。

## 寫入安全

高階寫入與 typed `ws` 寫入會在 `moodle commands --json` 標記，支援 `--dry-run`、要求明確 opt-in，
並服從全域 `--read-only`。泛用寫入不重試；回應遺失且無法 reconciliation 時，回傳 exit `12`
（`ambiguous`），不盲目重送。

另見[完整命令參考](Command-Reference-zh-TW)與[功能覆蓋](Feature-Coverage-zh-TW)。
