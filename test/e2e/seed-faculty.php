<?php
/**
 * 一位教師十年的授課史，以及其他角色的真實狀態。由 seed-faculty.sh 呼叫。
 *
 * seed-masters.php 是從**學生**那一側看六年：一個人的學習歷程。這一份補上另一側：
 *   - 同一位教師把同一門課開了十年，每一年是**一門不同的課、同一個課名**。
 *     這是重修那個問題的教師版，而且規模大得多：清單上會有十筆「Introduction to
 *     Programming」。用課名識別或去重，教師就會少掉九年的教學紀錄。
 *   - 每一年的學生都不同。「這門課的參與者」不能跨年快取。
 *   - 早年的課程封存（visible=0）。封存不是刪除，而教師仍然讀得到。
 *   - manager 與 coursecreator 在**類別**層而不是系統層——那是真實站台的樣子，
 *     而且他們的課程清單是空的，理由卻跟「新帳號」完全不同。
 *
 * 用法（容器內）：php /seed-faculty.php
 * 可重複執行：每一項都先查再建。
 */

define('CLI_SCRIPT', true);
require('/var/www/html/config.php');
// Moodle's generic CLI exception text hides whether a failed synthetic seed
// is a transient DB write or a deterministic fixture bug. Log only classes
// and codes, never SQL, fixture identity, or private response data.
$seedphase = 'bootstrap';
set_exception_handler(static function(Throwable $error) use (&$seedphase): void {
    $cause = $error->getPrevious();
    $diagnostic = $error->getMessage();
    if (property_exists($error, 'debuginfo')) {
        $diagnostic .= ' ' . (string)$error->debuginfo;
    }
    $category = 'other';
    if (preg_match('/database (?:table )?is locked|SQLITE_BUSY/i', $diagnostic)) {
        $category = 'sqlite_busy';
    } else if (preg_match('/unique constraint|SQLITE_CONSTRAINT/i', $diagnostic)) {
        $category = 'sqlite_constraint';
    } else if (preg_match('/database or disk is full|SQLITE_FULL/i', $diagnostic)) {
        $category = 'sqlite_full';
    }
    fwrite(STDERR, '[seed-faculty] ' . get_class($error) .
        ' code=' . (string)$error->getCode() .
        ' cause=' . ($cause ? get_class($cause) : 'none') .
        ' causecode=' . ($cause ? (string)$cause->getCode() : 'none') .
        ' phase=' . $seedphase . ' category=' . $category . PHP_EOL);
    exit(1);
});
require_once($CFG->libdir . '/gradelib.php');
require_once($CFG->dirroot . '/user/lib.php');
require_once($CFG->dirroot . '/enrol/manual/lib.php');
require_once($CFG->dirroot . '/mod/assign/lib.php');
require_once($CFG->dirroot . '/mod/assign/locallib.php');
require_once($CFG->dirroot . '/course/lib.php');

$now = time();

function ensure_user_f(string $username, string $first, string $last): stdClass {
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

/** 類別。真實站台的課程在類別底下，而權限也常常掛在類別上。 */
function ensure_category(string $name, string $idnumber): stdClass {
    global $DB;
    if ($existing = $DB->get_record('course_categories', ['idnumber' => $idnumber])) {
        return $existing;
    }
    return core_course_category::create([
        'name' => $name, 'idnumber' => $idnumber, 'visible' => 1,
    ])->get_db_record();
}

function ensure_course_f(string $shortname, string $fullname, int $startdate,
                         int $categoryid, bool $visible): stdClass {
    global $DB;
    if ($existing = $DB->get_record('course', ['shortname' => $shortname])) {
        // 每年重跑 fixture 時，「最近兩年」的邊界會往前移。只在第一次建立
        // 時寫狀態，過一年後就會留下本該封存的課程。fixture 必須收斂到
        // 現在定義的狀態，而不只是「有就算了」。
        // 用 Moodle API 移動類別與切換可見性；直接 update_record 會漏掉
        // context path、cache 與事件等生命週期副作用。
        update_course((object) [
            'id' => $existing->id,
            'fullname' => $fullname,
            'category' => $categoryid,
            'startdate' => $startdate,
            'enddate' => $startdate + 120 * DAYSECS,
            'visible' => $visible ? 1 : 0,
        ]);
        return $DB->get_record('course', ['id' => $existing->id], '*', MUST_EXIST);
    }
    return create_course((object) [
        'shortname' => $shortname, 'fullname' => $fullname,
        'category' => $categoryid, 'format' => 'topics', 'numsections' => 2,
        'startdate' => $startdate, 'enddate' => $startdate + 120 * DAYSECS,
        // 封存的年份：教師仍讀得到，學生看不到。這不是刪除。
        'visible' => $visible ? 1 : 0,
    ]);
}

function ensure_enrolment_f(stdClass $course, stdClass $user, string $roleshort): void {
    global $DB;
    $role = $DB->get_record('role', ['shortname' => $roleshort], '*', MUST_EXIST);
    $context = context_course::instance($course->id);
    $manual = $DB->get_record('enrol',
        ['courseid' => $course->id, 'enrol' => 'manual'], '*', MUST_EXIST);
    $enrolment = $DB->get_record('user_enrolments',
        ['enrolid' => $manual->id, 'userid' => $user->id]);
    if (!$enrolment) {
        enrol_get_plugin('manual')->enrol_user(
            $manual, $user->id, $role->id, time() - DAYSECS);
    } else if ((int) $enrolment->status !== ENROL_USER_ACTIVE) {
        $DB->set_field('user_enrolments', 'status', ENROL_USER_ACTIVE,
            ['id' => $enrolment->id]);
    }
    if (!$DB->record_exists('role_assignments',
            ['contextid' => $context->id, 'userid' => $user->id, 'roleid' => $role->id])) {
        role_assign($role->id, $user->id, $context->id);
    }
}

function ensure_assign_f(stdClass $course, string $name, int $duedate): stdClass {
    global $DB;
    if ($existing = $DB->get_record('assign', ['name' => $name, 'course' => $course->id])) {
        if ((int) $existing->duedate !== $duedate) {
            $existing->duedate = $duedate;
            $DB->update_record('assign', $existing);
            // 作業日期同時存在 assign 與 calendar_events。只改前者會讓 CLI
            // 的作業清單與行事曆各說一個日期。
            $cm = get_coursemodule_from_instance('assign', $existing->id, $course->id,
                false, MUST_EXIST);
            $context = context_module::instance($cm->id);
            assign_update_events(new assign($context, $cm, $course));
        }
        return $DB->get_record('assign', ['id' => $existing->id], '*', MUST_EXIST);
    }
    $module = $DB->get_record('modules', ['name' => 'assign'], '*', MUST_EXIST);
    $cm = (object) [
        'course' => $course->id, 'module' => $module->id, 'instance' => 0,
        'section' => 0, 'visible' => 1, 'visibleoncoursepage' => 1, 'added' => time(),
    ];
    $cm->id = add_course_module($cm);
    $data = (object) [
        'course' => $course->id, 'coursemodule' => $cm->id, 'modulename' => 'assign',
        'name' => $name, 'intro' => "<p>$name</p>", 'introformat' => FORMAT_HTML,
        'alwaysshowdescription' => 1, 'submissiondrafts' => 0,
        'requiresubmissionstatement' => 0, 'sendnotifications' => 0,
        'sendlatenotifications' => 0, 'sendstudentnotifications' => 0,
        'duedate' => $duedate, 'allowsubmissionsfromdate' => 0, 'cutoffdate' => 0,
        'gradingduedate' => 0, 'grade' => 100, 'completionsubmit' => 0,
        'teamsubmission' => 0, 'requireallteammemberssubmit' => 0,
        'teamsubmissiongroupingid' => 0, 'blindmarking' => 0,
        'attemptreopenmethod' => 'none', 'maxattempts' => -1,
        'markingworkflow' => 0, 'markingallocation' => 0, 'nosubmissions' => 0,
        'timelimit' => 0, 'preventsubmissionnotingroup' => 0,
        'assignsubmission_file_enabled' => 1,
        'assignsubmission_file_maxfiles' => 3,
        'assignsubmission_file_maxsizebytes' => 0,
        'assignsubmission_onlinetext_enabled' => 1,
        'assignsubmission_comments_enabled' => 0,
        'assignfeedback_comments_enabled' => 1,
    ];
    $instanceid = assign_add_instance($data, null);
    $DB->set_field('course_modules', 'instance', $instanceid, ['id' => $cm->id]);
    course_add_cm_to_section($course->id, $cm->id, 0);
    rebuild_course_cache($course->id, true);
    return $DB->get_record('assign', ['id' => $instanceid], '*', MUST_EXIST);
}

function ensure_submission_f(stdClass $assign, stdClass $user, ?float $mark): void {
    global $DB;
    if (!$DB->record_exists('assign_submission',
            ['assignment' => $assign->id, 'userid' => $user->id])) {
        $DB->insert_record('assign_submission', (object) [
            'assignment' => $assign->id, 'userid' => $user->id,
            'timecreated' => $assign->duedate - DAYSECS,
            'timemodified' => $assign->duedate - DAYSECS,
            'status' => 'submitted', 'groupid' => 0, 'attemptnumber' => 0, 'latest' => 1,
        ]);
    }
    if ($mark !== null) {
        grade_update('mod/assign', $assign->course, 'mod', 'assign', $assign->id, 0, [
            'userid' => $user->id, 'rawgrade' => $mark,
        ]);
        // 課程總分是另一個 grade_item，不重算就永遠是 null。
        grade_regrade_final_grades($assign->course);
    }
}

// ─── 角色 ───────────────────────────────────────────────────────────────
$seedphase = 'users';
$prof = $DB->get_record('user', ['username' => 'prof1'], '*', MUST_EXIST);
// 同事：prof1 讀不到他的課，所以「教師」不等於「看得到全部」。
$prof2 = ensure_user_f('prof2', 'Hui-Chen', 'Lo');
$mgr   = $DB->get_record('user', ['username' => 'mgr1'], '*', MUST_EXIST);
$cc    = $DB->get_record('user', ['username' => 'cc1'], '*', MUST_EXIST);

// ─── 類別 ───────────────────────────────────────────────────────────────
$seedphase = 'categories';
$dept    = ensure_category('Computer Science', 'DEPT-CS');
$archive = ensure_category('Archived years', 'DEPT-CS-ARCHIVE');

// ─── 十年的同一門課 ──────────────────────────────────────────────────────
//
// 每一年是一門新的課，課名一模一樣。最近兩年還開著，之前八年封存。
$years = [];
$made = 0;
for ($offset = 9; $offset >= 0; $offset--) {
    $year = (int) date('Y', $now) - $offset;
    $seedphase = 'year_course_' . $year;
    // 學年用固定的 9 月 1 日，而不是從「現在」往回減 365 天。後者每次
    // 執行都會讓十年資料微移幾秒，也會在閏年逐漸偏離學期邊界。
    $startdate = make_timestamp($year, 9, 1, 0, 0, 0);
    $live = $offset <= 1;
    $course = ensure_course_f(
        "CS1001-$year", 'Introduction to Programming',
        $startdate, $live ? $dept->id : $archive->id, $live);
    $seedphase = 'year_enrolments_' . $year;
    ensure_enrolment_f($course, $prof, 'editingteacher');
    $years[$year] = $course;

    // 每一年一批新學生：參與者名單不能跨年快取。
    $cohort = [];
    foreach (range(1, 3) as $n) {
        $student = ensure_user_f("s{$year}-$n", "Student$n", "Of$year");
        ensure_enrolment_f($course, $student, 'student');
        $cohort[] = $student;
    }

    $seedphase = 'year_assignments_' . $year;
    $hw = ensure_assign_f($course, 'Exercise 1', $startdate + 30 * DAYSECS);
    $final = ensure_assign_f($course, 'Final Project', $startdate + 100 * DAYSECS);
    $made += 2;
    $seedphase = 'year_submissions_' . $year;
    foreach ($cohort as $i => $student) {
        // 早年的都改完了；今年的還沒。
        ensure_submission_f($hw, $student, $live && $offset === 0 ? null : 70.0 + $i * 8);
        ensure_submission_f($final, $student, $live && $offset === 0 ? null : 65.0 + $i * 10);
    }
}

// 跨年重跑時，第十一年以前的課仍留在站上（資料不刪），但不再算進
// prof1 的「最近十屆」。退選比刪課更接近真實封存流程，也讓 fixture 每年
// 都收斂到剛好十屆，不會在長壽 volume 裡變成 11、12、…。
$keptids = array_fill_keys(array_map(
    static fn(stdClass $course): int => (int) $course->id, $years), true);
$seedphase = 'retired_courses';
$like = $DB->sql_like('shortname', ':historyprefix', false);
foreach ($DB->get_records_select('course', $like,
        ['historyprefix' => 'CS1001-%']) as $oldcourse) {
    if (!preg_match('/^CS1001-\d{4}$/', $oldcourse->shortname)
            || isset($keptids[(int) $oldcourse->id])) {
        continue;
    }
    $manual = $DB->get_record('enrol',
        ['courseid' => $oldcourse->id, 'enrol' => 'manual']);
    if ($manual && $DB->record_exists('user_enrolments',
            ['enrolid' => $manual->id, 'userid' => $prof->id])) {
        enrol_get_plugin('manual')->unenrol_user($manual, $prof->id);
    }
}
printf("[faculty] prof1 的十年：%d 門同名課程（%d 門封存），作業 %d 份\n",
    count($years), 8, $made);

// 同事的課：prof1 不在裡面，所以教師的清單不等於站台上的全部。
$seedphase = 'colleague_course';
$other = ensure_course_f('CS3001-' . date('Y', $now), 'Operating Systems',
    $now - 60 * DAYSECS, $dept->id, true);
ensure_enrolment_f($other, $prof2, 'editingteacher');
echo "[faculty] 同事 prof2 開了 CS3001，prof1 不在裡面\n";

// ─── 其他角色：在類別層，不是系統層 ─────────────────────────────────────
//
// 系統層的 manager 看得到整個站台；類別層的只看得到自己的系。兩者的課程清單
// 都是空的，成因卻完全不同——而空清單長得一模一樣。
$seedphase = 'category_roles';
foreach ([[$mgr, 'manager'], [$cc, 'coursecreator']] as [$user, $roleshort]) {
    $role = $DB->get_record('role', ['shortname' => $roleshort], '*', MUST_EXIST);
    $context = context_coursecat::instance($dept->id);
    if (!$DB->record_exists('role_assignments',
            ['contextid' => $context->id, 'userid' => $user->id, 'roleid' => $role->id])) {
        role_assign($role->id, $user->id, $context->id);
    }
}
echo "[faculty] mgr1 與 cc1 改掛在 Computer Science 類別上（系統層的保留）\n";

$seedphase = 'purge_caches';
purge_all_caches();
printf("[faculty] 完成：課程 %d 門、作業 %d 份、使用者 %d 個\n",
    $DB->count_records('course') - 1, $DB->count_records('assign'),
    $DB->count_records('user'));
