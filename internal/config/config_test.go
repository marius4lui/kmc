package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeLegacyCommandsAndFind(t *testing.T) {
	value, err := Normalize(Config{Commands: []Command{
		{Name: "dev", Command: "vite", Group: "development"},
		{Name: "release", Command: "echo release", GroupID: "deployment"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(value.Groups) != 2 || value.Groups[0].Commands[0].ID != "development.dev" {
		t.Fatalf("unexpected normalized config: %#v", value)
	}
	commands, err := Commands(value)
	if err != nil {
		t.Fatal(err)
	}
	found, ok := FindCommand(commands, "release")
	if !ok || found.ID != "deployment.release" {
		t.Fatalf("command not found: %#v", found)
	}
}

func TestReadWriteConfigAndSettings(t *testing.T) {
	root := t.TempDir()
	input := Config{Groups: []Group{{
		ID:       "deployment",
		Commands: []Command{{Name: "deploy", Command: "echo deploy"}},
	}}}
	if err := Write(root, input); err != nil {
		t.Fatal(err)
	}
	read, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if read.Groups[0].Label != "Deployment" || read.Groups[0].Commands[0].CWD != "." {
		t.Fatalf("defaults were not applied: %#v", read)
	}
	settings := DefaultSettings()
	settings.MaxFavoriteCommands = 5
	if err := WriteSettings(root, settings); err != nil {
		t.Fatal(err)
	}
	readSettings, err := ReadSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if readSettings.MaxFavoriteCommands != 5 {
		t.Fatalf("unexpected settings: %#v", readSettings)
	}
	ignored, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil || string(ignored) != ".kmc/\n" {
		t.Fatalf("settings ignore missing: %q, %v", ignored, err)
	}
}

func TestValidateDuplicates(t *testing.T) {
	input := Config{Groups: []Group{{
		ID: "manual", Label: "Manual", Type: "manual",
		Commands: []Command{
			{ID: "manual.same", Name: "one", Command: "true"},
			{ID: "manual.same", Name: "two", Command: "true"},
		},
	}}}
	result := Validate(input, DefaultSettings())
	if result.OK {
		t.Fatal("duplicate ids must fail validation")
	}
}
