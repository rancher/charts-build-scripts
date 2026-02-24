package auto

import (
	"context"
	"errors"
	"testing"

	"github.com/blang/semver"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/stretchr/testify/assert"
)

func Test_parseRepoPrefixVersionIfAny(t *testing.T) {

	versionWithRepoPrefix := "105.0.0+up1.0.0"
	versionWithoutRepoPrefix := "1.0.0"

	t.Run("version with repo prefix", func(t *testing.T) {
		repoPrefixVersion, version, found := parseRepoPrefixVersionIfAny(versionWithRepoPrefix)
		assert.Equal(t, true, found)
		assert.Equal(t, "105.0.0", repoPrefixVersion)
		assert.Equal(t, "1.0.0", version)
	})

	t.Run("version without repo prefix", func(t *testing.T) {
		repoPrefixVersion, version, found := parseRepoPrefixVersionIfAny(versionWithoutRepoPrefix)
		assert.Equal(t, false, found)
		assert.Equal(t, "", repoPrefixVersion)
		assert.Equal(t, "1.0.0", version)
	})
}

func Test_loadVersions(t *testing.T) {
	type input struct {
		bump *Bump
	}
	type expected struct {
		err  error
		bump *Bump
	}
	type test struct {
		name     string
		input    input
		expected expected
	}

	buildInputBump := func(toReleaseVersion string, existingVersions []string) *Bump {
		var toReleaseVersionPtr *string = &toReleaseVersion
		return &Bump{
			target: target{
				main: "rancher-chart",
			},
			Pkg: &charts.Package{
				Name:  "rancher-chart",
				Chart: charts.Chart{UpstreamChartVersion: toReleaseVersionPtr},
			},
			assetsVersionsMap: map[string][]string{
				"rancher-chart": existingVersions,
			},
		}
	}

	buildExpectedBump := func(latest, latestRepoPrefix, toRelease string) *Bump {
		latestVersion, latestRepoPrefixVersion, toReleaseVersion := &version{}, &version{}, &version{}

		if latest != "" {
			latestVersion.txt = latest
			latestVersion.updateSemver()
		}

		if latestRepoPrefix != "" {
			latestRepoPrefixVersion.txt = latestRepoPrefix
			latestRepoPrefixVersion.updateSemver()
		}

		if toRelease != "" {
			toReleaseVersion.txt = toRelease
			toReleaseVersion.updateSemver()
		}

		return &Bump{
			versions: &versions{
				latest:              latestVersion,
				latestRepoPrefix:    latestRepoPrefixVersion,
				toRelease:           toReleaseVersion,
				toReleaseRepoPrefix: &version{},
				currentRCs:          make([]rc, 0),
			},
		}

	}

	tests := []test{
		// Successes
		{
			name: "#1",
			input: input{
				bump: buildInputBump("2.0.0", []string{"1.0.0"}),
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("1.0.0", "", "2.0.0"),
			},
		},
		{
			name: "#2",
			input: input{
				bump: buildInputBump("3.0.0", []string{"2.0.0", "1.0.0"}),
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("2.0.0", "", "3.0.0"),
			},
		},
		{
			name: "#3",
			input: input{
				bump: buildInputBump("3.0.0", []string{"108.0.1+up2.0.0", "108.0.0+up1.0.0"}),
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("2.0.0", "108.0.1", "3.0.0"),
			},
		},
		{
			name: "#4",
			input: input{
				bump: buildInputBump("3.0.0", []string{"108.0.0+up1.0.0"}),
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("1.0.0", "108.0.0", "3.0.0"),
			},
		},
		// Errors
		{
			name: "#5",
			input: input{
				bump: buildInputBump("", []string{"108.0.0+up2.0.0"}),
			},
			expected: expected{
				err: errChartUpstreamVersion,
			},
		},
		{
			name: "#6",
			input: input{
				bump: buildInputBump("108.0.1+up3.0.0", []string{"108.0.0+up2.0.0"}),
			},
			expected: expected{
				err: errChartUpstreamVersionWrong,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.bump.loadVersions()
			assertError(t, err, tc.expected.err)
			if err == nil {
				assert.Equal(t, tc.expected.bump.versions.latest.txt, tc.input.bump.versions.latest.txt)
				assert.Equal(t, tc.expected.bump.versions.latest.svr, tc.input.bump.versions.latest.svr)

				assert.Equal(t, tc.expected.bump.versions.latestRepoPrefix.txt, tc.input.bump.versions.latestRepoPrefix.txt)
				assert.Equal(t, tc.expected.bump.versions.latestRepoPrefix.svr, tc.input.bump.versions.latestRepoPrefix.svr)

				assert.Equal(t, tc.expected.bump.versions.toRelease.txt, tc.input.bump.versions.toRelease.txt)
				assert.Equal(t, tc.expected.bump.versions.toRelease.svr, tc.input.bump.versions.toRelease.svr)
			}
		})
	}
}

func Test_applyVersionRules(t *testing.T) {
	type input struct {
		bump            *Bump
		cfg             *config.Config
		versionOverride string
	}
	type expected struct {
		err  error
		bump *Bump
	}
	type test struct {
		name     string
		input    input
		expected expected
	}

	buildInputBump := func(latest, latestRepoPrefix, toRelease string) *Bump {
		latestVersion, latestRepoPrefixVersion, toReleaseVersion := &version{}, &version{}, &version{}
		if latest != "" {
			latestVersion.txt = latest
			latestVersion.updateSemver()
		}
		if latestRepoPrefix != "" {
			latestRepoPrefixVersion.txt = latestRepoPrefix
			latestRepoPrefixVersion.updateSemver()
		}
		if toRelease != "" {
			toReleaseVersion.txt = toRelease
			toReleaseVersion.updateSemver()
		}

		return &Bump{
			target: target{
				branchLine: "2.10",
			},
			versions: &versions{
				latest:              latestVersion,
				latestRepoPrefix:    latestRepoPrefixVersion,
				toRelease:           toReleaseVersion,
				toReleaseRepoPrefix: &version{},
				currentRCs:          nil,
			},
		}
	}

	buildExpectedBump := func(toReleaseRepoPrefix string) *Bump {
		toReleaseRepoPrefixVersion := &version{}
		if toReleaseRepoPrefix != "" {
			toReleaseRepoPrefixVersion.txt = toReleaseRepoPrefix
			toReleaseRepoPrefixVersion.updateSemver()
		}

		return &Bump{
			versions: &versions{
				toReleaseRepoPrefix: toReleaseRepoPrefixVersion,
			},
		}
	}

	tests := []test{
		{
			name: "#1",
			input: input{
				bump: buildInputBump("", "", ""),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 105},
					},
				},
				versionOverride: "auto",
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("105.0.0"),
			},
		},
		{
			name: "#2",
			input: input{
				bump: buildInputBump("1.0.0", "105.0.0", "2.0.0"),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 105},
					},
				},
				versionOverride: "auto",
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("105.1.0"),
			},
		},
		{
			name: "#3",
			input: input{
				bump: buildInputBump("1.0.0", "105.0.0", "1.1.0"),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 105},
					},
				},
				versionOverride: "auto",
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("105.1.0"),
			},
		},
		{
			name: "#4",
			input: input{
				bump: buildInputBump("1.0.0", "105.0.0", "1.0.1"),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 105},
					},
				},
				versionOverride: "auto",
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("105.0.1"),
			},
		},
		{
			name: "#5",
			input: input{
				bump: buildInputBump("1.0.0", "105.0.0", "1.0.1"),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 105},
					},
				},
				versionOverride: "auto",
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("105.0.1"),
			},
		},
		{
			name: "#6",
			input: input{
				bump: buildInputBump("1.0.0", "104.0.0", "1.0.0"),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 105},
					},
				},
				versionOverride: "auto",
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("105.0.0"),
			},
		},
		{
			name: "#7",
			input: input{
				bump: buildInputBump("1.0.0", "103.0.0", "2.0.0"),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 105},
					},
				},
				versionOverride: "auto",
			},
			expected: expected{
				err: errors.New("difference between major versions is more than 1 or repoPrefix version is lower than latestRepoPrefix"),
			},
		},
		{
			name: "#8",
			input: input{
				bump: buildInputBump("1.0.0", "105.0.0", "2.0.0"),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 104},
					},
				},
				versionOverride: "auto",
			},
			expected: expected{
				err: errors.New("difference between major versions is more than 1 or repoPrefix version is lower than latestRepoPrefix"),
			},
		},
		{
			name: "#9",
			input: input{
				bump: buildInputBump("1.0.0", "105.0.0", "1.0.1"),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 105},
					},
				},
				versionOverride: "patch",
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("105.0.1"),
			},
		},
		{
			name: "#10",
			input: input{
				bump: buildInputBump("1.0.0", "105.0.0", "1.0.1"),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 105},
					},
				},
				versionOverride: "minor",
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("105.1.0"),
			},
		},
		{
			name: "#11",
			input: input{
				bump: buildInputBump("1.0.0", "105.0.0", "2.0.0"),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 105},
					},
				},
				versionOverride: "minor",
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("105.1.0"),
			},
		},
		{
			name: "#12",
			input: input{
				bump: buildInputBump("1.0.0", "105.0.0", "1.1.0"),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 105},
					},
				},
				versionOverride: "minor",
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("105.1.0"),
			},
		},
		{
			name: "#13",
			input: input{
				bump: buildInputBump("1.0.0", "105.0.0", "2.0.0"),
				cfg: &config.Config{
					VersionRules: &config.VersionRules{
						Rules: map[string]int{"2.10": 105},
					},
				},
				versionOverride: "patch",
			},
			expected: expected{
				err:  nil,
				bump: buildExpectedBump("105.0.1"),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.bump.applyVersionRules(tc.input.cfg, tc.input.versionOverride)

			assertError(t, err, tc.expected.err)
			if tc.expected.err == nil {
				assert.Equal(t,
					tc.expected.bump.versions.toReleaseRepoPrefix.txt,
					tc.input.bump.versions.toReleaseRepoPrefix.txt)

				assert.Equal(t,
					tc.expected.bump.versions.toReleaseRepoPrefix.svr,
					tc.input.bump.versions.toReleaseRepoPrefix.svr)
			}
		})
	}
}

func Test_calculateNextVersion(t *testing.T) {

	type input struct {
		bump            *Bump
		versionOverride string
		newChart        bool
	}
	type expected struct {
		err  error
		bump *Bump
	}
	type test struct {
		name     string
		input    *input
		expected *expected
	}

	ctx := context.Background()

	cfg := &config.Config{
		VersionRules: &config.VersionRules{
			Rules: map[string]int{
				"2.10": 105,
				"2.9":  104,
			},
		},
	}

	makeSemver := func(version string) *semver.Version {
		sv := semver.MustParse(version)
		return &sv
	}

	buildExpectedVersions := func(latest, latestRepoPrefix, toRelease, toReleaseRepoPrefix string) *versions {
		v := &versions{
			latest:              &version{},
			latestRepoPrefix:    &version{},
			toRelease:           &version{},
			toReleaseRepoPrefix: &version{},
		}

		if latest != "" {
			v.latest.txt = latest
			v.latest.svr = makeSemver(latest)
		}
		if latestRepoPrefix != "" {
			v.latestRepoPrefix.txt = latestRepoPrefix
			v.latestRepoPrefix.svr = makeSemver(latestRepoPrefix)
		}
		if toRelease != "" {
			v.toRelease.txt = toRelease
			v.toRelease.svr = makeSemver(toRelease)
		}
		if toReleaseRepoPrefix != "" {
			v.toReleaseRepoPrefix.txt = toReleaseRepoPrefix
			v.toReleaseRepoPrefix.svr = makeSemver(toReleaseRepoPrefix)
		}

		return v
	}

	buildInputBump := func(latestVersion string, toReleaseVersion string, versions []string) *Bump {
		var toReleaseVersionPtr *string = &toReleaseVersion

		input := &Bump{
			target: target{
				main:       "rancher-chart",
				branchLine: "2.10",
			},
			Pkg: &charts.Package{
				Name:  "rancher-chart",
				Chart: charts.Chart{UpstreamChartVersion: toReleaseVersionPtr},
			},
			assetsVersionsMap: map[string][]string{"rancher-chart": {latestVersion}},
		}

		input.assetsVersionsMap["rancher-chart"] = []string{}
		input.assetsVersionsMap["rancher-chart"] = append(input.assetsVersionsMap["rancher-chart"], latestVersion)

		if len(versions) > 0 {
			for _, v := range versions {
				input.assetsVersionsMap["rancher-chart"] = append(input.assetsVersionsMap["rancher-chart"], v)
			}
		}

		return input
	}

	buildExpectedBump := func(latest, latestRepoPrefix, toRelease, toReleaseRepoPrefix string) *Bump {
		var (
			toReleaseVersionPtr *string = &toRelease
			targetVersion       string
			targetSemver        semver.Version
		)

		if toReleaseRepoPrefix != "" {
			targetVersion = toReleaseRepoPrefix + "+up" + toRelease
			targetSemver = semver.MustParse(targetVersion)
		}

		return &Bump{
			target: target{main: "rancher-chart"},
			Pkg: &charts.Package{
				Name:                     "rancher-chart",
				Chart:                    charts.Chart{UpstreamChartVersion: toReleaseVersionPtr},
				AutoGeneratedBumpVersion: &targetSemver,
			},
			versions:          buildExpectedVersions(latest, latestRepoPrefix, toRelease, toReleaseRepoPrefix),
			assetsVersionsMap: map[string][]string{"rancher-chart": []string{latest}},
		}
	}

	tests := []test{
		{
			// success cases
			name: "#1",
			input: &input{
				bump:            buildInputBump("1.0.0", "2.0.0", []string{}),
				versionOverride: "auto",
				newChart:        false,
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("1.0.0", "", "2.0.0", "105.0.0"),
			},
		},
		{
			name: "#2",
			input: &input{
				bump:            buildInputBump("104.9.9+up1.0.0", "2.0.0", []string{}),
				versionOverride: "auto",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("1.0.0", "104.9.9", "2.0.0", "105.0.0"),
			},
		},
		{
			name: "#3",
			input: &input{
				bump:            buildInputBump("105.0.0+up1.0.0", "2.0.0", []string{}),
				versionOverride: "auto",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("1.0.0", "105.0.0", "2.0.0", "105.1.0"),
			},
		},
		{
			name: "#4",
			input: &input{
				bump:            buildInputBump("105.0.0+up1.0.0", "1.1.0", []string{}),
				versionOverride: "auto",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("1.0.0", "105.0.0", "1.1.0", "105.1.0"),
			},
		},
		{
			name: "#5",
			input: &input{
				bump:            buildInputBump("105.0.0+up1.0.0", "1.0.1", []string{}),
				versionOverride: "auto",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("1.0.0", "105.0.0", "1.0.1", "105.0.1"),
			},
		},
		{
			name: "#6",
			input: &input{
				bump:            buildInputBump("105.1.2+up1.1.1", "1.1.4", []string{}),
				versionOverride: "auto",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("1.1.1", "105.1.2", "1.1.4", "105.1.3"),
			},
		},
		{
			name: "#7",
			input: &input{
				bump:            buildInputBump("105.1.2+up1.1.1", "1.2.4", []string{}),
				versionOverride: "auto",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("1.1.1", "105.1.2", "1.2.4", "105.2.0"),
			},
		},
		{
			name: "#8",
			input: &input{
				bump:            buildInputBump("105.1.2+up1.1.1", "2.2.4", []string{}),
				versionOverride: "auto",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("1.1.1", "105.1.2", "2.2.4", "105.2.0"),
			},
		},
		{
			name: "#9",
			input: &input{
				bump:            buildInputBump("105.1.2+up1.1.1", "2.1.1", []string{}),
				versionOverride: "",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("1.1.1", "105.1.2", "2.1.1", "105.2.0"),
			},
		},
		{
			name: "#10",
			input: &input{
				bump:            buildInputBump("105.1.2+up1.1.1", "1.2.0", []string{}),
				versionOverride: "",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("1.1.1", "105.1.2", "1.2.0", "105.2.0"),
			},
		},

		{
			name: "#11",
			input: &input{
				bump:            buildInputBump("105.1.2+up1.1.1-rc.1", "1.1.1-rc.2", []string{"105.1.1+up1.1.0"}),
				versionOverride: "",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("1.1.0", "105.1.1", "1.1.1-rc.2", "105.1.2"),
			},
		},

		{
			name: "#12",
			input: &input{
				bump:            buildInputBump("105.1.2+up1.1.1-rc.2", "1.1.1-rc.3", []string{"105.1.2+up1.1.1-rc.1", "105.1.1+up1.1.0"}),
				versionOverride: "",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("1.1.0", "105.1.1", "1.1.1-rc.3", "105.1.2"),
			},
		},
		// failure cases
		{
			name: "#13",
			input: &input{
				bump:            buildInputBump("", "2.0.0", []string{}),
				versionOverride: "",
			},
			expected: &expected{
				err: errChartLatestVersion,
			},
		},
		{
			name: "#14",
			input: &input{
				bump:            buildInputBump("105.0.0+up1.0.0", "", []string{}),
				versionOverride: "",
			},
			expected: &expected{
				err: errChartUpstreamVersion,
			},
		},
		{
			name: "#15",
			input: &input{
				bump:            buildInputBump("105.0.0+up1.0.0", "105.1.1+up2.0.0", []string{}),
				versionOverride: "",
			},
			expected: &expected{
				err: errChartUpstreamVersionWrong,
			},
		},
		{
			name: "#16",
			input: &input{
				bump:            buildInputBump("105.0.0+up2.0.0", "1.0.0", []string{}),
				versionOverride: "",
			},
			expected: &expected{
				err: errBumpVersion,
			},
		},
		{
			name: "#17",
			input: &input{
				bump:            buildInputBump("105.1.2+up2.0.0", "3.0.0", []string{"105.0.0+up1.2.3"}),
				versionOverride: "minor",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("2.0.0", "105.1.2", "3.0.0", "105.2.0"),
			},
		},
		{
			name: "#18",
			input: &input{
				bump:            buildInputBump("105.1.2+up2.0.0", "2.1.0-rc.1", []string{"105.0.0+up1.2.3"}),
				versionOverride: "minor",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("2.0.0", "105.1.2", "2.1.0-rc.1", "105.2.0"),
			},
		},
		{
			name: "#19",
			input: &input{
				bump:            buildInputBump("105.1.0+up1.0.0-rc.1", "1.0.0-rc.2", []string{"105.0.0+up0.1.3"}),
				versionOverride: "minor",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("0.1.3", "105.0.0", "1.0.0-rc.2", "105.1.0"),
			},
		},
		{
			name: "#20",
			input: &input{
				bump:            buildInputBump("105.1.0+up1.0.0-rc.1", "1.0.0-rc.2", []string{"105.0.0+up0.1.3"}),
				versionOverride: "patch",
			},
			expected: &expected{
				err:  nil,
				bump: buildExpectedBump("0.1.3", "105.0.0", "1.0.0-rc.2", "105.0.1"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.bump.calculateNextVersion(
				ctx,
				cfg,
				tc.input.versionOverride,
				tc.input.newChart,
			)

			assertError(t, err, tc.expected.err)
			if tc.expected.err == nil {
				// AutoGeneratedBumpVersion
				assert.Equal(t,
					tc.expected.bump.Pkg.AutoGeneratedBumpVersion,
					tc.input.bump.Pkg.AutoGeneratedBumpVersion)

				// Latest
				assert.Equal(t,
					tc.expected.bump.versions.latest.txt,
					tc.input.bump.versions.latest.txt)

				assert.Equal(t,
					tc.expected.bump.versions.latest.svr,
					tc.input.bump.versions.latest.svr)

				// LatestRepoPrefix
				assert.Equal(t,
					tc.expected.bump.versions.latestRepoPrefix.txt,
					tc.input.bump.versions.latestRepoPrefix.txt)

				assert.Equal(t,
					tc.expected.bump.versions.latestRepoPrefix.svr,
					tc.input.bump.versions.latestRepoPrefix.svr)

				// toRelease
				assert.Equal(t,
					tc.expected.bump.versions.toRelease.txt,
					tc.input.bump.versions.toRelease.txt)

				assert.Equal(t,
					tc.expected.bump.versions.toRelease.svr,
					tc.input.bump.versions.toRelease.svr)

				// toReleaseRepoPrefix
				assert.Equal(t,
					tc.expected.bump.versions.toReleaseRepoPrefix.txt,
					tc.input.bump.versions.toReleaseRepoPrefix.txt)

				assert.Equal(t,
					tc.expected.bump.versions.toReleaseRepoPrefix.svr,
					tc.input.bump.versions.toReleaseRepoPrefix.svr)
			}
		})
	}
}
