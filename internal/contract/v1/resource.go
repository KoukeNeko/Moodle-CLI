package v1

import (
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// ResolvedURL is the resolve payload.
type ResolvedURL struct {
	URL string `json:"url"`
	// Kind is what the address points at, or "unknown" for a part of Moodle
	// this build does not recognise — which is not an error, the address may
	// be perfectly good.
	Kind   string  `json:"kind"`
	Module *string `json:"module"`
	// CMID is the course module id an activity address carries. It is NOT the
	// activity's own id; commands that take an assignment id accept the
	// address itself and do the lookup.
	CMID         *string `json:"cmid"`
	CourseID     *string `json:"course_id"`
	UserID       *string `json:"user_id"`
	DiscussionID *string `json:"discussion_id"`
	FileName     *string `json:"file_name"`
	// Command is what to run to act on this address, null when nothing here
	// can act on it yet.
	Command *string `json:"command"`
}

// Resolve converts a parsed address into its envelope.
//
// The meta source is local: this is parsing, and no site was asked anything.
func Resolve(raw string, resource site.Resource, command string) Envelope {
	payload := ResolvedURL{
		URL:          raw,
		Kind:         string(resource.Kind),
		Module:       optional(resource.Module),
		CMID:         optional(resource.CMID),
		CourseID:     optional(resource.CourseID),
		UserID:       optional(resource.UserID),
		DiscussionID: optional(resource.DiscussionID),
		FileName:     optional(resource.FileName),
		Command:      optional(command),
	}
	return NewEnvelope("resolve", payload, NewMeta(SourceLocal))
}
