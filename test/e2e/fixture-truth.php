<?php
/**
 * 直接問資料庫，回答測試賴以成立的前提。由 fixture-truth.sh 呼叫。
 *
 * 為什麼不用 CLI 問：被測的工具不能替自己的前提作證。舉例來說，
 * `pick_submittable` 是用 `moodle assignment status` 挑「還沒交」的作業的——
 * 如果那支指令把已交的報成 new，我們就會挑到交過的那份、submit 失敗、
 * 然後報告一個假的回歸。兩個缺陷互相作偽證，而且看起來很像真的。
 *
 * 用法（容器內）：
 *   php /fixture-truth.php submittable <username>   還沒交的作業 id，一行一個
 *   php /fixture-truth.php courseid <shortname>   課程 id
 *   php /fixture-truth.php calendar <username> [天數]  站台自己算的到期筆數
 *   php /fixture-truth.php forums <courseid>        該課程的論壇數（不套權限）
 *   php /fixture-truth.php readable <username> <courseid>  1／0
 */

define('CLI_SCRIPT', true);
require('/var/www/html/config.php');

$what = $argv[1] ?? '';

/** 這個帳號在這門課裡的身分不影響答案：問的是資料庫裡有幾個。 */
function forum_count(int $courseid): int {
    global $DB;
    return $DB->count_records('forum', ['course' => $courseid]);
}

function user_by_name(string $username): stdClass {
    global $DB;
    return $DB->get_record('user', ['username' => $username], '*', MUST_EXIST);
}

switch ($what) {
    case 'submittable':
        // 「還沒交」照資料庫的定義：沒有繳交紀錄，或最新那筆不是 submitted。
        // 刻意不管可用性限制與權限——那些是測試要觀察的東西，不是前提。
        $user = user_by_name($argv[2]);
        foreach ($DB->get_records('assign', null, 'id') as $assign) {
            // get_records, not get_record: a reopened attempt writes a second
            // row before it clears latest on the first, and get_record turns
            // that into a fatal. A control plane that dies half way through
            // prints a shorter list and says nothing — the caller then reads
            // the missing ids as "already submitted", which is the one answer
            // this file exists to make impossible.
            $rows = $DB->get_records('assign_submission',
                ['assignment' => $assign->id, 'userid' => $user->id, 'latest' => 1],
                'attemptnumber DESC');
            $submission = reset($rows);
            if (!$submission || $submission->status !== 'submitted') {
                echo $assign->id . "\n";
            }
        }
        break;

    case 'courseid':
        // 課程 id 是站台配的，fixture 只知道短名。問 CLI 也可以——但接下來要斷言的
        // 正是「這個帳號讀不到這門課」，用讀不到的東西找 id 是問錯人。
        echo $DB->get_field('course', 'id', ['shortname' => $argv[2]], MUST_EXIST) . "\n";
        break;

    case 'calendar':
        // Moodle's own upcoming view, run as that user — not a reimplementation
        // of it. calendar_get_view() is what core_calendar_get_calendar_upcoming_view
        // calls, so the count here is the site's answer to the same question
        // the command claims to answer, arrived at without the CLI.
        require_once($CFG->dirroot . '/calendar/lib.php');
        $user = user_by_name($argv[2]);
        \core\session\manager::set_user($user);
        $PAGE->set_url('/calendar/');
        $calendar = \calendar_information::create(time(), SITEID, null);
        // The lookahead is passed explicitly so the control plane asks the
        // same question the command does. Left to the site it answers for
        // calendar_lookahead days, which is a different window and would make
        // an honest command look wrong.
        $days = isset($argv[3]) ? (int) $argv[3] : 30;
        [$data] = calendar_get_view($calendar, 'upcoming', true, false, $days);
        echo count($data->events) . "\n";
        break;

    case 'suspended':
        // 停權不是退選。問資料庫這個帳號還有幾筆停權的選課紀錄——那正是
        // core_enrol_get_users_courses 不會回、而 CLI 不能因此否認的東西。
        $user = user_by_name($argv[2]);
        echo $DB->count_records('user_enrolments',
            ['userid' => $user->id, 'status' => ENROL_USER_SUSPENDED]) . "\n";
        break;

    case 'forums':
        echo forum_count((int)$argv[2]) . "\n";
        break;

    case 'readable':
        // 這個問題只有 Moodle 自己答得準，所以用它的 Access API 而不是猜。
        $user = user_by_name($argv[2]);
        $context = context_course::instance((int)$argv[3], IGNORE_MISSING);
        echo ($context && is_enrolled($context, $user) ) || ($context && has_capability('moodle/course:view', $context, $user))
            ? "1\n" : "0\n";
        break;

    case 'release':
        // 版本是「兩輪能不能比」的判準之一，所以取得它不能依賴 CLI 的設定檔：
        // e2e 收尾前會 `auth logout`，那時設定檔已經空了。
        echo $CFG->release . "\n";
        break;

    default:
        fwrite(STDERR, "usage: fixture-truth.php submittable|courseid|calendar|suspended|forums|readable|release\n");
        exit(2);
}
