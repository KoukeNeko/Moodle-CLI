package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
)

// withQuiz puts one quiz on the fake site: 10 raw marks scaled to 20.
func (f *fixture) withQuiz() {
	f.t.Helper()
	f.server.HandleValue(moodle.FunctionQuizzes, map[string]any{
		"quizzes": []any{map[string]any{
			"id": 7, "coursemodule": 31, "course": 2, "name": "Quiz &amp; test",
			"timeopen": 1790006400, "timeclose": 0, "timelimit": 1800,
			"attempts": 0, "grade": 20, "sumgrades": 10,
		}},
		"warnings": []any{},
	})
}

func TestQuizListJSONSatisfiesSchema(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withQuiz()

	stdout, stderr, code := f.run("quiz", "list", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	validate(t, "quiz.list", stdout)
	var doc struct {
		Data []v1.Quiz `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	// The listing carries the course's id and nothing else about it, so a null
	// short name has to be declared rather than read as a course without one.
	if !strings.Contains(stdout, `"course_short_name":null`) ||
		!strings.Contains(stdout, `"course_short_name"`) {
		t.Errorf("course_short_name should be null here:\n%s", stdout)
	}
	var meta struct {
		Meta struct {
			Partial bool     `json:"partial"`
			Missing []string `json:"missing"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &meta); err != nil {
		t.Fatal(err)
	}
	if !meta.Meta.Partial || len(meta.Meta.Missing) == 0 || meta.Meta.Missing[0] != "course_short_name" {
		t.Errorf("meta should name the field this route cannot read: %+v", meta.Meta)
	}

	item := doc.Data[0]
	if item.Name != "Quiz & test" {
		t.Errorf("name = %q; the listing escapes it", item.Name)
	}
	// Zero is Moodle's "no closing date", and must not become 1970.
	if item.ClosesAt != nil {
		t.Errorf("closes_at = %v for a quiz that never closes", *item.ClosesAt)
	}
	// Zero attempts is unlimited in Moodle, which is an answer, not a gap.
	if item.MaxAttempts == nil || *item.MaxAttempts != 0 {
		t.Errorf("max_attempts = %v", item.MaxAttempts)
	}
	if item.TimeLimitSeconds == nil || *item.TimeLimitSeconds != 1800 {
		t.Errorf("time_limit_seconds = %v", item.TimeLimitSeconds)
	}
}

func TestQuizShowScalesAttemptMarksToTheQuizGrade(t *testing.T) {
	// An attempt's sumgrades is raw marks out of the quiz's own sumgrades.
	// Printing 7 for a quiz marked out of 20 would read as a fail.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withQuiz()
	f.server.HandleValue(moodle.FunctionQuizAttempts, map[string]any{
		"attempts": []any{
			map[string]any{"id": 91, "attempt": 1, "state": "finished",
				"timestart": 1790006400, "timefinish": 1790007000, "sumgrades": 7},
			map[string]any{"id": 92, "attempt": 2, "state": "inprogress",
				"timestart": 1790008000, "timefinish": 0, "sumgrades": nil},
		},
		"warnings": []any{},
	})
	f.server.HandleValue(moodle.FunctionBestGrade, map[string]any{"hasgrade": true, "grade": 14})

	stdout, stderr, code := f.run("quiz", "show", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	validate(t, "quiz.show", stdout)
	var doc struct {
		Data v1.QuizDetail `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Data.Attempts) != 2 {
		t.Fatalf("attempts = %+v", doc.Data.Attempts)
	}
	if grade := doc.Data.Attempts[0].Grade; grade == nil || *grade != 14 {
		t.Errorf("first attempt grade = %v, want 14 (7 of 10 marks, out of 20)", grade)
	}
	if doc.Data.Attempts[1].Grade != nil || doc.Data.Attempts[1].FinishedAt != nil {
		t.Errorf("an unfinished attempt has a grade or an end: %+v", doc.Data.Attempts[1])
	}
	if doc.Data.BestGrade == nil || *doc.Data.BestGrade != 14 {
		t.Errorf("best grade = %v", doc.Data.BestGrade)
	}

	human, _, _ := f.run("quiz", "show", "7")
	for _, want := range []string{"Quiz & test", "Best grade", "14.00", "in progress", "unlimited"} {
		if !strings.Contains(human, want) {
			t.Errorf("human output lacks %q:\n%s", want, human)
		}
	}
}

func TestQuizShowFindsTheQuizFromItsAddress(t *testing.T) {
	// The address carries the course module id, 31 here, not the quiz id 7.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withQuiz()
	f.server.HandleValue(moodle.FunctionQuizAttempts, map[string]any{"attempts": []any{}})
	f.server.HandleValue(moodle.FunctionBestGrade, map[string]any{"hasgrade": false})

	stdout, stderr, code := f.run("quiz", "show", f.server.URL()+"/mod/quiz/view.php?id=31", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, `"id":"7"`) || !strings.Contains(stdout, `"attempts":[]`) {
		t.Errorf("the address did not resolve to quiz 7 with no attempts:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"best_grade":null`) {
		t.Errorf("no grade yet must be null:\n%s", stdout)
	}
}
