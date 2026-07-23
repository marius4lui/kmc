package cli

import (
	"testing"

	"github.com/marius4lui/kmc/internal/config"
)

func TestUIGroupLabelsAndVisibility(t *testing.T) {
	cfg := config.Config{Groups: []config.Group{
		{ID: "manual", Label: "Manual Commands", Type: "manual", Commands: []config.Command{{Name: "dev", Command: "echo dev"}}},
		{ID: "hidden", Label: "Hidden", Type: "manual", Commands: []config.Command{{Name: "secret", Command: "echo secret"}}},
	}}
	settings := config.DefaultSettings()
	settings.HiddenGroups = []string{"hidden"}
	if got := visibleGroupCount(cfg, settings); got != 1 {
		t.Fatalf("visible groups = %d, want 1", got)
	}
	if got := groupLabel(cfg, "manual"); got != "Manual Commands" {
		t.Fatalf("group label = %q", got)
	}
}

func TestUIGroupIDValidation(t *testing.T) {
	for _, value := range []string{"manual", "my-tools", "my.tools", "my_tools2"} {
		if !validGroupID(value) {
			t.Fatalf("%q should be valid", value)
		}
	}
	for _, value := range []string{"", "My Tools", "../escape", "ümlaut"} {
		if validGroupID(value) {
			t.Fatalf("%q should be invalid", value)
		}
	}
}
