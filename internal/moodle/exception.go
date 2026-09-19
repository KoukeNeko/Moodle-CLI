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
	// The session behind a browser-session credential is gone. Moodle says so
	// in as many words, and it is the same problem as an expired token: the
	// request never ran, so re-authenticating is the whole fix.
	"servicerequireslogin": {errs.CodeAuthentication, errs.ReasonTokenExpired,
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
	//
	// requireloginerror arrives as "Course or activity not accessible" from
	// require_login() when the caller holds a valid session but is not on the
	// course — an account reading a course it never enrolled in. An expired
	// session never reaches here: it is caught earlier, at the login page the
	// site serves instead of an answer.
	"nopermissions": {errs.CodePermissionDenied, "", ""},
	// 單數的 nopermission 是 required_capability_exception 的 errorcode，跟
	// 複數那個不是同一個碼。少了它，一個「你在這門課沒有這個權限」會被報成
	// 上游錯誤——指向站台，而該做的是換一個帳號或換一門課。
	"nopermission":                  {errs.CodePermissionDenied, "", ""},
	"nopermissiontoviewpage":        {errs.CodePermissionDenied, "", ""},
	"requireloginerror":             {errs.CodePermissionDenied, "", ""},
	"required_capability_exception": {errs.CodePermissionDenied, "", ""},
	"cannotviewprofile":             {errs.CodePermissionDenied, "", ""},
	// notingroup 是分組擋下來的：在獨立分組的活動上問一個自己不屬於的組，
	// Moodle 丟 moodle_exception 而不是 required_capability_exception，所以
	// 它不在上面那幾個碼裡。沒分類時會報成上游錯誤，訊息還是站台沒翻到的
	// `error/notingroup`——看起來像站台壞了，而該換的是問題裡的那個組。
	"notingroup": {errs.CodePermissionDenied, "",
		"this account is not in that group, and the activity separates them"},
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
	// The activity is on its way out: Moodle has accepted a delete and is
	// working through it. Nothing is wrong with the site, and nothing here
	// will start working again, so reporting an upstream fault invited a
	// retry that can only ever end in the activity being gone.
	"activityisscheduledfordeletion": {errs.CodeUnavailable, "",
		"the site is deleting this activity; it will not come back"},
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
