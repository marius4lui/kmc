package devurls

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAndDiscoverProjects(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "apps", "web", "landing")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pnpm-workspace.yaml"), []byte("packages:\n  - apps/**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := `{"dependencies":{"next":"^15.0.0"}}`
	if err := os.WriteFile(filepath.Join(app, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	searchRoot, err := FindProjectSearchRoot(app)
	if err != nil {
		t.Fatal(err)
	}
	if searchRoot != root {
		t.Fatalf("search root = %s, want %s", searchRoot, root)
	}
	projects, err := DiscoverProjects(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Name != "apps/web/landing" || projects[0].Type != "nextjs" {
		t.Fatalf("unexpected projects: %#v", projects)
	}
}

func TestStartCommands(t *testing.T) {
	command, err := StartCommand(Config{Type: "vite", Port: 43172})
	if err != nil {
		t.Fatal(err)
	}
	if command != "npx vite --host 127.0.0.1 --port 43172" {
		t.Fatalf("unexpected command %q", command)
	}
}

func TestSlug(t *testing.T) {
	if got := Slug("apps/web/My Landing"); got != "apps-web-my-landing" {
		t.Fatalf("slug = %q", got)
	}
}
