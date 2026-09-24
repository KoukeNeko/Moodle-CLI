# 驗證報告契約

[English](Report-Contract) · [首頁](Home-zh-TW)

GitHub Pages dashboard 把「函式清冊」與「實際執行 evidence」分開，絕不把缺少 artifact 轉成通過。

## 頁面

- **Overview**：支援版本、registry 聯集、規模真值與已執行角色 cell 數。
- **Function coverage**：版本、component、effect、transport、recipe 狀態與 registry 結果。
- **Role matrix**：各角色／domain 的 passed、expected denied、expected unavailable、failed 與 not run 數量。
- **Scale**：資料真值、學分 invariant、latency、RSS、REST calls 與磁碟觀測。
- **Runs**：commit、run、產生時間、runner 身分／image 與結果狀態。

可依版本、component、effect、結果篩選。`registry-covered`、`passed`、`partial`、`not-run` 是刻意分開
的狀態。
`recipe` 欄在具備可執行 fixture binding 與結果斷言前標為 `not-implemented`；只有產生參數 schema
並不等於已有測試 recipe。

## 角色矩陣 JSONL

`test/reports/role-matrix.jsonl` 每行代表一個實際執行的 CLI cell。每個 JSON object 必須包含
`version`、`role`、`function`、`outcome`；`domain` 可省略，此時由函式 namespace 推導。合法結果
只有 `passed`、`expected_denied`、`expected_unavailable`、`failed`；`skip` 不合法，會使報告建置失敗。

artifact 不存在時，dashboard 產生明確的 `not-run` placeholder；artifact 存在時，依版本、runtime
role、domain 彙總，所有尚未執行的 role/function cell 仍會列入明確的 `not_run` 分母，部分 artifact
不會讓整個角色看似已完成；重複 cell 會使 report build 失敗。對應函式會由 `registry-covered` 改為
`executed` 或 `failed`。預期的權限拒絕與不可用會保留為獨立授權結果，不會被重新包裝成一般 pass。

完整矩陣尚未產生前，各版的 `role-preflight-<version>.jsonl` 會提供每個 principal 一筆實際執行的
`core_webservice_get_site_info` CLI cell；dashboard 將 recipe 標成 `credential-preflight`，其餘函式
仍列在 `not_run` 分母。前置測試的 outcome 或 exit 異常會使 report build 失敗。

另外的 `role-matrix-service-<version>.jsonl` fragment 記錄可登入 principals 經 CLI 證實的 service
unavailable。Builder 會與 preflight cells 合併、拒絕重複的 role/function 配對，其他 cell 仍計入
`not_run`。

`test/reports/runtime-roles-<version>.json` 在函式 cell 尚未執行前提供正確分母。檔案存在時，
dashboard 的 placeholder 會採用該 Moodle runtime 動態發現的 principals（包含 custom/plugin
roles），而不是標準角色 fallback。已執行 cell 的角色必須存在於同版 inventory；inventory 格式錯誤、
角色重複、數量不符或缺少 site administrator 都會使報告建置失敗，不會靜默退回固定清單。

## 保存與去敏

Pages 目前發布最新的合成 snapshot；歷史趨勢累積仍是待完成里程碑，不能從只有一列的 Runs view
推論已有長期 baseline。完整 JSONL、JUnit、transcript、container diagnostics 以 Actions artifact
保存 90 天。兩者都不得包含 token、cookie、password、callback URL、authorization header 或未去敏
request secret。

Push 只建置 dashboard UI，不以 repository 內的 placeholder snapshot 覆蓋已發布的 long-haul 證據。
排程 long-haul 與明確要求發布的手動 run 會下載自身去敏後的 `test/reports/` artifact 再部署。
Report build 遇到重複巢狀的 `test/reports/reports/` 會失敗。

若要用較新的報告程式重新發布既有 long-haul 結果，可手動執行 **Verification dashboard**，填入該次
run 的數字 `evidence_run_id`。它下載該次去敏 artifact，分別顯示測試 commit 與報告 commit，不重跑
50k seed。`runner` 欄使用 scale summary 記錄的測試 runner；舊版 summary 未記錄名稱時會明確標示。

Report builder 讀取產生式 registry 與測試 artifact，附上精確來源 lineage；測試失敗後仍要建置報告，
讓失敗 evidence 可查。`make report-site` 可在本機重建同一份靜態 dashboard；CI 以
`make report-data-test` 驗證缺少、已觀測及非法／no-skip evidence 的行為。
