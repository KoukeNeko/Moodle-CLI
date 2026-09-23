<?php
/** Independent control-plane assertions for the scale fixture. */
define('CLI_SCRIPT', true);
require('/var/www/html/config.php');

$students = (int)$DB->count_records_select('user',
    "username LIKE 'scaleug%' OR username LIKE 'scalegr%'");
$courses = (int)$DB->count_records_select('course', "shortname LIKE 'SCALE-T%'");
$orientation = $DB->get_record('course', ['shortname' => 'SCALE-ORIENTATION'], '*', MUST_EXIST);

$counts = $DB->get_record_sql("SELECT COUNT(*) AS total,
        COUNT(*) FILTER (WHERE c.shortname LIKE 'SCALE-T%') AS academic,
        COUNT(*) FILTER (WHERE c.id = ?) AS orientation
      FROM {user_enrolments} ue
      JOIN {user} u ON u.id = ue.userid
      JOIN {enrol} e ON e.id = ue.enrolid
      JOIN {course} c ON c.id = e.courseid
     WHERE u.username LIKE 'scaleug%' OR u.username LIKE 'scalegr%'",
    [$orientation->id], MUST_EXIST);

$perstudent = $DB->get_record_sql("SELECT MIN(n) AS minimum, MAX(n) AS maximum,
        COUNT(*) AS students
      FROM (
        SELECT u.id, COUNT(*) AS n
          FROM {user} u
          JOIN {user_enrolments} ue ON ue.userid = u.id
          JOIN {enrol} e ON e.id = ue.enrolid
          JOIN {course} c ON c.id = e.courseid AND c.shortname LIKE 'SCALE-T%'
         WHERE u.username LIKE 'scaleug%'
         GROUP BY u.id
      ) facts", [], MUST_EXIST);
$pergrad = $DB->get_record_sql("SELECT MIN(n) AS minimum, MAX(n) AS maximum,
        COUNT(*) AS students
      FROM (
        SELECT u.id, COUNT(*) AS n
          FROM {user} u
          JOIN {user_enrolments} ue ON ue.userid = u.id
          JOIN {enrol} e ON e.id = ue.enrolid
          JOIN {course} c ON c.id = e.courseid AND c.shortname LIKE 'SCALE-T%'
         WHERE u.username LIKE 'scalegr%'
         GROUP BY u.id
      ) facts", [], MUST_EXIST);

// Parse the term from the persisted course identity and group facts in SQL.
// This deliberately does not reuse seed.php's cohort or course-choice code.
$ugterms = $DB->get_record_sql("SELECT MIN(n) AS minimum, MAX(n) AS maximum,
        COUNT(*) AS groups
      FROM (
        SELECT u.id, SUBSTRING(c.shortname FROM 'SCALE-T([0-9]+)-') AS term,
               COUNT(*) AS n
          FROM {user} u
          JOIN {user_enrolments} ue ON ue.userid = u.id
          JOIN {enrol} e ON e.id = ue.enrolid
          JOIN {course} c ON c.id = e.courseid AND c.shortname LIKE 'SCALE-T%'
         WHERE u.username LIKE 'scaleug%'
         GROUP BY u.id, SUBSTRING(c.shortname FROM 'SCALE-T([0-9]+)-')
      ) facts", [], MUST_EXIST);
$gradterms = $DB->get_record_sql("SELECT MIN(n) AS minimum, MAX(n) AS maximum,
        COUNT(*) AS groups
      FROM (
        SELECT u.id, SUBSTRING(c.shortname FROM 'SCALE-T([0-9]+)-') AS term,
               COUNT(*) AS n
          FROM {user} u
          JOIN {user_enrolments} ue ON ue.userid = u.id
          JOIN {enrol} e ON e.id = ue.enrolid
          JOIN {course} c ON c.id = e.courseid AND c.shortname LIKE 'SCALE-T%'
         WHERE u.username LIKE 'scalegr%'
         GROUP BY u.id, SUBSTRING(c.shortname FROM 'SCALE-T([0-9]+)-')
      ) facts", [], MUST_EXIST);

$dates = $DB->get_record_sql("SELECT MIN(startdate) AS firstdate, MAX(startdate) AS lastdate
                                FROM {course} WHERE shortname LIKE 'SCALE-T%'", [], MUST_EXIST);

$truth = [
    'schema_version' => 1,
    'students' => $students,
    'academic_courses' => $courses,
    'enrolments' => [
        'total' => (int)$counts->total,
        'academic' => (int)$counts->academic,
        'orientation' => (int)$counts->orientation,
    ],
    'undergraduate' => [
        'students' => (int)$perstudent->students,
        'courses_min' => (int)$perstudent->minimum,
        'courses_max' => (int)$perstudent->maximum,
        'term_groups' => (int)$ugterms->groups,
        'courses_per_term_min' => (int)$ugterms->minimum,
        'courses_per_term_max' => (int)$ugterms->maximum,
        'credits_per_term' => 3 * (int)$ugterms->minimum,
    ],
    'graduate' => [
        'students' => (int)$pergrad->students,
        'courses_min' => (int)$pergrad->minimum,
        'courses_max' => (int)$pergrad->maximum,
        'term_groups' => (int)$gradterms->groups,
        'courses_per_term_min' => (int)$gradterms->minimum,
        'courses_per_term_max' => (int)$gradterms->maximum,
        'credits_per_term' => 3 * (int)$gradterms->minimum,
    ],
    'span_days' => (int)(($dates->lastdate - $dates->firstdate) / DAYSECS),
];

$expected = [
    $students === 50000,
    $courses === 1000,
    (int)$counts->academic === 2320000,
    (int)$counts->orientation === 50000,
    (int)$counts->total === 2370000,
    (int)$perstudent->students === 40000,
    (int)$perstudent->minimum === 56 && (int)$perstudent->maximum === 56,
    (int)$ugterms->groups === 320000,
    (int)$ugterms->minimum === 7 && (int)$ugterms->maximum === 7,
    (int)$pergrad->students === 10000,
    (int)$pergrad->minimum === 8 && (int)$pergrad->maximum === 8,
    (int)$gradterms->groups === 40000,
    (int)$gradterms->minimum === 2 && (int)$gradterms->maximum === 2,
    $truth['span_days'] >= 3280,
];

echo json_encode($truth, JSON_UNESCAPED_SLASHES | JSON_PRETTY_PRINT) . "\n";
if (in_array(false, $expected, true)) {
    fwrite(STDERR, "scale truth invariant failed\n");
    exit(1);
}
