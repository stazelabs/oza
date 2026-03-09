//go:build windows

package oza

import (
	"fmt"
	"os"
)

// mmapReader on Windows falls back to a fileReader since mmap requires
// different Windows APIs. The openReader function will use fileReader directly.
type mmapReader struct{}

func newMmapReader(f *os.File, size int64) (*mmapReader, error) {
	return nil, fmt.Errorf("oza: mmap not supported on Windows")
}

func (r *mmapReader) ReadAt([]byte, int64) (int, error) { panic("unreachable") }
func (r *mmapReader) Size() int64                       { panic("unreachable") }
func (r *mmapReader) Close() error                      { panic("unreachable") }
