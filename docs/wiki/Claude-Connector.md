# Claude connector (host mode)

[繁體中文](Claude-Connector-zh-TW) · [Home](Home)

`moodle mcp host` serves the same tools as `moodle mcp serve` over HTTPS. Claude on the web, in the
phone apps, and in Cowork reaches Moodle through a connector added by URL, so the CLI does not have
to run on the machine Claude runs on.

## Connector and MCP

A connector is Claude's name for the feature, and it is always backed by MCP. A remote one is an MCP
server added by URL. A local one is a desktop extension, which Claude Desktop also lists as a
connector.

| Term | What it is | Where it runs | Command here |
| --- | --- | --- | --- |
| MCP | The protocol: JSON-RPC 2.0 messages carrying tool calls | — | — |
| Local (stdio) server | MCP over stdin and stdout | On your machine, started by the client | `moodle mcp serve` |
| Remote server, shown as a connector | MCP over Streamable HTTP | Anywhere reachable over HTTPS; Claude connects from Anthropic's servers | `moodle mcp host` |
| Desktop extension (MCPB) | A local MCP server packaged for Claude Desktop | On your machine | — |
| MCP connector in the Messages API | An API parameter that attaches a remote server to one request | — | — |

For claude.ai, Claude Desktop, mobile, and Cowork, Claude connects from Anthropic's servers, not from
your device. A server on `localhost` or a private network is not reachable from there without a
tunnel. Claude Code connects from your machine.

## Verification status

Tried on 2026-10-12 against a local Moodle 4.5.12 test site:

- claude.ai in the browser: a custom connector was added, signed in, listed 14 tools, and answered a
  question by calling two of them after approval.
- Claude Code 2.1.267 on loopback: sign-in and connection check.
- The official MCP Inspector, and an independent OAuth client, through a Cloudflare quick tunnel.

The host starting its own quick tunnel, with no `--public-url`, was run on Linux with cloudflared
2026.9.0 the same day, with the final code. The tunnel came up, the host confirmed that the public
address reaches it before announcing it, the public address answered the discovery documents, and the
tunnel stopped with the host without leaving a process behind. Killing the host with `kill -9`
stopped the guard and `cloudflared` within two seconds. With a `config.yml` in the home
directory whose `ingress:` rule pointed at another local service, the tunnel still reached the host
and that service received no requests. That the host stops when the tunnel is killed was run with an
earlier version of the code, and the final code is covered by a test with a stand-in for cloudflared.
A claude.ai sign-in through a tunnel the host started was not repeated; the claude.ai trial used a
tunnel started by hand.

Not tried: macOS, Windows, Claude Desktop, the phone apps, Cowork, adding the connector as a Team or Enterprise
Owner, and what Claude shows for a connector whose host has restarted. Rows in the troubleshooting
table are inferred from the code and Claude's documentation unless marked observed.

## How a connection works

```text
Claude ──HTTPS──▶ tunnel or reverse proxy ──▶ moodle mcp host (127.0.0.1) ──▶ Moodle
```

1. Claude calls `POST /mcp`. The host answers `401` and points at its discovery document.
2. Claude reads that document (`/.well-known/oauth-protected-resource/mcp`) and
   `/.well-known/oauth-authorization-server`, then registers itself at `/register`.
3. Claude opens `/authorize` in your browser. The page names the application, the site, the access
   (read-only or read and write), and where the browser returns, and asks for the host passphrase.
4. After you allow, Claude exchanges the code at `/token` with PKCE (S256) and receives an access
   token (1 hour) and a refresh token (30 days, replaced on every use).
5. Claude calls `/mcp` with `Authorization: Bearer …`. Each connection opens with a
   `server/discover` request under protocol revision 2026-07-28. The host refuses it with `400`, and
   Claude falls back to `initialize` under 2025-06-18. That `POST /mcp 400` appears once per
   connection in the log and is expected (observed).

The host is the authorization server and the resource server in one process. There is exactly one
person it admits: whoever knows the passphrase.

## Setup

1. Sign in on the host machine as for any other command. A machine with no keychain can use
   `MOODLE_WS_TOKEN` or `--credential-store file`; see
   [Authentication and sites](Authentication-and-Sites).
2. Create a passphrase file readable only by you, and start the host. With no address of your own,
   that is all it takes, provided [`cloudflared`](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/)
   is installed:

   ```sh
   umask 077
   openssl rand -base64 24 > ~/.moodle-host-passphrase
   moodle mcp host --passphrase-file ~/.moodle-host-passphrase
   ```

   Without `--public-url` the host starts a Cloudflare quick tunnel itself. It listens on a free
   loopback port, waits until the tunnel carries traffic, which can take up to a minute, checks that
   the address reaches the host, and prints the connector URL, such as
   `https://example-words.trycloudflare.com/mcp`. The address is random, new on every start, and open
   to anyone until the host stops: anyone who has it can open the sign-in page, and only the passphrase
   stands between them and an access token. Cloudflare relays all traffic and gives quick tunnels no
   uptime guarantee, so this is for trying the host out. If the tunnel stops, so does the host.

   `cloudflared` is run with a configuration file of the host's own, so a `config.yml` of yours cannot
   send the tunnel somewhere else, and without your `TUNNEL_*` and `MOODLE_*` environment variables.
   It needs outbound access to Cloudflare (port 7844, TCP and UDP), and Cloudflare may limit how many
   quick tunnels one address can create: error 1015 appeared after about five in a few minutes. With
   a quick tunnel, `--listen` must be a loopback address.

   Prefer the file to `MOODLE_HOST_PASSPHRASE`, which other processes can read and which can end up
   in shell history, and never put the passphrase on the command line. Without either, a passphrase
   is generated and printed on stderr for that run, so do not send stderr to a shared or retained
   log such as a systemd journal or a container log. A chosen passphrase must have at least 12
   characters.
3. For an address that stays, use a named tunnel or a reverse proxy that terminates HTTPS and
   forwards to the host, and pass that address:

   ```sh
   moodle mcp host --public-url https://moodle-mcp.example.com \
     --passphrase-file ~/.moodle-host-passphrase
   ```

   With `--public-url` the host listens on `127.0.0.1:8787` unless `--listen` says otherwise, and
   starts no tunnel.
4. In claude.ai, open **Customize > Connectors**, choose **Add > Add custom connector**, enter a name
   and the connector URL the host printed, which ends in `/mcp`, and choose **Continue**. Claude detects
   **Sign in now** and **Register automatically**; leave them, and choose **Add**. Then choose
   **Connect**, enter the passphrase on the page that opens, and choose **Allow**.

   In a chat, the connector is under **+ > Connectors**. In the trial Claude asked before each tool
   call, with **Allow once**, **Always allow**, and **Decline**.

On Team and Enterprise plans an Owner adds the connector under **Organization settings >
Connectors**, and each member then connects with the passphrase. Every member who connects acts as
the same Moodle account, the passphrase is a shared secret, and the only way to withdraw access is to
stop the host.

For Claude Code, `claude mcp add --transport http moodle <connector URL>` registers the server, and `claude mcp login moodle` or `/mcp` inside Claude Code starts the same
sign-in. Claude Code runs on your machine, so a loopback address works without a tunnel:

```sh
moodle mcp host --public-url http://localhost:8787
claude mcp add --transport http moodle http://localhost:8787/mcp
```

`--listen` should stay on loopback. It defaults to `127.0.0.1:8787` with `--public-url` and to a free
port without it. `--public-url` is an origin with no path. It must be `https`, except that `http` is
accepted for `localhost`, `127.0.0.1`, and `::1`, which only Claude Code on the same machine can
reach.

`--allow-write` offers the writing tools, as for `mcp serve`. They include a generic writer that can
call any write function the account is allowed to — posting, editing, deleting — and some changes
cannot be undone. Course content is written by other people, including other students, and reaches
Claude, so keep Claude's permission for the writing tools on asking, and read the arguments before
approving.

## Controls

- A request to `/mcp` needs a live access token. Tokens in a query string are not accepted.
- A passphrase is compared in constant time. Five wrong ones within ten minutes lock sign-in for
  everyone until the window passes, even for the right passphrase. Anyone who knows the address can
  keep sign-in locked this way; that does not reveal the passphrase.
- Registration is open: Claude registers itself when a server offers no client metadata document. A
  client id grants nothing. A redirect must be `https`, or `http` on a loopback address, and must
  match what was registered exactly; a loopback redirect may use any port.
- The sign-in page is open to anyone who knows the address. It names the site, the access, and where
  the browser returns, but not the account. The application's name is chosen by the application and
  is not verified, so the return address is the row to check; it is shown as a browser resolves it, so
  a look-alike letter appears as `xn--` text.
- PKCE with S256 is required. A code works once and for five minutes. A refresh token works once.
- Client ids and pending sign-ins are signed rather than stored, so unauthenticated requests do not
  use memory.
- Access and refresh tokens live in memory only, as hashes. Stopping the host ends every sign-in and
  invalidates the connector's registration.
- A request to `/mcp` with a foreign `Origin` header is refused, which keeps a web page from reaching
  a host on loopback. Anthropic's requests carry no `Origin` (observed).
- Read-only by default: without `--allow-write` the writing tools are not offered.
- The log on stderr records each request's method, escaped path, status, and duration, and each tool
  call's name, outcome, and duration. It never records a query string, an argument, or a token.

## Limits

- One host serves one site and one account. Anyone admitted acts as that account, and the host holds
  its Moodle credential while it runs.
- A tunnel provider terminates HTTPS and can see the traffic. So can Anthropic: tool results,
  including grades and submissions, are sent to Claude.
- Tool calls run one at a time, because the session behind them is not safe to share. A client
  disconnecting does not cancel a call; stopping the host does, after a five-second grace period.
- Responses are plain JSON. The host opens no event stream, keeps no session, does not accept batched
  requests, and speaks revision 2025-06-18 and older.
- Claude limits a tool result to about 150,000 characters and a tool call to 240 seconds on claude.ai
  and Desktop. A proxy in front can cut a call off sooner: Cloudflare answers `524` when the origin
  takes longer than 125 seconds to reply, and only Enterprise plans can raise that. A call waiting for
  its turn counts against it.
- Anthropic's servers connect from `160.79.104.0/21`, which matters when access is restricted by
  address (observed: `160.79.106.164`).
- A quick tunnel gets a new address on every start and no uptime guarantee, and it is open to the
  whole internet until the host stops. A restart without `--public-url` therefore leaves the old
  connector pointing at an address that no longer exists.
- `cloudflared` runs under a guard, a second copy of `moodle`, which stops it when the host exits for
  any reason, including being killed. Only killing the guard as well would leave it running, and on
  Linux the kernel stops it then too. The guard shows in the process list as
  `moodle --moodle-tunnel-guard …`.
- Sign-ins do not survive a restart. Any restart, including one to add `--allow-write` or one that
  generates a new passphrase, invalidates the connector: remove it in Claude and add it again.
- A refresh token works once, so a client that repeats a refresh whose reply was lost has to sign in
  again.
- If the Moodle credential expires while the host runs, a tool result says to sign in again with
  `moodle auth login`. A host started with `MOODLE_WS_TOKEN` or `MOODLE_SESSION` only picks up a new
  value on restart.

## Troubleshooting

| Symptom | Cause |
| --- | --- |
| “cannot start a Cloudflare quick tunnel” | `cloudflared` is not installed, Cloudflare is limiting quick tunnels (error 1015), the network blocks it (port 7844), or the tunnel did not come up within 60 seconds. Its own last lines are printed with the error. Install it, or pass `--public-url` with a tunnel or proxy of your own. |
| “the Cloudflare quick tunnel does not lead to this host” | The address answered, but not with this host's sign-in metadata, so something sends the tunnel elsewhere. The host does not announce it. Pass `--public-url`. |
| “the Cloudflare quick tunnel stopped” | `cloudflared` exited while the host ran. Start the host again for a new address, and add the connector again. |
| “a Cloudflare quick tunnel needs --listen on a loopback address” | `--listen` named a wildcard or network address. Use `127.0.0.1`, or pass `--public-url`. |
| `POST /mcp 400` once at the start of each connection | Claude's probe of the newer protocol revision. Expected (observed). |
| “Couldn't reach the MCP server” | The tunnel or proxy is down, a firewall blocks Anthropic's range, the host was restarted without `--public-url` and so has a new address, or the address does not end in exactly one `/mcp`: a bare address, `/mcp/`, or `/mcp/mcp` gets a `404`, logged as `POST / 404`, `POST /mcp/ 404`, or `POST /mcp/mcp 404`. |
| “Authorization with the MCP server failed” | `--public-url` differs from the address typed into Claude. The discovery document must name the connector URL exactly. |
| “This link is not valid” | The host restarted after the connector was added, or the sign-in names a return address the client did not register. Remove the connector and add it again. |
| “Too many attempts” | Five wrong passphrases within ten minutes. Wait for the window to pass. |
| Tools are missing | The host is read-only. Restart it with `--allow-write`, then remove the connector and add it again. |
| `403` on `/mcp` | The request carried an `Origin` header other than the public address. |

References: the Model Context Protocol
[authorization](https://modelcontextprotocol.io/specification/2025-06-18/basic/authorization) and
[transport](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)
specifications (2025-06-18), the
[2026-07-28 backward compatibility rules](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning#backward-compatibility-with-initialization-based-versions),
and Claude's documentation on
[building](https://claude.com/docs/connectors/building) and
[adding](https://claude.com/docs/connectors/custom/add-unlisted) connectors.
