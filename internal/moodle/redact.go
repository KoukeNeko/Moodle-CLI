package moodle

import (
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Redacted replaces a secret in diagnostic output.
const Redacted = "[redacted]"

// secretParams are names whose value must never be logged, whether they turn
// up as a query or form parameter, as a member of a JSON answer, or as
// name=value inside some other body.
//
// Moodle puts the token in the URL for file downloads, so redacting headers
// alone would still leak it. It also answers with credentials — login/token.php,
// the QR exchange, tool_mobile_get_autologin_key — so redacting only what is
// sent would still leak them.
var secretParams = map[string]bool{
	"wstoken":      true,
	"token":        true,
	"privatetoken": true,
	"qrloginkey":   true,
	"passport":     true,
	"password":     true,
	"sesskey":      true,
	// What tool_mobile_get_autologin_key answers with: the key, and a URL that
	// signs in whoever opens it.
	"key":          true,
	"autologinurl": true,
	// Part of core_webservice_get_site_info, and enough to download that
	// user's files.
	"userprivateaccesskey": true,
}

// secretHeaders are headers whose value must never be logged.
var secretHeaders = map[string]bool{
	"authorization":  true,
	"cookie":         true,
	"set-cookie":     true,
	"x-moodle-token": true,
}

// secretMember finds a JSON member whose name is secret, capturing the name.
// The value is a string, which may hold escaped quotes, or a bare scalar. A
// string that is never closed, because the text was cut off, is masked to the
// end rather than left alone.
//
// secretPair finds the same names as name=value, in a query string, a form, or
// a URL inside a string. Its value stops at the first character a value cannot
// hold, which includes the backslash that begins a JSON escape: an encoder may
// write the & between two parameters that way.
//
// Both work on the text, not on a parse, because the text is what gets
// printed, and it may be cut short, have a PHP notice in front of it, or not
// be JSON at all.
var secretMember, secretPair = secretPatterns()

func secretPatterns() (member, pair *regexp.Regexp) {
	names := make([]string, 0, len(secretParams))
	for name := range secretParams {
		names = append(names, regexp.QuoteMeta(name))
	}
	sort.Strings(names)
	alternatives := "(" + strings.Join(names, "|") + ")"
	member = regexp.MustCompile(`(?i)"` + alternatives + `"\s*:\s*("(?:[^"\\]|\\.)*(?:"|\\?$)|[^\s,}\]"{\[]+)`)
	pair = regexp.MustCompile(`(?i)` + alternatives + `=[^\s&"'\\<>#;]+`)
	return member, pair
}

// scanWindow is how much of a body is searched for secrets. Only the start of
// one is ever shown, and a response can run to megabytes.
const scanWindow = 4 << 10

// excerpt returns the start of a body for a person to read: at most limit
// bytes, with every secret in it masked.
//
// The masking comes first and the cut after, so the limit decides only how
// much is shown and never what counts as a secret: one that straddles it is
// masked whole, not shown up to the limit. The window can be short for the
// same reason, since a secret still open where it ends is masked to that end.
func excerpt(text string, limit int) string {
	more := false
	if len(text) > scanWindow {
		text, more = text[:scanWindow], true
	}
	text = secretMember.ReplaceAllString(text, `"${1}":"`+Redacted+`"`)
	text = secretPair.ReplaceAllString(text, `${1}=`+Redacted)
	if len(text) > limit {
		text, more = text[:limit], true
	}
	if more {
		text += "…"
	}
	return text
}

// RedactURL returns a copy of the URL safe to print.
func RedactURL(raw *url.URL) string {
	if raw == nil {
		return ""
	}
	clean := *raw
	if clean.User != nil {
		clean.User = url.User(clean.User.Username())
	}
	if query := clean.Query(); len(query) > 0 {
		for name := range query {
			if secretParams[strings.ToLower(name)] {
				query.Set(name, Redacted)
			}
		}
		clean.RawQuery = query.Encode()
	}
	return clean.String()
}

// RedactValues returns a copy of form values safe to print.
func RedactValues(values url.Values) url.Values {
	clean := url.Values{}
	for name, items := range values {
		if secretParams[strings.ToLower(name)] {
			clean.Set(name, Redacted)
			continue
		}
		for _, item := range items {
			clean.Add(name, item)
		}
	}
	return clean
}

// RedactHeader returns a copy of the headers safe to print.
func RedactHeader(header http.Header) http.Header {
	clean := http.Header{}
	for name, items := range header {
		if secretHeaders[strings.ToLower(name)] {
			clean.Set(name, Redacted)
			continue
		}
		for _, item := range items {
			clean.Add(name, item)
		}
	}
	return clean
}
