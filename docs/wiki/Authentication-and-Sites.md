# Authentication and sites

[繁體中文](Authentication-and-Sites-zh-TW) · [Home](Home)

## Start here

One command asks the site what it supports, says which method suits it and
why, lists every method with its status, and runs the one you choose:

```sh
moodle setup https://moodle.example.edu
```

It needs a terminal. For a script or an agent, use `site add` and
`auth login --method …` below, which take every credential on stdin.

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
| `mobilelaunch` | macOS or a Linux desktop with its callback handler installed; the site enables mobile services. |
| `browser-session` | You have a fresh `MoodleSession` value to exchange for a token. |
| `manual` | Complete login in a browser, then paste its callback URL if the browser exposes it. |

## Choosing authentication for a harness

A harness must combine site support with its own execution environment and available credentials.
For a registered site, first inspect the intended account and the supported methods:

```sh
moodle auth status --site school --account student --json --no-input
moodle auth methods --site school --json --no-input
```

If the credential works, use it for subsequent commands without calling `auth login` again. An injected
`MOODLE_WS_TOKEN` or `MOODLE_SESSION` overrides stored credentials; validate the returned identity
against the intended user. A browser session supports the available read-only AJAX/HTML routes;
Web Service operations, including submissions, require a token.

`auth methods` returns `data[]` entries with `name`, `description`, `availability`, and `reason`.
`available` means the probe found the method potentially usable; it does not mean the harness has
its input, a desktop browser, or a person to complete SSO/MFA. `unknown` means support could not be
established, not that the method is disabled. Use `name` and `availability` for decisions; description
and reason are explanatory text, not stable codes. The response has no unattended-support or
required-input fields, so apply the following rules in order:

| What the harness has | Action | Human or browser needed now? |
| --- | --- | --- |
| A working stored or injected credential | Reuse it; no login method needed. | No, provided the credential store is accessible. |
| An existing Web Service token to persist | `auth login --method token --token-stdin`. | No; supply the token through stdin. |
| A Moodle-native username/password and site support | `auth login --method password --username student --password-stdin`. | No; supply the password through stdin. Institutional SSO credentials are not equivalent. |
| SSO, a macOS/Linux desktop browser, and a person ready to sign in | `auth login --method mobilelaunch`; Linux needs a registered handler. | Yes, for browser sign-in. |
| Fresh decoded login QR data and site support | `auth login --method qr --qr-stdin`. | Not for the exchange; obtaining the QR data requires prior browser sign-in. |
| An explicitly supplied fresh browser session and site support | `auth login --method browser-session --session-cookie-stdin` to try exchanging it for a token. | Not for the exchange; Moodle may refuse it. |
| A person who can obtain the mobile-launch callback URL | `auth login --method manual`. | Yes; the person opens the printed URL and pastes the callback. |
| An explicitly authorized browser-session import for read-only work | `auth import-browser --store` or `auth import-session --stdin`. | An existing signed-in session is required; import does not issue a token. |
| Headless execution with SSO and none of the above credentials | Stop authentication work and report that a person must sign in and provision a credential. | Yes; the CLI has no device-code or remote callback handoff flow. |

Add `--site school` to these commands and replace `student` with the intended account or username.
For unattended credential exchanges, also pass `--json --no-input`. Keep secrets out of argv, logs,
and chat. On machines without a keychain, explicitly choose `--credential-store file` when persisting
a credential, or inject a credential for each process instead.

Choose `--method` explicitly after evaluating these conditions. Omitting it can select mobile launch;
an explicit method is attempted even when its probe is doubtful. `--json` only selects output format.
Currently, `mobilelaunch --no-input` skips the Enter prompt but still opens a browser and waits for
the callback. It is not a headless login mode; do not call it from an unattended harness without an
arranged browser sign-in. On authentication exit code `4`, stop dependent work and arrange renewed
credentials; configuration or credential-store failures (`3`) and network failures (`10`) need their
own handling. Do not repeatedly attempt SSO when no person is available.

For eCourse2 on a Mac, the verified interactive choice is `mobilelaunch`. After that one browser
sign-in, a harness running as the same user can reuse the saved token if it can access the keychain.
A separate headless host needs its own credential provisioning.

## Browser SSO on macOS and Linux

On macOS, run `moodle auth login --site school --method mobilelaunch` and press Enter to open your
browser. First sign-in installs a per-user application in `~/Applications`; subsequent sign-ins reuse
it. After completing SSO, allow the browser to open the sign-in handler if it asks. The CLI receives,
verifies, and stores the token automatically; no callback URL needs to be copied.

On Linux, register the handler once:

```sh
moodle auth register-handler
moodle auth handler-status
moodle auth login --site school --method mobilelaunch
```

Registration is per-user. The callback travels over D-Bus on Linux, or an Apple Event and an anonymous
stdin pipe on macOS, rather than appearing in a process command line. Both use a private Unix socket.
The login transaction is time-limited, single-use, bound to the site's canonical URL,
and the returned token is verified before storage.

## Token lifetime and signing in again

Institutional OAuth/SSO authenticates you in the browser. Moodle then returns a Web Service token
through mobile launch, which the CLI stores and reuses. The school's OAuth callback returns to
Moodle; the mobile-launch callback delivers the Web Service token to the CLI. These are separate
steps, and the CLI does not receive an OAuth refresh token.

Moodle 5.2 defaults to **12 weeks (84 days)** for a newly issued mobile Web Service token, controlled
by the administrator's `tokenduration` setting. This is a default, not a guarantee for a particular
site or token. See [Moodle's token-duration setting](https://github.com/moodle/moodle/blob/MOODLE_502_STABLE/public/admin/settings/security.php).

The expiry is calculated from token creation. Moodle may return an existing valid token on a later
sign-in, so signing in again does not necessarily start another 84 days. API use does not renew that
expiry. See [Moodle's token issuance logic](https://github.com/moodle/moodle/blob/MOODLE_502_STABLE/public/lib/external/classes/util.php).

The CLI currently checks whether the credential works; it does not report its creation or expiry
date. In the eCourse2 (`ecourse2.ccu.edu.tw`) verification on 2026-10-09, browser sign-in and course
listing succeeded, but the mobile-launch callback and the site/mobile configuration APIs checked
did not expose the token's actual expiry. Do not infer an expiry date from the sign-in time or the
Moodle default.

For a site using automatic mobile launch, check the intended account and sign in again when its
token expires or is revoked:

```sh
moodle auth status --site school --account student
moodle auth login --site school --account student --method mobilelaunch
moodle auth status --site school --account student
```

Replace `student` with the account name listed by `moodle site list`. On macOS or a configured
Linux desktop, press Enter, complete browser sign-in, and let the CLI receive and verify the token.
There is no automatic refresh in the CLI; unattended jobs need an interactive sign-in before they
can resume after token expiry. Other login methods can be repeated with the same explicit method.

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

## Machines with no keychain

A headless Linux box, an SSH session, WSL and most containers have no Secret Service, so there is
nowhere for the operating system to keep a credential. Signing in reports that, and offers two
routes: an environment variable for one run, or a file for good.

```sh
moodle --credential-store file auth login --site school --token-stdin
moodle --credential-store file auth status
```

To stop passing the flag, record the choice:

```yaml
preferences:
  credential_store: file
```

Nothing switches to a file on its own. The file is `credentials.json` beside the configuration,
created `0600` in a `0700` directory, and `auth login` prints its path. That keeps other accounts on
the machine out; it does not protect against anything running as this user. `moodle auth logout`
removes the entry, and the file itself once it holds nothing.

## Logout

```sh
moodle auth status
moodle auth logout
```

Logout deletes the local copy only. It intentionally does not revoke the Moodle token because Moodle
may have issued the same token to its mobile app. Revoke it from Moodle's Security keys page when
server-side invalidation is required.
