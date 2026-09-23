<?php
/**
 * Build and export the runtime-role fixture used by the role/function matrix.
 *
 * The matrix must not assume that a Moodle site has exactly the eight roles
 * installed by core. This seeder therefore creates two canary custom roles,
 * gives every runtime role a dedicated identity where Moodle permits one, and
 * emits the roles that are actually present in the database.
 *
 * Run inside the disposable Moodle container:
 *
 *   php /seed-role-matrix.php
 */

define('CLI_SCRIPT', true);
require('/var/www/html/config.php');
require_once($CFG->dirroot . '/user/lib.php');
require_once($CFG->dirroot . '/enrol/manual/lib.php');

/** Create a fixture user without changing it on repeat runs. */
function matrix_ensure_user(string $username, string $firstname, string $lastname): stdClass {
    global $CFG, $DB;
    if ($user = $DB->get_record('user', ['username' => $username, 'mnethostid' => $CFG->mnet_localhost_id])) {
        return $user;
    }
    $user = (object) [
        'username' => $username,
        'auth' => 'manual',
        'confirmed' => 1,
        'mnethostid' => $CFG->mnet_localhost_id,
        'email' => $username . '@roles.example.edu',
        'firstname' => $firstname,
        'lastname' => $lastname,
        'password' => 'Student123!',
    ];
    $user->id = user_create_user($user, true, false);
    return $DB->get_record('user', ['id' => $user->id], '*', MUST_EXIST);
}

/** Create a custom role and apply the archetype defaults exactly once. */
function matrix_ensure_role(string $shortname, string $name, string $archetype): stdClass {
    global $DB;
    if ($role = $DB->get_record('role', ['shortname' => $shortname])) {
        if ((string)$role->archetype !== $archetype) {
            throw new RuntimeException("Role $shortname has unexpected archetype {$role->archetype}");
        }
        return $role;
    }
    $id = create_role($name, $shortname,
        'Disposable Moodle CLI runtime-role matrix fixture.', $archetype);
    if ($archetype !== '') {
        reset_role_capabilities($id);
        set_role_contextlevels($id, get_default_contextlevels($archetype));
    } else {
        set_role_contextlevels($id, [CONTEXT_COURSE]);
    }
    return $DB->get_record('role', ['id' => $id], '*', MUST_EXIST);
}

/** Stable, shell-safe username that does not depend on a translated role name. */
function matrix_username(stdClass $role): string {
    $base = preg_replace('/[^a-z0-9]+/', '', strtolower((string)$role->shortname));
    if ($base === '') {
        $base = 'role';
    }
    return substr('matrix-' . $base . '-' . $role->id, 0, 100);
}

/**
 * Assign a role at one of the context levels it declares.
 *
 * Course roles go through manual enrolment so course-scoped functions observe
 * a real participant, not merely a role assignment. Other roles use a real
 * context of the declared kind. A role with no assignable context remains an
 * authenticated identity and is reported as such rather than silently omitted.
 */
function matrix_assign_role(stdClass $role, stdClass $user, stdClass $course): array {
    global $DB;
    $levels = array_map('intval', array_values(get_role_contextlevels($role->id)));
    $coursecontext = context_course::instance($course->id);

    // Core manager and coursecreator describe site-level personas in the
    // product matrix. Both can also be assigned lower down; picking the
    // course first would silently turn a site manager into a course manager.
    if (in_array((string)$role->archetype, ['manager', 'coursecreator'], true) &&
            in_array(CONTEXT_SYSTEM, $levels, true)) {
        $system = context_system::instance();
        if (!$DB->record_exists('role_assignments', [
                'contextid' => $system->id,
                'userid' => $user->id,
                'roleid' => $role->id,
            ])) {
            role_assign($role->id, $user->id, $system->id);
        }
        return ['status' => 'assigned', 'context_level' => CONTEXT_SYSTEM,
            'context_id' => (int)$system->id, 'course_id' => null];
    }

    if (in_array(CONTEXT_COURSE, $levels, true)) {
        $manual = $DB->get_record('enrol',
            ['courseid' => $course->id, 'enrol' => 'manual'], '*', MUST_EXIST);
        if (!$DB->record_exists('role_assignments', [
                'contextid' => $coursecontext->id,
                'userid' => $user->id,
                'roleid' => $role->id,
            ])) {
            enrol_get_plugin('manual')->enrol_user(
                $manual, $user->id, $role->id, time() - DAYSECS);
        }
        return ['status' => 'assigned', 'context_level' => CONTEXT_COURSE,
            'context_id' => $coursecontext->id, 'course_id' => (int)$course->id];
    }

    $contexts = [];
    if (in_array(CONTEXT_SYSTEM, $levels, true)) {
        $contexts[CONTEXT_SYSTEM] = context_system::instance();
    }
    if (in_array(CONTEXT_COURSECAT, $levels, true)) {
        $contexts[CONTEXT_COURSECAT] = context_coursecat::instance($course->category);
    }
    if (in_array(CONTEXT_MODULE, $levels, true)) {
        $cm = $DB->get_record('course_modules', ['course' => $course->id], 'id', IGNORE_MULTIPLE);
        if ($cm) {
            $contexts[CONTEXT_MODULE] = context_module::instance($cm->id);
        }
    }
    if (in_array(CONTEXT_USER, $levels, true)) {
        $contexts[CONTEXT_USER] = context_user::instance($user->id);
    }

    foreach ([CONTEXT_SYSTEM, CONTEXT_COURSECAT, CONTEXT_MODULE, CONTEXT_USER] as $level) {
        if (!isset($contexts[$level])) {
            continue;
        }
        $context = $contexts[$level];
        if (!$DB->record_exists('role_assignments', [
                'contextid' => $context->id,
                'userid' => $user->id,
                'roleid' => $role->id,
            ])) {
            role_assign($role->id, $user->id, $context->id);
        }
        return ['status' => 'assigned', 'context_level' => $level,
            'context_id' => (int)$context->id, 'course_id' => null];
    }

    return ['status' => 'unassigned', 'context_level' => null,
        'context_id' => null, 'course_id' => null];
}

global $CFG, $DB;

// These are canaries: one proves archetype inheritance is discovered, the
// other proves the harness does not reduce every custom role to an archetype.
matrix_ensure_role('matrixteacher', 'Matrix archetype teacher', 'teacher');
matrix_ensure_role('matrixblank', 'Matrix role without archetype', '');

$course = $DB->get_record('course', ['shortname' => 'CS204'], '*', MUST_EXIST);
$roles = $DB->get_records('role', null, 'sortorder ASC, id ASC');
$rows = [];
$coreShortnames = [
    'manager', 'coursecreator', 'editingteacher', 'teacher',
    'student', 'guest', 'user', 'frontpage',
];

foreach ($roles as $role) {
    $levels = array_map('intval', array_values(get_role_contextlevels($role->id)));
    sort($levels, SORT_NUMERIC);
    $row = [
        'role_id' => (int)$role->id,
        'shortname' => (string)$role->shortname,
        'name' => (string)$role->name,
        'archetype' => (string)$role->archetype,
        'context_levels' => $levels,
        'is_custom' => !in_array((string)$role->shortname, $coreShortnames, true),
        'is_site_administrator' => false,
    ];

    if ($role->archetype === 'guest') {
        $row += [
            'credential_kind' => 'guest',
            'username' => null,
            'assignment' => ['status' => 'implicit', 'context_level' => null,
                'context_id' => null, 'course_id' => null],
        ];
    } else if ($role->archetype === 'user' || $role->archetype === 'frontpage') {
        $user = matrix_ensure_user(matrix_username($role), 'Matrix', ucfirst($role->shortname));
        $row += [
            'credential_kind' => 'password',
            'username' => (string)$user->username,
            // Authenticated-user and front-page roles are supplied by Moodle
            // implicitly; a DB role_assignment would test different semantics.
            'assignment' => ['status' => 'implicit', 'context_level' => null,
                'context_id' => null, 'course_id' => $role->archetype === 'frontpage' ? SITEID : null],
        ];
    } else {
        $user = matrix_ensure_user(matrix_username($role), 'Matrix', ucfirst($role->shortname ?: 'Role'));
        $row += [
            'credential_kind' => 'password',
            'username' => (string)$user->username,
            'assignment' => matrix_assign_role($role, $user, $course),
        ];
    }
    $rows[] = $row;
}

// Site administrator is deliberately not modelled as a role: Moodle grants
// it outside role_assignments, and conflating the two would hide permission
// bugs. A dedicated admin identity avoids exercising only the installer admin.
$admin = matrix_ensure_user('matrix-siteadmin', 'Matrix', 'Site Administrator');
$admins = array_values(array_filter(array_map(
    'intval', explode(',', (string)($CFG->siteadmins ?? '')))));
if (!in_array((int)$admin->id, $admins, true)) {
    $admins[] = (int)$admin->id;
    sort($admins, SORT_NUMERIC);
    set_config('siteadmins', implode(',', $admins));
}
$rows[] = [
    'role_id' => null,
    'shortname' => 'site_administrator',
    'name' => 'Site administrator',
    'archetype' => null,
    'context_levels' => [CONTEXT_SYSTEM],
    'is_custom' => false,
    'is_site_administrator' => true,
    'credential_kind' => 'password',
    'username' => (string)$admin->username,
    'assignment' => ['status' => 'administrator', 'context_level' => CONTEXT_SYSTEM,
        'context_id' => (int)context_system::instance()->id, 'course_id' => null],
];

$document = [
    'schema_version' => 1,
    'moodle' => ['release' => (string)$CFG->release, 'version' => (string)$CFG->version],
    'fixture_course_id' => (int)$course->id,
    'runtime_role_count' => count($roles),
    'principal_count' => count($rows),
    'roles' => $rows,
];

echo json_encode($document,
    JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE), PHP_EOL;
