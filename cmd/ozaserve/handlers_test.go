package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stazelabs/oza/oza"
	"github.com/stazelabs/oza/ozawrite"
)

// buildTestOZA creates a small OZA archive in a temp file and returns the path.
func buildTestOZA(t *testing.T, search bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.oza")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	opts := ozawrite.WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: search,
	}
	w := ozawrite.NewWriter(f, opts)
	w.SetMetadata("title", "Test Archive")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "test")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")
	w.SetMetadata("main_entry", "0")

	if _, err := w.AddEntry("index.html", "Main Page", "text/html",
		[]byte(`<html><head><title>Main</title></head><body><h1>Main Page</h1><p>Welcome.</p></body></html>`), true); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddEntry("articles/alpha.html", "Alpha Article", "text/html",
		[]byte(`<html><body><h1>Alpha</h1><p>Alpha content about quantum physics.</p></body></html>`), true); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddEntry("articles/beta.html", "Beta Article", "text/html",
		[]byte(`<html><body><h1>Beta</h1><p>Beta content about relativity.</p></body></html>`), true); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddEntry("style.css", "Style", "text/css",
		[]byte(`body { margin: 0; }`), false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddEntry("logo.png", "Logo", "image/png",
		[]byte{0x89, 0x50, 0x4e, 0x47}, false); err != nil {
		t.Fatal(err)
	}
	// Add a redirect entry.
	if _, err := w.AddRedirect("old-page.html", "Old Page", 0); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// newTestLibrary creates a library with a single archive for testing.
func newTestLibrary(t *testing.T, search bool) (*library, string) {
	t.Helper()
	initTemplates()
	path := buildTestOZA(t, search)
	a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })

	slug := "test"
	lc, lo := computeLetterIndex(a)
	fa := collectFrontArticleIDs(a)
	title, _ := a.Metadata("title")

	lib := &library{
		archives:  make(map[string]*archiveEntry),
		startTime: time.Now(),
	}
	lib.archives[slug] = &archiveEntry{
		archive:         a,
		slug:            slug,
		filename:        "test.oza",
		title:           title,
		letterCounts:    lc,
		letterOffsets:   lo,
		frontArticleIDs: fa,
	}
	lib.slugs = []string{slug}
	return lib, slug
}

// newTestServer creates an httptest.Server with full routing.
func newTestServer(t *testing.T, search bool) (*httptest.Server, *library) {
	t.Helper()
	lib, _ := newTestLibrary(t, search)

	mux := http.NewServeMux()
	mux.HandleFunc("/", lib.handleRoot)
	mux.HandleFunc("/favicon.ico", handleFaviconSVG)
	mux.HandleFunc("/_favicon.svg", handleFaviconSVG)
	mux.HandleFunc("/_random", lib.handleRandomAll)
	mux.HandleFunc("/_search", lib.handleSearchAll)
	mux.HandleFunc("/_info", lib.handleGlobalInfo)
	mux.HandleFunc("/{archive}/_search", lib.handleSearchJSON)
	mux.HandleFunc("/{archive}/-/search", lib.handleSearchPage)
	mux.HandleFunc("/{archive}/-/random", lib.handleRandom)
	mux.HandleFunc("/{archive}/-/browse", lib.handleBrowse)
	mux.HandleFunc("/{archive}/-/info", lib.handleInfo)
	mux.HandleFunc("/{archive}/-/info.json", lib.handleInfoJSON)
	mux.HandleFunc("/{archive}/{path...}", lib.handleContent)

	srv := httptest.NewServer(securityHeaders(methodCheck(mux)))
	t.Cleanup(srv.Close)
	return srv, lib
}

// --- Content Serving ---

func TestHandleContentHTML(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/test/index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
	}
	csp := resp.Header.Get("Content-Security-Policy")
	if csp != "sandbox" {
		t.Errorf("CSP = %q, want sandbox", csp)
	}
	if resp.Header.Get("ETag") == "" {
		t.Error("ETag header missing")
	}
}

func TestHandleContentCSS(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/test/style.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "text/css; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/css; charset=utf-8", ct)
	}
	if resp.Header.Get("Content-Security-Policy") != "" {
		t.Error("CSP sandbox should not be set for non-HTML")
	}
}

func TestHandleContentImage(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/test/logo.png")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", resp.Header.Get("Content-Type"))
	}
}

func TestHandleContentRedirect(t *testing.T) {
	srv, _ := newTestServer(t, false)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/test/old-page.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
}

func TestHandleContentEmptyPath(t *testing.T) {
	srv, _ := newTestServer(t, false)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/test/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Empty path either redirects to main entry (302) or returns 404 if no main entry.
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 302 or 404", resp.StatusCode)
	}
}

func TestHandleContentETag(t *testing.T) {
	srv, _ := newTestServer(t, false)

	// First request to get ETag.
	resp, err := http.Get(srv.URL + "/test/style.css")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("ETag missing from first response")
	}

	// Conditional request.
	req, _ := http.NewRequest("GET", srv.URL+"/test/style.css", nil)
	req.Header.Set("If-None-Match", etag)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotModified {
		t.Fatalf("conditional GET status = %d, want 304", resp2.StatusCode)
	}
}

// --- 404 / Error Pages ---

func TestHandleContent404(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/test/nonexistent.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestHandleContent404UnknownArchive(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/nosucharchive/index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestHandleRoot404(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/nonexistent-path")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// This hits handleRoot which returns 404 for non-"/" paths.
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Search ---

func TestHandleSearchAllEmpty(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/_search")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var results []searchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results for empty query, got %d", len(results))
	}
}

func TestHandleSearchAllNoIndex(t *testing.T) {
	srv, _ := newTestServer(t, false) // no search index

	resp, err := http.Get(srv.URL + "/_search?q=quantum")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var results []searchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	// No search index, so results should be empty.
	if len(results) != 0 {
		t.Errorf("expected 0 results without search index, got %d", len(results))
	}
}

func TestHandleSearchJSON(t *testing.T) {
	srv, _ := newTestServer(t, true)

	resp, err := http.Get(srv.URL + "/test/_search?q=alpha")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", resp.Header.Get("Content-Type"))
	}
	var results []searchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
}

func TestHandleSearchJSONUnknownArchive(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/nosuch/_search?q=foo")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestHandleSearchPage(t *testing.T) {
	srv, _ := newTestServer(t, true)

	resp, err := http.Get(srv.URL + "/test/-/search?q=alpha")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestHandleSearchPageJSON(t *testing.T) {
	srv, _ := newTestServer(t, true)

	resp, err := http.Get(srv.URL + "/test/-/search?q=alpha&format=json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", resp.Header.Get("Content-Type"))
	}
}

// --- Browse ---

func TestHandleBrowse(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/test/-/browse")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestHandleBrowseWithLetter(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/test/-/browse?letter=A")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestHandleBrowsePagination(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/test/-/browse?letter=A&offset=0&limit=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestHandleBrowseUnknownArchive(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/nosuch/-/browse")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Random ---

func TestHandleRandom(t *testing.T) {
	srv, _ := newTestServer(t, false)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/test/-/random")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
}

func TestHandleRandomAll(t *testing.T) {
	srv, _ := newTestServer(t, false)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/_random")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
}

func TestHandleRandomUnknownArchive(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/nosuch/-/random")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Info Pages ---

func TestHandleInfo(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/test/-/info")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestHandleInfoJSON(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/test/-/info.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", resp.Header.Get("Content-Type"))
	}
}

func TestHandleGlobalInfo(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/_info")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestHandleInfoUnknownArchive(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/nosuch/-/info")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Security Headers ---

func TestSecurityHeaders(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	checks := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "SAMEORIGIN",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for header, want := range checks {
		got := resp.Header.Get(header)
		if got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

// --- Method Check ---

func TestMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t, false)

	req, _ := http.NewRequest("POST", srv.URL+"/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
	if resp.Header.Get("Allow") != "GET, HEAD" {
		t.Errorf("Allow = %q, want 'GET, HEAD'", resp.Header.Get("Allow"))
	}
}

// --- Favicon ---

func TestFavicon(t *testing.T) {
	srv, _ := newTestServer(t, false)

	resp, err := http.Get(srv.URL + "/favicon.ico")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "image/svg+xml" {
		t.Errorf("Content-Type = %q", resp.Header.Get("Content-Type"))
	}
}

// --- Helper functions ---

func TestCommaInt(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{-1, "-1"},
	}
	for _, tt := range tests {
		got := commaInt(tt.n)
		if got != tt.want {
			t.Errorf("commaInt(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestIndexCaseInsensitive(t *testing.T) {
	tests := []struct {
		haystack string
		needle   string
		want     int
	}{
		{"<BODY>content</BODY>", "<body", 0},
		{"<html><Body>text", "<body", 6},
		{"no match here", "<body", -1},
		{"<BODY class='x'>", "<body", 0},
		{"</BODY>", "</body", 0},
	}
	for _, tt := range tests {
		got := indexCaseInsensitive([]byte(tt.haystack), []byte(tt.needle))
		if got != tt.want {
			t.Errorf("indexCaseInsensitive(%q, %q) = %d, want %d", tt.haystack, tt.needle, got, tt.want)
		}
	}
}

func TestInjectBars(t *testing.T) {
	body := []byte("<html><body><p>Hello</p></body></html>")
	header := []byte("<nav>bar</nav>")
	footer := []byte("<footer>foot</footer>")

	result := injectBars(body, header, footer)
	got := string(result)

	// Header should appear after <body>.
	if want := "<body><nav>bar</nav><p>Hello</p>"; !containsString(got, want) {
		t.Errorf("header not injected correctly in %q", got)
	}
	// Footer should appear before </body>.
	if want := "<footer>foot</footer></body>"; !containsString(got, want) {
		t.Errorf("footer not injected correctly in %q", got)
	}
}

func TestInjectBarsNoBodyTag(t *testing.T) {
	body := []byte("<p>content</p>")
	header := []byte("<nav>H</nav>")
	footer := []byte("<footer>F</footer>")

	result := injectBars(body, header, footer)
	got := string(result)

	// Should prepend header and append footer.
	if want := "<nav>H</nav><p>content</p><footer>F</footer>"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
