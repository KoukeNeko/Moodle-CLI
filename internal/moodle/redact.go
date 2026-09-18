package moodle

import (
	"net/http"
	"net/url"
	"strings"
)

// Redacted replaces a secret in diagnostic output.
const Redacted = "[redacted]"

// secretParams are query or form names whose value must never be logged.
//
// Moodle puts the token in the URL for file downloads, so redacting headers
// alone would still leak it.
var secretParams = map[string]bool{
	"wstoken":      true,
	"token":        true,
	"privatetoken": true,
	"qrloginkey":   true,
	"passport":     true,
	"password":     true,
	"sesskey":      true,
}

// secretHeaders are headers whose value must never be logged.
var secretHeaders = map[string]bool{
	"authorization":  true,
	"cookie":         true,
	"set-cookie":     true,
	"x-moodle-token": true,
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
