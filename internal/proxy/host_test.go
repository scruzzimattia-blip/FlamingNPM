package proxy

import (
	"net/http"
	"testing"
)

func TestHostKeyFromRequest(t *testing.T) {
	r := &http.Request{Host: "Example.COM:443"}
	if got := hostKeyFromRequest(r); got != "example.com" {
		t.Fatalf("got %q", got)
	}
	r2 := &http.Request{Host: "app.test"}
	if got := hostKeyFromRequest(r2); got != "app.test" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeConfigHost(t *testing.T) {
	if got := normalizeConfigHost("FOO.bar:8080"); got != "foo.bar" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeConfigHost(""); got != "" {
		t.Fatalf("got %q", got)
	}
}
