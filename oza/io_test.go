package oza

import (
	"io"
	"os"
	"testing"
)

func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "oza-io-test-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		t.Fatalf("Write: %v", err)
	}
	f.Close()
	return f.Name()
}

func testReader(t *testing.T, name string, r reader, content []byte) {
	t.Helper()

	if r.Size() != int64(len(content)) {
		t.Errorf("%s: Size() = %d, want %d", name, r.Size(), len(content))
	}

	// Read at offset 0
	buf := make([]byte, len(content))
	n, err := r.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		t.Errorf("%s: ReadAt(0): %v", name, err)
	}
	if n != len(content) {
		t.Errorf("%s: ReadAt(0) n = %d, want %d", name, n, len(content))
	}
	if string(buf) != string(content) {
		t.Errorf("%s: content mismatch", name)
	}

	// Read at a mid offset
	if len(content) > 4 {
		buf2 := make([]byte, 4)
		n2, err2 := r.ReadAt(buf2, 2)
		if err2 != nil && err2 != io.EOF {
			t.Errorf("%s: ReadAt(2): %v", name, err2)
		}
		if n2 != 4 {
			t.Errorf("%s: ReadAt(2) n = %d, want 4", name, n2)
		}
		if string(buf2) != string(content[2:6]) {
			t.Errorf("%s: mid read mismatch", name)
		}
	}

	// Out-of-bounds read should return io.EOF
	oob := make([]byte, 4)
	_, err = r.ReadAt(oob, int64(len(content))+100)
	if err != io.EOF {
		t.Errorf("%s: out-of-bounds read: got %v, want io.EOF", name, err)
	}
}

func TestFileReader(t *testing.T) {
	content := []byte("Hello, OZA reader test! 0123456789")
	path := writeTempFile(t, content)

	r, err := openReader(path, false)
	if err != nil {
		t.Fatalf("openReader(pread): %v", err)
	}
	defer r.Close()

	testReader(t, "pread", r, content)
}

func TestMmapReader(t *testing.T) {
	content := []byte("Hello, OZA mmap test! 0123456789")
	path := writeTempFile(t, content)

	r, err := openReader(path, true)
	if err != nil {
		t.Fatalf("openReader(mmap): %v", err)
	}
	defer r.Close()

	testReader(t, "mmap", r, content)
}
