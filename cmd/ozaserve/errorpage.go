package main

import (
	"fmt"
	"net/http"
)

// writeErrorPage renders a styled full-page HTML error page matching the OZA design language.
func writeErrorPage(w http.ResponseWriter, r *http.Request, status int, heading, detail string) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)

	statusText := http.StatusText(status)
	icon := "&#x26A0;&#xFE0E;" // ⚠︎
	if status == http.StatusNotFound {
		icon = "&#x2205;" // ∅
	}

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%d %s — OZA</title>
`+faviconLink+`
<style>
*,*::before,*::after{box-sizing:border-box}
html,body{margin:0;padding:0;height:100%%;font-family:system-ui,sans-serif}
body{background:#f6f8fa;display:flex;flex-direction:column;align-items:center;justify-content:center;min-height:100%%;color:#24292f;text-align:center;padding:40px 24px}
.wordmark{font-size:1rem;color:#8c959f;margin-bottom:64px}
.wordmark a{color:#8c959f;text-decoration:none}
.wordmark a:hover{color:#0366d6}
.kanji{color:#C9A84C;font-weight:600}
.icon{font-size:4rem;line-height:1;margin-bottom:20px;color:#C9A84C;user-select:none}
.code{font-size:0.8rem;font-weight:600;letter-spacing:.1em;text-transform:uppercase;color:#8c959f;margin-bottom:12px}
h1{font-size:1.6rem;font-weight:600;margin:0 0 12px;color:#24292f}
p{font-size:1rem;color:#57606a;margin:0 0 36px;line-height:1.6;max-width:380px}
.actions{display:flex;gap:10px;justify-content:center;flex-wrap:wrap}
.btn{display:inline-block;padding:8px 20px;border-radius:6px;font-size:0.9rem;font-weight:500;text-decoration:none;border:1px solid #d0d7de;background:#fff;color:#24292f;cursor:pointer}
.btn:hover{background:#f3f4f6}
.btn-primary{background:#0366d6;border-color:#0366d6;color:#fff}
.btn-primary:hover{background:#0256b9;border-color:#0256b9}
</style></head><body>
<div class="wordmark"><a href="/"><span class="kanji">&#x738B;&#x5EA7;</span> OZA Library</a></div>
<div class="icon">%s</div>
<div class="code">%d %s</div>
<h1>%s</h1>
<p>%s</p>
<div class="actions">
  <a class="btn btn-primary" href="/">Library home</a>
  <a class="btn" href="javascript:history.back()">Go back</a>
</div>
</body></html>
`, status, statusText, icon, status, statusText, heading, detail)
}

// write404 is a convenience wrapper for 404 Not Found pages.
func write404(w http.ResponseWriter, r *http.Request) {
	writeErrorPage(w, r, http.StatusNotFound,
		"Page not found",
		"The page you requested doesn&#x2019;t exist or has been moved.")
}

// write500 is a convenience wrapper for 500 Internal Server Error pages.
func write500(w http.ResponseWriter, r *http.Request) {
	writeErrorPage(w, r, http.StatusInternalServerError,
		"Something went wrong",
		"An internal error occurred. Please try again or return to the library.")
}
