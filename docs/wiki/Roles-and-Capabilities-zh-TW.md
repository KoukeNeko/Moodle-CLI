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
