package cli_test

import (
	"strings"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
)

// 身分組。這個工具面對的不只是學生：Moodle 有八個標準角色，而一個帳號在不同
// 課程可以有不同角色——助教在某一門課是教職員、在另一門課是學生。其餘的測試
// 幾乎都以學生身分跑，所以「不是學生」的幾個形狀固定在這裡。

// TestAnAccountWithNoEnrolmentsIsEmptyAndNotAnError 是每個新帳號的起點：
// 已驗證、但一門課都沒有。
//
// 空集合是合法的答案，不是失敗。把「沒有東西」報成錯誤，會讓腳本對一個完全
// 正常的帳號不停重試；而回 null 而不是 [] 則是契約問題——讀的人得先分辨
// 「沒有」和「不知道」。
func TestAnAccountWithNoEnrolmentsIsEmptyAndNotAnError(t *testing.T) {
	f := newFixture(t)
	f.withCourses()
	f.server.HandleValue(moodle.FunctionAssignments, map[string]any{"courses": []any{}})
	f.server.HandleValue(moodle.FunctionCourseGrades, map[string]any{"grades": []any{}})
	f.server.HandleValue(moodle.FunctionActionEvents, map[string]any{"events": []any{}})
	f.server.HandleValue(moodle.FunctionForums, []any{})
	f.addSiteAndLogin()

	cases := []struct {
		kind string
		args []string
	}{
		{"course.list", []string{"course", "list", "--json"}},
		{"assignment.list", []string{"assignment", "list", "--json"}},
		{"grade.overview", []string{"grade", "overview", "--json"}},
		{"calendar.upcoming", []string{"calendar", "upcoming", "--json"}},
		{"forum.list", []string{"forum", "list", "--json"}},
	}
	for _, tc := range cases {
		stdout, stderr, code := f.run(tc.args...)
		if code != v1.ExitOK {
			t.Errorf("%s: exit %d, want 0 — having nothing is not a failure\n%s",
				tc.kind, code, stderr)
			continue
		}
		validate(t, tc.kind, stdout)
		if !strings.Contains(stdout, `"data":[]`) {
			t.Errorf("%s: an empty result must be the empty array, not null:\n%s",
				tc.kind, stdout)
		}
	}
}

// TestAnAccountWithNoEnrolmentsIsToldSoInPlainWords 是上一條的人類可讀版本：
// 讀的人不該看到一張空表格。
func TestAnAccountWithNoEnrolmentsIsToldSoInPlainWords(t *testing.T) {
	f := newFixture(t)
	f.withCourses()
	f.server.HandleValue(moodle.FunctionAssignments, map[string]any{"courses": []any{}})
	f.server.HandleValue(moodle.FunctionCourseGrades, map[string]any{"grades": []any{}})
	f.server.HandleValue(moodle.FunctionForums, []any{})
	f.addSiteAndLogin()

	cases := []struct {
		message string
		args    []string
	}{
		{"No courses came back", []string{"course", "list"}},
		{"No assignments", []string{"assignment", "list"}},
		{"No course totals came back", []string{"grade", "overview"}},
		{"No forums", []string{"forum", "list"}},
	}
	for _, tc := range cases {
		stdout, _, code := f.run(tc.args...)
		if code != v1.ExitOK {
			t.Errorf("%v: exit %d, want 0", tc.args, code)
			continue
		}
		if !strings.Contains(stdout, tc.message) {
			t.Errorf("%v should say %q, got:\n%s", tc.args, tc.message, stdout)
		}
	}
}

// TestAnAdminIsRefusedQRLoginByName —— Moodle 對站台管理員關掉 app 登入流程。
//
// 這是被誤解成「站台壞了」的典型：管理員拿自己的帳號試 QR 登入，得到的是
// 上游錯誤碼。分類對了才會指向正確的動作（換一個帳號），而不是叫使用者去問
// 伺服器管理員——他自己就是。
func TestAnAdminIsRefusedQRLoginByName(t *testing.T) {
	f := newFixture(t)
	f.server.FailException(moodle.FunctionQRTokens, "moodle_exception",
		"autologinnotallowedtoadmins", "Admin cannot use the app login flow")
	if _, _, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatal("site add failed")
	}

	_, stderr, code := f.run("auth", "login", "--method", "qr",
		"--qr", f.server.URL()+"?qrlogin=KEY123&userid=2")
	if code != v1.ExitPermissionDenied {
		t.Fatalf("exit %d, want %d (stderr: %s)", code, v1.ExitPermissionDenied, stderr)
	}
	if !strings.Contains(stderr, "administrator") {
		t.Errorf("the refusal should say it is about administrators:\n%s", stderr)
	}
	if strings.Contains(stderr, "run `moodle doctor`") {
		t.Errorf("a site administrator is not helped by being sent to check the site:\n%s", stderr)
	}
}

func TestAnEmptyEnrolmentListDoesNotClaimThereAreNoCourses(t *testing.T) {
	// 系統層級的管理者可以零選課而照樣讀得到課程——實測過 mgr1 讀得到 CS204 的論壇。
	// core_enrol_get_users_courses 回空陣列時說「沒有課程」，對他就是錯的：
	// 那句話宣稱的比這個呼叫回答的多。
	f := newFixture(t)
	f.withCourses()
	f.server.HandleValue(moodle.FunctionUserCourses, []any{})
	f.addSiteAndLogin()

	stdout, _, code := f.run("course", "list")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(stdout, "no courses") ||
		strings.Contains(stdout, "not enrolled on any course") {
		t.Errorf("an empty enrolment list was reported as an absence of courses:\n%s", stdout)
	}
	if !strings.Contains(stdout, "enrolments") {
		t.Errorf("the answer does not say which question it answered:\n%s", stdout)
	}
	// 不只是管理者。實測站台上，一批課程被封存的學生仍然持有有效的選課紀錄，
	// 而 core_enrol_get_users_courses 對他回空清單——「你沒有選修任何課程」
	// 對他是錯的。同一個空清單至少有四種成因。
	if strings.Contains(stdout, "not enrolled on any course") {
		t.Errorf("an empty list was read as an absence of enrolments:\n%s", stdout)
	}
	// 第四種：停權。學校不退選，學校停權，而 enrol_get_users_courses 的
	// onlyactive 在 Moodle 裡是寫死的 true——實測停權一筆選課，清單就空了。
	// 列了三個原因卻漏掉這一個，讀起來就像那三個是全部。
	if !strings.Contains(stdout, "suspended") {
		t.Errorf("the reasons given leave out the one an institution actually uses:\n%s", stdout)
	}
}
