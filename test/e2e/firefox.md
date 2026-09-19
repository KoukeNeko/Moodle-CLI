# 用真的 Firefox 產生瀏覽器 fixture

`internal/browser` 讀的是 Firefox 自己寫出來的檔案，所以它的 fixture 必須是
Firefox 自己寫的。這一份記錄怎麼在沒有桌面的機器上做到——**headless 就夠了**。

## 裝 Firefox（Ubuntu 26.04）

apt 的 `firefox` 是 snap 轉接套件，容器裡通常不能動。用 Mozilla 官方 deb 庫：

```sh
curl -fsSL https://packages.mozilla.org/apt/repo-signing-key.gpg \
  -o /etc/apt/keyrings/packages.mozilla.org.asc
echo "deb [signed-by=/etc/apt/keyrings/packages.mozilla.org.asc] \
  https://packages.mozilla.org/apt mozilla main" \
  > /etc/apt/sources.list.d/mozilla.list
printf 'Package: *\nPin: origin packages.mozilla.org\nPin-Priority: 1000\n' \
  > /etc/apt/preferences.d/mozilla
apt-get update && apt-get install -y firefox
```

## 驅動一次真的登入

不要偽造 cookie——要量的就是 Firefox 怎麼存它。用 Marionette（Firefox 內建的
自動化通道，長度前綴的 JSON over TCP 2828）：

```sh
firefox --headless --marionette --profile /path/to/profile --no-remote &
```

然後照 `WebDriver:NewSession` → `WebDriver:Navigate` → 填表單 → 送出。

三個實際踩到的坑：

- **從 `wwwroot` 本身進去。** 站台會把別的主機名導向 `wwwroot`，而那是不同的
  cookie 網域，`logintoken` 會對不上，登入頁只會說「Unable to log in」。
- **http 站會跳「你正要用不安全的連線送出資料」對話框。** 不要抑制它，
  用 `unhandledPromptBehavior: ignore` 開 session，再 `WebDriver:AcceptAlert`
  ——那正是真人會做的事。
- **確認真的登入了**，不要只看有沒有 cookie。查資料庫：

  ```sh
  docker exec moodle-cli-e2e-v52-std-1 php -r '
    define("CLI_SCRIPT",true); require("/var/www/html/config.php");
    $r=$DB->get_record("sessions",["sid"=>$argv[1]]);
    echo $r ? "userid={$r->userid}\n" : "沒有這個 session\n";' <sid>
  ```

  `userid=0` 是未登入的匿名 session。

## 檔案在哪

Firefox 156 用 `~/.config/mozilla/firefox`，**不是** `~/.mozilla/firefox`。

| 什麼時候 | session 在哪 |
|---|---|
| 執行中 | `<profile>/sessionstore-backups/recovery.jsonlz4` |
| 乾淨關閉後 | `<profile>/sessionstore.jsonlz4` |

Moodle 的 session cookie 沒有到期時間，所以 Firefox **不會**把它寫進
`cookies.sqlite`（實測：0 筆）。

## fixture 的處理

`internal/browser/testdata/` 有兩份：

- `recovery.jsonlz4`：結構是真的，兩個 cookie 值換成同形狀的假字串，
  免得把憑證形狀的東西寫進版控。重新打包用只產生 literal 的 LZ4——
  合法但不含 match。
- `firefox-compressed.jsonlz4`：Firefox 自己壓的，有真的 LZ4 match，
  而且**零 cookie**（產生它的瀏覽只造訪一個不設 cookie 的本機頁面）。
  上面那份測不到解壓器的 match 路徑，這份測得到。
