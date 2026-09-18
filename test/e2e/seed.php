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

// 三種作業設定，對應三種不同的提交流程。
$want = [
    'A1 direct submit' => ['submissiondrafts' => 0, 'requiresubmissionstatement' => 0],
    'A2 submit button' => ['submissiondrafts' => 1, 'requiresubmissionstatement' => 0],
    'A3 statement'     => ['submissiondrafts' => 1, 'requiresubmissionstatement' => 1],
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
    echo "[seed] $name -> submissiondrafts={$fields['submissiondrafts']} requiresubmissionstatement={$fields['requiresubmissionstatement']}\n";
}

// 權限、服務定義與模組設定都有快取，改完要清。
purge_all_caches();
echo "[seed] caches purged\n";
