package oza

import (
	"testing"
)

func TestCJKQueryGrams(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // minimum expected gram count (0 = expect empty)
	}{
		{name: "empty", input: "", want: 0},
		{name: "single ASCII", input: "a", want: 0},
		{name: "two ASCII", input: "ab", want: 0},
		{name: "three ASCII", input: "abc", want: 1},
		{name: "four ASCII", input: "abcd", want: 2},

		// CJK characters are 3 bytes each in UTF-8.
		// Each produces a unigram; adjacent pairs produce bigrams.
		{name: "single CJK", input: "日", want: 1},                   // unigram only
		{name: "two CJK", input: "日本", want: 3},                     // 2 unigrams + 1 bigram
		{name: "three CJK", input: "日本語", want: 5},                  // 3 unigrams + 2 bigrams
		{name: "mixed CJK+Latin", input: "日本abc", want: 4},          // 2 unigrams + 1 bigram + 1 ASCII trigram
		{name: "Latin+CJK", input: "abcde日本", want: 6},              // 3 ASCII trigrams + 2 unigrams + 1 bigram
		{name: "CJK separated by Latin", input: "日abc本", want: 3},   // 2 unigrams + 1 ASCII trigram, no bigram (not adjacent)

		// Korean (Hangul) is also CJK range.
		{name: "Korean", input: "한국어", want: 5}, // 3 unigrams + 2 bigrams

		// Invalid UTF-8 should not panic. Non-CJK byte runs still produce
		// sliding-window trigrams from the raw bytes.
		{name: "invalid UTF-8", input: "\xff\xfe\xfd", want: 1},
		{name: "mixed invalid", input: "abc\xff\xfedef", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grams := cjkQueryGrams([]byte(tt.input))
			if tt.want == 0 && len(grams) != 0 {
				t.Errorf("expected no grams, got %d", len(grams))
			}
			if tt.want > 0 && len(grams) < tt.want {
				t.Errorf("expected at least %d grams, got %d", tt.want, len(grams))
			}
		})
	}
}

func TestCJKQueryGramsDedup(t *testing.T) {
	// Repeated characters should not produce duplicate grams.
	grams := cjkQueryGrams([]byte("日日日"))
	seen := make(map[[3]byte]bool)
	for _, g := range grams {
		if seen[g] {
			t.Errorf("duplicate gram %v", g)
		}
		seen[g] = true
	}
}

func TestCJKQueryGramsNoPanicOnAllInputs(t *testing.T) {
	// Ensure no panic on edge-case inputs.
	inputs := []string{
		"",
		"a",
		"ab",
		"\x00\x00\x00",
		"日",
		"日a",
		"a日",
		"日a本",
		string([]byte{0xe6, 0x97}), // truncated UTF-8 for 日
		"abc" + string([]byte{0xe6}) + "def",
	}
	for _, s := range inputs {
		cjkQueryGrams([]byte(s)) // must not panic
	}
}
