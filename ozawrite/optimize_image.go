package ozawrite

import (
	"bytes"
	"encoding/binary"
)

// optimizeImage applies lossless optimization to image content based on MIME type.
// JPEG: strip EXIF, APP, and comment markers at the byte level (no re-encoding).
// Returns the original data unchanged for unsupported formats or on error.
func optimizeImage(mimeType string, data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	if mimeType == "image/jpeg" || mimeType == "image/jpg" {
		return stripJPEGMetadata(data)
	}
	return data
}

// stripJPEGMetadata removes APP0-APP15, COM (comment), and other non-essential
// markers from a JPEG file. This is a lossless byte-level operation — the
// entropy-coded image data is not modified.
func stripJPEGMetadata(data []byte) []byte {
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return data
	}

	var buf bytes.Buffer
	buf.Grow(len(data))
	buf.Write([]byte{0xFF, 0xD8}) // SOI

	i := 2
	for i < len(data)-1 {
		if data[i] != 0xFF {
			return data // unexpected byte; bail
		}

		marker := data[i+1]

		switch {
		case marker == 0xD9: // EOI
			buf.Write([]byte{0xFF, 0xD9})
			return buf.Bytes()

		case marker == 0xDA: // SOS — copy rest verbatim (entropy data + EOI)
			buf.Write(data[i:])
			return buf.Bytes()

		case marker >= 0xD0 && marker <= 0xD7: // RST (no length)
			buf.Write(data[i : i+2])
			i += 2
			continue

		case marker == 0x00: // stuffed byte; shouldn't appear here
			return data
		}

		// Marker with a length field.
		if i+4 > len(data) {
			return data
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if segLen < 2 || i+2+segLen > len(data) {
			return data
		}

		if shouldKeepJPEGMarker(marker) {
			buf.Write(data[i : i+2+segLen])
		}

		i += 2 + segLen
	}

	return data // shouldn't reach here for valid JPEG
}

// shouldKeepJPEGMarker returns true for markers required to decode the image.
func shouldKeepJPEGMarker(marker byte) bool {
	switch {
	case marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xCC:
		return true // SOF (frame headers)
	case marker == 0xC4: // DHT (Huffman tables)
		return true
	case marker == 0xCC: // DAC (arithmetic conditioning)
		return true
	case marker == 0xDB: // DQT (quantization tables)
		return true
	case marker == 0xDD: // DRI (restart interval)
		return true
	case marker == 0xDA: // SOS (handled separately above)
		return true
	case marker == 0xFE: // COM (comment) — strip
		return false
	case marker >= 0xE0 && marker <= 0xEF: // APP0-APP15 — strip
		return false
	default:
		return true // keep unknown markers to be safe
	}
}

// isImageMIME returns true for MIME types that optimizeImage can handle.
func isImageMIME(mimeType string) bool {
	return mimeType == "image/jpeg" || mimeType == "image/jpg"
}
