package v1

import "github.com/KoukeNeko/moodle-cli/internal/errs"

// Process exit codes. This table is part of the contract.
const (
	ExitOK               = 0
	ExitInternal         = 1
	ExitUsage            = 2
	ExitConfiguration    = 3
	ExitAuthentication   = 4
	ExitPermissionDenied = 5
	ExitNotFound         = 6
	ExitValidation       = 7
	ExitConflict         = 8
	ExitUnavailable      = 9
	ExitNetwork          = 10
	ExitUpstream         = 11
	// ExitAmbiguous means the request may already have been applied upstream.
	// Scripts must treat it as "unknown", never as a plain failure to retry.
	ExitAmbiguous = 12
	// ExitInterrupted means the caller stopped the command, with Ctrl-C or
	// SIGTERM. It follows the shell convention of 128 plus the signal number
	// rather than taking a place in the table above, because stopping a
	// command is a decision rather than a way it failed.
	//
	// A write that was already in flight reports ExitAmbiguous instead:
	// Moodle does not undo it because the client stopped listening.
	ExitInterrupted = 130
)

var exitByCode = map[errs.Code]int{
	errs.CodeInternal:         ExitInternal,
	errs.CodeUsage:            ExitUsage,
	errs.CodeConfiguration:    ExitConfiguration,
	errs.CodeAuthentication:   ExitAuthentication,
	errs.CodePermissionDenied: ExitPermissionDenied,
	errs.CodeNotFound:         ExitNotFound,
	errs.CodeValidation:       ExitValidation,
	errs.CodeConflict:         ExitConflict,
	errs.CodeUnavailable:      ExitUnavailable,
	errs.CodeNetwork:          ExitNetwork,
	errs.CodeUpstream:         ExitUpstream,
}

// ExitCode maps an error to its process exit code. An ambiguous outcome wins
// over the code, because that is the case a script must special-case first.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	e := errs.From(err)
	if e.EffectiveOutcome() == errs.OutcomeAmbiguous {
		return ExitAmbiguous
	}
	if e.Reason == errs.ReasonInterrupted {
		return ExitInterrupted
	}
	if code, ok := exitByCode[e.Code]; ok {
		return code
	}
	return ExitInternal
}
