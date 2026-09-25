// The rule enforced here is the one this project has broken five times:
//
//	A lower-fidelity route may lose certainty. It may never invent it.
//
// Every feature reads through more than one route — a web service token, a
// browser session, then the site's own pages — and they do not know the same
// amount. A field the page route cannot see must arrive as "not known", not as
// the Go zero value, because false and zero are answers in their own right: an
// offline assignment, a forum with no threads, a grade that is not withheld.
//
// A type check cannot tell which zero values are meaningful, so this asserts
// the narrower thing that is checkable: a field one route fills in and another
// leaves alone has to be able to say it was left alone.
package arch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// optionalFields are the fields whose zero value is a claim. Each names a
// state Moodle reports, so leaving one at its zero value says the site
// answered when it did not.
//
// Adding to this list is how a new field of that kind gets the guarantee. The
// entry is the type and field name as written in the domain package.
var optionalFields = map[string][]string{
	"internal/assignment/assignment.go": {
		// An offline assignment is a real setting; the page route cannot see it.
		"State.OnlineSubmission",
		// Moodle fills the list only when every member must submit.
		"State.MembersStillToSubmit",
	},
	"internal/grade/grade.go": {
		// A page draws a withheld grade and an unmarked one the same way.
		"Item.Hidden",
		// Moodle sends the lock only to an account that can manage grades.
		"Item.Locked",
	},
	"internal/forum/forum.go": {
		// VALUE_OPTIONAL in Moodle's own declaration; zero reads as empty.
		"Forum.Discussions",
	},
	"internal/quiz/quiz.go": {
		// Zero is a real setting for each — no limit, unlimited attempts —
		// and the page route cannot see any of them.
		"Quiz.TimeLimit",
		"Quiz.MaxAttempts",
		"Quiz.MaxGrade",
		// Null while unfinished or unmarked; zero would be a failing mark.
		"Attempt.Grade",
		"Detail.BestGrade",
	},
}

func TestAFieldThatCanBeUnknownSaysSo(t *testing.T) {
	for file, fields := range optionalFields {
		parsed, err := parser.ParseFile(token.NewFileSet(), "../../"+file, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, want := range fields {
			typeName, fieldName, ok := strings.Cut(want, ".")
			if !ok {
				t.Fatalf("%q is not Type.Field", want)
			}
			found, pointer := findField(parsed, typeName, fieldName)
			if !found {
				t.Errorf("%s: %s no longer exists; if it was renamed, move the "+
					"entry rather than dropping it", file, want)
				continue
			}
			if !pointer {
				t.Errorf("%s: %s is not a pointer, so a route that cannot see it "+
					"reports the zero value as though the site had said so", file, want)
			}
		}
	}
}

// findField reports whether the field exists and whether it is a pointer.
func findField(file *ast.File, typeName, fieldName string) (found, pointer bool) {
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok || spec.Name.Name != typeName {
			return true
		}
		structType, ok := spec.Type.(*ast.StructType)
		if !ok {
			return false
		}
		for _, field := range structType.Fields.List {
			for _, name := range field.Names {
				if name.Name != fieldName {
					continue
				}
				found = true
				_, pointer = field.Type.(*ast.StarExpr)
			}
		}
		return false
	})
	return found, pointer
}
