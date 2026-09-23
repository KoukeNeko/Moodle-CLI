package v1

import (
	"encoding/json"

	"github.com/KoukeNeko/moodle-cli/internal/workload"
)

// SiteAcademicConfiguration is the site.academic.configure payload.
type SiteAcademicConfiguration struct {
	Site                 string  `json:"site"`
	CreditsField         string  `json:"credits_field"`
	LevelField           string  `json:"level_field"`
	TermField            string  `json:"term_field"`
	UndergraduateMinimum float64 `json:"undergraduate_minimum"`
	GraduateMinimum      float64 `json:"graduate_minimum"`
	DryRun               bool    `json:"dry_run"`
}

// WorkloadCourse is one course contributing to a term total.
type WorkloadCourse struct {
	ID        string   `json:"id"`
	ShortName string   `json:"short_name"`
	FullName  string   `json:"full_name"`
	Credits   *float64 `json:"credits"`
	Level     string   `json:"level"`
	Term      string   `json:"term"`
	Missing   []string `json:"missing"`
}

// WorkloadGroup is one term/level comparison against its minimum.
type WorkloadGroup struct {
	Term         string           `json:"term"`
	Level        string           `json:"level"`
	Credits      float64          `json:"credits"`
	Minimum      float64          `json:"minimum"`
	MeetsMinimum bool             `json:"meets_minimum"`
	Complete     bool             `json:"complete"`
	Courses      []WorkloadCourse `json:"courses"`
}

// WorkloadResult is shared by workload.show and workload.validate.
type WorkloadResult struct {
	AllMeetMinimum bool            `json:"all_meet_minimum"`
	Groups         []WorkloadGroup `json:"groups"`
}

// Workload builds a workload response with backend provenance.
func Workload(kind string, result workload.Result, siteName, accountName string) Envelope {
	groups := make([]WorkloadGroup, 0, len(result.Groups))
	for _, group := range result.Groups {
		courses := make([]WorkloadCourse, 0, len(group.Courses))
		for _, course := range group.Courses {
			missing := course.Missing
			if missing == nil {
				missing = []string{}
			}
			courses = append(courses, WorkloadCourse{
				ID: course.ID, ShortName: course.ShortName, FullName: course.FullName,
				Credits: course.Credits, Level: course.Level, Term: course.Term, Missing: missing,
			})
		}
		groups = append(groups, WorkloadGroup{
			Term: group.Term, Level: group.Level, Credits: group.Credits,
			Minimum: group.Minimum, MeetsMinimum: group.MeetsMinimum,
			Complete: group.Complete, Courses: courses,
		})
	}
	return NewEnvelope(kind, WorkloadResult{AllMeetMinimum: result.AllMeet, Groups: groups},
		MetaFrom(result.Provenance, siteName, accountName))
}

// WorkflowCallResult is the common envelope for task-oriented wrappers around
// generated core functions.
type WorkflowCallResult struct {
	Operation   string          `json:"operation"`
	Function    string          `json:"function"`
	Version     string          `json:"version"`
	Params      map[string]any  `json:"params"`
	Response    json.RawMessage `json:"response"`
	DryRun      bool            `json:"dry_run"`
	Effect      string          `json:"effect"`
	Destructive bool            `json:"destructive"`
}
