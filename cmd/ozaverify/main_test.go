package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/stazelabs/oza/cmd/internal/testutil"
)

func newTestCmd(args ...string) *cobra.Command {
	cmd := &cobra.Command{
		Args: cobra.ExactArgs(1),
		RunE: run,
	}
	cmd.Flags().Bool("sections", false, "")
	cmd.Flags().Bool("chunks", false, "")
	cmd.Flags().Bool("all", false, "")
	cmd.Flags().Bool("quiet", false, "")
	cmd.Flags().Bool("signatures", false, "")
	cmd.Flags().StringArray("pubkey", nil, "")
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

func TestOzaverifySmoke(t *testing.T) {
	path := testutil.BuildTestOZA(t, false)
	cmd := newTestCmd(path)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ozaverify: %v", err)
	}
}

func TestOzaverifySections(t *testing.T) {
	path := testutil.BuildTestOZA(t, false)
	cmd := newTestCmd(path)
	cmd.Flags().Set("sections", "true")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ozaverify --sections: %v", err)
	}
}

func TestOzaverifyAll(t *testing.T) {
	path := testutil.BuildTestOZA(t, false)
	cmd := newTestCmd(path)
	cmd.Flags().Set("all", "true")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ozaverify --all: %v", err)
	}
}

func TestOzaverifyMissingFile(t *testing.T) {
	cmd := newTestCmd("/nonexistent/path.oza")
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing file")
	}
}
