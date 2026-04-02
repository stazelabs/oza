package oza

import (
	"fmt"
	"io"
	"os"
)

// reader is the internal I/O interface used by Archive.
// It supports random-access reads and reports the total file size.
type reader interface {
	io.ReaderAt
	Size() int64
	Close() error
}

// openReader opens the file at path and returns a reader.
// When useMmap is true, the file is memory-mapped for zero-copy reads.
// When useMmap is false, a standard pread-based reader is used.
func openReader(path string, useMmap bool) (reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("oza: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("oza: stat %q: %w", path, err)
	}
	size := info.Size()

	if useMmap {
		r, err := newMmapReader(f, size)
		if err != nil {
			// Fall back to pread on mmap failure.
			return &fileReader{f: f, size: size}, nil
		}
		return r, nil
	}

	return &fileReader{f: f, size: size}, nil
}

// fileReader is a pread-based reader backed by an *os.File.
type fileReader struct {
	f    *os.File
	size int64
}

func (r *fileReader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= r.size {
		return 0, io.EOF
	}
	n, err := r.f.ReadAt(p, off)
	if err == io.EOF && n == len(p) {
		// Full read at EOF boundary is not an error.
		err = nil
	}
	return n, err
}

func (r *fileReader) Size() int64  { return r.size }
func (r *fileReader) Close() error { return r.f.Close() }
