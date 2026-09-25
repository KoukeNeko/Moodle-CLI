# Authentication and sites

[繁體中文](Authentication-and-Sites-zh-TW) · [Home](Home)

## Add and select a site

```sh
moodle site add school https://moodle.example.edu
moodle site list
moodle site use school
moodle doctor
```

Site configuration records the URL, accounts, and current selection. It does not contain tokens or
browser sessions. Multiple accounts can be kept for one site and selected with `--account`.

## Discover login methods

```sh
moodle auth methods --site school
```

The result distinguishes `available`, `unavailable`, and `unknown`. An unreachable site is not
misreported as one that disables every method.

| Method | Use it when |
| --- | --- |
| `token` | You already have a Moodle Web Service token. |
| `password` | The site accepts Moodle-native username/password login for mobile services. |
| `qr` | You decoded a Moodle app login QR code. |
| `mobilelaunch` | Linux desktop callback handler is installed and the site enables mobile services. |
| `browser-session` | You have a fresh `MoodleSession` value to exchange for a token. |
| `manual` | Complete login in a browser, then paste its callback URL if the browser exposes it. |

## Browser SSO on Linux

```sh
moodle auth register-handler
moodle auth handler-status
moodle auth login --site school --method mobilelaunch
```

Registration is per-user. The browser callback travels over D-Bus rather than appearing in a process
command line. The login transaction is time-limited, single-use, bound to the site's canonical URL,
and the returned token is verified before storage.

## Import an existing browser session

```sh
moodle auth import-browser --site school --list-profiles
moodle auth import-browser --site school --store
# macOS: keep using Safari; no Firefox installation is needed.
moodle auth import-browser --site school --browser safari --store
```

Safari on macOS, Firefox session snapshots, and Chromium's Linux fallback encryption are supported.
Safari's cookie store is an undocumented format and macOS may deny access; the command reports this
instead of silently switching browsers. A currently signed-in Safari session may not be in the
on-disk cookie store at all; "no MoodleSession" does not mean you are signed out. If access is
denied, consider whether granting your terminal app access to browser data is appropriate before
changing macOS privacy settings. Chromium cookies
protected by macOS Keychain, Windows DPAPI, a Linux secret service (`v11`), or app-bound encryption
(`v20`) are refused rather than bypassed. Import is explicit because a browser profile contains
credentials for many sites. Only the matching Moodle cookie is returned or stored, but the browser's
storage necessarily has to be parsed to find it. Closing or signing out of the browser may invalidate
that session.

If Safari shows you as signed in but `import-browser` cannot find `MoodleSession`, do not keep
changing permissions or install another browser. In Safari, enable **Safari → Settings → Advanced →
Show features for web developers** if needed, then open **Develop → Show Web Inspector** on the
Moodle tab. In **Storage → Cookies**, select the Moodle domain and copy only the `MoodleSession`
cookie's **Value**. Run:

```sh
moodle auth import-session --site school
```

Paste the value at the hidden prompt and press Return. The CLI verifies it with that site before
storing it in the OS keychain. It never prints the value. For a secure pipe, use `--stdin`; do not
put the value in a command argument, shell history, screenshot, chat, or issue report. This manual
fallback does not need Full Disk Access. See [Apple's Safari developer-tools instructions](https://support.apple.com/en-ca/guide/safari/sfri20948/mac).

## Non-interactive use

Read secrets from stdin instead of command-line arguments:

```sh
printf '%s' "$TOKEN" | moodle auth login --site school --token-stdin
printf '%s' "$PASSWORD" | moodle auth login --site school \
  --method password --username student --password-stdin
```

For an ephemeral process, `MOODLE_WS_TOKEN` and `MOODLE_SESSION` override stored credentials without
writing them to disk or the keychain.

## Logout

```sh
moodle auth status
moodle auth logout
```

Logout deletes the local copy only. It intentionally does not revoke the Moodle token because Moodle
may have issued the same token to its mobile app. Revoke it from Moodle's Security keys page when
server-side invalidation is required.
