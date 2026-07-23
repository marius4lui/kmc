package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	runErr := fn()
	_ = writer.Close()
	os.Stdout = original
	var output bytes.Buffer
	_, _ = io.Copy(&output, reader)
	return output.String(), runErr
}

func TestVersionAndHelp(t *testing.T) {
	output, err := captureStdout(t, func() error { return Run(context.Background(), []string{"--version"}, "2.0.0") })
	if err != nil || strings.TrimSpace(output) != "2.0.0" {
		t.Fatalf("version output = %q, %v", output, err)
	}
	output, err = captureStdout(t, func() error { return Run(context.Background(), []string{"--help"}, "2.0.0") })
	if err != nil || !strings.Contains(output, "kmc run <command-id>") {
		t.Fatalf("help output = %q, %v", output, err)
	}
}

func TestAddValidateAndDelete(t *testing.T) {
	root := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := Run(context.Background(), []string{"add", "--name", "dev", "--command", "echo dev"}, "dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "kmc.json")); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"validate"}, "dev"); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"delete", "manual.dev"}, "dev"); err != nil {
		t.Fatal(err)
	}
}
