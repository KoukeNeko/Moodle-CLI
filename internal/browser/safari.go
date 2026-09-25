package browser

// Safari's Cookies.binarycookies is an undocumented format. This reader is
// deliberately narrow: it selects one named Moodle cookie for one exact host,
// bounds every offset, and treats an unfamiliar layout as protocol drift.
// The page/record layout is also used by browserutils/kooky's Safari reader:
// https://github.com/browserutils/kooky/blob/master/browser/safari/safari.go

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

const (
	safariMaxFile = 256 << 20
	safariMaxPage = 32 << 20
	safariEpoch   = 978307200 // 2001-01-01T00:00:00Z
)

// SafariCookiePath is the current sandboxed cookie store location. Its format
// and location are not a public Apple API, so callers must allow it to fail.
func SafariCookiePath(home string) string {
	return filepath.Join(home, "Library", "Containers", "com.apple.Safari",
		"Data", "Library", "Cookies", "Cookies.binarycookies")
}

// SafariProfiles discovers Safari only on macOS. A permission-denied store is
// still listed so an explicit import can give the user the real error.
func SafariProfiles(home string) []Profile {
	if runtime.GOOS != "darwin" {
		return nil
	}
	path := SafariCookiePath(home)
	_, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrPermission) {
		return nil
	}
	return []Profile{{Name: "Safari", Path: path, Kind: Safari, Default: true}}
}

// SafariSession reads a single named cookie from Safari's best-effort store.
// The caller must explicitly request browser import; this is never a login
// side effect. Moodle verifies the returned session before storing it.
func SafariSession(path, host, cookieName string) (Found, error) {
	if cookieName == "" {
		cookieName = SessionCookieName
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return Found{}, errs.Wrap(errs.CodeUnavailable, err,
				"macOS denied access to Safari's cookie store").
				WithHint("macOS may require explicit access for your terminal app; grant it only if you trust that app, or use the manual callback method")
		}
		if errors.Is(err, os.ErrNotExist) {
			return Found{}, errs.New(errs.CodeNotFound,
				"Safari's cookie store was not found").
				WithHint("sign in to this Moodle site in Safari, then try again")
		}
		return Found{}, errs.Wrap(errs.CodeUnavailable, err,
			"cannot open Safari's cookie store")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Found{}, errs.Wrap(errs.CodeUnavailable, err,
			"cannot inspect Safari's cookie store")
	}
	if info.Size() < 8 || info.Size() > safariMaxFile {
		return Found{}, safariFormatError("unexpected cookie store size")
	}
	header := make([]byte, 8)
	if _, err := io.ReadFull(f, header); err != nil || !bytes.Equal(header[:4], []byte("cook")) {
		return Found{}, safariFormatError("invalid cookie store header")
	}
	pageCount := binary.BigEndian.Uint32(header[4:])
	if pageCount == 0 || uint64(pageCount)*4 > uint64(info.Size()-8) {
		return Found{}, safariFormatError("invalid cookie store page count")
	}
	sizes := make([]byte, int(pageCount)*4)
	if _, err := io.ReadFull(f, sizes); err != nil {
		return Found{}, safariFormatError("truncated cookie store page index")
	}
	remaining := info.Size() - 8 - int64(len(sizes))
	for i := uint32(0); i < pageCount; i++ {
		size := binary.BigEndian.Uint32(sizes[4*i:])
		if size < 8 || size > safariMaxPage || int64(size) > remaining {
			return Found{}, safariFormatError("invalid cookie store page size")
		}
		remaining -= int64(size)
		page := make([]byte, size)
		if _, err := io.ReadFull(f, page); err != nil {
			return Found{}, safariFormatError("truncated cookie store page")
		}
		found, err := safariPageCookie(page, path, host, cookieName, time.Now())
		if err != nil {
			return Found{}, err
		}
		if found.Value != "" {
			return found, nil
		}
	}
	return Found{}, errs.New(errs.CodeNotFound,
		"no "+cookieName+" for "+host+" in Safari's cookie store").
		WithHint("Safari may keep an active session only in memory; this does not mean you are signed out. Use `moodle auth import-session` to enter its cookie privately")
}

func safariPageCookie(page []byte, source, host, cookieName string, now time.Time) (Found, error) {
	if len(page) < 8 || !bytes.Equal(page[:4], []byte{0, 0, 1, 0}) {
		return Found{}, safariFormatError("invalid cookie page header")
	}
	count := binary.LittleEndian.Uint32(page[4:8])
	if uint64(count)*4 > uint64(len(page)-8) {
		return Found{}, safariFormatError("invalid cookie page index")
	}
	for i := uint32(0); i < count; i++ {
		offset := binary.LittleEndian.Uint32(page[8+4*i:])
		if uint64(offset)+56 > uint64(len(page)) {
			return Found{}, safariFormatError("invalid cookie record offset")
		}
		record := page[offset:]
		size := binary.LittleEndian.Uint32(record[:4])
		if size < 56 || uint64(size) > uint64(len(record)) {
			return Found{}, safariFormatError("invalid cookie record size")
		}
		record = record[:size]
		domain, err := safariString(record, 16)
		if err != nil {
			return Found{}, err
		}
		if !hostMatches(domain, host) {
			continue
		}
		name, err := safariString(record, 20)
		if err != nil {
			return Found{}, err
		}
		if name != cookieName {
			continue
		}
		value, err := safariString(record, 28)
		if err != nil {
			return Found{}, err
		}
		if value == "" {
			continue
		}
		expiry := math.Float64frombits(binary.LittleEndian.Uint64(record[40:48]))
		if !math.IsNaN(expiry) && !math.IsInf(expiry, 0) && expiry > 0 &&
			expiry < float64(now.Unix()-safariEpoch) {
			continue
		}
		return Found{Host: domain, Name: name, Value: value, Source: source}, nil
	}
	return Found{}, nil
}

func safariString(record []byte, at int) (string, error) {
	offset := binary.LittleEndian.Uint32(record[at : at+4])
	if offset < 56 || uint64(offset) >= uint64(len(record)) {
		return "", safariFormatError("invalid cookie string offset")
	}
	end := bytes.IndexByte(record[offset:], 0)
	if end < 0 {
		return "", safariFormatError("unterminated cookie string")
	}
	return string(record[offset : int(offset)+end]), nil
}

func safariFormatError(message string) error {
	return errs.New(errs.CodeUpstream, "cannot read Safari's cookie store: "+message).
		WithReason(errs.ReasonProtocolDrift).
		WithHint("Safari's cookie format is not a public API and may have changed")
}
