package importers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/marius4lui/kmc/internal/config"
)

type Source struct {
	ID       string
	Title    string
	Files    []string
	Detected bool
	FilePath string
	File     string
}

var Sources = []Source{
	{ID: "npm", Title: "NPM Scripts", Files: []string{"package.json"}},
	{ID: "make", Title: "Make Commands", Files: []string{"Makefile", "makefile"}},
	{ID: "flutter", Title: "Flutter Commands", Files: []string{"pubspec.yaml"}},
	{ID: "docker", Title: "Docker Commands", Files: []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}},
}

func DetectSources(root string) []Source {
	result := make([]Source, len(Sources))
	for i, source := range Sources {
		result[i] = source
		for _, name := range source.Files {
			path := filepath.Join(root, name)
			if _, err := os.Stat(path); err == nil {
				result[i].Detected = true
				result[i].FilePath = path
				result[i].File = filepath.Base(path)
				break
			}
		}
	}
	return result
}

func Detect(root string, sourceIDs []string) ([]config.Command, error) {
	selected := selectedSources(sourceIDs)
	var imports []config.Command
	for _, source := range DetectSources(root) {
		if !source.Detected || !selected[source.ID] {
			continue
		}
		commands, err := importSource(source)
		if err != nil {
			return nil, err
		}
		for _, command := range commands {
			command.CWD = "."
			command.GroupID = source.ID
			command.Group = source.ID
			command.Source = source.File
			command.Imported = true
			command.ID = config.CommandID(command)
			imports = append(imports, command)
		}
	}
	return imports, nil
}

func Import(root string, current config.Config, sourceIDs []string) (config.Config, error) {
	if sourceIDs == nil {
		sourceIDs = allSourceIDs()
	}
	imports, err := Detect(root, sourceIDs)
	if err != nil {
		return config.Config{}, err
	}
	selected := selectedSources(sourceIDs)
	normalized, err := config.Normalize(current)
	if err != nil {
		return config.Config{}, err
	}
	groups := make([]config.Group, 0, len(normalized.Groups)+len(sourceIDs))
	for _, group := range normalized.Groups {
		if !selected[group.ID] {
			groups = append(groups, group)
		}
	}
	indexes := map[string]int{}
	for _, command := range imports {
		index, found := indexes[command.GroupID]
		if !found {
			index = len(groups)
			indexes[command.GroupID] = index
			groups = append(groups, config.GroupShell(command.GroupID, true))
		}
		groups[index].Commands = append(groups[index].Commands, command)
	}
	next, err := config.ReplaceGroups(config.Config{Schema: config.SchemaURL, Groups: groups}, groups)
	if err != nil {
		return config.Config{}, err
	}
	if err := config.Write(root, next); err != nil {
		return config.Config{}, err
	}
	return next, nil
}

func selectedSources(ids []string) map[string]bool {
	if ids == nil {
		ids = allSourceIDs()
	}
	selected := make(map[string]bool, len(ids))
	for _, id := range ids {
		selected[id] = true
	}
	return selected
}

func allSourceIDs() []string {
	ids := make([]string, len(Sources))
	for i, source := range Sources {
		ids[i] = source.ID
	}
	return ids
}

func importSource(source Source) ([]config.Command, error) {
	data, err := os.ReadFile(source.FilePath)
	if err != nil {
		return nil, err
	}
	switch source.ID {
	case "npm":
		return parsePackageJSON(data)
	case "make":
		return ParseMakefile(string(data)), nil
	case "flutter":
		return flutterCommands(), nil
	case "docker":
		return ParseDockerCompose(string(data)), nil
	default:
		return []config.Command{}, nil
	}
}

var makeTargetPattern = regexp.MustCompile(`^([A-Za-z0-9_.-]+):(?:\s|$)`)

func ParseMakefile(content string) []config.Command {
	var commands []config.Command
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		match := makeTargetPattern.FindStringSubmatch(line)
		if len(match) == 0 || strings.HasPrefix(match[1], ".") || seen[match[1]] {
			continue
		}
		name := match[1]
		seen[name] = true
		commands = append(commands, config.Command{Name: name, Command: "make " + name, Description: "Make target " + name})
	}
	return commands
}

var dockerServicePattern = regexp.MustCompile(`^  ([A-Za-z0-9_.-]+):\s*$`)

func ParseDockerCompose(content string) []config.Command {
	commands := []config.Command{
		{Name: "up", Command: "docker compose up", Description: "Start Docker Compose services"},
		{Name: "up-detached", Command: "docker compose up -d", Description: "Start Docker Compose in the background"},
		{Name: "down", Command: "docker compose down", Description: "Stop Docker Compose services"},
		{Name: "logs", Command: "docker compose logs -f", Description: "Follow Docker Compose logs"},
		{Name: "ps", Command: "docker compose ps", Description: "List Docker Compose services"},
	}
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		match := dockerServicePattern.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		service := match[1]
		commands = append(commands, config.Command{
			Name:        "restart-" + service,
			Command:     "docker compose restart " + service,
			Description: "Restart " + service,
		})
	}
	return commands
}

func flutterCommands() []config.Command {
	return []config.Command{
		{Name: "pub-get", Command: "flutter pub get", Description: "Install Flutter dependencies"},
		{Name: "analyze", Command: "flutter analyze", Description: "Run Flutter analyzer"},
		{Name: "test", Command: "flutter test", Description: "Run Flutter tests"},
		{Name: "run", Command: "flutter run", Description: "Run the Flutter app"},
		{Name: "build-apk", Command: "flutter build apk", Description: "Build Android APK"},
	}
}

func parsePackageJSON(data []byte) ([]config.Command, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, errors.New("package.json must contain an object")
	}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key := keyToken.(string)
		if key != "scripts" {
			var skipped any
			if err := decoder.Decode(&skipped); err != nil {
				return nil, err
			}
			continue
		}
		return decodeScripts(decoder)
	}
	return []config.Command{}, nil
}

func decodeScripts(decoder *json.Decoder) ([]config.Command, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token == nil {
		return []config.Command{}, nil
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, errors.New(`package.json "scripts" must contain an object`)
	}
	var commands []config.Command
	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		var script string
		if err := decoder.Decode(&script); err != nil {
			return nil, fmt.Errorf("decode package script %q: %w", nameToken, err)
		}
		name := nameToken.(string)
		commands = append(commands, config.Command{Name: name, Command: "npm run " + name, Description: script})
	}
	if _, err := decoder.Token(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return commands, nil
}
