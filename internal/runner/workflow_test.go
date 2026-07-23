package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/marius4lui/kmc/internal/workflows"
)

func TestBuildPlanInheritsDefaultsAndEnvironment(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := &workflows.Script{Workflow: workflows.Workflow{
		Env:      map[string]any{"A": "workflow", "B": 1},
		Defaults: workflows.Defaults{Shell: "sh", CWD: "sub"},
		Steps:    []workflows.Step{{Name: "Run tests", Run: "true", Env: map[string]any{"A": "step"}}},
	}}
	plan, err := BuildPlan(script, root, Options{Step: "run-tests", Env: map[string]string{"C": "cli"}})
	if err != nil {
		t.Fatal(err)
	}
	env := envMap(plan[0].Env)
	if plan[0].Shell != "sh" || plan[0].CWD != filepath.Join(root, "sub") {
		t.Fatalf("defaults not inherited: %#v", plan[0])
	}
	if env["A"] != "step" || env["B"] != "1" || env["C"] != "cli" {
		t.Fatalf("env = %#v", env)
	}
}

func TestRunnerRetriesAndContinues(t *testing.T) {
	if _, err := execShell(); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	marker := filepath.Join(root, "marker")
	script := &workflows.Script{Workflow: workflows.Workflow{Steps: []workflows.Step{
		{Name: "retry", Run: "test -f '" + marker + "' || (touch '" + marker + "'; exit 1)", Shell: "sh", Retries: 1},
		{Name: "ignored", Run: "exit 2", Shell: "sh", ContinueOnError: true},
		{Name: "finish", Run: "true", Shell: "sh"},
	}}}
	result := RunWorkflow(context.Background(), script, root, Options{})
	if !result.OK || len(result.Results) != 3 {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunnerTimeoutStopsLaterSteps(t *testing.T) {
	if _, err := execShell(); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	script := &workflows.Script{Workflow: workflows.Workflow{Steps: []workflows.Step{
		{Name: "slow", Run: "sleep 2", Shell: "sh", Timeout: 1},
		{Name: "later", Run: "true", Shell: "sh"},
	}}}
	result := RunWorkflow(context.Background(), script, root, Options{})
	if result.OK || result.ExitCode != 124 || len(result.Results) != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestDryRunDoesNotExecute(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "marker")
	script := &workflows.Script{Workflow: workflows.Workflow{Steps: []workflows.Step{{
		Name: "write", Run: "touch '" + marker + "'", Shell: "sh",
	}}}}
	result := RunWorkflow(context.Background(), script, root, Options{DryRun: true})
	if !result.OK || !result.DryRun || len(result.Plan) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("dry run created marker: %v", err)
	}
}

func execShell() (Shell, error) { return ResolveShell("sh") }
