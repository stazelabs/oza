package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var outPath string

	root := &cobra.Command{
		Use:   "ozakeygen",
		Short: "Generate an Ed25519 keypair for OZA archive signing",
		Long: `王座 ozakeygen — generate an Ed25519 keypair for signing OZA archives.

Outputs a PEM-encoded private key to stdout (or --out file) and prints
the hex-encoded public key to stderr for use with ozaverify --pubkey.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(outPath)
		},
	}
	root.Flags().StringVar(&outPath, "out", "", "Write private key PEM to this file (default: stdout)")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(outPath string) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generating key: %w", err)
	}

	// Encode private key as PKCS#8 PEM.
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("marshaling private key: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})

	pubHex := hex.EncodeToString(pub)

	fmt.Printf("Public key (hex):   %s\n", pubHex)

	if outPath != "" {
		if err := os.WriteFile(outPath, privPEM, 0600); err != nil {
			return fmt.Errorf("writing private key to %s: %w", outPath, err)
		}
		fmt.Printf("Private key written to: %s\n", outPath)
	} else {
		fmt.Printf("Private key (PEM):\n%s", privPEM)
	}

	return nil
}
