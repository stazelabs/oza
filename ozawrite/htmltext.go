package ozawrite

import (
	"bytes"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/html"
)

// extractVisibleText returns only the visible text content from HTML,
// stripping all tags and removing script/style element content entirely.
// A single space is inserted between text segments to prevent word
// concatenation across element boundaries.
func extractVisibleText(htmlContent []byte) []byte {
	if len(htmlContent) == 0 {
		return htmlContent
	}

	l := html.NewLexer(parse.NewInputBytes(htmlContent))
	var buf bytes.Buffer
	skip := false

	for {
		tt, _ := l.Next()
		switch tt {
		case html.ErrorToken:
			return bytes.TrimSpace(buf.Bytes())
		case html.StartTagToken, html.StartTagVoidToken:
			h := html.ToHash(l.Text())
			if h == html.Script || h == html.Style {
				skip = true
			}
		case html.EndTagToken:
			h := html.ToHash(l.Text())
			if h == html.Script || h == html.Style {
				skip = false
			}
		case html.TextToken:
			if !skip {
				text := l.Text()
				if len(bytes.TrimSpace(text)) > 0 {
					if buf.Len() > 0 {
						buf.WriteByte(' ')
					}
					buf.Write(text)
				}
			}
		}
	}
}
