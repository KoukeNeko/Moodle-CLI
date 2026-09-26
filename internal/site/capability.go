package site

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// FunctionInfo is one web service function this token can see.
type FunctionInfo struct {
	Name    string
	Version string
}

// Quirks are known deviations of a particular site, keyed by name rather than
// by version number. The release is only ever a hint for setting these.
type Quirks struct {
	// SiteInfoRejectsLang marks sites that fail when get_site_info is given a
	// language argument.
	SiteInfoRejectsLang bool
}

// Capabilities is what this account, with this credential, can actually do on
// this site.
//
// It is deliberately tied to the account and not to the site: two accounts on
// the same Moodle can see different functions.
type Capabilities struct {
	AccountID   ID
	Credential  CredentialKind
	Functions   map[string]FunctionInfo
	CanUpload   bool
	CanDownload bool
	Release     string
	Version     string
	SiteName    string
	// SiteURL is the root the site builds its own links from. It is not
	// necessarily the address the user configured, and file links come from
	// this one.
	SiteURL  string
	UserID   string
	Username string
	FullName string
	Quirks   Quirks
}

// NewCapabilities returns an empty, usable set.
func NewCapabilities() *Capabilities {
	return &Capabilities{Functions: map[string]FunctionInfo{}}
}

// Has reports whether a function is available.
//
// There is deliberately no way to record one as unavailable after a call
// fails. A refusal from Moodle is about a course, an activity or a user, not
// about the function: mod_assign_get_submission_status refusing one assignment
// says nothing about the next. Remembering the refusal against the function
// would answer a later, permitted question with a stale no.
func (c *Capabilities) Has(function string) bool {
	if c == nil {
		return false
	}
	_, ok := c.Functions[function]
	return ok
}

// FunctionNames returns the available function names, sorted.
func (c *Capabilities) FunctionNames() []string {
	names := make([]string, 0, len(c.Functions))
	for name := range c.Functions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Provenance says where a result came from and what is missing from it.
//
// It travels with the data rather than being inferred later: an agent reading
// null must be able to tell "this course has no end date" from "the backend
// that answered cannot see end dates".
type Provenance struct {
	Source  BackendKind
	Partial bool
	// Missing names contract fields that could not be retrieved. It is never
	// nil — an empty slice means nothing was missing.
	Missing []string
	// PartialReason says, in one sentence, what a partial answer left out
	// when the thing left out is not a field: rows the site filtered from an
	// otherwise successful reply. Moodle answers an activity this account may
	// not read with a warning rather than an error, so the short list looks
	// complete — measured on a real site, 11 of 15 assignments withheld from
	// a student while the listing reported four and nothing else.
	//
	// It is deliberately not part of the JSON contract, which says only that
	// the answer is partial: a sentence is for a person, and meta.missing is
	// a list of field names.
	PartialReason string
}

// NewProvenance returns a Provenance for a complete answer from one backend.
func NewProvenance(source BackendKind) Provenance {
	return Provenance{Source: source, Missing: []string{}}
}

// Requirement says what a use case needs before it can run.
type Requirement struct {
	// AnyFunction is satisfied when at least one of these functions exists.
	// A use case declares alternatives here rather than testing the Moodle
	// version, because the same version exposes different functions on
	// different sites.
	AnyFunction []string
	// AllFunctions must all exist.
	AllFunctions []string
	// Credential, when set, restricts which credential kinds can satisfy it.
	Credential CredentialKind
	// NeedsUpload and NeedsDownload check the site's file permissions.
	NeedsUpload   bool
	NeedsDownload bool
}

// SatisfiedBy reports whether capabilities meet the requirement, and if not,
// why — so the caller can say which function was missing instead of a bare
// "unavailable".
func (r Requirement) SatisfiedBy(c *Capabilities) (bool, string) {
	if c == nil {
		return false, "no capabilities have been discovered yet"
	}
	if r.Credential != "" && c.Credential != "" && r.Credential != c.Credential {
		return false, fmt.Sprintf("needs a %s credential, this account has %s", r.Credential, c.Credential)
	}
	// The subject of these two sentences is the service, not the site. This
	// list comes from core_webservice_get_site_info, which answers for the
	// external service the token belongs to — measured by removing one
	// function from moodle_mobile_app while the site still had it installed,
	// and watching the old wording call it missing from the site.
	if len(r.AnyFunction) > 0 {
		found := false
		for _, function := range r.AnyFunction {
			if c.Has(function) {
				found = true
				break
			}
		}
		if !found {
			return false, fmt.Sprintf("the web service this account signs in through exposes none of: %s",
				strings.Join(r.AnyFunction, ", "))
		}
	}
	for _, function := range r.AllFunctions {
		if !c.Has(function) {
			return false, fmt.Sprintf("the web service this account signs in through does not expose %s", function)
		}
	}
	if r.NeedsUpload && !c.CanUpload {
		return false, "this account may not upload files"
	}
	if r.NeedsDownload && !c.CanDownload {
		return false, "this account may not download files"
	}
	return true, ""
}

// Unavailable builds the error for a requirement that cannot be met.
func (r Requirement) Unavailable(reason string) error {
	return errs.New(errs.CodeUnavailable, reason).
		WithReason(errs.ReasonCapability).
		WithHint("run `moodle doctor` to see what this site offers")
}
