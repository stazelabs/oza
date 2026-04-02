package ozawrite

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

// TranscodeTools holds paths to external image transcoding tools.
// Discovered at startup via DiscoverTranscodeTools.
//
// Install libwebp to enable:
//
//	macOS:  brew install webp
//	Ubuntu: sudo apt install webp
//	Fedora: sudo dnf install libwebp-tools
type TranscodeTools struct {
	GIF2WebP string // path to gif2webp, "" if not found
	CWebP    string // path to cwebp, "" if not found

	mu    sync.Mutex
	stats TranscodeStats
}

// TranscodeStats tracks per-format transcoding results.
type TranscodeStats struct {
	GIFCount   int   // successfully transcoded
	GIFSaved   int64 // bytes saved (original - transcoded)
	GIFSkipped int   // kept original (error, larger output, tool missing)
	PNGCount   int
	PNGSaved   int64
	PNGSkipped int
}

// Stats returns a snapshot of the accumulated transcode statistics.
func (t *TranscodeTools) Stats() TranscodeStats {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stats
}

// DiscoverTranscodeTools probes PATH for gif2webp and cwebp.
func DiscoverTranscodeTools() *TranscodeTools {
	t := &TranscodeTools{}
	t.GIF2WebP, _ = exec.LookPath("gif2webp")
	t.CWebP, _ = exec.LookPath("cwebp")
	return t
}

// Available reports whether any transcoding tools were found.
func (t *TranscodeTools) Available() bool {
	return t.GIF2WebP != "" || t.CWebP != ""
}

// String returns a human-readable summary of discovered tools.
func (t *TranscodeTools) String() string {
	gif := t.GIF2WebP
	if gif == "" {
		gif = "(not found)"
	}
	cwp := t.CWebP
	if cwp == "" {
		cwp = "(not found)"
	}
	return fmt.Sprintf("gif2webp=%s cwebp=%s", gif, cwp)
}

// transcodeTimeout is the maximum time allowed for a single transcoding operation.
const transcodeTimeout = 30 * time.Second

// Transcode converts image content to WebP if a suitable tool is available.
// Returns the (possibly transcoded) content and the (possibly changed) MIME type.
// If transcoding fails, is unavailable, or produces larger output, the original
// content and MIME type are returned unchanged.
func (t *TranscodeTools) Transcode(mimeType string, data []byte) ([]byte, string) {
	switch mimeType {
	case "image/gif":
		if t.GIF2WebP == "" {
			t.mu.Lock()
			t.stats.GIFSkipped++
			t.mu.Unlock()
			return data, mimeType
		}
		out, err := t.runTool(t.GIF2WebP, []string{"-q", "75", "-m", "4"}, data)
		t.mu.Lock()
		if err != nil || len(out) >= len(data) {
			t.stats.GIFSkipped++
			t.mu.Unlock()
			return data, mimeType
		}
		t.stats.GIFCount++
		t.stats.GIFSaved += int64(len(data)) - int64(len(out))
		t.mu.Unlock()
		return out, "image/webp"

	case "image/png":
		if t.CWebP == "" {
			t.mu.Lock()
			t.stats.PNGSkipped++
			t.mu.Unlock()
			return data, mimeType
		}
		out, err := t.runTool(t.CWebP, []string{"-lossless"}, data)
		t.mu.Lock()
		if err != nil || len(out) >= len(data) {
			t.stats.PNGSkipped++
			t.mu.Unlock()
			return data, mimeType
		}
		t.stats.PNGCount++
		t.stats.PNGSaved += int64(len(data)) - int64(len(out))
		t.mu.Unlock()
		return out, "image/webp"
	}

	return data, mimeType
}

// runTool executes an external tool with input from a temp file and reads the
// output from another temp file. Both tools use the pattern: tool [flags] input -o output.
func (t *TranscodeTools) runTool(toolPath string, flags []string, data []byte) ([]byte, error) {
	inFile, err := os.CreateTemp("", "oza-transcode-in-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(inFile.Name()) }()

	outFile, err := os.CreateTemp("", "oza-transcode-out-*.webp")
	if err != nil {
		inFile.Close()
		return nil, err
	}
	outName := outFile.Name()
	outFile.Close()
	defer func() { _ = os.Remove(outName) }()

	if _, err := inFile.Write(data); err != nil {
		inFile.Close()
		return nil, err
	}
	inFile.Close()

	args := append(flags, inFile.Name(), "-o", outName)
	ctx, cancel := context.WithTimeout(context.Background(), transcodeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, toolPath, args...) //nolint:gosec // toolPath is from exec.LookPath, not user input
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", toolPath, err, out)
	}

	return os.ReadFile(outName)
}
