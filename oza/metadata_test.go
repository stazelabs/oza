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

func hasErrKey(errs []ValidationError, key string) bool {
	for _, e := range errs {
		if e.Key == key {
			return true
		}
	}
	return false
}

func TestValidateMetadataStrictValid(t *testing.T) {
	m := makeTestMetadata()
	if errs := ValidateMetadataStrict(m); len(errs) != 0 {
		t.Fatalf("expected no errors for valid metadata, got %v", errs)
	}
}

func TestValidateMetadataStrictRFC3339Date(t *testing.T) {
	m := makeTestMetadata()
	m["date"] = []byte("2025-03-15T10:30:00Z")
	if errs := ValidateMetadataStrict(m); len(errs) != 0 {
		t.Fatalf("expected no errors for RFC3339 date, got %v", errs)
	}
}

func TestValidateMetadataStrictBadDate(t *testing.T) {
	m := makeTestMetadata()
	m["date"] = []byte("March 15, 2025")
	if errs := ValidateMetadataStrict(m); !hasErrKey(errs, "date") {
		t.Fatal("expected error for bad date format")
	}
}

func TestValidateMetadataStrictBadLanguage(t *testing.T) {
	m := makeTestMetadata()
	m["language"] = []byte("English")
	if errs := ValidateMetadataStrict(m); !hasErrKey(errs, "language") {
		t.Fatal("expected error for bad language format")
	}
}

func TestValidateMetadataStrictUppercaseLanguage(t *testing.T) {
	m := makeTestMetadata()
	m["language"] = []byte("EN")
	if errs := ValidateMetadataStrict(m); !hasErrKey(errs, "language") {
		t.Fatal("expected error for uppercase language")
	}
}

func TestValidateMetadataStrictLanguageSubtag(t *testing.T) {
	m := makeTestMetadata()
	m["language"] = []byte("zh-Hans")
	if errs := ValidateMetadataStrict(m); len(errs) != 0 {
		t.Fatalf("expected no errors for language with subtag, got %v", errs)
	}
}

func TestValidateMetadataStrictEmptyTitle(t *testing.T) {
	m := makeTestMetadata()
	m["title"] = []byte("  ")
	if errs := ValidateMetadataStrict(m); !hasErrKey(errs, "title") {
		t.Fatal("expected error for whitespace-only title")
	}
}

func TestValidateMetadataStrictMissingKey(t *testing.T) {
	m := makeTestMetadata()
	delete(m, "creator")
	if errs := ValidateMetadataStrict(m); !hasErrKey(errs, "creator") {
		t.Fatal("expected error for missing creator")
	}
}

func TestValidateMetadataStrictBadFaviconEntry(t *testing.T) {
	m := makeTestMetadata()
	m["favicon_entry"] = []byte("not-a-number")
	if errs := ValidateMetadataStrict(m); !hasErrKey(errs, "favicon_entry") {
		t.Fatal("expected error for bad favicon_entry")
	}
}

func TestValidateMetadataStrictValidFaviconEntry(t *testing.T) {
	m := makeTestMetadata()
	m["favicon_entry"] = []byte("42")
	if errs := ValidateMetadataStrict(m); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateMetadataStrictEmptyLicense(t *testing.T) {
	m := makeTestMetadata()
	m["license"] = []byte("")
	if errs := ValidateMetadataStrict(m); !hasErrKey(errs, "license") {
		t.Fatal("expected error for empty license")
	}
}

func TestValidateMetadataStrictInvalidUTF8(t *testing.T) {
	m := makeTestMetadata()
	m["title"] = []byte{0xff, 0xfe}
	if errs := ValidateMetadataStrict(m); !hasErrKey(errs, "title") {
		t.Fatal("expected error for invalid UTF-8")
	}
}

func TestValidateMetadataStrictMultipleErrors(t *testing.T) {
	m := makeTestMetadata()
	m["date"] = []byte("bad")
	m["language"] = []byte("ENGLISH")
	m["title"] = []byte("")
	errs := ValidateMetadataStrict(m)
	if len(errs) < 3 {
		t.Fatalf("expected at least 3 errors, got %d: %v", len(errs), errs)
	}
}

func TestParseMetadataDuplicateKey(t *testing.T) {
	// Hand-craft binary metadata with a duplicate key.
	// Format: uint32 count, then per pair: uint16 key_len + key + uint32 val_len + value.
	var buf []byte
	buf = appendUint32(buf, 2) // 2 pairs

	// Pair 1: "title" = "first"
	buf = appendUint16(buf, 5)
	buf = append(buf, "title"...)
	buf = appendUint32(buf, 5)
	buf = append(buf, "first"...)

	// Pair 2: "title" = "second" (duplicate key)
	buf = appendUint16(buf, 5)
	buf = append(buf, "title"...)
	buf = appendUint32(buf, 6)
	buf = append(buf, "second"...)

	_, err := ParseMetadata(buf)
	if !errors.Is(err, ErrDuplicateMetadataKey) {
		t.Errorf("expected ErrDuplicateMetadataKey, got %v", err)
	}
}

func TestParseMetadataUniqueKeys(t *testing.T) {
	// Two distinct keys should parse fine.
	var buf []byte
	buf = appendUint32(buf, 2)

	buf = appendUint16(buf, 1)
	buf = append(buf, "a"...)
	buf = appendUint32(buf, 1)
	buf = append(buf, "1"...)

	buf = appendUint16(buf, 1)
	buf = append(buf, "b"...)
	buf = appendUint32(buf, 1)
	buf = append(buf, "2"...)

	m, err := ParseMetadata(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(m["a"]) != "1" || string(m["b"]) != "2" {
		t.Errorf("got %v, want a=1 b=2", m)
	}
}

func appendUint16(buf []byte, v uint16) []byte {
	return append(buf, byte(v), byte(v>>8))
}

func appendUint32(buf []byte, v uint32) []byte {
	return append(buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func TestValidationErrorString(t *testing.T) {
	e := ValidationError{Key: "date", Message: "not ISO 8601"}
	want := `oza: metadata "date": not ISO 8601`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
