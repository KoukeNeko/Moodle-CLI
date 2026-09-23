# 驗證報告契約

[English](Report-Contract) · [首頁](Home-zh-TW)

GitHub Pages dashboard 把「函式清冊」與「實際執行 evidence」分開，絕不把缺少 artifact 轉成通過。

## 頁面

- **Overview**：支援版本、registry 聯集、規模真值與已執行角色 cell 數。
- **Function coverage**：版本、component、effect、transport、recipe 狀態與 registry 結果。
- **Role matrix**：各角色／domain 的 passed、expected denied、expected unavailable、failed 數量。
- **Scale**：資料真值、學分 invariant、latency、RSS、REST calls 與磁碟觀測。
- **Runs**：commit、run、產生時間、runner 身分／image 與結果狀態。

可依版本、component、effect、結果篩選。`registry-covered`、`passed`、`partial`、`not-run` 是刻意分開
的狀態。

## 角色矩陣 JSONL

`test/reports/role-matrix.jsonl` 每行代表一個實際執行的 CLI cell。每個 JSON object 必須包含
`version`、`role`、`function`、`outcome`；`domain` 可省略，此時由函式 namespace 推導。合法結果
只有 `passed`、`expected_denied`、`expected_unavailable`、`failed`；`skip` 不合法，會使報告建置失敗。

artifact 不存在時，dashboard 產生明確的 `not-run` placeholder；artifact 存在時，依版本、runtime
role、domain 彙總，對應函式也會由 `registry-covered` 改為 `executed` 或 `failed`。預期的權限拒絕與
不可用會保留為獨立授權結果，不會被重新包裝成一般 pass。

`test/reports/runtime-roles-<version>.json` 在函式 cell 尚未執行前提供正確分母。檔案存在時，
dashboard 的 placeholder 會採用該 Moodle runtime 動態發現的 principals（包含 custom/plugin
roles），而不是標準角色 fallback。已執行 cell 的角色必須存在於同版 inventory；inventory 格式錯誤、
角色重複、數量不符或缺少 site administrator 都會使報告建置失敗，不會靜默退回固定清單。

## 保存與去敏

Pages 目前發布最新的合成 snapshot；歷史趨勢累積仍是待完成里程碑，不能從只有一列的 Runs view
推論已有長期 baseline。完整 JSONL、JUnit、transcript、container diagnostics 以 Actions artifact
保存 90 天。兩者都不得包含 token、cookie、password、callback URL、authorization header 或未去敏
request secret。

Report builder 讀取產生式 registry 與測試 artifact，附上精確來源 lineage；測試失敗後仍要建置報告，
讓失敗 evidence 可查。`make report-site` 可在本機重建同一份靜態 dashboard；CI 以
`make report-data-test` 驗證缺少、已觀測及非法／no-skip evidence 的行為。
