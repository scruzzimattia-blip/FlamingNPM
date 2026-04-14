package proxy

import "testing"

func TestNormalizePathPrefix(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"api", "/api"},
		{"/api/", "/api"},
		{"/v1/foo/", "/v1/foo"},
	}
	for _, tc := range tests {
		if got := NormalizePathPrefix(tc.in); got != tc.want {
			t.Fatalf("NormalizePathPrefix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStripPathPrefixIfMatches(t *testing.T) {
	if got := StripPathPrefixIfMatches("/api/foo", "/api"); got != "/foo" {
		t.Fatalf("got %q", got)
	}
	if got := StripPathPrefixIfMatches("/api", "/api"); got != "/" {
		t.Fatalf("got %q", got)
	}
	if got := StripPathPrefixIfMatches("/api1/list", "/api"); got != "/api1/list" {
		t.Fatalf("got %q", got)
	}
	if got := StripPathPrefixIfMatches("/other", "/api"); got != "/other" {
		t.Fatalf("got %q", got)
	}
}
