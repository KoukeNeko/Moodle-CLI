package cli_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
)

// post builds one entry of Moodle's discussion posts reply.
func post(id int, parent any, author, subject, message string, created int64) map[string]any {
	return map[string]any{
		"id": id, "subject": subject, "message": message, "messageformat": 1,
		"discussionid": 1, "parentid": parent, "timecreated": created,
		"isdeleted":   false,
		"author":      map[string]any{"fullname": author, "isdeleted": false},
		"attachments": []any{},
		"urls":        map[string]any{"view": "https://moodle.example.edu/mod/forum/discuss.php?d=1#p" + strconv.Itoa(id)},
	}
}

func (f *fixture) withForum() {
	f.t.Helper()
	f.server.HandleValue(moodle.FunctionForums, []any{
		map[string]any{
			"id": 2, "cmid": 5, "course": 2, "type": "general",
			// Moodle HTML-escapes the name here and not in the sibling call.
			"name": "Q&amp;A 討論區", "intro": "<p>課程問答。</p>",
			"numdiscussions": 1,
		},
	})
	f.server.HandleValue(moodle.FunctionForumDiscussion, map[string]any{
		"discussions": []any{map[string]any{
			// "id" is the opening post's id; "discussion" is the thread's.
			"id": 7, "discussion": 1,
			"name": "第一次作業的常見問題", "userfullname": "Tammy Teacher",
			"usermodifiedfullname": "Sam Student",
			"created":              1789000000, "timemodified": 1789000100,
			"numreplies": 1, "numunread": 0,
			"pinned": false, "locked": false, "canreply": true,
		}},
		"warnings": []any{},
	})
	// Newest first, the way Moodle sends them.
	f.server.HandleValue(moodle.FunctionForumPosts, map[string]any{
		"posts": []any{
			post(8, 7, "Sam Student", "Re: 第一次作業的常見問題", "<p>可以交 PDF 以外的嗎？</p>", 1789000100),
			post(7, nil, "Tammy Teacher", "第一次作業的常見問題", "<p>先看<strong>評分標準</strong>。</p>", 1789000000),
		},
		"forumid": 2, "courseid": 2, "warnings": []any{},
	})
}

func TestForumNameIsConsistentBetweenTheTwoCalls(t *testing.T) {
	// get_forums_by_courses HTML-escapes the name and get_forum_discussions
	// does not, so a forum called "Q&A" would otherwise be two different
	// forums depending on which command you ran.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withForum()

	stdout, stderr, code := f.run("forum", "list", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "forum.list", stdout)

	var doc struct {
		Data []v1.Forum `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Data) != 1 {
		t.Fatalf("got %d forums", len(doc.Data))
	}
	if doc.Data[0].Name != "Q&A 討論區" {
		t.Errorf("name = %q, want it un-escaped", doc.Data[0].Name)
	}
}

func TestADiscussionIsIdentifiedByItsOwnIdNotTheOpeningPosts(t *testing.T) {
	// Moodle sends both in the same object. They are both small integers, so
	// taking the wrong one reads a different thread or none at all.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withForum()

	stdout, stderr, code := f.run("forum", "discussions", "2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "forum.discussions", stdout)

	var doc struct {
		Data []v1.Discussion `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Data[0].ID != "1" {
		t.Errorf("id = %q, want the discussion id (1), not the opening post's (7)", doc.Data[0].ID)
	}
}

func TestAThreadIsPrintedInReadingOrder(t *testing.T) {
	// Moodle returns posts newest first, so the answer arrives before the
	// question. Anyone reading top to bottom meets the thread backwards.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withForum()

	stdout, stderr, code := f.run("forum", "read", "1", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "forum.thread", stdout)

	var doc struct {
		Data []v1.Post `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Data) != 2 {
		t.Fatalf("got %d posts", len(doc.Data))
	}
	if doc.Data[0].ParentID != nil {
		t.Error("the thread does not open with the post that started it")
	}
	if doc.Data[0].ID != "7" || doc.Data[1].ID != "8" {
		t.Errorf("order = %s then %s, want 7 then 8", doc.Data[0].ID, doc.Data[1].ID)
	}

	human, _, _ := f.run("forum", "read", "1")
	question := strings.Index(human, "評分標準")
	answer := strings.Index(human, "PDF 以外")
	if question == -1 || answer == -1 || question > answer {
		t.Errorf("the reply appears before the post it answers:\n%s", human)
	}
}

func TestReadingAThreadMarksNothingAsRead(t *testing.T) {
	// Reading from a terminal should not tell the site you have read it, and
	// Moodle's mark-as-read calls can complete an activity.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withForum()

	if _, _, code := f.run("forum", "read", "1"); code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	for _, request := range f.server.Requests() {
		if strings.Contains(request.Function, "view_forum") ||
			strings.Contains(request.Function, "mark_posts_read") {
			t.Errorf("reading a thread called %s", request.Function)
		}
	}
}

func TestADeletedPostKeepsItsPlaceInTheThread(t *testing.T) {
	// Removing it entirely would lose the shape of the conversation around it.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withForum()
	deleted := post(8, 7, "", "", "", 1789000100)
	deleted["isdeleted"] = true
	deleted["author"] = nil
	f.server.HandleValue(moodle.FunctionForumPosts, map[string]any{
		"posts": []any{
			deleted,
			post(7, nil, "Tammy Teacher", "第一次作業的常見問題", "<p>先看評分標準。</p>", 1789000000),
		},
		"forumid": 2, "courseid": 2, "warnings": []any{},
	})

	stdout, _, code := f.run("forum", "read", "1", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	var doc struct {
		Data []v1.Post `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Data) != 2 {
		t.Fatalf("got %d posts, want the deleted one kept", len(doc.Data))
	}
	if !doc.Data[1].Deleted {
		t.Error("a deleted post was not marked as deleted")
	}

	human, _, _ := f.run("forum", "read", "1")
	if !strings.Contains(human, "[deleted post]") {
		t.Errorf("the deleted post vanished from the human output:\n%s", human)
	}
}

func TestAForumAddressIsResolvedThroughItsCourseModuleId(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withForum()

	stdout, stderr, code := f.run("forum", "discussions",
		"https://moodle.example.edu/mod/forum/view.php?id=5", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	var doc struct {
		Data []v1.Discussion `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Data) != 1 || doc.Data[0].ForumID != "2" {
		t.Errorf("the cmid 5 address did not resolve to forum 2: %+v", doc.Data)
	}
}

func TestADiscussionAddressCarriesTheIdDirectly(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withForum()

	if _, stderr, code := f.run("forum", "read",
		"https://moodle.example.edu/mod/forum/discuss.php?d=1"); code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
}

func TestReadRefusesAnAddressThatIsNotAThread(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withForum()

	_, stderr, code := f.run("forum", "read",
		"https://moodle.example.edu/mod/forum/view.php?id=5")
	if code != v1.ExitUsage {
		t.Fatalf("exit %d, want %d", code, v1.ExitUsage)
	}
	if !strings.Contains(stderr, "discuss.php") {
		t.Errorf("the error does not show what a thread address looks like:\n%s", stderr)
	}
}

func TestAForumWithNothingInItSaysSo(t *testing.T) {
	// 公告區就是這樣：存在、但沒有討論串。空白的表格看起來像壞掉，
	// 一句話才是答案。兩種空——沒有論壇、論壇裡沒有討論串——是不同的句子。
	f := newFixture(t)
	f.addSiteAndLogin()
	f.server.HandleValue(moodle.FunctionForums, []any{})
	f.server.HandleValue(moodle.FunctionForumDiscussion, map[string]any{
		"discussions": []any{}, "warnings": []any{},
	})
	f.server.HandleValue(moodle.FunctionForumPosts, map[string]any{
		"posts": []any{}, "forumid": 2, "courseid": 2, "warnings": []any{},
	})

	stdout, _, code := f.run("forum", "list")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "No forums") {
		t.Errorf("an empty forum list should say so:\n%s", stdout)
	}

	json, _, code := f.run("forum", "list", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "forum.list", json)
	if !strings.Contains(json, `"data":[]`) {
		t.Errorf("an empty forum list must be the empty array:\n%s", json)
	}

	stdout, _, code = f.run("forum", "discussions", "2")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "No discussions") {
		t.Errorf("a forum with no threads should say so:\n%s", stdout)
	}

}

func TestAThreadHeldBackByItsGroupIsNotAnEmptyThread(t *testing.T) {
	// 獨立分組的論壇對「這串不是你那組的」回的是一次**成功**的呼叫加一個空陣列，
	// 沒有任何 warning。照原樣印出去，就變成對一串讀都沒讀到的討論說「這裡沒有東西」。
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withForum()
	f.server.HandleValue(moodle.FunctionForumPosts, map[string]any{
		"posts": []any{}, "forumid": 2, "courseid": 2, "warnings": []any{},
	})

	stdout, stderr, code := f.run("forum", "read", "1")
	if code != v1.ExitPermissionDenied {
		t.Fatalf("exit %d, want %d\n%s", code, v1.ExitPermissionDenied, stderr)
	}
	if stdout != "" {
		t.Errorf("a refusal wrote to stdout:\n%s", stdout)
	}
	if strings.Contains(stderr, "Nothing in this thread") {
		t.Errorf("a thread the account cannot read was reported as an empty one:\n%s", stderr)
	}
	if !strings.Contains(stderr, "opening post") {
		t.Errorf("the message does not say why an empty reply settles it:\n%s", stderr)
	}
}

func TestACourseTheAccountCannotReadIsNotAForumlessCourse(t *testing.T) {
	// mod_forum_get_forums_by_courses 算出 warning 之後就丟掉——它的 returns
	// 定義裡根本沒有那個欄位。於是「這門課你讀不到」跟「這門課沒有論壇」回的
	// 是同一個空陣列。空陣列照印就是對一門根本沒讀到的課說「這裡沒有論壇」。
	f := newFixture(t)
	f.addSiteAndLogin()
	f.server.HandleValue(moodle.FunctionForums, []any{})
	f.server.HandleValue(moodle.FunctionNavigationOptions, map[string]any{
		"courses": []any{},
		"warnings": []any{map[string]any{
			"item": "course", "itemid": 2, "warningcode": "1",
			"message": "No access rights in course context",
		}},
	})

	stdout, stderr, code := f.run("forum", "list", "--course", "2")
	if code != v1.ExitPermissionDenied {
		t.Fatalf("exit %d, want %d\n%s", code, v1.ExitPermissionDenied, stderr)
	}
	if strings.Contains(stdout, "No forums") {
		t.Errorf("a course the account cannot read was reported as one with no forums:\n%s", stdout)
	}
}

func TestACourseTheAccountCanReadButHasNoVisibleForumsIsNotRefused(t *testing.T) {
	// 探針確認讀得到，就不能拒絕。而且即使如此也只能說「這個帳號看不到論壇」：
	// 那支函式還會依活動可見性與 mod/forum:viewdiscussion 過濾。
	f := newFixture(t)
	f.addSiteAndLogin()
	f.server.HandleValue(moodle.FunctionForums, []any{})
	f.server.HandleValue(moodle.FunctionNavigationOptions, map[string]any{
		"courses":  []any{map[string]any{"id": 2, "options": []any{}}},
		"warnings": []any{},
	})

	stdout, stderr, code := f.run("forum", "list", "--course", "2")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "visible to this account") {
		t.Errorf("an empty listing claimed more than the reply supports:\n%s", stdout)
	}
}

func TestAnUnavailableProbeLeavesTheListingStanding(t *testing.T) {
	// 站台沒有這支探針時，我們什麼都沒學到——不能因此把一次成功的列表變成拒絕。
	f := newFixture(t)
	f.addSiteAndLogin()
	f.server.HandleValue(moodle.FunctionForums, []any{})
	f.server.FailException(moodle.FunctionNavigationOptions,
		"webservice_access_exception", "accessexception", "Access control exception")

	stdout, _, code := f.run("forum", "list", "--course", "2")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, want 0: a probe that said nothing must not refuse", code)
	}
	if !strings.Contains(stdout, "No forums") {
		t.Errorf("the listing did not stand:\n%s", stdout)
	}
}
