package oza

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// RequiredMetadataKeys lists the metadata keys that must be present in every OZA archive.
var RequiredMetadataKeys = []string{"title", "language", "creator", "date", "source"}

// ParseMetadata parses key-value metadata pairs from data.
//
// Format: uint32 pair_count, per pair: uint16 key_len + key + uint32 val_len + value.
func ParseMetadata(data []byte) (map[string][]byte, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("oza: metadata too short")
	}

	count := int(binary.LittleEndian.Uint32(data[0:4]))
	m := make(map[string][]byte)
	off := 4

	for i := range count {
		// Read key
		if off+2 > len(data) {
			return nil, fmt.Errorf("oza: metadata truncated at pair %d (key length)", i)
		}
		klen := int(binary.LittleEndian.Uint16(data[off : off+2]))
		off += 2

		if off+klen > len(data) {
			return nil, fmt.Errorf("oza: metadata truncated at pair %d (key bytes)", i)
		}
		key := string(data[off : off+klen])
		off += klen

		// Read value
		if off+4 > len(data) {
			return nil, fmt.Errorf("oza: metadata truncated at pair %d (value length)", i)
		}
		vlen := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4

		if off+vlen > len(data) {
			return nil, fmt.Errorf("oza: metadata truncated at pair %d (value bytes)", i)
		}
		val := make([]byte, vlen)
		copy(val, data[off:off+vlen])
		off += vlen

		m[key] = val
	}

	return m, nil
}

// MarshalMetadata serializes key-value metadata pairs.
func MarshalMetadata(pairs map[string][]byte) ([]byte, error) {
	// Calculate size: 4 (count) + sum of (2 + len(key) + 4 + len(val)) per pair.
	size := 4
	for k, v := range pairs {
		size += 2 + len(k) + 4 + len(v)
	}

	buf := make([]byte, size)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(pairs)))
	off := 4

	for k, v := range pairs {
		binary.LittleEndian.PutUint16(buf[off:off+2], uint16(len(k)))
		off += 2
		copy(buf[off:], k)
		off += len(k)

		binary.LittleEndian.PutUint32(buf[off:off+4], uint32(len(v)))
		off += 4
		copy(buf[off:], v)
		off += len(v)
	}

	return buf, nil
}

// ValidateMetadata checks that all required metadata keys are present.
func ValidateMetadata(m map[string][]byte) error {
	for _, key := range RequiredMetadataKeys {
		if _, ok := m[key]; !ok {
			return fmt.Errorf("%w: %q", ErrMissingMetadata, key)
		}
	}
	return nil
}

// ValidationError describes a single metadata validation failure.
type ValidationError struct {
	Key     string // metadata key (e.g. "date")
	Message string // human-readable description
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("oza: metadata %q: %s", e.Key, e.Message)
}

// ValidateMetadataStrict checks that all required keys are present and that
// values with a defined format conform to the spec (FORMAT.md §3.4):
//
//   - date: ISO 8601 (YYYY-MM-DD or YYYY-MM-DDThh:mm:ssZ)
//   - language: BCP-47 primary subtag (2–3 lowercase ASCII letters, optional subtags)
//   - title, creator, source: non-empty valid UTF-8
//   - favicon_entry, main_entry: decimal uint32 if present
//   - license: non-empty if present
//
// All issues are collected and returned; the caller gets the full picture rather
// than failing on the first problem. Returns nil if validation passes.
func ValidateMetadataStrict(m map[string][]byte) []ValidationError {
	var errs []ValidationError
	add := func(key, msg string) {
		errs = append(errs, ValidationError{Key: key, Message: msg})
	}

	// Required key presence.
	for _, key := range RequiredMetadataKeys {
		if _, ok := m[key]; !ok {
			add(key, "required key missing")
		}
	}

	// All values must be valid UTF-8.
	for key, val := range m {
		if !utf8.Valid(val) {
			add(key, "value is not valid UTF-8")
		}
	}

	// Non-empty required string keys.
	for _, key := range []string{"title", "creator", "source"} {
		if v, ok := m[key]; ok && len(strings.TrimSpace(string(v))) == 0 {
			add(key, "must be non-empty")
		}
	}

	// date: ISO 8601.
	if v, ok := m["date"]; ok && len(v) > 0 {
		s := string(v)
		valid := false
		for _, layout := range []string{"2006-01-02", time.RFC3339} {
			if _, err := time.Parse(layout, s); err == nil {
				valid = true
				break
			}
		}
		if !valid {
			add("date", fmt.Sprintf("not ISO 8601 (got %q, want YYYY-MM-DD or RFC 3339)", s))
		}
	}

	// language: BCP-47.
	if v, ok := m["language"]; ok && len(v) > 0 {
		s := string(v)
		subtags := strings.Split(s, "-")
		primary := subtags[0]
		if len(primary) < 2 || len(primary) > 3 {
			add("language", fmt.Sprintf("primary subtag must be 2-3 letters (got %q)", primary))
		} else {
			for _, c := range primary {
				if c < 'a' || c > 'z' {
					add("language", fmt.Sprintf("primary subtag must be lowercase ASCII letters (got %q)", primary))
					break
				}
			}
		}
	}

	// Optional uint32 keys.
	for _, key := range []string{"favicon_entry", "main_entry"} {
		if v, ok := m[key]; ok {
			n, err := strconv.ParseUint(string(v), 10, 32)
			if err != nil {
				add(key, fmt.Sprintf("must be a decimal uint32 (got %q)", string(v)))
			}
			_ = n
		}
	}

	// Optional non-empty keys.
	if v, ok := m["license"]; ok && len(strings.TrimSpace(string(v))) == 0 {
		add("license", "must be non-empty if present")
	}

	return errs
}
