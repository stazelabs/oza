package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stazelabs/oza/oza"
)

func main() {
	root := &cobra.Command{
		Use:   "ozaverify <archive.oza>",
		Short: "Verify integrity of an OZA archive",
		Args:  cobra.ExactArgs(1),
		RunE:  run,
	}

	root.Flags().Bool("sections", false, "Also verify per-section SHA-256 checksums")
	root.Flags().Bool("chunks", false, "Also verify per-entry content hashes")
	root.Flags().Bool("all", false, "Verify all three integrity tiers")
	root.Flags().Bool("quiet", false, "Suppress OK lines; only print failures and the final summary")
	root.Flags().Bool("signatures", false, "Verify Ed25519 signatures (requires --pubkey)")
	root.Flags().StringArray("pubkey", nil, "Trusted Ed25519 public key in hex (repeatable)")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	path := args[0]

	verifySections, _ := cmd.Flags().GetBool("sections")
	verifyChunks, _ := cmd.Flags().GetBool("chunks")
	verifyAll, _ := cmd.Flags().GetBool("all")
	quiet, _ := cmd.Flags().GetBool("quiet")
	verifySigs, _ := cmd.Flags().GetBool("signatures")
	pubkeyHexes, _ := cmd.Flags().GetStringArray("pubkey")

	a, err := oza.Open(path)
	if err != nil {
		return err
	}
	defer a.Close()

	if verifyAll {
		return runVerifyAll(a, quiet)
	}

	// Always verify file-level checksum.
	if err := a.Verify(); err != nil {
		fmt.Printf("FAIL  file           %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK    file")

	if verifySections || verifyChunks {
		results, err := a.VerifyAll()
		if err != nil {
			return err
		}
		failed := false
		for _, r := range results {
			if r.Tier == "file" {
				continue // already printed
			}
			if r.Tier == "section" && !verifySections {
				continue
			}
			if r.Tier == "entry" && !verifyChunks {
				continue
			}
			status := "OK   "
			if !r.OK {
				status = "FAIL "
				failed = true
			}
			fmt.Printf("%s  %-12s  %s\n", status, r.Tier, r.ID)
		}
		if failed {
			os.Exit(1)
		}
	}

	if verifySigs {
		if err := runVerifySignatures(a, pubkeyHexes); err != nil {
			return err
		}
	} else if a.HasSignatures() && len(pubkeyHexes) == 0 {
		fmt.Fprintln(os.Stderr, "note: archive has signatures; use --signatures --pubkey <hex> to verify them")
	}

	return nil
}

func runVerifySignatures(a *oza.Archive, pubkeyHexes []string) error {
	if len(pubkeyHexes) == 0 {
		fmt.Fprintln(os.Stderr, "note: --signatures requires at least one --pubkey; skipping")
		return nil
	}

	var trustedKeys []ed25519.PublicKey
	for _, h := range pubkeyHexes {
		b, err := hex.DecodeString(h)
		if err != nil {
			return fmt.Errorf("invalid --pubkey %q: %w", h, err)
		}
		if len(b) != ed25519.PublicKeySize {
			return fmt.Errorf("invalid --pubkey %q: expected %d bytes, got %d", h, ed25519.PublicKeySize, len(b))
		}
		trustedKeys = append(trustedKeys, ed25519.PublicKey(b))
	}

	results, err := a.VerifySignatures(trustedKeys)
	if err != nil {
		return fmt.Errorf("signature verification: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("note: no trusted keys matched any signature in the archive")
		return nil
	}

	failed := false
	for _, r := range results {
		pubHex := hex.EncodeToString(r.PublicKey[:])
		if r.OK {
			fmt.Printf("OK    sig         key_id=%d pubkey=%s\n", r.KeyID, pubHex)
		} else {
			fmt.Printf("FAIL  sig         key_id=%d pubkey=%s\n", r.KeyID, pubHex)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
	return nil
}

func runVerifyAll(a *oza.Archive, quiet bool) error {
	results, err := a.VerifyAll()
	if err != nil {
		return err
	}

	passed, failed := 0, 0
	for _, r := range results {
		if !r.OK {
			failed++
			fmt.Printf("FAIL  %-10s  %s\n", r.Tier, r.ID)
		} else {
			passed++
			if !quiet {
				fmt.Printf("OK    %-10s  %s\n", r.Tier, r.ID)
			}
		}
	}

	fmt.Printf("\n%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
	return nil
}
