# 安裝

[English](Installation) · [首頁](Home-zh-TW)

目前沒有已發布的 release。請準備 Git、GNU Make 與 Go 1.26 以上版本，從原始碼建置：

```sh
git clone https://github.com/KoukeNeko/Moodle-CLI.git
cd Moodle-CLI
make build
./bin/moodle version
```

安裝至 `~/.local/bin`：

```sh
make install
```

需要其他位置時可覆寫 `PREFIX` 或 `BINDIR`：

```sh
make install PREFIX=/opt/moodle-cli
make install BINDIR="$HOME/bin"
```

`make build` 會在 `bin/moodle` 產生 pure-Go binary。Release 設定已涵蓋 Linux、macOS、Windows 的
amd64 與 arm64，但在倉庫 Releases 頁面真的出現檔案以前，都不應宣稱有可下載 release。

## 驗證建置

```sh
make verify
```

它依序執行 unit／contract test、race detector、格式與 vet 檢查，最後建置 binary。Docker Moodle
測試會下載 image 並建立本機站台，因此獨立執行；詳見[開發與測試](Development-and-Testing-zh-TW)。

## 平台說明

CLI 與手動登入流程可在 Linux、macOS、Windows 建置。自動 browser callback 目前只支援 Linux，
因為它使用 per-user D-Bus service；其他平台請用 `moodle auth login --method manual`。
