package oza

import "errors"

var (
	ErrInvalidMagic       = errors.New("oza: invalid magic number")
	ErrNotFound           = errors.New("oza: entry not found")
	ErrIsRedirect         = errors.New("oza: entry is a redirect")
	ErrNotRedirect        = errors.New("oza: entry is not a redirect")
	ErrChecksumMismatch   = errors.New("oza: checksum verification failed")
	ErrUnsupportedVersion = errors.New("oza: unsupported format version")
	ErrInvalidEntry       = errors.New("oza: invalid entry record")
	ErrRedirectLoop       = errors.New("oza: redirect loop detected")
	ErrCorruptedSection   = errors.New("oza: corrupted section")
	ErrCorruptedChunk     = errors.New("oza: corrupted chunk")
	ErrMissingMetadata    = errors.New("oza: required metadata key missing")
)
