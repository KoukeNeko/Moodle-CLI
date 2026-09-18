package moodle

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// route is one way of reaching a Moodle function.
//
// Some functions answer identically over the web service endpoint and over the
// one a signed-in browser uses — the calendar and forum replies are the same
// shape down to their quirks. Where that is true the mapping is written once
// and the route is what differs, rather than the whole backend being written
// twice and drifting.
type route interface {
	// kind names the route for provenance, so a caller can tell which one
	// answered: they do not reach the same amount.
	kind() site.BackendKind
	// requirement is what a site must offer for this route to be worth trying.
	requirement(functions []string) site.Requirement
	// call invokes one function.
	call(ctx context.Context, function string, args map[string]any, out any) error
}

// wsRoute reaches Moodle with a web service token.
type wsRoute struct {
	client *Client
	token  string
}

func (r wsRoute) kind() site.BackendKind { return site.BackendWS }

func (r wsRoute) requirement(functions []string) site.Requirement {
	return site.Requirement{AnyFunction: functions, Credential: site.CredentialWSToken}
}

func (r wsRoute) call(ctx context.Context, function string, args map[string]any, out any) error {
	return r.client.Call(ctx, r.token, function, Params(args), out)
}

// ajaxRoute reaches Moodle with a browser session.
type ajaxRoute struct {
	session *AjaxSession
}

func (r ajaxRoute) kind() site.BackendKind { return site.BackendAJAX }

// requirement is empty whatever the functions are.
//
// There is no way to ask this endpoint what it offers: get_site_info is itself
// not exposed over it, and nothing lists what is. Availability is established
// by calling and reading the refusal.
func (r ajaxRoute) requirement([]string) site.Requirement {
	return site.Requirement{}
}

func (r ajaxRoute) call(ctx context.Context, function string, args map[string]any, out any) error {
	return r.session.Call(ctx, function, args, out)
}
