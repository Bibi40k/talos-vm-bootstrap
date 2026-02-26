package buildctl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareGoVersions(t *testing.T) {
	cases := []struct {
		a, b string
		exp  int
	}{
		{"1.26.0", "1.26.0", 0},
		{"1.27.0", "1.26.9", 1},
		{"1.25.9", "1.26.0", -1},
		{"go1.26.1", "1.26.0", 1},
		{"1.26rc1", "1.26.0", -1},
	}
	for _, tc := range cases {
		got := compareGoVersions(tc.a, tc.b)
		if got < 0 && tc.exp >= 0 || got > 0 && tc.exp <= 0 || got == 0 && tc.exp != 0 {
			t.Fatalf("compareGoVersions(%q,%q)=%d, expected sign %d", tc.a, tc.b, got, tc.exp)
		}
	}
}

func TestParseVersion(t *testing.T) {
	got := parseVersion("go1.26.3-linux")
	if got[0] != 1 || got[1] != 26 || got[2] != 3 {
		t.Fatalf("unexpected parse: %#v", got)
	}
}

func TestRequireOut(t *testing.T) {
	if err := RequireOut(""); err == nil {
		t.Fatalf("expected error for empty OUT")
	}
	if err := RequireOut("build/devvm/kubeconfig"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCoverageColor(t *testing.T) {
	cases := []struct {
		in  string
		exp string
	}{
		{"85.2%", "green"},
		{"60.0%", "yellow"},
		{"46.2%", "red"},
	}
	for _, tc := range cases {
		got, err := coverageColor(tc.in)
		if err != nil {
			t.Fatalf("coverageColor(%q): %v", tc.in, err)
		}
		if got != tc.exp {
			t.Fatalf("coverageColor(%q)=%q, expected %q", tc.in, got, tc.exp)
		}
	}
}

func TestParsePercent(t *testing.T) {
	if _, err := parsePercent("x"); err == nil {
		t.Fatalf("expected parse error")
	}
	got, err := parsePercent("46.3%")
	if err != nil {
		t.Fatalf("parsePercent: %v", err)
	}
	if got != 46.3 {
		t.Fatalf("unexpected percent: %v", got)
	}
}

func TestFilterPackages(t *testing.T) {
	in := []string{
		"github.com/acme/project/internal/bootstrap",
		"github.com/acme/project/tools/buildctl",
		"github.com/acme/project/internal/workflow",
	}
	got := filterPackages(in)
	if len(got) != 2 {
		t.Fatalf("unexpected filtered package count: %d", len(got))
	}
	for _, p := range got {
		if p == "github.com/acme/project/tools/buildctl" {
			t.Fatalf("tools package was not filtered")
		}
	}
}

func TestWriteBadgeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coverage.json")
	if err := writeBadgeFile(path, "coverage-core", "83.0%"); err != nil {
		t.Fatalf("writeBadgeFile: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	s := string(raw)
	if s == "" || !containsAll(s, []string{`"label":"coverage-core"`, `"message":"83.0%"`, `"color":"green"`}) {
		t.Fatalf("unexpected badge json: %s", s)
	}
}

func containsAll(s string, parts []string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
