package update

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveChannels(t *testing.T) {
	releases := []Release{
		{TagName: "v2.0.0-rc.1", Prerelease: true},
		{TagName: "v1.9.0"},
		{TagName: "v1.8.0"},
	}
	stable, err := Resolve(releases, Stable, "")
	if err != nil || stable.TagName != "v1.9.0" {
		t.Fatalf("stable = %#v, %v", stable, err)
	}
	experimental, err := Resolve(releases, Experimental, "")
	if err != nil || experimental.TagName != "v2.0.0-rc.1" {
		t.Fatalf("experimental = %#v, %v", experimental, err)
	}
}

func TestAssetAndChecksum(t *testing.T) {
	release := Release{TagName: "v2.0.0", Assets: []Asset{{Name: "kmc_2.0.0_linux_amd64.tar.gz"}}}
	asset, err := AssetFor(release, "linux", "amd64")
	if err != nil || asset.Name != "kmc_2.0.0_linux_amd64.tar.gz" {
		t.Fatalf("asset = %#v, %v", asset, err)
	}
	content := []byte("kmc")
	sum := fmt.Sprintf("%x", sha256.Sum256(content))
	if got, err := ChecksumFor([]byte(sum+"  "+asset.Name+"\n"), asset.Name); err != nil || got != sum {
		t.Fatalf("checksum = %q, %v", got, err)
	}
	path := filepath.Join(t.TempDir(), "asset")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFile(path, sum); err != nil {
		t.Fatal(err)
	}
}

func TestCompareVersions(t *testing.T) {
	if CompareVersions("v2.0.0", "v2.0.0-rc.1") <= 0 {
		t.Fatal("final release should sort after prerelease")
	}
	if CompareVersions("v1.10.0", "v1.9.0") <= 0 {
		t.Fatal("numeric version comparison failed")
	}
}
