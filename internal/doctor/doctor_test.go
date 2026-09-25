package doctor_test

import (
	"context"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/doctor"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

type fakeProbe struct{ config *auth.PublicConfig }

func (f fakeProbe) PublicConfig(context.Context) (*auth.PublicConfig, error) {
	return f.config, nil
}

type fakeSession struct{ capabilities *site.Capabilities }

func (f fakeSession) Capabilities(context.Context) (*site.Capabilities, error) {
	return f.capabilities, nil
}

// noWebServices is the site this project keeps as a test fixture: mobile web
// services switched off, so no token can be issued and a browser session is
// the only way in.
func noWebServices() *auth.PublicConfig {
	return &auth.PublicConfig{
		SiteName: "Dockerized_Moodle", EnableWebServices: 0,
		EnableMobileWebService: 0, ShowLoginForm: 1,
	}
}

// browserSession is what a session holds on such a site: the call that says
// what the site offers is not exposed over the AJAX endpoint, so the answer
// is empty rather than negative.
func browserSession() *site.Capabilities {
	capabilities := site.NewCapabilities()
	capabilities.Credential = site.CredentialBrowserSession
	return capabilities
}

func run(t *testing.T, config *auth.PublicConfig, capabilities *site.Capabilities) doctor.Report {
	t.Helper()
	return doctor.Run(context.Background(), doctor.Input{
		SiteName: "nows", SiteURL: "http://127.0.0.1:8522",
		Probe:   fakeProbe{config: config},
		Session: fakeSession{capabilities: capabilities},
	})
}

func find(t *testing.T, report doctor.Report, name string) doctor.Check {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("no check called %q in %+v", name, report.Checks)
	return doctor.Check{}
}

func TestASiteWithoutWebServicesIsNotReportedAsUnusable(t *testing.T) {
	// Measured on the fixture site: six of the seven features below work
	// through the fallback routes. A failed diagnosis exits 9, which tells a
	// script the site cannot be used while the commands beside it succeed.
	report := run(t, noWebServices(), browserSession())
	if !report.OK() {
		t.Errorf("a site this account is signed in to was reported as failed:\n%+v",
			report.Checks)
	}
	if got := find(t, report, "Web services").Status; got != doctor.StatusWarning {
		t.Errorf("web services status = %q, want a finding rather than a failure", got)
	}
}

func TestTheFeaturesSayWhichRouteAnswersThem(t *testing.T) {
	report := run(t, noWebServices(), browserSession())
	for _, want := range []struct {
		name   string
		status doctor.Status
		detail string
	}{
		{"Course listing", doctor.StatusOK, "ajax"},
		{"Assignments", doctor.StatusOK, "html"},
		{"Grades", doctor.StatusOK, "html"},
		{"Calendar", doctor.StatusOK, "ajax"},
		{"Forums", doctor.StatusOK, "html"},
		// The one with no fallback, and the reason: reading a page cannot see
		// whether saving is submitting.
		{"Assignment submission", doctor.StatusWarning, "browser session"},
	} {
		check := find(t, report, want.name)
		if check.Status != want.status {
			t.Errorf("%s: status = %q, want %q (%s)",
				want.name, check.Status, want.status, check.Detail)
		}
		if !strings.Contains(check.Detail, want.detail) {
			t.Errorf("%s: detail = %q, want it to mention %q",
				want.name, check.Detail, want.detail)
		}
	}
}

func TestWhatWasNeverAskedIsNotReportedAsNo(t *testing.T) {
	// CanDownload is false here because nothing asked, not because the site
	// refused. Rendering that as a refusal tells a student they may not
	// download their own coursework.
	report := run(t, noWebServices(), browserSession())
	for _, name := range []string{"File upload", "Moodle version"} {
		check := find(t, report, name)
		if check.Status != doctor.StatusSkipped {
			t.Errorf("%s: status = %q, want skipped — the question was never put",
				name, check.Status)
		}
		if strings.Contains(check.Detail, "not allowed") {
			t.Errorf("%s: %q reads as a refusal", name, check.Detail)
		}
	}
}

func TestABrowserSessionDownloadsWhatPagesLinkTo(t *testing.T) {
	// The session fetches pluginfile.php the way its browser does, so the
	// download row is an answer here, not a question never put.
	report := run(t, noWebServices(), browserSession())
	if check := find(t, report, "File download"); check.Status != doctor.StatusOK {
		t.Errorf("File download: status = %q, want ok (%s)", check.Status, check.Detail)
	}
}

func TestASiteWithWebServicesIsUnaffected(t *testing.T) {
	config := &auth.PublicConfig{
		SiteName: "Dockerized_Moodle", EnableWebServices: 1,
		EnableMobileWebService: 1, ShowLoginForm: 1,
	}
	capabilities := site.NewCapabilities()
	capabilities.Credential = site.CredentialWSToken
	capabilities.Release = "5.2.3"
	capabilities.CanDownload = true
	for _, name := range []string{
		"core_enrol_get_users_courses", "mod_assign_get_assignments",
		"mod_assign_save_submission", "gradereport_user_get_grade_items",
		"core_calendar_get_action_events_by_timesort",
		"mod_forum_get_forums_by_courses",
	} {
		capabilities.Functions[name] = site.FunctionInfo{Name: name}
	}

	report := run(t, config, capabilities)
	if !report.OK() {
		t.Fatalf("a working site was reported as failed:\n%+v", report.Checks)
	}
	// "available", with no talk of routes: there is only one here.
	if detail := find(t, report, "Forums").Detail; detail != "available" {
		t.Errorf("Forums detail = %q on a site with a token", detail)
	}
	if detail := find(t, report, "File download").Detail; detail != "allowed" {
		t.Errorf("File download detail = %q", detail)
	}
}
