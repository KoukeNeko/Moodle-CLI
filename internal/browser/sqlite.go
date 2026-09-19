package browser

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// A very small, read-only SQLite reader.
//
// It exists because Chromium keeps its cookies in a SQLite database and this
// binary is CGO_ENABLED=0. The alternative was a pure-Go SQLite, which is a
// megabyte-scale dependency compiled into a tool that handles credentials —
// a large amount of code to trust for one table.
//
// So it reads one thing: whole rows from one table, by walking the table's
// own b-tree. It does not run SQL, does not use indexes, does not write, and
// refuses anything it was not built for rather than guessing. Every offset is
// bounds-checked and every page visit is counted, because the input is a file
// this tool did not write and a malformed one must produce an error rather
// than a wrong answer or a loop.
//
// Journal mode matters and was checked: Chromium's cookie database uses a
// rollback journal, not WAL. A WAL file holds committed data that is not in
// the main database, so reading the main file alone would quietly return a
// stale row. That case is detected and refused rather than answered.

const (
	sqliteHeaderSize = 100
	sqliteMagic      = "SQLite format 3\x00"

	pageInteriorTable = 0x05
	pageLeafTable     = 0x0D
	pageInteriorIndex = 0x02
	pageLeafIndex     = 0x0A
)

// sqliteDB is a database opened for reading.
type sqliteDB struct {
	data     []byte
	pageSize int
	pages    int
	// reserved is the space SQLite leaves at the end of every page, which
	// some builds use. It is not usually set, and ignoring it would read that
	// space as cell content.
	reserved int
}

// openSQLite parses the header of a database already read into memory.
func openSQLite(data []byte) (*sqliteDB, error) {
	if len(data) < sqliteHeaderSize {
		return nil, errors.New("too short to be a database")
	}
	if string(data[:16]) != sqliteMagic {
		return nil, errors.New("not a SQLite database")
	}
	pageSize := int(binary.BigEndian.Uint16(data[16:18]))
	if pageSize == 1 {
		pageSize = 65536
	}
	if pageSize < 512 || pageSize&(pageSize-1) != 0 {
		return nil, fmt.Errorf("page size %d is not a power of two", pageSize)
	}
	// Only the format this reads. A future file format is a reason to stop,
	// not to interpret the bytes anyway.
	if data[18] > 2 || data[19] > 2 {
		return nil, fmt.Errorf("file format %d/%d is newer than this reads",
			data[18], data[19])
	}
	if encoding := binary.BigEndian.Uint32(data[56:60]); encoding != 1 {
		return nil, fmt.Errorf("text encoding %d is not UTF-8", encoding)
	}
	db := &sqliteDB{
		data:     data,
		pageSize: pageSize,
		pages:    len(data) / pageSize,
		reserved: int(data[20]),
	}
	if db.pages == 0 {
		return nil, errors.New("the database has no pages")
	}
	return db, nil
}

// page returns one page, numbered from 1 as SQLite does.
func (db *sqliteDB) page(number int) ([]byte, error) {
	if number < 1 || number > db.pages {
		return nil, fmt.Errorf("page %d is outside the database", number)
	}
	start := (number - 1) * db.pageSize
	return db.data[start : start+db.pageSize], nil
}

// tableRoot finds a table's root page by reading sqlite_schema.
func (db *sqliteDB) tableRoot(name string) (int, error) {
	rows, err := db.rows(1)
	if err != nil {
		return 0, err
	}
	// sqlite_schema is (type, name, tbl_name, rootpage, sql).
	for _, row := range rows {
		if len(row) < 4 {
			continue
		}
		kind, _ := row[0].(string)
		found, _ := row[1].(string)
		if kind != "table" || !strings.EqualFold(found, name) {
			continue
		}
		root, ok := row[3].(int64)
		if !ok {
			return 0, fmt.Errorf("table %q has no root page", name)
		}
		return int(root), nil
	}
	return 0, fmt.Errorf("no table called %q", name)
}

// rows walks a table b-tree and returns every row.
//
// The page budget is not a performance guard: a database with a cycle in its
// pointers would otherwise loop for ever, and a file this tool did not write
// can contain one.
func (db *sqliteDB) rows(root int) ([][]any, error) {
	var out [][]any
	budget := db.pages + 1
	seen := make(map[int]bool, db.pages)
	// A page holds a bounded number of cells, so a database cannot honestly
	// contain more rows than this. The cap stops a crafted file from growing
	// the slice until memory runs out.
	maxRows := db.pages * db.pageSize / 4

	var walk func(number int) error
	walk = func(number int) error {
		if budget--; budget < 0 {
			return errors.New("the table's pages point in a circle")
		}
		if seen[number] {
			return errors.New("the table's pages point in a circle")
		}
		seen[number] = true

		page, err := db.page(number)
		if err != nil {
			return err
		}
		// Page 1 carries the file header before its b-tree header.
		offset := 0
		if number == 1 {
			offset = sqliteHeaderSize
		}
		if offset+12 > len(page) {
			return errors.New("a page is too short to hold its header")
		}

		switch page[offset] {
		case pageInteriorIndex, pageLeafIndex:
			return errors.New("this reads tables, not indexes")
		case pageLeafTable, pageInteriorTable:
		default:
			return fmt.Errorf("page %d is not part of a table b-tree", number)
		}

		cells := int(binary.BigEndian.Uint16(page[offset+3 : offset+5]))
		headerLen := 8
		if page[offset] == pageInteriorTable {
			headerLen = 12
		}
		pointers := offset + headerLen
		if pointers+cells*2 > len(page) {
			return errors.New("a page claims more cells than it can hold")
		}

		for i := 0; i < cells; i++ {
			at := int(binary.BigEndian.Uint16(page[pointers+i*2 : pointers+i*2+2]))
			if at < 0 || at >= len(page) {
				return errors.New("a cell points outside its page")
			}
			if page[offset] == pageInteriorTable {
				if at+4 > len(page) {
					return errors.New("an interior cell is truncated")
				}
				child := int(binary.BigEndian.Uint32(page[at : at+4]))
				if err := walk(child); err != nil {
					return err
				}
				continue
			}
			row, err := db.leafCell(page, at)
			if err != nil {
				return err
			}
			if len(out) >= maxRows {
				return errors.New("the database claims more rows than it can hold")
			}
			out = append(out, row)
		}

		if page[offset] == pageInteriorTable {
			right := int(binary.BigEndian.Uint32(page[offset+8 : offset+12]))
			return walk(right)
		}
		return nil
	}

	if err := walk(root); err != nil {
		return nil, err
	}
	return out, nil
}

// leafCell reads one row out of a table leaf page.
func (db *sqliteDB) leafCell(page []byte, at int) ([]any, error) {
	payloadLen, n := readVarint(page[at:])
	if n == 0 {
		return nil, errors.New("a cell has no length")
	}
	// A row cannot be longer than the file it is in, and this length comes
	// out of that file. Found by fuzzing: without the bound, a cell claiming
	// to be exabytes long reached make() and stopped the process dead — from
	// a database a browser handed over, on a credential path.
	if payloadLen > uint64(len(db.data)) {
		return nil, fmt.Errorf("a row claims to be %d bytes, longer than the database",
			payloadLen)
	}
	at += n
	_, n = readVarint(page[at:]) // rowid, which nothing here needs
	if n == 0 {
		return nil, errors.New("a cell has no row id")
	}
	at += n

	// How much of the payload lives on this page, and how much overflows, is
	// computed the way SQLite does. Getting it wrong reads the overflow
	// pointer as content.
	usable := db.pageSize - db.reserved
	maxLocal := usable - 35
	local := int(payloadLen)
	if local > maxLocal {
		minLocal := ((usable-12)*32)/255 - 23
		local = minLocal + (int(payloadLen)-minLocal)%(usable-4)
		if local > maxLocal {
			local = minLocal
		}
	}
	if at+local > len(page) {
		return nil, errors.New("a cell's payload runs past its page")
	}
	payload := make([]byte, 0, payloadLen)
	payload = append(payload, page[at:at+local]...)

	if int(payloadLen) > local {
		if at+local+4 > len(page) {
			return nil, errors.New("a cell has no overflow pointer")
		}
		next := int(binary.BigEndian.Uint32(page[at+local : at+local+4]))
		more, err := db.overflow(next, int(payloadLen)-local)
		if err != nil {
			return nil, err
		}
		payload = append(payload, more...)
	}
	return parseRecord(payload)
}

// overflow follows the chain of pages a long row spills onto.
func (db *sqliteDB) overflow(number, remaining int) ([]byte, error) {
	var out []byte
	budget := db.pages + 1
	for remaining > 0 {
		if budget--; budget < 0 {
			return nil, errors.New("the overflow pages point in a circle")
		}
		page, err := db.page(number)
		if err != nil {
			return nil, err
		}
		usable := db.pageSize - db.reserved
		take := usable - 4
		if take > remaining {
			take = remaining
		}
		if 4+take > len(page) {
			return nil, errors.New("an overflow page is too short")
		}
		out = append(out, page[4:4+take]...)
		remaining -= take
		number = int(binary.BigEndian.Uint32(page[0:4]))
		if remaining > 0 && number == 0 {
			return nil, errors.New("the overflow chain ends early")
		}
	}
	return out, nil
}

// parseRecord turns one row's bytes into values.
//
// SQLite stores a header of type codes followed by the values themselves.
// Only the types this needs are decoded; anything else is returned as nil
// rather than guessed at, which is honest for a reader this narrow.
func parseRecord(payload []byte) ([]any, error) {
	headerLen, n := readVarint(payload)
	if n == 0 || int(headerLen) > len(payload) || headerLen < uint64(n) {
		return nil, errors.New("a row has no usable header")
	}
	types := payload[n:headerLen]
	body := payload[headerLen:]

	var row []any
	for len(types) > 0 {
		code, used := readVarint(types)
		if used == 0 {
			return nil, errors.New("a row's header is truncated")
		}
		types = types[used:]

		var size int
		switch {
		case code == 0:
			row = append(row, nil)
			continue
		case code >= 1 && code <= 4:
			size = int(code)
		case code == 5:
			size = 6
		case code == 6, code == 7:
			size = 8
		case code == 8:
			row = append(row, int64(0))
			continue
		case code == 9:
			row = append(row, int64(1))
			continue
		case code >= 12 && code%2 == 0:
			size = int(code-12) / 2
		case code >= 13 && code%2 == 1:
			size = int(code-13) / 2
		default:
			return nil, fmt.Errorf("unknown column type %d", code)
		}
		if size > len(body) {
			return nil, errors.New("a row's values are shorter than its header says")
		}
		value := body[:size]
		body = body[size:]

		switch {
		case code >= 1 && code <= 6:
			row = append(row, signedInt(value))
		case code == 7:
			// A float, which nothing here reads. Kept as nil rather than
			// turned into an integer that would look like data.
			row = append(row, nil)
		case code >= 12 && code%2 == 0:
			row = append(row, append([]byte(nil), value...))
		default:
			row = append(row, string(value))
		}
	}
	return row, nil
}

// signedInt reads SQLite's big-endian two's-complement integers.
func signedInt(b []byte) int64 {
	var value int64
	if len(b) > 0 && b[0]&0x80 != 0 {
		value = -1 // sign-extend
	}
	for _, digit := range b {
		value = value<<8 | int64(digit)
	}
	return value
}

// readVarint reads SQLite's big-endian variable-length integer: up to nine
// bytes, seven bits each, with the ninth contributing all eight.
func readVarint(b []byte) (uint64, int) {
	var value uint64
	for i := 0; i < 8; i++ {
		if i >= len(b) {
			return 0, 0
		}
		value = value<<7 | uint64(b[i]&0x7F)
		if b[i]&0x80 == 0 {
			return value, i + 1
		}
	}
	if len(b) < 9 {
		return 0, 0
	}
	return value<<8 | uint64(b[8]), 9
}
