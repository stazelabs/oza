package snippet

import (
	"strings"
	"testing"
)

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain text", "hello world", "hello world"},
		{"basic tags", "<p>hello</p> <b>world</b>", "hello world"},
		{"nested tags", "<div><p>hello <em>world</em></p></div>", "hello world"},
		{"script removed", "<p>before</p><script>var x=1;</script><p>after</p>", "before after"},
		{"style removed", "<p>before</p><style>.x{color:red}</style><p>after</p>", "before after"},
		{"whitespace collapse", "<p>  hello   \n  world  </p>", "hello world"},
		{"br and block elements", "<p>line one</p><p>line two</p>", "line one line two"},
		{"entities", "<p>a &amp; b &lt; c</p>", "a & b < c"},
		{"self-closing", "hello<br/>world", "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripHTML(tt.in)
			if got != tt.want {
				t.Errorf("StripHTML(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtract(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		query  string
		maxLen int
		want   string
	}{
		{
			name:   "short text no truncation",
			text:   "hello world",
			query:  "hello",
			maxLen: 200,
			want:   "hello world",
		},
		{
			name:   "match at start",
			text:   "alpha bravo charlie delta echo foxtrot golf hotel india",
			query:  "alpha",
			maxLen: 20,
			want:   "alpha bravo charlie…",
		},
		{
			name:   "match in middle",
			text:   "alpha bravo charlie delta echo foxtrot golf hotel india",
			query:  "delta",
			maxLen: 25,
			want:   "…charlie delta echo…",
		},
		{
			name:   "match at end",
			text:   "alpha bravo charlie delta echo foxtrot golf hotel india",
			query:  "india",
			maxLen: 20,
			want:   "…golf hotel india",
		},
		{
			name:   "no match fallback to prefix",
			text:   "alpha bravo charlie delta echo foxtrot golf hotel india",
			query:  "zzzz",
			maxLen: 15,
			want:   "alpha bravo…",
		},
		{
			name:   "unicode text",
			text:   "日本語のテキストです。検索テスト。",
			query:  "検索",
			maxLen: 10,
			want:   "…トです。検索テスト。",
		},
		{
			name:   "exact fit no ellipsis",
			text:   "exact",
			query:  "exact",
			maxLen: 5,
			want:   "exact",
		},
		{
			name:   "zero maxLen",
			text:   "hello world",
			query:  "hello",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "case insensitive match",
			text:   "The Quick Brown Fox Jumps Over The Lazy Dog",
			query:  "quick brown",
			maxLen: 20,
			want:   "The Quick Brown Fox…",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Extract(tt.text, tt.query, tt.maxLen)
			if got != tt.want {
				t.Errorf("Extract(%q, %q, %d) = %q, want %q", tt.text, tt.query, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestExtractSection(t *testing.T) {
	doc := `<html><body>
<h1>Title</h1>
<p>Intro paragraph.</p>
<h2>History</h2>
<p>History content here.</p>
<p>More history.</p>
<h2>Geography</h2>
<p>Geography content.</p>
<h3>Climate</h3>
<p>Climate details.</p>
<h2>Economy</h2>
<p>Economy content.</p>
</body></html>`

	tests := []struct {
		name    string
		heading string
		wantHas string // substring that must appear in result
		wantNot string // substring that must NOT appear in result
		empty   bool   // expect empty result
	}{
		{
			name:    "match h2 section",
			heading: "History",
			wantHas: "History content here.",
			wantNot: "Geography content.",
		},
		{
			name:    "match h2 with subsection",
			heading: "Geography",
			wantHas: "Climate details.",
			wantNot: "Economy content.",
		},
		{
			name:    "match h3 subsection only",
			heading: "Climate",
			wantHas: "Climate details.",
			wantNot: "Geography content.",
		},
		{
			name:    "case insensitive",
			heading: "history",
			wantHas: "History content here.",
		},
		{
			name:    "no match",
			heading: "Nonexistent",
			empty:   true,
		},
		{
			name:    "last section to end",
			heading: "Economy",
			wantHas: "Economy content.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSection(doc, tt.heading)
			if tt.empty {
				if got != "" {
					t.Errorf("ExtractSection(%q) = %q, want empty", tt.heading, got)
				}
				return
			}
			if got == "" {
				t.Fatalf("ExtractSection(%q) returned empty, want content", tt.heading)
			}
			if !contains(got, tt.wantHas) {
				t.Errorf("ExtractSection(%q) = %q, want substring %q", tt.heading, got, tt.wantHas)
			}
			if tt.wantNot != "" && contains(got, tt.wantNot) {
				t.Errorf("ExtractSection(%q) = %q, should NOT contain %q", tt.heading, got, tt.wantNot)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(s, substr)
}
