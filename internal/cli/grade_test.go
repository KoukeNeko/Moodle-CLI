package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
)

// gradeItem builds one row of Moodle's grade report. Only the fields this
// project reads are set; Moodle sends many more, and some of them appear only
// once an item has been graded.
func gradeItem(id int, name, itemtype string, extra map[string]any) map[string]any {
	item := map[string]any{
		"id": id, "itemname": name, "itemtype": itemtype,
		"itemmodule": "assign", "cmid": id + 10, "scaleid": nil,
		"graderaw": nil, "grademin": 0, "grademax": 100,
		"gradeformatted": "-", "gradedategraded": nil,
		"gradeishidden": false, "gradeislocked": nil,
		"feedback": "", "feedbackformat": 0,
	}
	for k, v := range extra {
		item[k] = v
	}
	return item
}

func (f *fixture) withGrades(items ...map[string]any) {
	f.t.Helper()
	f.server.HandleValue(moodle.FunctionGradeItems, map[string]any{
		"usergrades": []any{map[string]any{
			"courseid": 2, "userid": 4, "gradeitems": toAny(items),
		}},
	})
}

func toAny(items []map[string]any) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func gradeReport(t *testing.T, stdout string) v1.GradeReport {
	t.Helper()
	var doc struct {
		Data v1.GradeReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Data
}

func TestUnmarkedWorkIsNullAndNotZero(t *testing.T) {
	// Reporting an unmarked assignment as zero tells a student they failed
	// something nobody has looked at yet.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withGrades(gradeItem(2, "Essay 1", "mod", nil))

	stdout, stderr, code := f.run("grade", "list", "--course", "2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "grade.list", stdout)

	report := gradeReport(t, stdout)
	if len(report.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(report.Items))
	}
	if report.Items[0].Grade != nil {
		t.Errorf("grade = %v, want null", *report.Items[0].Grade)
	}
	if report.Items[0].Percentage != nil {
		t.Errorf("percentage = %v, want null", *report.Items[0].Percentage)
	}
}

func TestZeroIsReportedAsAMark(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withGrades(gradeItem(2, "Essay 1", "mod", map[string]any{
		"graderaw": 0, "gradeformatted": "0.00", "gradedategraded": 1789000000,
	}))

	stdout, _, code := f.run("grade", "list", "--course", "2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	report := gradeReport(t, stdout)
	if report.Items[0].Grade == nil || *report.Items[0].Grade != 0 {
		t.Fatalf("grade = %v, want 0", report.Items[0].Grade)
	}
	if report.Items[0].Percentage == nil || *report.Items[0].Percentage != 0 {
		t.Errorf("percentage = %v, want 0", report.Items[0].Percentage)
	}
}

func TestAHeldBackGradeIsNotReportedAsUnmarked(t *testing.T) {
	// The site is withholding it. Saying "not marked" would tell the student
	// their work was never looked at.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withGrades(gradeItem(2, "Essay 1", "mod", map[string]any{"gradeishidden": true}))

	stdout, _, code := f.run("grade", "list", "--course", "2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	report := gradeReport(t, stdout)
	if !report.Items[0].Hidden {
		t.Error("a withheld grade was not reported as withheld")
	}

	human, _, _ := f.run("grade", "list", "--course", "2")
	if !strings.Contains(human, "hidden") {
		t.Errorf("the human output does not distinguish a withheld grade:\n%s", human)
	}
}

func TestAScaleGetsNoPercentageAndKeepsMoodlesWording(t *testing.T) {
	// The raw value is a position in a list of words. A percentage computed
	// from it would look meaningful and be nonsense.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withGrades(gradeItem(2, "Presentation", "mod", map[string]any{
		"scaleid": 4, "graderaw": 3, "grademin": 1, "grademax": 5,
		"gradeformatted": "Good", "gradedategraded": 1789000000,
	}))

	stdout, _, code := f.run("grade", "list", "--course", "2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	report := gradeReport(t, stdout)
	if report.Items[0].Percentage != nil {
		t.Errorf("percentage = %v, want null for a scale", *report.Items[0].Percentage)
	}
	if report.Items[0].Display != "Good" {
		t.Errorf("display = %q, want Moodle's own wording", report.Items[0].Display)
	}

	human, _, _ := f.run("grade", "list", "--course", "2")
	if !strings.Contains(human, "Good") {
		t.Errorf("the scale's wording did not reach the reader:\n%s", human)
	}
}

func TestTheCourseTotalIsNotListedAsAnActivity(t *testing.T) {
	// Moodle sends it in the same array with no name. Left in place it shows
	// up as a nameless assignment, and its maximum counts only what has been
	// marked so far — so it is never the sum of the rows above it.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withGrades(
		gradeItem(2, "Essay 1", "mod", map[string]any{
			"graderaw": 85, "gradeformatted": "85.00", "gradedategraded": 1789000000,
		}),
		gradeItem(3, "Essay 2", "mod", nil),
		map[string]any{
			"id": 1, "itemname": nil, "itemtype": "course", "itemmodule": nil,
			"cmid": nil, "scaleid": nil, "graderaw": 85, "grademin": 0,
			"grademax": 100, "gradeformatted": "85.00", "gradedategraded": nil,
			"gradeishidden": false, "gradeislocked": nil,
			"feedback": "", "feedbackformat": 0,
		},
	)

	stdout, _, code := f.run("grade", "list", "--course", "2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "grade.list", stdout)

	report := gradeReport(t, stdout)
	if len(report.Items) != 2 {
		t.Fatalf("got %d items, want 2 — the course total is not one of them", len(report.Items))
	}
	for _, item := range report.Items {
		if item.Kind == "course" {
			t.Error("the course total is listed among the activities")
		}
	}
	if report.Total == nil {
		t.Fatal("the course total is missing")
	}
	if report.Total.Grade == nil || *report.Total.Grade != 85 {
		t.Errorf("total = %v, want 85", report.Total.Grade)
	}

	human, _, _ := f.run("grade", "list", "--course", "2")
	// The number is surprising on its own: two items, one marked, total 85.
	// Saying why is the difference between a right answer and a trusted one.
	if !strings.Contains(human, "counting only what has been marked so far") {
		t.Errorf("the human output does not explain the total:\n%s", human)
	}
}

func TestGradeListNeedsACourse(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	_, stderr, code := f.run("grade", "list")
	if code != v1.ExitUsage {
		t.Fatalf("exit %d, want %d", code, v1.ExitUsage)
	}
	if !strings.Contains(stderr, "overview") {
		t.Errorf("the error does not point at the command for every course:\n%s", stderr)
	}
}

func TestGradeOverviewAcrossCourses(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	f.server.HandleValue(moodle.FunctionCourseGrades, map[string]any{
		"grades": []any{
			map[string]any{"courseid": 2, "grade": "85.00", "rawgrade": 85},
			map[string]any{"courseid": 3, "grade": "-", "rawgrade": nil},
		},
	})

	stdout, stderr, code := f.run("grade", "overview", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "grade.overview", stdout)

	var doc struct {
		Data []v1.CourseTotal `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Data) != 2 {
		t.Fatalf("got %d courses, want 2", len(doc.Data))
	}
	if doc.Data[1].Grade != nil {
		t.Errorf("an ungraded course reported %v", *doc.Data[1].Grade)
	}

	human, _, _ := f.run("grade", "overview")
	if !strings.Contains(human, "not graded yet") {
		t.Errorf("an ungraded course reads as a grade of '-':\n%s", human)
	}
}

func TestGradesAskForTheCallersOwnId(t *testing.T) {
	// Without it Moodle reads the call as a request for everyone's grades and
	// refuses it as "View grades of other users" — which reads like a problem
	// with the account rather than a missing parameter.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withGrades(gradeItem(2, "Essay 1", "mod", nil))

	if _, _, code := f.run("grade", "list", "--course", "2"); code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	var sent string
	for _, request := range f.server.Requests() {
		if request.Function == moodle.FunctionGradeItems {
			sent = request.Params.Get("userid")
		}
	}
	if sent != "4" {
		t.Errorf("userid = %q, want the signed-in user's own id", sent)
	}
}

func TestAnEmptyGradebookSaysSoAndIsNotEmptyOnlyByAccident(t *testing.T) {
	// 選了課但還沒被評分：成績單存在、裡面沒有東西。這和「這門課根本沒有
	// 你的成績單」是兩件事，前者是一句話，後者是找不到。
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withGrades() // 一筆項目都沒有

	stdout, _, code := f.run("grade", "list", "--course", "2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "grade.list", stdout)

	human, _, _ := f.run("grade", "list", "--course", "2")
	if !strings.Contains(human, "Nothing in this gradebook") {
		t.Errorf("an unmarked gradebook should say so:\n%s", human)
	}
}

func TestACourseWithNoGradebookForThisAccountIsNotFound(t *testing.T) {
	// 帳號不在這門課上，Moodle 回空陣列而不是拒絕——因為它是被問「這個人
	// 在這門課的成績」，答案就是沒有。那是找不到，不是空成績單：把兩者
	// 混在一起，使用者會以為自己修了課卻什麼都沒被評分。
	f := newFixture(t)
	f.addSiteAndLogin()
	f.server.HandleValue(moodle.FunctionGradeItems, map[string]any{
		"usergrades": []any{}, "warnings": []any{},
	})

	stdout, stderr, code := f.run("grade", "list", "--course", "9", "--json")
	if code != v1.ExitNotFound {
		t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s",
			code, v1.ExitNotFound, stdout, stderr)
	}
	if !strings.Contains(stdout, "no gradebook") {
		t.Errorf("the document should say there is no gradebook:\n%s", stdout)
	}
}
