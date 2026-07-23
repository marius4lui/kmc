package config

import "fmt"

type Config struct {
	Schema   string    `json:"$schema,omitempty"`
	Groups   []Group   `json:"groups,omitempty"`
	Commands []Command `json:"commands,omitempty"`
}

type Group struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	Type        string    `json:"type"`
	Source      string    `json:"source,omitempty"`
	Commands    []Command `json:"commands"`
}

type Command struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	CWD         string `json:"cwd,omitempty"`
	GroupID     string `json:"groupId,omitempty"`
	Group       string `json:"group,omitempty"`
	Source      string `json:"source,omitempty"`
	Imported    bool   `json:"imported,omitempty"`
}

type Settings struct {
	DefaultGroup        string   `json:"defaultGroup"`
	LastSelectedGroup   string   `json:"lastSelectedGroup"`
	HiddenGroups        []string `json:"hiddenGroups"`
	FavoriteGroups      []string `json:"favoriteGroups"`
	FavoriteCommands    []string `json:"favoriteCommands"`
	MaxFavoriteCommands int      `json:"maxFavoriteCommands"`
}

func DefaultSettings() Settings {
	return Settings{
		DefaultGroup:        "development",
		LastSelectedGroup:   "development",
		HiddenGroups:        []string{},
		FavoriteGroups:      []string{},
		FavoriteCommands:    []string{},
		MaxFavoriteCommands: 3,
	}
}

func CommandID(command Command) string {
	if command.ID != "" {
		return command.ID
	}
	groupID := command.GroupID
	if groupID == "" {
		groupID = command.Group
	}
	if groupID == "" {
		groupID = "manual"
	}
	return groupID + "." + command.Name
}

func NormalizeCommand(command Command, groupID string) (Command, error) {
	if groupID == "" {
		groupID = command.GroupID
	}
	if groupID == "" {
		groupID = command.Group
	}
	if groupID == "" {
		groupID = "manual"
	}
	if command.Name == "" || command.Command == "" {
		return Command{}, fmt.Errorf(`each command in %s needs "name" and "command" strings`, ConfigFile)
	}
	command.GroupID = groupID
	command.Group = groupID
	command.ID = CommandID(command)
	if command.CWD == "" {
		command.CWD = "."
	}
	if command.Source == "" {
		command.Source = KnownGroups[groupID].Source
		if command.Source == "" {
			command.Source = "manual"
		}
	}
	return command, nil
}

func GroupShell(groupID string, imported bool) Group {
	defaults := KnownGroups[groupID]
	groupType := defaults.Type
	if groupType == "" {
		if imported {
			groupType = "imported"
		} else {
			groupType = "manual"
		}
	}
	label := defaults.Label
	if label == "" {
		label = groupID
	}
	return Group{
		ID:          groupID,
		Label:       label,
		Description: defaults.Description,
		Icon:        defaults.Icon,
		Type:        groupType,
		Source:      defaults.Source,
		Commands:    []Command{},
	}
}

func NormalizeGroup(group Group) (Group, error) {
	if group.ID == "" {
		return Group{}, fmt.Errorf(`each group in %s needs an "id" string`, ConfigFile)
	}
	defaults := GroupShell(group.ID, false)
	if group.Label == "" {
		group.Label = defaults.Label
	}
	if group.Description == "" {
		group.Description = defaults.Description
	}
	if group.Icon == "" {
		group.Icon = defaults.Icon
	}
	if group.Type == "" {
		group.Type = defaults.Type
	}
	if group.Source == "" {
		group.Source = defaults.Source
	}
	if group.Commands == nil {
		group.Commands = []Command{}
	}
	for i := range group.Commands {
		normalized, err := NormalizeCommand(group.Commands[i], group.ID)
		if err != nil {
			return Group{}, err
		}
		group.Commands[i] = normalized
	}
	return group, nil
}

func Normalize(input Config) (Config, error) {
	result := Config{Schema: input.Schema}
	if result.Schema == "" {
		result.Schema = SchemaURL
	}
	if input.Groups != nil {
		result.Groups = make([]Group, len(input.Groups))
		for i, group := range input.Groups {
			normalized, err := NormalizeGroup(group)
			if err != nil {
				return Config{}, err
			}
			result.Groups[i] = normalized
		}
		return result, nil
	}
	if input.Commands != nil {
		groupIndexes := map[string]int{}
		result.Groups = []Group{}
		for _, item := range input.Commands {
			command, err := NormalizeCommand(item, "")
			if err != nil {
				return Config{}, err
			}
			index, found := groupIndexes[command.GroupID]
			if !found {
				index = len(result.Groups)
				groupIndexes[command.GroupID] = index
				result.Groups = append(result.Groups, GroupShell(command.GroupID, command.Imported))
			}
			result.Groups[index].Commands = append(result.Groups[index].Commands, command)
		}
		for i := range result.Groups {
			normalized, err := NormalizeGroup(result.Groups[i])
			if err != nil {
				return Config{}, err
			}
			result.Groups[i] = normalized
		}
		return result, nil
	}
	return Config{}, fmt.Errorf(`%s must contain a "groups" array`, ConfigFile)
}

func Commands(input Config) ([]Command, error) {
	normalized, err := Normalize(input)
	if err != nil {
		return nil, err
	}
	var commands []Command
	for _, group := range normalized.Groups {
		commands = append(commands, group.Commands...)
	}
	return commands, nil
}

func FindCommand(commands []Command, identifier string) (Command, bool) {
	for _, command := range commands {
		if CommandID(command) == identifier || command.Name == identifier || command.Group+"."+command.Name == identifier {
			return command, true
		}
	}
	return Command{}, false
}
