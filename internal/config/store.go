package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func Read(root string) (Config, error) {
	path := filepath.Join(root, ConfigFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Normalize(Config{Schema: SchemaURL, Groups: []Group{}})
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	var value Config
	if err := json.Unmarshal(data, &value); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return Normalize(value)
}

func Serialize(input Config) (Config, error) {
	normalized, err := Normalize(input)
	if err != nil {
		return Config{}, err
	}
	result := Config{Schema: normalized.Schema, Groups: []Group{}}
	for _, group := range normalized.Groups {
		if len(group.Commands) == 0 {
			continue
		}
		serialized := group
		serialized.Commands = make([]Command, len(group.Commands))
		for i, command := range group.Commands {
			command.GroupID = ""
			command.Group = ""
			serialized.Commands[i] = command
		}
		result.Groups = append(result.Groups, serialized)
	}
	return result, nil
}

func Write(root string, input Config) error {
	value, err := Serialize(input)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", ConfigFile, err)
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(root, ConfigFile), data, 0o644)
}

func ReplaceGroups(input Config, groups []Group) (Config, error) {
	input.Groups = groups
	input.Commands = nil
	return Serialize(input)
}

func ReadSettings(root string) (Settings, error) {
	settings := DefaultSettings()
	path := filepath.Join(root, SettingsFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("read %s: %w", path, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Settings{}, fmt.Errorf("parse %s: %w", path, err)
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		return Settings{}, err
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &merged); err != nil {
		return Settings{}, err
	}
	for key, value := range raw {
		merged[key] = value
	}
	encoded, err = json.Marshal(merged)
	if err != nil {
		return Settings{}, err
	}
	if err := json.Unmarshal(encoded, &settings); err != nil {
		return Settings{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return settings, nil
}

func WriteSettings(root string, settings Settings) error {
	dir := filepath.Join(root, SettingsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, SettingsFile), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return EnsureSettingsIgnored(root)
}

func EnsureSettingsIgnored(root string) error {
	path := filepath.Join(root, ".gitignore")
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	entry := []byte(SettingsDir + "/")
	for _, line := range bytes.Split(current, []byte{'\n'}) {
		if bytes.Equal(bytes.TrimSuffix(line, []byte{'\r'}), entry) {
			return nil
		}
	}
	if len(current) > 0 && current[len(current)-1] != '\n' {
		current = append(current, '\n')
	}
	current = append(current, entry...)
	current = append(current, '\n')
	return os.WriteFile(path, current, 0o644)
}
