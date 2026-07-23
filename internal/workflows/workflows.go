package workflows

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string { return strings.Join(e.Errors, "\n") }

type Registry struct {
	Version int
	Scripts map[string]Reference
}

type Reference struct {
	File        string
	Description string
}

type Workflow struct {
	Name        string
	Description string
	Env         map[string]any
	Defaults    Defaults
	Steps       []Step
}

type Defaults struct {
	Shell string
	CWD   string
}

type Step struct {
	Name            string
	Run             string
	Shell           string
	CWD             string
	Env             map[string]any
	Timeout         int
	Retries         int
	ContinueOnError bool
	Index           int
}

type Script struct {
	ID          string
	Description string
	File        string
	Workflow    Workflow
}

type Loaded struct {
	ProjectRoot  string
	RegistryFile string
	Registry     Registry
	Scripts      map[string]*Script
}

type LoadOptions struct {
	ValidateCWDs bool
}

func DefaultLoadOptions() LoadOptions { return LoadOptions{ValidateCWDs: true} }

func Load(projectRoot string) (*Loaded, error) {
	return LoadWithOptions(projectRoot, DefaultLoadOptions())
}

func LoadWithOptions(projectRoot string, options LoadOptions) (*Loaded, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	registryFile := filepath.Join(root, ".kmc", "scripts.yml")
	registryValue, err := readYAML(registryFile)
	if err != nil {
		return nil, err
	}
	registry, validation := validateRegistry(registryValue, registryFile)
	loaded := &Loaded{
		ProjectRoot: root, RegistryFile: registryFile, Registry: registry,
		Scripts: make(map[string]*Script),
	}
	if len(validation) != 0 {
		return nil, &ValidationError{Errors: validation}
	}

	ids := make([]string, 0, len(registry.Scripts))
	for id := range registry.Scripts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		reference := registry.Scripts[id]
		file := resolvePath(filepath.Dir(registryFile), reference.File)
		if !inside(root, file) {
			validation = append(validation, fmt.Sprintf(`%s: script %q points outside the project root.`, registryFile, id))
			continue
		}
		if resolved, resolveErr := filepath.EvalSymlinks(file); resolveErr == nil && !insideResolved(root, resolved) {
			validation = append(validation, fmt.Sprintf(`%s: script %q points outside the project root.`, registryFile, id))
			continue
		}
		value, readErr := readYAML(file)
		if readErr != nil {
			var validationErr *ValidationError
			if errors.As(readErr, &validationErr) {
				validation = append(validation, validationErr.Errors...)
				continue
			}
			return nil, readErr
		}
		workflow, workflowErrors := validateWorkflow(value, file, root)
		validation = append(validation, workflowErrors...)
		description := reference.Description
		if description == "" {
			description = workflow.Description
		}
		loaded.Scripts[id] = &Script{ID: id, Description: description, File: file, Workflow: workflow}
	}
	if options.ValidateCWDs {
		for _, id := range ids {
			script := loaded.Scripts[id]
			if script == nil {
				continue
			}
			for i, step := range script.Workflow.Steps {
				cwd := step.CWD
				if cwd == "" {
					cwd = script.Workflow.Defaults.CWD
				}
				if cwd == "" {
					cwd = "."
				}
				resolved := resolvePath(root, cwd)
				info, statErr := os.Stat(resolved)
				switch {
				case statErr != nil:
					validation = append(validation, fmt.Sprintf("%s: step %d cwd does not exist: %s", script.File, i+1, resolved))
				case !info.IsDir():
					validation = append(validation, fmt.Sprintf("%s: step %d cwd is not a directory: %s", script.File, i+1, resolved))
				case !insideResolved(root, resolved):
					validation = append(validation, fmt.Sprintf("%s: step %d cwd escapes the project root.", script.File, i+1))
				}
			}
		}
	}
	if len(validation) != 0 {
		return nil, &ValidationError{Errors: validation}
	}
	return loaded, nil
}

func readYAML(file string) (map[string]any, error) {
	source, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, &ValidationError{Errors: []string{fmt.Sprintf("%s: file was not found.", file)}}
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(source)) == "" {
		return nil, &ValidationError{Errors: []string{fmt.Sprintf("%s: file is empty.", file)}}
	}
	var value any
	if err := yaml.Unmarshal(source, &value); err != nil {
		return nil, &ValidationError{Errors: []string{fmt.Sprintf("%s: %v", file, err)}}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, &ValidationError{Errors: []string{fmt.Sprintf("%s: document must be an object.", file)}}
	}
	return object, nil
}

func validateRegistry(value map[string]any, file string) (Registry, []string) {
	registry := Registry{Scripts: make(map[string]Reference)}
	var result []string
	unknownFields(value, set("version", "scripts"), file, &result)
	version, ok := integer(value["version"])
	if !ok || version != 1 {
		result = append(result, fmt.Sprintf(`%s: "version" must be 1.`, file))
	} else {
		registry.Version = version
	}
	scripts, ok := value["scripts"].(map[string]any)
	if !ok {
		result = append(result, fmt.Sprintf(`%s: "scripts" must be an object.`, file))
		return registry, result
	}
	for name, raw := range scripts {
		label := fmt.Sprintf(`%s: script %q`, file, name)
		if strings.TrimSpace(name) == "" {
			result = append(result, fmt.Sprintf("%s: script names must not be empty.", file))
		}
		object, ok := raw.(map[string]any)
		if !ok {
			result = append(result, label+" must be an object.")
			continue
		}
		unknownFields(object, set("file", "description"), label, &result)
		reference := Reference{}
		reference.File, ok = object["file"].(string)
		if !ok || strings.TrimSpace(reference.File) == "" {
			result = append(result, label+` needs a non-empty "file" string.`)
		}
		if rawDescription, exists := object["description"]; exists {
			reference.Description, ok = rawDescription.(string)
			if !ok {
				result = append(result, label+`: "description" must be a string.`)
			}
		}
		registry.Scripts[name] = reference
	}
	return registry, result
}

func validateWorkflow(value map[string]any, file, projectRoot string) (Workflow, []string) {
	workflow := Workflow{}
	var result []string
	unknownFields(value, set("name", "description", "env", "defaults", "steps"), file, &result)
	stringField(value, "name", file, &workflow.Name, &result)
	stringField(value, "description", file, &workflow.Description, &result)
	workflow.Env = validateEnv(value["env"], file+": env", &result)

	if raw, exists := value["defaults"]; exists {
		defaults, ok := raw.(map[string]any)
		if !ok {
			result = append(result, fmt.Sprintf(`%s: "defaults" must be an object.`, file))
		} else {
			unknownFields(defaults, set("shell", "cwd"), file+": defaults", &result)
			workflow.Defaults.Shell = optionalNonEmptyString(defaults, "shell", file+": defaults.shell", &result)
			workflow.Defaults.CWD = optionalString(defaults, "cwd", file+": defaults.cwd", &result)
		}
	}
	rawSteps, ok := value["steps"].([]any)
	if !ok || len(rawSteps) == 0 {
		result = append(result, fmt.Sprintf(`%s: "steps" must be a non-empty array.`, file))
		return workflow, result
	}
	names := make(map[string]int)
	for index, raw := range rawSteps {
		label := fmt.Sprintf("%s: step %d", file, index+1)
		object, ok := raw.(map[string]any)
		if !ok {
			result = append(result, label+" must be an object.")
			continue
		}
		unknownFields(object, set("name", "run", "shell", "cwd", "env", "timeout", "retries", "continue_on_error"), label, &result)
		step := Step{Index: index}
		step.Name = requiredNonEmptyString(object, "name", label, &result)
		step.Run = requiredNonEmptyString(object, "run", label, &result)
		step.Shell = optionalNonEmptyString(object, "shell", label+`: "shell"`, &result)
		step.CWD = optionalString(object, "cwd", label+`: "cwd"`, &result)
		step.Env = validateEnv(object["env"], label+": env", &result)
		step.Timeout = optionalInteger(object, "timeout", true, label+`: "timeout" must be a positive integer.`, &result)
		step.Retries = optionalInteger(object, "retries", false, label+`: "retries" must be a non-negative integer.`, &result)
		if rawContinue, exists := object["continue_on_error"]; exists {
			var valid bool
			step.ContinueOnError, valid = rawContinue.(bool)
			if !valid {
				result = append(result, label+`: "continue_on_error" must be a boolean.`)
			}
		}
		if step.Name != "" {
			names[step.Name]++
		}
		workflow.Steps = append(workflow.Steps, step)
	}
	for name, count := range names {
		if count > 1 {
			result = append(result, fmt.Sprintf(`%s: step name %q is duplicated.`, file, name))
		}
	}
	checkCWD := func(label, cwd string) {
		if cwd != "" && !inside(projectRoot, resolvePath(projectRoot, cwd)) {
			result = append(result, fmt.Sprintf("%s: %s escapes the project root.", file, label))
		}
	}
	checkCWD("defaults.cwd", workflow.Defaults.CWD)
	for i, step := range workflow.Steps {
		checkCWD(fmt.Sprintf("step %d.cwd", i+1), step.CWD)
	}
	return workflow, result
}

func SelectSteps(workflow Workflow, selector string) ([]Step, error) {
	if selector == "" {
		return append([]Step(nil), workflow.Steps...), nil
	}
	numeric, numericErr := strconv.Atoi(selector)
	var matches []Step
	for index, step := range workflow.Steps {
		if (numericErr == nil && index+1 == numeric) || step.Name == selector || Slug(step.Name) == selector {
			step.Index = index
			matches = append(matches, step)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf(`step %q was not found`, selector)
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf(`step selector %q is ambiguous`, selector)
	}
	return matches, nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func Slug(value string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-"), "-")
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

func insideResolved(root, target string) bool {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		resolvedTarget = target
	}
	return inside(resolvedRoot, resolvedTarget)
}

func set(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func unknownFields(value map[string]any, allowed map[string]struct{}, label string, errors *[]string) {
	for field := range value {
		if _, ok := allowed[field]; !ok {
			*errors = append(*errors, fmt.Sprintf(`%s: field %q is not supported in scripts schema version 1.`, label, field))
		}
	}
}

func integer(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case int64:
		return int(number), true
	case uint64:
		return int(number), true
	default:
		return 0, false
	}
}

func optionalInteger(object map[string]any, field string, positive bool, message string, errors *[]string) int {
	raw, exists := object[field]
	if !exists {
		return 0
	}
	value, ok := integer(raw)
	if !ok || (positive && value <= 0) || (!positive && value < 0) {
		*errors = append(*errors, message)
		return 0
	}
	return value
}

func validateEnv(raw any, label string, errors *[]string) map[string]any {
	if raw == nil {
		return nil
	}
	value, ok := raw.(map[string]any)
	if !ok {
		*errors = append(*errors, label+" must be an object.")
		return nil
	}
	for key, item := range value {
		switch item.(type) {
		case nil, string, int, int64, uint64, float64, bool:
		default:
			*errors = append(*errors, fmt.Sprintf("%s.%s must be a scalar value.", label, key))
		}
	}
	return value
}

func stringField(object map[string]any, field, label string, target *string, errors *[]string) {
	raw, exists := object[field]
	if !exists {
		return
	}
	value, ok := raw.(string)
	if !ok {
		*errors = append(*errors, fmt.Sprintf(`%s: %q must be a string.`, label, field))
		return
	}
	*target = value
}

func requiredNonEmptyString(object map[string]any, field, label string, errors *[]string) string {
	value, ok := object[field].(string)
	if !ok || strings.TrimSpace(value) == "" {
		*errors = append(*errors, fmt.Sprintf(`%s needs a non-empty %q string.`, label, field))
		return ""
	}
	return value
}

func optionalNonEmptyString(object map[string]any, field, label string, errors *[]string) string {
	raw, exists := object[field]
	if !exists {
		return ""
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		*errors = append(*errors, label+" must be a non-empty string.")
		return ""
	}
	return value
}

func optionalString(object map[string]any, field, label string, errors *[]string) string {
	raw, exists := object[field]
	if !exists {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		*errors = append(*errors, label+" must be a string.")
		return ""
	}
	return value
}
