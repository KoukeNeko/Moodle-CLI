<?php
/**
 * 範圍突變：一次只動一個「縮減維度」，而且保留物件本身。
 *
 * 受測的性質**不是**「端點應該還是要回傳這個物件」——通常它正確地不該回傳。
 * 受測的是：**端點不再回傳一個確實存在的物件時，CLI 不得把那個觀測
 * 強化成更廣的不存在宣稱。**
 *
 * 每一種突變都是這個專案真的踩過的坑，寫成腳本是為了不必再靠豐富 fixture
 * 碰巧撞到第二次。全部可還原，還原就是這個檔案存在的前提。
 *
 * 用法（容器內）：
 *   php /mutate.php list
 *   php /mutate.php apply  <name>
 *   php /mutate.php revert <name>
 */

define('CLI_SCRIPT', true);
require('/var/www/html/config.php');

/**
 * 每一項：說明、套用、還原。
 *
 * 說明那一欄寫的是「這個狀態下，空回應**不能**證明什麼」——那才是產物。
 */
function mutations(): array {
    global $DB;
    return [
        'suspend-enrolment' => [
            'why' => 'ug2 的選課停權。空的課程清單不能證明他沒有選過課。',
            'apply' => function () use ($DB) {
                $u = $DB->get_record('user', ['username' => 'ug2'], '*', MUST_EXIST);
                foreach ($DB->get_records('user_enrolments', ['userid' => $u->id]) as $r) {
                    $r->status = ENROL_USER_SUSPENDED;
                    $DB->update_record('user_enrolments', $r);
                }
            },
            'revert' => function () use ($DB) {
                $u = $DB->get_record('user', ['username' => 'ug2'], '*', MUST_EXIST);
                foreach ($DB->get_records('user_enrolments', ['userid' => $u->id]) as $r) {
                    $r->status = ENROL_USER_ACTIVE;
                    $DB->update_record('user_enrolments', $r);
                }
            },
        ],
        'hide-grade-items' => [
            'why' => 'CS5001 的每個成績項目設為隱藏。空的成績報表不能證明沒有成績。',
            'apply' => function () use ($DB) {
                $c = $DB->get_record('course', ['shortname' => 'CS5001'], '*', MUST_EXIST);
                $DB->set_field('grade_items', 'hidden', 1, ['courseid' => $c->id]);
            },
            'revert' => function () use ($DB) {
                $c = $DB->get_record('course', ['shortname' => 'CS5001'], '*', MUST_EXIST);
                $DB->set_field('grade_items', 'hidden', 0, ['courseid' => $c->id]);
            },
        ],
        'courses-hide-grades' => [
            'why' => '每門課關掉 showgrades。空的總分清單不能證明沒有被評分。',
            'apply' => function () use ($DB) {
                $DB->set_field_select('course', 'showgrades', 0, 'id <> ?', [SITEID]);
            },
            'revert' => function () use ($DB) {
                $DB->set_field_select('course', 'showgrades', 1, 'id <> ?', [SITEID]);
            },
        ],
        'discussions-in-another-group' => [
            'why' => 'CS1001 的分組論壇裡，每一串討論都移到 UG-B。'
                . 'UG-A 的成員看到的空清單不能證明這個論壇沒有討論。',
            'apply' => function () use ($DB) {
                $course = $DB->get_record('course', ['shortname' => 'CS1001'], '*', MUST_EXIST);
                $forum = $DB->get_record('forum',
                    ['course' => $course->id, 'name' => 'Student Questions'], '*', MUST_EXIST);
                $other = $DB->get_record('groups',
                    ['courseid' => $course->id, 'name' => 'UG-B'], '*', MUST_EXIST);
                foreach ($DB->get_records('forum_discussions', ['forum' => $forum->id]) as $d) {
                    $DB->set_field('forum_discussions', 'groupid', $other->id, ['id' => $d->id]);
                }
            },
            'revert' => function () use ($DB) {
                $course = $DB->get_record('course', ['shortname' => 'CS1001'], '*', MUST_EXIST);
                $forum = $DB->get_record('forum',
                    ['course' => $course->id, 'name' => 'Student Questions'], '*', MUST_EXIST);
                $mine = $DB->get_record('groups',
                    ['courseid' => $course->id, 'name' => 'UG-A'], '*', MUST_EXIST);
                foreach ($DB->get_records('forum_discussions', ['forum' => $forum->id]) as $d) {
                    $DB->set_field('forum_discussions', 'groupid', $mine->id, ['id' => $d->id]);
                }
            },
        ],
        'hide-own-submission-summary' => [
            'why' => '對 student 角色 PROHIBIT mod/assign:viewownsubmissionsummary。'
                . 'mod_assign_get_submission_status 會整個省略 lastattempt——'
                . '缺席的繳交狀態不能讀成「還沒交」。',
            'apply' => function () use ($DB) {
                $role = $DB->get_record('role', ['shortname' => 'student'], '*', MUST_EXIST);
                assign_capability('mod/assign:viewownsubmissionsummary', CAP_PROHIBIT,
                    $role->id, \context_system::instance()->id, true);
            },
            'revert' => function () use ($DB) {
                // assign_capability at the system context overwrites the role's
                // own definition rather than layering over it, so unassigning
                // would delete the definition instead of restoring it — and
                // leave every student on the site without the capability, with
                // nothing in the database to show what happened. Put back what
                // Moodle ships for this role.
                $role = $DB->get_record('role', ['shortname' => 'student'], '*', MUST_EXIST);
                $default = get_default_capabilities('student');
                assign_capability('mod/assign:viewownsubmissionsummary',
                    $default['mod/assign:viewownsubmissionsummary'] ?? CAP_ALLOW,
                    $role->id, \context_system::instance()->id, true);
            },
        ],
        'drop-service-function' => [
            'why' => '從 moodle_mobile_app 移除一支站台仍然裝著的函式。'
                . '「這個站台沒有」不能從服務的清單推出來。',
            'apply' => function () use ($DB) {
                $svc = $DB->get_record('external_services',
                    ['shortname' => 'moodle_mobile_app'], '*', MUST_EXIST);
                $DB->delete_records('external_services_functions', [
                    'externalserviceid' => $svc->id,
                    'functionname' => 'core_calendar_get_calendar_monthly_view',
                ]);
            },
            'revert' => function () use ($DB) {
                $svc = $DB->get_record('external_services',
                    ['shortname' => 'moodle_mobile_app'], '*', MUST_EXIST);
                if (!$DB->record_exists('external_services_functions', [
                        'externalserviceid' => $svc->id,
                        'functionname' => 'core_calendar_get_calendar_monthly_view'])) {
                    $DB->insert_record('external_services_functions', (object) [
                        'externalserviceid' => $svc->id,
                        'functionname' => 'core_calendar_get_calendar_monthly_view',
                    ]);
                }
            },
        ],
    ];
}

$what = $argv[1] ?? '';
$name = $argv[2] ?? '';
$all = mutations();

if ($what === 'list') {
    foreach ($all as $key => $m) {
        printf("%-24s %s\n", $key, $m['why']);
    }
    exit(0);
}
if (!in_array($what, ['apply', 'revert'], true) || !isset($all[$name])) {
    fwrite(STDERR, "usage: mutate.php list | apply <name> | revert <name>\n");
    exit(2);
}

$all[$name][$what]();
// 設定與成績都有快取，不清就會量到突變前的世界。
purge_all_caches();
echo "[mutate] $what $name\n";
