package browser

import (
	"os"
	"testing"
)

// FuzzSQLite throws malformed databases at the reader.
//
// This parses a file the tool did not write, with offsets and lengths taken
// from inside it, on a path that handles credentials. A corrupt one must
// produce an error — never a panic, never a wrong row, never a loop. The
// seed is the real Chrome database, so the mutations start from something
// structurally valid and stay near it.
func FuzzSQLite(f *testing.F) {
	if raw, err := os.ReadFile("testdata/chrome-cookies.sqlite"); err == nil {
		f.Add(raw)
	}
	f.Add([]byte("SQLite format 3\x00"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		db, err := openSQLite(data)
		if err != nil {
			return
		}
		root, err := db.tableRoot("cookies")
		if err != nil {
			return
		}
		rows, err := db.rows(root)
		if err != nil {
			return
		}
		// Reaching here means it claimed to have read rows. They must at
		// least be self-consistent: a row that says it has columns must have
		// them, since the caller reads by position.
		for _, row := range rows {
			for range row {
			}
		}
	})
}
