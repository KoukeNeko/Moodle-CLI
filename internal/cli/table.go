package cli

import (
	"bytes"
	"io"
	"strings"

	"golang.org/x/text/width"
)

// table lays out tab-terminated cells like text/tabwriter with two spaces of
// padding, but measures a cell in terminal columns instead of runes. A CJK
// character takes two columns, and tabwriter counting it as one pushed every
// later column out of line on a site with Chinese course names.
//
// Columns are formed the way tabwriter forms them: a column is a run of
// consecutive lines that all have a tab-terminated cell at that position, and
// the text after a line's last tab is not part of any column.
type table struct {
	out    io.Writer
	buf    bytes.Buffer
	lines  [][]string
	widths []int
	err    error
}

func newTable(out io.Writer) *table { return &table{out: out} }

func (t *table) Write(p []byte) (int, error) { return t.buf.Write(p) }

// Flush writes everything buffered so far and resets the table.
func (t *table) Flush() error {
	text := t.buf.String()
	t.buf.Reset()
	if text == "" {
		return nil
	}
	complete := strings.HasSuffix(text, "\n")
	t.lines = t.lines[:0]
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		t.lines = append(t.lines, strings.Split(line, "\t"))
	}
	// tabwriter drops an empty cell that no newline ended.
	if last := t.lines[len(t.lines)-1]; !complete && len(last) > 1 && last[len(last)-1] == "" {
		t.lines[len(t.lines)-1] = last[:len(last)-1]
	}
	t.err = nil
	t.format(0, len(t.lines))
	if t.err == nil && complete {
		_, t.err = io.WriteString(t.out, "\n")
	}
	return t.err
}

// format is tabwriter's format: find each block of lines sharing the current
// column, size it, and recurse into the next column within that block.
func (t *table) format(line0, line1 int) {
	column := len(t.widths)
	for this := line0; this < line1; this++ {
		if column >= len(t.lines[this])-1 {
			continue
		}
		t.writeLines(line0, this)
		line0 = this
		cellWidth := 0
		for ; this < line1; this++ {
			line := t.lines[this]
			if column >= len(line)-1 {
				break
			}
			if w := displayWidth(line[column]) + 2; w > cellWidth {
				cellWidth = w
			}
		}
		t.widths = append(t.widths, cellWidth)
		t.format(line0, this)
		t.widths = t.widths[:len(t.widths)-1]
		line0 = this
	}
	t.writeLines(line0, line1)
}

func (t *table) writeLines(line0, line1 int) {
	for i := line0; i < line1 && t.err == nil; i++ {
		var row strings.Builder
		for j, cell := range t.lines[i] {
			row.WriteString(cell)
			if j < len(t.widths) && j < len(t.lines[i])-1 {
				row.WriteString(strings.Repeat(" ", t.widths[j]-displayWidth(cell)))
			}
		}
		if i < len(t.lines)-1 {
			row.WriteByte('\n')
		}
		_, t.err = io.WriteString(t.out, row.String())
	}
}

// displayWidth is how many terminal columns s takes.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		switch width.LookupRune(r).Kind() {
		case width.EastAsianWide, width.EastAsianFullwidth:
			n += 2
		default:
			n++
		}
	}
	return n
}
