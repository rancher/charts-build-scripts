package registries

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var addTagHelper = func(imgTagMap map[string][]string, repo, tag string) map[string][]string {
	new := newTagMap(imgTagMap)
	new[repo] = append(new[repo], tag)
	return new
}

func assertError(t *testing.T, err, expectedErr error) {
	if expectedErr != nil {
		assert.EqualError(t, err, expectedErr.Error())
	} else {
		assert.NoError(t, err)
	}
}

func Test_splitDockerOnlyAndStgImgTags(t *testing.T) {
	ctx := context.Background()

	type input struct {
		dockerImgTags map[string][]string
		stgImgTags    map[string][]string
	}
	type output struct {
		dockerOnly map[string][]string
		stgAlso    map[string][]string
	}
	type test struct {
		name   string
		input  input
		output output
	}

	tests := []test{
		{
			name: "#1",
			input: input{
				dockerImgTags: map[string][]string{
					"rancher/fleet":   {"v1.0.0", "v2.0.0", "v3.0.0"},
					"rancher/kubectl": {"v1.0.0", "v2.0.0"},
				},
				stgImgTags: map[string][]string{
					"rancher/fleet":   {"v1.0.0", "v2.0.0"},
					"rancher/kubectl": {"v1.0.0"},
				},
			},
			output: output{
				dockerOnly: map[string][]string{
					"rancher/fleet":   {"v3.0.0"},
					"rancher/kubectl": {"v2.0.0"},
				},
				stgAlso: map[string][]string{
					"rancher/fleet":   {"v1.0.0", "v2.0.0"},
					"rancher/kubectl": {"v1.0.0"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dockerResult, stagingResult := splitDockerOnlyAndStgImgTags(ctx, tt.input.dockerImgTags, tt.input.stgImgTags)
			require.Equal(t, tt.output.dockerOnly, dockerResult)
			require.Equal(t, tt.output.stgAlso, stagingResult)
		})
	}

}

func Test_splitTags(t *testing.T) {
	type input struct {
		dockerTags []string
		stgTags    []string
	}
	type output struct {
		dockerOnlyTags []string
		stgAlsoTags    []string
	}
	type test struct {
		name   string
		input  input
		output output
	}

	tests := []test{
		{
			name: "#1",
			input: input{
				dockerTags: []string{"v1.0.0", "v2.0.0", "v3.0.0"},
				stgTags:    []string{"v1.0.0"},
			},
			output: output{
				dockerOnlyTags: []string{"v2.0.0", "v3.0.0"},
				stgAlsoTags:    []string{"v1.0.0"},
			},
		},
		{
			name: "#2",
			input: input{
				dockerTags: []string{"v1.0.0", "v2.0.0", "v3.0.0"},
				stgTags:    []string{},
			},
			output: output{
				dockerOnlyTags: []string{"v1.0.0", "v2.0.0", "v3.0.0"},
				stgAlsoTags:    []string{},
			},
		},
		{
			name: "#3",
			input: input{
				dockerTags: []string{},
				stgTags:    []string{"v1.0.0", "v2.0.0", "v3.0.0"},
			},
			output: output{
				dockerOnlyTags: []string{},
				stgAlsoTags:    []string{},
			},
		},
		{
			name: "#4",
			input: input{
				dockerTags: []string{"v1.0.0", "v2.0.0", "v3.0.0"},
				stgTags:    []string{"v1.0.0", "v2.0.0", "v3.0.0"},
			},
			output: output{
				dockerOnlyTags: []string{},
				stgAlsoTags:    []string{"v1.0.0", "v2.0.0", "v3.0.0"},
			},
		},
		{
			name: "#5",
			input: input{
				dockerTags: []string{"v1.0.0"},
				stgTags:    []string{"v1.0.0"},
			},
			output: output{
				dockerOnlyTags: []string{},
				stgAlsoTags:    []string{"v1.0.0"},
			},
		},
		{
			name: "#6",
			input: input{
				dockerTags: []string{},
				stgTags:    []string{},
			},
			output: output{
				dockerOnlyTags: []string{},
				stgAlsoTags:    []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultDocker, resultStaging := splitTags(tt.input.dockerTags, tt.input.stgTags)
			require.Equal(t, tt.output.dockerOnlyTags, resultDocker)
			require.Equal(t, tt.output.stgAlsoTags, resultStaging)
		})
	}
}

func Test_filterDockerNotPrimeTags(t *testing.T) {
	ctx := context.Background()

	type input struct {
		dockerImgTags map[string][]string
		primeImgTags  map[string][]string
	}
	type output struct {
		dockerOnlyTags map[string][]string
	}
	type test struct {
		name   string
		input  input
		output output
	}

	tests := []test{
		{
			name: "#1",
			input: input{
				dockerImgTags: map[string][]string{
					"rancher/fleet": {"v1.0.0", "v2.0.0"},
				},
				primeImgTags: map[string][]string{
					"rancher/fleet": {"v1.0.0"},
				},
			},
			output: output{
				dockerOnlyTags: map[string][]string{
					"rancher/fleet": {"v2.0.0"},
				},
			},
		},
		{
			name: "#2",
			input: input{
				dockerImgTags: map[string][]string{
					"rancher/fleet": {"v1.0.0"},
				},
				primeImgTags: map[string][]string{
					"rancher/fleet": {"v1.0.0"},
				},
			},
			output: output{
				dockerOnlyTags: map[string][]string{},
			},
		},
		{
			name: "#3",
			input: input{
				dockerImgTags: map[string][]string{
					"rancher/fleet": {"v1.0.0"},
				},
				primeImgTags: map[string][]string{
					"rancher/fleet": {"v1.0.0", "sha256-something-big-here"},
				},
			},
			output: output{
				dockerOnlyTags: map[string][]string{},
			},
		},
		{
			name: "#4",
			input: input{
				dockerImgTags: map[string][]string{},
				primeImgTags: map[string][]string{
					"rancher/fleet": {"v1.0.0", "sha256-something-big-here"},
				},
			},
			output: output{
				dockerOnlyTags: map[string][]string{},
			},
		},
		{
			name: "#5",
			input: input{
				dockerImgTags: map[string][]string{},
				primeImgTags:  map[string][]string{},
			},
			output: output{
				dockerOnlyTags: map[string][]string{},
			},
		},
		{
			name: "#6",
			input: input{
				dockerImgTags: map[string][]string{
					"rancher/fleet": {},
				}, primeImgTags: map[string][]string{},
			},
			output: output{
				dockerOnlyTags: map[string][]string{
					"rancher/fleet": {},
				},
			},
		},
		{
			name: "#7",
			input: input{
				dockerImgTags: map[string][]string{
					"rancher/fleet": {"v1.0.0", "v2.0.0"},
					"rancher/shell": {"v1.0.0", "v2.0.0", "v3.0.0"},
				},
				primeImgTags: map[string][]string{
					"rancher/fleet": {"v1.0.0"},
					"rancher/shell": {"v1.0.0"},
				},
			},
			output: output{
				dockerOnlyTags: map[string][]string{
					"rancher/fleet": {"v2.0.0"},
					"rancher/shell": {"v2.0.0", "v3.0.0"},
				},
			},
		},
		{ // first time ever the image is synced
			name: "#8",
			input: input{
				dockerImgTags: map[string][]string{
					"rancher/fleet": {"v1.0.0"},
				},
				primeImgTags: map[string][]string{},
			},
			output: output{
				dockerOnlyTags: map[string][]string{
					"rancher/fleet": {"v1.0.0"},
				},
			},
		},
		{ // real mocked data that is already synced
			name: "#9",
			input: input{
				dockerImgTags: mockedAssetsFleetAgentTagMap,
				primeImgTags:  mockedPrimeFleetAgentTagMap,
			},
			output: output{
				dockerOnlyTags: map[string][]string{},
			},
		},
		{ // real mocked data that needs to be synced
			name: "#10",
			input: input{
				dockerImgTags: addTagHelper(mockedAssetsFleetAgentTagMap, "rancher/fleet-agent", "v999.99.99"),
				primeImgTags:  mockedPrimeFleetAgentTagMap,
			},
			output: output{
				dockerOnlyTags: map[string][]string{"rancher/fleet-agent": {"v999.99.99"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterDockerNotPrimeTags(ctx, tt.input.dockerImgTags, tt.input.primeImgTags)
			require.Equal(t, tt.output.dockerOnlyTags, result)
		})
	}
}

func Test_checkRegistriesImagesTags(t *testing.T) {
	ctx := context.Background()

	originalCreateFunc := createAssetValuesRepoTagMap
	originalListFunc := listRegistryImageTags
	defer func() {
		createAssetValuesRepoTagMap = originalCreateFunc
		listRegistryImageTags = originalListFunc
	}()

	type input struct {
		createAssetsMock func(context.Context) (map[string][]string, error)
		listRegistryMock func(context.Context, map[string][]string, string) (map[string][]string, error)
	}

	type output struct {
		assetsImageTagMap map[string][]string
		dockerToPrime     map[string][]string
		stagingToPrime    map[string][]string
		err               error
	}

	type test struct {
		name   string
		input  input
		output output
	}

	tests := []test{
		// success - staging -> prime sync needed
		{
			name: "#1.0 - Sync - stagingToPrime (1 tag)",
			input: input{
				createAssetsMock: func(context.Context) (map[string][]string, error) {
					return map[string][]string{
						"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"},
					}, nil
				},
				listRegistryMock: func(ctx context.Context, imageTagMap map[string][]string, registry string) (map[string][]string, error) {
					primeMock := map[string][]string{"rancher/fleet": {"v1.0.0", "v2.0.0"}}
					stagingMock := map[string][]string{"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"}}
					if registry == PrimeURL {
						return primeMock, nil
					}
					if registry == StagingURL {
						return stagingMock, nil
					}
					t.Fatalf("unexpected, should receive a registry[%s]", registry)
					return nil, nil
				},
			},
			output: output{
				assetsImageTagMap: map[string][]string{"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"}},
				dockerToPrime:     map[string][]string{},
				stagingToPrime:    map[string][]string{"rancher/fleet": {"v3.0.0"}},
				err:               nil,
			},
		},
		{
			name: "#1.1 - Sync - stagingToPrime (with .sig/.att tags)",
			input: input{
				createAssetsMock: func(context.Context) (map[string][]string, error) {
					return map[string][]string{
						"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"},
					}, nil
				},
				listRegistryMock: func(ctx context.Context, imageTagMap map[string][]string, registry string) (map[string][]string, error) {
					primeMock := map[string][]string{"rancher/fleet": {
						"v1.0.0", "v2.0.0",
						"sha256-fb4eb424291c2fb4181aee88dc4e4b454cce78488707e84872084116a5f38dbf.sig", "sha256-fb4eb424291c2fb4181aee88dc4e4b454cce78488707e84872084116a5f38dbf.att",
					}}
					stagingMock := map[string][]string{"rancher/fleet": {
						"v1.0.0", "v2.0.0", "v3.0.0",
						"sha256-fb4eb424291c2fb4181aee88dc4e4b454cce78488707e84872084116a5f38dbf.sig", "sha256-fb4eb424291c2fb4181aee88dc4e4b454cce78488707e84872084116a5f38dbf.att",
					}}
					if registry == PrimeURL {
						return primeMock, nil
					}
					if registry == StagingURL {
						return stagingMock, nil
					}
					t.Fatalf("unexpected, should receive a registry[%s]", registry)
					return nil, nil
				},
			},
			output: output{
				assetsImageTagMap: map[string][]string{"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"}},
				dockerToPrime:     map[string][]string{},
				stagingToPrime:    map[string][]string{"rancher/fleet": {"v3.0.0"}},
				err:               nil,
			},
		},
		{
			name: "#1.2 - Sync - stagingToPrime (real data)",
			input: input{
				createAssetsMock: func(context.Context) (map[string][]string, error) {
					repo := "rancher/fleet-agent"
					tag := "v999.99.99"
					return addTagHelper(mockedAssetsFleetAgentTagMap, repo, tag), nil
				},
				listRegistryMock: func(ctx context.Context, imageTagMap map[string][]string, registry string) (map[string][]string, error) {
					if registry == PrimeURL {
						return mockedPrimeFleetAgentTagMap, nil
					}
					if registry == StagingURL {
						repo := "rancher/fleet-agent"
						tag := "v999.99.99"
						return addTagHelper(mockedStagingFleetAgentTagMap, repo, tag), nil
					}
					t.Fatalf("unexpected, should receive a registry[%s]", registry)
					return nil, nil
				},
			},
			output: output{
				assetsImageTagMap: addTagHelper(mockedAssetsFleetAgentTagMap, "rancher/fleet-agent", "v999.99.99"),
				dockerToPrime:     map[string][]string{},
				stagingToPrime:    map[string][]string{"rancher/fleet-agent": {"v999.99.99"}},
				err:               nil,
			},
		},
		{
			name: "#2.0 - Sync - dockerToPrime (1 tag)",
			input: input{
				createAssetsMock: func(context.Context) (map[string][]string, error) {
					return map[string][]string{
						"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"},
					}, nil
				},
				listRegistryMock: func(ctx context.Context, imageTagMap map[string][]string, registry string) (map[string][]string, error) {
					primeMock := map[string][]string{"rancher/fleet": {"v1.0.0", "v2.0.0"}}
					stagingMock := map[string][]string{"rancher/fleet": {"v1.0.0", "v2.0.0"}}
					if registry == PrimeURL {
						return primeMock, nil
					}
					if registry == StagingURL {
						return stagingMock, nil
					}
					t.Fatalf("unexpected, should receive a registry[%s]", registry)
					return nil, nil
				},
			},
			output: output{
				assetsImageTagMap: map[string][]string{"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"}},
				dockerToPrime:     map[string][]string{"rancher/fleet": {"v3.0.0"}},
				stagingToPrime:    map[string][]string{},
				err:               nil,
			},
		},
		{
			name: "#2.1 - Sync - dockerToPrime (with .sig/.att tags)",
			input: input{
				createAssetsMock: func(context.Context) (map[string][]string, error) {
					return map[string][]string{
						"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"},
					}, nil
				},
				listRegistryMock: func(ctx context.Context, imageTagMap map[string][]string, registry string) (map[string][]string, error) {
					primeMock := map[string][]string{"rancher/fleet": {
						"v1.0.0", "v2.0.0",
						"sha256-fb4eb424291c2fb4181aee88dc4e4b454cce78488707e84872084116a5f38dbf.sig", "sha256-fb4eb424291c2fb4181aee88dc4e4b454cce78488707e84872084116a5f38dbf.att",
					}}
					stagingMock := map[string][]string{"rancher/fleet": {
						"v1.0.0", "v2.0.0",
						"sha256-fb4eb424291c2fb4181aee88dc4e4b454cce78488707e84872084116a5f38dbf.sig", "sha256-fb4eb424291c2fb4181aee88dc4e4b454cce78488707e84872084116a5f38dbf.att",
					}}
					if registry == PrimeURL {
						return primeMock, nil
					}
					if registry == StagingURL {
						return stagingMock, nil
					}
					t.Fatalf("unexpected, should receive a registry[%s]", registry)
					return nil, nil
				},
			},
			output: output{
				assetsImageTagMap: map[string][]string{"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"}},
				dockerToPrime:     map[string][]string{"rancher/fleet": {"v3.0.0"}},
				stagingToPrime:    map[string][]string{},
				err:               nil,
			},
		},
		{
			name: "#2.2 - Sync - dockerToPrime (real data)",
			input: input{
				createAssetsMock: func(context.Context) (map[string][]string, error) {
					repo := "rancher/fleet-agent"
					tag := "v999.99.99"
					return addTagHelper(mockedAssetsFleetAgentTagMap, repo, tag), nil
				},
				listRegistryMock: func(ctx context.Context, imageTagMap map[string][]string, registry string) (map[string][]string, error) {
					if registry == PrimeURL {
						return mockedPrimeFleetAgentTagMap, nil
					}
					if registry == StagingURL {
						return mockedStagingFleetAgentTagMap, nil
					}
					t.Fatalf("unexpected, should receive a registry[%s]", registry)
					return nil, nil
				},
			},
			output: output{
				assetsImageTagMap: addTagHelper(mockedAssetsFleetAgentTagMap, "rancher/fleet-agent", "v999.99.99"),
				dockerToPrime:     map[string][]string{"rancher/fleet-agent": {"v999.99.99"}},
				stagingToPrime:    map[string][]string{},
				err:               nil,
			},
		},
		{
			name: "#3.0 - No Sync (assets=staging=prime)",
			input: input{
				createAssetsMock: func(context.Context) (map[string][]string, error) {
					return map[string][]string{
						"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"},
					}, nil
				},
				listRegistryMock: func(ctx context.Context, imageTagMap map[string][]string, registry string) (map[string][]string, error) {
					primeMock := map[string][]string{"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"}}
					stagingMock := map[string][]string{"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"}}
					if registry == PrimeURL {
						return primeMock, nil
					}
					if registry == StagingURL {
						return stagingMock, nil
					}
					t.Fatalf("unexpected, should receive a registry[%s]", registry)
					return nil, nil
				},
			},
			output: output{
				assetsImageTagMap: map[string][]string{"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"}},
				dockerToPrime:     map[string][]string{},
				stagingToPrime:    map[string][]string{},
				err:               nil,
			},
		},
		{
			name: "#3.1 - NoSync (assets=staging=prime) even with .sig and .att tags",
			input: input{
				createAssetsMock: func(context.Context) (map[string][]string, error) {
					return map[string][]string{
						"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"},
					}, nil
				},
				listRegistryMock: func(ctx context.Context, imageTagMap map[string][]string, registry string) (map[string][]string, error) {
					primeMock := map[string][]string{"rancher/fleet": {
						"v1.0.0", "v2.0.0", "v3.0.0",
						"sha256-fb4eb424291c2fb4181aee88dc4e4b454cce78488707e84872084116a5f38dbf.sig", "sha256-fb4eb424291c2fb4181aee88dc4e4b454cce78488707e84872084116a5f38dbf.att",
					}}
					stagingMock := map[string][]string{"rancher/fleet": {
						"v1.0.0", "v2.0.0", "v3.0.0",
						"sha256-fb4eb424291c2fb4181aee88dc4e4b454cce78488707e84872084116a5f38dbf.sig", "sha256-fb4eb424291c2fb4181aee88dc4e4b454cce78488707e84872084116a5f38dbf.att",
					}}
					if registry == PrimeURL {
						return primeMock, nil
					}
					if registry == StagingURL {
						return stagingMock, nil
					}
					t.Fatalf("unexpected, should receive a registry[%s]", registry)
					return nil, nil
				},
			},
			output: output{
				assetsImageTagMap: map[string][]string{"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"}},
				dockerToPrime:     map[string][]string{},
				stagingToPrime:    map[string][]string{},
				err:               nil,
			},
		},
		{
			name: "#3.2 - No Sync (real mocked data)",
			input: input{
				createAssetsMock: func(context.Context) (map[string][]string, error) {
					return mockedAssetsFleetAgentTagMap, nil
				},
				listRegistryMock: func(ctx context.Context, imageTagMap map[string][]string, registry string) (map[string][]string, error) {
					if registry == PrimeURL {
						return mockedPrimeFleetAgentTagMap, nil
					}
					if registry == StagingURL {
						return mockedStagingFleetAgentTagMap, nil
					}
					t.Fatalf("unexpected, should receive a registry[%s]", registry)
					return nil, nil
				},
			},
			output: output{
				assetsImageTagMap: mockedAssetsFleetAgentTagMap,
				dockerToPrime:     map[string][]string{},
				stagingToPrime:    map[string][]string{},
				err:               nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createAssetValuesRepoTagMap = tt.input.createAssetsMock
			listRegistryImageTags = tt.input.listRegistryMock

			assetsImageTagMap, dockerToPrime, stagingToPrime, err := checkRegistriesImagesTags(ctx)
			assertError(t, err, tt.output.err)
			require.Equal(t, tt.output.assetsImageTagMap, assetsImageTagMap)
			require.Equal(t, tt.output.dockerToPrime, dockerToPrime)
			require.Equal(t, tt.output.stagingToPrime, stagingToPrime)
		})
	}
}

func Test_checkImagesFromDocker(t *testing.T) {
	ctx := context.Background()

	// reset monkey patching
	originalFetch := fetchTagsFromRegistryRepo
	defer func() {
		fetchTagsFromRegistryRepo = originalFetch
	}()

	type input struct {
		mockedAssetsTags map[string][]string
		mockFetchTags    func(context.Context, string, string) ([]string, error)
	}

	type output struct {
		failedImages         map[string][]string
		outOfNamespaceImages []string
		err                  error
	}

	type test struct {
		name   string
		input  input
		output output
	}

	tests := []test{
		{
			name: "#1",
			input: input{
				mockedAssetsTags: map[string][]string{
					"rancher/fleet": {"v1.0.0", "v2.0.0", "v3.0.0"},
					"rancher/shell": {"v1.0.0"},
				},
				mockFetchTags: func(ctx context.Context, repo, asset string) ([]string, error) {
					if repo+asset == DockerURL+"rancher/fleet" {
						return []string{"v1.0.0", "v2.0.0", "v3.0.0"}, nil
					}
					return []string{"v1.0.0"}, nil
				},
			},
			output: output{
				failedImages:         map[string][]string{},
				outOfNamespaceImages: []string{},
				err:                  nil,
			},
		},

		{
			name: "#2",
			input: input{
				mockedAssetsTags: map[string][]string{
					"rancher/fleet": {"v1.0.0", "v2.0.0"},
				},
				mockFetchTags: func(ctx context.Context, repo, asset string) ([]string, error) {
					return []string{"v1.0.0", "v2.0.0", "v3.0.0", "some-crazy-tag"}, nil
				},
			},
			output: output{
				failedImages:         map[string][]string{},
				outOfNamespaceImages: []string{},
				err:                  nil,
			},
		},

		{
			name: "#3",
			input: input{
				mockedAssetsTags: map[string][]string{
					"rancher/fleet": {"v1.0.0", "v2.0.0"},
				},
				mockFetchTags: func(ctx context.Context, repo, asset string) ([]string, error) {
					return []string{"v1.0.0"}, nil
				},
			},
			output: output{
				failedImages: map[string][]string{
					"rancher/fleet": {"v2.0.0"},
				},
				outOfNamespaceImages: []string{},
				err:                  nil,
			},
		},

		{
			name: "#4",
			input: input{
				mockedAssetsTags: map[string][]string{
					"rancher/fleet": {"v1.0.0", "v2.0.0"},
				},
				mockFetchTags: func(ctx context.Context, repo, asset string) ([]string, error) {
					return []string{}, nil
				},
			},
			output: output{
				failedImages: map[string][]string{
					"rancher/fleet": {"no docker tags found for this image!"},
				},
				outOfNamespaceImages: []string{},
				err:                  nil,
			},
		},

		{
			name: "#5",
			input: input{
				mockedAssetsTags: map[string][]string{
					"pirate/fleet": {"v1.0.0", "v2.0.0"},
				},
				mockFetchTags: func(ctx context.Context, repo, asset string) ([]string, error) {
					return []string{}, nil
				},
			},
			output: output{
				failedImages:         map[string][]string{},
				outOfNamespaceImages: []string{"pirate/fleet"},
				err:                  nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// monkey patch
			fetchTagsFromRegistryRepo = tt.input.mockFetchTags

			failedImgs, outNsImgs, err := checkImagesFromDocker(ctx, tt.input.mockedAssetsTags)
			assertError(t, err, tt.output.err)
			require.Equal(t, tt.output.failedImages, failedImgs)
			require.Equal(t, tt.output.outOfNamespaceImages, outNsImgs)
		})
	}
}
