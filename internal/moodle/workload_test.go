package moodle_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/tests/testmoodle"
)

func TestWorkloadLoadsEveryCourseInTwoRequests(t *testing.T) {
	server := testmoodle.New()
	t.Cleanup(server.Close)
	enrolments := make([]any, 0, 50)
	details := make([]any, 0, 50)
	for i := 1; i <= 50; i++ {
		enrolments = append(enrolments, map[string]any{
			"id": i, "shortname": fmt.Sprintf("C%03d", i), "fullname": fmt.Sprintf("Course %d", i),
		})
		details = append(details, map[string]any{
			"id": i, "shortname": fmt.Sprintf("C%03d", i), "fullname": fmt.Sprintf("Course %d", i),
			"customfields": []any{
				map[string]any{"shortname": "credits", "valueraw": 3},
				map[string]any{"shortname": "academic_level", "valueraw": "undergraduate"},
				map[string]any{"shortname": "academic_term", "valueraw": "2026-Fall"},
			},
		})
	}
	server.HandleValue(moodle.FunctionUserCourses, enrolments)
	server.HandleValue(moodle.FunctionCoursesByField, map[string]any{"courses": details})

	backend := moodle.NewWorkloadBackend(newClient(t, server), "token")
	courses, err := backend.Courses(context.Background(), "4", config.Academic{
		CreditsField: "credits", LevelField: "academic_level", TermField: "academic_term",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(courses) != 50 {
		t.Fatalf("courses = %d", len(courses))
	}
	if server.CallsTo(moodle.FunctionUserCourses) != 1 || server.CallsTo(moodle.FunctionCoursesByField) != 1 {
		t.Fatalf("request counts: enrol=%d detail=%d", server.CallsTo(moodle.FunctionUserCourses), server.CallsTo(moodle.FunctionCoursesByField))
	}
}
