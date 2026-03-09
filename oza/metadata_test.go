package oza

import (
	"errors"
	"testing"
)

func makeTestMetadata() map[string][]byte {
	return map[string][]byte{
		"title":    []byte("Test Archive"),
		"language": []byte("en"),
		"creator":  []byte("Test"),
		"date":     []byte("2026-01-01"),
		"source":   []byte("https://example.com"),
		"extra":    []byte("some extra value"),
	}
}

func TestParseMetadataRoundTrip(t *testing.T) {
	orig := makeTestMetadata()
	data, err := MarshalMetadata(orig)
	if err != nil {
		t.Fatalf("MarshalMetadata: %v", err)
	}

	got, err := ParseMetadata(data)
	if err != nil {
		t.Fatalf("ParseMetadata: %v", err)
	}

	if len(got) != len(orig) {
		t.Fatalf("got %d pairs, want %d", len(got), len(orig))
	}
	for k, wantVal := range orig {
		gotVal, ok := got[k]
		if !ok {
			t.Errorf("missing key %q", k)
			continue
		}
		if string(gotVal) != string(wantVal) {
			t.Errorf("key %q: got %q, want %q", k, gotVal, wantVal)
		}
	}
}

func TestValidateMetadataMissingKey(t *testing.T) {
	m := makeTestMetadata()
	delete(m, "title")

	err := ValidateMetadata(m)
	if !errors.Is(err, ErrMissingMetadata) {
		t.Errorf("expected ErrMissingMetadata, got %v", err)
	}
}

func TestValidateMetadataAllPresent(t *testing.T) {
	m := makeTestMetadata()
	if err := ValidateMetadata(m); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMetadataEmptyValue(t *testing.T) {
	m := map[string][]byte{
		"title":    []byte(""),
		"language": []byte("en"),
		"creator":  []byte("x"),
		"date":     []byte("2026-01-01"),
		"source":   []byte("src"),
	}

	data, err := MarshalMetadata(m)
	if err != nil {
		t.Fatalf("MarshalMetadata: %v", err)
	}

	got, err := ParseMetadata(data)
	if err != nil {
		t.Fatalf("ParseMetadata: %v", err)
	}

	if string(got["title"]) != "" {
		t.Errorf("empty value: got %q, want %q", got["title"], "")
	}
}

func FuzzParseMetadata(f *testing.F) {
	data, _ := MarshalMetadata(makeTestMetadata())
	f.Add(data)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseMetadata(data)
	})
}
