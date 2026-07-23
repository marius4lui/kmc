package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/marius4lui/kmc/internal/workflows"
)

type Entry struct {
	Root        string `json:"root"`
	Fingerprint string `json:"fingerprint"`
	TrustedAt   string `json:"trustedAt"`
}

type Store struct {
	Version      int              `json:"version"`
	Repositories map[string]Entry `json:"repositories"`
}

type Status struct {
	Trusted     bool
	Changed     bool
	Entry       *Entry
	Fingerprint string
}

func File() (string, error) {
	base := os.Getenv("KMC_CONFIG_HOME")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, "kmc", "trusted-repositories.json"), nil
}

func RepositoryFingerprint(loaded *workflows.Loaded) (string, error) {
	hash := sha256.New()
	if err := hashFile(hash, loaded.RegistryFile); err != nil {
		return "", err
	}
	files := make([]string, 0, len(loaded.Scripts))
	for _, script := range loaded.Scripts {
		files = append(files, script.File)
	}
	sort.Strings(files)
	for _, file := range files {
		if err := hashFile(hash, file); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func StatusFor(loaded *workflows.Loaded) (Status, error) {
	store, err := readStore()
	if err != nil {
		return Status{}, err
	}
	fingerprint, err := RepositoryFingerprint(loaded)
	if err != nil {
		return Status{}, err
	}
	key := repositoryID(loaded.ProjectRoot)
	entry, exists := store.Repositories[key]
	result := Status{Fingerprint: fingerprint}
	if exists {
		result.Entry = &entry
		result.Trusted = entry.Root == loaded.ProjectRoot && entry.Fingerprint == fingerprint
		result.Changed = entry.Fingerprint != fingerprint
	}
	return result, nil
}

func Trust(loaded *workflows.Loaded) error {
	store, err := readStore()
	if err != nil {
		return err
	}
	fingerprint, err := RepositoryFingerprint(loaded)
	if err != nil {
		return err
	}
	store.Repositories[repositoryID(loaded.ProjectRoot)] = Entry{
		Root: loaded.ProjectRoot, Fingerprint: fingerprint, TrustedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	return writeStore(store)
}

func Untrust(projectRoot string) (bool, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return false, err
	}
	store, err := readStore()
	if err != nil {
		return false, err
	}
	key := repositoryID(root)
	_, removed := store.Repositories[key]
	delete(store.Repositories, key)
	return removed, writeStore(store)
}

func repositoryID(root string) string {
	absolute, err := filepath.Abs(root)
	if err == nil {
		root = absolute
	}
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	return hex.EncodeToString(sum[:])
}

type byteWriter interface {
	Write([]byte) (int, error)
}

func hashFile(destination byteWriter, file string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	_, err = destination.Write(content)
	return err
}

func readStore() (Store, error) {
	file, err := File()
	if err != nil {
		return Store{}, err
	}
	content, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return Store{Version: 1, Repositories: make(map[string]Entry)}, nil
	}
	if err != nil {
		return Store{}, err
	}
	var store Store
	if err := json.Unmarshal(content, &store); err != nil {
		return Store{}, fmt.Errorf("read trust store: %w", err)
	}
	if store.Repositories == nil {
		store.Repositories = make(map[string]Entry)
	}
	return store, nil
}

func writeStore(store Store) error {
	file, err := File()
	if err != nil {
		return err
	}
	if store.Version == 0 {
		store.Version = 1
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	content, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(file), ".trusted-repositories-*.tmp")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, file)
}
