package trust

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marius4lui/kmc/internal/workflows"
)

func TestTrustCompatibleStoreAndFingerprintChanges(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KMC_CONFIG_HOME", filepath.Join(root, "config"))
	if err := os.MkdirAll(filepath.Join(root, ".kmc", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(root, ".kmc", "scripts.yml")
	workflow := filepath.Join(root, ".kmc", "scripts", "test.yml")
	if err := os.WriteFile(registry, []byte("version: 1\nscripts:\n  test:\n    file: scripts/test.yml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workflow, []byte("steps:\n  - name: Before\n    run: echo before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := workflows.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Trust(loaded); err != nil {
		t.Fatal(err)
	}
	status, err := StatusFor(loaded)
	if err != nil || !status.Trusted || status.Changed {
		t.Fatalf("status = %#v, err = %v", status, err)
	}
	if err := os.WriteFile(workflow, []byte("steps:\n  - name: After\n    run: echo after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := workflows.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	status, err = StatusFor(changed)
	if err != nil || status.Trusted || !status.Changed {
		t.Fatalf("changed status = %#v, err = %v", status, err)
	}
	removed, err := Untrust(root)
	if err != nil || !removed {
		t.Fatalf("removed = %v, err = %v", removed, err)
	}
}
