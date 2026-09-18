package moodle

import (
	"fmt"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// exception is Moodle's error shape. It arrives with HTTP 200, so the status
// line alone never tells you whether a call worked.
type exception struct {
	Exception string `json:"exception"`
	ErrorCode string `json:"errorcode"`
	Message   string `json:"message"`
	// Error carries the human text on /login/token.php, which reports failure
	// with "error" where the REST endpoint uses "message". Without this the
	// wording Moodle chose is lost and the user sees only a generic line.
	Error     string `json:"error"`
	DebugInfo string `json:"debuginfo"`
}

func (e exception) empty() bool {
	return e.Exception == "" && e.ErrorCode == ""
}

// classification maps a Moodle errorcode onto our vocabulary.
type classification struct {
	code   errs.Code
	reason errs.Reason
	hint   string
}

// knownErrorCodes are the Moodle errorcodes worth translating. Anything else
// becomes CodeUpstream with the original preserved in the envelope, so a new
// Moodle error never has to break the contract.
var knownErrorCodes = map[string]classification{
	// Credential problems. The request was rejected before it ran, so a
	// retry after re-authenticating is safe.
	"invalidtoken": {errs.CodeAuthentication, errs.ReasonTokenExpired,
		"sign in again with `moodle auth login`"},
	"accessexception": {errs.CodeAuthentication, errs.ReasonTokenExpired,
		"sign in again with `moodle auth login`"},
	"invalidlogin": {errs.CodeAuthentication, "",
		"check the username and password"},
	"usernotfullysetup": {errs.CodeAuthentication, "",
		"finish setting up the account in a browser first"},

	// The site or the service is not offering what we need.
	"enablewsdescription": {errs.CodeUnavailable, errs.ReasonMobileServicesDisabled,
		"an administrator must enable web services on this site"},
	"servicenotavailable": {errs.CodeUnavailable, errs.ReasonMobileServicesDisabled,
		"an administrator must enable the mobile web service on this site"},
	"accessdenied": {errs.CodeUnavailable, errs.ReasonCapability, ""},
	"pluginnotenabledorconfigured": {errs.CodeUnavailable, errs.ReasonCapability,
		"this site is not configured for the app login flow"},
	"qrcodedisabled": {errs.CodeUnavailable, errs.ReasonCapability,
		"an administrator must enable QR login on this site"},
	"apprequired": {errs.CodeUnavailable, errs.ReasonCapability, ""},
	"httpsrequired": {errs.CodeUnavailable, errs.ReasonCapability,
		"this flow only works on an https site"},

	// Permissions: the user is who they say they are, but may not do this.
	"nopermissions":                 {errs.CodePermissionDenied, "", ""},
	"nopermissiontoviewpage":        {errs.CodePermissionDenied, "", ""},
	"required_capability_exception": {errs.CodePermissionDenied, "", ""},
	"cannotviewprofile":             {errs.CodePermissionDenied, "", ""},
	"autologinnotallowedtoadmins": {errs.CodePermissionDenied, "",
		"Moodle refuses this flow for site administrators; use a normal account"},

	// Bad input.
	"invalidparameter":          {errs.CodeValidation, "", ""},
	"invalidextparam":           {errs.CodeValidation, "", ""},
	"invalidrecord":             {errs.CodeNotFound, "", ""},
	"invalidrecordunknown":      {errs.CodeNotFound, "", ""},
	"invalidcoursemodule":       {errs.CodeNotFound, "", ""},
	"dmlmissingrecordexception": {errs.CodeNotFound, "", ""},
	"invalidkey":                {errs.CodeValidation, "", "the key is single-use and short-lived; get a fresh one"},

	// The site is up but refusing work.
	"maintenanceinprogress": {errs.CodeUnavailable, "", "the site is in maintenance mode"},
}

// asError converts a Moodle exception into our error type.
func (e exception) asError(function string) error {
	upstream := &errs.Upstream{
		Exception: e.Exception,
		ErrorCode: e.ErrorCode,
		Message:   firstNonEmpty(e.Message, e.Error),
	}
	message := e.Message
	if message == "" {
		message = e.Error
	}
	if message == "" {
		message = fmt.Sprintf("Moodle rejected %s", function)
	}

	known, ok := knownErrorCodes[e.ErrorCode]
	if !ok {
		// Unrecognised: report it as upstream and keep Moodle's own words.
		// Guessing a friendlier code here would hide a real failure.
		return &errs.Error{
			Code:     errs.CodeUpstream,
			Outcome:  errs.OutcomeKnown,
			Message:  message,
			Upstream: upstream,
		}
	}
	out := &errs.Error{
		Code:     known.code,
		Reason:   known.reason,
		Outcome:  errs.OutcomeKnown,
		Message:  message,
		Hint:     known.hint,
		Upstream: upstream,
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
