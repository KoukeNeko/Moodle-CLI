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

/**
 * 把一筆選課設成停權。
 *
 * 停權跟退選不一樣：紀錄還在、成績也還在，只是 core_enrol_get_users_courses
 * 不再回這門課。所以「清單裡沒有」不能讀成「沒有修過」。
 */
function suspend_enrolment(stdClass $course, stdClass $user): void {
    global $DB;
    $row = $DB->get_record_sql(
        'SELECT ue.* FROM {user_enrolments} ue
           JOIN {enrol} e ON e.id = ue.enrolid
          WHERE e.courseid = ? AND ue.userid = ?',
        [$course->id, $user->id]);
    if ($row && (int)$row->status !== ENROL_USER_SUSPENDED) {
        $DB->set_field('user_enrolments', 'status', ENROL_USER_SUSPENDED, ['id' => $row->id]);
    }
}

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
    // grade_update 只寫該項目的分數；課程總分是另一個 grade_item，要重算才會有值。
    // 少了這一步，`grade overview` 對每一門課都回 "not graded yet"——一門明明
    // 拿了 74 分的課，看起來像從來沒有被批改過。
    grade_regrade_final_grades($assign->course);
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

/**
 * 站台層級的角色：看得到站台，卻一門課都沒有。
 *
 * manager 與 coursecreator 都不是課程裡的角色，所以他們的課程清單是空的——
 * 而且跟「零選課的新帳號」一樣空，儘管成因完全不同。
 */
function ensure_system_role(string $username, string $first, string $last, string $roleshort): stdClass {
    global $DB;
    $user = ensure_user($username, $first, $last);
    if (!get_config('masters', $roleshort . '_assigned')) {
        $role = $DB->get_record('role', ['shortname' => $roleshort], '*', MUST_EXIST);
        role_assign($role->id, $user->id, context_system::instance()->id);
        set_config($roleshort . '_assigned', 1, 'masters');
    }
    return $user;
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
$mgr = ensure_system_role('mgr1', 'Site', 'Manager', 'manager');
// 可以開課、但自己沒有選任何課。
$cc  = ensure_system_role('cc1', 'Course', 'Creator', 'coursecreator');

// ─── 大學四年 ────────────────────────────────────────────────────────────
//
// 為什麼要有這一段：一個真實帳號的課程清單不是六門，是六年。分頁、排序、
// 「早就結束的課」與「進行中的課」混在一起、成績分散在十幾門課裡——這些只有在
// 資料量接近真實時才會出問題。而且同一個人在這段期間裡的身分是會變的：
// 大一到大四是學生，畢業之後在同一個站台上變成碩士生，再變成大學部的助教。
//
// enddate 都設在 startdate 之後 120 天（ensure_course 的預設），所以前幾年的課
// 都是「已經結束」的狀態——那正是 core_enrol_get_users_courses 仍然會回、但
// 語意跟進行中的課不同的一批。
$undergrad = [
    // [代碼, 名稱, 開課日（距今天數）]
    ['UG1101', 'Calculus I',                 -2160],  // 大一上
    ['UG1102', 'Introduction to Computing',  -2160],
    ['UG1201', 'Calculus II',                -1980],  // 大一下
    ['UG1202', 'Data Structures',            -1980],
    ['UG2101', 'Discrete Mathematics',       -1800],  // 大二上
    ['UG2102', 'Computer Organization',      -1800],
    ['UG2201', 'Algorithms',                 -1620],  // 大二下
    ['UG2202', 'Operating Systems Concepts', -1620],
    ['UG3101', 'Database Systems',           -1440],  // 大三上
    ['UG3102', 'Computer Networks',          -1440],
    ['UG3201', 'Software Engineering',       -1260],  // 大三下
    ['UG3202', 'Probability and Statistics', -1260],
    ['UG4101', 'Compilers',                  -1080],  // 大四上
    ['UG4102', 'Machine Learning Basics',    -1080],
    ['UG4201', 'Senior Project',              -900],  // 大四下
];
$courses = [];
foreach ($undergrad as [$short, $full, $offsetdays]) {
    $course = ensure_course($short, $full, $now + $offsetdays * $DAY);
    // 同一個人的大學時期：那時候他是學生，不是助教。
    ensure_enrolment($course, $grad, 'student');
    ensure_enrolment($course, $prof, 'editingteacher');
    $courses[$short] = $course;
}
echo '[masters] 大學四年 ' . count($undergrad) . " 門課（全部已結束）\n";

// ─── 重修 ────────────────────────────────────────────────────────────────
//
// 大一下的微積分二被當掉，大二下重修一次。這在 Moodle 裡是**兩門不同的課**，
// 不是同一門課的第二次：每個學期各開一門，兩門的 fullname 一模一樣。
//
// 對客戶端來說這裡有三個陷阱：
//   1. 課程清單裡會出現兩筆名字相同的課。用名字去識別、或是用名字去做去重，
//      就會把重修那次吃掉，學生會看到自己少了一門課。
//   2. 兩門課各有一個成績。「你微積分二幾分？」沒有單一答案——舊的那次是 42 分
//      （不及格），新的那次是 71 分。把成績照名字合併會得到一個不存在的數字。
//   3. 舊的那次選課會被停權（ENROL_USER_SUSPENDED），因為學期結束了學籍就收回。
//      停權的選課**不會**出現在 core_enrol_get_users_courses 裡，所以那一次的
//      42 分是查不到的——而「查不到」不等於「沒有發生過」。
//
// 42 分這個數字是刻意的：不及格是一個**有分數**的狀態，跟「還沒評分」差得最遠，
// 而兩者在一張全是「-」的表裡看起來一樣。
$retake = ensure_course('UG1201R', 'Calculus II', $now - 1620 * $DAY);  // 大二下重修
ensure_enrolment($retake, $grad, 'student');
ensure_enrolment($retake, $prof, 'editingteacher');
$courses['UG1201R'] = $retake;

// 第一次那門課的選課停權：學期結束、學籍收回。停權的選課不會出現在
// core_enrol_get_users_courses，所以那 42 分從一般的課程清單查不到。
suspend_enrolment($courses['UG1201'], $grad);
echo "[masters] 重修：UG1201（大一下，42 分不及格、選課已停權）→ UG1201R（大二下，71 分通過），兩門同名\n";

// ─── 碩士四個學期的課程 ──────────────────────────────────────────────────
$semesters = [
    ['CS5001', 'Advanced Algorithms',   -540],  // 第一學期（最久以前）
    ['CS5002', 'Machine Learning',      -540],
    ['CS5003', 'Distributed Systems',   -360],  // 第二學期
    ['CS5004', 'Research Methods',      -360],
    ['CS5005', 'Thesis Seminar I',      -180],  // 第三學期
    ['CS5006', 'Thesis Seminar II',      -30],  // 第四學期（進行中）
];
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
    //
    // 大學四年：每學期一份已完成並評分過的作業。成績分散在十幾門課，
    // `grade overview` 要能把它們全部帶回來，而不是只帶回最近的幾門。
    ['UG1101', 'Problem Set 1',        ['duedate' => $now - 2100 * $DAY], 'submitted', 74.0],
    ['UG1102', 'Lab Report 1',         ['duedate' => $now - 2100 * $DAY], 'submitted', 81.0],
    // 被當掉的那一次：有交、有分數，分數不及格。42 分不是「沒有分數」。
    ['UG1201', 'Problem Set 6',        ['duedate' => $now - 1920 * $DAY], 'submitted', 42.0],
    // 重修那次：同樣的課名、同樣的作業名，不同的分數。
    ['UG1201R', 'Problem Set 6',       ['duedate' => $now - 1560 * $DAY], 'submitted', 71.0],
    ['UG1202', 'Linked List Exercise', ['duedate' => $now - 1920 * $DAY], 'submitted', 88.0],
    ['UG2101', 'Proof Exercise 3',     ['duedate' => $now - 1740 * $DAY], 'submitted', 79.0],
    ['UG2102', 'Assembly Lab',         ['duedate' => $now - 1740 * $DAY], 'submitted', 83.5],
    ['UG2201', 'Graph Algorithms',     ['duedate' => $now - 1560 * $DAY], 'submitted', 91.0],
    ['UG2202', 'Scheduler Simulation', ['duedate' => $now - 1560 * $DAY], 'submitted', 76.0],
    ['UG3101', 'ER Modelling',         ['duedate' => $now - 1380 * $DAY], 'submitted', 85.0],
    ['UG3102', 'Socket Programming',   ['duedate' => $now - 1380 * $DAY], 'submitted', 90.0],
    ['UG3201', 'Requirements Document',
        ['duedate' => $now - 1200 * $DAY, 'submissiondrafts' => 1], 'submitted', 87.5],
    ['UG3202', 'Hypothesis Testing',   ['duedate' => $now - 1200 * $DAY], 'submitted', 72.0],
    ['UG4101', 'Parser Implementation', ['duedate' => $now - 1020 * $DAY], 'submitted', 93.0],
    ['UG4102', 'Classifier Notebook',  ['duedate' => $now - 1020 * $DAY], 'submitted', 89.0],
    // 大四專題：交了、但從來沒有被評分。過了好幾年仍然是 notgraded，
    // 而「沒有分數」與「零分」在這裡差得最遠。
    ['UG4201', 'Final Report',         ['duedate' => $now - 840 * $DAY], 'submitted', null],
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
    // 這一份是留給 full-run.sh 交的，所以不加限制、不要求聲明、不預先繳交。
    // 以前那個角色由 Chapter 2 Draft 兼著，直到 seed-permissions 把它用日期擋起來
    // ——兩個 fixture 各自都對，合起來就沒有作業可交了，整套測試停在第 10 節。
    ['CS5006', 'Weekly Reflection', ['duedate' => $now + 14 * $DAY], 'new', null],
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
echo "[masters] 身分：學生、助教（非編輯教師）、教師、manager、coursecreator、零選課帳號\n";

purge_all_caches();
echo "[masters] 完成\n";
