# 開發與測試

[English](Development-and-Testing) · [首頁](Home-zh-TW)

## 本機驗證

```sh
make test          # unit、contract、architecture test
make test-race     # 同一批 package 加 race detector
make lint          # gofmt 檢查與 go vet
make verify        # 以上全部，再做 production-style build
```

測試使用 `-count=1`，因為 architecture suite 會在 runtime 讀 package source；Go test cache 無法推導這項輸入。

## Docker Moodle 測試站

版本 selector 為 `v45`、`v51`、`v52`：

```sh
make moodle-up V=v52
make moodle-decade V=v52
make moodle-down V=v52
make moodle-purge V=v52
```

每個版本都建立標準站與關閉 Mobile Web Services 的限制站。完整 end-to-end suite 執行 250 次以上命令，
涵蓋預期失敗、contract envelope、read fallback、身分隔離、作業提交、下載安全與語意斷言。

`make moodle-decade` 是老化測試，deterministic fixture 包含：

- 10 個學年 cohort 與課程；
- 30 個學生身分；
- 20 份作業、60 次 submission；
- 八門封存、兩門進行中課程；
- 已交件、草稿、逾期、缺成績與真正的零分；
- Calendar event 與跨年度匯流資料。

Suite 會檢查 exact count 與 3,287 天資料跨度，因此默默遺失封存紀錄、或把零分當缺值都無法通過。

## 架構

這是 modular monolith。Feature package 定義自己消費的 backend interface；`moodle` 與 `webread` 實作
adapter；`authmethod/*` 實作登入策略；`bootstrap` 負責接線；CLI 與 MCP 只呼叫 feature use case，
不直接做 HTTP。

Architecture test 會強制 import direction。新行為應加入最小的擁有者 feature，不建立共享 god package。
調整 package 邊界前，請閱讀 repository 的 [docs/architecture.md](https://github.com/KoukeNeko/Moodle-CLI/blob/main/docs/architecture.md)
與 ADR。

## CI 與 release

GitHub Actions 會在支援的 Go line 與下一個 line 執行測試、lint、coverage；在 Linux／macOS／Windows
建置；從編譯後 binary 驗 JSON contract；對三個 Moodle 版本跑十年 Docker 情境與已暴露唯讀函式的
角色矩陣；並建置 snapshot release package。Push 與 PR 測試不會取得發版簽章 secrets。

符合 `v*` 的 tag 必須先通過該 tag 自身的 `make verify` 與三版 Moodle smoke／角色矩陣，才會使用
Apple 簽章 secrets 啟動 GoReleaser；stable tag 也會在建立 draft 前確認 `HOMEBREW_TAP_TOKEN` 存在。
Archive 包含 binary、README 與 license，並附 checksum、SBOM
與 keyless checksum signature。兩個 macOS binary 會先以 Developer ID 簽署、送 Apple notarization，
再封裝進 archive；另一個 macOS runner 會打開實際下載檔，驗證 `codesign` 與 Gatekeeper，通過後才公開
draft。Stable release 接著更新 `KoukeNeko/homebrew-tap`。Windows Authenticode 尚未設定。

目前沒有 public release，因此 workflow 已達 distribution-ready，但 Apple credential 尚未在真正的 tag
artifact 上完成證明；第一個 tag 才是最後的端到端驗證。
