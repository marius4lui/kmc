package devurls

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	DefaultPortStart = 43100
	DefaultPortEnd   = 43999
)

var ignoredDirs = map[string]bool{
	".git": true, ".next": true, ".nuxt": true, ".output": true,
	"build": true, "coverage": true, "dist": true, "node_modules": true, "out": true,
}

type Project struct {
	Type         string `json:"type"`
	Label        string `json:"label"`
	StartCommand string `json:"startCommand"`
	Root         string `json:"root,omitempty"`
	Name         string `json:"name,omitempty"`
}

type Config struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Root string `json:"root"`
	Port int    `json:"port"`
	Host string `json:"host"`
}

type Site struct {
	CWD  string `json:"cwd"`
	Name string `json:"name"`
	Type string `json:"type"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type packageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      json.RawMessage   `json:"workspaces"`
}

func readPackage(root string) (*packageJSON, error) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}
	return &pkg, nil
}

func (p *packageJSON) dependency(name string) bool {
	return p.Dependencies[name] != "" || p.DevDependencies[name] != ""
}

func DetectProject(root string) (*Project, error) {
	pkg, err := readPackage(root)
	if err != nil || pkg == nil {
		return nil, err
	}
	switch {
	case pkg.dependency("next"):
		return &Project{Type: "nextjs", Label: "Next.js", StartCommand: "npx next dev --port {port}"}, nil
	case pkg.dependency("vite"):
		return &Project{Type: "vite", Label: "Vite", StartCommand: "npx vite --host 127.0.0.1 --port {port}"}, nil
	case pkg.dependency("@nestjs/core"):
		script := firstScript(pkg.Scripts, "start:dev", "dev", "start")
		if script == "" {
			return &Project{Type: "nestjs", Label: "NestJS", StartCommand: "PORT={port} npx nest start --watch"}, nil
		}
		return &Project{Type: "nestjs", Label: "NestJS", StartCommand: "PORT={port} npm run " + script}, nil
	case pkg.dependency("express"):
		if pkg.Scripts["dev"] != "" {
			return &Project{Type: "express", Label: "Express", StartCommand: "PORT={port} npm run dev"}, nil
		}
		if pkg.Scripts["start"] != "" {
			return &Project{Type: "express", Label: "Express", StartCommand: "PORT={port} npm start"}, nil
		}
	}
	return nil, nil
}

func firstScript(scripts map[string]string, names ...string) string {
	for _, name := range names {
		if scripts[name] != "" {
			return name
		}
	}
	return ""
}

func workspacePatterns(pkg *packageJSON) []string {
	if pkg == nil || len(pkg.Workspaces) == 0 {
		return nil
	}
	var direct []string
	if json.Unmarshal(pkg.Workspaces, &direct) == nil {
		return direct
	}
	var nested struct {
		Packages []string `json:"packages"`
	}
	_ = json.Unmarshal(pkg.Workspaces, &nested)
	return nested.Packages
}

func hasWorkspaceMarker(root string, pkg *packageJSON) bool {
	if len(workspacePatterns(pkg)) > 0 {
		return true
	}
	for _, name := range []string{"pnpm-workspace.yaml", "pnpm-workspace.yml", "lerna.json", "turbo.json", "nx.json"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
	}
	return false
}

func FindProjectSearchRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	fallback := current
	for {
		pkg, err := readPackage(current)
		if err != nil {
			return "", err
		}
		if pkg != nil {
			fallback = current
		}
		if hasWorkspaceMarker(current, pkg) {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return fallback, nil
		}
		current = parent
	}
}

func DiscoverProjects(root string) ([]Project, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var projects []Project
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && ignoredDirs[entry.Name()] {
				return filepath.SkipDir
			}
			relative, _ := filepath.Rel(root, path)
			if relative != "." && len(strings.Split(relative, string(filepath.Separator))) > 4 {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != "package.json" {
			return nil
		}
		dir := filepath.Dir(path)
		project, detectErr := DetectProject(dir)
		if detectErr != nil {
			return detectErr
		}
		if project != nil {
			name, _ := filepath.Rel(root, dir)
			if name == "." {
				name = filepath.Base(dir)
			}
			project.Root, project.Name = dir, filepath.ToSlash(name)
			projects = append(projects, *project)
		}
		return nil
	})
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	return projects, err
}

func Slug(value string) string {
	var out strings.Builder
	dash := false
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			out.WriteRune(r)
			dash = false
		} else if out.Len() > 0 && !dash {
			out.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func FindFreePort(start, end int) (int, error) {
	for port := start; port <= end; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		_ = listener.Close()
		return port, nil
	}
	return 0, fmt.Errorf("no free port found in %d-%d", start, end)
}

func ReadConfig(projectRoot string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, ".kmc", "dev.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func WriteConfig(projectRoot string, config Config) error {
	dir := filepath.Join(projectRoot, ".kmc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "dev.json"), append(data, '\n'), 0o644)
}

func EnsureConfig(projectRoot string, project Project) (*Config, error) {
	existing, err := ReadConfig(projectRoot)
	if err != nil || existing != nil {
		return existing, err
	}
	port, err := FindFreePort(DefaultPortStart, DefaultPortEnd)
	if err != nil {
		return nil, err
	}
	name := Slug(filepath.Base(project.Root))
	if name == "" {
		name = "app"
	}
	config := &Config{Name: name, Type: project.Type, Root: project.Root, Port: port, Host: name + ".kmc.localhost"}
	return config, WriteConfig(projectRoot, *config)
}

func StartCommand(config Config) (string, error) {
	switch config.Type {
	case "nextjs":
		return fmt.Sprintf("npx next dev --port %d", config.Port), nil
	case "vite":
		return fmt.Sprintf("npx vite --host 127.0.0.1 --port %d", config.Port), nil
	case "nestjs":
		return fmt.Sprintf("PORT=%d npm run start:dev", config.Port), nil
	case "express":
		return fmt.Sprintf("PORT=%d npm run dev", config.Port), nil
	default:
		return "", fmt.Errorf("unsupported project type %q", config.Type)
	}
}

func ConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kmc"), nil
}

func UpsertCaddySite(config Config, cwd string) (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	sitesFile := filepath.Join(dir, "dev-sites.json")
	var sites []Site
	if data, readErr := os.ReadFile(sitesFile); readErr == nil {
		_ = json.Unmarshal(data, &sites)
	}
	next := make([]Site, 0, len(sites)+1)
	for _, site := range sites {
		if site.CWD != cwd && site.Host != config.Host {
			next = append(next, site)
		}
	}
	next = append(next, Site{CWD: cwd, Name: config.Name, Type: config.Type, Host: config.Host, Port: config.Port})
	sort.Slice(next, func(i, j int) bool { return next[i].Host < next[j].Host })
	data, _ := json.MarshalIndent(next, "", "  ")
	if err := os.WriteFile(sitesFile, append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	var caddy strings.Builder
	for index, site := range next {
		if index > 0 {
			caddy.WriteByte('\n')
		}
		fmt.Fprintf(&caddy, "%s {\n    reverse_proxy localhost:%d\n}\n", site.Host, site.Port)
	}
	path := filepath.Join(dir, "Caddyfile")
	return path, os.WriteFile(path, []byte(caddy.String()), 0o644)
}

func ReloadCaddy(path string) error {
	command := exec.Command("caddy", "reload", "--config", path)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reload failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func TrustCaddy() error {
	command := exec.Command("caddy", "trust")
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	return command.Run()
}
