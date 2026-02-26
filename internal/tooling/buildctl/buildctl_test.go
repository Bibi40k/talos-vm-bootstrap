package buildctl

import "testing"

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
