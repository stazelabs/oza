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
	"unicode"
	"unicode/utf8"

	"github.com/stazelabs/oza/oza"
)

type searchResult struct {
	Path       string `json:"path"`
	Title      string `json:"title"`
	TitleMatch bool   `json:"title_match,omitempty"`
}

// handleSearchAll serves GET /_search?q=... — global JSON search across all archives.
func (lib *library) handleSearchAll(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

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
			results = append(results, searchResult{
				Path:       entryHref(slug, sr.Entry.Path()),
				Title:      sr.Entry.Title(),
				TitleMatch: sr.TitleMatch,
			})
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

	var results []searchResult
	if ae.archive.HasSearch() {
		sResults, err := ae.archive.Search(q, oza.SearchOptions{Limit: 20})
		if err == nil {
			for _, sr := range sResults {
				results = append(results, searchResult{
					Path:       entryHref(slug, sr.Entry.Path()),
					Title:      sr.Entry.Title(),
					TitleMatch: sr.TitleMatch,
				})
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

	var results []searchResult
	if q != "" && ae.archive.HasSearch() {
		sResults, err := ae.archive.Search(q, oza.SearchOptions{Limit: limit})
		if err == nil {
			for _, sr := range sResults {
				results = append(results, searchResult{
					Path:       entryHref(slug, sr.Entry.Path()),
					Title:      sr.Entry.Title(),
					TitleMatch: sr.TitleMatch,
				})
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>&#x738B;&#x5EA7; Search — %s</title>
`+faviconLink+`
<style>
body { font-family: system-ui, sans-serif; max-width: 1000px; margin: 40px auto; padding: 0 20px; }
h1 { border-bottom: 1px solid #ddd; padding-bottom: 10px; margin-bottom: 4px; }
h1 a { color: inherit; text-decoration: none; }
h2 { font-size: 1.15em; margin: 4px 0 16px; color: #333; }
form { margin-bottom: 20px; display: flex; gap: 8px; }
form input[type=text] { flex: 1; padding: 8px 12px; font-size: 1em; border: 1px solid #ccc; border-radius: 4px; }
form input[type=text]:focus { outline: none; border-color: #0366d6; }
form button { padding: 8px 16px; font-size: 1em; border: 1px solid #ccc; border-radius: 4px; cursor: pointer; }
.results a { display: block; padding: 8px 0; border-bottom: 1px solid #eee; color: #0366d6; text-decoration: none; }
.results a:hover { text-decoration: underline; }
.empty { color: #666; font-style: italic; }
.nav { margin-top: 10px; font-size: 0.9em; }
.nav a { color: #0366d6; }
</style></head><body>
<h1><a href="/"><span style="color:#C9A84C">&#x738B;&#x5EA7;</span> OZA</a></h1><h2>Search — <a href="/%s/">%s</a></h2>
<form method="get">
<input type="text" name="q" value="%s" placeholder="Search..." autofocus>
<button type="submit">Search</button>
</form>`,
		html.EscapeString(ae.title),
		html.EscapeString(slug), html.EscapeString(ae.title),
		html.EscapeString(q))

	if q != "" {
		if len(results) == 0 {
			fmt.Fprint(w, `<p class="empty">No results found.</p>`)
		} else {
			fmt.Fprintf(w, `<p>%d result(s):</p><div class="results">`, len(results))
			for _, res := range results {
				fmt.Fprintf(w, `<a href="%s">%s</a>`, html.EscapeString(res.Path), html.EscapeString(res.Title))
			}
			fmt.Fprint(w, `</div>`)
		}
	}

	fmt.Fprintf(w, `<div class="nav"><a href="/">Library</a> · <a href="/%s/">Back to main page</a> · <a href="/%s/-/browse">Browse</a> · <a href="/%s/-/random">Random</a></div>`,
		html.EscapeString(slug), html.EscapeString(slug), html.EscapeString(slug))
	fmt.Fprint(w, footerBarHTML(!lib.noInfo))
	fmt.Fprint(w, `</body></html>`)
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>&#x738B;&#x5EA7; Browse — %s</title>
`+faviconLink+`
<style>
body { font-family: system-ui, sans-serif; max-width: 1000px; margin: 40px auto; padding: 0 20px; }
h1 { border-bottom: 1px solid #ddd; padding-bottom: 10px; margin-bottom: 4px; }
h1 a { color: inherit; text-decoration: none; }
h2 { font-size: 1.15em; margin: 4px 0 16px; color: #333; }
.total { color: #666; font-size: 0.9em; margin: -6px 0 16px; }
.letters { display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 20px; }
.letters a { display: inline-block; padding: 6px 10px; border: 1px solid #ccc; border-radius: 4px; text-decoration: none; color: #0366d6; font-weight: bold; }
.letters a:hover { background: #f6f8fa; }
.letters a.active { background: #0366d6; color: white; border-color: #0366d6; }
.letters span { display: inline-block; padding: 6px 10px; border: 1px solid #eee; border-radius: 4px; color: #ccc; font-weight: bold; cursor: default; }
.letter-info { color: #666; font-size: 0.9em; margin-bottom: 12px; }
.entries a { display: block; padding: 6px 0; border-bottom: 1px solid #eee; color: #0366d6; text-decoration: none; }
.entries a:hover { text-decoration: underline; }
.pager { display: flex; align-items: center; gap: 16px; margin-top: 16px; font-size: 0.9em; color: #666; }
.pager a { color: #0366d6; text-decoration: none; }
.pager a:hover { text-decoration: underline; }
.nav { margin-top: 24px; font-size: 0.9em; }
.nav a { color: #0366d6; }
</style></head><body>
<h1><a href="/"><span style="color:#C9A84C">&#x738B;&#x5EA7;</span> OZA</a></h1><h2>Browse — <a href="/%s/">%s</a></h2>
<p class="total">%s entries total</p>
<div class="letters">`,
		html.EscapeString(ae.title),
		html.EscapeString(slug), html.EscapeString(ae.title),
		commaInt(int(ae.archive.EntryCount())))

	// A–Z letter bar — greyed out if no entries start with that letter.
	for c := byte('A'); c <= 'Z'; c++ {
		l := string(c)
		if ae.letterCounts[c] == 0 {
			fmt.Fprintf(w, `<span>%s</span>`, l)
		} else if letter == l {
			fmt.Fprintf(w, `<a href="/%s/-/browse?letter=%s" class="active">%s</a>`,
				html.EscapeString(slug), l, l)
		} else {
			fmt.Fprintf(w, `<a href="/%s/-/browse?letter=%s">%s</a>`,
				html.EscapeString(slug), l, l)
		}
	}
	hashClass := ""
	if letter == "#" {
		hashClass = ` class="active"`
	}
	fmt.Fprintf(w, `<a href="/%s/-/browse?letter=%%23"%s>#</a>`, html.EscapeString(slug), hashClass)
	fmt.Fprint(w, `</div>`)

	if letter != "" {
		// Collect all entries matching the selected letter via a full title-order scan.
		// This is O(N) but correct. A future optimisation would add prefix iteration
		// to the oza package.
		var entries []searchResult
		if letter == "#" {
			for e := range ae.archive.EntriesByTitle() {
				t := e.Title()
				if t == "" {
					continue
				}
				ru, _ := utf8.DecodeRuneInString(t)
				if !unicode.IsLetter(ru) {
					entries = append(entries, searchResult{
						Path:  entryHref(slug, e.Path()),
						Title: t,
					})
				}
			}
		} else {
			upper := strings.ToUpper(letter)
			lower := strings.ToLower(letter)
			for e := range ae.archive.EntriesByTitle() {
				t := e.Title()
				if len(t) == 0 {
					continue
				}
				first := string(t[0])
				if first == upper || first == lower {
					entries = append(entries, searchResult{
						Path:  entryHref(slug, e.Path()),
						Title: t,
					})
				}
			}
		}
		letterCount := len(entries)

		fmt.Fprintf(w, `<p class="letter-info">%s entries</p>`, commaInt(letterCount))
		fmt.Fprint(w, `<div class="entries">`)
		if letterCount == 0 || offset >= letterCount {
			fmt.Fprint(w, `<p style="color:#666;font-style:italic">No entries.</p>`)
		} else {
			end := offset + limit
			if end > len(entries) {
				end = len(entries)
			}
			for _, res := range entries[offset:end] {
				fmt.Fprintf(w, `<a href="%s">%s</a>`,
					html.EscapeString(res.Path), html.EscapeString(res.Title))
			}
		}
		fmt.Fprint(w, `</div>`)

		if letterCount > 0 {
			pageEnd := offset + limit
			if pageEnd > letterCount {
				pageEnd = letterCount
			}
			fmt.Fprint(w, `<div class="pager">`)
			if offset > 0 {
				prev := offset - limit
				if prev < 0 {
					prev = 0
				}
				fmt.Fprintf(w, `<a href="/%s/-/browse?letter=%s&amp;offset=%d&amp;limit=%d">&#8592; Previous</a>`,
					html.EscapeString(slug), html.EscapeString(letter), prev, limit)
			}
			fmt.Fprintf(w, `<span>%s&#x2013;%s of %s</span>`,
				commaInt(offset+1), commaInt(pageEnd), commaInt(letterCount))
			if offset+limit < letterCount {
				fmt.Fprintf(w, `<a href="/%s/-/browse?letter=%s&amp;offset=%d&amp;limit=%d">Next &#8594;</a>`,
					html.EscapeString(slug), html.EscapeString(letter), offset+limit, limit)
			}
			fmt.Fprint(w, `</div>`)
		}
	}

	fmt.Fprintf(w, `<div class="nav"><a href="/">Library</a> · <a href="/%s/">Back to main page</a> · <a href="/%s/-/search">Search</a> · <a href="/%s/-/random">Random</a></div>`,
		html.EscapeString(slug), html.EscapeString(slug), html.EscapeString(slug))
	fmt.Fprint(w, footerBarHTML(!lib.noInfo))
	fmt.Fprint(w, `</body></html>`)
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

	// For HTML content, inject the sticky navigation bar and footer bar.
	if entry.MIMEIndex() == oza.MIMEIndexHTML {
		bar := headerBarHTML(slug, ae.title, ae.letterCounts)
		content = injectHeaderBar(content, []byte(bar))
		content = injectFooterBar(content, []byte(footerBarHTML(!lib.noInfo)))
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
	w.Write(content)
}

// injectHeaderBar inserts bar after the opening <body...> tag.
// Falls back to prepending if no <body> tag is found.
func injectHeaderBar(body, bar []byte) []byte {
	lower := bytes.ToLower(body)
	idx := bytes.Index(lower, []byte("<body"))
	if idx == -1 {
		return append(bar, body...)
	}
	closeIdx := bytes.IndexByte(body[idx:], '>')
	if closeIdx == -1 {
		return append(bar, body...)
	}
	insertAt := idx + closeIdx + 1
	result := make([]byte, 0, len(body)+len(bar))
	result = append(result, body[:insertAt]...)
	result = append(result, bar...)
	result = append(result, body[insertAt:]...)
	return result
}

// injectFooterBar inserts bar before the closing </body> tag.
// Falls back to appending if no </body> tag is found.
func injectFooterBar(body, bar []byte) []byte {
	lower := bytes.ToLower(body)
	idx := bytes.Index(lower, []byte("</body"))
	if idx == -1 {
		return append(body, bar...)
	}
	result := make([]byte, 0, len(body)+len(bar))
	result = append(result, body[:idx]...)
	result = append(result, bar...)
	result = append(result, body[idx:]...)
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
#oza-bar{position:sticky;top:0;z-index:999999;background:#f6f8fa;border-bottom:1px solid #d0d7de;padding:4px 12px;font:13px/1.4 system-ui,sans-serif;display:flex;align-items:center;gap:10px;flex-wrap:wrap;box-sizing:border-box}
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
