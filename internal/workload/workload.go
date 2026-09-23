// Package workload calculates academic credits from institution-mapped Moodle
// course custom fields.
package workload

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Course is one enrolled course and the academic metadata Moodle exposed.
type Course struct {
	ID        string
	ShortName string
	FullName  string
	Credits   *float64
	Level     string
	Term      string
	Missing   []string
}

// Backend reads enrolled courses and their custom fields.
type Backend interface {
	Name() site.BackendKind
	Requirement() site.Requirement
	Courses(context.Context, string, config.Academic) ([]Course, error)
}

// Group is one term and academic level, which have one applicable minimum.
type Group struct {
	Term         string
	Level        string
	Credits      float64
	Minimum      float64
	MeetsMinimum bool
	Complete     bool
	Courses      []Course
}

// Result is a workload calculation plus provenance.
type Result struct {
	Groups     []Group
	AllMeet    bool
	Provenance site.Provenance
}

// Service calculates workload without knowing how Moodle was queried.
type Service struct{ backend Backend }

func NewService(backend Backend) *Service { return &Service{backend: backend} }

// Show calculates credits, optionally for one term.
func (s *Service) Show(ctx context.Context, capabilities *site.Capabilities, academic config.Academic, term string) (Result, error) {
	if err := academic.Validate(); err != nil {
		return Result{}, err
	}
	if s == nil || s.backend == nil {
		return Result{}, errs.New(errs.CodeUnavailable, "no workload backend is configured")
	}
	if ok, why := s.backend.Requirement().SatisfiedBy(capabilities); !ok {
		return Result{}, s.backend.Requirement().Unavailable(why)
	}
	if capabilities == nil || strings.TrimSpace(capabilities.UserID) == "" {
		return Result{}, errs.New(errs.CodeInternal, "the account's Moodle user id is unknown")
	}
	courses, err := s.backend.Courses(ctx, capabilities.UserID, academic)
	if err != nil {
		return Result{}, err
	}

	groups := map[string]*Group{}
	for _, course := range courses {
		if term != "" && course.Term != term {
			continue
		}
		key := course.Term + "\x00" + course.Level
		group := groups[key]
		if group == nil {
			minimum, known := minimumFor(academic, course.Level)
			group = &Group{
				Term: course.Term, Level: course.Level, Minimum: minimum,
				Complete: known && course.Term != "", Courses: []Course{},
			}
			groups[key] = group
		}
		group.Courses = append(group.Courses, course)
		if course.Credits == nil || len(course.Missing) > 0 {
			group.Complete = false
		} else {
			group.Credits += *course.Credits
		}
	}

	out := make([]Group, 0, len(groups))
	allMeet := true
	for _, group := range groups {
		group.MeetsMinimum = group.Complete && group.Credits >= group.Minimum
		if !group.MeetsMinimum {
			allMeet = false
		}
		sort.Slice(group.Courses, func(i, j int) bool {
			return group.Courses[i].ID < group.Courses[j].ID
		})
		out = append(out, *group)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Term == out[j].Term {
			return out[i].Level < out[j].Level
		}
		return out[i].Term < out[j].Term
	})
	return Result{Groups: out, AllMeet: allMeet, Provenance: site.NewProvenance(s.backend.Name())}, nil
}

func minimumFor(academic config.Academic, level string) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "undergraduate", "undergrad", "ug":
		return academic.Minimum.Undergraduate, true
	case "graduate", "postgraduate", "grad", "pg":
		return academic.Minimum.Graduate, true
	case "noncredit", "non-credit", "orientation":
		// Institutions commonly enrol every account in a zero-credit
		// orientation or policy course. It belongs in the visible workload,
		// but must not manufacture a second failed academic programme.
		return 0, true
	default:
		return 0, false
	}
}

// ValidationError explains why --require-minimum failed.
func ValidationError(result Result) error {
	failing := []string{}
	for _, group := range result.Groups {
		if !group.MeetsMinimum {
			failing = append(failing, fmt.Sprintf("%s/%s %.2f of %.2f", group.Term, group.Level, group.Credits, group.Minimum))
		}
	}
	if len(failing) == 0 {
		return nil
	}
	return errs.New(errs.CodeValidation, "academic workload minimum is not met").
		WithHint(strings.Join(failing, "; "))
}
