package moodle_test

import (
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/moodle"
)

func TestEncode(t *testing.T) {
	cases := []struct {
		name   string
		params moodle.Params
		want   string
	}{
		{
			name:   "scalar",
			params: moodle.Params{"userid": 4},
			want:   "userid=4",
		},
		{
			name:   "bool becomes 1 or 0, not true or false",
			params: moodle.Params{"yes": true, "no": false},
			want:   "no=0&yes=1",
		},
		{
			name:   "string list",
			params: moodle.Params{"courseids": []string{"2", "3"}},
			want:   "courseids%5B0%5D=2&courseids%5B1%5D=3",
		},
		{
			name: "list of maps is the shape most Moodle functions want",
			params: moodle.Params{"options": []any{
				map[string]any{"name": "onlyactive", "value": 1},
			}},
			want: "options%5B0%5D%5Bname%5D=onlyactive&options%5B0%5D%5Bvalue%5D=1",
		},
		{
			name: "criteria pairs",
			params: moodle.Params{"criteria": []any{
				map[string]any{"key": "shortname", "value": "CS204"},
			}},
			want: "criteria%5B0%5D%5Bkey%5D=shortname&criteria%5B0%5D%5Bvalue%5D=CS204",
		},
		{
			name:   "nested map",
			params: moodle.Params{"filter": map[string]any{"a": "1", "b": "2"}},
			want:   "filter%5Ba%5D=1&filter%5Bb%5D=2",
		},
		{
			name: "deeply nested",
			params: moodle.Params{"a": []any{
				map[string]any{"b": []any{map[string]any{"c": "d"}}},
			}},
			want: "a%5B0%5D%5Bb%5D%5B0%5D%5Bc%5D=d",
		},
		{
			name:   "float that is really an integer stays integral",
			params: moodle.Params{"id": float64(123)},
			want:   "id=123",
		},
		{
			name:   "nil is omitted rather than sent as an empty string",
			params: moodle.Params{"a": nil, "b": "x"},
			want:   "b=x",
		},
		{
			name:   "empty string is still sent",
			params: moodle.Params{"a": ""},
			want:   "a=",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			values, err := moodle.Encode(c.params)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if got := values.Encode(); got != c.want {
				t.Errorf("Encode() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestEncodeIsDeterministic(t *testing.T) {
	params := moodle.Params{"z": 1, "a": 2, "m": []string{"x", "y"}}
	first, err := moodle.Encode(params)
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		next, err := moodle.Encode(params)
		if err != nil {
			t.Fatal(err)
		}
		if next.Encode() != first.Encode() {
			t.Fatalf("encoding is not stable:\n%s\n%s", first.Encode(), next.Encode())
		}
	}
}

func TestEncodeRejectsUnsupportedTypes(t *testing.T) {
	// Failing loudly beats sending Moodle something like "%!s(struct…)".
	type custom struct{ A int }
	if _, err := moodle.Encode(moodle.Params{"x": custom{1}}); err == nil {
		t.Fatal("Encode accepted an unsupported type")
	}
}
