package main

import (
	"fmt"
	"html"
	"net/http"
)

// writeIndexPage renders the library index: a sortable table of archives with
// instant AJAX search, a ZIM dropdown, and a random article button.
func (lib *library) writeIndexPage(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; form-action 'none'")

	fmt.Fprint(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>&#x738B;&#x5EA7; OZA Library</title>
`+faviconLink+`
<style>
body { font-family: system-ui, sans-serif; max-width: 1000px; margin: 40px auto; padding: 0 20px; }
h1 { border-bottom: 1px solid #ddd; padding-bottom: 10px; }
.search-wrap { position: relative; margin-bottom: 20px; }
.search-box { display: flex; gap: 8px; align-items: stretch; }
.search-box input, .search-box select, .search-box button { padding: 8px 12px; font-size: 1em; border: 1px solid #ccc; border-radius: 4px; box-sizing: border-box; }
.search-box input { flex: 1; }
.search-box input:focus { outline: none; border-color: #0366d6; box-shadow: 0 0 0 2px rgba(3,102,214,0.15); }
.search-box button { cursor: pointer; background: #f6f8fa; white-space: nowrap; }
.search-box button:hover { background: #e1e4e8; }
#search-results { position: absolute; top: 100%; left: 0; right: 0; z-index: 100; background: white; border: 1px solid #ddd; border-top: none; border-radius: 0 0 4px 4px; box-shadow: 0 4px 12px rgba(0,0,0,0.15); max-height: 400px; overflow-y: auto; display: none; }
#search-results.active { display: block; }
#search-results a { display: block; padding: 8px 12px; border-bottom: 1px solid #eee; color: #0366d6; text-decoration: none; }
#search-results a:hover { background: #f6f8fa; text-decoration: underline; }
#search-results .empty { color: #666; padding: 8px 12px; font-style: italic; }
#search-results .loading { color: #666; padding: 8px 12px; }
#search-results .loading::after { content: ''; display: inline-block; width: 12px; height: 12px; border: 2px solid #ccc; border-top-color: #0366d6; border-radius: 50%; animation: spin 0.6s linear infinite; margin-left: 6px; vertical-align: middle; }
@keyframes spin { to { transform: rotate(360deg); } }
table { width: 100%; border-collapse: collapse; table-layout: fixed; }
col.title { width: 30%; }
col.date  { width: 10%; }
col.count { width: 10%; }
th { text-align: left; padding: 8px 10px; border-bottom: 2px solid #ddd; cursor: pointer; user-select: none; white-space: nowrap; }
th:hover { background: #f6f8fa; }
th.sorted { color: #0366d6; }
td { padding: 8px 10px; border-bottom: 1px solid #eee; vertical-align: top; overflow-wrap: break-word; }
td.num { text-align: right; white-space: nowrap; }
td.date { white-space: nowrap; }
th.num { text-align: right; }
a { text-decoration: none; color: #0366d6; }
a:hover { text-decoration: underline; }
.sub { color: #666; font-size: 0.82em; margin-top: 2px; }
.actions { margin-top: 4px; }
.arrow { font-size: 0.75em; margin-left: 4px; }
</style></head><body>
<h1><span style="color:#C9A84C">&#x738B;&#x5EA7;</span> OZA Library</h1>
<div class="search-wrap">
<div class="search-box">
<input type="text" id="search-input" placeholder="Search articles..." autocomplete="off">
<select id="search-archive"`)

	if len(lib.slugs) == 1 {
		fmt.Fprint(w, ` style="display:none"`)
	}
	fmt.Fprint(w, `>`)
	if len(lib.slugs) > 1 {
		fmt.Fprint(w, `<option value="_all">All</option>`)
	}
	for _, slug := range lib.slugs {
		e := lib.archives[slug]
		fmt.Fprintf(w, `<option value="%s">%s</option>`,
			html.EscapeString(slug), html.EscapeString(e.title))
	}
	fmt.Fprint(w, `</select>
<button type="button" id="random-btn" title="Random article">Random</button>
</div>
<div id="search-results"></div>
</div>
<table>
<colgroup><col class="title"><col><col class="date"><col class="count">`)
	if !lib.noInfo {
		fmt.Fprint(w, `<col style="width:40px">`)
	}
	fmt.Fprint(w, `</colgroup>
<thead><tr>
<th data-col="0">Title<span class="arrow"></span></th>
<th data-col="1">File<span class="arrow"></span></th>
<th data-col="2">Date<span class="arrow"></span></th>
<th data-col="3" class="num">Entries<span class="arrow"></span></th>`)
	if !lib.noInfo {
		fmt.Fprint(w, `<th></th>`)
	}
	fmt.Fprint(w, `
</tr></thead>
<tbody>`)

	for _, slug := range lib.slugs {
		e := lib.archives[slug]

		// Title cell: link + optional description subtitle + action links.
		titleCell := fmt.Sprintf(`<a href="/%s/">%s</a>`,
			html.EscapeString(slug), html.EscapeString(e.title))
		if e.description != "" {
			titleCell += fmt.Sprintf(`<div class="sub">%s</div>`, html.EscapeString(e.description))
		}
		titleCell += fmt.Sprintf(
			`<div class="sub actions"><a href="/%s/-/browse">Browse</a> · <a href="/%s/-/search">Search</a> · <a href="/%s/-/random">Random</a></div>`,
			html.EscapeString(slug), html.EscapeString(slug), html.EscapeString(slug))

		// File cell: filename + optional description.
		fileCell := html.EscapeString(e.filename)

		// Date cell.
		dateVal := e.date
		dateDisplay := e.date
		if dateDisplay == "" {
			dateVal = ""
			dateDisplay = "&#x2014;" // em dash
		}

		infoCell := ""
		if !lib.noInfo {
			infoCell = fmt.Sprintf(`<td><a href="/%s/-/info" title="Archive info">&#x2139;&#xFE0E;</a></td>`,
				html.EscapeString(slug))
		}

		fmt.Fprintf(w, "<tr><td data-val=%q>%s</td><td data-val=%q>%s</td><td data-val=%q class=\"date\">%s</td><td data-val=%q class=\"num\">%d</td>%s</tr>\n",
			e.title, titleCell,
			e.filename, fileCell,
			dateVal, dateDisplay,
			fmt.Sprintf("%d", e.archive.EntryCount()), e.archive.EntryCount(),
			infoCell)
	}

	// Inline JS: sortable table + instant search with 200ms debounce + random button.
	fmt.Fprint(w, `</tbody></table>
<script>
(function(){
  var col = 0, asc = true;
  var ths = document.querySelectorAll('th[data-col]');
  var tbody = document.querySelector('tbody');
  function sort(c, a) {
    col = c; asc = a;
    ths.forEach(function(th, i) {
      var arrow = th.querySelector('.arrow');
      arrow.textContent = i === c ? (a ? ' \u25b2' : ' \u25bc') : '';
      th.classList.toggle('sorted', i === c);
    });
    var rows = Array.from(tbody.rows);
    rows.sort(function(ra, rb) {
      var av = ra.cells[c].dataset.val;
      var bv = rb.cells[c].dataset.val;
      var cmp = c === 3 ? +av - +bv : av.toLowerCase().localeCompare(bv.toLowerCase());
      return a ? cmp : -cmp;
    });
    rows.forEach(function(r){ tbody.appendChild(r); });
  }
  ths.forEach(function(th, i){
    th.addEventListener('click', function(){ sort(i, col === i ? !asc : true); });
  });
  sort(0, true);

  // Instant search
  var input = document.getElementById('search-input');
  var archiveSelect = document.getElementById('search-archive');
  var resultsDiv = document.getElementById('search-results');
  var timer = null;
  var activeReq = 0;
  function showResults() { resultsDiv.classList.add('active'); }
  function hideResults() { resultsDiv.classList.remove('active'); }
  input.addEventListener('input', function(){
    clearTimeout(timer);
    var q = input.value.trim();
    if (!q) { resultsDiv.innerHTML = ''; hideResults(); return; }
    timer = setTimeout(function(){
      var slug = archiveSelect.value;
      var url = slug === '_all'
        ? '/_search?q=' + encodeURIComponent(q)
        : '/' + encodeURIComponent(slug) + '/_search?q=' + encodeURIComponent(q);
      var reqId = ++activeReq;
      resultsDiv.innerHTML = '<div class="loading">Searching</div>';
      showResults();
      fetch(url)
        .then(function(r){ return r.json(); })
        .then(function(data){
          if (reqId !== activeReq) return;
          if (!data.length) {
            resultsDiv.innerHTML = '<div class="empty">No results</div>';
            return;
          }
          resultsDiv.innerHTML = data.map(function(r){
            var el = document.createElement('a');
            el.href = r.path;
            el.textContent = r.title;
            return el.outerHTML;
          }).join('');
        })
        .catch(function(){
          if (reqId !== activeReq) return;
          resultsDiv.innerHTML = '<div class="empty">Search failed</div>';
        });
    }, 200);
  });
  input.addEventListener('focus', function(){
    if (resultsDiv.innerHTML) showResults();
  });
  document.addEventListener('mousedown', function(e){
    if (!e.target.closest('.search-wrap')) hideResults();
  });
  // Random button
  document.getElementById('random-btn').addEventListener('click', function(){
    var slug = archiveSelect.value;
    var url = slug === '_all' ? '/_random' : '/' + encodeURIComponent(slug) + '/-/random';
    window.location.href = url;
  });
})();
</script>
`)
	fmt.Fprint(w, footerBarHTML())
	fmt.Fprint(w, `</body></html>`)
}
