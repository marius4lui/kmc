package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/marius4lui/kmc/internal/config"
	"github.com/marius4lui/kmc/internal/devurls"
	"github.com/marius4lui/kmc/internal/importers"
	"github.com/marius4lui/kmc/internal/trust"
	"github.com/marius4lui/kmc/internal/workflows"
)

var (
	cyan   = "\x1b[36m"
	green  = "\x1b[32m"
	red    = "\x1b[31m"
	yellow = "\x1b[33m"
	bold   = "\x1b[1m"
	dim    = "\x1b[2m"
	reset  = "\x1b[0m"
)

func init() {
	if os.Getenv("NO_COLOR") != "" {
		cyan, green, red, yellow, bold, dim, reset = "", "", "", "", "", "", ""
	}
}

func clearScreen() {
	if isTerminal(os.Stdout) {
		fmt.Print("\x1b[2J\x1b[3J\x1b[H")
	}
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func banner(commandCount int) {
	clearScreen()
	label := "commands"
	if commandCount == 1 {
		label = "command"
	}
	fmt.Printf("%s╭────────────────────────────────────────╮%s\n", cyan, reset)
	fmt.Printf("%s│%s %skmc%s %sinteractive command center%s       %s│%s\n", cyan, reset, bold, reset, dim, reset, cyan, reset)
	line := fmt.Sprintf("kmc.json · %d %s", commandCount, label)
	fmt.Printf("%s│%s %s%-38s%s%s│%s\n", cyan, reset, dim, line, reset, cyan, reset)
	fmt.Printf("%s╰────────────────────────────────────────╯%s\n\n", cyan, reset)
}

func selectOne(message string, options []string, defaultValue string) (string, error) {
	answer := ""
	prompt := &survey.Select{Message: message, Options: options, PageSize: 12}
	for _, option := range options {
		if defaultValue != "" && option == defaultValue {
			prompt.Default = defaultValue
			answer = defaultValue
			break
		}
	}
	err := survey.AskOne(prompt, &answer)
	return answer, err
}

func multiSelect(message string, options, defaults []string) ([]string, error) {
	answer := append([]string(nil), defaults...)
	err := survey.AskOne(&survey.MultiSelect{Message: message, Options: options, Default: defaults, PageSize: 12}, &answer)
	return answer, err
}

func inputOne(message, defaultValue string, required bool) (string, error) {
	answer := defaultValue
	prompt := &survey.Input{Message: message, Default: defaultValue}
	var validators []survey.Validator
	if required {
		validators = append(validators, survey.Required)
	}
	err := survey.AskOne(prompt, &answer, survey.WithValidator(survey.ComposeValidators(validators...)))
	return strings.TrimSpace(answer), err
}

func confirmOne(message string, defaultValue bool) (bool, error) {
	answer := defaultValue
	err := survey.AskOne(&survey.Confirm{Message: message, Default: defaultValue}, &answer)
	return answer, err
}

func waitForEnter() {
	var ignored string
	_ = survey.AskOne(&survey.Input{Message: "OK", Default: ""}, &ignored)
}

func runInteractiveUI(ctx context.Context) error {
	for {
		root := mustCWD()
		cfg, err := config.Read(root)
		if err != nil {
			return err
		}
		settings, err := config.ReadSettings(root)
		if err != nil {
			return err
		}
		commands, _ := config.Commands(cfg)
		banner(len(commands))
		visibleGroups := visibleGroupCount(cfg, settings)
		if len(commands) == 0 {
			fmt.Printf("%sNo commands yet. Add one manually or import detected project tools.%s\n\n", dim, reset)
		} else {
			fmt.Printf("%s%d commands · %d groups", dim, len(commands), visibleGroups)
			if len(settings.FavoriteCommands) > 0 {
				fmt.Printf(" · %d favorites", len(settings.FavoriteCommands))
			}
			fmt.Printf("%s\n\n", reset)
		}
		action, err := selectOne("Choose", []string{
			"Run — Run commands saved in kmc.json",
			"Scripts — Run and manage local YAML workflows",
			"Dev URLs — Manage stable local HTTPS development URLs",
			"Manage — Add, edit, or delete saved commands",
			"Import — Import commands from detected project tools",
			"Preferences — Configure groups, favorites, and local settings",
			"Quit — Exit kmc",
		}, "")
		if err != nil {
			return nil
		}
		switch {
		case strings.HasPrefix(action, "Run —"):
			command, pickErr := groupedCommandPicker(cfg, settings)
			if pickErr != nil {
				return pickErr
			}
			if command != nil {
				return runShellCommand(ctx, *command)
			}
		case strings.HasPrefix(action, "Scripts —"):
			if err := scriptsUI(ctx, root); err != nil {
				showError(err)
			}
		case strings.HasPrefix(action, "Dev URLs —"):
			if err := devURLsUI(ctx, root); err != nil {
				showError(err)
			}
		case strings.HasPrefix(action, "Manage —"):
			if err := manageUI(root); err != nil {
				showError(err)
			}
		case strings.HasPrefix(action, "Import —"):
			if err := importUI(root); err != nil {
				showError(err)
			}
		case strings.HasPrefix(action, "Preferences —"):
			if err := settingsUI(root); err != nil {
				showError(err)
			}
		case strings.HasPrefix(action, "Quit —"):
			return nil
		}
	}
}

func showError(err error) {
	fmt.Printf("\n%s%s%s\n", red, err, reset)
	waitForEnter()
}

func visibleGroupCount(cfg config.Config, settings config.Settings) int {
	hidden := stringSet(settings.HiddenGroups)
	count := 0
	for _, group := range cfg.Groups {
		if !hidden[group.ID] && len(group.Commands) > 0 {
			count++
		}
	}
	return count
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func groupLabel(cfg config.Config, id string) string {
	for _, group := range cfg.Groups {
		if group.ID == id {
			return group.Label
		}
	}
	if defaults, ok := config.KnownGroups[id]; ok && defaults.Label != "" {
		return defaults.Label
	}
	return id
}

func groupedCommandPicker(cfg config.Config, settings config.Settings) (*config.Command, error) {
	commands, err := config.Commands(cfg)
	if err != nil {
		return nil, err
	}
	hidden, favorites := stringSet(settings.HiddenGroups), stringSet(settings.FavoriteCommands)
	byGroup := map[string][]config.Command{}
	var favoriteCommands []config.Command
	for _, command := range commands {
		if hidden[command.GroupID] {
			continue
		}
		byGroup[command.GroupID] = append(byGroup[command.GroupID], command)
		if favorites[config.CommandID(command)] {
			favoriteCommands = append(favoriteCommands, command)
		}
	}
	var groupIDs []string
	addGroup := func(id string) {
		if id == "" || len(byGroup[id]) == 0 {
			return
		}
		for _, existing := range groupIDs {
			if existing == id {
				return
			}
		}
		groupIDs = append(groupIDs, id)
	}
	for _, id := range settings.FavoriteGroups {
		addGroup(id)
	}
	addGroup(settings.LastSelectedGroup)
	addGroup(settings.DefaultGroup)
	for _, id := range []string{"manual", "npm", "make", "flutter", "docker"} {
		addGroup(id)
	}
	var remaining []string
	for id := range byGroup {
		remaining = append(remaining, id)
	}
	sort.Strings(remaining)
	for _, id := range remaining {
		addGroup(id)
	}
	var labels []string
	lookup := map[string]string{}
	if len(favoriteCommands) > 0 {
		label := fmt.Sprintf("★ Favorites — %d pinned commands", len(favoriteCommands))
		labels = append(labels, label)
		lookup[label] = "__favorites"
	}
	for _, id := range groupIDs {
		label := fmt.Sprintf("%s — %d commands", groupLabel(cfg, id), len(byGroup[id]))
		if stringSet(settings.FavoriteGroups)[id] {
			label = "★ " + label
		}
		labels = append(labels, label)
		lookup[label] = id
	}
	labels = append(labels, "Back")
	selected, err := selectOne("Run", labels, "")
	if err != nil || selected == "Back" {
		return nil, err
	}
	groupID := lookup[selected]
	selectedCommands := byGroup[groupID]
	if groupID == "__favorites" {
		selectedCommands = favoriteCommands
	} else {
		settings.LastSelectedGroup = groupID
		_ = config.WriteSettings(mustCWD(), settings)
	}
	var commandLabels []string
	commandLookup := map[string]config.Command{}
	for _, command := range selectedCommands {
		label := command.Name
		if favorites[config.CommandID(command)] {
			label = "★ " + label
		}
		if command.Description != "" {
			label += " — " + command.Description
		}
		commandLabels = append(commandLabels, label)
		commandLookup[label] = command
	}
	commandLabels = append(commandLabels, "Back")
	commandMessage := groupLabel(cfg, groupID)
	if groupID == "__favorites" {
		commandMessage = "Favorites"
	}
	selected, err = selectOne(commandMessage, commandLabels, "")
	if err != nil || selected == "Back" {
		return nil, err
	}
	command := commandLookup[selected]
	return &command, nil
}

func manageUI(root string) error {
	for {
		clearScreen()
		fmt.Printf("%sManage%s\n%sManual commands and existing entries%s\n\n", bold, reset, dim, reset)
		action, err := selectOne("Manage", []string{"Add manual command", "Edit command", "Delete command", "Back"}, "")
		if err != nil || action == "Back" {
			return err
		}
		switch action {
		case "Add manual command":
			err = commandFormUI(root, "")
		case "Edit command":
			id, pickErr := pickCommand(root, "Edit which command?", false)
			if pickErr == nil && id != "" {
				err = commandFormUI(root, id)
			} else {
				err = pickErr
			}
		case "Delete command":
			id, pickErr := pickCommand(root, "Delete which command?", true)
			if pickErr == nil && id != "" {
				confirmed, confirmErr := confirmOne(fmt.Sprintf("Delete %q?", id), false)
				if confirmErr != nil {
					err = confirmErr
				} else if confirmed {
					err = withWorkingDir(root, func() error { return deleteCommand([]string{id}) })
				}
			} else {
				err = pickErr
			}
		}
		if err != nil {
			showError(err)
		}
	}
}

func pickCommand(root, message string, includeImported bool) (string, error) {
	cfg, err := config.Read(root)
	if err != nil {
		return "", err
	}
	commands, err := config.Commands(cfg)
	if err != nil {
		return "", err
	}
	var options []string
	for _, command := range commands {
		if !includeImported && command.Imported {
			continue
		}
		options = append(options, fmt.Sprintf("%s — %s", config.CommandID(command), command.Command))
	}
	if len(options) == 0 {
		return "", errors.New("no matching commands")
	}
	options = append(options, "Back")
	answer, err := selectOne(message, options, "")
	if err != nil || answer == "Back" {
		return "", err
	}
	id, _, _ := strings.Cut(answer, " — ")
	return id, nil
}

func commandFormUI(root, existingID string) error {
	cfg, err := config.Read(root)
	if err != nil {
		return err
	}
	var existing config.Command
	if existingID != "" {
		commands, _ := config.Commands(cfg)
		var found bool
		existing, found = config.FindCommand(commands, existingID)
		if !found {
			return fmt.Errorf("command %q was not found", existingID)
		}
		if existing.Imported {
			return errors.New("imported commands are managed by their source files")
		}
	}
	var groups []config.Group
	for _, group := range cfg.Groups {
		if group.Type == "manual" {
			groups = append(groups, group)
		}
	}
	var groupOptions []string
	groupLookup := map[string]string{}
	for _, group := range groups {
		label := group.Label
		groupOptions = append(groupOptions, label)
		groupLookup[label] = group.ID
	}
	groupOptions = append(groupOptions, "Create new group")
	defaultGroup := groupLabel(cfg, existing.GroupID)
	groupChoice, err := selectOne("Group", groupOptions, defaultGroup)
	if err != nil {
		return err
	}
	groupID := groupLookup[groupChoice]
	var createdGroup *config.Group
	if groupChoice == "Create new group" {
		groupID, err = inputOne("Group id", "", true)
		if err != nil {
			return err
		}
		if !validGroupID(groupID) {
			return errors.New("group id may contain lowercase letters, numbers, dots, dashes, or underscores")
		}
		label, inputErr := inputOne("Group label", titleGroupID(groupID), true)
		if inputErr != nil {
			return inputErr
		}
		description, inputErr := inputOne("Group description", "", false)
		if inputErr != nil {
			return inputErr
		}
		group := config.Group{ID: groupID, Label: label, Description: description, Type: "manual", Commands: []config.Command{}}
		createdGroup = &group
	}
	name, err := inputOne("Name", existing.Name, true)
	if err != nil {
		return err
	}
	commandText, err := inputOne("Command", existing.Command, true)
	if err != nil {
		return err
	}
	description, err := inputOne("Description", existing.Description, false)
	if err != nil {
		return err
	}
	cwd := existing.CWD
	if cwd == "" {
		cwd = "."
	}
	cwd, err = inputOne("Working directory", cwd, true)
	if err != nil {
		return err
	}
	fields := commandFields{name: name, command: commandText, description: description, cwd: cwd, group: groupID}
	if err := withWorkingDir(root, func() error { return upsertCommand(existingID, fields) }); err != nil {
		return err
	}
	if createdGroup != nil {
		next, readErr := config.Read(root)
		if readErr != nil {
			return readErr
		}
		for index := range next.Groups {
			if next.Groups[index].ID == createdGroup.ID {
				next.Groups[index].Label = createdGroup.Label
				next.Groups[index].Description = createdGroup.Description
				next.Groups[index].Type = createdGroup.Type
			}
		}
		if writeErr := config.Write(root, next); writeErr != nil {
			return writeErr
		}
	}
	fmt.Printf("\n%sSaved%s %s%s%s\n", green, reset, bold, groupID+"."+name, reset)
	waitForEnter()
	return nil
}

func validGroupID(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || strings.ContainsRune("._-", char) {
			continue
		}
		return false
	}
	return true
}

func titleGroupID(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return strings.ContainsRune("._-", r) })
	for index := range parts {
		if parts[index] != "" {
			parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
		}
	}
	return strings.Join(parts, " ")
}

func importUI(root string) error {
	clearScreen()
	fmt.Printf("%sImport project commands%s\n%sSpace selects sources. Enter imports selected detected sources.%s\n\n", bold, reset, dim, reset)
	sources := importers.DetectSources(root)
	var options, defaults []string
	lookup := map[string]string{}
	for _, source := range sources {
		status := "not detected"
		if source.Detected {
			status = "detected: " + source.File
		}
		label := fmt.Sprintf("%s — %s", source.Title, status)
		fmt.Println(label)
		if source.Detected {
			options = append(options, label)
			defaults = append(defaults, label)
			lookup[label] = source.ID
		}
	}
	fmt.Println()
	if len(options) == 0 {
		waitForEnter()
		return nil
	}
	selected, err := multiSelect("Import from", options, defaults)
	if err != nil || len(selected) == 0 {
		return err
	}
	var ids []string
	for _, label := range selected {
		ids = append(ids, lookup[label])
	}
	cfg, err := config.Read(root)
	if err != nil {
		return err
	}
	next, err := importers.Import(root, cfg, ids)
	if err != nil {
		return err
	}
	commands, _ := config.Commands(next)
	fmt.Printf("\n%sImported%s. %d commands configured.\n", green, reset, len(commands))
	waitForEnter()
	return nil
}

func settingsUI(root string) error {
	for {
		cfg, err := config.Read(root)
		if err != nil {
			return err
		}
		settings, err := config.ReadSettings(root)
		if err != nil {
			return err
		}
		clearScreen()
		fmt.Printf("%sSettings%s\n%sLocal preferences for this project%s\n\n", bold, reset, dim, reset)
		printSettingsSummary(cfg, settings)
		fmt.Println()
		action, err := selectOne("Change", []string{
			"Default group — Choose which group opens first",
			"Hidden groups — Hide groups from the run menu",
			"Favorite groups — Pin groups near the top",
			"Favorite commands — Pin commands in Favorites",
			"Favorite command limit",
			"Validate config",
			"Install kmc skill",
			"Back",
		}, "")
		if err != nil || action == "Back" {
			return err
		}
		switch {
		case strings.HasPrefix(action, "Default group"):
			labels, lookup := groupOptions(cfg)
			selected, selectErr := selectOne("Default group", labels, groupLabel(cfg, settings.DefaultGroup))
			if selectErr != nil {
				return selectErr
			}
			settings.DefaultGroup = lookup[selected]
		case strings.HasPrefix(action, "Hidden groups"):
			labels, lookup := groupOptions(cfg)
			defaults := labelsForIDs(cfg, settings.HiddenGroups)
			selected, selectErr := multiSelect("Hidden groups", labels, defaults)
			if selectErr != nil {
				return selectErr
			}
			settings.HiddenGroups = idsForLabels(selected, lookup)
		case strings.HasPrefix(action, "Favorite groups"):
			labels, lookup := groupOptions(cfg)
			defaults := labelsForIDs(cfg, settings.FavoriteGroups)
			selected, selectErr := multiSelect("Favorite groups", labels, defaults)
			if selectErr != nil {
				return selectErr
			}
			settings.FavoriteGroups = idsForLabels(selected, lookup)
		case strings.HasPrefix(action, "Favorite commands"):
			commands, _ := config.Commands(cfg)
			var labels, defaults []string
			lookup := map[string]string{}
			current := stringSet(settings.FavoriteCommands)
			for _, command := range commands {
				label := fmt.Sprintf("%s — %s", config.CommandID(command), command.Command)
				labels = append(labels, label)
				lookup[label] = config.CommandID(command)
				if current[config.CommandID(command)] {
					defaults = append(defaults, label)
				}
			}
			selected, selectErr := multiSelect(fmt.Sprintf("Favorite commands (max %d)", settings.MaxFavoriteCommands), labels, defaults)
			if selectErr != nil {
				return selectErr
			}
			if len(selected) > settings.MaxFavoriteCommands {
				showError(fmt.Errorf("choose at most %d favorite commands", settings.MaxFavoriteCommands))
				continue
			}
			settings.FavoriteCommands = idsForLabels(selected, lookup)
		case action == "Favorite command limit":
			raw, inputErr := inputOne("Max favorite commands", strconv.Itoa(settings.MaxFavoriteCommands), true)
			if inputErr != nil {
				return inputErr
			}
			limit, parseErr := strconv.Atoi(raw)
			if parseErr != nil || limit < 1 {
				showError(errors.New("favorite command limit must be positive"))
				continue
			}
			settings.MaxFavoriteCommands = limit
			if len(settings.FavoriteCommands) > limit {
				settings.FavoriteCommands = settings.FavoriteCommands[:limit]
			}
		case action == "Validate config":
			clearScreen()
			if err := withWorkingDir(root, validate); err != nil {
				showError(err)
			} else {
				waitForEnter()
			}
			continue
		case action == "Install kmc skill":
			clearScreen()
			fmt.Printf("%sInstall kmc skill%s\n\n%s$ npx skills add marius4lui/kmc%s\n\n", bold, reset, cyan, reset)
			command := exec.Command("npx", "skills", "add", "marius4lui/kmc")
			command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := command.Run(); err != nil {
				showError(err)
			}
			waitForEnter()
			continue
		}
		if err := config.WriteSettings(root, settings); err != nil {
			return err
		}
	}
}

func printSettingsSummary(cfg config.Config, settings config.Settings) {
	fmt.Printf("%sDefault group%s      %s\n", dim, reset, groupLabel(cfg, settings.DefaultGroup))
	fmt.Printf("%sLast run group%s     %s\n", dim, reset, groupLabel(cfg, settings.LastSelectedGroup))
	fmt.Printf("%sHidden groups%s      %s\n", dim, reset, displayIDs(cfg, settings.HiddenGroups))
	fmt.Printf("%sFavorite groups%s    %s\n", dim, reset, displayIDs(cfg, settings.FavoriteGroups))
	fmt.Printf("%sFavorite commands%s  %s\n", dim, reset, displayValues(settings.FavoriteCommands))
	fmt.Printf("%sFavorite limit%s     %d\n", dim, reset, settings.MaxFavoriteCommands)
}

func displayValues(values []string) string {
	if len(values) == 0 {
		return dim + "None" + reset
	}
	return strings.Join(values, ", ")
}

func displayIDs(cfg config.Config, ids []string) string {
	var labels []string
	for _, id := range ids {
		labels = append(labels, groupLabel(cfg, id))
	}
	return displayValues(labels)
}

func groupOptions(cfg config.Config) ([]string, map[string]string) {
	var labels []string
	lookup := map[string]string{}
	for _, group := range cfg.Groups {
		labels = append(labels, group.Label)
		lookup[group.Label] = group.ID
	}
	return labels, lookup
}

func labelsForIDs(cfg config.Config, ids []string) []string {
	var labels []string
	for _, id := range ids {
		labels = append(labels, groupLabel(cfg, id))
	}
	return labels
}

func idsForLabels(labels []string, lookup map[string]string) []string {
	var ids []string
	for _, label := range labels {
		if id := lookup[label]; id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func scriptsUI(ctx context.Context, root string) error {
	for {
		clearScreen()
		fmt.Printf("%sKMC Scripts%s\n%sLocal YAML workflows from .kmc/scripts.yml%s\n\n", bold, reset, dim, reset)
		configured := fileExists(filepath.Join(root, ".kmc", "scripts.yml"))
		var loaded *workflows.Loaded
		var status trust.Status
		if configured {
			var err error
			loaded, err = workflows.LoadWithOptions(root, workflows.LoadOptions{ValidateCWDs: false})
			if err != nil {
				fmt.Printf("%sThe scripts configuration is invalid.%s\n%s%s%s\n\n", red, reset, dim, strings.Split(err.Error(), "\n")[0], reset)
			} else {
				status, _ = trust.StatusFor(loaded)
				trustLabel := dim + "not trusted" + reset
				if status.Trusted {
					trustLabel = green + "trusted" + reset
				} else if status.Changed {
					trustLabel = yellow + "changed" + reset
				}
				fmt.Printf("%sWorkflows%s  %d\n%sTrust%s      %s\n\n", dim, reset, len(loaded.Scripts), dim, reset, trustLabel)
			}
		} else {
			fmt.Printf("%sNo .kmc/scripts.yml found. Initialize KMC Scripts to get started.%s\n\n", dim, reset)
		}
		var options []string
		if loaded != nil && len(loaded.Scripts) > 0 {
			options = append(options, "Run script — Choose and execute a workflow")
		}
		if configured {
			options = append(options, "Validate — Check registry, workflows, paths, and shells")
		}
		options = append(options, "Initialize missing files")
		if loaded != nil && !status.Trusted {
			options = append(options, "Trust repository")
		}
		if status.Trusted {
			options = append(options, "Remove trust")
		}
		options = append(options, "Back")
		action, err := selectOne("Choose", options, "")
		if err != nil || action == "Back" {
			return err
		}
		switch {
		case strings.HasPrefix(action, "Run script"):
			if err := chooseAndRunScript(ctx, loaded); err != nil {
				showError(err)
			}
		case strings.HasPrefix(action, "Validate"):
			clearScreen()
			if _, err := workflows.Load(root); err != nil {
				showError(err)
			} else {
				fmt.Printf("%sScripts configuration is valid.%s\n", green, reset)
				waitForEnter()
			}
		case strings.HasPrefix(action, "Initialize"):
			clearScreen()
			if err := initScripts(root); err != nil {
				showError(err)
			} else {
				waitForEnter()
			}
		case action == "Trust repository":
			if err := trust.Trust(loaded); err != nil {
				showError(err)
			} else {
				fmt.Printf("\n%sTrusted%s %s\n", green, reset, loaded.ProjectRoot)
				waitForEnter()
			}
		case action == "Remove trust":
			_, err := trust.Untrust(root)
			if err != nil {
				showError(err)
			} else {
				fmt.Printf("\n%sTrust removed%s %s\n", yellow, reset, root)
				waitForEnter()
			}
		}
	}
}

func chooseAndRunScript(ctx context.Context, loaded *workflows.Loaded) error {
	var ids []string
	for id := range loaded.Scripts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var labels []string
	lookup := map[string]string{}
	for _, id := range ids {
		script := loaded.Scripts[id]
		description := script.Description
		if description == "" {
			description = script.Workflow.Name
		}
		label := id + " — " + description
		labels = append(labels, label)
		lookup[label] = id
	}
	labels = append(labels, "Back")
	answer, err := selectOne("Run KMC Script", labels, "")
	if err != nil || answer == "Back" {
		return err
	}
	status, err := trust.StatusFor(loaded)
	if err != nil {
		return err
	}
	if !status.Trusted {
		confirmed, confirmErr := confirmOne("This repository contains local commands. Trust the current workflow fingerprint and run?", false)
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			return nil
		}
		if err := trust.Trust(loaded); err != nil {
			return err
		}
	}
	clearScreen()
	return runWorkflow(ctx, loaded, loaded.Scripts[lookup[answer]], []string{"--yes"})
}

type devUIState struct {
	project *devurls.Project
	config  *devurls.Config
	status  string
}

func devURLsUI(ctx context.Context, root string) error {
	state, err := configureDevUI(root)
	if err != nil {
		return err
	}
	for {
		clearScreen()
		fmt.Printf("%sDev URLs%s\n%sStable local project URL through Caddy%s\n\n", bold, reset, dim, reset)
		printDevState(state)
		fmt.Println()
		var options []string
		if state.config != nil {
			options = append(options,
				"Start project",
				"Reload Caddy",
				"Trust local HTTPS certs",
				"Change name/host",
			)
		}
		options = append(options, "Select detected project", "Change project path", "Back")
		action, err := selectOne("Dev URLs", options, "")
		if err != nil || action == "Back" {
			return err
		}
		switch action {
		case "Start project":
			return startDevUI(ctx, state.config)
		case "Reload Caddy":
			caddyfile, upsertErr := devurls.UpsertCaddySite(*state.config, root)
			if upsertErr != nil {
				state.status = yellow + upsertErr.Error() + reset
			} else if reloadErr := devurls.ReloadCaddy(caddyfile); reloadErr != nil {
				state.status = yellow + reloadErr.Error() + reset
			} else {
				state.status = green + "reloaded" + reset
			}
		case "Trust local HTTPS certs":
			if trustErr := devurls.TrustCaddy(); trustErr != nil {
				state.status = yellow + trustErr.Error() + reset
			} else {
				state.status = green + "local CA trusted; restart the browser if it still warns" + reset
			}
		case "Change name/host":
			name, inputErr := inputOne("Name", state.config.Name, true)
			if inputErr != nil {
				return inputErr
			}
			if devurls.Slug(name) != name {
				state.status = yellow + "Use lowercase letters, numbers, and dashes." + reset
				continue
			}
			state.config.Name, state.config.Host = name, name+".kmc.localhost"
			if err := devurls.WriteConfig(root, *state.config); err != nil {
				return err
			}
			caddyfile, err := devurls.UpsertCaddySite(*state.config, root)
			if err == nil {
				err = devurls.ReloadCaddy(caddyfile)
			}
			if err != nil {
				state.status = yellow + err.Error() + reset
			} else {
				state.status = green + "reloaded" + reset
			}
		case "Select detected project":
			state, err = selectDevProjectUI(root)
			if err != nil {
				return err
			}
		case "Change project path":
			state, err = changeDevPathUI(root, state)
			if err != nil {
				return err
			}
		}
	}
}

func configureDevUI(root string) (devUIState, error) {
	existing, err := devurls.ReadConfig(root)
	if err != nil {
		return devUIState{}, err
	}
	if existing != nil {
		project, detectErr := devurls.DetectProject(existing.Root)
		return devUIState{project: project, config: existing, status: "not reloaded yet"}, detectErr
	}
	project, err := devurls.DetectProject(root)
	if err != nil {
		return devUIState{}, err
	}
	if project == nil {
		return selectDevProjectUI(root)
	}
	project.Root, project.Name = root, filepath.Base(root)
	return configureSelectedDevUI(root, *project)
}

func selectDevProjectUI(root string) (devUIState, error) {
	searchRoot, err := devurls.FindProjectSearchRoot(root)
	if err != nil {
		return devUIState{}, err
	}
	projects, err := devurls.DiscoverProjects(searchRoot)
	if err != nil || len(projects) == 0 {
		return devUIState{status: "not configured"}, err
	}
	selected := projects[0]
	if len(projects) > 1 {
		var labels []string
		lookup := map[string]devurls.Project{}
		for _, project := range projects {
			label := fmt.Sprintf("%s (%s) — %s", project.Name, project.Label, project.Root)
			labels = append(labels, label)
			lookup[label] = project
		}
		answer, selectErr := selectOne("Project", labels, "")
		if selectErr != nil {
			return devUIState{}, selectErr
		}
		selected = lookup[answer]
	}
	return configureSelectedDevUI(root, selected)
}

func configureSelectedDevUI(root string, project devurls.Project) (devUIState, error) {
	current, err := devurls.ReadConfig(root)
	if err != nil {
		return devUIState{}, err
	}
	if current == nil {
		current, err = devurls.EnsureConfig(root, project)
		if err != nil {
			return devUIState{}, err
		}
	} else {
		rootChanged := filepath.Clean(current.Root) != filepath.Clean(project.Root)
		current.Type, current.Root = project.Type, project.Root
		if rootChanged {
			current.Name = devurls.Slug(project.Name)
			current.Host = current.Name + ".kmc.localhost"
		}
		if err := devurls.WriteConfig(root, *current); err != nil {
			return devUIState{}, err
		}
	}
	caddyfile, err := devurls.UpsertCaddySite(*current, root)
	status := green + "reloaded" + reset
	if err == nil {
		err = devurls.ReloadCaddy(caddyfile)
	}
	if err != nil {
		status = yellow + err.Error() + reset
	}
	return devUIState{project: &project, config: current, status: status}, nil
}

func changeDevPathUI(root string, current devUIState) (devUIState, error) {
	defaultPath := root
	if current.config != nil && current.config.Root != "" {
		defaultPath = current.config.Root
	}
	path, err := inputOne("Project path", defaultPath, true)
	if err != nil {
		return current, err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return current, err
	}
	if !fileExists(filepath.Join(path, "package.json")) {
		return current, errors.New("path must contain package.json")
	}
	project, err := devurls.DetectProject(path)
	if err != nil || project == nil {
		if err == nil {
			err = errors.New("no supported project detected")
		}
		return current, err
	}
	project.Root, project.Name = path, filepath.Base(path)
	return configureSelectedDevUI(root, *project)
}

func printDevState(state devUIState) {
	projectLabel := yellow + "Not supported" + reset
	if state.project != nil {
		projectLabel = state.project.Label
	}
	fmt.Printf("%sDetected project%s %s\n", dim, reset, projectLabel)
	if state.config == nil {
		return
	}
	dir, _ := devurls.ConfigDir()
	fmt.Printf("%sName%s             %s\n", dim, reset, state.config.Name)
	fmt.Printf("%sType%s             %s\n", dim, reset, state.config.Type)
	fmt.Printf("%sPath%s             %s\n", dim, reset, state.config.Root)
	fmt.Printf("%sURL%s              %shttps://%s%s\n", dim, reset, cyan, state.config.Host, reset)
	fmt.Printf("%sPort%s             %d\n", dim, reset, state.config.Port)
	fmt.Printf("%sCaddyfile%s        %s\n", dim, reset, filepath.Join(dir, "Caddyfile"))
	fmt.Printf("%sCaddy%s            %s\n", dim, reset, state.status)
}

func startDevUI(ctx context.Context, cfg *devurls.Config) error {
	commandText, err := devurls.StartCommand(*cfg)
	if err != nil {
		return err
	}
	clearScreen()
	fmt.Printf("%s%s dev server%s\n", bold, cfg.Type, reset)
	fmt.Printf("%s────────────────────────────────────────────────%s\n", dim, reset)
	fmt.Printf("%sURL%s  %shttps://%s%s\n", dim, reset, cyan, cfg.Host, reset)
	fmt.Printf("%sPath%s %s\n", dim, reset, cfg.Root)
	fmt.Printf("%s$%s %s\n\n", cyan, reset, commandText)
	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.CommandContext(ctx, "cmd", "/d", "/s", "/c", commandText)
	} else {
		command = exec.CommandContext(ctx, "sh", "-c", commandText)
	}
	command.Dir = cfg.Root
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	return command.Run()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func withWorkingDir(root string, fn func() error) error {
	current, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(root); err != nil {
		return err
	}
	defer os.Chdir(current)
	return fn()
}
