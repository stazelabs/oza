package ozawrite

import (
	"testing"
)

func TestExtractVisibleText(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "basic tag stripping",
			html: "<p>Hello <b>world</b></p>",
			want: "Hello  world",
		},
		{
			name: "script removal",
			html: "<p>text</p><script>var x=1;</script><p>more</p>",
			want: "text more",
		},
		{
			name: "style removal",
			html: `<style>.foo{color:red}</style><p>visible</p>`,
			want: "visible",
		},
		{
			name: "nested tags",
			html: `<div><span><a href="x">link</a></span></div>`,
			want: "link",
		},
		{
			name: "attributes stripped",
			html: `<p class="foo" id="bar">text</p>`,
			want: "text",
		},
		{
			name: "empty input",
			html: "",
			want: "",
		},
		{
			name: "plain text",
			html: "just text",
			want: "just text",
		},
		{
			name: "doctype and comments",
			html: `<!DOCTYPE html><!-- comment --><p>text</p>`,
			want: "text",
		},
		{
			name: "multiple elements separated by space",
			html: "<p>one</p><p>two</p><p>three</p>",
			want: "one two three",
		},
		{
			name: "script with attributes",
			html: `<script type="text/javascript">alert('hi')</script><p>safe</p>`,
			want: "safe",
		},
		{
			name: "inline style in head",
			html: `<html><head><style>body{margin:0}</style></head><body><p>content</p></body></html>`,
			want: "content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(extractVisibleText([]byte(tt.html)))
			if got != tt.want {
				t.Errorf("extractVisibleText(%q) = %q, want %q", tt.html, got, tt.want)
			}
		})
	}
}
