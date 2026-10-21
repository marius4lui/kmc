package cli

import (
	"context"
	"testing"

	"github.com/marius4lui/kmc/internal/config"
)

func TestEditExplicitDefaultFields(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		options                 []string
		group, cwd, description string
	}{
		{"omitted fields", []string{"--command", "echo changed"}, "tools", "src", "Old description"},
		{"project root", []string{"--cwd", "."}, "tools", ".", "Old description"},
		{"manual group", []string{"--group", "manual"}, "manual", "src", "Old description"},
		{"empty description", []string{"--description", ""}, "tools", "src", ""},
		{"all defaults", []string{"--cwd", ".", "--group", "manual", "--description", ""}, "manual", ".", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			ctx := context.Background()
			if err := Run(ctx, []string{"add", "--name", "test", "--command", "echo original", "--group", "tools", "--cwd", "src", "--description", "Old description"}, "dev"); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"edit", "tools.test"}, tc.options...)
			if err := Run(ctx, args, "dev"); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Read(root)
			if err != nil {
				t.Fatal(err)
			}
			commands, err := config.Commands(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(commands) != 1 {
				t.Fatalf("got %d commands, want 1", len(commands))
			}
			got := commands[0]
			wantCommand := "echo original"
			if tc.name == "omitted fields" {
				wantCommand = "echo changed"
			}
			if got.Name != "test" || got.Command != wantCommand || got.ID != tc.group+".test" {
				t.Fatalf("unexpected saved command: %+v", got)
			}
			if got.GroupID != tc.group || got.CWD != tc.cwd || got.Description != tc.description {
				t.Fatalf("got group=%q cwd=%q description=%q; want %q %q %q", got.GroupID, got.CWD, got.Description, tc.group, tc.cwd, tc.description)
			}
		})
	}
}

func TestEditUIExplicitDefaults(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := addCommand([]string{"--name", "test", "--command", "echo original", "--group", "tools", "--cwd", "src", "--description", "Old description"}); err != nil {
		t.Fatal(err)
	}
	fields := commandFields{name: "test", command: "echo edited", cwd: ".", group: "manual", descriptionSet: true, cwdSet: true, groupSet: true}
	if err := upsertCommand("tools.test", fields); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Read(root)
	if err != nil {
		t.Fatal(err)
	}
	commands, err := config.Commands(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}
	got := commands[0]
	if got.ID != "manual.test" || got.CWD != "." || got.Description != "" || got.Command != "echo edited" {
		t.Fatalf("unexpected UI-style edit result: %+v", got)
	}
}
