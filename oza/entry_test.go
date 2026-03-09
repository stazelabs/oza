package oza

import "testing"

func makeContentEntry() EntryRecord {
	return EntryRecord{
		ID:             42,
		Type:           EntryContent,
		Flags:          EntryFlagFrontArticle,
		MIMEIndex:      0,
		ChunkID:        3,
		BlobOffset:     1024,
		BlobSize:       4096,
		RedirectTarget: 0,
		ContentHash:    0xDEADBEEFCAFEBABE,
	}
}

func makeRedirectEntry() EntryRecord {
	return EntryRecord{
		ID:             99,
		Type:           EntryRedirect,
		Flags:          0,
		MIMEIndex:      0,
		ChunkID:        0,
		BlobOffset:     0,
		BlobSize:       0,
		RedirectTarget: 42,
		ContentHash:    0,
	}
}

func makeMetadataRefEntry() EntryRecord {
	return EntryRecord{
		ID:             0,
		Type:           EntryMetadataRef,
		Flags:          0,
		MIMEIndex:      1,
		ChunkID:        0,
		BlobOffset:     0,
		BlobSize:       128,
		RedirectTarget: 0,
		ContentHash:    0xABCD,
	}
}

func testVarEntryRoundTrip(t *testing.T, name string, e EntryRecord) {
	t.Helper()
	buf := AppendVarEntryRecord(nil, e)
	got, n, err := ParseVarEntryRecord(buf)
	if err != nil {
		t.Fatalf("%s: ParseVarEntryRecord: %v", name, err)
	}
	if n != len(buf) {
		t.Errorf("%s: consumed %d bytes, buf is %d", name, n, len(buf))
	}
	// ID and RedirectTarget are not serialized; zero them for comparison.
	e.ID = 0
	e.RedirectTarget = 0
	if got != e {
		t.Errorf("%s: round-trip mismatch:\n  got  %+v\n  want %+v", name, got, e)
	}
}

func TestParseVarEntryRecordRoundTrip(t *testing.T) {
	testVarEntryRoundTrip(t, "content", makeContentEntry())
	testVarEntryRoundTrip(t, "metadataRef", makeMetadataRefEntry())
}

func TestEntryIsRedirect(t *testing.T) {
	content := makeContentEntry()
	redirect := makeRedirectEntry()

	if content.IsRedirect() {
		t.Error("content entry should not be redirect")
	}
	if !redirect.IsRedirect() {
		t.Error("redirect entry should be redirect")
	}
}

func TestEntryIsFrontArticle(t *testing.T) {
	front := makeContentEntry() // has EntryFlagFrontArticle
	notFront := makeRedirectEntry()

	if !front.IsFrontArticle() {
		t.Error("entry with FlagFrontArticle should be front article")
	}
	if notFront.IsFrontArticle() {
		t.Error("redirect entry without flag should not be front article")
	}
}

func TestParseVarEntryRecordShortData(t *testing.T) {
	_, _, err := ParseVarEntryRecord(make([]byte, 1))
	if err == nil {
		t.Error("expected error for short data")
	}
}

func TestRedirectRecordRoundTrip(t *testing.T) {
	rr := RedirectRecord{
		Flags:    EntryFlagFrontArticle,
		TargetID: 42,
	}
	b := rr.Marshal()
	got, err := ParseRedirectRecord(b[:])
	if err != nil {
		t.Fatalf("ParseRedirectRecord: %v", err)
	}
	if got != rr {
		t.Errorf("RedirectRecord round-trip mismatch: got %+v, want %+v", got, rr)
	}
	if !got.IsFrontArticle() {
		t.Error("expected IsFrontArticle=true")
	}
}

func TestRedirectRecordShortData(t *testing.T) {
	_, err := ParseRedirectRecord(make([]byte, 3))
	if err == nil {
		t.Error("expected error for short redirect record data")
	}
}

func TestRedirectIDHelpers(t *testing.T) {
	id := MakeRedirectID(42)
	if !IsRedirectID(id) {
		t.Errorf("IsRedirectID(0x%08x) should be true", id)
	}
	if RedirectIndex(id) != 42 {
		t.Errorf("RedirectIndex(0x%08x) = %d, want 42", id, RedirectIndex(id))
	}
	// Content ID should not be a redirect ID.
	if IsRedirectID(42) {
		t.Error("IsRedirectID(42) should be false")
	}
}

func FuzzParseVarEntryRecord(f *testing.F) {
	b := AppendVarEntryRecord(nil, makeContentEntry())
	f.Add(b)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseVarEntryRecord(data)
	})
}
