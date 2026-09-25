package browser_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/browser"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

func safariFixture(t *testing.T, domain, name, value string, expires time.Time) string {
	t.Helper()
	record := make([]byte, 56)
	for _, field := range []struct {
		at    int
		value string
	}{
		{16, domain}, {20, name}, {24, "/"}, {28, value},
	} {
		binary.LittleEndian.PutUint32(record[field.at:], uint32(len(record)))
		record = append(record, field.value...)
		record = append(record, 0)
	}
	binary.LittleEndian.PutUint32(record[:4], uint32(len(record)))
	binary.LittleEndian.PutUint64(record[40:],
		math.Float64bits(float64(expires.Unix()-978307200)))
	page := make([]byte, 12)
	copy(page[:4], []byte{0, 0, 1, 0})
	binary.LittleEndian.PutUint32(page[4:], 1)
	binary.LittleEndian.PutUint32(page[8:], 12)
	page = append(page, record...)
	var file bytes.Buffer
	file.WriteString("cook")
	if err := binary.Write(&file, binary.BigEndian, uint32(1)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&file, binary.BigEndian, uint32(len(page))); err != nil {
		t.Fatal(err)
	}
	file.Write(page)
	path := filepath.Join(t.TempDir(), "Cookies.binarycookies")
	if err := os.WriteFile(path, file.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSafariSessionFindsOnlyTheRequestedHostAndName(t *testing.T) {
	path := safariFixture(t, ".ecourse2.ccu.edu.tw", "MoodleSession", "secret-session",
		time.Now().Add(time.Hour))
	found, err := browser.SafariSession(path, "ecourse2.ccu.edu.tw", "")
	if err != nil {
		t.Fatal(err)
	}
	if found.Value != "secret-session" || found.Name != "MoodleSession" || found.Source != path {
		t.Fatalf("unexpected cookie: %+v", found)
	}
	for _, tc := range []struct{ host, name string }{
		{"unrelated.ccu.edu.tw", "MoodleSession"},
		{"ecourse2.ccu.edu.tw", "different"},
	} {
		_, err := browser.SafariSession(path, tc.host, tc.name)
		if errs.From(err).Code != errs.CodeNotFound {
			t.Fatalf("host=%q name=%q: expected not_found, got %v", tc.host, tc.name, err)
		}
	}
}

func TestSafariSessionDoesNotReturnExpiredCookie(t *testing.T) {
	path := safariFixture(t, "ecourse2.ccu.edu.tw", "MoodleSession", "expired",
		time.Now().Add(-time.Hour))
	_, err := browser.SafariSession(path, "ecourse2.ccu.edu.tw", "")
	if errs.From(err).Code != errs.CodeNotFound {
		t.Fatalf("expected not_found, got %v", err)
	}
}

func TestSafariSessionRejectsDamagedOffsets(t *testing.T) {
	path := safariFixture(t, "ecourse2.ccu.edu.tw", "MoodleSession", "secret",
		time.Now().Add(time.Hour))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The page starts after the 8-byte header and one 4-byte page size.
	// A record offset beyond the page must not cause a panic or overread.
	binary.LittleEndian.PutUint32(raw[20:], math.MaxUint32)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = browser.SafariSession(path, "ecourse2.ccu.edu.tw", "")
	if errs.From(err).Code != errs.CodeUpstream || !strings.Contains(err.Error(), "offset") {
		t.Fatalf("expected a bounded protocol error, got %v", err)
	}
}

func TestExplicitSafariFileIsRecognized(t *testing.T) {
	path := safariFixture(t, "ecourse2.ccu.edu.tw", "MoodleSession", "session",
		time.Now().Add(time.Hour))
	found, err := browser.ReadSession(browser.Profile{Path: path}, "ecourse2.ccu.edu.tw", "")
	if err != nil || found.Value != "session" {
		t.Fatalf("explicit Safari file: found=%+v err=%v", found, err)
	}
}

func TestSafariProfileDiscoveryIsMacOnly(t *testing.T) {
	home := t.TempDir()
	path := browser.SafariCookiePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("cook"), 0o600); err != nil {
		t.Fatal(err)
	}
	profiles := browser.SafariProfiles(home)
	if runtime.GOOS == "darwin" {
		if len(profiles) != 1 || profiles[0].Kind != browser.Safari || profiles[0].Path != path {
			t.Fatalf("Safari profile not found: %+v", profiles)
		}
	} else if len(profiles) != 0 {
		t.Fatalf("non-macOS host discovered Safari: %+v", profiles)
	}
}
