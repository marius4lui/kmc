package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteEmptyConfigRoundTrip(t *testing.T) {
	for name, input := range map[string]Config{
		"groups":          {Groups: []Group{}},
		"legacy commands": {Commands: []Command{}},
		"empty group":     {Groups: []Group{{ID: "manual"}}},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := Write(root, input); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(root, ConfigFile))
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			if string(raw["groups"]) != "[]" {
				t.Errorf("groups = %s, want []", raw["groups"])
			}
			read, err := Read(root)
			if err != nil {
				t.Fatal(err)
			}
			if result := Validate(read, DefaultSettings()); !result.OK || result.CommandCount != 0 {
				t.Fatalf("empty config must validate: %#v", result)
			}
		})
	}
}
