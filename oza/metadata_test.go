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

func TestValidationErrorString(t *testing.T) {
	e := ValidationError{Key: "date", Message: "not ISO 8601"}
	want := `oza: metadata "date": not ISO 8601`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
