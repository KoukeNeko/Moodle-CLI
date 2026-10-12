package mcphost

import (
	"html/template"
	"net/http"
)

// The pages are plain HTML with no script. There is one decision on them —
// allow or deny — and a form does that without anything that could run if a
// client's name were crafted to look like code.
var pages = template.Must(template.New("pages").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
:root { color-scheme: light dark; --fg: #1b1f24; --bg: #ffffff; --muted: #5b6470; --line: #d3d8de; --accent: #1f5fbf; --bad: #b3261e; }
@media (prefers-color-scheme: dark) { :root { --fg: #e8eaed; --bg: #14171a; --muted: #9aa3ad; --line: #353b42; --accent: #7aa7f0; --bad: #f2a29c; } }
body { margin: 0; background: var(--bg); color: var(--fg); font: 16px/1.5 system-ui, sans-serif; }
main { max-width: 28rem; margin: 0 auto; padding: 2rem 1rem; }
h1 { font-size: 1.3rem; margin: 0 0 1.25rem; }
dl { margin: 0 0 1.25rem; border-top: 1px solid var(--line); }
dl div { display: flex; gap: 1rem; padding: .5rem 0; border-bottom: 1px solid var(--line); }
dt { flex: 0 0 6rem; color: var(--muted); }
dd { margin: 0; overflow-wrap: anywhere; }
label { display: block; margin-bottom: .25rem; }
input[type=password] { box-sizing: border-box; width: 100%; padding: .6rem; font: inherit; color: inherit; background: transparent; border: 1px solid var(--line); border-radius: 6px; }
.note { color: var(--muted); font-size: .9rem; }
.error { color: var(--bad); }
.actions { display: flex; gap: .75rem; margin-top: 1rem; }
button { font: inherit; padding: .6rem 1.2rem; border-radius: 6px; border: 1px solid var(--line); background: transparent; color: inherit; cursor: pointer; }
button.primary { background: var(--accent); border-color: var(--accent); color: var(--bg); }
</style>
</head>
<body>
<main>
<h1>{{.Title}}</h1>
{{if .Consent}}
<dl>
<div><dt>Application</dt><dd>{{.Client}}</dd></div>
<div><dt>Site</dt><dd>{{.Site}}</dd></div>
<div><dt>Access</dt><dd>{{if .Write}}Read and write{{else}}Read only{{end}}</dd></div>
<div><dt>Returns to</dt><dd>{{.ReturnsTo}}</dd></div>
</dl>
<p class="note">The application chose its own name; it has not been verified.</p>
{{if .Write}}<p class="note">This can change things on the site, and some changes cannot be undone.</p>{{end}}
<form method="post" action="/authorize">
<input type="hidden" name="request" value="{{.Request}}">
<label for="passphrase">Host passphrase</label>
<input id="passphrase" type="password" name="passphrase" autocomplete="off" autofocus>
{{if .Message}}<p class="error" role="alert">{{.Message}}</p>{{end}}
<div class="actions">
<button class="primary" type="submit" name="decision" value="allow">Allow</button>
<button type="submit" name="decision" value="deny">Deny</button>
</div>
</form>
{{else}}
<p>{{.Message}}</p>
{{end}}
</main>
</body>
</html>
`))

type page struct {
	Title   string
	Message string

	// The remaining fields belong to the consent form.
	Consent   bool
	Client    string
	Site      string
	Write     bool
	ReturnsTo string
	Request   string
}

// render writes a page with the headers a sign-in page needs: nothing cached,
// nothing framed, and no referrer that would carry the address of a pending
// request to wherever a link on it led.
func render(w http.ResponseWriter, status int, content page) {
	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("X-Frame-Options", "DENY")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	// form-action is left out on purpose: browsers apply it to the redirect
	// that follows a submission, and that redirect leaves for Claude.
	header.Set("Content-Security-Policy",
		"default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'")
	w.WriteHeader(status)
	_ = pages.Execute(w, content)
}
