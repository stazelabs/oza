package oza

import (
	"bytes"
	"testing"

	"github.com/andybalholm/brotli"
)

func TestRoundTrip_Brotli(t *testing.T) {
	raw := []byte("the quick brown fox jumps over the lazy dog")

	var buf bytes.Buffer
	bw := brotli.NewWriterLevel(&buf, 4)
	if _, err := bw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := bw.Close(); err != nil {
		t.Fatal(err)
	}
	compressed := buf.Bytes()

	got, err := decompressBytes(compressed, CompBrotli, 0, nil)
	if err != nil {
		t.Fatalf("decompressBytes(CompBrotli): %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Errorf("Brotli round-trip mismatch: got %q, want %q", got, raw)
	}
}
