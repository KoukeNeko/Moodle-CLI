package cli_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/cli"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
)

// 環境變數帶來的憑證。它繞過設定檔與金鑰圈，所以沒有任何一筆存下來的記錄
// 可以描述它——而「沒有記錄」正是這裡會出錯的地方。

// TestATokenFromTheEnvironmentIsNotTheStoredAccount —— 環境變數的憑證不屬於
// 設定檔裡任何一個帳號，即使那裡剛好有一個。
//
// 這是最容易寫錯的一種：設定檔裡存著 grad1，命令卻帶著別人的 token 跑。
// 沿用存下來的那筆記錄，文件就會報出「帳號 grad1、使用者 grad1、user_id 7」，
// 而實際上這些資料是 user_id 12 的。對讀文件的腳本與 AI agent 來說，那比
// 沒有名字更糟——它會把一個人的資料記到另一個人頭上。
func TestATokenFromTheEnvironmentIsNotTheStoredAccount(t *testing.T) {
	f := newFixture(t)
	// 站台依 token 回答不同的身分：帶 grad1 的 token 就是 grad1，帶別的
	// 就是別人。真實的 Moodle 就是這樣。
	f.server.Handle(moodle.FunctionSiteInfo, func(params url.Values) (any, error) {
		if params.Get("wstoken") == "good-token" {
			return siteInfo("Ming-Hua Chen", "grad1", 7), nil
		}
		return siteInfo("No Course", "nocourse", 12), nil
	})
	f.server.AddToken("good-token")
	f.server.AddToken("other-token")
	f.addSiteAndLogin() // 存下 grad1

	// 先確認沒有環境變數時，報的是存下來的帳號。
	stored, _, code := f.run("auth", "status", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stored, `"account":"grad1"`) {
		t.Fatalf("the stored account should be reported as itself:\n%s", stored)
	}

	// 換成另一個人的 token：身分兩個欄位都要跟著換。
	t.Setenv(cli.EnvWSToken, "other-token")
	stdout, stderr, code := f.run("auth", "status", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	validate(t, "auth.status", stdout)
	for _, want := range []string{
		`"account":"env"`,       // 不是 grad1
		`"username":"nocourse"`, // 站台說這個 token 是誰的
		`"user_id":"12"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("auth status should report %s:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, `"user_id":"7"`) {
		t.Errorf("the stored account's id leaked into another account's status:\n%s", stdout)
	}
}

// siteInfo is a minimal get_site_info answer for one person.
func siteInfo(fullName, username string, userID int) map[string]any {
	return map[string]any{
		"sitename": "Test Moodle", "username": username,
		"firstname": fullName, "lastname": "", "userid": userID,
		"release": "5.2.3", "downloadfiles": 1, "uploadfiles": 1,
		"functions": []any{},
	}
}

// TestATokenFromTheEnvironmentNeedsNoStoredAccount —— CI 與 headless 機器上
// 沒有金鑰圈可以存憑證，環境變數是唯一的路。
//
// 這條路徑先前完全沒有測試：它繞過設定檔與金鑰圈，而「繞過」正是它會出錯的
// 地方——沒有帳號可存時仍然要能跑，並且在 meta 裡誠實地說出憑證是從哪裡來的。
func TestATokenFromTheEnvironmentNeedsNoStoredAccount(t *testing.T) {
	f := newFixture(t)
	f.withCourses(map[string]any{
		"id": 2, "shortname": "CS204", "fullname": "Operating Systems",
		"startdate": 1757894400, "enddate": 0, "visible": 1,
	})
	if _, _, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatal("site add failed")
	}
	// 刻意不登入：這裡要驗的正是「沒有存任何帳號」。
	t.Setenv(cli.EnvWSToken, "good-token")

	stdout, stderr, code := f.run("course", "list", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, want 0 — a token in the environment is enough\n%s",
			code, stderr)
	}
	validate(t, "course.list", stdout)
	if !strings.Contains(stdout, `"account":"env"`) {
		t.Errorf("the document should name the credential's origin:\n%s", stdout)
	}
	// 憑證只活在這個行程裡，沒有東西被寫進設定檔。
	if strings.Contains(stdout, "good-token") {
		t.Errorf("the token leaked into the document:\n%s", stdout)
	}
}

// TestABlankCredentialIsNotACredential —— 設了環境變數但沒有值，等於沒設。
//
// 一個空字串憑證會被送去當 token，回來的錯誤指向站台（「invalid token」），
// 而真正該做的事是登入。分辨這兩者，使用者才不會去查一個沒有壞的站台。
func TestABlankCredentialIsNotACredential(t *testing.T) {
	f := newFixture(t)
	f.server.AddToken("good-token")
	if _, _, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatal("site add failed")
	}
	t.Setenv(cli.EnvWSToken, "   ")
	t.Setenv(cli.EnvSession, "")

	// --json 的錯誤文件走 stdout，那是契約；stderr 保持安靜是對的，
	// 所以斷言要看 stdout。
	stdout, _, code := f.run("course", "list", "--json")
	if code != v1.ExitAuthentication {
		t.Fatalf("exit %d, want %d — blank is not a credential\n%s",
			code, v1.ExitAuthentication, stdout)
	}
	validate(t, "error", stdout)
	if !strings.Contains(stdout, "credential_missing") {
		t.Errorf("a blank credential should read as missing, not as rejected:\n%s", stdout)
	}
	if !strings.Contains(stdout, "auth login") {
		t.Errorf("the refusal should say how to sign in:\n%s", stdout)
	}
}

func TestATokenAndABrowserSessionAreNeverUsedTogether(t *testing.T) {
	// 兩者是不同的身分：token 屬於它被發給的人，cookie 屬於登入的人。
	// 組裝時 WS 後端吃 token、讀頁面的後端吃 cookie，而功能會自己從前者掉到後者。
	// 若一次帶著兩個，一次回退就會用另一個人的資料回答問題，而且兩條路都成功，
	// 所以什麼都不會說。openSessionFor 只有在沒有 token 時才去看 MOODLE_SESSION。
	f := newFixture(t)
	if _, _, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatal("site add failed")
	}
	t.Setenv(cli.EnvWSToken, "good-token")
	t.Setenv(cli.EnvSession, "MoodleSession=someone-else")

	stdout, stderr, code := f.run("auth", "status", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	validate(t, "auth.status", stdout)
	// 站台照 token 回答身分。cookie 從頭到尾沒有機會參與：openSessionFor
	// 只有在 token 是空的時候才會去讀它。
	if !strings.Contains(stdout, `"username":"student1"`) {
		t.Errorf("the identity did not come from the token alone:\n%s", stdout)
	}
	if strings.Contains(stdout, "someone-else") {
		t.Errorf("the browser session reached the answer:\n%s", stdout)
	}
}
