package browser

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Chromium's cookie encryption, as its own source describes it.
//
// A value is stored with a three-byte version prefix. v10 uses a password
// that is written into Chromium itself; v11 uses one held by the desktop's
// secret service; v20 on Windows binds the key to the browser process and is
// deliberately out of reach. This reads v10 and says so about the others,
// rather than failing in a way that looks like an absent cookie.
const (
	chromiumSalt       = "saltysalt"
	chromiumKeyLen     = 16
	chromiumHardcoded  = "peanuts"
	chromiumPrefixV10  = "v10"
	chromiumPrefixV11  = "v11"
	chromiumPrefixV20  = "v20"
	chromiumSchemaHash = 24 // the version from which the plaintext is prefixed
)

// chromiumIterations differs by platform, and getting it wrong produces a key
// that decrypts to rubbish rather than an error — so it is stated per
// platform rather than assumed to be one number.
func chromiumIterations() int {
	if runtime.GOOS == "darwin" {
		return 1003
	}
	return 1
}

// ChromiumSession looks for one site's session cookie in a Chromium profile.
//
// Unlike Firefox, Chromium does keep session cookies in its database —
// measured on Chrome 153: the row is there with is_persistent 0 and no
// expiry. What it does not do is keep them in the clear.
func ChromiumSession(profile, host, cookieName string) (Found, error) {
	if cookieName == "" {
		cookieName = SessionCookieName
	}
	path := filepath.Join(profile, "Cookies")
	if _, err := os.Stat(path); err != nil {
		// Chrome keeps per-profile directories under the user data directory;
		// accept either being named.
		path = filepath.Join(profile, "Default", "Cookies")
	}

	// A write-ahead log holds committed rows that are not in the main file,
	// so reading the main file alone would quietly answer with a stale row —
	// or miss the cookie entirely. Chrome uses a rollback journal today;
	// if that changes, this says so rather than guessing.
	if _, err := os.Stat(path + "-wal"); err == nil {
		return Found{}, errs.New(errs.CodeUnavailable,
			"this profile's cookie database has a write-ahead log").
			WithHint("that holds changes this reader cannot see, so the answer " +
				"would be out of date; close the browser and try again")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Found{}, errs.New(errs.CodeNotFound,
				"this profile has no cookie database").
				WithHint("if the browser has never run with this profile there " +
					"is nothing here yet")
		}
		return Found{}, errs.Wrap(errs.CodeUnavailable, err, "cannot read "+path)
	}

	db, err := openSQLite(raw)
	if err != nil {
		return Found{}, errs.Wrap(errs.CodeUpstream, err,
			"cannot read the cookie database").
			WithReason(errs.ReasonProtocolDrift)
	}
	schema, _ := db.userVersion("version")
	root, err := db.tableRoot("cookies")
	if err != nil {
		return Found{}, errs.Wrap(errs.CodeUpstream, err,
			"cannot find the cookies in that database").
			WithReason(errs.ReasonProtocolDrift)
	}
	rows, err := db.rows(root)
	if err != nil {
		return Found{}, errs.Wrap(errs.CodeUpstream, err,
			"cannot read the cookie database").
			WithReason(errs.ReasonProtocolDrift)
	}

	// The columns are addressed by name, because Chromium adds them over
	// time: reading by position would silently shift when it next does.
	columns, err := db.columns("cookies")
	if err != nil {
		return Found{}, err
	}
	hostAt, nameAt := columns["host_key"], columns["name"]
	encryptedAt, plainAt := columns["encrypted_value"], columns["value"]
	if hostAt < 0 || nameAt < 0 || encryptedAt < 0 {
		return Found{}, errs.New(errs.CodeUpstream,
			"that cookie database does not have the columns this reads").
			WithReason(errs.ReasonProtocolDrift)
	}

	for _, row := range rows {
		rowHost, _ := value[string](row, hostAt)
		rowName, _ := value[string](row, nameAt)
		if rowName != cookieName || !hostMatches(rowHost, host) {
			continue
		}
		if plain, ok := value[string](row, plainAt); ok && plain != "" {
			// Some rows are not encrypted at all, on a system with no secret
			// service configured.
			return Found{Host: rowHost, Name: rowName, Value: plain, Source: path}, nil
		}
		encrypted, ok := value[[]byte](row, encryptedAt)
		if !ok || len(encrypted) == 0 {
			continue
		}
		decrypted, err := decryptChromium(encrypted, rowHost, schema)
		if err != nil {
			return Found{}, err
		}
		return Found{Host: rowHost, Name: rowName, Value: decrypted, Source: path}, nil
	}

	return Found{}, errs.New(errs.CodeNotFound,
		"no "+cookieName+" for "+host+" in this profile's cookie database").
		WithHint("that is what the database holds, not whether you are signed " +
			"in; a different profile would look the same")
}

// decryptChromium unwraps one stored value.
func decryptChromium(encrypted []byte, host string, schema int64) (string, error) {
	if len(encrypted) < 3 {
		return "", errs.New(errs.CodeUpstream, "the stored value is too short to read").
			WithReason(errs.ReasonProtocolDrift)
	}
	prefix := string(encrypted[:3])
	switch prefix {
	case chromiumPrefixV20:
		// App-bound encryption. The key is held for the browser process
		// itself, and the ways around it are the ones an attacker uses. This
		// project does not do them, and says so rather than appearing to try.
		return "", errs.New(errs.CodeUnavailable,
			"this cookie is sealed to the browser itself (v20)").
			WithHint("reading it would mean working around a protection the " +
				"browser applied on purpose; sign in with another method instead")
	case chromiumPrefixV11:
		return "", errs.New(errs.CodeUnavailable,
			"this cookie's key is held by the desktop's secret service (v11)").
			WithHint("that is not read yet; sign in with another method, or " +
				"import from Firefox")
	case chromiumPrefixV10:
	default:
		return "", errs.New(errs.CodeUpstream,
			fmt.Sprintf("this cookie is stored in an unknown form (%q)", prefix)).
			WithReason(errs.ReasonProtocolDrift)
	}

	key, err := pbkdf2.Key(sha1.New, chromiumHardcoded, []byte(chromiumSalt),
		chromiumIterations(), chromiumKeyLen)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternal, err, "cannot derive the key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternal, err, "cannot build the cipher")
	}
	body := encrypted[3:]
	if len(body) == 0 || len(body)%block.BlockSize() != 0 {
		return "", errs.New(errs.CodeUpstream,
			"the stored value is not a whole number of blocks").
			WithReason(errs.ReasonProtocolDrift)
	}
	plain := make([]byte, len(body))
	// The initialisation vector is sixteen spaces. That is Chromium's choice,
	// not this project's.
	cipher.NewCBCDecrypter(block, bytes.Repeat([]byte{' '}, block.BlockSize())).
		CryptBlocks(plain, body)

	plain, err = stripPadding(plain, block.BlockSize())
	if err != nil {
		return "", err
	}
	// From schema 24 the plaintext begins with a hash of the cookie's own
	// host, which Chromium checks and removes. A reader that skipped this
	// would hand back thirty-two bytes of hash followed by the value.
	if schema >= chromiumSchemaHash {
		digest := sha256.Sum256([]byte(host))
		if !bytes.HasPrefix(plain, digest[:]) {
			return "", errs.New(errs.CodeValidation,
				"that cookie does not belong to the host it is filed under").
				WithHint("the database may have been copied from another machine")
		}
		plain = plain[len(digest):]
	}
	return string(plain), nil
}

// stripPadding removes PKCS#7 padding, refusing what is not valid rather than
// trimming whatever the last byte happens to say.
func stripPadding(plain []byte, blockSize int) ([]byte, error) {
	if len(plain) == 0 {
		return nil, errors.New("nothing to unpad")
	}
	size := int(plain[len(plain)-1])
	if size == 0 || size > blockSize || size > len(plain) {
		return nil, errs.New(errs.CodeUpstream,
			"the decrypted value is not padded as expected").
			WithReason(errs.ReasonProtocolDrift).
			WithHint("the key may be wrong, which happens when the browser " +
				"stores its own key elsewhere")
	}
	for _, b := range plain[len(plain)-size:] {
		if int(b) != size {
			return nil, errs.New(errs.CodeUpstream,
				"the decrypted value is not padded as expected").
				WithReason(errs.ReasonProtocolDrift)
		}
	}
	return plain[:len(plain)-size], nil
}

// value reads one column, when it holds the type expected.
func value[T any](row []any, at int) (T, bool) {
	var zero T
	if at < 0 || at >= len(row) {
		return zero, false
	}
	found, ok := row[at].(T)
	return found, ok
}

// columns maps a table's column names to their position, read from the SQL
// the database stores for itself.
func (db *sqliteDB) columns(table string) (map[string]int, error) {
	rows, err := db.rows(1)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		kind, _ := value[string](row, 0)
		name, _ := value[string](row, 1)
		if kind != "table" || !strings.EqualFold(name, table) {
			continue
		}
		statement, _ := value[string](row, 4)
		open := strings.Index(statement, "(")
		if open < 0 {
			return nil, errs.New(errs.CodeUpstream,
				"cannot read that table's definition").
				WithReason(errs.ReasonProtocolDrift)
		}
		found := map[string]int{
			"host_key": -1, "name": -1, "value": -1, "encrypted_value": -1,
		}
		for i, part := range splitColumns(statement[open+1:]) {
			field := strings.Fields(strings.TrimSpace(part))
			if len(field) == 0 {
				continue
			}
			column := strings.Trim(field[0], `"'`+"`[]")
			if _, wanted := found[column]; wanted {
				found[column] = i
			}
		}
		return found, nil
	}
	return nil, errs.New(errs.CodeUpstream, "no such table").
		WithReason(errs.ReasonProtocolDrift)
}

// splitColumns splits a CREATE TABLE body on commas that are not inside
// brackets, so a type like DECIMAL(10,2) does not split into two columns.
func splitColumns(body string) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range body {
		switch r {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				parts = append(parts, body[start:i])
				return parts
			}
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, body[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, body[start:])
}

// userVersion reads one value from the meta table Chromium keeps its schema
// version in. A database without one is not an error: the caller treats a
// missing version as "older than the change it guards".
func (db *sqliteDB) userVersion(key string) (int64, error) {
	root, err := db.tableRoot("meta")
	if err != nil {
		return 0, err
	}
	rows, err := db.rows(root)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		name, _ := value[string](row, 0)
		if name != key {
			continue
		}
		if number, ok := value[int64](row, 1); ok {
			return number, nil
		}
		if text, ok := value[string](row, 1); ok {
			var parsed int64
			if _, err := fmt.Sscanf(text, "%d", &parsed); err == nil {
				return parsed, nil
			}
		}
	}
	return 0, errors.New("no such meta key")
}
