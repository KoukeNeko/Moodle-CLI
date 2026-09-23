# 十年規模測試模型

[English](Scale-Test-Model) · [首頁](Home-zh-TW)

規模測試使用獨立 PostgreSQL 16 環境。Moodle 5.2.3 會拒絕 PostgreSQL 15，因此 fixture 遵守 Moodle
真正的最低需求，不繞過 environment check。

## 固定真值

| 指標 | 數值 |
| --- | ---: |
| 學年／學期 | 10／20 |
| 學生 | 50,000（40,000 大學生；10,000 研究生） |
| 正式課程 instance | 1,000（每學期 50） |
| 正式選課 | 2,320,000 |
| Orientation 選課 | 50,000 |
| 選課資料總數 | 2,370,000 |
| 大學部負荷 | 每學期 7 × 3 = 21 學分，共 8 學期 |
| 研究所負荷 | 每學期 2 × 3 = 6 學分，共 4 學期 |

一門零學分 orientation 課包含全部 50,000 人，強制驗證 participant pagination。課程自訂欄位保存
學分、學制與學期。Fixture 另外保留跨年同名、重修、停權、封存、群組、availability、零分、未評分、
草稿、逾期、跨角色成員與權限 override。

## 獨立性與隔離

Seed 固定且可重複執行並收斂。Control plane 直接查 Moodle PostgreSQL；CLI 是受測面，兩者不共用計數
邏輯。破壞性 function recipe 使用小型拋棄式 fixture，不在大型站執行。Bootstrap 後 Moodle scale
container 會與公網 egress 斷線。

## Gate

- Seed 最長 180 分鐘；完整 job 最長 240 分鐘
- 單次 CLI 最長 120 秒；peak RSS 最多 1 GiB
- 產生資料與 Moodle data 合計最多 8 GiB
- 記錄 p50、p95、最大 latency、RSS、REST request 數與磁碟
- 記錄 runner identity／image，避免誤把不同硬體的結果互相比較

目前 harness 會執行上述絕對上限與線性 REST request budget。相對最近五次可比較成功結果中位數的
25% regression gate 尚未實作；在聚合存在以前，dashboard 不得宣稱已有歷史 baseline。

用 `make moodle-scale V=v52` 執行。公開頁只保存合成摘要；逐呼叫去敏 evidence 以 Actions artifact
保留 90 天。
