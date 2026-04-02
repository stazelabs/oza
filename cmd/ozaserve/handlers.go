package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"

	"github.com/stazelabs/oza/cmd/internal/snippet"
	"github.com/stazelabs/oza/oza"
)

type searchResult struct {
	Path       string `json:"path"`
	Title      string `json:"title"`
	TitleMatch bool   `json:"title_match,omitempty"`
	Snippet    string `json:"snippet,omitempty"`
}

type searchPageData struct {
	Slug       string
	Title      string
	Query      string
	Results    []searchResult
	FooterHTML string
}

type browseLetterLink struct {
	Label    string
	Active   bool
	Disabled bool
}

type browsePageData struct {
	Slug         string
	Title        string
	TotalEntries int
	Letters      []browseLetterLink
	Letter       string
	LetterCount  int
	Entries      []searchResult
	ShowPager    bool
	HasPrev      bool
	PrevOffset   int
	HasNext      bool
	NextOffset   int
	PageStart    int
	PageEnd      int
	Limit        int
	FooterHTML   string
}

// parseSnippetParams extracts snippet options from the request query string.
func parseSnippetParams(r *http.Request) (enabled bool, maxLen int) {
	enabled = r.URL.Query().Get("snippets") == "true"
	maxLen = 200
	if s := r.URL.Query().Get("snippet_length"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 500 {
			maxLen = n
		}
	}
	return
}

// handleSearchAll serves GET /_search?q=... — global JSON search across all archives.
func (lib *library) handleSearchAll(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	snippets, snippetLen := parseSnippetParams(r)
	limit := 20
	var results []searchResult
	for _, slug := range lib.slugs {
		ae := lib.archives[slug]
		if !ae.archive.HasSearch() {
			continue
		}
		sResults, err := ae.archive.Search(q, oza.SearchOptions{Limit: limit - len(results)})
		if err != nil {
			continue
		}
		for _, sr := range sResults {
			res := searchResult{
				Path:       entryHref(slug, sr.Entry.Path()),
				Title:      sr.Entry.Title(),
				TitleMatch: sr.TitleMatch,
			}
			if snippets {
				res.Snippet = snippet.ForEntry(sr.Entry, q, snippetLen)
			}
			results = append(results, res)
		}
		if len(results) >= limit {
			break
		}
	}
	if results == nil {
		results = []searchResult{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleSearchJSON serves GET /{archive}/_search?q=... — per-archive JSON search API.
func (lib *library) handleSearchJSON(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("archive")
	ae, ok := lib.archives[slug]
	if !ok {
		write404(w, r)
		return
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	snippets, snippetLen := parseSnippetParams(r)
	var results []searchResult
	if ae.archive.HasSearch() {
		sResults, err := ae.archive.Search(q, oza.SearchOptions{Limit: 20})
		if err == nil {
			for _, sr := range sResults {
				res := searchResult{
					Path:       entryHref(slug, sr.Entry.Path()),
					Title:      sr.Entry.Title(),
					TitleMatch: sr.TitleMatch,
				}
				if snippets {
					res.Snippet = snippet.ForEntry(sr.Entry, q, snippetLen)
				}
				results = append(results, res)
			}
		}
	}
	if results == nil {
		results = []searchResult{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleSearchPage serves GET /{archive}/-/search?q=...&limit=25[&format=json]
func (lib *library) handleSearchPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("archive")
	ae, ok := lib.archives[slug]
	if !ok {
		write404(w, r)
		return
	}

	q := r.URL.Query().Get("q")
	format := r.URL.Query().Get("format")
	limit := 25
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	snippets, snippetLen := parseSnippetParams(r)
	var results []searchResult
	if q != "" && ae.archive.HasSearch() {
		sResults, err := ae.archive.Search(q, oza.SearchOptions{Limit: limit})
		if err == nil {
			for _, sr := range sResults {
				res := searchResult{
					Path:       entryHref(slug, sr.Entry.Path()),
					Title:      sr.Entry.Title(),
					TitleMatch: sr.TitleMatch,
				}
				if snippets {
					res.Snippet = snippet.ForEntry(sr.Entry, q, snippetLen)
				}
				results = append(results, res)
			}
		}
	}

	if format == "json" {
		if results == nil {
			results = []searchResult{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
		return
	}

	renderTemplate(w, "search.html", searchPageData{
		Slug:       slug,
		Title:      ae.title,
		Query:      q,
		Results:    results,
		FooterHTML: footerBarHTML(!lib.noInfo),
	})
}

// handleRandom serves GET /{archive}/-/random — redirects to a random front article.
func (lib *library) handleRandom(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("archive")
	ae, ok := lib.archives[slug]
	if !ok {
		write404(w, r)
		return
	}

	if len(ae.frontArticleIDs) == 0 {
		writeErrorPage(w, r, http.StatusNotFound, "No articles available", "This archive has no readable articles.")
		return
	}
	id := ae.frontArticleIDs[rand.IntN(len(ae.frontArticleIDs))]
	e, err := ae.archive.EntryByID(id)
	if err != nil {
		log.Printf("error getting random article for %s: %v", slug, err)
		write500(w, r)
		return
	}
	http.Redirect(w, r, entryHref(slug, e.Path()), http.StatusFound)
}

// handleRandomAll serves GET /_random — picks a random archive, then a random article.
func (lib *library) handleRandomAll(w http.ResponseWriter, r *http.Request) {
	slug := lib.slugs[rand.IntN(len(lib.slugs))]
	ae := lib.archives[slug]

	if len(ae.frontArticleIDs) == 0 {
		writeErrorPage(w, r, http.StatusNotFound, "No articles available", "This archive has no readable articles.")
		return
	}
	id := ae.frontArticleIDs[rand.IntN(len(ae.frontArticleIDs))]
	e, err := ae.archive.EntryByID(id)
	if err != nil {
		log.Printf("error getting random article for %s: %v", slug, err)
		write500(w, r)
		return
	}
	http.Redirect(w, r, entryHref(slug, e.Path()), http.StatusFound)
}

// handleBrowse serves GET /{archive}/-/browse?letter=A[&offset=0&limit=50]
func (lib *library) handleBrowse(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("archive")
	ae, ok := lib.archives[slug]
	if !ok {
		write404(w, r)
		return
	}

	letter := r.URL.Query().Get("letter")
	offset, limit := 0, 50
	if s := r.URL.Query().Get("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			offset = n
		}
	}
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	// Build A–Z + # letter bar.
	letters := make([]browseLetterLink, 0, 27)
	for c := byte('A'); c <= 'Z'; c++ {
		letters = append(letters, browseLetterLink{
			Label:    string(c),
			Active:   letter == string(c),
			Disabled: ae.letterCounts[c] == 0,
		})
	}
	letters = append(letters, browseLetterLink{Label: "#", Active: letter == "#"})

	data := browsePageData{
		Slug:         slug,
		Title:        ae.title,
		TotalEntries: int(ae.archive.EntryCount()),
		Letters:      letters,
		Letter:       letter,
		Limit:        limit,
		FooterHTML:   footerBarHTML(!lib.noInfo),
	}

	if letter != "" {
		// Use the pre-computed letter-to-offset map to serve only the
		// requested page via BrowseTitles — O(page_size) instead of O(N).
		key := strings.ToUpper(letter)
		lr, ok := ae.letterOffsets[key]
		if !ok {
			lr = letterRange{}
		}

		letterCount := lr.count
		data.LetterCount = letterCount

		if letterCount > 0 && offset < letterCount {
			pageLimit := limit
			if offset+pageLimit > letterCount {
				pageLimit = letterCount - offset
			}
			page := ae.archive.BrowseTitles(lr.offset+offset, pageLimit)
			entries := make([]searchResult, 0, len(page))
			for _, e := range page {
				entries = append(entries, searchResult{
					Path:  entryHref(slug, e.Path()),
					Title: e.Title(),
				})
			}
			data.Entries = entries

			pageEnd := offset + len(entries)
			data.ShowPager = true
			data.PageStart = offset + 1
			data.PageEnd = pageEnd
			data.HasPrev = offset > 0
			if offset > 0 {
				prev := offset - limit
				if prev < 0 {
					prev = 0
				}
				data.PrevOffset = prev
			}
			data.HasNext = offset+limit < letterCount
			data.NextOffset = offset + limit
		}
	}

	renderTemplate(w, "browse.html", data)
}

// handleContent serves GET /{archive}/{path...}
// An empty path redirects to the archive's main entry.
func (lib *library) handleContent(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("archive")
	contentPath := r.PathValue("path")

	ae, ok := lib.archives[slug]
	if !ok {
		write404(w, r)
		return
	}

	// Empty path: redirect to main entry.
	if contentPath == "" {
		main, err := ae.archive.MainEntry()
		if err != nil {
			write404(w, r)
			return
		}
		resolved, err := main.Resolve()
		if err != nil {
			log.Printf("error resolving main entry for %s: %v", slug, err)
			write500(w, r)
			return
		}
		http.Redirect(w, r, entryHref(slug, resolved.Path()), http.StatusFound)
		return
	}

	entry, err := ae.archive.EntryByPath(contentPath)
	if err != nil {
		write404(w, r)
		return
	}

	// Follow archive-internal redirects as HTTP 302s.
	if entry.IsRedirect() {
		resolved, err := entry.Resolve()
		if err != nil {
			log.Printf("error resolving redirect for %s/%s: %v", slug, contentPath, err)
			write500(w, r)
			return
		}
		http.Redirect(w, r, entryHref(slug, resolved.Path()), http.StatusFound)
		return
	}

	// ETag / conditional request support.
	etag := makeETag(ae, contentPath)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	content, err := entry.ReadContent()
	if err != nil {
		log.Printf("error reading content for %s/%s: %v", slug, contentPath, err)
		write500(w, r)
		return
	}

	// Set MIME type. Append charset for text types that omit it — browsers may
	// guess wrong without it and OZA MIME types typically don't include charset.
	mime := entry.MIMEType()
	if mime == "" {
		mime = "application/octet-stream"
	}
	if strings.HasPrefix(mime, "text/") && !strings.Contains(mime, "charset") {
		mime += "; charset=utf-8"
	}

	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", etag)

	// For HTML content, set CSP sandbox and inject the sticky navigation bar and footer bar.
	if entry.MIMEIndex() == oza.MIMEIndexHTML {
		w.Header().Set("Content-Security-Policy", "sandbox")
		header := headerBarHTML(slug, ae.title, ae.letterCounts)
		content = injectBars(content, []byte(header), []byte(footerBarHTML(!lib.noInfo)))
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
	w.Write(content)
}

// indexCaseInsensitive finds the first occurrence of needle (assumed lowercase
// ASCII) in haystack, matching case-insensitively. Returns -1 if not found.
func indexCaseInsensitive(haystack, needle []byte) int {
	nLen := len(needle)
	for i := 0; i <= len(haystack)-nLen; i++ {
		match := true
		for j := 0; j < nLen; j++ {
			c := haystack[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 32
			}
			if c != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// injectBars inserts headerBar after the opening <body...> tag and footerBar
// before the closing </body> tag in a single pass, avoiding full-body lowercase
// copies. Falls back to prepending/appending if the tags are not found.
func injectBars(body, headerBar, footerBar []byte) []byte {
	// Find <body
	bodyOpenIdx := indexCaseInsensitive(body, []byte("<body"))
	var insertAfterBody int
	if bodyOpenIdx == -1 {
		insertAfterBody = 0
	} else {
		closeIdx := bytes.IndexByte(body[bodyOpenIdx:], '>')
		if closeIdx == -1 {
			insertAfterBody = 0
		} else {
			insertAfterBody = bodyOpenIdx + closeIdx + 1
		}
	}

	// Find </body — search only after the header insertion point.
	bodyCloseIdx := indexCaseInsensitive(body[insertAfterBody:], []byte("</body"))
	var insertBeforeClose int
	if bodyCloseIdx == -1 {
		insertBeforeClose = len(body)
	} else {
		insertBeforeClose = insertAfterBody + bodyCloseIdx
	}

	result := make([]byte, 0, len(body)+len(headerBar)+len(footerBar))
	result = append(result, body[:insertAfterBody]...)
	result = append(result, headerBar...)
	result = append(result, body[insertAfterBody:insertBeforeClose]...)
	result = append(result, footerBar...)
	result = append(result, body[insertBeforeClose:]...)
	return result
}

// footerBarHTML returns a self-contained HTML+CSS footer bar injected into every page.
// showInfoLink adds a link to the global server info page (/_info).
func footerBarHTML(showInfoLink bool) string {
	infoLink := ""
	if showInfoLink {
		infoLink = ` · <a href="/_info">Server info</a>`
	}
	return `<style>
#oza-footer{position:fixed;bottom:0;left:0;right:0;z-index:999998;background:#f6f8fa;border-top:1px solid #d0d7de;padding:4px 12px;font:12px/1.4 system-ui,sans-serif;display:flex;align-items:center;justify-content:center;gap:8px;color:#666}
#oza-footer a{color:#0366d6;text-decoration:none;display:inline-flex;align-items:center;gap:3px}
#oza-footer a:hover{text-decoration:underline}
#oza-footer .oza-kanji{color:#C9A84C;font-weight:600}
body{padding-bottom:32px!important}
</style>
<div id="oza-footer"><span class="oza-kanji">&#x738B;&#x5EA7;</span> <a href="https://github.com/stazelabs/oza"><svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path fill-rule="evenodd" d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>ozaserve</a><span>·</span><a href="https://github.com/stazelabs/oza/blob/main/LICENSE">Apache 2.0</a><span>·</span><a href="/_docs">Documentation</a>` + infoLink + `</div>`
}

// headerBarHTML returns a self-contained sticky navigation bar for HTML content pages.
func headerBarHTML(slug, title string, letterCounts map[byte]int) string {
	es := html.EscapeString(slug)
	et := html.EscapeString(title)

	var b strings.Builder
	b.WriteString(`<style>
#oza-bar{position:fixed;top:0;left:0;right:0;z-index:999999;background:#f6f8fa;border-bottom:1px solid #d0d7de;padding:4px 12px;font:13px/1.4 system-ui,sans-serif;display:flex;align-items:center;gap:10px;flex-wrap:wrap;box-sizing:border-box}
body{margin-top:32px!important}
#oza-bar *{box-sizing:border-box;margin:0;padding:0}
#oza-bar a{color:#0366d6;text-decoration:none}
#oza-bar a:hover{text-decoration:underline}
#oza-bar .oza-title{font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:200px}
#oza-bar .oza-sep{color:#d0d7de}
#oza-bar form{display:flex;gap:4px}
#oza-bar input[type=text]{padding:2px 6px;border:1px solid #d0d7de;border-radius:3px;font-size:13px;width:160px}
#oza-bar input[type=text]:focus{outline:none;border-color:#0366d6}
#oza-bar .oza-btn{padding:2px 8px;border:1px solid #d0d7de;border-radius:3px;background:#fff;font-size:13px;cursor:pointer;color:#0366d6;text-decoration:none;white-space:nowrap}
#oza-bar .oza-btn:hover{background:#f0f3f6;text-decoration:none}
#oza-bar .oza-az{display:flex;gap:2px;flex-wrap:wrap}
#oza-bar .oza-az a{padding:1px 4px;border-radius:2px;font-size:12px;font-weight:600}
#oza-bar .oza-az a:hover{background:#ddf4ff;text-decoration:none}
#oza-bar .oza-az span{padding:1px 4px;font-size:12px;font-weight:600;color:#ccc}
</style>`)

	b.WriteString(`<div id="oza-bar">`)
	b.WriteString(`<a href="/" style="color:#C9A84C;font-weight:600">&#x738B;&#x5EA7;</a>`)
	b.WriteString(`<span class="oza-sep">|</span>`)
	fmt.Fprintf(&b, `<a class="oza-title" href="/%s/">%s</a>`, es, et)
	fmt.Fprintf(&b, `<form action="/%s/-/search" method="get"><input type="text" name="q" placeholder="Search&#x2026;"><button class="oza-btn" type="submit">Search</button></form>`, es)
	fmt.Fprintf(&b, `<a class="oza-btn" href="/%s/-/random">Random</a>`, es)
	b.WriteString(`<span class="oza-sep">|</span><span class="oza-az">`)
	for c := byte('A'); c <= 'Z'; c++ {
		if letterCounts[c] > 0 {
			fmt.Fprintf(&b, `<a href="/%s/-/browse?letter=%c">%c</a>`, es, c, c)
		} else {
			fmt.Fprintf(&b, `<span>%c</span>`, c)
		}
	}
	b.WriteString(`</span></div>`)
	return b.String()
}

// commaInt formats n with comma thousands separators.
func commaInt(n int) string {
	s := strconv.Itoa(n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out)
}
