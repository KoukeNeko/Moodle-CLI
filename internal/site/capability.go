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
// by version number. The release is only ever a hint for setting these
//.
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
	Unavailable map[string]errs.Reason
	CanUpload   bool
	CanDownload bool
	Release     string
	Version     string
	SiteName    string
	UserID      string
	Username    string
	FullName    string
	Quirks      Quirks
}

// NewCapabilities returns an empty, usable set.
func NewCapabilities() *Capabilities {
	return &Capabilities{
		Functions:   map[string]FunctionInfo{},
		Unavailable: map[string]errs.Reason{},
	}
}

// Has reports whether a function is available.
func (c *Capabilities) Has(function string) bool {
	if c == nil {
		return false
	}
	if _, blocked := c.Unavailable[function]; blocked {
		return false
	}
	_, ok := c.Functions[function]
	return ok
}

// MarkUnavailable records a function that turned out not to work, so it is not
// tried again in this process. The AJAX backend has no function list at all,
// so this is the only way it learns.
func (c *Capabilities) MarkUnavailable(function string, reason errs.Reason) {
	if c.Unavailable == nil {
		c.Unavailable = map[string]errs.Reason{}
	}
	c.Unavailable[function] = reason
}

// FunctionNames returns the available function names, sorted.
func (c *Capabilities) FunctionNames() []string {
	names := make([]string, 0, len(c.Functions))
	for name := range c.Functions {
		if _, blocked := c.Unavailable[name]; blocked {
			continue
		}
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
	if len(r.AnyFunction) > 0 {
		found := false
		for _, function := range r.AnyFunction {
			if c.Has(function) {
				found = true
				break
			}
		}
		if !found {
			return false, fmt.Sprintf("this site exposes none of: %s", strings.Join(r.AnyFunction, ", "))
		}
	}
	for _, function := range r.AllFunctions {
		if !c.Has(function) {
			return false, fmt.Sprintf("this site does not expose %s", function)
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
