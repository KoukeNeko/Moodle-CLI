package v1

import (
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/doctor"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// The public JSON shapes live here, not in the command layer.
//
// They are the contract, and more than one presentation has to emit them: the
// CLI today, MCP in Phase 9. Defining them next to the Cobra commands would
// force MCP either to duplicate them or to import the CLI, which the import
// rules forbid.

// Course is one course on the wire.
type Course struct {
	// ID is a string: an identifier is opaque, not a number to compute with
	//.
	ID        string   `json:"id"`
	ShortName string   `json:"short_name"`
	FullName  string   `json:"full_name"`
	StartDate *string  `json:"start_date"`
	EndDate   *string  `json:"end_date"`
	Visible   bool     `json:"visible"`
	Progress  *float64 `json:"progress"`
}

// CourseList converts a listing into its envelope.
func CourseList(result course.ListResult, siteName, accountName string) Envelope {
	courses := make([]Course, 0, len(result.Courses))
	for _, item := range result.Courses {
		courses = append(courses, Course{
			ID:        item.ID,
			ShortName: item.ShortName,
			FullName:  item.FullName,
			StartDate: Timestamp(item.StartDate),
			EndDate:   Timestamp(item.EndDate),
			Visible:   item.Visible,
			Progress:  item.Progress,
		})
	}
	meta := MetaFrom(result.Provenance, siteName, accountName)
	if result.NextCursor != "" {
		next := result.NextCursor
		meta.NextCursor = &next
	}
	return NewEnvelope("course.list", courses, meta)
}

// Check is one doctor finding on the wire.
type Check struct {
	Name   string  `json:"name"`
	Status string  `json:"status"`
	Detail string  `json:"detail"`
	Reason *string `json:"reason"`
	Hint   *string `json:"hint"`
}

// Diagnosis is the doctor payload.
type Diagnosis struct {
	Site    string  `json:"site"`
	SiteURL string  `json:"site_url"`
	Account *string `json:"account"`
	OK      bool    `json:"ok"`
	Checks  []Check `json:"checks"`
}

// DoctorReport converts a diagnosis into its envelope.
func DoctorReport(report doctor.Report) Envelope {
	payload := Diagnosis{
		Site: report.SiteName, SiteURL: report.SiteURL,
		OK: report.OK(), Checks: []Check{},
	}
	payload.Account = optional(report.AccountName)
	for _, check := range report.Checks {
		payload.Checks = append(payload.Checks, Check{
			Name:   check.Name,
			Status: string(check.Status),
			Detail: check.Detail,
			Reason: optional(string(check.Reason)),
			Hint:   optional(check.Hint),
		})
	}
	return NewEnvelope("doctor", payload, NewMeta(SourceWS))
}

// SiteCapabilities is the site.inspect payload.
type SiteCapabilities struct {
	Site          string   `json:"site"`
	SiteURL       string   `json:"site_url"`
	SiteName      string   `json:"site_name"`
	Release       string   `json:"release"`
	Username      string   `json:"username"`
	UserID        string   `json:"user_id"`
	CanUpload     bool     `json:"can_upload"`
	CanDownload   bool     `json:"can_download"`
	FunctionCount int      `json:"function_count"`
	Functions     []string `json:"functions"`
}

// SiteInspect converts capabilities into their envelope. The function list is
// included only when asked for: it is long, but the count is always reported
// so it stays comparable between sites.
func SiteInspect(siteName, siteURL string, capabilities *site.Capabilities, includeFunctions bool) Envelope {
	names := capabilities.FunctionNames()
	payload := SiteCapabilities{
		Site: siteName, SiteURL: siteURL,
		SiteName: capabilities.SiteName, Release: capabilities.Release,
		Username: capabilities.Username, UserID: capabilities.UserID,
		CanUpload: capabilities.CanUpload, CanDownload: capabilities.CanDownload,
		FunctionCount: len(names), Functions: []string{},
	}
	if includeFunctions {
		payload.Functions = names
	}
	return NewEnvelope("site.inspect", payload, NewMeta(SourceWS))
}

// AuthStatus is the auth.login and auth.status payload.
type AuthStatus struct {
	Site     string  `json:"site"`
	Account  *string `json:"account"`
	Username *string `json:"username"`
	UserID   *string `json:"user_id"`
	FullName *string `json:"full_name"`
	SiteName *string `json:"site_name"`
	Release  *string `json:"release"`
	Valid    bool    `json:"valid"`
}

// Timestamp formats a time for the contract: RFC 3339 in UTC, or null.
//
// A nil time stays null. Moodle uses 0 for "not set", and rendering that as
// 1970 would be a wrong answer that looks like a real one.
func Timestamp(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

// MetaFrom builds response metadata from a backend's provenance.
func MetaFrom(provenance site.Provenance, siteName, accountName string) Meta {
	meta := Meta{
		Source:  Source(provenance.Source),
		Partial: provenance.Partial,
		Missing: provenance.Missing,
	}
	if meta.Missing == nil {
		meta.Missing = []string{}
	}
	if meta.Source == "" {
		meta.Source = SourceLocal
	}
	meta.Site = optional(siteName)
	meta.Account = optional(accountName)
	return meta
}

// optional returns nil for an empty string, so an absent value is null rather
// than "".
func optional(value string) *string {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}
