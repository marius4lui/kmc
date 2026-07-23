package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultRepository = "marius4lui/kmc"
	Stable            = "stable"
	Experimental      = "experimental"
	Nightly           = "nightly"
)

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Release struct {
	TagName    string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

type Client struct {
	Repository string
	HTTP       *http.Client
	APIBase    string
}

func NewClient(repository string) *Client {
	if repository == "" {
		repository = DefaultRepository
	}
	return &Client{
		Repository: repository,
		HTTP:       &http.Client{Timeout: 30 * time.Second},
		APIBase:    "https://api.github.com",
	}
}

func (c *Client) releasesURL() string {
	return strings.TrimRight(c.APIBase, "/") + "/repos/" + c.Repository + "/releases?per_page=100"
}

func (c *Client) Releases(ctx context.Context) ([]Release, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.releasesURL(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "kmc-updater")
	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return nil, fmt.Errorf("GitHub releases returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var releases []Release
	if err := json.NewDecoder(response.Body).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func Resolve(releases []Release, channel, version string) (*Release, error) {
	if version != "" {
		version = normalizeVersion(version)
		for index := range releases {
			if !releases[index].Draft && normalizeVersion(releases[index].TagName) == version {
				return &releases[index], nil
			}
		}
		return nil, fmt.Errorf("release %s was not found", version)
	}
	if channel == "" {
		channel = Stable
	}
	candidates := make([]Release, 0, len(releases))
	for _, release := range releases {
		if release.Draft {
			continue
		}
		switch channel {
		case Stable:
			if release.Prerelease || strings.Contains(normalizeVersion(release.TagName), "-") {
				continue
			}
		case Experimental:
			if !release.Prerelease && !strings.Contains(normalizeVersion(release.TagName), "-") {
				continue
			}
		case Nightly:
			if !strings.Contains(strings.ToLower(release.TagName), "nightly") {
				continue
			}
		default:
			return nil, fmt.Errorf("unknown channel %q", channel)
		}
		candidates = append(candidates, release)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return CompareVersions(candidates[i].TagName, candidates[j].TagName) > 0
	})
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no %s release is available", channel)
	}
	return &candidates[0], nil
}

func normalizeVersion(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "v")
}

func CompareVersions(left, right string) int {
	parse := func(value string) ([3]int, string) {
		var numbers [3]int
		parts := strings.SplitN(normalizeVersion(value), "-", 2)
		for index, raw := range strings.Split(parts[0], ".") {
			if index >= len(numbers) {
				break
			}
			numbers[index], _ = strconv.Atoi(raw)
		}
		suffix := ""
		if len(parts) == 2 {
			suffix = parts[1]
		}
		return numbers, suffix
	}
	lv, ls := parse(left)
	rv, rs := parse(right)
	for index := 0; index < 3; index++ {
		if lv[index] > rv[index] {
			return 1
		}
		if lv[index] < rv[index] {
			return -1
		}
	}
	if ls == rs {
		return 0
	}
	if ls == "" {
		return 1
	}
	if rs == "" {
		return -1
	}
	if ls > rs {
		return 1
	}
	return -1
}

func Platform(goos, goarch string) (string, string, error) {
	switch goos {
	case "linux", "darwin", "windows":
	default:
		return "", "", fmt.Errorf("unsupported operating system %q", goos)
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return "", "", fmt.Errorf("unsupported architecture %q", goarch)
	}
	return goos, goarch, nil
}

func AssetFor(release Release, goos, goarch string) (Asset, error) {
	goos, goarch, err := Platform(goos, goarch)
	if err != nil {
		return Asset{}, err
	}
	extension := ".tar.gz"
	if goos == "windows" {
		extension = ".zip"
	}
	version := normalizeVersion(release.TagName)
	expected := fmt.Sprintf("kmc_%s_%s_%s%s", version, goos, goarch, extension)
	for _, asset := range release.Assets {
		if asset.Name == expected {
			return asset, nil
		}
	}
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, "_"+goos+"_") && strings.Contains(name, "_"+goarch) && strings.HasSuffix(name, extension) {
			return asset, nil
		}
	}
	return Asset{}, fmt.Errorf("release %s has no asset for %s/%s (expected %s)", release.TagName, goos, goarch, expected)
}

func FindAsset(release Release, name string) (Asset, bool) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

func (c *Client) Download(ctx context.Context, asset Asset, destination string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "kmc-updater")
	response, err := c.HTTP.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("download %s returned %s", asset.Name, response.Status)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func ChecksumFor(checksums []byte, name string) (string, error) {
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[len(fields)-1], "*") == name {
			if len(fields[0]) != sha256.Size*2 {
				return "", fmt.Errorf("invalid SHA-256 for %s", name)
			}
			if _, err := hex.DecodeString(fields[0]); err != nil {
				return "", fmt.Errorf("invalid SHA-256 for %s: %w", name, err)
			}
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("%s is missing from checksums.txt", name)
}

func VerifyFile(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filepath.Base(path), expected, actual)
	}
	return nil
}

func ExtractBinary(archive, destination, goos string) error {
	if goos == "windows" {
		reader, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer reader.Close()
		for _, entry := range reader.File {
			if strings.EqualFold(filepath.Base(entry.Name), "kmc.exe") {
				source, err := entry.Open()
				if err != nil {
					return err
				}
				defer source.Close()
				return writeBinary(destination, source)
			}
		}
		return errors.New("archive does not contain kmc.exe")
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(header.Name) == "kmc" && header.Typeflag == tar.TypeReg {
			return writeBinary(destination, tarReader)
		}
	}
	return errors.New("archive does not contain kmc")
}

func writeBinary(destination string, source io.Reader) error {
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, source)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(destination, 0o755)
}

func AtomicReplace(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	backup := destination + ".previous"
	_ = os.Remove(backup)
	if _, err := os.Stat(destination); err == nil {
		if err := os.Rename(destination, backup); err != nil {
			return fmt.Errorf("create rollback copy: %w", err)
		}
	}
	if err := os.Rename(source, destination); err != nil {
		_ = os.Rename(backup, destination)
		return fmt.Errorf("install new binary: %w", err)
	}
	_ = os.Remove(backup)
	return nil
}

func CurrentPlatformAsset(release Release) (Asset, error) {
	return AssetFor(release, runtime.GOOS, runtime.GOARCH)
}
