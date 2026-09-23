package moodle

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/internal/workload"
)

const FunctionCoursesByField = "core_course_get_courses_by_field"

// WorkloadBackend reads the caller's enrolments and all matching course
// details in two requests. It never performs one request per course.
type WorkloadBackend struct {
	client *Client
	token  string
}

func NewWorkloadBackend(client *Client, token string) *WorkloadBackend {
	return &WorkloadBackend{client: client, token: token}
}

func (b *WorkloadBackend) Name() site.BackendKind { return site.BackendWS }
func (b *WorkloadBackend) Requirement() site.Requirement {
	return site.Requirement{
		AllFunctions: []string{FunctionUserCourses, FunctionCoursesByField},
		Credential:   site.CredentialWSToken,
	}
}

type workloadEnrolment struct {
	ID        int64  `json:"id"`
	ShortName string `json:"shortname"`
	FullName  string `json:"fullname"`
}

type workloadField struct {
	ShortName string `json:"shortname"`
	Value     any    `json:"value"`
	ValueRaw  any    `json:"valueraw"`
}

type workloadDetail struct {
	ID           int64           `json:"id"`
	ShortName    string          `json:"shortname"`
	FullName     string          `json:"fullname"`
	CustomFields []workloadField `json:"customfields"`
}

func (b *WorkloadBackend) Courses(ctx context.Context, userID string, academic config.Academic) ([]workload.Course, error) {
	id, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid Moodle user id %q: %w", userID, err)
	}
	var enrolments []workloadEnrolment
	if err := b.client.Call(ctx, b.token, FunctionUserCourses, Params{"userid": id}, &enrolments); err != nil {
		return nil, err
	}
	if len(enrolments) == 0 {
		return []workload.Course{}, nil
	}
	ids := make([]string, 0, len(enrolments))
	byID := make(map[int64]workloadEnrolment, len(enrolments))
	for _, enrolment := range enrolments {
		ids = append(ids, strconv.FormatInt(enrolment.ID, 10))
		byID[enrolment.ID] = enrolment
	}
	var response struct {
		Courses []workloadDetail `json:"courses"`
	}
	if err := b.client.Call(ctx, b.token, FunctionCoursesByField,
		Params{"field": "ids", "value": strings.Join(ids, ",")}, &response); err != nil {
		return nil, err
	}

	details := make(map[int64]workloadDetail, len(response.Courses))
	for _, detail := range response.Courses {
		details[detail.ID] = detail
	}
	out := make([]workload.Course, 0, len(enrolments))
	for _, enrolment := range enrolments {
		course := workload.Course{
			ID: strconv.FormatInt(enrolment.ID, 10), ShortName: enrolment.ShortName,
			FullName: enrolment.FullName, Missing: []string{},
		}
		detail, ok := details[enrolment.ID]
		if !ok {
			course.Missing = []string{academic.CreditsField, academic.LevelField, academic.TermField}
			out = append(out, course)
			continue
		}
		fields := map[string]string{}
		for _, field := range detail.CustomFields {
			value := field.ValueRaw
			if value == nil {
				value = field.Value
			}
			fields[field.ShortName] = strings.TrimSpace(fmt.Sprint(value))
		}
		course.Level = fields[academic.LevelField]
		course.Term = fields[academic.TermField]
		if value, present := fields[academic.CreditsField]; present && value != "" {
			if credits, parseErr := strconv.ParseFloat(value, 64); parseErr == nil && credits >= 0 {
				course.Credits = &credits
			} else {
				course.Missing = append(course.Missing, academic.CreditsField)
			}
		} else {
			course.Missing = append(course.Missing, academic.CreditsField)
		}
		if course.Level == "" {
			course.Missing = append(course.Missing, academic.LevelField)
		}
		if course.Term == "" {
			course.Missing = append(course.Missing, academic.TermField)
		}
		out = append(out, course)
	}
	return out, nil
}
