package main

import "testing"

func TestEntryURL(t *testing.T) {
	tests := []struct {
		base, slug, path string
		want             string
	}{
		{"http://localhost:8080", "wiki", "A/Einstein", "http://localhost:8080/wiki/A/Einstein"},
		{"http://localhost:8080", "wiki", "path with spaces", "http://localhost:8080/wiki/path%20with%20spaces"},
		{"http://localhost:8080", "wiki", "page?foo=1", "http://localhost:8080/wiki/page%3Ffoo=1"},
		{"http://localhost:8080", "wiki", "page#section", "http://localhost:8080/wiki/page%23section"},
		{"http://localhost:8080", "wiki", "", "http://localhost:8080/wiki/"},
		{"", "wiki", "index.html", "/wiki/index.html"},
		{"http://localhost:8080", "wiki", "a/b/c.html", "http://localhost:8080/wiki/a/b/c.html"},
		{"http://localhost:8080", "wiki", "café.html", "http://localhost:8080/wiki/caf%C3%A9.html"},
		{"http://localhost:8080", "wiki", "../etc/passwd", "http://localhost:8080/wiki/../etc/passwd"},
	}
	for _, tt := range tests {
		got := entryURL(tt.base, tt.slug, tt.path)
		if got != tt.want {
			t.Errorf("entryURL(%q, %q, %q) = %q, want %q", tt.base, tt.slug, tt.path, got, tt.want)
		}
	}
}

func TestEntryHref(t *testing.T) {
	tests := []struct {
		slug, path string
		want       string
	}{
		{"wiki", "A/Einstein", "/wiki/A/Einstein"},
		{"wiki", "path with spaces", "/wiki/path%20with%20spaces"},
		{"wiki", "page?query", "/wiki/page%3Fquery"},
		{"wiki", "page#frag", "/wiki/page%23frag"},
		{"wiki", "", "/wiki/"},
		{"wiki", "a%20b", "/wiki/a%2520b"}, // already-encoded input gets double-encoded
		{"archive", "index.html", "/archive/index.html"},
	}
	for _, tt := range tests {
		got := entryHref(tt.slug, tt.path)
		if got != tt.want {
			t.Errorf("entryHref(%q, %q) = %q, want %q", tt.slug, tt.path, got, tt.want)
		}
	}
}
