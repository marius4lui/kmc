package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/marius4lui/kmc/internal/workflows"
)

type PlanStep struct {
	workflows.Step
	CWD             string
	Shell           string
	Env             []string
	Retries         int
	ContinueOnError bool
}

type Result struct {
	Step     PlanStep
	Code     int
	TimedOut bool
	Err      error
}

type RunResult struct {
	OK       bool
	DryRun   bool
	Plan     []PlanStep
	Results  []Result
	ExitCode int
}

type Event struct {
	Type    string
	Step    PlanStep
	Attempt int
	Result  *Result
}

type Options struct {
	Step    string
	Env     map[string]string
	DryRun  bool
	OnEvent func(Event)
}

type Shell struct {
	Command string
	Args    []string
}

func BuildPlan(script *workflows.Script, projectRoot string, options Options) ([]PlanStep, error) {
	selected, err := workflows.SelectSteps(script.Workflow, options.Step)
	if err != nil {
		return nil, err
	}
	inherited := envMap(os.Environ())
	for key, value := range scalarEnv(script.Workflow.Env) {
		inherited[key] = value
	}
	plan := make([]PlanStep, 0, len(selected))
	for _, item := range selected {
		cwd := item.CWD
		if cwd == "" {
			cwd = script.Workflow.Defaults.CWD
		}
		if cwd == "" {
			cwd = "."
		}
		cwd = resolvePath(projectRoot, cwd)
		if !inside(projectRoot, cwd) {
			return nil, fmt.Errorf("step %q cwd escapes the project root", item.Name)
		}
		env := cloneMap(inherited)
		for key, value := range scalarEnv(item.Env) {
			env[key] = value
		}
		for key, value := range options.Env {
			env[key] = value
		}
		shell := item.Shell
		if shell == "" {
			shell = script.Workflow.Defaults.Shell
		}
		plan = append(plan, PlanStep{
			Step: item, CWD: cwd, Shell: shell, Env: envSlice(env),
			Retries: item.Retries, ContinueOnError: item.ContinueOnError,
		})
	}
	return plan, nil
}

func RunWorkflow(ctx context.Context, script *workflows.Script, projectRoot string, options Options) RunResult {
	plan, err := BuildPlan(script, projectRoot, options)
	if err != nil {
		return RunResult{OK: false, ExitCode: 1, Results: []Result{{Err: err}}}
	}
	if options.DryRun {
		return RunResult{OK: true, DryRun: true, Plan: plan}
	}
	output := RunResult{OK: true, Plan: plan}
	for _, step := range plan {
		emit(options, Event{Type: "stepStarted", Step: step})
		var result Result
		for attempt := 0; attempt <= step.Retries; attempt++ {
			if attempt > 0 {
				emit(options, Event{Type: "stepRetried", Step: step, Attempt: attempt})
			}
			result = Execute(ctx, step)
			if result.Code == 0 && !result.TimedOut {
				break
			}
		}
		output.Results = append(output.Results, result)
		if result.Code == 0 && !result.TimedOut {
			emit(options, Event{Type: "stepSucceeded", Step: step, Result: &result})
			continue
		}
		emit(options, Event{Type: "stepFailed", Step: step, Result: &result})
		if !step.ContinueOnError {
			output.OK = false
			output.ExitCode = result.Code
			if result.TimedOut {
				output.ExitCode = 124
			}
			return output
		}
	}
	return output
}

func Execute(parent context.Context, step PlanStep) Result {
	ctx := parent
	cancel := func() {}
	if step.Timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, time.Duration(step.Timeout)*time.Second)
	}
	defer cancel()
	shell, err := ResolveShell(step.Shell)
	if err != nil {
		return Result{Step: step, Code: 1, Err: err}
	}
	args := append(append([]string(nil), shell.Args...), step.Run)
	command := exec.Command(shell.Command, args...)
	command.Dir = step.CWD
	command.Env = step.Env
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	configureProcessGroup(command)
	if err := command.Start(); err != nil {
		return Result{Step: step, Code: 1, Err: err}
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		return processResult(step, err, false)
	case <-ctx.Done():
		killProcessGroup(command.Process)
		err := <-done
		timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
		result := processResult(step, err, timedOut)
		if !timedOut {
			result.Code = 130
		}
		return result
	}
}

func ResolveShell(requested string) (Shell, error) {
	if requested == "" {
		if runtime.GOOS == "windows" {
			if _, err := exec.LookPath("pwsh"); err == nil {
				requested = "pwsh"
			} else {
				requested = "powershell"
			}
		} else if requested = os.Getenv("SHELL"); requested == "" {
			requested = "sh"
		}
	}
	name := strings.ToLower(filepath.Base(requested))
	allowed := map[string]bool{"bash": true, "sh": true, "zsh": true, "dash": true, "ksh": true}
	if runtime.GOOS == "windows" {
		allowed = map[string]bool{"powershell": true, "powershell.exe": true, "pwsh": true, "pwsh.exe": true, "cmd": true, "cmd.exe": true}
	}
	if !allowed[name] {
		return Shell{}, fmt.Errorf("shell %q is not supported on this platform", requested)
	}
	command, err := exec.LookPath(requested)
	if err != nil {
		return Shell{}, fmt.Errorf("shell %q was not found on this system", requested)
	}
	switch name {
	case "cmd", "cmd.exe":
		return Shell{Command: command, Args: []string{"/d", "/s", "/c"}}, nil
	case "powershell", "powershell.exe", "pwsh", "pwsh.exe":
		return Shell{Command: command, Args: []string{"-NoLogo", "-NonInteractive", "-Command"}}, nil
	default:
		return Shell{Command: command, Args: []string{"-c"}}, nil
	}
}

func processResult(step PlanStep, err error, timedOut bool) Result {
	result := Result{Step: step, TimedOut: timedOut, Err: err}
	if err == nil {
		return result
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.Code = exitErr.ExitCode()
	} else {
		result.Code = 1
	}
	if timedOut {
		result.Code = 124
	}
	return result
}

func scalarEnv(input map[string]any) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		if value == nil {
			result[key] = ""
		} else {
			result[key] = fmt.Sprint(value)
		}
	}
	return result
}

func envMap(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		key, item, ok := strings.Cut(value, "=")
		if ok {
			result[key] = item
		}
	}
	return result
}

func envSlice(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sortStrings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func cloneMap(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func inside(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func resolvePath(base, target string) string {
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Clean(filepath.Join(base, target))
}

func emit(options Options, event Event) {
	if options.OnEvent != nil {
		options.OnEvent(event)
	}
}
