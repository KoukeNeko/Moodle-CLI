package assignment

import (
	"context"
	"sync"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// OverviewItem keeps a failed status read distinct from an unsubmitted item.
// A list may be readable even when Moodle refuses the status of one activity.
type OverviewItem struct {
	Assignment        Summary
	Submission        *State
	StatusErrorCode   errs.Code
	StatusErrorReason errs.Reason
}

type OverviewResult struct {
	Items      []OverviewItem
	Provenance site.Provenance
}

// Overview lists assignments once, then reads each account-specific state.
// A small fixed worker pool bounds the number of simultaneous page requests;
// the wire still requires one status read per assignment on Moodle sites that
// do not expose a bulk submission-status function.
func (s *Service) Overview(ctx context.Context, capabilities *site.Capabilities, courseIDs []string) (OverviewResult, error) {
	listed, err := s.List(ctx, capabilities, courseIDs)
	if err != nil {
		return OverviewResult{}, err
	}
	result := OverviewResult{
		Items:      make([]OverviewItem, len(listed.Assignments)),
		Provenance: listed.Provenance,
	}
	for i, item := range listed.Assignments {
		result.Items[i].Assignment = item
	}
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(4, len(result.Items)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range jobs {
				state, readErr := s.Status(ctx, capabilities, result.Items[i].Assignment.ID)
				if readErr != nil {
					failure := errs.From(readErr)
					result.Items[i].StatusErrorCode = failure.Code
					result.Items[i].StatusErrorReason = failure.Reason
					continue
				}
				result.Items[i].Submission = &state
			}
		}()
	}
	for i := range result.Items {
		jobs <- i
	}
	close(jobs)
	workers.Wait()
	seenMissing := make(map[string]bool, len(result.Provenance.Missing))
	for _, field := range result.Provenance.Missing {
		seenMissing[field] = true
	}
	addMissing := func(field string) {
		if !seenMissing[field] {
			result.Provenance.Missing = append(result.Provenance.Missing, field)
			seenMissing[field] = true
		}
	}
	for _, item := range result.Items {
		if item.Submission == nil {
			result.Provenance.Partial = true
			addMissing("data[].submission")
			continue
		}
		if item.Submission.Provenance.Partial {
			result.Provenance.Partial = true
		}
		for _, field := range item.Submission.Provenance.Missing {
			addMissing("data[].submission." + field)
		}
	}
	return result, nil
}
