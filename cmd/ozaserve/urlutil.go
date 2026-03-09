package main

import "net/url"

// entryURL builds a correctly percent-encoded absolute URL for an archive entry.
// Entry paths can contain characters like '?' and '#' that are special in URLs
// and must be encoded to avoid being misinterpreted as query string or fragment.
func entryURL(base, slug, path string) string {
	u := &url.URL{Path: "/" + slug + "/" + path}
	return base + u.EscapedPath()
}

// entryHref returns a root-relative, percent-encoded href for an archive entry.
func entryHref(slug, path string) string {
	u := &url.URL{Path: "/" + slug + "/" + path}
	return u.EscapedPath()
}
