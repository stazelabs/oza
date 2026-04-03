package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stazelabs/oza/oza"
)

// ---------- helpers ----------

// createFixture writes a file into dir, creating intermediate directories.
func createFixture(t *testing.T, dir, relPath, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func convertDir(t *testing.T, inputDir string, opts ConvertOptions) string {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "output.oza")
	c, err := NewConverter(inputDir, outPath, opts)
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	if err := c.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return outPath
}

func openOZA(t *testing.T, path string) *oza.Archive {
	t.Helper()
	a, err := oza.Open(path)
	if err != nil {
		t.Fatalf("oza.Open: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}

func fastOpts() ConvertOptions {
	return ConvertOptions{
		ZstdLevel:   3,
		DictSamples: 100,
		ChunkSize:   512 * 1024,
		TrainDict:   false,
		BuildSearch: false,
	}
}

// ---------- unit tests ----------

func TestDetectMIME(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"index.html", "text/html"},
		{"style.css", "text/css"},
		{"app.js", "text/javascript"},
		{"logo.png", "image/png"},
		{"photo.jpg", "image/jpeg"},
		{"icon.svg", "image/svg+xml"},
		{"docs/guide.md", "text/markdown; charset=utf-8"},
		{"docs/guide.markdown", "text/markdown; charset=utf-8"},
		{"font.woff2", "font/woff2"},
		{"font.woff", "font/woff"},
		{"data.json", "application/json"},
		{"manifest.webmanifest", "application/manifest+json"},
		{"image.webp", "image/webp"},
		{"image.avif", "image/avif"},
		{"favicon.ico", "image/x-icon"},
		{"unknown.qqq", "application/octet-stream"},
	}

	for _, tt := range tests {
		got := detectMIME(tt.path)
		// mime.TypeByExtension may return charset params; just check prefix.
		if !strings.HasPrefix(got, strings.Split(tt.want, ";")[0]) {
			t.Errorf("detectMIME(%q) = %q, want prefix %q", tt.path, got, tt.want)
		}
	}
}

func TestExtractHTMLTitle(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{"title tag", "<html><head><title>Hello World</title></head><body></body></html>", "Hello World"},
		{"h1 fallback", "<html><body><h1>My Page</h1></body></html>", "My Page"},
		{"no title", "<html><body><p>content</p></body></html>", ""},
		{"whitespace", "<html><head><title>  Trimmed  </title></head></html>", "Trimmed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHTMLTitle([]byte(tt.html))
			if got != tt.want {
				t.Errorf("extractHTMLTitle = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractMDTitle(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want string
	}{
		{"standard", "# Getting Started\n\nSome text.", "Getting Started"},
		{"frontmatter title", "---\ntitle: Frontmatter Title\n---\n\nBody text.", "Frontmatter Title"},
		{"frontmatter quoted", "---\ntitle: \"Quoted Title\"\n---\n\nBody.", "Quoted Title"},
		{"heading over frontmatter", "---\ntitle: FM Title\n---\n# Heading Title\n\nBody.", "FM Title"},
		{"no heading", "Just some text.\n\nMore text.", ""},
		{"h2 not matched", "## Subsection\n\nText.", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMDTitle([]byte(tt.md))
			if got != tt.want {
				t.Errorf("extractMDTitle = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMainEntryDetection(t *testing.T) {
	tests := []struct {
		name    string
		entries []siteEntry
		want    string
	}{
		{
			"index.html wins",
			[]siteEntry{
				{RelPath: "about.html"}, {RelPath: "index.html"},
			},
			"index.html",
		},
		{
			"index.md fallback",
			[]siteEntry{
				{RelPath: "about.html"}, {RelPath: "index.md"},
			},
			"index.md",
		},
		{
			"README.md fallback",
			[]siteEntry{
				{RelPath: "about.html"}, {RelPath: "README.md"},
			},
			"README.md",
		},
		{
			"first root HTML",
			[]siteEntry{
				{RelPath: "docs/intro.html"}, {RelPath: "about.html"}, {RelPath: "zebra.html"},
			},
			"about.html",
		},
		{
			"no HTML",
			[]siteEntry{
				{RelPath: "style.css"}, {RelPath: "logo.png"},
			},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMainEntry(tt.entries)
			if got != tt.want {
				t.Errorf("detectMainEntry = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------- integration tests (programmatic fixtures) ----------

func TestConvert(t *testing.T) {
	dir := t.TempDir()

	createFixture(t, dir, "index.html", `<!DOCTYPE html>
<html><head><title>Home</title></head>
<body><h1>Welcome</h1><p>Hello world.</p><a href="about.html">About</a></body></html>`)

	createFixture(t, dir, "about.html", `<!DOCTYPE html>
<html><head><title>About</title></head>
<body><h1>About Us</h1><p>We are OZA.</p></body></html>`)

	createFixture(t, dir, "style.css", `body { font-family: sans-serif; }`)

	// 1x1 red PNG (minimal valid PNG).
	createFixture(t, dir, "images/logo.png", "\x89PNG\r\n\x1a\n")

	outPath := convertDir(t, dir, fastOpts())
	a := openOZA(t, outPath)

	if a.EntryCount() == 0 {
		t.Fatal("OZA has zero entries")
	}

	// Verify index.html exists.
	e, err := a.EntryByPath("index.html")
	if err != nil {
		t.Fatalf("EntryByPath(index.html): %v", err)
	}
	content, err := e.ReadContent()
	if err != nil {
		t.Fatalf("ReadContent: %v", err)
	}
	if !strings.Contains(string(content), "Welcome") {
		t.Error("index.html content does not contain 'Welcome'")
	}

	// Verify metadata.
	mainEntry, err := a.Metadata("main_entry")
	if err != nil {
		t.Fatalf("Metadata(main_entry): %v", err)
	}
	if mainEntry != "index.html" {
		t.Errorf("main_entry = %q, want index.html", mainEntry)
	}

	converter, err := a.Metadata("converter")
	if err != nil {
		t.Fatalf("Metadata(converter): %v", err)
	}
	if converter != "site2oza" {
		t.Errorf("converter = %q, want site2oza", converter)
	}

	// Verify checksums.
	results, err := a.VerifyAll()
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("Checksum failed for %s", r.ID)
		}
	}
}

func TestConvertMDInclusion(t *testing.T) {
	dir := t.TempDir()

	createFixture(t, dir, "docs/guide.md", "# User Guide\n\nThis is the guide.\n")
	createFixture(t, dir, "docs/setup.md", "---\ntitle: Setup Instructions\n---\n\nHow to set up.\n")

	outPath := convertDir(t, dir, fastOpts())
	a := openOZA(t, outPath)

	// guide.md should be stored as-is (not rendered to HTML).
	e, err := a.EntryByPath("docs/guide.md")
	if err != nil {
		t.Fatalf("EntryByPath(docs/guide.md): %v", err)
	}
	content, err := e.ReadContent()
	if err != nil {
		t.Fatalf("ReadContent: %v", err)
	}
	if !strings.Contains(string(content), "# User Guide") {
		t.Error("guide.md should contain raw markdown, not rendered HTML")
	}

	// Title should be extracted from # heading.
	if e.Title() != "User Guide" {
		t.Errorf("guide.md title = %q, want 'User Guide'", e.Title())
	}

	// setup.md should have title from frontmatter.
	e2, err := a.EntryByPath("docs/setup.md")
	if err != nil {
		t.Fatalf("EntryByPath(docs/setup.md): %v", err)
	}
	if e2.Title() != "Setup Instructions" {
		t.Errorf("setup.md title = %q, want 'Setup Instructions'", e2.Title())
	}

	// No .html versions should exist.
	if _, err := a.EntryByPath("docs/guide.html"); err == nil {
		t.Error("docs/guide.html should not exist — MD is stored natively")
	}
}

func TestConvertSkipHidden(t *testing.T) {
	dir := t.TempDir()

	createFixture(t, dir, "index.html", "<html><body>ok</body></html>")
	createFixture(t, dir, ".hidden", "secret")
	createFixture(t, dir, ".git/config", "[core]")

	outPath := convertDir(t, dir, fastOpts())
	a := openOZA(t, outPath)

	if _, err := a.EntryByPath(".hidden"); err == nil {
		t.Error(".hidden should be excluded")
	}
	if _, err := a.EntryByPath(".git/config"); err == nil {
		t.Error(".git/config should be excluded")
	}
}

func TestConvertExcludePattern(t *testing.T) {
	dir := t.TempDir()

	createFixture(t, dir, "index.html", "<html><body>ok</body></html>")
	createFixture(t, dir, "draft.html", "<html><body>draft</body></html>")
	createFixture(t, dir, "notes.draft.md", "# Draft\n")

	opts := fastOpts()
	opts.Exclude = []string{"*.draft.md", "draft.html"}
	outPath := convertDir(t, dir, opts)
	a := openOZA(t, outPath)

	if _, err := a.EntryByPath("draft.html"); err == nil {
		t.Error("draft.html should be excluded")
	}
	if _, err := a.EntryByPath("notes.draft.md"); err == nil {
		t.Error("notes.draft.md should be excluded")
	}
	if _, err := a.EntryByPath("index.html"); err != nil {
		t.Error("index.html should not be excluded")
	}
}

func TestConvertTOCGeneration(t *testing.T) {
	dir := t.TempDir()

	// Stub index.md (no links, like React's) — should trigger TOC generation.
	createFixture(t, dir, "index.md", "---\ntitle: Home\n---\n{/* stub */}\n")
	createFixture(t, dir, "docs/guide.md", "# User Guide\n\nHow to use the tool.\n")
	createFixture(t, dir, "docs/faq.md", "# FAQ\n\nFrequently asked questions.\n")
	createFixture(t, dir, "about.md", "# About\n\nAbout the project.\n")

	outPath := convertDir(t, dir, fastOpts())
	a := openOZA(t, outPath)

	e, err := a.EntryByPath("index.html")
	if err != nil {
		t.Fatalf("EntryByPath(index.html): %v", err)
	}
	content, err := e.ReadContent()
	if err != nil {
		t.Fatalf("ReadContent: %v", err)
	}
	html := string(content)

	// TOC should link to the .md files directly.
	if !strings.Contains(html, "docs/guide.md") {
		t.Error("TOC should contain link to docs/guide.md")
	}
	if !strings.Contains(html, "docs/faq.md") {
		t.Error("TOC should contain link to docs/faq.md")
	}
	if !strings.Contains(html, "about.md") {
		t.Error("TOC should contain link to about.md")
	}
	// Should have group headings.
	if !strings.Contains(html, "Docs") {
		t.Error("TOC should contain 'Docs' section heading")
	}
}

func TestConvertWithSearch(t *testing.T) {
	dir := t.TempDir()

	createFixture(t, dir, "index.html", `<!DOCTYPE html>
<html><head><title>Home</title></head>
<body><h1>Welcome</h1><p>Introduction to the homepage of our website.</p></body></html>`)

	createFixture(t, dir, "quantum.html", `<!DOCTYPE html>
<html><head><title>Quantum Computing</title></head>
<body><h1>Quantum Computing</h1><p>Advanced quantum entanglement techniques and superposition.</p></body></html>`)

	createFixture(t, dir, "classical.html", `<!DOCTYPE html>
<html><head><title>Classical Physics</title></head>
<body><h1>Classical Physics</h1><p>Newtonian mechanics and thermodynamics fundamentals.</p></body></html>`)

	opts := fastOpts()
	opts.BuildSearch = true
	opts.SearchPruneFreq = 0 // disable pruning for small test corpus
	outPath := convertDir(t, dir, opts)
	a := openOZA(t, outPath)

	results, err := a.Search("quantum", oza.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Error("Search('quantum') returned no results")
	}
}

// ---------- integration tests (real doc repos) ----------

func siteAvailable(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("testdata not found at %s; run 'make site-testdata' first", path)
	}
}

func TestConvertReactDocs(t *testing.T) {
	const testDir = "../../testdata/sites/react.dev"
	siteAvailable(t, testDir)

	opts := fastOpts()
	opts.BuildSearch = true
	opts.Title = "React Documentation"
	outPath := convertDir(t, testDir, opts)
	a := openOZA(t, outPath)

	if a.EntryCount() < 50 {
		t.Errorf("expected at least 50 entries, got %d", a.EntryCount())
	}

	// Verify search works.
	searchResults, err := a.Search("useState", oza.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(searchResults) == 0 {
		t.Error("Search('useState') returned no results in React docs")
	}

	// Verify metadata.
	title, err := a.Metadata("title")
	if err != nil {
		t.Fatalf("Metadata(title): %v", err)
	}
	if title != "React Documentation" {
		t.Errorf("title = %q, want 'React Documentation'", title)
	}

	// Verify checksums.
	results2, err := a.VerifyAll()
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}
	for _, r := range results2 {
		if !r.OK {
			t.Errorf("Checksum failed for %s", r.ID)
		}
	}
}

func TestConvertGoByExample(t *testing.T) {
	const testDir = "../../testdata/sites/gobyexample"
	siteAvailable(t, testDir)

	opts := fastOpts()
	opts.BuildSearch = true
	opts.Title = "Go by Example"
	outPath := convertDir(t, testDir, opts)
	a := openOZA(t, outPath)

	if a.EntryCount() < 10 {
		t.Errorf("expected at least 10 entries, got %d", a.EntryCount())
	}

	// Should be a pure HTML site — search for goroutines.
	searchResults, err := a.Search("goroutine", oza.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(searchResults) == 0 {
		t.Error("Search('goroutine') returned no results in Go by Example")
	}

	// Verify checksums.
	results2, err := a.VerifyAll()
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}
	for _, r := range results2 {
		if !r.OK {
			t.Errorf("Checksum failed for %s", r.ID)
		}
	}
}

func TestConvertGoDocs(t *testing.T) {
	const testDir = "../../testdata/sites/go-doc"
	siteAvailable(t, testDir)

	opts := fastOpts()
	opts.BuildSearch = true
	opts.Title = "Go Documentation"
	outPath := convertDir(t, testDir, opts)
	a := openOZA(t, outPath)

	if a.EntryCount() < 5 {
		t.Errorf("expected at least 5 entries, got %d", a.EntryCount())
	}

	// Verify checksums.
	results, err := a.VerifyAll()
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("Checksum failed for %s", r.ID)
		}
	}
}

// ---------- Tier 2 benchmark tests ----------

func TestBenchMDNConvert(t *testing.T) {
	const testDir = "../../testdata/sites/mdn-content"
	siteAvailable(t, testDir)

	if testing.Short() {
		t.Skip("skipping MDN benchmark in short mode")
	}

	opts := ConvertOptions{
		ZstdLevel:   6,
		DictSamples: 2000,
		ChunkSize:   4 * 1024 * 1024,
		TrainDict:   true,
		BuildSearch: true,
		Verbose:     true,
		Title:       "MDN Web Docs",
		Language:    "en",
		Source:      "https://developer.mozilla.org",
	}
	outPath := filepath.Join(t.TempDir(), "mdn.oza")
	c, err := NewConverter(testDir, outPath, opts)
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	if err := c.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	c.stats.Print(os.Stdout)

	a := openOZA(t, outPath)

	// MDN should have thousands of entries.
	if a.EntryCount() < 1000 {
		t.Errorf("expected at least 1000 entries, got %d", a.EntryCount())
	}

	// MCP use case: search for fetch API.
	searchResults, sErr := a.Search("fetch", oza.SearchOptions{Limit: 10})
	if sErr != nil {
		t.Fatalf("Search: %v", sErr)
	}
	if len(searchResults) == 0 {
		t.Error("Search('fetch') returned no results in MDN")
	}
	t.Logf("MDN: %d entries, output %s", a.EntryCount(), formatBytes(c.stats.OutputSize))
}

func TestBenchK8sConvert(t *testing.T) {
	const testDir = "../../testdata/sites/kubernetes-docs"
	siteAvailable(t, testDir)

	if testing.Short() {
		t.Skip("skipping K8s benchmark in short mode")
	}

	opts := ConvertOptions{
		ZstdLevel:   6,
		DictSamples: 2000,
		ChunkSize:   4 * 1024 * 1024,
		TrainDict:   true,
		BuildSearch: true,
		Verbose:     true,
		Title:       "Kubernetes Documentation",
		Language:    "en",
		Source:      "https://kubernetes.io/docs/",
	}
	outPath := filepath.Join(t.TempDir(), "k8s.oza")
	c, err := NewConverter(testDir, outPath, opts)
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	if err := c.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	c.stats.Print(os.Stdout)

	a := openOZA(t, outPath)

	// MCP use case: search for CronJob.
	searchResults, sErr := a.Search("CronJob", oza.SearchOptions{Limit: 10})
	if sErr != nil {
		t.Fatalf("Search: %v", sErr)
	}
	if len(searchResults) == 0 {
		t.Error("Search('CronJob') returned no results in K8s docs")
	}
	t.Logf("K8s: %d entries, output %s", a.EntryCount(), formatBytes(c.stats.OutputSize))
}

func TestBenchTerraformConvert(t *testing.T) {
	const testDir = "../../testdata/sites/terraform-aws"
	siteAvailable(t, testDir)

	if testing.Short() {
		t.Skip("skipping Terraform benchmark in short mode")
	}

	opts := ConvertOptions{
		ZstdLevel:   6,
		DictSamples: 2000,
		ChunkSize:   4 * 1024 * 1024,
		TrainDict:   true,
		BuildSearch: true,
		Verbose:     true,
		Title:       "Terraform AWS Provider",
		Language:    "en",
		Source:      "https://registry.terraform.io/providers/hashicorp/aws/latest/docs",
	}
	outPath := filepath.Join(t.TempDir(), "terraform-aws.oza")
	c, err := NewConverter(testDir, outPath, opts)
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	if err := c.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	c.stats.Print(os.Stdout)

	a := openOZA(t, outPath)

	// MCP use case: search for S3 bucket.
	searchResults, sErr := a.Search("aws_s3_bucket", oza.SearchOptions{Limit: 10})
	if sErr != nil {
		t.Fatalf("Search: %v", sErr)
	}
	if len(searchResults) == 0 {
		t.Error("Search('aws_s3_bucket') returned no results in Terraform docs")
	}
	t.Logf("Terraform: %d entries, output %s", a.EntryCount(), formatBytes(c.stats.OutputSize))
}
