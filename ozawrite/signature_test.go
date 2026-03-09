package ozawrite_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"os"
	"testing"

	"github.com/stazelabs/oza/oza"
	"github.com/stazelabs/oza/ozawrite"
)

func generateTestKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

// newSignedArchive writes a minimal OZA archive signed with keys, closes it,
// and opens it for reading. Returns the open archive and the raw file bytes.
func newSignedArchive(t *testing.T, keys []ozawrite.SigningKey) (*oza.Archive, []byte) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "signed*.oza")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()

	opts := ozawrite.WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: false,
		SigningKeys:  keys,
	}
	w := ozawrite.NewWriter(f, opts)
	requiredMeta(w)
	if _, err := w.AddEntry("index.html", "Index", "text/html", []byte("<html>hi</html>"), true); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	a, err := oza.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	return a, data
}

func TestSignatureRoundTrip(t *testing.T) {
	pub, priv := generateTestKey(t)
	keys := []ozawrite.SigningKey{{Key: priv, KeyID: 42}}

	a, _ := newSignedArchive(t, keys)

	if !a.HasSignatures() {
		t.Fatal("archive should have has_signatures flag set")
	}

	recs, err := a.ReadSignatures()
	if err != nil {
		t.Fatalf("ReadSignatures: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 signature record, got %d", len(recs))
	}
	if recs[0].KeyID != 42 {
		t.Errorf("key_id = %d, want 42", recs[0].KeyID)
	}
	if !bytes.Equal(recs[0].PublicKey[:], pub) {
		t.Error("public key mismatch in record")
	}

	results, err := a.VerifySignatures([]ed25519.PublicKey{pub})
	if err != nil {
		t.Fatalf("VerifySignatures: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 verify result, got %d", len(results))
	}
	if !results[0].OK {
		t.Error("signature should verify OK")
	}
}

func TestSignatureMultipleKeys(t *testing.T) {
	pub1, priv1 := generateTestKey(t)
	pub2, priv2 := generateTestKey(t)
	keys := []ozawrite.SigningKey{
		{Key: priv1, KeyID: 1},
		{Key: priv2, KeyID: 2},
	}

	a, _ := newSignedArchive(t, keys)

	results, err := a.VerifySignatures([]ed25519.PublicKey{pub1, pub2})
	if err != nil {
		t.Fatalf("VerifySignatures: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("key_id=%d: signature should verify OK", r.KeyID)
		}
	}
}

func TestSignatureUnknownKey(t *testing.T) {
	_, priv := generateTestKey(t)
	otherPub, _ := generateTestKey(t)
	keys := []ozawrite.SigningKey{{Key: priv, KeyID: 1}}

	a, _ := newSignedArchive(t, keys)

	// Verify with a key that didn't sign — should return empty results, not error.
	results, err := a.VerifySignatures([]ed25519.PublicKey{otherPub})
	if err != nil {
		t.Fatalf("VerifySignatures: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for unknown key, got %d", len(results))
	}
}

func TestNoSignatureFlag(t *testing.T) {
	w, f, cleanup := newTestWriter(t)
	defer cleanup()
	requiredMeta(w)
	if _, err := w.AddEntry("a.html", "A", "text/html", []byte("<html>a</html>"), true); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	a, err := oza.Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	if a.HasSignatures() {
		t.Error("unsigned archive should not have has_signatures flag")
	}
	recs, err := a.ReadSignatures()
	if err != nil {
		t.Fatalf("ReadSignatures on unsigned archive: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected 0 records, got %d", len(recs))
	}
}

func TestSignatureKeyIDPreserved(t *testing.T) {
	_, priv := generateTestKey(t)
	const wantKeyID = uint32(0xDEADBEEF)
	keys := []ozawrite.SigningKey{{Key: priv, KeyID: wantKeyID}}

	a, _ := newSignedArchive(t, keys)

	recs, err := a.ReadSignatures()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].KeyID != wantKeyID {
		t.Errorf("key_id = 0x%X, want 0x%X", recs[0].KeyID, wantKeyID)
	}
}

// TestSignatureTrailerLayout verifies the raw byte layout of the trailer.
func TestSignatureTrailerLayout(t *testing.T) {
	pub, priv := generateTestKey(t)
	keys := []ozawrite.SigningKey{{Key: priv, KeyID: 7}}

	_, data := newSignedArchive(t, keys)

	hdr, err := oza.ParseHeader(data[:oza.HeaderSize])
	if err != nil {
		t.Fatal(err)
	}

	trailerOff := hdr.ChecksumOff + 32
	trailer := data[trailerOff:]

	count := binary.LittleEndian.Uint32(trailer[0:4])
	if count != 1 {
		t.Fatalf("trailer count = %d, want 1", count)
	}

	rec := trailer[4:]
	if !bytes.Equal(rec[0:32], pub) {
		t.Error("public key in trailer does not match")
	}
	keyID := binary.LittleEndian.Uint32(rec[96:100])
	if keyID != 7 {
		t.Errorf("key_id in trailer = %d, want 7", keyID)
	}
	for i, b := range rec[100:128] {
		if b != 0 {
			t.Errorf("reserved byte %d = 0x%X, want 0", i, b)
		}
	}
}
