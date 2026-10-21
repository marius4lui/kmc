package update

import "testing"

func TestCompareVersionsSemVerPrecedence(t *testing.T) {
	versions := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.10", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0",
		"1.0.1+build.1", "1.1.0", "2.0.0",
	}
	for i, left := range versions {
		for j, right := range versions {
			want := 0
			if i < j {
				want = -1
			} else if i > j {
				want = 1
			}
			if got := CompareVersions(left, right); got != want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", left, right, got, want)
			}
		}
	}
}

func TestCompareVersionsMetadataAndIdentifiers(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{" v2.0.1+build.001 ", "2.0.1+other-build.2", 0},
		{"2.0.0-beta.2+build.10", "2.0.0-beta.2+build.2", 0},
		{"2.0.0-1", "2.0.0-a", -1},
		{"2.0.0-999999999999999999999999999999", "2.0.0-a", -1},
		{"2.0.0-beta.1000000000000000000000000000000", "2.0.0-beta.999999999999999999999999999999", 1},
		{"1000000000000000000000000000000.0.0", "999999999999999999999999999999.0.0", 1},
		{"2.1000000000000000000000000000000.0", "2.999999999999999999999999999999.0", 1},
		{"2.0.1000000000000000000000000000000", "2.0.999999999999999999999999999999", 1},
		{"2.0.0-beta2", "2.0.0-beta10", 1},
		{"2.0.0-A", "2.0.0-a", -1},
		{"2.0.0-alpha-1", "2.0.0-alpha-2", -1},
	}
	for _, test := range tests {
		if got := CompareVersions(test.left, test.right); got != test.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
		if got := CompareVersions(test.right, test.left); got != -test.want {
			t.Errorf("reverse CompareVersions(%q, %q) = %d, want %d", test.right, test.left, got, -test.want)
		}
	}
}

func TestCompareVersionsInvalid(t *testing.T) {
	invalid := []string{
		"", "dev", "nightly", "1", "1.2", "1.2.3.4", "1.2.x", "01.2.3", "1.02.3", "1.2.03",
		"1.2.3-", "1.2.3-beta..1", "1.2.3-beta.01", "1.2.3+", "1.2.3+build..1",
		"1.2.3+build+1", "1.2.3-beta_1", "1.2.3+build_1", "1.2.3-béta", "vv1.2.3", "-1.2.3",
	}
	for _, version := range invalid {
		if got := CompareVersions(version, "0.0.0-alpha"); got != -1 {
			t.Errorf("invalid %q must sort below a valid version, got %d", version, got)
		}
		if got := CompareVersions("0.0.0-alpha", version); got != 1 {
			t.Errorf("valid version must sort above invalid %q, got %d", version, got)
		}
		if got := CompareVersions(version, version); got != 0 {
			t.Errorf("invalid %q must compare equal to itself, got %d", version, got)
		}
	}
}

func TestResolveSemanticVersions(t *testing.T) {
	tests := []struct {
		name, channel, want string
		releases            []Release
	}{
		{"beta", Beta, "v2.0.0-beta.10", []Release{{TagName: "v2.0.0-beta.2", Prerelease: true}, {TagName: "v2.0.0-beta.10", Prerelease: true}}},
		{"experimental", Experimental, "v2.0.0-beta.10", []Release{{TagName: "v2.0.0-beta.2"}, {TagName: "v2.0.0-beta.10"}}},
		{"stable metadata", Stable, "v2.0.1+build.1", []Release{{TagName: "v2.0.0"}, {TagName: "v2.0.1+build.1"}}},
		{"stable metadata hyphen", Stable, "v2.0.1+build-one", []Release{{TagName: "v2.0.0"}, {TagName: "v2.0.1+build-one"}}},
		{"nightly", Nightly, "v2.0.0-nightly.10", []Release{{TagName: "v2.0.0-nightly.2", Prerelease: true}, {TagName: "v2.0.0-nightly.10", Prerelease: true}}},
		{"unversioned nightly", Nightly, "nightly-20261021", []Release{{TagName: "nightly-20261020", Prerelease: true}, {TagName: "nightly-20261021", Prerelease: true}}},
		{"invalid tag", Stable, "v2.0.0", []Release{{TagName: "99.0.invalid"}, {TagName: "v2.0.0"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for order := 0; order < 2; order++ {
				release, err := Resolve(test.releases, test.channel, "")
				if err != nil || release.TagName != test.want {
					t.Fatalf("Resolve(order %d) = %#v, %v, want %s", order, release, err, test.want)
				}
				test.releases[0], test.releases[1] = test.releases[1], test.releases[0]
			}
		})
	}
}

func TestResolvePreservesEqualPrecedenceAndExplicitTags(t *testing.T) {
	for _, releases := range [][]Release{
		{{TagName: "v2.0.1+build.2"}, {TagName: "v2.0.1+build.1"}},
		{{TagName: "nightly-20261021"}, {TagName: "nightly-20261020"}},
	} {
		channel := Stable
		if releases[0].TagName == "nightly-20261021" {
			channel = Nightly
		}
		release, err := Resolve(releases, channel, "")
		if err != nil || release.TagName != releases[0].TagName {
			t.Fatalf("Resolve = %#v, %v; equal precedence must preserve input order", release, err)
		}
		release, err = Resolve(releases, channel, releases[1].TagName)
		if err != nil || release.TagName != releases[1].TagName {
			t.Fatalf("explicit Resolve = %#v, %v; want %s", release, err, releases[1].TagName)
		}
	}
}

func TestResolveBuildMetadataIsNotPrerelease(t *testing.T) {
	releases := []Release{{TagName: "v2.0.1+build-one"}}
	for _, channel := range []string{Beta, Experimental} {
		if release, err := Resolve(releases, channel, ""); err == nil || release != nil {
			t.Errorf("Resolve(%s) = %#v, %v; build metadata must not create a prerelease", channel, release, err)
		}
	}
}

func TestCompareVersionsUnversionedNightly(t *testing.T) {
	if got := CompareVersions("nightly-20261021", "nightly-20261020"); got != 1 {
		t.Fatalf("newer dated nightly must report an available update, got %d", got)
	}
	if got := CompareVersions("nightly-20261020", "nightly-20261021"); got != -1 {
		t.Fatalf("older dated nightly must not report an update, got %d", got)
	}
}
