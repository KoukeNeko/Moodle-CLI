# Complete command reference

[English](Command-Reference) · [繁體中文](Command-Reference-zh-TW)

This page is generated from `moodle commands --json`; every public command must appear here. The synopsis is authoritative for positional arguments. Site, role, and capability restrictions are reported explicitly.

## Global flags

| Flag | Meaning |
| --- | --- |
| `--json` | Emit the versioned JSON contract on stdout. |
| `--pretty` | Indent JSON output; meaningful with --json. |
| `--read-only` | Hide and refuse every command that may mutate Moodle. |
| `--backend auto|ws-only` | Allow automatic fallback routes, or restrict the run to Web Services. |

## Commands (107)

### `moodle api`

Work with the site's web service functions directly

- Synopsis: `moodle api [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle api call`

Call one web service function directly

- Synopsis: `moodle api call <function> [flags]`
- JSON response kind: `api.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--allow-write` | permit a function that can change something |
| `--dry-run` | show what would be sent without sending it |
| `--param` | a parameter as name=value, repeatable; Moodle's bracket notation works, such as courseids[0]=2 |
| `--params-json` | all parameters as a JSON object, for anything with nested structure |
| `--site` | site to call |

### `moodle api functions`

List the functions this account can call, and what is known about them

- Synopsis: `moodle api functions [flags]`
- JSON response kind: `api.functions`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--match` | only functions whose name contains this |
| `--site` | site to list functions from |
| `--unreviewed` | only functions this project has not reviewed |
| `--writes` | only functions that can change something |

### `moodle assignment`

List assignments and hand work in

- Synopsis: `moodle assignment [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle assignment extend`

Set submission extension dates

- Synopsis: `moodle assignment extend [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_assign_save_user_extensions` |
| `--site` | site to call mod_assign_save_user_extensions on |
| `--yes` | confirm this Moodle write |

### `moodle assignment grade`

Grade one assignment submission

- Synopsis: `moodle assignment grade [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_assign_save_grade` |
| `--site` | site to call mod_assign_save_grade on |
| `--yes` | confirm this Moodle write |

### `moodle assignment list`

List your assignments

- Synopsis: `moodle assignment list [flags]`
- JSON response kind: `assignment.list`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--course` | limit to these course ids (repeatable); every course by default |
| `--current` | only courses running now: started, and not yet past their end date |
| `--site` | site to list assignments from |

### `moodle assignment lock`

Lock selected submissions

- Synopsis: `moodle assignment lock [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_assign_lock_submissions` |
| `--site` | site to call mod_assign_lock_submissions on |
| `--yes` | confirm this Moodle write |

### `moodle assignment reveal-identities`

Reveal identities for blind marking

- Synopsis: `moodle assignment reveal-identities [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_assign_reveal_identities` |
| `--site` | site to call mod_assign_reveal_identities on |
| `--yes` | confirm this Moodle write |

### `moodle assignment revert`

Revert selected submissions to draft

- Synopsis: `moodle assignment revert [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_assign_revert_submissions_to_draft` |
| `--site` | site to call mod_assign_revert_submissions_to_draft on |
| `--yes` | confirm this Moodle write |

### `moodle assignment show`

Show one assignment and where you stand in it

- Synopsis: `moodle assignment show <assignment-id|url> [flags]`
- JSON response kind: `assignment.show`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--site` | site to read the assignment from |

### `moodle assignment status`

Show whether your work is saved, handed in, or neither

- Synopsis: `moodle assignment status <assignment-id|url> [flags]`
- JSON response kind: `assignment.status`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--site` | site to read the assignment from |

### `moodle assignment submissions`

List assignment submissions

- Synopsis: `moodle assignment submissions [flags]`
- JSON response kind: `workflow.call`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_assign_get_submissions` |
| `--site` | site to call mod_assign_get_submissions on |

### `moodle assignment submit`

Hand work in, and report what Moodle says afterwards

- Synopsis: `moodle assignment submit <assignment-id|url> <file>... [flags]`
- JSON response kind: `assignment.submit`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--accept-statement` | record that you accept this assignment's submission statement |
| `--account` | account to act as |
| `--draft` | save the work without handing it in for grading |
| `--dry-run` | show what would be sent without sending anything |
| `--site` | site to submit to |
| `--yes` | do not ask for confirmation |

### `moodle assignment unlock`

Unlock selected submissions

- Synopsis: `moodle assignment unlock [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_assign_unlock_submissions` |
| `--site` | site to call mod_assign_unlock_submissions on |
| `--yes` | confirm this Moodle write |

### `moodle auth`

Sign in to Moodle and inspect the current session

- Synopsis: `moodle auth [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle auth handler-status`

Show whether the browser sign-in handler is installed

- Synopsis: `moodle auth handler-status [flags]`
- JSON response kind: `auth.handler`
- Data effect: **read-only**

### `moodle auth import-browser`

Take this site's session from a browser you are already signed in to

- Synopsis: `moodle auth import-browser [flags]`
- JSON response kind: `auth.import_browser`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--browser` | select safari, firefox or chromium (default: discover profiles) |
| `--cookie-name` | the session cookie's name, if the site has renamed it (default MoodleSession) |
| `--list-profiles` | list the browser profiles on this machine and stop |
| `--profile` | read this browser profile directory or Safari cookie file |
| `--site` | site to import a session for |
| `--store` | keep the session in the OS keychain so later commands need no flag |

### `moodle auth import-session`

Verify and store a Moodle session pasted privately from your browser

- Synopsis: `moodle auth import-session [flags]`
- JSON response kind: `auth.login`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | name for the imported account (default: browser-<user id>) |
| `--cookie-name` | session cookie name, if the site renamed it |
| `--site` | site to import the session for |
| `--stdin` | read the cookie value or name=value from stdin instead of a hidden terminal prompt |

### `moodle auth login`

Sign in and store the credential in the OS keychain

- Synopsis: `moodle auth login [flags]`
- JSON response kind: `auth.login`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | name for this account (defaults to the Moodle username) |
| `--callback` | a pasted <scheme>://token=... callback URL (manual method) |
| `--method` | login method: token, password, qr, mobilelaunch, browser-session or manual |
| `--passport` | the passport used to start the login, so the callback can be verified |
| `--password-stdin` | read the password from stdin |
| `--qr` | the decoded content of a login QR code (qr method) |
| `--session-cookie` | a session your browser already holds, as MoodleSession=… (browser-session method) |
| `--site` | site to sign in to |
| `--token` | an existing web service token |
| `--token-stdin` | read the token from stdin |
| `--username` | Moodle username (password method) |

### `moodle auth logout`

Delete the stored credential from this machine

- Synopsis: `moodle auth logout [flags]`
- JSON response kind: `auth.logout`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to sign out |
| `--site` | site to sign out of |

### `moodle auth methods`

Show which login methods this site supports

- Synopsis: `moodle auth methods [flags]`
- JSON response kind: `auth.methods`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--site` | site to check |

### `moodle auth register-handler`

Let your browser hand sign-ins back to this tool

- Synopsis: `moodle auth register-handler [flags]`
- JSON response kind: `auth.handler`
- Data effect: **write**

### `moodle auth status`

Show who is signed in, and check the credential still works

- Synopsis: `moodle auth status [flags]`
- JSON response kind: `auth.status`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to check |
| `--site` | site to check |

### `moodle auth unregister-handler`

Remove the browser sign-in handler

- Synopsis: `moodle auth unregister-handler [flags]`
- JSON response kind: `auth.handler`
- Data effect: **write**

### `moodle calendar`

See what you still have to do

- Synopsis: `moodle calendar [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle calendar create`

Create calendar events

- Synopsis: `moodle calendar create [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_calendar_create_calendar_events` |
| `--site` | site to call core_calendar_create_calendar_events on |
| `--yes` | confirm this Moodle write |

### `moodle calendar delete`

Delete calendar events

- Synopsis: `moodle calendar delete [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_calendar_delete_calendar_events` |
| `--site` | site to call core_calendar_delete_calendar_events on |
| `--yes` | confirm this Moodle write |

### `moodle calendar upcoming`

List deadlines and to-dos, overdue work included

- Synopsis: `moodle calendar upcoming [flags]`
- JSON response kind: `calendar.upcoming`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--days` | how many days ahead to look; 0 for everything Moodle offers |
| `--limit` | maximum number of events to return |
| `--site` | site to read the calendar from |

### `moodle calendar update`

Update a calendar event through Moodle's event form

- Synopsis: `moodle calendar update [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_calendar_submit_create_update_form` |
| `--site` | site to call core_calendar_submit_create_update_form on |
| `--yes` | confirm this Moodle write |

### `moodle commands`

Describe every command, for scripts and agents

- Synopsis: `moodle commands [flags]`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `-h, --help` | help for commands |

### `moodle completion`

Read and update completion status

- Synopsis: `moodle completion [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle completion activity`

Show activity completion in a course

- Synopsis: `moodle completion activity [flags]`
- JSON response kind: `workflow.call`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_completion_get_activities_completion_status` |
| `--site` | site to call core_completion_get_activities_completion_status on |

### `moodle completion course`

Show course completion

- Synopsis: `moodle completion course [flags]`
- JSON response kind: `workflow.call`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_completion_get_course_completion_status` |
| `--site` | site to call core_completion_get_course_completion_status on |

### `moodle completion mark`

Manually mark activity completion

- Synopsis: `moodle completion mark [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_completion_update_activity_completion_status_manually` |
| `--site` | site to call core_completion_update_activity_completion_status_manually on |
| `--yes` | confirm this Moodle write |

### `moodle course`

Read and manage courses allowed by your Moodle role

- Synopsis: `moodle course [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle course contents`

Show sections, activities and files in a course

- Synopsis: `moodle course contents [flags]`
- JSON response kind: `workflow.call`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_course_get_contents` |
| `--site` | site to call core_course_get_contents on |

### `moodle course create`

Create one or more courses

- Synopsis: `moodle course create [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_course_create_courses` |
| `--site` | site to call core_course_create_courses on |
| `--yes` | confirm this Moodle write |

### `moodle course delete`

Delete disposable courses

- Synopsis: `moodle course delete [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_course_delete_courses` |
| `--site` | site to call core_course_delete_courses on |
| `--yes` | confirm this Moodle write |

### `moodle course list`

List your courses

- Synopsis: `moodle course list [flags]`
- JSON response kind: `course.list`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to list courses for |
| `--current` | only courses running now: started, and not yet past their end date |
| `--cursor` | continue a previous listing |
| `--limit` | maximum number of courses to return |
| `--site` | site to list courses from |

### `moodle course search`

Search courses the account may discover

- Synopsis: `moodle course search [flags]`
- JSON response kind: `workflow.call`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_course_search_courses` |
| `--site` | site to call core_course_search_courses on |

### `moodle course show`

Show a course by id, shortname or idnumber

- Synopsis: `moodle course show [flags]`
- JSON response kind: `workflow.call`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_course_get_courses_by_field` |
| `--site` | site to call core_course_get_courses_by_field on |

### `moodle course update`

Update one or more courses

- Synopsis: `moodle course update [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_course_update_courses` |
| `--site` | site to call core_course_update_courses on |
| `--yes` | confirm this Moodle write |

### `moodle doctor`

Diagnose a site: what works, what does not, and why

- Synopsis: `moodle doctor [flags]`
- JSON response kind: `doctor`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to diagnose |
| `--site` | site to diagnose |

### `moodle enrolment`

Inspect enrolment methods and manage course enrolments

- Synopsis: `moodle enrolment [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle enrolment add`

Manually enrol users in courses

- Synopsis: `moodle enrolment add [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe enrol_manual_enrol_users` |
| `--site` | site to call enrol_manual_enrol_users on |
| `--yes` | confirm this Moodle write |

### `moodle enrolment methods`

List enrolment methods available in a course

- Synopsis: `moodle enrolment methods [flags]`
- JSON response kind: `workflow.call`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_enrol_get_course_enrolment_methods` |
| `--site` | site to call core_enrol_get_course_enrolment_methods on |

### `moodle enrolment remove`

Remove one user enrolment from a course

- Synopsis: `moodle enrolment remove [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_enrol_unenrol_user_enrolment` |
| `--site` | site to call core_enrol_unenrol_user_enrolment on |
| `--yes` | confirm this Moodle write |

### `moodle enrolment update`

Update one user enrolment through Moodle's enrolment form

- Synopsis: `moodle enrolment update [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_enrol_submit_user_enrolment_form` |
| `--site` | site to call core_enrol_submit_user_enrolment_form on |
| `--yes` | confirm this Moodle write |

### `moodle file`

Fetch files the site is holding

- Synopsis: `moodle file [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle file download`

Download a file from the site you are signed in to

- Synopsis: `moodle file download <url> [flags]`
- JSON response kind: `file.download`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--as` | save under this name instead of the one the site suggests |
| `--dir` | directory to save into (default: the working directory) |
| `--force` | replace a file that is already there |
| `--site` | site to download from |

### `moodle forum`

Read course discussions

- Synopsis: `moodle forum [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle forum create`

Create a forum discussion

- Synopsis: `moodle forum create [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_forum_add_discussion` |
| `--site` | site to call mod_forum_add_discussion on |
| `--yes` | confirm this Moodle write |

### `moodle forum delete`

Delete a discussion post

- Synopsis: `moodle forum delete [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_forum_delete_post` |
| `--site` | site to call mod_forum_delete_post on |
| `--yes` | confirm this Moodle write |

### `moodle forum discussions`

List the threads in one forum

- Synopsis: `moodle forum discussions <forum-id|url> [flags]`
- JSON response kind: `forum.discussions`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--site` | site to read the forum from |

### `moodle forum edit`

Edit a discussion post

- Synopsis: `moodle forum edit [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_forum_update_discussion_post` |
| `--site` | site to call mod_forum_update_discussion_post on |
| `--yes` | confirm this Moodle write |

### `moodle forum favourite`

Favourite a discussion

- Synopsis: `moodle forum favourite [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_forum_toggle_favourite_state` |
| `--site` | site to call mod_forum_toggle_favourite_state on |
| `--yes` | confirm this Moodle write |

### `moodle forum list`

List the forums in your courses

- Synopsis: `moodle forum list [flags]`
- JSON response kind: `forum.list`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--course` | limit to these course ids (repeatable); every course by default |
| `--site` | site to list forums from |

### `moodle forum lock`

Lock a discussion at a timestamp

- Synopsis: `moodle forum lock [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_forum_set_lock_state` |
| `--site` | site to call mod_forum_set_lock_state on |
| `--yes` | confirm this Moodle write |

### `moodle forum pin`

Pin a discussion

- Synopsis: `moodle forum pin [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_forum_set_pin_state` |
| `--site` | site to call mod_forum_set_pin_state on |
| `--yes` | confirm this Moodle write |

### `moodle forum read`

Read one thread, oldest post first

- Synopsis: `moodle forum read <discussion-id|url> [flags]`
- JSON response kind: `forum.thread`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--site` | site to read the discussion from |

### `moodle forum reply`

Reply to a forum discussion

- Synopsis: `moodle forum reply [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_forum_add_discussion_post` |
| `--site` | site to call mod_forum_add_discussion_post on |
| `--yes` | confirm this Moodle write |

### `moodle forum subscribe`

Subscribe to a forum

- Synopsis: `moodle forum subscribe [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_forum_set_forum_subscription` |
| `--site` | site to call mod_forum_set_forum_subscription on |
| `--yes` | confirm this Moodle write |

### `moodle forum unfavourite`

Remove a discussion from favourites

- Synopsis: `moodle forum unfavourite [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_forum_toggle_favourite_state` |
| `--site` | site to call mod_forum_toggle_favourite_state on |
| `--yes` | confirm this Moodle write |

### `moodle forum unlock`

Unlock a discussion

- Synopsis: `moodle forum unlock [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_forum_set_lock_state` |
| `--site` | site to call mod_forum_set_lock_state on |
| `--yes` | confirm this Moodle write |

### `moodle forum unpin`

Unpin a discussion

- Synopsis: `moodle forum unpin [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_forum_set_pin_state` |
| `--site` | site to call mod_forum_set_pin_state on |
| `--yes` | confirm this Moodle write |

### `moodle forum unsubscribe`

Unsubscribe from a forum

- Synopsis: `moodle forum unsubscribe [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe mod_forum_set_forum_subscription` |
| `--site` | site to call mod_forum_set_forum_subscription on |
| `--yes` | confirm this Moodle write |

### `moodle grade`

Read your grades

- Synopsis: `moodle grade [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle grade category-create`

Create gradebook categories

- Synopsis: `moodle grade category-create [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_grades_create_gradecategories` |
| `--site` | site to call core_grades_create_gradecategories on |
| `--yes` | confirm this Moodle write |

### `moodle grade list`

Show one course's gradebook

- Synopsis: `moodle grade list [flags]`
- JSON response kind: `grade.list`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--course` | course to show the gradebook of |
| `--site` | site to read grades from |

### `moodle grade overview`

Show your total in every course

- Synopsis: `moodle grade overview [flags]`
- JSON response kind: `grade.overview`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--site` | site to read grades from |

### `moodle grade update`

Update grades for a component

- Synopsis: `moodle grade update [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_grades_update_grades` |
| `--site` | site to call core_grades_update_grades on |
| `--yes` | confirm this Moodle write |

### `moodle group`

Manage course groups and memberships

- Synopsis: `moodle group [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle group create`

Create groups

- Synopsis: `moodle group create [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_group_create_groups` |
| `--site` | site to call core_group_create_groups on |
| `--yes` | confirm this Moodle write |

### `moodle group delete`

Delete groups

- Synopsis: `moodle group delete [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_group_delete_groups` |
| `--site` | site to call core_group_delete_groups on |
| `--yes` | confirm this Moodle write |

### `moodle group list`

List groups in a course

- Synopsis: `moodle group list [flags]`
- JSON response kind: `workflow.call`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_group_get_course_groups` |
| `--site` | site to call core_group_get_course_groups on |

### `moodle group member-add`

Add members to groups

- Synopsis: `moodle group member-add [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_group_add_group_members` |
| `--site` | site to call core_group_add_group_members on |
| `--yes` | confirm this Moodle write |

### `moodle group member-remove`

Remove members from groups

- Synopsis: `moodle group member-remove [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_group_delete_group_members` |
| `--site` | site to call core_group_delete_group_members on |
| `--yes` | confirm this Moodle write |

### `moodle group update`

Update groups

- Synopsis: `moodle group update [flags]`
- JSON response kind: `workflow.call`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_group_update_groups` |
| `--site` | site to call core_group_update_groups on |
| `--yes` | confirm this Moodle write |

### `moodle mcp`

Serve Moodle to an AI agent

- Synopsis: `moodle mcp [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle mcp serve`

Run a Model Context Protocol server on stdin and stdout

- Synopsis: `moodle mcp serve [flags]`
- JSON response kind: `mcp.serve`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--allow-write` | offer the tools that can change things on the site |
| `--site` | site to serve |

### `moodle participant`

Find and inspect course participants

- Synopsis: `moodle participant [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle participant list`

List participants enrolled in a course

- Synopsis: `moodle participant list [flags]`
- JSON response kind: `workflow.call`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_enrol_get_enrolled_users` |
| `--site` | site to call core_enrol_get_enrolled_users on |

### `moodle participant search`

Search users eligible for course enrolment

- Synopsis: `moodle participant search [flags]`
- JSON response kind: `workflow.call`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_enrol_search_users` |
| `--site` | site to call core_enrol_search_users on |

### `moodle participant show`

Show course-scoped user profiles

- Synopsis: `moodle participant show [flags]`
- JSON response kind: `workflow.call`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--dry-run` | validate and show the operation without sending it |
| `--param` | a top-level parameter as name=value, repeatable |
| `--params-json` | structured parameters as a JSON object; see `moodle ws describe core_user_get_course_user_profiles` |
| `--site` | site to call core_user_get_course_user_profiles on |

### `moodle quiz`

See your quizzes and how your attempts went

- Synopsis: `moodle quiz [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle quiz list`

List the quizzes in your courses, with when they open and close

- Synopsis: `moodle quiz list [flags]`
- JSON response kind: `quiz.list`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--course` | limit to these course ids (repeatable); every course by default |
| `--current` | only courses running now: started, and not yet past their end date |
| `--site` | site to list quizzes from |

### `moodle quiz show`

Show one quiz and your attempts at it

- Synopsis: `moodle quiz show <quiz-id|url> [flags]`
- JSON response kind: `quiz.show`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--site` | site to read the quiz from |

### `moodle resolve`

Say what a Moodle link points at

- Synopsis: `moodle resolve <url> [flags]`
- JSON response kind: `resolve`
- Data effect: **read-only**

### `moodle schema`

Print a response kind's JSON Schema, or a command's contract with its safety

- Synopsis: `moodle schema [kind | command...] [flags]`
- Data effect: **read-only**

### `moodle shell-completion`

Print a tab-completion script for your shell

- Synopsis: `moodle shell-completion <bash|fish|powershell|zsh> [flags]`
- Data effect: **read-only**

### `moodle site`

Manage the Moodle sites this tool knows about

- Synopsis: `moodle site [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle site academic`

Configure institution-specific workload fields

- Synopsis: `moodle site academic [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle site academic configure`

Map course custom fields to credits, academic level and term

- Synopsis: `moodle site academic configure [flags]`
- JSON response kind: `site.academic.configure`
- Data effect: **write**

| Flag | Meaning |
| --- | --- |
| `--credits-field` | course custom-field shortname containing credits |
| `--dry-run` | show the configuration without saving it |
| `--graduate-minimum` | minimum graduate credits per term |
| `--level-field` | course custom-field shortname containing undergraduate or graduate |
| `--site` | site to configure |
| `--term-field` | course custom-field shortname containing the academic term |
| `--undergraduate-minimum` | minimum undergraduate credits per term |
| `--yes` | save the configuration without another prompt |

### `moodle site add`

Register a Moodle site

- Synopsis: `moodle site add <name> <url> [flags]`
- JSON response kind: `site.add`
- Data effect: **read-only**

### `moodle site inspect`

Show what this account can do on this site

- Synopsis: `moodle site inspect [flags]`
- JSON response kind: `site.inspect`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to inspect |
| `--functions` | include the full function list |
| `--site` | site to inspect |

### `moodle site list`

List the configured sites

- Synopsis: `moodle site list [flags]`
- JSON response kind: `site.list`
- Data effect: **read-only**

### `moodle site remove`

Forget a site and delete its stored credentials

- Synopsis: `moodle site remove <name> [flags]`
- JSON response kind: `site.remove`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--yes` | remove without confirmation |

### `moodle site use`

Make a site the default for later commands

- Synopsis: `moodle site use <name> [flags]`
- JSON response kind: `site.use`
- Data effect: **read-only**

### `moodle version`

Print version and build information

- Synopsis: `moodle version [flags]`
- Data effect: **read-only**

### `moodle workload`

Calculate term credits from configured course custom fields

- Synopsis: `moodle workload [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle workload show`

Show courses, credits and the applicable minimum by term

- Synopsis: `moodle workload show [flags]`
- JSON response kind: `workload.show`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--site` | site to show workload for |
| `--term` | only this academic term |

### `moodle workload validate`

Validate each term against the configured credit minimum

- Synopsis: `moodle workload validate [flags]`
- JSON response kind: `workload.validate`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--require-minimum` | exit with validation code 7 when any term is incomplete or below its minimum |
| `--site` | site to validate workload for |
| `--term` | only this academic term |

### `moodle ws`

Inspect and call typed Moodle core web services

- Synopsis: `moodle ws [command]`
- Type: command group; select a subcommand
- Data effect: **read-only**

### `moodle ws call`

Validate and call one registered core function

- Synopsis: `moodle ws call <function> [flags]`
- JSON response kind: `ws.call`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--account` | account to act as |
| `--allow-write` | permit a registered function whose effect is write |
| `--dry-run` | validate and show the request without sending it |
| `--param` | a parameter as name=value, repeatable |
| `--params-json` | parameters as a JSON object, including nested structures |
| `--site` | site to call |

### `moodle ws describe`

Show versioned parameters, returns, effects and requirements

- Synopsis: `moodle ws describe <function> [flags]`
- JSON response kind: `ws.describe`
- Data effect: **read-only**

### `moodle ws list`

List the core function union for Moodle 4.5, 5.1 and 5.2

- Synopsis: `moodle ws list [flags]`
- JSON response kind: `ws.list`
- Data effect: **read-only**

| Flag | Meaning |
| --- | --- |
| `--component` | only this Moodle component |
| `--credential` | only functions that issue or change credentials |
| `--deprecated` | only functions deprecated in at least one version |
| `--destructive` | only destructive writes |
| `--effect` | only read or write functions |
| `--match` | only names containing this text |
| `--version` | only functions installed by v45, v51 or v52 |
