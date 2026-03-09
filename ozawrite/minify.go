package ozawrite

import (
	"github.com/tdewolff/minify/v2"
)

// minifyContent applies minification to content based on its MIME type.
// Returns the original content unchanged if minification fails or is not
// applicable.
func minifyContent(m *minify.M, mimeType string, content []byte) []byte {
	if len(content) == 0 {
		return content
	}
	out, err := m.Bytes(mimeType, content)
	if err != nil {
		return content // minification failed; keep original
	}
	return out
}
