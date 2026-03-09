//go:build !windows

package oza

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

// mmapReader is a memory-mapped reader.
type mmapReader struct {
	data []byte
	size int64
}

func newMmapReader(f *os.File, size int64) (*mmapReader, error) {
	if size == 0 {
		f.Close()
		return &mmapReader{data: nil, size: 0}, nil
	}
	data, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_PRIVATE)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("oza: mmap: %w", err)
	}
	// File descriptor no longer needed after mmap.
	f.Close()
	return &mmapReader{data: data, size: size}, nil
}

func (r *mmapReader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= r.size {
		return 0, io.EOF
	}
	n := copy(p, r.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (r *mmapReader) Size() int64 { return r.size }

func (r *mmapReader) Close() error {
	if r.data == nil {
		return nil
	}
	return syscall.Munmap(r.data)
}
