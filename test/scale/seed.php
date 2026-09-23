<?php
/**
 * Deterministic ten-year PostgreSQL fixture.
 *
 * Object lifecycles (courses and custom fields) go through Moodle APIs. The
 * two fact tables whose cardinality is the test itself use Moodle's supported
 * insert_records() bulk path: 50,000 users and 2,370,000 enrolments would be
 * prohibitively slow as individual plugin calls, while insert_records()
 * normalises values against the live schema and emits 500-row PostgreSQL
 * inserts. The scale site is disposable and never shares data with the role
 * and destructive-operation matrix.
 */

define('CLI_SCRIPT', true);
require('/var/www/html/config.php');
require_once($CFG->dirroot . '/course/lib.php');
require_once($CFG->dirroot . '/user/lib.php');

const UG_STUDENTS = 40000;
const GRAD_STUDENTS = 10000;
const TERMS = 20;
const COURSES_PER_TERM = 50;
const UG_COURSES_PER_TERM = 7;
const GRAD_COURSES_PER_TERM = 2;
const UG_ACTIVE_TERMS = 8;
const GRAD_ACTIVE_TERMS = 4;
const EXPECTED_ACADEMIC_ENROLMENTS = 2320000;
const EXPECTED_TOTAL_ENROLMENTS = 2370000;

$seed = (int)($argv[1] ?? 20260923);
$now = time();

set_config('noemailever', 1);
set_config('noreplyaddress', 'noreply@example.invalid');

function ensure_customfield_category(): \core_customfield\category_controller {
    global $DB;
    $record = $DB->get_record('customfield_category', [
        'component' => 'core_course', 'area' => 'course', 'itemid' => 0,
        'name' => 'Scale academic metadata',
    ]);
    if ($record) {
        return \core_customfield\category_controller::create($record->id);
    }
    $category = \core_customfield\category_controller::create(0, (object)[
        'name' => 'Scale academic metadata',
        'component' => 'core_course', 'area' => 'course', 'itemid' => 0,
        'contextid' => \context_system::instance()->id,
    ]);
    $category->save();
    return $category;
}

function ensure_text_field(\core_customfield\category_controller $category,
                           string $shortname, string $name, int $sortorder): void {
    global $DB;
    if ($existing = $DB->get_record('customfield_field', [
        'categoryid' => $category->get('id'), 'shortname' => $shortname,
    ])) {
        // Earlier interrupted fixture versions marked fields locked, which
        // correctly made the handler refuse to persist instance values. Make
        // reruns converge instead of requiring a volume reset.
        $config = json_decode($existing->configdata ?: '{}', true) ?: [];
        $config['locked'] = 0;
        $DB->set_field('customfield_field', 'configdata', json_encode($config),
            ['id' => $existing->id]);
        return;
    }
    $handler = $category->get_handler();
    $field = \core_customfield\field_controller::create(
        0, (object)['type' => 'text'], $category);
    $handler->save_field_configuration($field, (object)[
        'categoryid' => $category->get('id'), 'name' => $name,
        'shortname' => $shortname, 'description' => '',
        'descriptionformat' => FORMAT_HTML, 'type' => 'text',
        'sortorder' => $sortorder,
        'configdata' => json_encode([
            'required' => 0, 'uniquevalues' => 0, 'locked' => 0,
            'visibility' => 2, 'defaultvalue' => '',
            'defaultvalueformat' => FORMAT_MOODLE, 'displaysize' => 20,
            'maxlength' => 80, 'ispassword' => 0, 'link' => '',
            'linktarget' => '',
        ]),
    ]);
}

function term_label(int $term): string {
    $year = 2016 + intdiv($term, 2);
    return sprintf('%d-%s', $year, $term % 2 === 0 ? 'Fall' : 'Spring');
}

function term_start(int $term): int {
    $year = 2016 + intdiv($term, 2);
    $month = $term % 2 === 0 ? 8 : 2;
    return gmmktime(0, 0, 0, $month, 1, $year);
}

function ensure_course_scale(string $shortname, string $fullname, int $start,
                             string $credits, string $level, string $term): stdClass {
    global $DB;
    $isnew = false;
    if (!$course = $DB->get_record('course', ['shortname' => $shortname])) {
        $course = create_course((object)[
            'shortname' => $shortname, 'fullname' => $fullname,
            'category' => 1, 'format' => 'topics', 'numsections' => 1,
            'startdate' => $start, 'enddate' => $start + 150 * DAYSECS,
            'visible' => 1,
        ]);
        $isnew = true;
    }
    \core_course\customfield\course_handler::create()->instance_form_save((object)[
        'id' => $course->id,
        'customfield_credits' => $credits,
        'customfield_academic_level' => $level,
        'customfield_academic_term' => $term,
    ], $isnew);
    return $course;
}

function missing_users(array $existing, string $password, int $now): Generator {
    global $CFG;
    for ($i = 1; $i <= UG_STUDENTS; $i++) {
        $username = sprintf('scaleug%05d', $i);
        if (isset($existing[$username])) {
            continue;
        }
        yield (object)[
            'auth' => 'manual', 'confirmed' => 1,
            'mnethostid' => $CFG->mnet_localhost_id, 'username' => $username,
            'password' => $password, 'firstname' => 'Undergraduate',
            'lastname' => sprintf('%05d', $i),
            'email' => $username . '@example.invalid',
            'timecreated' => $now, 'timemodified' => $now,
        ];
    }
    for ($i = 1; $i <= GRAD_STUDENTS; $i++) {
        $username = sprintf('scalegr%05d', $i);
        if (isset($existing[$username])) {
            continue;
        }
        yield (object)[
            'auth' => 'manual', 'confirmed' => 1,
            'mnethostid' => $CFG->mnet_localhost_id, 'username' => $username,
            'password' => $password, 'firstname' => 'Graduate',
            'lastname' => sprintf('%05d', $i),
            'email' => $username . '@example.invalid',
            'timecreated' => $now, 'timemodified' => $now,
        ];
    }
}

function enrolment_rows(array $users, array $enrolids, int $orientation,
                        int $seed, int $now): Generator {
    for ($i = 1; $i <= UG_STUDENTS; $i++) {
        $userid = $users[sprintf('scaleug%05d', $i)];
        $startterm = ($i - 1 + $seed) % (TERMS - UG_ACTIVE_TERMS + 1);
        for ($offset = 0; $offset < UG_ACTIVE_TERMS; $offset++) {
            $term = $startterm + $offset;
            $base = (($i - 1) * UG_COURSES_PER_TERM + $offset * 3 + $seed) % 40;
            for ($j = 0; $j < UG_COURSES_PER_TERM; $j++) {
                $courseindex = ($base + $j) % 40;
                yield enrolment_row($enrolids[$term][$courseindex], $userid, $now);
            }
        }
        yield enrolment_row($orientation, $userid, $now);
    }
    for ($i = 1; $i <= GRAD_STUDENTS; $i++) {
        $userid = $users[sprintf('scalegr%05d', $i)];
        $startterm = ($i - 1 + $seed) % (TERMS - GRAD_ACTIVE_TERMS + 1);
        for ($offset = 0; $offset < GRAD_ACTIVE_TERMS; $offset++) {
            $term = $startterm + $offset;
            $base = (($i - 1) * GRAD_COURSES_PER_TERM + $offset + $seed) % 10;
            for ($j = 0; $j < GRAD_COURSES_PER_TERM; $j++) {
                $courseindex = 40 + (($base + $j) % 10);
                yield enrolment_row($enrolids[$term][$courseindex], $userid, $now);
            }
        }
        yield enrolment_row($orientation, $userid, $now);
    }
}

function enrolment_row(int $enrolid, int $userid, int $now): stdClass {
    return (object)[
        'status' => ENROL_USER_ACTIVE, 'enrolid' => $enrolid,
        'userid' => $userid, 'timestart' => $now - DAYSECS,
        'timeend' => 0, 'modifierid' => 0,
        'timecreated' => $now, 'timemodified' => $now,
    ];
}

echo "[scale] custom fields\n";
$fieldcategory = ensure_customfield_category();
ensure_text_field($fieldcategory, 'credits', 'Credits', 1);
ensure_text_field($fieldcategory, 'academic_level', 'Academic level', 2);
ensure_text_field($fieldcategory, 'academic_term', 'Academic term', 3);
purge_all_caches();

echo "[scale] 50,000 users via insert_records\n";
$existing = $DB->get_records_select_menu('user',
    "username LIKE 'scaleug%' OR username LIKE 'scalegr%'", [], '', 'username,id');
$password = hash_internal_user_password('Student123!');
$DB->insert_records('user', missing_users($existing, $password, $now));
$userrecords = $DB->get_records_select('user',
    "username LIKE 'scaleug%' OR username LIKE 'scalegr%'", [], '', 'id,username');
$users = [];
foreach ($userrecords as $record) {
    $users[$record->username] = (int)$record->id;
}
if (count($users) !== UG_STUDENTS + GRAD_STUDENTS) {
    throw new RuntimeException('scale user cardinality is ' . count($users));
}

echo "[scale] 1,000 academic course instances\n";
$enrolids = [];
for ($term = 0; $term < TERMS; $term++) {
    $enrolids[$term] = [];
    for ($courseindex = 0; $courseindex < COURSES_PER_TERM; $courseindex++) {
        $level = $courseindex < 40 ? 'undergraduate' : 'graduate';
        $catalogue = $level === 'undergraduate' ? $courseindex + 1 : $courseindex - 39;
        $course = ensure_course_scale(
            sprintf('SCALE-T%02d-C%02d', $term, $courseindex),
            sprintf('%s Course %02d', ucfirst($level), $catalogue),
            term_start($term), '3', $level, term_label($term));
        $manual = $DB->get_record('enrol',
            ['courseid' => $course->id, 'enrol' => 'manual'], '*', MUST_EXIST);
        $enrolids[$term][$courseindex] = (int)$manual->id;
    }
    printf("  term %02d/19 complete\n", $term);
}
$orientation = ensure_course_scale('SCALE-ORIENTATION', 'University orientation',
    term_start(0), '0', 'noncredit', 'orientation');
$orientationenrol = $DB->get_record('enrol',
    ['courseid' => $orientation->id, 'enrol' => 'manual'], '*', MUST_EXIST);

echo "[scale] replace 2,370,000 deterministic enrolment facts\n";
$DB->execute("DELETE FROM {user_enrolments}
               WHERE userid IN (SELECT id FROM {user}
                                  WHERE username LIKE 'scaleug%'
                                     OR username LIKE 'scalegr%')");
$DB->insert_records('user_enrolments', enrolment_rows(
    $users, $enrolids, (int)$orientationenrol->id, $seed, $now));

// Only two accounts need to authenticate as students. Keep the 2.37m scale
// facts independent from the much smaller capability graph.
$studentrole = $DB->get_record('role', ['shortname' => 'student'], '*', MUST_EXIST);
foreach ([['scaleug00001', UG_ACTIVE_TERMS, UG_COURSES_PER_TERM, 40],
          ['scalegr00001', GRAD_ACTIVE_TERMS, GRAD_COURSES_PER_TERM, 10]] as $sample) {
    [$username, $active, $perterm, $pool] = $sample;
    $userid = $users[$username];
    $startterm = ($seed) % (TERMS - $active + 1);
    for ($offset = 0; $offset < $active; $offset++) {
        $term = $startterm + $offset;
        $base = $username === 'scaleug00001'
            ? (($offset * 3 + $seed) % 40)
            : (($offset + $seed) % 10);
        for ($j = 0; $j < $perterm; $j++) {
            $idx = $username === 'scaleug00001'
                ? (($base + $j) % $pool)
                : 40 + (($base + $j) % $pool);
            $enrol = $DB->get_record('enrol', ['id' => $enrolids[$term][$idx]], '*', MUST_EXIST);
            role_assign($studentrole->id, $userid, context_course::instance($enrol->courseid)->id);
        }
    }
    role_assign($studentrole->id, $userid, context_course::instance($orientation->id)->id);
}

purge_all_caches();

$truth = [
    'schema_version' => 1, 'seed' => $seed, 'terms' => TERMS,
    'students' => count($users), 'undergraduates' => UG_STUDENTS,
    'graduates' => GRAD_STUDENTS,
    'academic_courses' => TERMS * COURSES_PER_TERM,
    'orientation_courses' => 1,
    'academic_enrolments' => EXPECTED_ACADEMIC_ENROLMENTS,
    'orientation_enrolments' => UG_STUDENTS + GRAD_STUDENTS,
    'total_enrolments' => EXPECTED_TOTAL_ENROLMENTS,
];
echo json_encode($truth, JSON_UNESCAPED_SLASHES) . "\n";
