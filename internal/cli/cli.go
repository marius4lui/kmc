package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/marius4lui/kmc/internal/config"
	"github.com/marius4lui/kmc/internal/devurls"
	"github.com/marius4lui/kmc/internal/importers"
	"github.com/marius4lui/kmc/internal/runner"
	"github.com/marius4lui/kmc/internal/trust"
	"github.com/marius4lui/kmc/internal/update"
	"github.com/marius4lui/kmc/internal/workflows"
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func ExitCode(err error) int {
	var target *exitError
	if errors.As(err, &target) {
		return target.code
	}
	return 1
}

func Run(ctx context.Context, args []string, version string) error {
	if len(args) == 0 {
		return runInteractiveUI(ctx)
	}
	switch args[0] {
	case "-h", "--help", "help":
		printHelp()
		return nil
	case "-v", "--version", "version":
		fmt.Println(version)
		return nil
	case "run":
		if len(args) < 2 {
			return errors.New("run requires a command or script id")
		}
		return runID(ctx, args[1], args[2:])
	case "scripts":
		return scripts(ctx, args[1:])
	case "trust":
		return trustCommand(args[1:])
	case "untrust":
		removed, err := trust.Untrust(mustCWD())
		if err == nil {
			if removed {
				fmt.Println("Repository trust removed.")
			} else {
				fmt.Println("Repository was not trusted.")
			}
		}
		return err
	case "validate":
		return validate()
	case "import":
		if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
			return importUI(mustCWD())
		}
		return importCommands()
	case "add":
		if len(args) == 1 && isTerminal(os.Stdin) && isTerminal(os.Stdout) {
			return commandFormUI(mustCWD(), "")
		}
		return addCommand(args[1:])
	case "edit":
		if len(args) == 2 && isTerminal(os.Stdin) && isTerminal(os.Stdout) {
			return commandFormUI(mustCWD(), args[1])
		}
		return editCommand(args[1:])
	case "delete", "remove":
		if len(args) == 1 && isTerminal(os.Stdin) && isTerminal(os.Stdout) {
			id, err := pickCommand(mustCWD(), "Delete which command?", true)
			if err != nil || id == "" {
				return err
			}
			confirmed, err := confirmOne(fmt.Sprintf("Delete %q?", id), false)
			if err != nil || !confirmed {
				return err
			}
			return deleteCommand([]string{id})
		}
		return deleteCommand(args[1:])
	case "settings":
		if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
			return settingsUI(mustCWD())
		}
		return printSettings()
	case "update":
		return updateCommand(ctx, args[1:], version)
	case "channel":
		return channelCommand(args[1:])
	case "doctor":
		return doctor(version)
	case "dev":
		return devCommand(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q; run \"kmc help\"", args[0])
	}
}

func printHelp() {
	fmt.Print(`kmc — interactive project command center

Usage:
  kmc
  kmc run <command-id>
  kmc scripts list|validate|init
  kmc scripts run <script> [--dry-run] [--step <step>] [--env KEY=value]
  kmc trust [status]
  kmc untrust
  kmc add [--name NAME --command COMMAND --group GROUP --cwd PATH]
  kmc edit <command-id> [options]
  kmc delete <command-id>
  kmc import
  kmc validate
  kmc settings
  kmc update [--check] [--channel stable|experimental|nightly] [--version VERSION]
  kmc channel [set stable|experimental|nightly]
  kmc doctor
  kmc dev detect|configure|start|reload|trust
`)
}

func mustCWD() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func validate() error {
	root := mustCWD()
	value, err := config.Read(root)
	if err != nil {
		return err
	}
	settings, err := config.ReadSettings(root)
	if err != nil {
		return err
	}
	result := config.Validate(value, settings)
	fmt.Println("Validate kmc project")
	for _, check := range result.Checks {
		status := "OK"
		if !check.OK {
			status = "FAIL"
		}
		detail := ""
		if check.Detail != "" {
			detail = ": " + check.Detail
		}
		fmt.Printf("%-4s %s%s\n", status, check.Label, detail)
	}
	fmt.Printf("%d command(s) configured.\n", result.CommandCount)
	if !result.OK {
		return &exitError{code: 1, err: errors.New("validation failed")}
	}
	fmt.Println("Everything looks okay.")
	return nil
}

func runID(ctx context.Context, id string, args []string) error {
	if _, err := os.Stat(filepath.Join(mustCWD(), ".kmc", "scripts.yml")); err == nil {
		loaded, loadErr := workflows.Load(mustCWD())
		if loadErr != nil {
			return loadErr
		}
		if script := loaded.Scripts[id]; script != nil {
			return runWorkflow(ctx, loaded, script, args)
		}
	}
	value, err := config.Read(mustCWD())
	if err != nil {
		return err
	}
	commands, err := config.Commands(value)
	if err != nil {
		return err
	}
	command, found := config.FindCommand(commands, id)
	if !found {
		return fmt.Errorf("command %q was not found", id)
	}
	return runShellCommand(ctx, command)
}

func runShellCommand(ctx context.Context, command config.Command) error {
	cwd := filepath.Join(mustCWD(), command.CWD)
	var process *exec.Cmd
	if runtime.GOOS == "windows" {
		process = exec.CommandContext(ctx, "cmd", "/d", "/s", "/c", command.Command)
	} else {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "sh"
		}
		process = exec.CommandContext(ctx, shell, "-c", command.Command)
	}
	process.Dir = cwd
	process.Stdin, process.Stdout, process.Stderr = os.Stdin, os.Stdout, os.Stderr
	fmt.Printf("$ %s\n\n", command.Command)
	if err := process.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return &exitError{code: exit.ExitCode(), err: err}
		}
		return err
	}
	return nil
}

func scripts(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("scripts requires list, validate, init, or run")
	}
	root := mustCWD()
	switch args[0] {
	case "init":
		return initScripts(root)
	case "list":
		loaded, err := workflows.Load(root)
		if err != nil {
			return err
		}
		ids := make([]string, 0, len(loaded.Scripts))
		for id := range loaded.Scripts {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			script := loaded.Scripts[id]
			fmt.Printf("%s\t%s\n", id, script.Description)
		}
		return nil
	case "validate":
		loaded, err := workflows.Load(root)
		if err != nil {
			return err
		}
		fmt.Printf("%d script(s) valid.\n", len(loaded.Scripts))
		return nil
	case "run":
		if len(args) < 2 {
			return errors.New("scripts run requires a script id")
		}
		loaded, err := workflows.Load(root)
		if err != nil {
			return err
		}
		script := loaded.Scripts[args[1]]
		if script == nil {
			return fmt.Errorf("script %q was not found", args[1])
		}
		return runWorkflow(ctx, loaded, script, args[2:])
	default:
		return fmt.Errorf("unknown scripts command %q", args[0])
	}
}

func runWorkflow(ctx context.Context, loaded *workflows.Loaded, script *workflows.Script, args []string) error {
	options, yes, err := parseWorkflowOptions(args)
	if err != nil {
		return err
	}
	status, err := trust.StatusFor(loaded)
	if err != nil {
		return err
	}
	if !status.Trusted && !options.DryRun {
		if !yes {
			return errors.New("repository is not trusted; review workflows and run \"kmc trust\" or pass --yes")
		}
		if err := trust.Trust(loaded); err != nil {
			return err
		}
	}
	options.OnEvent = func(event runner.Event) {
		switch event.Type {
		case "stepStarted":
			fmt.Printf("[%d] %s\n$ %s\n", event.Step.Index+1, event.Step.Name, event.Step.Run)
		case "stepRetried":
			fmt.Printf("Retry %d/%d\n", event.Attempt, event.Step.Retries)
		}
	}
	result := runner.RunWorkflow(ctx, script, loaded.ProjectRoot, options)
	if result.DryRun {
		for _, step := range result.Plan {
			fmt.Printf("[%d] %s\n  cwd: %s\n  shell: %s\n  $ %s\n", step.Index+1, step.Name, step.CWD, step.Shell, step.Run)
		}
	}
	if !result.OK {
		return &exitError{code: result.ExitCode, err: fmt.Errorf("workflow %q failed", script.ID)}
	}
	return nil
}

func parseWorkflowOptions(args []string) (runner.Options, bool, error) {
	options := runner.Options{Env: map[string]string{}}
	yes := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--dry-run":
			options.DryRun = true
		case "--yes":
			yes = true
		case "--step":
			index++
			if index >= len(args) {
				return options, yes, errors.New("--step requires a value")
			}
			options.Step = args[index]
		case "--env":
			index++
			if index >= len(args) {
				return options, yes, errors.New("--env requires KEY=value")
			}
			key, value, ok := strings.Cut(args[index], "=")
			if !ok || key == "" {
				return options, yes, fmt.Errorf("invalid environment value %q", args[index])
			}
			options.Env[key] = value
		case "--verbose", "--no-color":
		default:
			return options, yes, fmt.Errorf("unknown workflow option %q", args[index])
		}
	}
	return options, yes, nil
}

func trustCommand(args []string) error {
	loaded, err := workflows.Load(mustCWD())
	if err != nil {
		return err
	}
	status, err := trust.StatusFor(loaded)
	if err != nil {
		return err
	}
	if len(args) > 0 && args[0] == "status" {
		switch {
		case status.Trusted:
			fmt.Println("trusted")
		case status.Changed:
			fmt.Println("not trusted (workflow files changed)")
		default:
			fmt.Println("not trusted")
		}
		return nil
	}
	if err := trust.Trust(loaded); err != nil {
		return err
	}
	fmt.Printf("Trusted %s\n", loaded.ProjectRoot)
	return nil
}

func initScripts(root string) error {
	dir := filepath.Join(root, ".kmc", "scripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	registry := filepath.Join(root, ".kmc", "scripts.yml")
	workflow := filepath.Join(dir, "test.yml")
	if _, err := os.Stat(registry); errors.Is(err, os.ErrNotExist) {
		content := "version: 1\n\nscripts:\n  test:\n    file: ./scripts/test.yml\n    description: Run project tests\n"
		if err := os.WriteFile(registry, []byte(content), 0o644); err != nil {
			return err
		}
	}
	if _, err := os.Stat(workflow); errors.Is(err, os.ErrNotExist) {
		content := "name: Project tests\nsteps:\n  - name: Test\n    run: echo \"Configure your tests in .kmc/scripts/test.yml\"\n"
		if err := os.WriteFile(workflow, []byte(content), 0o644); err != nil {
			return err
		}
	}
	fmt.Println("KMC Scripts initialized in .kmc/")
	return nil
}

func importCommands() error {
	root := mustCWD()
	current, err := config.Read(root)
	if err != nil {
		return err
	}
	next, err := importers.Import(root, current, nil)
	if err != nil {
		return err
	}
	commands, _ := config.Commands(next)
	fmt.Printf("Imported %d commands.\n", len(commands))
	return nil
}

type commandFields struct {
	name, command, description, cwd, group string
}

func parseCommandFields(args []string) (commandFields, []string, error) {
	fields := commandFields{cwd: ".", group: "manual"}
	var positional []string
	for index := 0; index < len(args); index++ {
		if !strings.HasPrefix(args[index], "--") {
			positional = append(positional, args[index])
			continue
		}
		if index+1 >= len(args) {
			return fields, positional, fmt.Errorf("%s requires a value", args[index])
		}
		value := args[index+1]
		index++
		switch args[index-1] {
		case "--name":
			fields.name = value
		case "--command":
			fields.command = value
		case "--description":
			fields.description = value
		case "--cwd":
			fields.cwd = value
		case "--group":
			fields.group = value
		default:
			return fields, positional, fmt.Errorf("unknown option %q", args[index-1])
		}
	}
	return fields, positional, nil
}

func addCommand(args []string) error {
	fields, _, err := parseCommandFields(args)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(os.Stdin)
	if fields.name == "" {
		fields.name = prompt(reader, "Name")
	}
	if fields.command == "" {
		fields.command = prompt(reader, "Command")
	}
	return upsertCommand("", fields)
}

func editCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("edit requires a command id")
	}
	fields, positional, err := parseCommandFields(args)
	if err != nil {
		return err
	}
	if len(positional) == 0 {
		return errors.New("edit requires a command id")
	}
	return upsertCommand(positional[0], fields)
}

func upsertCommand(existingID string, fields commandFields) error {
	root := mustCWD()
	value, err := config.Read(root)
	if err != nil {
		return err
	}
	commands, _ := config.Commands(value)
	var existing config.Command
	if existingID != "" {
		var found bool
		existing, found = config.FindCommand(commands, existingID)
		if !found {
			return fmt.Errorf("command %q was not found", existingID)
		}
	}
	if fields.name == "" {
		fields.name = existing.Name
	}
	if fields.command == "" {
		fields.command = existing.Command
	}
	if fields.description == "" {
		fields.description = existing.Description
	}
	if fields.cwd == "." && existing.CWD != "" {
		fields.cwd = existing.CWD
	}
	if fields.group == "manual" && existing.GroupID != "" {
		fields.group = existing.GroupID
	}
	next := config.Command{Name: fields.name, Command: fields.command, Description: fields.description, CWD: fields.cwd, GroupID: fields.group, Group: fields.group}
	next, err = config.NormalizeCommand(next, fields.group)
	if err != nil {
		return err
	}
	groups := value.Groups
	for index := range groups {
		filtered := groups[index].Commands[:0]
		for _, command := range groups[index].Commands {
			if config.CommandID(command) != existingID && config.CommandID(command) != config.CommandID(next) {
				filtered = append(filtered, command)
			}
		}
		groups[index].Commands = filtered
	}
	groupIndex := -1
	for index := range groups {
		if groups[index].ID == fields.group {
			groupIndex = index
		}
	}
	if groupIndex < 0 {
		groups = append(groups, config.GroupShell(fields.group, false))
		groupIndex = len(groups) - 1
	}
	groups[groupIndex].Commands = append(groups[groupIndex].Commands, next)
	value.Groups = groups
	if err := config.Write(root, value); err != nil {
		return err
	}
	fmt.Printf("Saved %s\n", config.CommandID(next))
	return nil
}

func deleteCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("delete requires a command id")
	}
	root := mustCWD()
	value, err := config.Read(root)
	if err != nil {
		return err
	}
	found := false
	for index := range value.Groups {
		filtered := value.Groups[index].Commands[:0]
		for _, command := range value.Groups[index].Commands {
			if config.CommandID(command) == args[0] || command.Name == args[0] {
				found = true
				continue
			}
			filtered = append(filtered, command)
		}
		value.Groups[index].Commands = filtered
	}
	if !found {
		return fmt.Errorf("command %q was not found", args[0])
	}
	if err := config.Write(root, value); err != nil {
		return err
	}
	fmt.Printf("Deleted %s\n", args[0])
	return nil
}

func printSettings() error {
	settings, err := config.ReadSettings(mustCWD())
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	fmt.Println(string(data))
	return nil
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Printf("%s: ", label)
	value, _ := reader.ReadString('\n')
	return strings.TrimSpace(value)
}

type updateSettings struct {
	Channel string `json:"channel"`
}

func updateSettingsFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kmc", "update.json"), nil
}

func readUpdateSettings() updateSettings {
	result := updateSettings{Channel: update.Stable}
	file, err := updateSettingsFile()
	if err != nil {
		return result
	}
	data, err := os.ReadFile(file)
	if err == nil {
		_ = json.Unmarshal(data, &result)
	}
	return result
}

func writeUpdateSettings(settings updateSettings) error {
	file, err := updateSettingsFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	return os.WriteFile(file, append(data, '\n'), 0o644)
}

func channelCommand(args []string) error {
	settings := readUpdateSettings()
	if len(args) == 0 {
		fmt.Println(settings.Channel)
		return nil
	}
	if len(args) != 2 || args[0] != "set" {
		return errors.New("usage: kmc channel set stable|experimental|nightly")
	}
	if args[1] != update.Stable && args[1] != update.Experimental && args[1] != update.Nightly {
		return fmt.Errorf("unknown channel %q", args[1])
	}
	settings.Channel = args[1]
	if err := writeUpdateSettings(settings); err != nil {
		return err
	}
	fmt.Printf("Update channel set to %s.\n", settings.Channel)
	return nil
}

func updateCommand(ctx context.Context, args []string, currentVersion string) error {
	settings := readUpdateSettings()
	channel, targetVersion := settings.Channel, ""
	check := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--check":
			check = true
		case "--channel":
			index++
			if index >= len(args) {
				return errors.New("--channel requires a value")
			}
			channel = args[index]
		case "--version":
			index++
			if index >= len(args) {
				return errors.New("--version requires a value")
			}
			targetVersion = args[index]
		default:
			return fmt.Errorf("unknown update option %q", args[index])
		}
	}
	client := update.NewClient("")
	releases, err := client.Releases(ctx)
	if err != nil {
		return err
	}
	release, err := update.Resolve(releases, channel, targetVersion)
	if err != nil {
		return err
	}
	if check {
		if update.CompareVersions(release.TagName, currentVersion) > 0 {
			fmt.Printf("Update available: %s -> %s (%s)\n", currentVersion, release.TagName, channel)
		} else {
			fmt.Printf("kmc %s is up to date on %s.\n", currentVersion, channel)
		}
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return errors.New("self-update on Windows is handled by install.ps1; run the installer with -Command update")
	}
	asset, err := update.CurrentPlatformAsset(*release)
	if err != nil {
		return err
	}
	checksumAsset, ok := update.FindAsset(*release, "kmc_checksums.txt")
	if !ok {
		return errors.New("release is missing kmc_checksums.txt")
	}
	temp, err := os.MkdirTemp("", "kmc-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	archive := filepath.Join(temp, asset.Name)
	checksums := filepath.Join(temp, checksumAsset.Name)
	if err := client.Download(ctx, asset, archive); err != nil {
		return err
	}
	if err := client.Download(ctx, checksumAsset, checksums); err != nil {
		return err
	}
	checksumData, err := os.ReadFile(checksums)
	if err != nil {
		return err
	}
	expected, err := update.ChecksumFor(checksumData, asset.Name)
	if err != nil {
		return err
	}
	if err := update.VerifyFile(archive, expected); err != nil {
		return err
	}
	binary := filepath.Join(temp, "kmc")
	if err := update.ExtractBinary(archive, binary, runtime.GOOS); err != nil {
		return err
	}
	verify := exec.Command(binary, "--version")
	if output, verifyErr := verify.CombinedOutput(); verifyErr != nil {
		return fmt.Errorf("downloaded binary failed verification: %w: %s", verifyErr, strings.TrimSpace(string(output)))
	}
	if err := update.AtomicReplace(binary, executable); err != nil {
		return err
	}
	settings.Channel = channel
	_ = writeUpdateSettings(settings)
	fmt.Printf("Updated kmc to %s (%s).\n", release.TagName, channel)
	return nil
}

func doctor(version string) error {
	executable, _ := os.Executable()
	settings := readUpdateSettings()
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Binary: %s\n", executable)
	fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Channel: %s\n", settings.Channel)
	if configDir, err := os.UserConfigDir(); err == nil {
		fmt.Printf("Config: %s\n", filepath.Join(configDir, "kmc"))
	}
	if npm, err := exec.LookPath("npm"); err == nil {
		command := exec.Command(npm, "root", "-g")
		if output, runErr := command.Output(); runErr == nil {
			legacy := filepath.Join(strings.TrimSpace(string(output)), "@marius4lui", "kmc")
			if _, statErr := os.Stat(legacy); statErr == nil {
				fmt.Printf("Legacy npm installation: %s\n", legacy)
			}
		}
	}
	return nil
}

func devCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("dev requires detect, configure, start, reload, or trust")
	}
	root := mustCWD()
	switch args[0] {
	case "detect":
		searchRoot, err := devurls.FindProjectSearchRoot(root)
		if err != nil {
			return err
		}
		projects, err := devurls.DiscoverProjects(searchRoot)
		if err != nil {
			return err
		}
		for _, project := range projects {
			fmt.Printf("%s\t%s\t%s\n", project.Name, project.Label, project.Root)
		}
		if len(projects) == 0 {
			return errors.New("no supported dev project detected")
		}
		return nil
	case "configure":
		searchRoot, err := devurls.FindProjectSearchRoot(root)
		if err != nil {
			return err
		}
		projects, err := devurls.DiscoverProjects(searchRoot)
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			return errors.New("no supported dev project detected")
		}
		selected := projects[0]
		if len(args) > 1 {
			found := false
			for _, project := range projects {
				if project.Name == args[1] || project.Root == args[1] {
					selected, found = project, true
					break
				}
			}
			if !found {
				return fmt.Errorf("dev project %q was not found", args[1])
			}
		}
		value, err := devurls.EnsureConfig(root, selected)
		if err != nil {
			return err
		}
		caddyfile, err := devurls.UpsertCaddySite(*value, root)
		if err != nil {
			return err
		}
		fmt.Printf("Configured https://%s -> localhost:%d\nCaddyfile: %s\n", value.Host, value.Port, caddyfile)
		return nil
	case "start":
		value, err := devurls.ReadConfig(root)
		if err != nil {
			return err
		}
		if value == nil {
			return errors.New("dev URL is not configured; run \"kmc dev configure\"")
		}
		command, err := devurls.StartCommand(*value)
		if err != nil {
			return err
		}
		fmt.Printf("URL: https://%s\n$ %s\n", value.Host, command)
		var process *exec.Cmd
		if runtime.GOOS == "windows" {
			process = exec.CommandContext(ctx, "cmd", "/d", "/s", "/c", command)
		} else {
			process = exec.CommandContext(ctx, "sh", "-c", command)
		}
		process.Dir = value.Root
		process.Stdin, process.Stdout, process.Stderr = os.Stdin, os.Stdout, os.Stderr
		return process.Run()
	case "reload":
		dir, err := devurls.ConfigDir()
		if err != nil {
			return err
		}
		return devurls.ReloadCaddy(filepath.Join(dir, "Caddyfile"))
	case "trust":
		return devurls.TrustCaddy()
	default:
		return fmt.Errorf("unknown dev command %q", args[0])
	}
}
