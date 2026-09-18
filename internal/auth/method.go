package auth

import (
	"context"
	"fmt"
	"io"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Availability is what a probe could establish about a login method.
//
// The three-way answer matters: "I could not tell" is not "no". Collapsing
// them into a bool would make an offline site look like one that forbids every
// login method.
type Availability string

const (
	Available   Availability = "available"
	Unavailable Availability = "unavailable"
	Unknown     Availability = "unknown"
)

// ProbeResult is a method's verdict on a site.
type ProbeResult struct {
	Availability Availability
	// Reason explains an Unavailable or Unknown verdict, in words a user can
	// act on.
	Reason string
}

// Request carries what a method needs from the caller.
type Request struct {
	Site site.Site
	// WWWRoot is the site's canonical root as Moodle reports it. Callback
	// hashes are computed over this, which is not always the URL the user
	// typed.
	WWWRoot string

	// Username and Password are used by the password method only.
	Username string
	Password string

	// Callback is a pasted "<scheme>://token=..." URL.
	Callback string
	// Passport ties a callback to the login that started it.
	Passport string

	// QR is the decoded content of a login QR code.
	QR string

	// Token is an existing web service token.
	Token string

	// In and Out let a method prompt. Out is the diagnostic stream: a method
	// must never write to stdout, which belongs to the result.
	In  io.Reader
	Out io.Writer
}

// Credential is what a successful login produced.
type Credential struct {
	Token        string
	PrivateToken string
	// Method names how it was obtained, for the configuration record.
	Method string
}

// Method is one way of signing in.
//
// A method reports whether it could work and performs the login. It does not
// decide whether it should be preferred: ordering is the coordinator's job,
// because only the coordinator can see the alternatives.
type Method interface {
	Name() string
	Describe() string
	Probe(ctx context.Context, target site.Site, config *PublicConfig) ProbeResult
	Authenticate(ctx context.Context, req Request) (Credential, error)
}

// NotAvailable builds the error for a method that cannot run.
func NotAvailable(method, reason string) error {
	return errs.New(errs.CodeUnavailable,
		fmt.Sprintf("the %s login method is not available on this site: %s", method, reason)).
		WithReason(errs.ReasonCapability)
}
