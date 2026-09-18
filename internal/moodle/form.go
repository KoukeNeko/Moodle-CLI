package moodle

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Params are the arguments of a Moodle web service call.
//
// Values may be strings, numbers, bools, []any or map[string]any, nested to
// any depth. Moodle flattens structures into bracketed names, so
//
//	Params{"options": []any{map[string]any{"name": "x", "value": 1}}}
//
// becomes options[0][name]=x&options[0][value]=1.
type Params map[string]any

// Encode flattens params into Moodle's bracket notation.
//
// This is the only encoder in the project on purpose: writing
// values.Set("options[0][name]", …) by hand at each call site is how three
// slightly different spellings end up alive at once.
func Encode(params Params) (url.Values, error) {
	values := url.Values{}
	// Sort so the output is deterministic, which keeps golden tests and trace
	// logs stable.
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := encodeValue(values, key, params[key]); err != nil {
			return nil, err
		}
	}
	return values, nil
}

func encodeValue(values url.Values, name string, value any) error {
	switch typed := value.(type) {
	case nil:
		// Moodle has no "null" in form encoding; omitting the key is the only
		// faithful representation, and sending "" would mean an empty string.
		return nil
	case string:
		values.Set(name, typed)
	case bool:
		// Moodle expects 1/0, not true/false.
		if typed {
			values.Set(name, "1")
		} else {
			values.Set(name, "0")
		}
	case int:
		values.Set(name, strconv.Itoa(typed))
	case int64:
		values.Set(name, strconv.FormatInt(typed, 10))
	case float64:
		// JSON numbers decode as float64; keep integers integral so an id
		// never goes out as "123.000000".
		if typed == float64(int64(typed)) {
			values.Set(name, strconv.FormatInt(int64(typed), 10))
		} else {
			values.Set(name, strconv.FormatFloat(typed, 'f', -1, 64))
		}
	case []string:
		for i, item := range typed {
			values.Set(fmt.Sprintf("%s[%d]", name, i), item)
		}
	case []int:
		for i, item := range typed {
			values.Set(fmt.Sprintf("%s[%d]", name, i), strconv.Itoa(item))
		}
	case []any:
		for i, item := range typed {
			if err := encodeValue(values, fmt.Sprintf("%s[%d]", name, i), item); err != nil {
				return err
			}
		}
	case Params:
		return encodeValue(values, name, map[string]any(typed))
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := encodeValue(values, fmt.Sprintf("%s[%s]", name, key), typed[key]); err != nil {
				return err
			}
		}
	default:
		return errs.New(errs.CodeInternal,
			fmt.Sprintf("cannot encode parameter %q of type %T for Moodle", name, value))
	}
	return nil
}
