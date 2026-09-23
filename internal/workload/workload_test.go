package workload_test

import (
	"context"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/internal/workload"
)

type backend struct{ courses []workload.Course }

func (backend) Name() site.BackendKind        { return site.BackendWS }
func (backend) Requirement() site.Requirement { return site.Requirement{} }
func (b backend) Courses(context.Context, string, config.Academic) ([]workload.Course, error) {
	return b.courses, nil
}

func number(value float64) *float64 { return &value }

func academic() config.Academic {
	return config.Academic{
		CreditsField: "credits", LevelField: "academic_level", TermField: "academic_term",
		Minimum: config.AcademicMinimum{Undergraduate: 21, Graduate: 6},
	}
}

func TestCreditsAreGroupedByTermAndLevel(t *testing.T) {
	courses := []workload.Course{}
	for i := 0; i < 7; i++ {
		courses = append(courses, workload.Course{
			ID: string(rune('a' + i)), Term: "2026-Fall", Level: "undergraduate", Credits: number(3), Missing: []string{},
		})
	}
	for i := 0; i < 2; i++ {
		courses = append(courses, workload.Course{
			ID: string(rune('h' + i)), Term: "2026-Fall", Level: "graduate", Credits: number(3), Missing: []string{},
		})
	}
	capabilities := site.NewCapabilities()
	capabilities.UserID = "4"
	result, err := workload.NewService(backend{courses: courses}).Show(context.Background(), capabilities, academic(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Groups) != 2 || !result.AllMeet {
		t.Fatalf("result = %+v", result)
	}
	if result.Groups[0].Credits != 6 || result.Groups[1].Credits != 21 {
		t.Fatalf("wrong totals: %+v", result.Groups)
	}
}

func TestMissingMetadataCannotPassTheMinimum(t *testing.T) {
	capabilities := site.NewCapabilities()
	capabilities.UserID = "4"
	result, err := workload.NewService(backend{courses: []workload.Course{{
		ID: "2", Term: "2026-Fall", Level: "undergraduate", Missing: []string{"credits"},
	}}}).Show(context.Background(), capabilities, academic(), "")
	if err != nil {
		t.Fatal(err)
	}
	if result.AllMeet || result.Groups[0].Complete || result.Groups[0].MeetsMinimum {
		t.Fatalf("incomplete metadata passed: %+v", result)
	}
	if err := workload.ValidationError(result); err == nil || errs.From(err).Code != errs.CodeValidation {
		t.Fatalf("validation error = %v", err)
	}
}

func TestNonCreditOrientationIsVisibleButHasNoMinimum(t *testing.T) {
	capabilities := site.NewCapabilities()
	capabilities.UserID = "4"
	result, err := workload.NewService(backend{courses: []workload.Course{
		{ID: "2", Term: "orientation", Level: "noncredit", Credits: number(0), Missing: []string{}},
	}}).Show(context.Background(), capabilities, academic(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Groups) != 1 || !result.AllMeet || !result.Groups[0].MeetsMinimum {
		t.Fatalf("zero-credit orientation affected an academic minimum: %+v", result)
	}
}
