package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
)

// workflowSpec gives a stable, task-oriented name to one reviewed core
// function. The generated registry still owns its parameter and return types.
// This layer owns discoverability, write confirmation and command metadata.
type workflowSpec struct {
	Name     string
	Short    string
	Function string
	Write    bool
	Defaults map[string]any
}

func newWorkflowCommand(r *Renderer, deps Deps, mode *safety.Mode, spec workflowSpec) *cobra.Command {
	var flags sessionFlags
	var params []string
	var paramsJSON string
	var dryRun, yes bool
	annotations := map[string]string{annotationKind: "workflow.call"}
	if spec.Write {
		annotations[annotationMutates] = "true"
	}
	cmd := &cobra.Command{
		Use:         spec.Name,
		Short:       spec.Short,
		Args:        cobra.NoArgs,
		Annotations: annotations,
		RunE: func(cmd *cobra.Command, args []string) error {
			values, err := collectTypedParams(params, paramsJSON)
			if err != nil {
				return err
			}
			for key, value := range spec.Defaults {
				if _, set := values[key]; !set {
					values[key] = value
				}
			}
			if spec.Write && !dryRun && !yes {
				return errs.New(errs.CodeUsage, spec.Name+" changes Moodle and requires confirmation").
					WithHint("inspect it with --dry-run, then pass --yes")
			}
			if deps.WS == nil {
				return errs.New(errs.CodeInternal, "the typed web-service caller is not configured")
			}
			resolved, session, err := openSession(cmd, deps, flags)
			if err != nil {
				return err
			}
			result, err := deps.WS(session,
				safety.Mode{DryRun: dryRun, ReadOnly: mode.ReadOnly},
				spec.Write && yes).Call(cmd.Context(), resolved.capabilities,
				resolved.capabilities.Release, spec.Function, values, dryRun)
			if err != nil {
				return err
			}
			payload := v1.WorkflowCallResult{
				Operation: strings.ReplaceAll(cmd.CommandPath(), " ", "."),
				Function:  result.Function, Version: result.Version, Params: result.Params,
				Response: result.Response, DryRun: result.DryRun,
				Effect: string(result.Effect), Destructive: result.Destructive,
			}
			meta := v1.NewMeta(v1.SourceWS)
			meta.Site = stringPointer(resolved.resolved.SiteName)
			meta.Account = stringPointer(resolved.resolved.AccountName)
			return r.Render(Result{
				Envelope: v1.NewEnvelope("workflow.call", payload, meta),
				Human: func(w io.Writer) error {
					if result.DryRun {
						fmt.Fprintf(w, "Would run %s using %s. Nothing was sent.\n", cmd.CommandPath(), result.Function)
						encoded, marshalErr := json.MarshalIndent(result.Params, "", "  ")
						if marshalErr != nil {
							return marshalErr
						}
						_, marshalErr = fmt.Fprintf(w, "%s\n", encoded)
						return marshalErr
					}
					return writeWSCall(w, result)
				},
			})
		},
	}
	flags.bind(cmd, spec.Short)
	cmd.Flags().StringArrayVar(&params, "param", nil, "a top-level parameter as name=value, repeatable")
	cmd.Flags().StringVar(&paramsJSON, "params-json", "", "structured parameters as a JSON object; see `moodle ws describe "+spec.Function+"`")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate and show the operation without sending it")
	if spec.Write {
		cmd.Flags().BoolVar(&yes, "yes", false, "confirm this Moodle write")
	}
	return cmd
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func addWorkflowCommands(parent *cobra.Command, r *Renderer, deps Deps, mode *safety.Mode, specs ...workflowSpec) {
	for _, spec := range specs {
		parent.AddCommand(newWorkflowCommand(r, deps, mode, spec))
	}
}

func courseWorkflowSpecs() []workflowSpec {
	return []workflowSpec{
		{Name: "search", Short: "Search courses the account may discover", Function: "core_course_search_courses"},
		{Name: "show", Short: "Show a course by id, shortname or idnumber", Function: "core_course_get_courses_by_field"},
		{Name: "contents", Short: "Show sections, activities and files in a course", Function: "core_course_get_contents"},
		{Name: "create", Short: "Create one or more courses", Function: "core_course_create_courses", Write: true},
		{Name: "update", Short: "Update one or more courses", Function: "core_course_update_courses", Write: true},
		{Name: "delete", Short: "Delete disposable courses", Function: "core_course_delete_courses", Write: true},
	}
}

func assignmentWorkflowSpecs() []workflowSpec {
	return []workflowSpec{
		{Name: "submissions", Short: "List assignment submissions", Function: "mod_assign_get_submissions"},
		{Name: "grade", Short: "Grade one assignment submission", Function: "mod_assign_save_grade", Write: true},
		{Name: "extend", Short: "Set submission extension dates", Function: "mod_assign_save_user_extensions", Write: true},
		{Name: "lock", Short: "Lock selected submissions", Function: "mod_assign_lock_submissions", Write: true},
		{Name: "unlock", Short: "Unlock selected submissions", Function: "mod_assign_unlock_submissions", Write: true},
		{Name: "revert", Short: "Revert selected submissions to draft", Function: "mod_assign_revert_submissions_to_draft", Write: true},
		{Name: "reveal-identities", Short: "Reveal identities for blind marking", Function: "mod_assign_reveal_identities", Write: true},
	}
}

func gradeWorkflowSpecs() []workflowSpec {
	return []workflowSpec{
		{Name: "update", Short: "Update grades for a component", Function: "core_grades_update_grades", Write: true},
		{Name: "category-create", Short: "Create gradebook categories", Function: "core_grades_create_gradecategories", Write: true},
	}
}

func forumWorkflowSpecs() []workflowSpec {
	return []workflowSpec{
		{Name: "create", Short: "Create a forum discussion", Function: "mod_forum_add_discussion", Write: true},
		{Name: "reply", Short: "Reply to a forum discussion", Function: "mod_forum_add_discussion_post", Write: true},
		{Name: "edit", Short: "Edit a discussion post", Function: "mod_forum_update_discussion_post", Write: true},
		{Name: "delete", Short: "Delete a discussion post", Function: "mod_forum_delete_post", Write: true},
		{Name: "subscribe", Short: "Subscribe to a forum", Function: "mod_forum_set_forum_subscription", Write: true, Defaults: map[string]any{"targetstate": true}},
		{Name: "unsubscribe", Short: "Unsubscribe from a forum", Function: "mod_forum_set_forum_subscription", Write: true, Defaults: map[string]any{"targetstate": false}},
		{Name: "pin", Short: "Pin a discussion", Function: "mod_forum_set_pin_state", Write: true, Defaults: map[string]any{"targetstate": 1}},
		{Name: "unpin", Short: "Unpin a discussion", Function: "mod_forum_set_pin_state", Write: true, Defaults: map[string]any{"targetstate": 0}},
		{Name: "lock", Short: "Lock a discussion at a timestamp", Function: "mod_forum_set_lock_state", Write: true},
		{Name: "unlock", Short: "Unlock a discussion", Function: "mod_forum_set_lock_state", Write: true, Defaults: map[string]any{"targetstate": 0}},
		{Name: "favourite", Short: "Favourite a discussion", Function: "mod_forum_toggle_favourite_state", Write: true, Defaults: map[string]any{"targetstate": true}},
		{Name: "unfavourite", Short: "Remove a discussion from favourites", Function: "mod_forum_toggle_favourite_state", Write: true, Defaults: map[string]any{"targetstate": false}},
	}
}

func calendarWorkflowSpecs() []workflowSpec {
	return []workflowSpec{
		{Name: "create", Short: "Create calendar events", Function: "core_calendar_create_calendar_events", Write: true},
		{Name: "update", Short: "Update a calendar event through Moodle's event form", Function: "core_calendar_submit_create_update_form", Write: true},
		{Name: "delete", Short: "Delete calendar events", Function: "core_calendar_delete_calendar_events", Write: true},
	}
}

func newParticipantCommand(r *Renderer, deps Deps, mode *safety.Mode) *cobra.Command {
	cmd := &cobra.Command{Use: "participant", Short: "Find and inspect course participants"}
	addWorkflowCommands(cmd, r, deps, mode, participantWorkflowSpecs()...)
	return cmd
}

func participantWorkflowSpecs() []workflowSpec {
	return []workflowSpec{
		workflowSpec{Name: "list", Short: "List participants enrolled in a course", Function: "core_enrol_get_enrolled_users"},
		workflowSpec{Name: "show", Short: "Show course-scoped user profiles", Function: "core_user_get_course_user_profiles"},
		workflowSpec{Name: "search", Short: "Search users eligible for course enrolment", Function: "core_enrol_search_users"},
	}
}

func newEnrolmentCommand(r *Renderer, deps Deps, mode *safety.Mode) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "enrolment",
		Aliases: []string{"enrollment", "enrol"},
		Short:   "Inspect enrolment methods and manage course enrolments",
		Long: "Lists the enrolment methods available in a course and performs the " +
			"core enrolment lifecycle. Moodle does not expose a generic core function " +
			"for deleting an enrolment-method instance; remove therefore removes a " +
			"user enrolment, while method-instance administration remains available " +
			"through the typed ws registry when a plugin exposes it.",
	}
	addWorkflowCommands(cmd, r, deps, mode, enrolmentWorkflowSpecs()...)
	return cmd
}

func enrolmentWorkflowSpecs() []workflowSpec {
	return []workflowSpec{
		{Name: "methods", Short: "List enrolment methods available in a course", Function: "core_enrol_get_course_enrolment_methods"},
		{Name: "add", Short: "Manually enrol users in courses", Function: "enrol_manual_enrol_users", Write: true},
		{Name: "update", Short: "Update one user enrolment through Moodle's enrolment form", Function: "core_enrol_submit_user_enrolment_form", Write: true},
		{Name: "remove", Short: "Remove one user enrolment from a course", Function: "core_enrol_unenrol_user_enrolment", Write: true},
	}
}

func newGroupCommand(r *Renderer, deps Deps, mode *safety.Mode) *cobra.Command {
	cmd := &cobra.Command{Use: "group", Short: "Manage course groups and memberships"}
	addWorkflowCommands(cmd, r, deps, mode, groupWorkflowSpecs()...)
	return cmd
}

func groupWorkflowSpecs() []workflowSpec {
	return []workflowSpec{
		workflowSpec{Name: "list", Short: "List groups in a course", Function: "core_group_get_course_groups"},
		workflowSpec{Name: "create", Short: "Create groups", Function: "core_group_create_groups", Write: true},
		workflowSpec{Name: "update", Short: "Update groups", Function: "core_group_update_groups", Write: true},
		workflowSpec{Name: "delete", Short: "Delete groups", Function: "core_group_delete_groups", Write: true},
		workflowSpec{Name: "member-add", Short: "Add members to groups", Function: "core_group_add_group_members", Write: true},
		workflowSpec{Name: "member-remove", Short: "Remove members from groups", Function: "core_group_delete_group_members", Write: true},
	}
}

func newCompletionCommand(r *Renderer, deps Deps, mode *safety.Mode) *cobra.Command {
	cmd := &cobra.Command{Use: "completion", Short: "Read and update completion status"}
	addWorkflowCommands(cmd, r, deps, mode, completionWorkflowSpecs()...)
	return cmd
}

func completionWorkflowSpecs() []workflowSpec {
	return []workflowSpec{
		workflowSpec{Name: "course", Short: "Show course completion", Function: "core_completion_get_course_completion_status"},
		workflowSpec{Name: "activity", Short: "Show activity completion in a course", Function: "core_completion_get_activities_completion_status"},
		workflowSpec{Name: "mark", Short: "Manually mark activity completion", Function: "core_completion_update_activity_completion_status_manually", Write: true},
	}
}
