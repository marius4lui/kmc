package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Exercise the real argument parsing and PATH helper without running the command
// dispatch: these tests must never download or install a binary.
func pathInstaller(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer requires a Unix shell")
	}
	source, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	dispatch := strings.LastIndex(string(source), "\ncase \"$COMMAND\" in\n")
	if dispatch < 0 {
		t.Fatal("installer command dispatch not found")
	}
	path := filepath.Join(t.TempDir(), "path-installer.sh")
	if err := os.WriteFile(path, append(source[:dispatch], []byte("\nadd_to_path\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func installerEnvironment(home, shell, path string) []string {
	var env []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "HOME", "SHELL", "PATH", "ZDOTDIR", "BAR", "ENV", "BASH_ENV":
			continue
		}
		env = append(env, entry)
	}
	return append(env, "HOME="+home, "SHELL="+shell, "PATH="+path,
		"ZDOTDIR="+filepath.Join(home, "zsh"), "BAR=expanded-value")
}

func runPathInstaller(t *testing.T, installer, home, shell, path, prefix string, modify bool) string {
	t.Helper()
	args := []string{installer, "--prefix", prefix}
	if modify {
		args = append(args, "--modify-path")
	}
	cmd := exec.Command("/bin/sh", args...)
	cmd.Env = installerEnvironment(home, shell, path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("PATH helper failed: %v\n%s", err, output)
	}
	return string(output)
}

func TestInstallerPathLiteralPrefixes(t *testing.T) {
	installer := pathInstaller(t)
	prefixes := []string{"plain", "with spaces", "$BAR", "single'quote", `double"quote`, `back\slash`, "[brackets]*?", `mixed $BAR's "path"\name`}
	for _, prefixName := range prefixes {
		t.Run(prefixName, func(t *testing.T) {
			for _, modify := range []bool{false, true} {
				name := "suggestion"
				if modify {
					name = "profile"
				}
				t.Run(name, func(t *testing.T) {
					home := t.TempDir()
					prefix := filepath.Join(home, prefixName)
					output := runPathInstaller(t, installer, home, "sh", "/usr/bin:/bin", prefix, modify)
					profile := filepath.Join(home, ".profile")
					var contents []byte
					if modify {
						var err error
						contents, err = os.ReadFile(profile)
						if err != nil {
							t.Fatal(err)
						}
						runPathInstaller(t, installer, home, "sh", "/usr/bin:/bin", prefix, true)
						again, err := os.ReadFile(profile)
						if err != nil || string(again) != string(contents) {
							t.Fatalf("repeated installation changed profile: %v\n%s", err, again)
						}
					} else {
						if _, err := os.Stat(profile); !os.IsNotExist(err) {
							t.Fatalf("suggestion unexpectedly wrote a profile: %v", err)
						}
						const heading = "Add this to your shell profile:\n  "
						if !strings.HasPrefix(output, heading) {
							t.Fatalf("unexpected suggestion: %q", output)
						}
						contents = []byte(strings.TrimPrefix(output, heading))
						if err := os.WriteFile(profile, contents, 0600); err != nil {
							t.Fatal(err)
						}
					}
					// PATH must expand in the future shell, not at installation time.
					const laterPath = "/later path:/usr/bin:/bin"
					cmd := exec.Command("/bin/sh", "-c", `. "$1"; printf '%s' "$PATH"`, "sh", profile)
					cmd.Env = installerEnvironment(home, "sh", laterPath)
					actual, err := cmd.CombinedOutput()
					want := filepath.Join(prefix, "bin") + ":" + laterPath
					if err != nil || string(actual) != want {
						t.Fatalf("profile PATH = %q, want %q (error %v)\nprofile:\n%s", actual, want, err, contents)
					}
				})
			}
		})
	}
}

func TestInstallerPathProfiles(t *testing.T) {
	installer := pathInstaller(t)
	for _, tc := range []struct{ shell, profile string }{
		{"/bin/sh", ".profile"},
		{"/bin/bash", ".bashrc"},
		{"/bin/zsh", "zsh/.zshrc"},
	} {
		t.Run(tc.shell, func(t *testing.T) {
			home := t.TempDir()
			if err := os.Mkdir(filepath.Join(home, "zsh"), 0700); err != nil {
				t.Fatal(err)
			}
			prefix := filepath.Join(home, "prefix")
			path := filepath.Join(prefix, "bin") + ":/usr/bin:/bin"
			if output := runPathInstaller(t, installer, home, tc.shell, path, prefix, true); output != "" {
				t.Fatalf("directory already in PATH: unexpected output %q", output)
			}
			profile := filepath.Join(home, tc.profile)
			if _, err := os.Stat(profile); !os.IsNotExist(err) {
				t.Fatalf("directory already in PATH: profile was written: %v", err)
			}
			runPathInstaller(t, installer, home, tc.shell, "/usr/bin:/bin", prefix, true)
			if _, err := os.Stat(profile); err != nil {
				t.Fatalf("expected shell profile was not written: %v", err)
			}
		})
	}
}
