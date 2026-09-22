# JSON contract

[English](JSON-Contract) · [首頁](Home-zh-TW)

輸出給程式使用時請明確加上 `--json`；pipe 不會偷偷改變格式。

```sh
moodle version --json
moodle course list --json
moodle commands --json
moodle schema assignment.submit
```

## Envelope

成功回應含穩定的 top-level `schema_version`、`kind`、`data` 與 `meta`：

```json
{
  "schema_version": 1,
  "kind": "version",
  "data": {},
  "meta": {"source": "local"}
}
```

錯誤使用 `kind: "error"`，並提供 `code`、`reason`、`outcome`、`retryable` 等結構化欄位。Consumer
必須依這些欄位分支，不可比對英文 `message`。`reason` 刻意設計成 open set，必須容忍未來新增的值。

## Exit code

| Exit | Error code | 意義 |
| ---: | --- | --- |
| 0 | — | 成功 |
| 1 | `internal` | 本機 invariant 或非預期內部錯誤 |
| 2 | `usage` | 命令或參數錯誤 |
| 3 | `configuration` | 本機設定缺少或無效 |
| 4 | `authentication` | 憑證失效或被拒絕 |
| 5 | `permission_denied` | 已登入但沒有權限 |
| 6 | `not_found` | 找不到指定資源 |
| 7 | `validation` | 輸入或回傳狀態驗證失敗 |
| 8 | `conflict` | Server state 與操作衝突 |
| 9 | `unavailable` | Capability 或本機設施不可用 |
| 10 | `network` | Transport failure |
| 11 | `upstream` | Moodle 回傳失敗或無效內容 |
| 12 | 任意 code、`outcome: ambiguous` | 寫入可能已經發生 |

Ambiguous outcome 優先於底層錯誤類別。不要自動重試 exit 12；應回讀受影響的 Moodle object，再依目前
狀態決定下一步。

## Schema 與命令探索

`moodle schema` 列出內嵌 response schema；`moodle schema <kind>` 印出單一 schema。
`moodle commands --json` 描述目前命令樹，也標示命令是否改變狀態，讓 shell 與 agent 直接以 binary
本身作為 discovery source。

改變 contract v1 的含意或形狀必須建立新的 schema version。人類訊息與 diagnostic detail 不是穩定 API。
