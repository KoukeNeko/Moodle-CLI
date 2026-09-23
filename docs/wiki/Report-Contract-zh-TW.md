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

## 保存與去敏

Pages 目前發布最新的合成 snapshot；歷史趨勢累積仍是待完成里程碑，不能從只有一列的 Runs view
推論已有長期 baseline。完整 JSONL、JUnit、transcript、container diagnostics 以 Actions artifact
保存 90 天。兩者都不得包含 token、cookie、password、callback URL、authorization header 或未去敏
request secret。

Report builder 讀取產生式 registry 與測試 artifact，附上精確來源 lineage；測試失敗後仍要建置報告，
讓失敗 evidence 可查。`make report-site` 可在本機重建同一份靜態 dashboard。
