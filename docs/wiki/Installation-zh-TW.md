# 安裝

[English](Installation) · [首頁](Home-zh-TW)

使用套件管理器前，請先確認 [Releases 頁面](https://github.com/KoukeNeko/Moodle-CLI/releases)
已有公開穩定版且套件倉庫已更新。如果尚無公開版本，請先[從原始碼建置](#從原始碼建置)。

## Homebrew（macOS 或 Linux）

使用本專案的 tap；此 formula 不在 Homebrew Core：

```sh
brew tap KoukeNeko/tap
brew install koukeneko/tap/moodle-cli
moodle version
```

日後更新可執行 `brew update`，再執行 `brew upgrade moodle-cli`。Formula 對 macOS 與 Linux 的
amd64、arm64 各自記錄 release 的 SHA-256。

## Scoop（Windows）

加入本專案 bucket 並安裝 manifest：

```powershell
scoop bucket add koukeneko https://github.com/KoukeNeko/scoop-bucket
scoop install koukeneko/moodle-cli
moodle version
```

日後更新可執行 `scoop update`，再執行 `scoop update moodle-cli`。Manifest 支援 Windows amd64
與 arm64，分別核對 archive 的 SHA-256。Windows binary 尚未設定 Authenticode 簽章，因此
SmartScreen 可能要求確認。

## 直接下載

[Releases 頁面](https://github.com/KoukeNeko/Moodle-CLI/releases)列出已公開的 archive、
`checksums.txt` 與其 keyless workflow signature。選擇對應作業系統與架構的檔案，先核對
SHA-256，再將解壓後的 `moodle`（Windows 為 `moodle.exe`）放進 `PATH`。雜湊不符時不可執行。

套件倉庫只會在 stable release 通過真正的 macOS 簽章及 notarization 驗證後更新。若最新 tag
暫時不在 tap 或 bucket，可先用上一個穩定版，或等待發布流程完成。

## 從原始碼建置

請準備 Git、GNU Make 與 Go 1.26 以上版本：

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

`make build` 會在 `bin/moodle` 產生 pure-Go binary。

## 驗證建置

```sh
make verify
```

它依序執行 unit／contract test、race detector、格式與 vet 檢查，最後建置 binary。Docker Moodle
測試會下載 image 並建立本機站台，因此獨立執行；詳見[開發與測試](Development-and-Testing-zh-TW)。

## 平台說明

CLI 與手動登入流程可在 Linux、macOS、Windows 建置。自動 browser callback 目前只支援 Linux，
因為它使用 per-user D-Bus service；其他平台請用 `moodle auth login --method manual`。
