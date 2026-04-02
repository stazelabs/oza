package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/stazelabs/oza/cmd/internal/testutil"
)

func newTestCmd(args ...string) *cobra.Command {
	cmd := &cobra.Command{RunE: run}
	cmd.Flags().BoolP("list", "l", false, "")
	cmd.Flags().BoolP("meta", "m", false, "")
	cmd.Flags().BoolP("info", "t", false, "")
	cmd.Flags().StringP("output", "o", "", "")
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

func TestOzacatList(t *testing.T) {
	path := testutil.BuildTestOZA(t, false)
	cmd := newTestCmd(path)
	cmd.Flags().Set("list", "true")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ozacat -l: %v", err)
	}
}

func TestOzacatMeta(t *testing.T) {
	path := testutil.BuildTestOZA(t, false)
	cmd := newTestCmd(path)
	cmd.Flags().Set("meta", "true")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ozacat -m: %v", err)
	}
}

func TestOzacatExtract(t *testing.T) {
	path := testutil.BuildTestOZA(t, false)
	cmd := newTestCmd(path, "index.html")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ozacat extract: %v", err)
	}
}

func TestOzacatMissingFile(t *testing.T) {
	cmd := newTestCmd("/nonexistent/path.oza")
	cmd.Flags().Set("list", "true")
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing file")
	}
}
