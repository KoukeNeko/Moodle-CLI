package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// selectFields narrows an envelope's data to the named top-level fields: each
// row of a list, or the one object. The envelope and meta stay whole, so a
// caller still learns where the answer came from and what it is missing.
//
// A name the kind does not have is refused with the names it does have. An
// agent that guessed wrong learns the vocabulary in one round trip, where a
// silently empty result would read as a record with no such value.
func selectFields(envelope v1.Envelope, fields []string) (v1.Envelope, error) {
	names := normalizeFields(fields)
	if len(names) == 0 {
		return envelope, nil
	}
	raw, err := json.Marshal(envelope.Data)
	if err != nil {
		return envelope, errs.Wrap(errs.CodeInternal, err, "cannot encode the result")
	}
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return envelope, errs.Wrap(errs.CodeInternal, err, "cannot re-read the result")
	}

	available := schemaFields(envelope.Kind)
	switch value := data.(type) {
	case []any:
		rows := make([]any, 0, len(value))
		for _, item := range value {
			row, ok := item.(map[string]any)
			if !ok {
				return envelope, fieldsNotApplicable(envelope.Kind)
			}
			available = mergeNames(available, keys(row))
			rows = append(rows, row)
		}
		if err := checkFields(names, available); err != nil {
			return envelope, err
		}
		for i, row := range rows {
			rows[i] = pick(row.(map[string]any), names)
		}
		envelope.Data = rows
	case map[string]any:
		available = mergeNames(available, keys(value))
		if err := checkFields(names, available); err != nil {
			return envelope, err
		}
		envelope.Data = pick(value, names)
	default:
		return envelope, fieldsNotApplicable(envelope.Kind)
	}
	return envelope, nil
}

// schemaFields reads the field names a kind's data carries from its published
// schema. The schema is the authority: an empty list has no rows to learn the
// names from, and a misspelt field must still be refused.
func schemaFields(kind string) []string {
	doc, err := v1.Schema(kind)
	if err != nil {
		return nil
	}
	var schema struct {
		Properties struct {
			Data struct {
				Properties map[string]any `json:"properties"`
				Items      struct {
					Properties map[string]any `json:"properties"`
				} `json:"items"`
			} `json:"data"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(doc, &schema); err != nil {
		return nil
	}
	names := keys(schema.Properties.Data.Properties)
	return mergeNames(names, keys(schema.Properties.Data.Items.Properties))
}

func checkFields(names, available []string) error {
	known := map[string]bool{}
	for _, name := range available {
		known[name] = true
	}
	for _, name := range names {
		if !known[name] {
			return errs.New(errs.CodeUsage, fmt.Sprintf("unknown field %q", name)).
				WithHint("available fields: " + strings.Join(available, ", "))
		}
	}
	return nil
}

func fieldsNotApplicable(kind string) error {
	return errs.New(errs.CodeUsage,
		fmt.Sprintf("--fields selects from objects, and %s data is not made of them", kind)).
		WithHint("drop --fields for this command")
}

// normalizeFields splits comma lists and drops blanks and repeats, keeping
// the order asked for.
func normalizeFields(fields []string) []string {
	var names []string
	seen := map[string]bool{}
	for _, group := range fields {
		for _, name := range strings.Split(group, ",") {
			name = strings.TrimSpace(name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// pick keeps the named fields. A field the schema has but this row left out
// is reported as null, the same as the contract's own absent value.
func pick(row map[string]any, names []string) map[string]any {
	picked := make(map[string]any, len(names))
	for _, name := range names {
		picked[name] = row[name]
	}
	return picked
}

func keys(values map[string]any) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func mergeNames(groups ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, group := range groups {
		for _, name := range group {
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out
}
