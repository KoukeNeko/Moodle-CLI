<?php
/**
 * 測試站台佈建（站台設定部分）。由 seed.sh 呼叫。
 *
 * 用 Moodle 自己的 API，而不是直接改資料庫或只用 cfg.php，因為實測發現：
 *   - `admin/cli/cfg.php --name=enablemobilewebservice --set=1` 只寫設定值，
 *     不會把 external_services 裡 moodle_mobile_app 那一列的 enabled 設成 1。
 *     少了這一步，/login/token.php 會回 servicenotavailable。
 *   - 「已驗證使用者」角色預設沒有 webservice/rest:use。少了它，token 發得出來，
 *     但呼叫 REST 會得到 accessexception。
 *   - moosh activity-add 的 -o 選項不會套用到 assign 的設定欄位，
 *     所以三種作業設定要在這裡改回正確值。
 *   - moosh course-enrol 在 SQLite 上會把 user_enrolments 寫進去、卻沒有建出
 *     role_assignments（Moodle 4.5 實測）。那樣學生連 mod/assign:view 都沒有，
 *     作業列表是空的，每一張表卻都「有資料」，很難查。選課改用 Moodle API。
 *
 * 用法（容器內）：
 *   php /seed.php std    # 標準站：開啟 Web Services
 *   php /seed.php nows   # 變體站：關閉 Mobile Web Services
 */

define('CLI_SCRIPT', true);
require('/var/www/html/config.php');
require_once($CFG->libdir . '/accesslib.php');

$mode = $argv[1] ?? 'std';
if (!in_array($mode, ['std', 'nows'], true)) {
    fwrite(STDERR, "usage: php seed.php [std|nows]\n");
    exit(2);
}

// 測試站一律不寄信：容器沒有可用的 SMTP，預設的 noreply@localhost 又是無效網域，
// 會讓選課時的「課程歡迎信」在 moosh 的開發者除錯模式下噴出 backtrace。
set_config('noemailever', 1);
set_config('noreplyaddress', 'noreply@example.com');

$systemcontext = context_system::instance();
$authuserrole = $DB->get_record('role', ['shortname' => 'user'], '*', MUST_EXIST);

if ($mode === 'std') {
    set_config('enablewebservices', 1);
    set_config('enablemobilewebservice', 1);
    set_config('webserviceprotocols', 'rest');
    $DB->set_field('external_services', 'enabled', 1, ['shortname' => MOODLE_OFFICIAL_MOBILE_SERVICE]);
    assign_capability('webservice/rest:use', CAP_ALLOW, $authuserrole->id, $systemcontext->id, true);
    echo "[seed] web services enabled (rest + mobile service + webservice/rest:use)\n";
} else {
    set_config('enablewebservices', 0);
    set_config('enablemobilewebservice', 0);
    $DB->set_field('external_services', 'enabled', 0, ['shortname' => MOODLE_OFFICIAL_MOBILE_SERVICE]);
    echo "[seed] mobile web services DISABLED (variant site)\n";
}

// 選課與角色指派。
require_once($CFG->dirroot . '/enrol/manual/lib.php');
$course = $DB->get_record('course', ['shortname' => 'CS204'], '*', MUST_EXIST);
$manual = $DB->get_record('enrol',
    ['courseid' => $course->id, 'enrol' => 'manual'], '*', MUST_EXIST);
$enrolplugin = enrol_get_plugin('manual');

// student2 是為了交件測試：交出去不可逆，重跑一次要有乾淨的帳號。
$people = [
    'teacher1' => 'editingteacher',
    'student1' => 'student',
    'student2' => 'student',
];
foreach ($people as $username => $roleshort) {
    $user = $DB->get_record('user', ['username' => $username]);
    if (!$user) {
        echo "[seed] WARNING: user not found: $username\n";
        continue;
    }
    $role = $DB->get_record('role', ['shortname' => $roleshort], '*', MUST_EXIST);
    // 起始日往前挪一天：Moodle 只認「已經開始」的選課，剛好在這一秒開始的會被
    // 當成還沒生效，佈建完立刻查就會看到一門課都沒有。
    $enrolplugin->enrol_user($manual, $user->id, $role->id, time() - DAYSECS);
}
echo "[seed] enrolled: " . implode(', ', array_keys($people)) . "\n";

// 三種作業設定，對應三種不同的提交流程。
// nosubmissions 也要歸零：moosh 建立作業時沒有啟用任何繳交外掛，assign 那一列
// 就被標成「不收繳交」。Moodle 5.1 實測，這時 mod_assign_get_submission_status
// 會回 nopermission（而不是 submissionsenabled=false），完全看不出真正原因。
$want = [
    'A1 direct submit' => ['submissiondrafts' => 0, 'requiresubmissionstatement' => 0, 'nosubmissions' => 0,
                           'intro' => '<p>存檔即視為提交，不需要另外按下提交鍵。</p>'],
    'A2 submit button' => ['submissiondrafts' => 1, 'requiresubmissionstatement' => 0, 'nosubmissions' => 0,
                           'intro' => '<p>存檔之後還要再按一次<strong>提交評分</strong>，否則作業停在草稿。</p>'],
    'A3 statement'     => ['submissiondrafts' => 1, 'requiresubmissionstatement' => 1, 'nosubmissions' => 0,
                           'intro' => '<p>提交前必須同意提交聲明。</p>'],
];
foreach ($want as $name => $fields) {
    $rec = $DB->get_record('assign', ['name' => $name]);
    if (!$rec) {
        echo "[seed] WARNING: assignment not found: $name\n";
        continue;
    }
    foreach ($fields as $field => $value) {
        $DB->set_field('assign', $field, $value, ['id' => $rec->id]);
    }

    // 沒有啟用任何繳交外掛的話，Moodle 會回 submissionsenabled=false，
    // 學生根本無法提交 —— 那樣的測試站驗不出提交流程。
    $plugins = [
        ['assignsubmission', 'file', 'enabled', '1'],
        ['assignsubmission', 'file', 'maxfilesubmissions', '3'],
        ['assignsubmission', 'file', 'maxsubmissionsizebytes', '0'],
        ['assignsubmission', 'onlinetext', 'enabled', '1'],
    ];
    foreach ($plugins as [$subtype, $plugin, $key, $value]) {
        $conditions = ['assignment' => $rec->id, 'subtype' => $subtype,
                       'plugin' => $plugin, 'name' => $key];
        if ($existing = $DB->get_record('assign_plugin_config', $conditions)) {
            $DB->set_field('assign_plugin_config', 'value', $value, ['id' => $existing->id]);
        } else {
            $row = (object) array_merge($conditions, ['value' => $value]);
            $DB->insert_record('assign_plugin_config', $row);
        }
    }

    echo "[seed] $name -> submissiondrafts={$fields['submissiondrafts']} requiresubmissionstatement={$fields['requiresubmissionstatement']} submission plugins enabled\n";
}

// 權限、服務定義與模組設定都有快取，改完要清。
purge_all_caches();
echo "[seed] caches purged\n";
