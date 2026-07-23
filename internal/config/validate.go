package config

type Check struct {
	OK     bool
	Label  string
	Detail string
}

type ValidationResult struct {
	OK           bool
	Checks       []Check
	CommandCount int
}

func Validate(input Config, settings Settings) ValidationResult {
	checks := []Check{}
	commands, commandErr := Commands(input)
	groupIDs := make([]string, 0, len(input.Groups))
	for _, group := range input.Groups {
		groupIDs = append(groupIDs, group.ID)
	}
	duplicateGroups := duplicates(groupIDs)
	ids := make([]string, 0, len(commands))
	for _, command := range commands {
		ids = append(ids, CommandID(command))
	}
	duplicateCommands := duplicates(ids)
	known := map[string]bool{}
	for id := range KnownGroups {
		known[id] = true
	}
	for _, id := range groupIDs {
		known[id] = true
	}
	checks = append(checks,
		Check{OK: input.Groups != nil, Label: ConfigFile + " contains a groups array"},
		Check{OK: len(duplicateGroups) == 0, Label: "group ids are unique", Detail: join(duplicateGroups)},
		Check{OK: everyGroupComplete(input.Groups), Label: "each group has id, label, and type"},
		Check{OK: commandErr == nil && len(duplicateCommands) == 0, Label: "command ids are unique", Detail: join(duplicateCommands)},
		Check{OK: commandErr == nil && everyCommandComplete(commands), Label: "each command has name and command"},
		Check{OK: commandErr == nil && everyCommandGroupKnown(commands, known), Label: "all command groups are known"},
		Check{OK: everyKnown(settings.FavoriteGroups, known), Label: "favoriteGroups point to known groups"},
		Check{OK: everyKnown(settings.HiddenGroups, known), Label: "hiddenGroups point to known groups"},
		Check{OK: settings.MaxFavoriteCommands >= 1, Label: "maxFavoriteCommands is a positive integer"},
		Check{OK: len(settings.FavoriteCommands) <= settings.MaxFavoriteCommands, Label: "favoriteCommands respects maxFavoriteCommands"},
		Check{OK: true, Label: SettingsFile + " is optional and local"},
	)
	result := ValidationResult{OK: true, Checks: checks, CommandCount: len(commands)}
	for _, check := range checks {
		result.OK = result.OK && check.OK
	}
	return result
}

func duplicates(values []string) []string {
	seen, added := map[string]bool{}, map[string]bool{}
	var result []string
	for _, value := range values {
		if seen[value] && !added[value] {
			result = append(result, value)
			added[value] = true
		}
		seen[value] = true
	}
	return result
}

func join(values []string) string {
	var result string
	for i, value := range values {
		if i > 0 {
			result += ", "
		}
		result += value
	}
	return result
}

func everyGroupComplete(groups []Group) bool {
	for _, group := range groups {
		if group.ID == "" || group.Label == "" || group.Type == "" {
			return false
		}
	}
	return true
}

func everyCommandComplete(commands []Command) bool {
	for _, command := range commands {
		if command.Name == "" || command.Command == "" {
			return false
		}
	}
	return true
}

func everyCommandGroupKnown(commands []Command, known map[string]bool) bool {
	for _, command := range commands {
		if !known[command.GroupID] {
			return false
		}
	}
	return true
}

func everyKnown(values []string, known map[string]bool) bool {
	for _, value := range values {
		if !known[value] {
			return false
		}
	}
	return true
}
