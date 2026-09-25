# 安全模型

[English](Security-Model) · [首頁](Home-zh-TW)

Moodle CLI 會處理認證資料、連線到學校服務，也能交作業，因此安全邊界刻意維持狹窄而明確。

## 憑證

- Token 與 browser session 存在作業系統 keychain 的 Moodle CLI 專用項目；設定檔只含中繼資料。
- `MOODLE_WS_TOKEN` 與 `MOODLE_SESSION` 提供單一 process 的一次性憑證，優先於已存憑證。
- Secret 應從 stdin 或環境變數傳入，不要放在 command-line argument。
- `auth logout` 只刪本機副本，不撤銷 Moodle token，因為該 token 可能與官方 mobile app 共用。

## 瀏覽器存取

只有明確執行 `auth import-browser` 才會讀 browser profile。命令在 macOS Safari、Firefox 與 Chromium 系
儲存內容中尋找指定 host。瀏覽器把各站 cookie 存在一起，所以搜尋時可能需要解析其他 cookie；但只有符合的
Moodle session 會被回傳或保存，任何 cookie 都不寫入 log。指定 `--browser safari` 不會檢查其他瀏覽器的 profile。

找不到 cookie 只代表可讀 snapshot 內沒有，不代表使用者一定未登入。

`auth import-session` 不讀取任何 browser profile，而是在隱藏輸入的終端機提示中接收單一 cookie；明確指定
`--stdin` 時才從管線讀取。CLI 向所選 Moodle 驗證成功後才存入作業系統鑰匙圈。Cookie 值不能放在命令參數，
錯誤訊息也不會回顯。只能從自己的瀏覽器複製，絕對不要放進截圖、聊天或 issue。

## Linux callback handler

自動 mobile-launch callback 使用 per-user desktop 與 D-Bus service file。D-Bus activation 讓含 token 的
callback 不出現在 `/proc/<pid>/cmdline`。只有同時符合以下條件才接受 callback：

1. 對應仍有效、尚未逾時的 login transaction；
2. 具備預期 site hash 與 passport proof；
3. 從未被 claim；
4. 其中 token 確實被目標 Moodle 接受。

macOS 與 Windows 不安裝半套、未驗證的 handler，而是明確回報未支援並導向 manual flow。

## 網路與寫入

- 只向設定的 Moodle origin 發 request，不把憑證帶往其他 origin。
- 官方 Web Service 函式經人工審查的 mutation／retry registry；未知函式視為寫入且絕不重試。
- AJAX 與 HTML adapter 只讀。
- 作業寫入後會回讀狀態；仍無法判定時回報 `ambiguous`，禁止自動重送。
- 有 request throttling，並遵守 Moodle 的 `Retry-After`。

## 不在保護範圍內

CLI 無法防護已遭入侵的作業系統、未鎖定的 user keychain、惡意 Moodle server，或可被其他 process 讀取的
browser profile。它不提供遙測、自動更新、提權、process injection 或遠端憑證備份。

若漏洞揭露會暴露使用者或憑證，請先私下通知 repository owner，不要直接開公開 issue。
