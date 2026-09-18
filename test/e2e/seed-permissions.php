<?php
/**
 * 權限與可見性的量測 fixture：獨立分組、可用性限制。由 seed-permissions.sh 呼叫。
 *
 * 為什麼跟 seed-masters.php 分開：
 *   - seed-masters 建的是 e2e 全功能跑測的題材，那一輪的期望輸出必須穩定。
 *     這裡的每一項都會讓某個帳號少看到東西——清單短一截、狀態變成拒絕——
 *     混進去就等於讓煙霧測試的期望值取決於限制條件。
 *   - 這些 fixture 是「套上去量、量完還原」用的，所以附帶 revert。
 *
 * 用法（容器內）：php /seed-permissions.php apply
 *                 php /seed-permissions.php revert
 */

define('CLI_SCRIPT', true);
require('/var/www/html/config.php');
require_once($CFG->dirroot . '/mod/forum/lib.php');

// 這兩個名字是還原時的唯一憑據：只刪自己建的。
const GROUP_A = 'moodle-cli-e2e-A';
const GROUP_B = 'moodle-cli-e2e-B';
const DISCUSSION_B = 'B 組才看得到的討論';

/**
 * 改完 course_modules 一定要清 modinfo 快取。
 *
 * 少了這一步，限制不會生效，而呼叫照樣成功——量到的是一組看起來「沒差別」的
 * 假結果，而且完全沒有錯誤可以提醒你。這是這份檔案存在的主要理由之一。
 */
function apply_and_purge(callable $work): void {
    $work();
    purge_all_caches();
}

/**
 * 寫入一條可用性限制。
 *
 * showc 必須與 c 等長。少了它，\core_availability\tree 會在**教師**那側丟
 * coding_exception（'->c, ->showc mismatch'），學生那側卻照樣被擋下——兩個帳號
 * 看到的東西不一樣，很容易被讀成「限制對教師無效」。
 */
function set_availability(int $cmid, ?array $conditions): void {
    global $DB;
    if ($conditions === null) {
        $DB->set_field('course_modules', 'availability', null, ['id' => $cmid]);
        return;
    }
    $DB->set_field('course_modules', 'availability', json_encode([
        'op' => '&',
        'c' => $conditions,
        'showc' => array_fill(0, count($conditions), true),
    ]), ['id' => $cmid]);
}

function cmid_of(string $modname, int $instanceid): int {
    $cm = get_coursemodule_from_instance($modname, $instanceid, 0, false, MUST_EXIST);
    return (int) $cm->id;
}

function group_named(int $courseid, string $idnumber): ?stdClass {
    global $DB;
    return $DB->get_record('groups', ['courseid' => $courseid, 'idnumber' => $idnumber]) ?: null;
}

/**
 * 活動層級的 capability 覆寫。
 *
 * 跟可用性限制長得很像——站台一樣是回一次成功的呼叫、把活動從清單裡拿掉、
 * 附一筆 item=module 的 warning——但擋人的機制完全不同：這裡是 require_capability
 * 直接擋，前者是 \core_availability 的條件判定。兩條都要有 fixture，才不會用其中
 * 一條的行為去推論另一條。
 */
function set_activity_override(int $cmid, string $roleshort, string $capability, ?int $permission): void {
    global $DB;
    $role = $DB->get_record('role', ['shortname' => $roleshort], '*', MUST_EXIST);
    $context = context_module::instance($cmid);
    role_change_permission($role->id, $context, $capability, $permission ?? CAP_INHERIT);
}

$mode = $argv[1] ?? 'apply';

$ta      = $DB->get_record('course', ['shortname' => 'CS1001'], '*', MUST_EXIST);
$masters = $DB->get_record('course', ['shortname' => 'CS5006'], '*', MUST_EXIST);
$forum   = $DB->get_record('forum', ['course' => $ta->id, 'type' => 'general'], '*', MUST_EXIST);
$assign  = $DB->get_record('assign', ['course' => $ta->id, 'name' => 'Exercise 1 Loops'], '*', MUST_EXIST);
$chapter = $DB->get_record('assign', ['course' => $masters->id, 'name' => 'Chapter 2 Draft'], '*', MUST_EXIST);

$forumcm   = cmid_of('forum', $forum->id);
$assigncm  = cmid_of('assign', $assign->id);
$chaptercm = cmid_of('assign', $chapter->id);

$users = [];
foreach (['grad1', 'ug1', 'ug2'] as $name) {
    $users[$name] = $DB->get_record('user', ['username' => $name], '*', MUST_EXIST);
}

if ($mode === 'revert') {
    apply_and_purge(function () use ($DB, $ta, $forumcm, $assigncm, $chaptercm) {
        foreach ([GROUP_A, GROUP_B] as $idnumber) {
            if ($group = group_named($ta->id, $idnumber)) {
                $DB->delete_records('groups_members', ['groupid' => $group->id]);
                $DB->delete_records('groups', ['id' => $group->id]);
                echo "[perm] 刪除分組 $idnumber\n";
            }
        }
        // 分組刪掉之後，留在討論上的 groupid 會指向不存在的組。-1 是「所有人」。
        $DB->set_field('forum_discussions', 'groupid', -1, ['course' => $ta->id]);
        foreach ([$forumcm, $assigncm] as $cmid) {
            $DB->set_field('course_modules', 'groupmode', 0, ['id' => $cmid]);
        }
        $DB->set_field('course', 'groupmode', 0, ['id' => $ta->id]);
        set_availability($chaptercm, null);
        set_activity_override($assigncm, 'student', 'mod/assign:view', null);

        foreach ($DB->get_records('forum_discussions',
                ['course' => $ta->id, 'name' => DISCUSSION_B]) as $discussion) {
            $DB->delete_records('forum_posts', ['discussion' => $discussion->id]);
            $DB->delete_records('forum_discussions', ['id' => $discussion->id]);
            echo "[perm] 刪除討論 #{$discussion->id}\n";
        }
    });
    echo "[perm] 已還原\n";
    return;
}

apply_and_purge(function () use ($DB, $ta, $users, $forum, $forumcm, $assigncm, $chaptercm) {
    // 分組模式設在**活動**上，不是課程上：課程那一層只是新活動的預設值，
    // 除非 groupmodeforce，否則單獨設課程不會讓任何活動真的分組。
    $ids = [];
    foreach ([GROUP_A => 'UG-A', GROUP_B => 'UG-B'] as $idnumber => $name) {
        $group = group_named($ta->id, $idnumber);
        if (!$group) {
            $group = (object) [
                'courseid' => $ta->id, 'name' => $name, 'idnumber' => $idnumber,
                'description' => '', 'descriptionformat' => FORMAT_HTML,
                'timecreated' => time(), 'timemodified' => time(),
            ];
            $group->id = $DB->insert_record('groups', $group);
        }
        $ids[$idnumber] = (int) $group->id;
    }

    // 助教 grad1 跟 ug1 同在 A 組，ug2 單獨在 B 組：這樣才量得到
    // 「有 capability 但不在那個組」跟 accessallgroups 的差別。
    $members = [
        [GROUP_A, 'grad1'], [GROUP_A, 'ug1'], [GROUP_B, 'ug2'],
    ];
    foreach ($members as [$idnumber, $username]) {
        $groupid = $ids[$idnumber];
        $userid = $users[$username]->id;
        if (!$DB->record_exists('groups_members', ['groupid' => $groupid, 'userid' => $userid])) {
            $DB->insert_record('groups_members', (object) [
                'groupid' => $groupid, 'userid' => $userid,
                'timeadded' => time(), 'component' => '', 'itemid' => 0,
            ]);
        }
    }

    // SEPARATEGROUPS = 1。
    foreach ([$forumcm, $assigncm] as $cmid) {
        $DB->set_field('course_modules', 'groupmode', SEPARATEGROUPS, ['id' => $cmid]);
    }
    $DB->set_field('course', 'groupmode', SEPARATEGROUPS, ['id' => $ta->id]);

    // 既有的那串歸 A 組，另外補一串只有 B 組看得到的。
    $DB->set_field('forum_discussions', 'groupid', $ids[GROUP_A],
        ['course' => $ta->id, 'forum' => $forum->id]);
    if (!$DB->record_exists('forum_discussions',
            ['forum' => $forum->id, 'name' => DISCUSSION_B])) {
        // 走 forum_add_discussion() 而不是直接寫表：第一篇貼文是它建的，
        // 而「討論一定有第一篇貼文」正是我們用來分辨「被過濾」與「真的是空的」
        // 的依據。直接插一筆 forum_discussions 會造出一個違反那條性質的資料。
        forum_add_discussion((object) [
            'course' => $ta->id, 'forum' => $forum->id, 'name' => DISCUSSION_B,
            'message' => '<p>只有 B 組看得到。</p>', 'messageformat' => FORMAT_HTML,
            'messagetrust' => 0, 'attachment' => null,
            'groupid' => $ids[GROUP_B], 'mailnow' => 0,
        ], null, null, $users['ug2']->id);
    }
    echo "[perm] CS1001：獨立分組，A={grad1,ug1} B={ug2}，各一串討論\n";

    // 可用性限制：CS5006 的 Chapter 2 Draft 七天後才開放。
    // 站台的回答是一次成功的呼叫，把該活動從清單裡拿掉並附一筆
    // item=module 的 warning——不是錯誤。
    set_availability($chaptercm, [
        ['type' => 'date', 'd' => '>=', 't' => time() + 7 * DAYSECS],
    ]);
    echo "[perm] CS5006：Chapter 2 Draft 以日期限制存取\n";

    // 活動層級的 capability 覆寫：CS1001 的 Exercise 1 Loops 對 student 角色
    // PROHIBIT mod/assign:view。教師不受影響，所以同一門課兩種帳號會看到
    // 不一樣的清單——那正是「別拿一個帳號的結果去推論另一個」要測的東西。
    set_activity_override($assigncm, 'student', 'mod/assign:view', CAP_PROHIBIT);
    echo "[perm] CS1001：Exercise 1 Loops 對 student 角色 PROHIBIT mod/assign:view\n";
});
echo "[perm] 完成\n";
