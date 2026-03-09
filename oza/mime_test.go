package oza

import "testing"

var testMIMETypes = []string{
	"text/html",
	"text/css",
	"application/javascript",
	"image/png",
	"application/json",
}

func TestParseMIMETableRoundTrip(t *testing.T) {
	data, err := MarshalMIMETable(testMIMETypes)
	if err != nil {
		t.Fatalf("MarshalMIMETable: %v", err)
	}

	got, err := ParseMIMETable(data)
	if err != nil {
		t.Fatalf("ParseMIMETable: %v", err)
	}

	if len(got) != len(testMIMETypes) {
		t.Fatalf("got %d types, want %d", len(got), len(testMIMETypes))
	}
	for i := range testMIMETypes {
		if got[i] != testMIMETypes[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], testMIMETypes[i])
		}
	}
}

func TestMIMEIndexConvention(t *testing.T) {
	// Wrong index 0
	bad := []string{"text/plain", "text/css", "application/javascript"}
	_, err := MarshalMIMETable(bad)
	if err == nil {
		t.Error("expected error for wrong index 0")
	}

	// Wrong index 1
	bad = []string{"text/html", "text/plain", "application/javascript"}
	_, err = MarshalMIMETable(bad)
	if err == nil {
		t.Error("expected error for wrong index 1")
	}

	// Wrong index 2
	bad = []string{"text/html", "text/css", "text/plain"}
	_, err = MarshalMIMETable(bad)
	if err == nil {
		t.Error("expected error for wrong index 2")
	}
}

func TestParseMIMETableRejectsWrongConvention(t *testing.T) {
	// Manually build a table with wrong index 0
	types := []string{"text/plain", "text/css", "application/javascript"}
	size := 2
	for _, t := range types {
		size += 2 + len(t)
	}
	buf := make([]byte, size)
	buf[0] = byte(len(types))
	buf[1] = 0
	off := 2
	for _, s := range types {
		buf[off] = byte(len(s))
		buf[off+1] = 0
		off += 2
		copy(buf[off:], s)
		off += len(s)
	}

	_, err := ParseMIMETable(buf)
	if err == nil {
		t.Error("expected error for wrong MIME convention in ParseMIMETable")
	}
}

func TestMIMETableEmpty(t *testing.T) {
	data, err := MarshalMIMETable(nil)
	if err != nil {
		t.Fatalf("MarshalMIMETable(nil): %v", err)
	}

	got, err := ParseMIMETable(data)
	if err != nil {
		t.Fatalf("ParseMIMETable empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 types, got %d", len(got))
	}
}

func FuzzParseMIMETable(f *testing.F) {
	data, _ := MarshalMIMETable(testMIMETypes)
	f.Add(data)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseMIMETable(data)
	})
}
