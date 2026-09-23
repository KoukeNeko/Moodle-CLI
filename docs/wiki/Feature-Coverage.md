# Feature coverage

[English](Feature-Coverage) · [繁體中文](Feature-Coverage-zh-TW)

This is the union of core `external_functions` from Moodle 4.5.12, 5.1.7, and 5.2.3. Use `moodle ws describe <function>` for versioned parameter/return schemas and `moodle ws call` to invoke it.

**Union: 780 core functions**

Registry presence does not mean every role may execute the function; Moodle remains the runtime authority.

| Function | Component | Effect | Versions | Transport | Deprecated |
| --- | --- | --- | --- | --- | --- |
| `aiplacement_courseassist_explain_text` | `aiplacement_courseassist` | write | v51, v52 | AJAX, REST | no |
| `aiplacement_courseassist_summarise_text` | `aiplacement_courseassist` | write | v45, v51, v52 | AJAX, REST | no |
| `aiplacement_editor_generate_image` | `aiplacement_editor` | write | v45, v51, v52 | AJAX, REST | no |
| `aiplacement_editor_generate_text` | `aiplacement_editor` | write | v45, v51, v52 | AJAX, REST | no |
| `auth_email_get_signup_settings` | `auth_email` | read | v45, v51, v52 | AJAX, REST | no |
| `auth_email_signup_user` | `auth_email` | write | v45, v51, v52 | AJAX, REST | no |
| `block_accessreview_get_module_data` | `block_accessreview` | read | v45, v51, v52 | AJAX, REST | no |
| `block_accessreview_get_section_data` | `block_accessreview` | read | v45, v51, v52 | AJAX, REST | no |
| `block_recentlyaccesseditems_get_recent_items` | `block_recentlyaccesseditems` | read | v45, v51, v52 | AJAX, REST | no |
| `block_starredcourses_get_starred_courses` | `block_starredcourses` | read | v45, v51, v52 | AJAX, REST | no |
| `core_admin_set_block_protection` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_admin_set_plugin_order` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_admin_set_plugin_state` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_ai_delete_provider_instance` | `moodle` | write | v51, v52 | AJAX, REST | no |
| `core_ai_get_policy_status` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_ai_set_action` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_ai_set_policy_status` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_ai_set_provider_order` | `moodle` | write | v51, v52 | AJAX, REST | no |
| `core_ai_set_provider_status` | `moodle` | write | v51, v52 | AJAX, REST | no |
| `core_auth_confirm_user` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_auth_is_age_digital_consent_verification_enabled` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_auth_is_minor` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_auth_request_password_reset` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_auth_resend_confirmation_email` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_backup_get_async_backup_links_backup` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_backup_get_async_backup_links_restore` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_backup_get_async_backup_progress` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_backup_get_copy_progress` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_backup_submit_copy_form` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_badges_disable_badges` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_badges_enable_badges` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_badges_get_badge` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_badges_get_user_badge_by_hash` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_badges_get_user_badges` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_block_fetch_addable_blocks` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_block_get_course_blocks` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_block_get_dashboard_blocks` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_blog_add_entry` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_blog_delete_entry` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_blog_get_access_information` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_blog_get_entries` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_blog_prepare_entry_for_edition` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_blog_update_entry` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_blog_view_entries` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_calendar_create_calendar_events` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_calendar_delete_calendar_events` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_calendar_delete_subscription` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_calendar_get_action_events_by_course` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_calendar_get_action_events_by_courses` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_calendar_get_action_events_by_timesort` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_calendar_get_allowed_event_types` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_calendar_get_calendar_access_information` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_calendar_get_calendar_day_view` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_calendar_get_calendar_event_by_id` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_calendar_get_calendar_events` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_calendar_get_calendar_export_token` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_calendar_get_calendar_monthly_view` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_calendar_get_calendar_upcoming_view` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_calendar_get_timestamps` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_calendar_submit_create_update_form` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_calendar_update_event_start_day` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_change_editmode` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_check_get_result_admintree` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_cohort_add_cohort_members` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_cohort_create_cohorts` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_cohort_delete_cohort_members` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_cohort_delete_cohorts` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_cohort_get_cohort_members` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_cohort_get_cohorts` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_cohort_search_cohorts` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_cohort_update_cohorts` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_comment_add_comments` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_comment_delete_comments` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_comment_get_comments` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_competency_add_competency_to_course` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_add_competency_to_plan` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_add_competency_to_template` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_add_related_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_approve_plan` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_competency_framework_viewed` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_competency_viewed` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_complete_plan` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_count_competencies` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_count_competencies_in_course` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_count_competencies_in_template` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_count_competency_frameworks` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_count_course_module_competencies` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_count_courses_using_competency` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_count_templates` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_count_templates_using_competency` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_create_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_create_competency_framework` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_create_plan` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_create_template` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_create_user_evidence_competency` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_delete_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_delete_competency_framework` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_delete_evidence` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_delete_plan` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_delete_template` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_delete_user_evidence` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_delete_user_evidence_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_duplicate_competency_framework` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_duplicate_template` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_get_scale_values` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_grade_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_grade_competency_in_course` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_grade_competency_in_plan` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_list_competencies` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_list_competencies_in_template` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_list_competency_frameworks` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_list_course_competencies` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_list_course_module_competencies` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_list_plan_competencies` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_list_templates` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_list_templates_using_competency` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_list_user_plans` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_move_down_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_move_up_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_plan_cancel_review_request` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_plan_request_review` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_plan_start_review` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_plan_stop_review` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_read_competency` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_read_competency_framework` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_read_plan` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_read_template` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_read_user_evidence` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_remove_competency_from_course` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_remove_competency_from_plan` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_remove_competency_from_template` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_remove_related_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_reopen_plan` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_reorder_course_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_reorder_plan_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_reorder_template_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_request_review_of_user_evidence_linked_competencies` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_search_competencies` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_set_course_competency_ruleoutcome` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_set_parent_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_template_has_related_data` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_template_viewed` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_unapprove_plan` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_unlink_plan_from_template` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_update_competency` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_update_competency_framework` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_update_course_competency_settings` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_update_plan` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_update_template` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_user_competency_cancel_review_request` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_user_competency_plan_viewed` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_user_competency_request_review` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_user_competency_start_review` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_user_competency_stop_review` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_competency_user_competency_viewed` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_user_competency_viewed_in_course` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_competency_user_competency_viewed_in_plan` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_completion_get_activities_completion_status` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_completion_get_course_completion_status` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_completion_mark_course_self_completed` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_completion_override_activity_completion_status` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_completion_update_activity_completion_status_manually` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_contentbank_copy_content` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_contentbank_delete_content` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_contentbank_rename_content` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_contentbank_set_content_visibility` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_course_add_content_item_to_user_favourites` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_course_check_updates` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_course_create_categories` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_course_create_courses` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_course_delete_categories` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_course_delete_courses` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_course_delete_modules` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_course_duplicate_course` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_course_edit_module` | `moodle` | write | v45, v51, v52 | AJAX, REST | yes |
| `core_course_edit_section` | `moodle` | write | v45, v51, v52 | AJAX, REST | yes |
| `core_course_get_activity_chooser_footer` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_course_get_categories` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_course_get_contents` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_course_get_course_content_items` | `moodle` | read | v45, v51, v52 | AJAX, REST | yes |
| `core_course_get_course_module` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_course_get_course_module_by_instance` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_course_get_courses` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_course_get_courses_by_field` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_course_get_enrolled_courses_by_timeline_classification` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_course_get_enrolled_courses_with_action_events_by_timeline_classification` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_course_get_enrolled_users_by_cmid` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_course_get_module` | `moodle` | read | v45, v51, v52 | AJAX, REST | yes |
| `core_course_get_recent_courses` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_course_get_updates_since` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_course_get_user_administration_options` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_course_get_user_navigation_options` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_course_import_course` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_course_remove_content_item_from_user_favourites` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_course_search_courses` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_course_set_favourite_courses` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_course_toggle_activity_recommendation` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_course_update_categories` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_course_update_courses` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_course_view_course` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_course_view_module_instance_list` | `moodle` | write | v51, v52 | REST | no |
| `core_courseformat_create_module` | `moodle` | write | v45, v51, v52 | AJAX, REST | yes |
| `core_courseformat_file_handlers` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_courseformat_get_overview_information` | `moodle` | read | v51, v52 | AJAX, REST | no |
| `core_courseformat_get_section_content_items` | `moodle` | read | v51, v52 | AJAX, REST | no |
| `core_courseformat_get_state` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_courseformat_log_view_overview_information` | `moodle` | write | v51, v52 | REST | no |
| `core_courseformat_new_module` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_courseformat_update_course` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_create_userfeedback_action_record` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_customfield_convert_category` | `moodle` | write | v52 | AJAX, REST | no |
| `core_customfield_create_category` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_customfield_delete_category` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_customfield_delete_field` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_customfield_move_category` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_customfield_move_field` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_customfield_reload_template` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_customfield_toggle_shared` | `moodle` | write | v51, v52 | AJAX, REST | no |
| `core_dynamic_tabs_get_content` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_enrol_get_course_enrolment_methods` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_enrol_get_enrolled_users` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_enrol_get_enrolled_users_with_capability` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_enrol_get_potential_users` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_enrol_get_users_courses` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_enrol_search_users` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_enrol_submit_user_enrolment_form` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_enrol_unenrol_user_enrolment` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_fetch_notifications` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_files_delete_draft_files` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_files_get_files` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_files_get_unused_draft_itemid` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_files_upload` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_filters_get_all_states` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_filters_get_available_in_context` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_form_dynamic_form` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_form_get_filetypes_browser_data` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_get_component_strings` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_get_fragment` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_get_string` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_get_strings` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_get_user_dates` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_grades_create_gradecategories` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_grades_get_enrolled_users_for_search_widget` | `moodle` | read | v45 | AJAX, REST | yes |
| `core_grades_get_enrolled_users_for_selector` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_grades_get_feedback` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_grades_get_gradable_users` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_grades_get_grade_tree` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_grades_get_gradeitems` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_grades_get_groups_for_search_widget` | `moodle` | read | v45 | AJAX, REST | no |
| `core_grades_get_groups_for_selector` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_grades_grader_gradingpanel_point_fetch` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_grades_grader_gradingpanel_point_store` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_grades_grader_gradingpanel_scale_fetch` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_grades_grader_gradingpanel_scale_store` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_grades_update_grades` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_grading_get_definitions` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_grading_get_gradingform_instances` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_grading_save_definitions` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_group_add_group_members` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_group_assign_grouping` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_group_create_groupings` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_group_create_groups` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_group_delete_group_members` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_group_delete_groupings` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_group_delete_groups` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_group_get_activity_allowed_groups` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_group_get_activity_groupmode` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_group_get_course_groupings` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_group_get_course_groups` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_group_get_course_user_groups` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_group_get_group_members` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_group_get_groupings` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_group_get_groups` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_group_get_groups_for_selector` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_group_unassign_grouping` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_group_update_groupings` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_group_update_groups` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_h5p_get_trusted_h5p_file` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_block_user` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_confirm_contact_request` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_create_contact_request` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_data_for_messagearea_search_messages` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_decline_contact_request` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_delete_contacts` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_delete_conversations_by_id` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_delete_message` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_delete_message_for_all_users` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_blocked_users` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_message_get_contact_requests` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_conversation` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_conversation_between_users` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_conversation_counts` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_conversation_members` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_conversation_messages` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_conversations` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_member_info` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_message_processor` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_messages` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_received_contact_requests_count` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_self_conversation` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_unread_conversation_counts` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_unread_conversations_count` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_unread_notification_count` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_message_get_unsent_message` | `moodle` | read | v51, v52 | AJAX, REST | no |
| `core_message_get_user_contacts` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_user_message_preferences` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_get_user_notification_preferences` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_message_mark_all_conversation_messages_as_read` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_mark_all_notifications_as_read` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_mark_message_read` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_mark_notification_read` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_message_processor_config_form` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_message_search_users` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_message_mute_conversations` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_search_contacts` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_message_send_instant_messages` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_send_messages_to_conversation` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_set_default_notification` | `moodle` | write | v51, v52 | AJAX, REST | no |
| `core_message_set_favourite_conversations` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_set_unsent_message` | `moodle` | write | v51, v52 | AJAX, REST | no |
| `core_message_unblock_user` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_unmute_conversations` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_message_unset_favourite_conversations` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_moodlenet_auth_check` | `moodle` | write | v45, v51 | AJAX, REST | no |
| `core_moodlenet_get_share_info_activity` | `moodle` | read | v45, v51 | AJAX, REST | no |
| `core_moodlenet_get_shared_course_info` | `moodle` | read | v45, v51 | AJAX, REST | no |
| `core_moodlenet_send_activity` | `moodle` | read | v45, v51 | AJAX, REST | no |
| `core_moodlenet_send_course` | `moodle` | read | v45, v51 | AJAX, REST | no |
| `core_my_view_page` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_notes_create_notes` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_notes_delete_notes` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_notes_get_course_notes` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_notes_get_notes` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_notes_update_notes` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_notes_view_notes` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_output_load_fontawesome_icon_map` | `moodle` | read | v45 | AJAX, REST | yes |
| `core_output_load_fontawesome_icon_system_map` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_output_load_template` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_output_load_template_with_dependencies` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_output_poll_stored_progress` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_payment_get_available_gateways` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_question_get_random_question_summaries` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_question_move_questions` | `moodle` | write | v51, v52 | AJAX, REST | no |
| `core_question_search_shared_banks` | `moodle` | read | v51, v52 | AJAX, REST | no |
| `core_question_update_flag` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_rating_add_rating` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_rating_get_item_ratings` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_reportbuilder_audiences_delete` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_can_view_system_report` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_reportbuilder_columns_add` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_columns_delete` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_columns_reorder` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_columns_sort_get` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_columns_sort_reorder` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_columns_sort_toggle` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_conditions_add` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_conditions_delete` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_conditions_reorder` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_conditions_reset` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_filters_add` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_filters_delete` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_filters_reorder` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_filters_reset` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_list_reports` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_reportbuilder_reports_delete` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_reports_get` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_retrieve_report` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_reportbuilder_retrieve_system_report` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_reportbuilder_schedules_delete` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_schedules_send` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_schedules_toggle` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_set_filters` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_reportbuilder_view_report` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_role_assign_roles` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_role_unassign_roles` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_search_get_relevant_users` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_search_get_results` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_search_get_search_areas_list` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_search_get_top_results` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_search_view_results` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_session_time_remaining` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_session_touch` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_sms_set_gateway_status` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_table_get_dynamic_table_content` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_tag_get_tag_areas` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_tag_get_tag_cloud` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_tag_get_tag_collections` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_tag_get_tagindex` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_tag_get_tagindex_per_area` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_tag_get_tags` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_tag_update_tags` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_update_inplace_editable` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_user_add_user_device` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_user_add_user_private_files` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_user_agree_site_policy` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_user_create_users` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_user_delete_users` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_user_get_course_user_profiles` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_user_get_private_files_info` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_user_get_user_preferences` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_user_get_users` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_user_get_users_by_field` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_user_prepare_private_files_for_edition` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_user_remove_user_device` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_user_search_identity` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_user_set_user_preferences` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_user_update_picture` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_user_update_private_files` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_user_update_user_device_public_key` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_user_update_user_preferences` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_user_update_users` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_user_view_user_list` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_user_view_user_profile` | `moodle` | write | v45, v51, v52 | REST | no |
| `core_webservice_get_site_info` | `moodle` | read | v45, v51, v52 | REST | no |
| `core_xapi_delete_state` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_xapi_delete_states` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_xapi_get_state` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_xapi_get_states` | `moodle` | read | v45, v51, v52 | AJAX, REST | no |
| `core_xapi_post_state` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `core_xapi_statement_post` | `moodle` | write | v45, v51, v52 | AJAX, REST | no |
| `customfield_number_recalculate_value` | `customfield_number` | write | v45, v51, v52 | AJAX, REST | no |
| `editor_tiny_get_configuration` | `editor_tiny` | read | v51, v52 | REST | no |
| `enrol_guest_get_instance_info` | `enrol_guest` | read | v45, v51, v52 | REST | no |
| `enrol_guest_validate_password` | `enrol_guest` | write | v45, v51, v52 | REST | no |
| `enrol_manual_enrol_users` | `enrol_manual` | write | v45, v51, v52 | REST | no |
| `enrol_manual_unenrol_users` | `enrol_manual` | write | v45, v51, v52 | REST | no |
| `enrol_meta_add_instances` | `enrol_meta` | write | v45, v51, v52 | AJAX, REST | no |
| `enrol_meta_delete_instances` | `enrol_meta` | write | v45, v51, v52 | AJAX, REST | no |
| `enrol_self_enrol_user` | `enrol_self` | write | v45, v51, v52 | REST | no |
| `enrol_self_get_instance_info` | `enrol_self` | read | v45, v51, v52 | REST | no |
| `gradereport_grader_get_users_in_report` | `gradereport_grader` | read | v45, v51, v52 | AJAX, REST | no |
| `gradereport_overview_get_course_grades` | `gradereport_overview` | read | v45, v51, v52 | REST | no |
| `gradereport_overview_view_grade_report` | `gradereport_overview` | write | v45, v51, v52 | REST | no |
| `gradereport_singleview_get_grade_items_for_search_widget` | `gradereport_singleview` | read | v45, v51, v52 | AJAX, REST | no |
| `gradereport_user_get_access_information` | `gradereport_user` | read | v45, v51, v52 | REST | no |
| `gradereport_user_get_grade_items` | `gradereport_user` | read | v45, v51, v52 | REST | no |
| `gradereport_user_get_grades_table` | `gradereport_user` | read | v45, v51, v52 | REST | no |
| `gradereport_user_view_grade_report` | `gradereport_user` | write | v45, v51, v52 | REST | no |
| `gradingform_guide_grader_gradingpanel_fetch` | `gradingform_guide` | write | v45, v51, v52 | AJAX, REST | no |
| `gradingform_guide_grader_gradingpanel_store` | `gradingform_guide` | write | v45, v51, v52 | AJAX, REST | no |
| `gradingform_rubric_grader_gradingpanel_fetch` | `gradingform_rubric` | write | v45, v51, v52 | AJAX, REST | no |
| `gradingform_rubric_grader_gradingpanel_store` | `gradingform_rubric` | write | v45, v51, v52 | AJAX, REST | no |
| `media_videojs_get_language` | `media_videojs` | read | v45, v51, v52 | AJAX, REST | no |
| `message_airnotifier_are_notification_preferences_configured` | `message_airnotifier` | read | v45, v51, v52 | REST | no |
| `message_airnotifier_enable_device` | `message_airnotifier` | write | v45, v51, v52 | REST | no |
| `message_airnotifier_get_user_devices` | `message_airnotifier` | read | v45, v51, v52 | REST | no |
| `message_airnotifier_is_system_configured` | `message_airnotifier` | read | v45, v51, v52 | REST | no |
| `message_popup_get_popup_notifications` | `message_popup` | read | v45, v51, v52 | AJAX, REST | no |
| `message_popup_get_unread_popup_notification_count` | `message_popup` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_assign_copy_previous_attempt` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_get_assignments` | `mod_assign` | read | v45, v51, v52 | REST | no |
| `mod_assign_get_grades` | `mod_assign` | read | v45, v51, v52 | REST | no |
| `mod_assign_get_participant` | `mod_assign` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_assign_get_submission_status` | `mod_assign` | read | v45, v51, v52 | REST | no |
| `mod_assign_get_submissions` | `mod_assign` | read | v45, v51, v52 | REST | no |
| `mod_assign_get_user_flags` | `mod_assign` | read | v45, v51, v52 | REST | no |
| `mod_assign_get_user_mappings` | `mod_assign` | read | v45, v51, v52 | REST | no |
| `mod_assign_list_participants` | `mod_assign` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_assign_lock_submissions` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_remove_submission` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_reveal_identities` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_revert_submissions_to_draft` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_save_grade` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_save_grades` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_save_submission` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_save_user_extensions` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_set_user_flags` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_start_submission` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_submit_for_grading` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_submit_grading_form` | `mod_assign` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_assign_unlock_submissions` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_view_assign` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_view_grading_table` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_assign_view_submission_status` | `mod_assign` | write | v45, v51, v52 | REST | no |
| `mod_bigbluebuttonbn_can_join` | `mod_bigbluebuttonbn` | read | v45, v51, v52 | REST | no |
| `mod_bigbluebuttonbn_completion_validate` | `mod_bigbluebuttonbn` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_bigbluebuttonbn_end_meeting` | `mod_bigbluebuttonbn` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_bigbluebuttonbn_get_bigbluebuttonbns_by_courses` | `mod_bigbluebuttonbn` | read | v45, v51, v52 | REST | no |
| `mod_bigbluebuttonbn_get_join_url` | `mod_bigbluebuttonbn` | write | v45, v51, v52 | REST | no |
| `mod_bigbluebuttonbn_get_recordings` | `mod_bigbluebuttonbn` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_bigbluebuttonbn_get_recordings_to_import` | `mod_bigbluebuttonbn` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_bigbluebuttonbn_meeting_info` | `mod_bigbluebuttonbn` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_bigbluebuttonbn_update_recording` | `mod_bigbluebuttonbn` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_bigbluebuttonbn_view_bigbluebuttonbn` | `mod_bigbluebuttonbn` | write | v45, v51, v52 | REST | no |
| `mod_book_get_books_by_courses` | `mod_book` | read | v45, v51, v52 | REST | no |
| `mod_book_view_book` | `mod_book` | write | v45, v51, v52 | REST | no |
| `mod_chat_get_chat_latest_messages` | `mod_chat` | read | v45 | REST | no |
| `mod_chat_get_chat_users` | `mod_chat` | read | v45 | REST | no |
| `mod_chat_get_chats_by_courses` | `mod_chat` | read | v45 | REST | no |
| `mod_chat_get_session_messages` | `mod_chat` | read | v45 | REST | no |
| `mod_chat_get_sessions` | `mod_chat` | read | v45 | REST | no |
| `mod_chat_login_user` | `mod_chat` | write | v45 | REST | no |
| `mod_chat_send_chat_message` | `mod_chat` | write | v45 | REST | no |
| `mod_chat_view_chat` | `mod_chat` | write | v45 | REST | no |
| `mod_chat_view_sessions` | `mod_chat` | write | v45 | REST | no |
| `mod_choice_delete_choice_responses` | `mod_choice` | write | v45, v51, v52 | REST | no |
| `mod_choice_get_choice_options` | `mod_choice` | read | v45, v51, v52 | REST | no |
| `mod_choice_get_choice_results` | `mod_choice` | read | v45, v51, v52 | REST | no |
| `mod_choice_get_choices_by_courses` | `mod_choice` | read | v45, v51, v52 | REST | no |
| `mod_choice_submit_choice_response` | `mod_choice` | write | v45, v51, v52 | REST | no |
| `mod_choice_view_choice` | `mod_choice` | write | v45, v51, v52 | REST | no |
| `mod_data_add_entry` | `mod_data` | write | v45, v51, v52 | REST | no |
| `mod_data_approve_entry` | `mod_data` | write | v45, v51, v52 | REST | no |
| `mod_data_delete_entry` | `mod_data` | write | v45, v51, v52 | REST | no |
| `mod_data_delete_saved_preset` | `mod_data` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_data_get_data_access_information` | `mod_data` | read | v45, v51, v52 | REST | no |
| `mod_data_get_databases_by_courses` | `mod_data` | read | v45, v51, v52 | REST | no |
| `mod_data_get_entries` | `mod_data` | read | v45, v51, v52 | REST | no |
| `mod_data_get_entry` | `mod_data` | read | v45, v51, v52 | REST | no |
| `mod_data_get_fields` | `mod_data` | read | v45, v51, v52 | REST | no |
| `mod_data_get_mapping_information` | `mod_data` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_data_search_entries` | `mod_data` | read | v45, v51, v52 | REST | no |
| `mod_data_update_entry` | `mod_data` | write | v45, v51, v52 | REST | no |
| `mod_data_view_database` | `mod_data` | write | v45, v51, v52 | REST | no |
| `mod_feedback_get_analysis` | `mod_feedback` | read | v45, v51, v52 | REST | no |
| `mod_feedback_get_current_completed_tmp` | `mod_feedback` | read | v45, v51, v52 | REST | no |
| `mod_feedback_get_feedback_access_information` | `mod_feedback` | read | v45, v51, v52 | REST | no |
| `mod_feedback_get_feedbacks_by_courses` | `mod_feedback` | read | v45, v51, v52 | REST | no |
| `mod_feedback_get_finished_responses` | `mod_feedback` | read | v45, v51, v52 | REST | no |
| `mod_feedback_get_items` | `mod_feedback` | read | v45, v51, v52 | REST | no |
| `mod_feedback_get_last_completed` | `mod_feedback` | read | v45, v51, v52 | REST | no |
| `mod_feedback_get_non_respondents` | `mod_feedback` | read | v45, v51, v52 | REST | no |
| `mod_feedback_get_page_items` | `mod_feedback` | read | v45, v51, v52 | REST | no |
| `mod_feedback_get_responses_analysis` | `mod_feedback` | read | v45, v51, v52 | REST | no |
| `mod_feedback_get_unfinished_responses` | `mod_feedback` | read | v45, v51, v52 | REST | no |
| `mod_feedback_launch_feedback` | `mod_feedback` | write | v45, v51, v52 | REST | no |
| `mod_feedback_process_page` | `mod_feedback` | write | v45, v51, v52 | REST | no |
| `mod_feedback_questions_reorder` | `mod_feedback` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_feedback_view_feedback` | `mod_feedback` | write | v45, v51, v52 | REST | no |
| `mod_folder_get_folders_by_courses` | `mod_folder` | read | v45, v51, v52 | REST | no |
| `mod_folder_view_folder` | `mod_folder` | write | v45, v51, v52 | REST | no |
| `mod_forum_add_discussion` | `mod_forum` | write | v45, v51, v52 | REST | no |
| `mod_forum_add_discussion_post` | `mod_forum` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_forum_can_add_discussion` | `mod_forum` | read | v45, v51, v52 | REST | no |
| `mod_forum_delete_post` | `mod_forum` | write | v45, v51, v52 | REST | no |
| `mod_forum_get_discussion_post` | `mod_forum` | read | v45, v51, v52 | REST | no |
| `mod_forum_get_discussion_posts` | `mod_forum` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_forum_get_discussion_posts_by_userid` | `mod_forum` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_forum_get_forum_access_information` | `mod_forum` | read | v45, v51, v52 | REST | no |
| `mod_forum_get_forum_discussions` | `mod_forum` | read | v45, v51, v52 | REST | no |
| `mod_forum_get_forums_by_courses` | `mod_forum` | read | v45, v51, v52 | REST | no |
| `mod_forum_mark_posts_read` | `mod_forum` | write | v51, v52 | AJAX, REST | no |
| `mod_forum_prepare_draft_area_for_post` | `mod_forum` | write | v45, v51, v52 | REST | no |
| `mod_forum_set_forum_subscription` | `mod_forum` | write | v51, v52 | AJAX, REST | no |
| `mod_forum_set_forum_tracking` | `mod_forum` | write | v51, v52 | AJAX, REST | no |
| `mod_forum_set_lock_state` | `mod_forum` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_forum_set_pin_state` | `mod_forum` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_forum_set_subscription_state` | `mod_forum` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_forum_toggle_favourite_state` | `mod_forum` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_forum_update_discussion_post` | `mod_forum` | write | v45, v51, v52 | REST | no |
| `mod_forum_view_forum` | `mod_forum` | write | v45, v51, v52 | REST | no |
| `mod_forum_view_forum_discussion` | `mod_forum` | write | v45, v51, v52 | REST | no |
| `mod_glossary_add_entry` | `mod_glossary` | write | v45, v51, v52 | REST | no |
| `mod_glossary_delete_entry` | `mod_glossary` | write | v45, v51, v52 | REST | no |
| `mod_glossary_get_authors` | `mod_glossary` | read | v45, v51, v52 | REST | no |
| `mod_glossary_get_categories` | `mod_glossary` | read | v45, v51, v52 | REST | no |
| `mod_glossary_get_entries_by_author` | `mod_glossary` | read | v45, v51, v52 | REST | no |
| `mod_glossary_get_entries_by_author_id` | `mod_glossary` | read | v45, v51, v52 | REST | no |
| `mod_glossary_get_entries_by_category` | `mod_glossary` | read | v45, v51, v52 | REST | no |
| `mod_glossary_get_entries_by_date` | `mod_glossary` | read | v45, v51, v52 | REST | no |
| `mod_glossary_get_entries_by_letter` | `mod_glossary` | read | v45, v51, v52 | REST | no |
| `mod_glossary_get_entries_by_search` | `mod_glossary` | read | v45, v51, v52 | REST | no |
| `mod_glossary_get_entries_by_term` | `mod_glossary` | read | v45, v51, v52 | REST | no |
| `mod_glossary_get_entries_to_approve` | `mod_glossary` | read | v45, v51, v52 | REST | no |
| `mod_glossary_get_entry_by_id` | `mod_glossary` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_glossary_get_glossaries_by_courses` | `mod_glossary` | read | v45, v51, v52 | REST | no |
| `mod_glossary_prepare_entry_for_edition` | `mod_glossary` | read | v45, v51, v52 | REST | no |
| `mod_glossary_update_entry` | `mod_glossary` | write | v45, v51, v52 | REST | no |
| `mod_glossary_view_entry` | `mod_glossary` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_glossary_view_glossary` | `mod_glossary` | write | v45, v51, v52 | REST | no |
| `mod_h5pactivity_get_attempts` | `mod_h5pactivity` | read | v45, v51, v52 | REST | no |
| `mod_h5pactivity_get_h5pactivities_by_courses` | `mod_h5pactivity` | read | v45, v51, v52 | REST | no |
| `mod_h5pactivity_get_h5pactivity_access_information` | `mod_h5pactivity` | read | v45, v51, v52 | REST | no |
| `mod_h5pactivity_get_results` | `mod_h5pactivity` | read | v45, v51, v52 | REST | no |
| `mod_h5pactivity_get_user_attempts` | `mod_h5pactivity` | read | v45, v51, v52 | REST | no |
| `mod_h5pactivity_log_report_viewed` | `mod_h5pactivity` | write | v45, v51, v52 | REST | no |
| `mod_h5pactivity_view_h5pactivity` | `mod_h5pactivity` | write | v45, v51, v52 | REST | no |
| `mod_imscp_get_imscps_by_courses` | `mod_imscp` | read | v45, v51, v52 | REST | no |
| `mod_imscp_view_imscp` | `mod_imscp` | write | v45, v51, v52 | REST | no |
| `mod_label_get_labels_by_courses` | `mod_label` | read | v45, v51, v52 | REST | no |
| `mod_lesson_finish_attempt` | `mod_lesson` | write | v45, v51, v52 | REST | no |
| `mod_lesson_get_attempts_overview` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_get_content_pages_viewed` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_get_lesson` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_get_lesson_access_information` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_get_lessons_by_courses` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_get_page_data` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_get_pages` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_get_pages_possible_jumps` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_get_questions_attempts` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_get_user_attempt` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_get_user_attempt_grade` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_get_user_grade` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_get_user_timers` | `mod_lesson` | read | v45, v51, v52 | REST | no |
| `mod_lesson_launch_attempt` | `mod_lesson` | write | v45, v51, v52 | REST | no |
| `mod_lesson_process_page` | `mod_lesson` | write | v45, v51, v52 | REST | no |
| `mod_lesson_view_lesson` | `mod_lesson` | write | v45, v51, v52 | REST | no |
| `mod_lti_create_tool_proxy` | `mod_lti` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_create_tool_type` | `mod_lti` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_delete_course_tool_type` | `mod_lti` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_delete_tool_proxy` | `mod_lti` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_delete_tool_type` | `mod_lti` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_get_ltis_by_courses` | `mod_lti` | read | v45, v51, v52 | REST | no |
| `mod_lti_get_tool_launch_data` | `mod_lti` | read | v45, v51, v52 | REST | no |
| `mod_lti_get_tool_proxies` | `mod_lti` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_get_tool_proxy_registration_request` | `mod_lti` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_get_tool_types` | `mod_lti` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_get_tool_types_and_proxies` | `mod_lti` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_get_tool_types_and_proxies_count` | `mod_lti` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_is_cartridge` | `mod_lti` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_toggle_showinactivitychooser` | `mod_lti` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_update_tool_type` | `mod_lti` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_lti_view_lti` | `mod_lti` | read | v45, v51, v52 | REST | no |
| `mod_page_get_pages_by_courses` | `mod_page` | read | v45, v51, v52 | REST | no |
| `mod_page_view_page` | `mod_page` | write | v45, v51, v52 | REST | no |
| `mod_quiz_add_random_questions` | `mod_quiz` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_create_grade_item_per_section` | `mod_quiz` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_create_grade_items` | `mod_quiz` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_delete_grade_items` | `mod_quiz` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_delete_overrides` | `mod_quiz` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_get_attempt_access_information` | `mod_quiz` | read | v45, v51, v52 | REST | no |
| `mod_quiz_get_attempt_data` | `mod_quiz` | read | v45, v51, v52 | REST | no |
| `mod_quiz_get_attempt_review` | `mod_quiz` | read | v45, v51, v52 | REST | no |
| `mod_quiz_get_attempt_summary` | `mod_quiz` | read | v45, v51, v52 | REST | no |
| `mod_quiz_get_combined_review_options` | `mod_quiz` | read | v45, v51, v52 | REST | no |
| `mod_quiz_get_edit_grading_page_data` | `mod_quiz` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_get_overrides` | `mod_quiz` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_get_quiz_access_information` | `mod_quiz` | read | v45, v51, v52 | REST | no |
| `mod_quiz_get_quiz_feedback_for_grade` | `mod_quiz` | read | v45, v51, v52 | REST | no |
| `mod_quiz_get_quiz_required_qtypes` | `mod_quiz` | read | v45, v51, v52 | REST | no |
| `mod_quiz_get_quizzes_by_courses` | `mod_quiz` | read | v45, v51, v52 | REST | no |
| `mod_quiz_get_reopen_attempt_confirmation` | `mod_quiz` | read | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_get_user_attempts` | `mod_quiz` | read | v45, v51, v52 | REST | yes |
| `mod_quiz_get_user_best_grade` | `mod_quiz` | read | v45, v51, v52 | REST | no |
| `mod_quiz_get_user_quiz_attempts` | `mod_quiz` | read | v51, v52 | REST | no |
| `mod_quiz_process_attempt` | `mod_quiz` | write | v45, v51, v52 | REST | no |
| `mod_quiz_reopen_attempt` | `mod_quiz` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_save_attempt` | `mod_quiz` | write | v45, v51, v52 | REST | no |
| `mod_quiz_save_overrides` | `mod_quiz` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_set_question_version` | `mod_quiz` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_start_attempt` | `mod_quiz` | write | v45, v51, v52 | REST | no |
| `mod_quiz_update_filter_condition` | `mod_quiz` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_update_grade_items` | `mod_quiz` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_update_slots` | `mod_quiz` | write | v45, v51, v52 | AJAX, REST | no |
| `mod_quiz_view_attempt` | `mod_quiz` | write | v45, v51, v52 | REST | no |
| `mod_quiz_view_attempt_review` | `mod_quiz` | write | v45, v51, v52 | REST | no |
| `mod_quiz_view_attempt_summary` | `mod_quiz` | write | v45, v51, v52 | REST | no |
| `mod_quiz_view_quiz` | `mod_quiz` | write | v45, v51, v52 | REST | no |
| `mod_resource_get_resources_by_courses` | `mod_resource` | read | v45, v51, v52 | REST | no |
| `mod_resource_view_resource` | `mod_resource` | write | v45, v51, v52 | REST | no |
| `mod_scorm_get_scorm_access_information` | `mod_scorm` | read | v45, v51, v52 | REST | no |
| `mod_scorm_get_scorm_attempt_count` | `mod_scorm` | read | v45, v51, v52 | REST | no |
| `mod_scorm_get_scorm_sco_tracks` | `mod_scorm` | read | v45, v51, v52 | REST | no |
| `mod_scorm_get_scorm_scoes` | `mod_scorm` | read | v45, v51, v52 | REST | no |
| `mod_scorm_get_scorm_user_data` | `mod_scorm` | read | v45, v51, v52 | REST | no |
| `mod_scorm_get_scorms_by_courses` | `mod_scorm` | read | v45, v51, v52 | REST | no |
| `mod_scorm_insert_scorm_tracks` | `mod_scorm` | write | v45, v51, v52 | REST | no |
| `mod_scorm_launch_sco` | `mod_scorm` | write | v45, v51, v52 | REST | no |
| `mod_scorm_view_scorm` | `mod_scorm` | write | v45, v51, v52 | REST | no |
| `mod_survey_get_questions` | `mod_survey` | read | v45 | REST | no |
| `mod_survey_get_surveys_by_courses` | `mod_survey` | read | v45 | REST | no |
| `mod_survey_submit_answers` | `mod_survey` | write | v45 | REST | no |
| `mod_survey_view_survey` | `mod_survey` | write | v45 | REST | no |
| `mod_url_get_urls_by_courses` | `mod_url` | read | v45, v51, v52 | REST | no |
| `mod_url_view_url` | `mod_url` | write | v45, v51, v52 | REST | no |
| `mod_wiki_edit_page` | `mod_wiki` | write | v45, v51, v52 | REST | no |
| `mod_wiki_get_page_contents` | `mod_wiki` | read | v45, v51, v52 | REST | no |
| `mod_wiki_get_page_for_editing` | `mod_wiki` | write | v45, v51, v52 | REST | no |
| `mod_wiki_get_subwiki_files` | `mod_wiki` | read | v45, v51, v52 | REST | no |
| `mod_wiki_get_subwiki_pages` | `mod_wiki` | read | v45, v51, v52 | REST | no |
| `mod_wiki_get_subwikis` | `mod_wiki` | read | v45, v51, v52 | REST | no |
| `mod_wiki_get_wikis_by_courses` | `mod_wiki` | read | v45, v51, v52 | REST | no |
| `mod_wiki_new_page` | `mod_wiki` | write | v45, v51, v52 | REST | no |
| `mod_wiki_view_page` | `mod_wiki` | write | v45, v51, v52 | REST | no |
| `mod_wiki_view_wiki` | `mod_wiki` | write | v45, v51, v52 | REST | no |
| `mod_workshop_add_submission` | `mod_workshop` | write | v45, v51, v52 | REST | no |
| `mod_workshop_delete_submission` | `mod_workshop` | write | v45, v51, v52 | REST | no |
| `mod_workshop_evaluate_assessment` | `mod_workshop` | write | v45, v51, v52 | REST | no |
| `mod_workshop_evaluate_submission` | `mod_workshop` | write | v45, v51, v52 | REST | no |
| `mod_workshop_get_assessment` | `mod_workshop` | read | v45, v51, v52 | REST | no |
| `mod_workshop_get_assessment_form_definition` | `mod_workshop` | read | v45, v51, v52 | REST | no |
| `mod_workshop_get_grades` | `mod_workshop` | read | v45, v51, v52 | REST | no |
| `mod_workshop_get_grades_report` | `mod_workshop` | read | v45, v51, v52 | REST | no |
| `mod_workshop_get_reviewer_assessments` | `mod_workshop` | read | v45, v51, v52 | REST | no |
| `mod_workshop_get_submission` | `mod_workshop` | read | v45, v51, v52 | REST | no |
| `mod_workshop_get_submission_assessments` | `mod_workshop` | read | v45, v51, v52 | REST | no |
| `mod_workshop_get_submissions` | `mod_workshop` | read | v45, v51, v52 | REST | no |
| `mod_workshop_get_user_plan` | `mod_workshop` | read | v45, v51, v52 | REST | no |
| `mod_workshop_get_workshop_access_information` | `mod_workshop` | read | v45, v51, v52 | REST | no |
| `mod_workshop_get_workshops_by_courses` | `mod_workshop` | read | v45, v51, v52 | REST | no |
| `mod_workshop_update_assessment` | `mod_workshop` | write | v45, v51, v52 | REST | no |
| `mod_workshop_update_submission` | `mod_workshop` | write | v45, v51, v52 | REST | no |
| `mod_workshop_view_submission` | `mod_workshop` | write | v45, v51, v52 | REST | no |
| `mod_workshop_view_workshop` | `mod_workshop` | write | v45, v51, v52 | REST | no |
| `paygw_paypal_create_transaction_complete` | `paygw_paypal` | write | v45, v51, v52 | AJAX, REST | no |
| `paygw_paypal_get_config_for_js` | `paygw_paypal` | read | v45, v51, v52 | AJAX, REST | no |
| `qbank_columnsortorder_set_column_size` | `qbank_columnsortorder` | write | v45, v51, v52 | AJAX, REST | no |
| `qbank_columnsortorder_set_columnbank_order` | `qbank_columnsortorder` | write | v45, v51, v52 | AJAX, REST | no |
| `qbank_columnsortorder_set_hidden_columns` | `qbank_columnsortorder` | write | v45, v51, v52 | AJAX, REST | no |
| `qbank_editquestion_set_status` | `qbank_editquestion` | write | v45, v51, v52 | AJAX, REST | no |
| `qbank_managecategories_move_category` | `qbank_managecategories` | write | v45, v51, v52 | AJAX, REST | no |
| `qbank_tagquestion_submit_tags_form` | `qbank_tagquestion` | write | v45, v51, v52 | AJAX, REST | no |
| `qbank_viewquestiontext_set_question_text_format` | `qbank_viewquestiontext` | write | v45, v51, v52 | AJAX, REST | no |
| `quizaccess_seb_validate_quiz_keys` | `quizaccess_seb` | read | v45, v51, v52 | AJAX, REST | no |
| `report_competency_data_for_report` | `report_competency` | read | v45, v51, v52 | AJAX, REST | no |
| `report_insights_action_executed` | `report_insights` | write | v45, v51, v52 | AJAX, REST | no |
| `report_insights_set_fixed_prediction` | `report_insights` | write | v45 | AJAX, REST | yes |
| `report_insights_set_notuseful_prediction` | `report_insights` | write | v45 | AJAX, REST | yes |
| `tiny_autosave_reset_session` | `tiny_autosave` | write | v45, v51, v52 | AJAX, REST | no |
| `tiny_autosave_resume_session` | `tiny_autosave` | write | v45, v51, v52 | AJAX, REST | no |
| `tiny_autosave_update_session` | `tiny_autosave` | write | v45, v51, v52 | AJAX, REST | no |
| `tiny_equation_filter` | `tiny_equation` | read | v45, v51, v52 | AJAX, REST | no |
| `tiny_media_preview` | `tiny_media` | read | v51, v52 | AJAX, REST | no |
| `tiny_premium_get_api_key` | `tiny_premium` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_admin_presets_delete_preset` | `tool_admin_presets` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_analytics_potential_contexts` | `tool_analytics` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_behat_get_entity_generator` | `tool_behat` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_approve_data_request` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_bulk_approve_data_requests` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_bulk_deny_data_requests` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_cancel_data_request` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_confirm_contexts_for_deletion` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_contact_dpo` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_create_category_form` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_create_data_request` | `tool_dataprivacy` | write | v45, v51, v52 | REST | no |
| `tool_dataprivacy_create_purpose_form` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_delete_category` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_delete_purpose` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_deny_data_request` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_get_access_information` | `tool_dataprivacy` | read | v45, v51, v52 | REST | no |
| `tool_dataprivacy_get_activity_options` | `tool_dataprivacy` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_get_category_options` | `tool_dataprivacy` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_get_data_request` | `tool_dataprivacy` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_get_data_requests` | `tool_dataprivacy` | read | v45, v51, v52 | REST | no |
| `tool_dataprivacy_get_purpose_options` | `tool_dataprivacy` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_get_users` | `tool_dataprivacy` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_mark_complete` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_set_context_defaults` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_set_context_form` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_set_contextlevel_form` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_submit_selected_courses_form` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_dataprivacy_tree_extra_branches` | `tool_dataprivacy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_competencies_manage_page` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_competency_frameworks_manage_page` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_competency_summary` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_course_competencies_page` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_plan_page` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_plans_page` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_related_competencies_section` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_template_competencies_page` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_templates_manage_page` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_user_competency_summary` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_user_competency_summary_in_course` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_user_competency_summary_in_plan` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_user_evidence_list_page` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_data_for_user_evidence_page` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_list_courses_using_competency` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_lp_search_cohorts` | `tool_lp` | read | v45, v51, v52 | REST | no |
| `tool_lp_search_users` | `tool_lp` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_mobile_call_external_functions` | `tool_mobile` | write | v45, v51, v52 | REST | no |
| `tool_mobile_get_autologin_key` | `tool_mobile` | write | v45, v51, v52 | REST | no |
| `tool_mobile_get_config` | `tool_mobile` | read | v45, v51, v52 | REST | no |
| `tool_mobile_get_content` | `tool_mobile` | read | v45, v51, v52 | REST | no |
| `tool_mobile_get_plugins_supporting_mobile` | `tool_mobile` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_mobile_get_public_config` | `tool_mobile` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_mobile_get_tokens_for_qr_login` | `tool_mobile` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_mobile_validate_subscription_key` | `tool_mobile` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_moodlenet_search_courses` | `tool_moodlenet` | read | v45, v51 | AJAX, REST | no |
| `tool_moodlenet_verify_webfinger` | `tool_moodlenet` | read | v45, v51 | AJAX, REST | no |
| `tool_policy_get_policy_version` | `tool_policy` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_policy_get_user_acceptances` | `tool_policy` | read | v45, v51, v52 | REST | no |
| `tool_policy_set_acceptances_status` | `tool_policy` | write | v45, v51, v52 | REST | no |
| `tool_policy_submit_accept_on_behalf` | `tool_policy` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_templatelibrary_list_templates` | `tool_templatelibrary` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_templatelibrary_load_canonical_template` | `tool_templatelibrary` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_usertours_complete_tour` | `tool_usertours` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_usertours_fetch_and_start_tour` | `tool_usertours` | read | v45, v51, v52 | AJAX, REST | no |
| `tool_usertours_reset_tour` | `tool_usertours` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_usertours_step_shown` | `tool_usertours` | write | v45, v51, v52 | AJAX, REST | no |
| `tool_xmldb_invoke_move_action` | `tool_xmldb` | write | v45, v51, v52 | AJAX, REST | no |
