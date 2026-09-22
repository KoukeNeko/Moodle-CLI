# Moodle CLI 使用手冊

Moodle CLI 是給學生、script 與 agent 使用的獨立命令列客戶端。它優先走 Moodle 官方 Web Service API；
站台沒有開放足夠函式時，使用唯讀 fallback；交作業則會確認 Moodle 最終狀態。

[English](Home)

## 從這裡開始

1. [安裝目前原始碼](Installation-zh-TW)。
2. 新增站台並選擇[登入方式](Authentication-and-Sites-zh-TW)。
3. 查看[命令指南](Commands-zh-TW)。
4. 自動化前閱讀 [JSON contract](JSON-Contract-zh-TW)。

## 設計承諾

- 設定檔只含站台與帳號中繼資料；憑證放在作業系統 keychain。
- 依目前站台實際回報的 capability 決定功能，不以版本號猜測。
- HTML fallback 只讀；未知 Web Service 函式一律視為寫入。
- 結果不明的寫入會標記 ambiguous，絕不盲目重送。
- 人類輸出與版本化 JSON contract 是兩個明確分離的介面。

## 實際驗證環境

Docker integration suite 目前驗證 Moodle 4.5.12、5.1.7 與 5.2.3。長期資料 fixture 模擬十個學年，
而不是只有全新的示範站。完整命令與範圍見[開發與測試](Development-and-Testing-zh-TW)。

Moodle CLI 是獨立專案，與 Moodle 或 Moodle HQ 沒有隸屬關係。「Moodle」是 Moodle Pty Ltd 的商標。
