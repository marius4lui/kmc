package importers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marius4lui/kmc/internal/config"
)

func TestDetectAndImportAllSources(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"package.json":       `{"scripts":{"dev":"vite","test":"node --test"}}`,
		"Makefile":           "build:\n\tgo build\nbuild:\n",
		"pubspec.yaml":       "name: example\n",
		"docker-compose.yml": "services:\n  api:\n    image: example\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	imports, err := Detect(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(imports) != 14 {
		t.Fatalf("got %d imports, want 14", len(imports))
	}
	if imports[0].ID != "npm.dev" || imports[0].Description != "vite" {
		t.Fatalf("npm order or fields changed: %#v", imports[0])
	}
	next, err := Import(root, config.Config{Groups: []config.Group{}}, []string{"npm"})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Groups) != 1 || next.Groups[0].ID != "npm" || len(next.Groups[0].Commands) != 2 {
		t.Fatalf("unexpected imported config: %#v", next)
	}
}

func TestMakefileCompatibility(t *testing.T) {
	commands := ParseMakefile(".PHONY: test\ntest: deps\nname=value\nfoo:=bar\nrelease:\nrelease:\n")
	if len(commands) != 2 || commands[0].Name != "test" || commands[1].Name != "release" {
		t.Fatalf("unexpected targets: %#v", commands)
	}
}

func TestDockerCompatibility(t *testing.T) {
	commands := ParseDockerCompose("services:\n  api:\n    image: api\n   ignored:\n  worker: {}\n")
	if len(commands) != 6 || commands[5].Name != "restart-api" {
		t.Fatalf("unexpected compose commands: %#v", commands)
	}
}
