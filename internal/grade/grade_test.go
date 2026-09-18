package grade_test

import (
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/grade"
)

func number(value float64) *float64 { return &value }

func TestPercentageIsNeverComputedForAScale(t *testing.T) {
	// A scale's raw value is a position in a list of words. Dividing it by the
	// number of options produces something that looks like a percentage and
	// means nothing — "3 out of 5" on a scale of adjectives is not 60%.
	if got := grade.Percentage(number(3), number(1), number(5), true); got != nil {
		t.Errorf("percentage = %v, want nil for a scale", *got)
	}
}

func TestPercentageUsesTheRangeNotJustTheMaximum(t *testing.T) {
	// An item marked from 40 to 100 gives 70 a percentage of 50, not 70.
	got := grade.Percentage(number(70), number(40), number(100), false)
	if got == nil {
		t.Fatal("no percentage was computed")
	}
	if *got != 50 {
		t.Errorf("percentage = %v, want 50", *got)
	}
}

func TestPercentageIsNilWhenThereIsNothingToCompute(t *testing.T) {
	cases := map[string]struct{ value, min, max *float64 }{
		"ungraded":    {nil, number(0), number(100)},
		"no maximum":  {number(5), number(0), nil},
		"empty range": {number(0), number(0), number(0)},
		"inverted":    {number(5), number(10), number(0)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := grade.Percentage(c.value, c.min, c.max, false); got != nil {
				t.Errorf("percentage = %v, want nil", *got)
			}
		})
	}
}

func TestZeroIsAGradeAndNotAMissingOne(t *testing.T) {
	// The distinction the whole model turns on: a student who scored nothing
	// has been marked, and must not be shown as unmarked.
	zero := grade.Item{Grade: number(0), Min: number(0), Max: number(100)}
	if !zero.Graded() {
		t.Error("a mark of zero was reported as unmarked")
	}
	if got := grade.Percentage(zero.Grade, zero.Min, zero.Max, false); got == nil || *got != 0 {
		t.Errorf("percentage = %v, want 0", got)
	}
	if (grade.Item{}).Graded() {
		t.Error("an item with no mark was reported as marked")
	}
}
