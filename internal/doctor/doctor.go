// Package doctor diagnoses a site and account.
//
// Its whole reason to exist is to tell apart the causes that look identical
// from the outside: the network is down, the administrator disabled web
// services, the token expired, or the site simply does not expose a function.
// "Moodle connection failed" is the answer this package is meant to replace.
package doctor

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Status is the outcome of one check.
type Status string

const (
	StatusOK      Status = "ok"
	StatusWarning Status = "warning"
	StatusFailed  Status = "failed"
	// StatusSkipped means an earlier check made this one impossible to run,
	// which is different from the check failing.
	StatusSkipped Status = "skipped"
)

// Check is one diagnosis.
type Check struct {
	Name   string
	Status Status
	Detail string
	Reason errs.Reason
	Hint   string
}

// Report is the full diagnosis.
type Report struct {
	SiteName    string
	SiteURL     string
	AccountName string
	Checks      []Check
}

// OK reports whether nothing failed.
func (r Report) OK() bool {
	for _, check := range r.Checks {
		if check.Status == StatusFailed {
			return false
		}
	}
	return true
}

// Input is everything doctor needs.
type Input struct {
	SiteName    string
	SiteURL     string
	AccountName string
	Probe       auth.Prober
	// Session is nil when no account is configured yet, which is a normal
	// state rather than a failure.
	Session auth.Capabilitier
}

// featureChecks are the capability groups worth reporting on. Each names the
// functions a feature needs, so the report says which one is missing instead
// of just "unavailable".
var featureChecks = []struct {
	name        string
	anyFunction []string
}{
	{"Course listing", []string{
		"core_enrol_get_users_courses",
		"core_course_get_enrolled_courses_by_timeline_classification",
	}},
	{"Assignments", []string{"mod_assign_get_assignments"}},
	{"Assignment submission", []string{"mod_assign_save_submission"}},
	{"Grades", []string{"gradereport_overview_get_course_grades", "gradereport_user_get_grade_items"}},
	{"Calendar", []string{"core_calendar_get_calendar_upcoming_view", "core_calendar_get_action_events_by_timesort"}},
	{"Forums", []string{"mod_forum_get_forums_by_courses"}},
}

// Run performs the diagnosis. It never returns an error for a site problem:
// a site that is down is a finding, not a crash.
func Run(ctx context.Context, in Input) Report {
	report := Report{SiteName: in.SiteName, SiteURL: in.SiteURL, AccountName: in.AccountName}

	config, err := in.Probe.PublicConfig(ctx)
	if err != nil {
		e := errs.From(err)
		report.Checks = append(report.Checks, Check{
			Name:   "Site reachable",
			Status: StatusFailed,
			Detail: e.Error(),
			Reason: e.Reason,
			Hint:   hintFor(e),
		})
		// Everything below needs the site to answer at all.
		report.Checks = append(report.Checks, skipped("Web services", "the site did not answer"))
		report.Checks = append(report.Checks, skipped("Authentication", "the site did not answer"))
		return report
	}

	report.Checks = append(report.Checks, Check{
		Name: "Site reachable", Status: StatusOK, Detail: config.SiteName,
	})

	if config.MaintenanceEnabled == 1 {
		report.Checks = append(report.Checks, Check{
			Name: "Maintenance mode", Status: StatusWarning,
			Detail: "the site is in maintenance mode",
		})
	}

	wsCheck := Check{Name: "Web services", Status: StatusOK, Detail: "enabled"}
	switch {
	case config.EnableWebServices != 1:
		wsCheck = Check{
			Name: "Web services", Status: StatusFailed,
			Detail: "disabled on this site",
			Reason: errs.ReasonMobileServicesDisabled,
			Hint:   "an administrator must enable web services in Site administration",
		}
	case config.EnableMobileWebService != 1:
		wsCheck = Check{
			Name: "Web services", Status: StatusFailed,
			Detail: "the mobile web service is disabled",
			Reason: errs.ReasonMobileServicesDisabled,
			Hint:   "an administrator must enable the mobile web service, or sign in with --method browser-session",
		}
	}
	report.Checks = append(report.Checks, wsCheck)

	report.Checks = append(report.Checks, loginMethods(config))

	if in.Session == nil {
		report.Checks = append(report.Checks, Check{
			Name: "Authentication", Status: StatusWarning,
			Detail: "no account is signed in",
			Reason: errs.ReasonCredentialMissing,
			Hint:   "sign in with `moodle auth login`",
		})
		return report
	}

	capabilities, err := in.Session.Capabilities(ctx)
	if err != nil {
		e := errs.From(err)
		report.Checks = append(report.Checks, Check{
			Name:   "Authentication",
			Status: StatusFailed,
			Detail: e.Error(),
			Reason: e.Reason,
			Hint:   hintFor(e),
		})
		return report
	}

	report.Checks = append(report.Checks, Check{
		Name: "Authentication", Status: StatusOK,
		Detail: describeAccount(capabilities),
	})
	report.Checks = append(report.Checks, Check{
		Name: "Moodle version", Status: StatusOK, Detail: capabilities.Release,
	})
	report.Checks = append(report.Checks, fileCheck("File download", capabilities.CanDownload))
	report.Checks = append(report.Checks, fileCheck("File upload", capabilities.CanUpload))

	for _, feature := range featureChecks {
		requirement := site.Requirement{AnyFunction: feature.anyFunction}
		if ok, why := requirement.SatisfiedBy(capabilities); ok {
			report.Checks = append(report.Checks, Check{
				Name: feature.name, Status: StatusOK, Detail: "available",
			})
		} else {
			// Not a failure: a site that does not expose forums is not broken,
			// it just cannot do that. The detail names the missing function.
			report.Checks = append(report.Checks, Check{
				Name: feature.name, Status: StatusWarning, Detail: why,
				Reason: errs.ReasonCapability,
			})
		}
	}
	return report
}

func loginMethods(config *auth.PublicConfig) Check {
	var available []string
	if config.EnableMobileWebService == 1 {
		available = append(available, "token")
		if config.ShowLoginForm == 1 {
			available = append(available, "password")
		}
		if config.QRCodeType == 2 {
			available = append(available, "qr")
		}
		if config.TypeOfLogin == 2 || config.TypeOfLogin == 3 || config.HasIdentityProviders {
			available = append(available, "browser-sso")
		}
	}
	if len(available) == 0 {
		return Check{
			Name: "Login methods", Status: StatusWarning,
			Detail: "no web service login method is available",
			Hint:   "try `moodle auth login --method browser-session`",
		}
	}
	return Check{Name: "Login methods", Status: StatusOK, Detail: join(available)}
}

func fileCheck(name string, allowed bool) Check {
	if allowed {
		return Check{Name: name, Status: StatusOK, Detail: "allowed"}
	}
	return Check{
		Name: name, Status: StatusWarning, Detail: "not allowed for this account",
		Reason: errs.ReasonCapability,
	}
}

func describeAccount(c *site.Capabilities) string {
	if c.FullName != "" && c.Username != "" {
		return c.FullName + " (" + c.Username + ")"
	}
	if c.Username != "" {
		return c.Username
	}
	return "signed in"
}

func skipped(name, why string) Check {
	return Check{Name: name, Status: StatusSkipped, Detail: why}
}

func hintFor(e *errs.Error) string {
	if e.Hint != "" {
		return e.Hint
	}
	switch e.Code {
	case errs.CodeNetwork:
		return "check the site URL and your network connection"
	case errs.CodeAuthentication:
		return "sign in again with `moodle auth login`"
	default:
		return ""
	}
}

func join(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += ", "
		}
		out += item
	}
	return out
}
