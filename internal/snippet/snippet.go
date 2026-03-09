// Package snippet provides search-result snippet extraction for OZA entries.
// It strips HTML tags to produce plain text, then extracts a windowed excerpt
// centered on the query match location.
package snippet

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/stazelabs/oza/oza"
)

// blockElements contains HTML elements that imply a word boundary.
var blockElements = map[atom.Atom]bool{
	atom.Address: true, atom.Article: true, atom.Aside: true,
	atom.Blockquote: true, atom.Br: true, atom.Dd: true,
	atom.Details: true, atom.Dialog: true, atom.Div: true,
	atom.Dl: true, atom.Dt: true, atom.Fieldset: true,
	atom.Figcaption: true, atom.Figure: true, atom.Footer: true,
	atom.Form: true, atom.H1: true, atom.H2: true, atom.H3: true,
	atom.H4: true, atom.H5: true, atom.H6: true, atom.Header: true,
	atom.Hr: true, atom.Li: true, atom.Main: true, atom.Nav: true,
	atom.Ol: true, atom.P: true, atom.Pre: true, atom.Section: true,
	atom.Summary: true, atom.Table: true, atom.Tbody: true,
	atom.Td: true, atom.Tfoot: true, atom.Th: true, atom.Thead: true,
	atom.Tr: true, atom.Ul: true,
}

// StripHTML removes all HTML tags from s using the x/net/html tokenizer,
// returning plain text. Content inside <script> and <style> elements is
// omitted. Runs of whitespace are collapsed to single spaces. Block-level
// elements inject a space boundary so adjacent text nodes don't merge.
func StripHTML(s string) string {
	tok := html.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	skip := 0 // nesting depth inside script/style elements

	// ensureSpace adds a space separator if the last character isn't already one.
	ensureSpace := func() {
		if b.Len() > 0 {
			last, _ := utf8.DecodeLastRuneInString(b.String())
			if last != ' ' {
				b.WriteByte(' ')
			}
		}
	}

	for {
		tt := tok.Next()
		switch tt {
		case html.ErrorToken:
			return strings.TrimSpace(b.String())

		case html.StartTagToken, html.SelfClosingTagToken:
			tn, _ := tok.TagName()
			a := atom.Lookup(tn)
			if a == atom.Script || a == atom.Style {
				skip++
			}
			if blockElements[a] {
				ensureSpace()
			}

		case html.EndTagToken:
			tn, _ := tok.TagName()
			a := atom.Lookup(tn)
			if (a == atom.Script || a == atom.Style) && skip > 0 {
				skip--
			}
			if blockElements[a] {
				ensureSpace()
			}

		case html.TextToken:
			if skip > 0 {
				continue
			}
			text := string(tok.Text())
			for _, r := range text {
				if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
					ensureSpace()
				} else {
					b.WriteRune(r)
				}
			}
		}
	}
}

// Extract returns a snippet from text whose visible content (excluding
// ellipsis markers) is at most maxLen runes, centered on the first
// case-insensitive occurrence of query. If query is not found, the first
// maxLen runes are returned. An ellipsis ("…") is prepended or appended
// when the window is truncated.
func Extract(text, query string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}

	// Find query position (case-insensitive) in rune space.
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	byteIdx := strings.Index(lowerText, lowerQuery)

	var runeIdx int
	if byteIdx >= 0 {
		runeIdx = utf8.RuneCountInString(text[:byteIdx])
	} else {
		runeIdx = -1
	}

	var start, end int

	if runeIdx < 0 {
		// No match — return prefix.
		start = 0
		end = maxLen
		if end > len(runes) {
			end = len(runes)
		}
	} else {
		queryRunes := utf8.RuneCountInString(query)
		half := (maxLen - queryRunes) / 2
		if half < 0 {
			half = 0
		}
		start = runeIdx - half
		if start < 0 {
			start = 0
		}
		end = start + maxLen
		if end > len(runes) {
			end = len(runes)
			start = end - maxLen
			if start < 0 {
				start = 0
			}
		}
	}

	// Try to break at word boundaries, but only within a small margin
	// to avoid losing too much content (especially in CJK text with no spaces).
	const margin = 10
	if start > 0 {
		for i := start; i < start+margin && i < end; i++ {
			if runes[i] == ' ' {
				start = i + 1
				break
			}
		}
	}
	if end < len(runes) {
		for i := end; i > end-margin && i > start; i-- {
			if runes[i-1] == ' ' {
				end = i - 1
				break
			}
		}
	}

	prefix := ""
	suffix := ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(runes) {
		suffix = "…"
	}
	return prefix + string(runes[start:end]) + suffix
}

// headingLevel returns the heading level (1-6) for an atom, or 0 if not a heading.
func headingLevel(a atom.Atom) int {
	switch a {
	case atom.H1:
		return 1
	case atom.H2:
		return 2
	case atom.H3:
		return 3
	case atom.H4:
		return 4
	case atom.H5:
		return 5
	case atom.H6:
		return 6
	}
	return 0
}

// ExtractSection extracts the HTML content under the first heading whose text
// matches heading (case-insensitive). It returns everything from that heading
// up to the next heading of equal or higher level (or end of document).
// Returns "" if no matching heading is found.
func ExtractSection(htmlContent, heading string) string {
	tok := html.NewTokenizer(strings.NewReader(htmlContent))
	lowerHeading := strings.ToLower(strings.TrimSpace(heading))

	// Phase 1: find the matching heading and its level.
	var matchLevel int
	var startOffset int
	found := false

	// We track byte offsets via raw tokens.
	var rawLen int
	for {
		tt := tok.Next()
		if tt == html.ErrorToken {
			return ""
		}
		raw := tok.Raw()
		if tt == html.StartTagToken {
			tn, _ := tok.TagName()
			level := headingLevel(atom.Lookup(tn))
			if level > 0 {
				headingStart := rawLen
				// Collect text inside this heading.
				var headingText strings.Builder
				for {
					tt2 := tok.Next()
					raw2 := tok.Raw()
					rawLen += len(raw2)
					if tt2 == html.ErrorToken || tt2 == html.EndTagToken {
						tn2, _ := tok.TagName()
						if headingLevel(atom.Lookup(tn2)) == level {
							break
						}
					}
					if tt2 == html.TextToken {
						headingText.Write(tok.Text())
					}
				}
				if strings.ToLower(strings.TrimSpace(headingText.String())) == lowerHeading {
					matchLevel = level
					startOffset = headingStart
					found = true
					rawLen += len(raw)
					break
				}
			}
		}
		rawLen += len(raw)
	}
	if !found {
		return ""
	}

	// Phase 2: scan until a heading of equal or higher level (lower number).
	for {
		tt := tok.Next()
		if tt == html.ErrorToken {
			// End of document — take everything from startOffset.
			return strings.TrimSpace(htmlContent[startOffset:])
		}
		raw := tok.Raw()
		if tt == html.StartTagToken {
			tn, _ := tok.TagName()
			level := headingLevel(atom.Lookup(tn))
			if level > 0 && level <= matchLevel {
				endOffset := rawLen
				return strings.TrimSpace(htmlContent[startOffset:endOffset])
			}
		}
		rawLen += len(raw)
	}
}

// ForEntry generates a snippet for the given entry. Returns "" if the entry
// is not HTML, is a redirect, or if the content cannot be read.
func ForEntry(entry oza.Entry, query string, maxLen int) string {
	if entry.IsRedirect() {
		return ""
	}
	if entry.MIMEIndex() != oza.MIMEIndexHTML {
		return ""
	}
	content, err := entry.ReadContent()
	if err != nil {
		return ""
	}
	plain := StripHTML(string(content))
	return Extract(plain, query, maxLen)
}
