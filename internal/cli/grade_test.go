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

func TestStaffAreToldTheGradebookIsNotAboutThem(t *testing.T) {
	// gradereport_user_get_grade_items 靠 moodle/grade:viewall 放行老師，然後
	// 對「他自己的成績」回一張課程的成績項目表、分數全空——跟一個還沒被評分的
	// 學生看到的一模一樣。回覆本身沒有任何欄位分得出來，所以要另外問一次。
	f := newFixture(t)
	f.withGrades(gradeItem(1, "Essay 1", "mod", nil))
	f.server.HandleValue(moodle.FunctionGradableUsers, map[string]any{
		// 這門課的評分對象是別人，呼叫者（user 4）不在裡面。
		"users":    []any{map[string]any{"id": 9}, map[string]any{"id": 10}},
		"warnings": []any{},
	})
	f.addSiteAndLogin()

	stdout, stderr, code := f.run("grade", "list", "--course", "2")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "not a graded participant") {
		t.Errorf("an empty gradebook read as work that is merely unmarked:\n%s", stdout)
	}

	jsonOut, _, code := f.run("grade", "list", "--course", "2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "grade.list", jsonOut)
	if !gradeReport(t, jsonOut).NotGradable {
		t.Error("the contract did not carry what the human output said")
	}
}

func TestAStudentWithNothingMarkedIsNotCalledStaff(t *testing.T) {
	// 同樣是一張全空的成績表，但呼叫者確實是評分對象。說他不是，就是把一個
	// 作業還沒被改的學生講成教職員。
	f := newFixture(t)
	f.withGrades(gradeItem(1, "Essay 1", "mod", nil))
	f.server.HandleValue(moodle.FunctionGradableUsers, map[string]any{
		"users":    []any{map[string]any{"id": 4}, map[string]any{"id": 9}},
		"warnings": []any{},
	})
	f.addSiteAndLogin()

	stdout, _, code := f.run("grade", "list", "--course", "2")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(stdout, "not a graded participant") {
		t.Errorf("a student whose work is unmarked was reported as staff:\n%s", stdout)
	}
}

func TestARefusedGradableProbeClaimsNothing(t *testing.T) {
	// 站台可以把 moodle/site:viewuseridentity 開給任何角色，所以「這支被拒絕」
	// 完全不能反推呼叫者是誰。拿不到答案就不要下結論。
	f := newFixture(t)
	f.withGrades(gradeItem(1, "Essay 1", "mod", nil))
	f.server.FailException(moodle.FunctionGradableUsers,
		"required_capability_exception", "nopermission", "error/nopermission")
	f.addSiteAndLogin()

	stdout, _, code := f.run("grade", "list", "--course", "2")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: a probe that said nothing must not change the answer", code)
	}
	if strings.Contains(stdout, "not a graded participant") {
		t.Errorf("a refusal was read as an answer:\n%s", stdout)
	}
}

func TestAGradebookWithNoVisibleItemsDoesNotPrintABareHeader(t *testing.T) {
	// 活動層級的權限覆寫會把成績項目從回覆裡拿掉、把課程總分留著，而且不附任何
	// warning——實測過。舊的輸出是一列表頭、底下什麼都沒有，看起來像指令壞了。
	f := newFixture(t)
	f.server.HandleValue(moodle.FunctionGradeItems, map[string]any{
		"usergrades": []any{map[string]any{
			"courseid": 2, "userid": 4, "gradeitems": []any{
				gradeItem(9, "", "course", map[string]any{"itemname": nil, "itemmodule": nil}),
			},
		}},
	})
	f.server.HandleValue(moodle.FunctionGradableUsers, map[string]any{
		"users": []any{map[string]any{"id": 4}}, "warnings": []any{},
	})
	f.addSiteAndLogin()

	stdout, stderr, code := f.run("grade", "list", "--course", "2")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if strings.Contains(stdout, "ITEM") {
		t.Errorf("an empty table was printed with only its header:\n%s", stdout)
	}
	if !strings.Contains(stdout, "No grade items are visible") {
		t.Errorf("the listing did not say what it could honestly claim:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Course total") {
		t.Errorf("the course total was dropped along with the table:\n%s", stdout)
	}
}

func TestAFailingGradeIsNotAnAbsentOne(t *testing.T) {
	// 被當掉是一個**有分數**的狀態。把 42 分讀成「還沒評分」，或是反過來把
	// 「還沒評分」讀成 0 分，都會讓學生對自己的學業狀況有錯誤的認識——
	// 而這兩件事在一張全是「-」的表裡看起來一模一樣。
	f := newFixture(t)
	f.withGrades(
		gradeItem(1, "Problem Set 6", "mod", map[string]any{
			"graderaw": 42.0, "gradeformatted": "42.00",
			"gradedategraded": 1629072000,
		}),
		gradeItem(2, "Problem Set 7", "mod", nil), // 真的還沒評分
	)
	f.server.HandleValue(moodle.FunctionGradableUsers, map[string]any{
		"users": []any{map[string]any{"id": 4}}, "warnings": []any{},
	})
	f.addSiteAndLogin()

	stdout, stderr, code := f.run("grade", "list", "--course", "2")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "42") {
		t.Errorf("a failing grade was not shown:\n%s", stdout)
	}

	report := gradeReport(t, mustGradeJSON(t, f))
	var failed, ungraded *float64
	for _, item := range report.Items {
		switch item.Name {
		case "Problem Set 6":
			failed = item.Grade
		case "Problem Set 7":
			ungraded = item.Grade
		}
	}
	if failed == nil || *failed != 42 {
		t.Errorf("the failing grade did not survive the contract: %v", failed)
	}
	if ungraded != nil {
		t.Errorf("unmarked work was given the number %v instead of null", *ungraded)
	}
}

func mustGradeJSON(t *testing.T, f *fixture) string {
	t.Helper()
	stdout, stderr, code := f.run("grade", "list", "--course", "2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	validate(t, "grade.list", stdout)
	return stdout
}

func TestAGradeFlagTheSiteWithheldIsNotReportedAsFalse(t *testing.T) {
	// Moodle 把 gradeislocked 送給能管理成績的帳號，對其他人送 null——實測學生
	// 拿到的正是 null。DTO 這一層本來就用指標接住了，但到 domain 被壓回 bool，
	// 於是契約印出 "locked": false：一個站台明確拒絕回答的事實。
	f := newFixture(t)
	f.server.HandleValue(moodle.FunctionGradeItems, map[string]any{
		"usergrades": []any{map[string]any{
			"courseid": 2, "userid": 4,
			"gradeitems": []any{map[string]any{
				"id": 1, "itemtype": "mod", "itemname": "Essay 1",
				"gradeishidden": false, "gradeislocked": nil,
			}},
		}},
	})
	f.server.HandleValue(moodle.FunctionGradableUsers, map[string]any{
		"users": []any{map[string]any{"id": 4}}, "warnings": []any{},
	})
	f.addSiteAndLogin()

	stdout, _, code := f.run("grade", "list", "--course", "2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "grade.list", stdout)
	if !strings.Contains(stdout, `"locked":null`) {
		t.Errorf("a flag the site withheld was published as false:\n%s", stdout)
	}
}

func TestAGradeFlagTheSiteDidGiveIsKept(t *testing.T) {
	f := newFixture(t)
	f.server.HandleValue(moodle.FunctionGradeItems, map[string]any{
		"usergrades": []any{map[string]any{
			"courseid": 2, "userid": 4,
			"gradeitems": []any{map[string]any{
				"id": 1, "itemtype": "mod", "itemname": "Essay 1",
				"gradeishidden": false, "gradeislocked": true,
			}},
		}},
	})
	f.server.HandleValue(moodle.FunctionGradableUsers, map[string]any{
		"users": []any{map[string]any{"id": 4}}, "warnings": []any{},
	})
	f.addSiteAndLogin()

	stdout, _, code := f.run("grade", "list", "--course", "2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, `"locked":true`) {
		t.Errorf("a flag the site did give was lost:\n%s", stdout)
	}
}
