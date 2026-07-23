package workflows

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validRegistry = "version: 1\nscripts:\n  test:\n    file: ./scripts/test.yml\n    description: Run checks\n"

func workflowProject(t *testing.T, registry, workflow string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".kmc", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".kmc", "scripts.yml"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	if workflow != "" {
		if err := os.WriteFile(filepath.Join(root, ".kmc", "scripts", "test.yml"), []byte(workflow), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLoadAndValidate(t *testing.T) {
	root := workflowProject(t, validRegistry, "name: Checks\nenv:\n  BASE: yes\ndefaults:\n  shell: sh\nsteps:\n  - name: Test all\n    run: echo ok\n")
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Scripts["test"].Workflow.Steps[0].Name; got != "Test all" {
		t.Fatalf("step name = %q", got)
	}
}

func TestRejectsUnknownFieldsTraversalAndInvalidNumbers(t *testing.T) {
	root := workflowProject(t, validRegistry, "steps:\n  - name: Same\n    run: echo nope\n    uses: build\n    cwd: ../../\n    timeout: 0\n  - name: Same\n    run: echo 2\n    retries: -1\n")
	_, err := Load(root)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %T %v", err, err)
	}
	for _, want := range []string{`field "uses" is not supported`, "escapes the project root", "positive integer", "non-negative integer", "duplicated"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%s", want, err)
		}
	}
}

func TestSelectSteps(t *testing.T) {
	workflow := Workflow{Steps: []Step{{Name: "Run tests"}, {Name: "Package"}}}
	for selector, want := range map[string]string{"1": "Run tests", "run-tests": "Run tests", "Package": "Package"} {
		selected, err := SelectSteps(workflow, selector)
		if err != nil {
			t.Fatal(err)
		}
		if selected[0].Name != want {
			t.Errorf("selector %q selected %q", selector, selected[0].Name)
		}
	}
	if _, err := SelectSteps(workflow, "missing"); err == nil {
		t.Fatal("expected missing selector error")
	}
}

func TestRejectsSymlinkedWorkflowOutsideRoot(t *testing.T) {
	root := workflowProject(t, validRegistry, "")
	outside := filepath.Join(t.TempDir(), "outside.yml")
	if err := os.WriteFile(outside, []byte("steps:\n  - name: nope\n    run: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".kmc", "scripts", "test.yml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "outside the project root") {
		t.Fatalf("error = %v", err)
	}
}
