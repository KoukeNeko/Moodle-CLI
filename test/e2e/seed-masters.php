<?php
/**
 * 四學期碩士班的情境資料。由 seed-masters.sh 呼叫。
 *
 * 為什麼要有這一份，而不是沿用 seed.php：
 *   - seed.php 建的是「一門課、三個作業」的最小集合，用來驗單一行為。
 *     真實使用是跨學期、跨課程、跨角色的，而那些組合才會暴露問題——
 *     例如同一個帳號在 A 課是學生、在 B 課是助教時，作業列表該包含什麼。
 *   - 這裡刻意讓 grad1 在後兩學期兼任大學部課程的助教（teacher 角色）。
 *     助教看得到別人的繳交、但不該把它當成自己的。
 *
 * 用法（容器內）：php /seed-masters.php
 */

define('CLI_SCRIPT', true);
require('/var/www/html/config.php');
require_once($CFG->libdir . '/gradelib.php');
require_once($CFG->dirroot . '/user/lib.php');
require_once($CFG->dirroot . '/enrol/manual/lib.php');
require_once($CFG->dirroot . '/mod/assign/lib.php');
require_once($CFG->dirroot . '/mod/assign/locallib.php');
require_once($CFG->dirroot . '/mod/forum/lib.php');
require_once($CFG->dirroot . '/course/lib.php');

$now = time();
$DAY = DAYSECS;

/** 建立使用者，已存在就沿用。 */
function ensure_user(string $username, string $first, string $last): stdClass {
    global $DB, $CFG;
    if ($existing = $DB->get_record('user', ['username' => $username])) {
        return $existing;
    }
    $user = (object) [
        'username' => $username, 'auth' => 'manual', 'confirmed' => 1,
        'mnethostid' => $CFG->mnet_localhost_id, 'email' => "$username@example.edu",
        'firstname' => $first, 'lastname' => $last, 'password' => 'Student123!',
    ];
    $user->id = user_create_user($user, true, false);
    return $DB->get_record('user', ['id' => $user->id], '*', MUST_EXIST);
}

/** 建立課程，已存在就沿用。 */
function ensure_course(string $shortname, string $fullname, int $startdate): stdClass {
    global $DB;
    if ($existing = $DB->get_record('course', ['shortname' => $shortname])) {
        return $existing;
    }
    return create_course((object) [
        'shortname' => $shortname, 'fullname' => $fullname,
        'category' => 1, 'format' => 'topics', 'numsections' => 3,
        'startdate' => $startdate, 'enddate' => $startdate + 120 * DAYSECS,
        'visible' => 1,
    ]);
}

/** 選課並指派角色。起始日往前挪：Moodle 只認已經開始的選課。 */
function ensure_enrolment(stdClass $course, stdClass $user, string $roleshort): void {
    global $DB;
    $role = $DB->get_record('role', ['shortname' => $roleshort], '*', MUST_EXIST);
    $context = context_course::instance($course->id);
    if ($DB->record_exists('role_assignments',
            ['contextid' => $context->id, 'userid' => $user->id, 'roleid' => $role->id])) {
        return;
    }
    $manual = $DB->get_record('enrol',
        ['courseid' => $course->id, 'enrol' => 'manual'], '*', MUST_EXIST);
    enrol_get_plugin('manual')->enrol_user($manual, $user->id, $role->id, time() - DAYSECS);
}

/**
 * 建立作業並套用設定。
 *
 * 走 assign_add_instance() 而不是直接寫資料表：成績項目是它建的，少了那一步，
 * Moodle 自己讀提交狀態時會丟「Cannot load the grade item」，成績簿裡的項目
 * 也會沒有名字。順序照 course/modedit.php：先有 course module，才有 instance。
 */
function ensure_assign(stdClass $course, string $name, array $opts): stdClass {
    global $DB, $CFG;
    if ($existing = $DB->get_record('assign', ['name' => $name, 'course' => $course->id])) {
        return $existing;
    }
    $module = $DB->get_record('modules', ['name' => 'assign'], '*', MUST_EXIST);

    $cm = (object) [
        'course' => $course->id, 'module' => $module->id, 'instance' => 0,
        'section' => 0, 'visible' => 1, 'visibleoncoursepage' => 1, 'added' => time(),
    ];
    $cm->id = add_course_module($cm);

    $data = (object) array_merge([
        'course' => $course->id, 'coursemodule' => $cm->id, 'modulename' => 'assign',
        'name' => $name,
        'intro' => '<p>' . ($opts['intro'] ?? $name) . '</p>', 'introformat' => FORMAT_HTML,
        'alwaysshowdescription' => 1, 'submissiondrafts' => 0,
        'requiresubmissionstatement' => 0, 'sendnotifications' => 0,
        'sendlatenotifications' => 0, 'sendstudentnotifications' => 0,
        'duedate' => 0, 'allowsubmissionsfromdate' => 0, 'cutoffdate' => 0,
        'gradingduedate' => 0, 'grade' => 100, 'completionsubmit' => 0,
        'teamsubmission' => 0, 'requireallteammemberssubmit' => 0,
        'teamsubmissiongroupingid' => 0, 'blindmarking' => 0,
        'attemptreopenmethod' => 'none', 'maxattempts' => -1,
        'markingworkflow' => 0, 'markingallocation' => 0, 'nosubmissions' => 0,
        'timelimit' => 0, 'preventsubmissionnotingroup' => 0,
        // 繳交外掛：沒有這些，submissionsenabled 會是 false，什麼都交不了。
        'assignsubmission_file_enabled' => 1,
        'assignsubmission_file_maxfiles' => 3,
        'assignsubmission_file_maxsizebytes' => 0,
        'assignsubmission_onlinetext_enabled' => 1,
        'assignsubmission_comments_enabled' => 0,
        'assignfeedback_comments_enabled' => 1,
    ], $opts);
    unset($data->intro);
    $data->intro = '<p>' . ($opts['intro'] ?? $name) . '</p>';

    $instanceid = assign_add_instance($data, null);
    $DB->set_field('course_modules', 'instance', $instanceid, ['id' => $cm->id]);
    course_add_cm_to_section($course->id, $cm->id, 0);
    rebuild_course_cache($course->id, true);

    return $DB->get_record('assign', ['id' => $instanceid], '*', MUST_EXIST);
}

/** 讓某人對某作業處於指定狀態。 */
function ensure_submission(stdClass $assign, stdClass $user, string $status, ?string $filename = null): void {
    global $DB;
    $course = get_course($assign->course);
    $cm = get_coursemodule_from_instance('assign', $assign->id, $course->id);
    $context = context_module::instance($cm->id);
    $obj = new assign($context, $cm, $course);
    \core\session\manager::set_user($user);
    $submission = $obj->get_user_submission($user->id, true);
    $submission->status = $status;
    $submission->timemodified = time();
    $DB->update_record('assign_submission', $submission);

    if ($filename) {
        $fs = get_file_storage();
        if (!$fs->file_exists($context->id, 'assignsubmission_file',
                'submission_files', $submission->id, '/', $filename)) {
            $fs->create_file_from_string((object) [
                'contextid' => $context->id, 'component' => 'assignsubmission_file',
                'filearea' => 'submission_files', 'itemid' => $submission->id,
                'filepath' => '/', 'filename' => $filename, 'userid' => $user->id,
            ], "%PDF-1.4 " . $filename . "\n");
        }
    }
}

/** 給分與評語。 */
function ensure_grade(stdClass $assign, stdClass $user, float $mark, string $feedback): void {
    grade_update('mod/assign', $assign->course, 'mod', 'assign', $assign->id, 0, [
        'userid' => $user->id, 'rawgrade' => $mark,
        'feedback' => '<p>' . $feedback . '</p>', 'feedbackformat' => FORMAT_HTML,
    ]);
}

/**
 * 清掉某人在某作業的提交。
 *
 * 情境裡標成「還沒交」的作業需要這一步：full-run.sh 會把其中一份交出去，
 * 重跑種子資料時要能回到同一個起點，否則第二次就跑不下去了。
 */
function clear_submission(stdClass $assign, stdClass $user): void {
    global $DB;
    $course = get_course($assign->course);
    $cm = get_coursemodule_from_instance('assign', $assign->id, $course->id);
    $context = context_module::instance($cm->id);
    $fs = get_file_storage();
    foreach ($DB->get_records('assign_submission',
            ['assignment' => $assign->id, 'userid' => $user->id]) as $submission) {
        $fs->delete_area_files($context->id, 'assignsubmission_file',
            'submission_files', $submission->id);
        $DB->delete_records('assignsubmission_onlinetext', ['submission' => $submission->id]);
        $DB->delete_records('assign_submission', ['id' => $submission->id]);
    }
}

/** 討論串加一則回覆。 */
function ensure_discussion(stdClass $course, string $forumname, string $subject,
                           string $body, stdClass $author, ?array $reply = null): void {
    global $DB, $CFG;
    $forum = $DB->get_record('forum', ['course' => $course->id, 'type' => 'general']);
    if (!$forum) {
        $module = $DB->get_record('modules', ['name' => 'forum'], '*', MUST_EXIST);
        $forum = (object) [
            'course' => $course->id, 'type' => 'general', 'name' => $forumname,
            'intro' => '<p>課程討論。</p>', 'introformat' => FORMAT_HTML,
            'timemodified' => time(),
        ];
        $forum->id = $DB->insert_record('forum', $forum);
        $cm = (object) [
            'course' => $course->id, 'module' => $module->id, 'instance' => $forum->id,
            'section' => 0, 'visible' => 1, 'visibleoncoursepage' => 1, 'added' => time(),
        ];
        $cm->id = add_course_module($cm);
        course_add_cm_to_section($course->id, $cm->id, 0);
    }
    if ($DB->record_exists('forum_discussions', ['forum' => $forum->id, 'name' => $subject])) {
        return;
    }
    \core\session\manager::set_user($author);
    $discussionid = forum_add_discussion((object) [
        'course' => $course->id, 'forum' => $forum->id, 'name' => $subject,
        'message' => '<p>' . $body . '</p>', 'messageformat' => FORMAT_HTML,
        'messagetrust' => 0, 'attachment' => null, 'groupid' => -1, 'mailnow' => 0,
    ], null, null, $author->id);

    if ($reply) {
        [$replier, $text] = $reply;
        $first = $DB->get_record('forum_posts',
            ['discussion' => $discussionid, 'parent' => 0], '*', MUST_EXIST);
        \core\session\manager::set_user($replier);
        forum_add_new_post((object) [
            'discussion' => $discussionid, 'parent' => $first->id,
            'course' => $course->id, 'forum' => $forum->id,
            'subject' => 'Re: ' . $subject, 'message' => '<p>' . $text . '</p>',
            'messageformat' => FORMAT_HTML, 'messagetrust' => 0, 'attachment' => '',
            'userid' => $replier->id, 'created' => time(), 'modified' => time(),
            'mailnow' => 0, 'itemid' => 0,
        ], null);
    }
}

// ─── 角色 ───────────────────────────────────────────────────────────────
$grad = ensure_user('grad1', 'Ming-Hua', 'Chen');       // 碩士生，也是助教
$prof = ensure_user('prof1', 'Wei', 'Lin');             // 授課教師
$peer = ensure_user('grad2', 'Yi-Ting', 'Wang');        // 同學
$ug1  = ensure_user('ug1', 'Alex', 'Undergrad');        // 大學部學生
$ug2  = ensure_user('ug2', 'Bella', 'Undergrad');
// 不是每個帳號都有課。新帳號的起點就是這樣，而「什麼都沒有」是最容易
// 被寫成錯誤或 null 的地方：Moodle 的 user 角色，零選課。
$none = ensure_user('nocourse', 'No', 'Course');
// 站台層級的高權限，卻沒有任何課程角色——助教要處理的身分之外的另一種
// 「看得到站台、看不到課」。
$mgr = ensure_user('mgr1', 'Site', 'Manager');
if (!get_config('masters', 'manager_assigned')) {
    $role = $DB->get_record('role', ['shortname' => 'manager'], '*', MUST_EXIST);
    role_assign($role->id, $mgr->id, context_system::instance()->id);
    set_config('manager_assigned', 1, 'masters');
}

// ─── 四個學期的課程 ──────────────────────────────────────────────────────
$semesters = [
    ['CS5001', 'Advanced Algorithms',   -540],  // 第一學期（最久以前）
    ['CS5002', 'Machine Learning',      -540],
    ['CS5003', 'Distributed Systems',   -360],  // 第二學期
    ['CS5004', 'Research Methods',      -360],
    ['CS5005', 'Thesis Seminar I',      -180],  // 第三學期
    ['CS5006', 'Thesis Seminar II',      -30],  // 第四學期（進行中）
];
$courses = [];
foreach ($semesters as [$short, $full, $offsetdays]) {
    $course = ensure_course($short, $full, $now + $offsetdays * $DAY);
    ensure_enrolment($course, $grad, 'student');
    ensure_enrolment($course, $peer, 'student');
    ensure_enrolment($course, $prof, 'editingteacher');
    $courses[$short] = $course;
}

// 大學部課程：grad1 在這裡是助教，不是學生。
$ta = ensure_course('CS1001', 'Introduction to Programming', $now - 180 * $DAY);
ensure_enrolment($ta, $prof, 'editingteacher');
ensure_enrolment($ta, $grad, 'teacher');       // 非編輯教師＝助教
ensure_enrolment($ta, $ug1, 'student');
ensure_enrolment($ta, $ug2, 'student');
$courses['CS1001'] = $ta;

echo "[masters] 課程 " . count($courses) . " 門，grad1 在 CS1001 是助教\n";

// ─── 作業：涵蓋每一種提交狀態 ────────────────────────────────────────────
$plan = [
    // [課程, 名稱, 設定, grad1 的狀態, 分數]
    ['CS5001', 'HW1 Divide and Conquer', ['duedate' => $now - 500 * $DAY], 'submitted', 92.0],
    ['CS5001', 'HW2 Dynamic Programming', ['duedate' => $now - 470 * $DAY], 'submitted', 78.5],
    ['CS5002', 'Lab1 Linear Regression', ['duedate' => $now - 500 * $DAY], 'submitted', 88.0],
    ['CS5002', 'Final Project Report',
        ['duedate' => $now - 450 * $DAY, 'requiresubmissionstatement' => 1], 'submitted', 95.0],
    ['CS5003', 'Paper Review: Raft', ['duedate' => $now - 320 * $DAY], 'submitted', 85.0],
    ['CS5003', 'Consensus Implementation',
        ['duedate' => $now - 300 * $DAY, 'submissiondrafts' => 1], 'submitted', 90.0],
    ['CS5004', 'Literature Survey', ['duedate' => $now - 150 * $DAY], 'submitted', 87.0],
    ['CS5005', 'Thesis Proposal', ['duedate' => $now - 100 * $DAY], 'submitted', 91.0],
    // 進行中的學期：各種未完成狀態
    ['CS5006', 'Chapter 1 Draft',
        ['duedate' => $now + 7 * $DAY, 'submissiondrafts' => 1], 'draft', null],
    ['CS5006', 'Chapter 2 Draft',
        ['duedate' => $now + 21 * $DAY, 'submissiondrafts' => 1], 'new', null],
    ['CS5006', 'Ethics Declaration',
        ['duedate' => $now - 2 * $DAY, 'requiresubmissionstatement' => 1], 'new', null],
    ['CS5006', 'Progress Log',
        ['duedate' => $now + 3 * $DAY, 'cutoffdate' => $now + 10 * $DAY], 'submitted', null],
];

$made = 0;
foreach ($plan as [$short, $name, $opts, $status, $mark]) {
    $assign = ensure_assign($courses[$short], $name, $opts);
    if ($status === 'new') {
        clear_submission($assign, $grad);
    } else {
        ensure_submission($assign, $grad, $status, 'report.pdf');
    }
    if ($mark !== null) {
        ensure_grade($assign, $grad, $mark, 'Clear argument; check the edge cases.');
    }
    $made++;
}

// 助教的課：大學部學生交了作業，grad1 看得到但那不是他的。
$uga = ensure_assign($ta, 'Exercise 1 Loops', ['duedate' => $now + 5 * $DAY]);
ensure_submission($uga, $ug1, 'submitted', 'ex1.pdf');
ensure_submission($uga, $ug2, 'draft', 'ex1-draft.pdf');
echo "[masters] 作業 " . ($made + 1) . " 份，含助教課一份\n";

// ─── 討論區 ─────────────────────────────────────────────────────────────
ensure_discussion($courses['CS5006'], 'Thesis Discussion',
    '口試時程什麼時候公布？', '想先安排實驗的時間。', $grad,
    [$prof, '下個月初會公布，請先把第一章交出來。']);
ensure_discussion($courses['CS5003'], 'Course Q&A',
    'Raft 的 leader election 有推薦讀物嗎？', '論文之外想看實作。', $peer,
    [$grad, '可以看 etcd 的 raft 套件，註解寫得很清楚。']);
ensure_discussion($ta, 'Student Questions',
    '作業一的迴圈題可以用 while 嗎？', '規格只寫了 for。', $ug1,
    [$grad, '可以，只要邏輯正確。（助教回覆）']);
echo "[masters] 討論串 3 串，含助教身分的回覆\n";
echo "[masters] 身分：學生、助教（非編輯教師）、教師、manager、零選課帳號\n";

purge_all_caches();
echo "[masters] 完成\n";
