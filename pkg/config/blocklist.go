package config

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"

	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"go.yaml.in/yaml/v3"
)

type Blocklist struct {
	Charts map[string][]string
}

var (
	cachedBlocklist *Blocklist
	blocklistOnce   sync.Once
	blocklistErr    error

	// EnforceBlocklist makes LoadBlockList return an error instead of an empty blocklist.
	// It must be set before the first LoadBlockList call, the result is cached.
	EnforceBlocklist bool
)

func LoadBlockList(ctx context.Context) (*Blocklist, error) {
	blocklistOnce.Do(func() {
		cachedBlocklist, blocklistErr = fetchBlocklistFromRemote(ctx)
		if blocklistErr == nil && EnforceBlocklist && len(cachedBlocklist.Charts) == 0 {
			cachedBlocklist, blocklistErr = nil, errors.New("blocklist is enforced but it is empty")
		}
	})
	return cachedBlocklist, blocklistErr
}

func fetchBlocklistFromRemote(ctx context.Context) (*Blocklist, error) {
	// Open git repo
	repo, err := git.OpenGitRepo(ctx, ".")
	if err != nil {
		return nil, errors.New("load blocklist open git repo: " + err.Error())
	}

	// Fetch latest automation-core branch
	if err := repo.FetchRemoteTrackingBranch(path.AutoCoreBranch); err != nil {
		// If this repo doesn't have a rancher/charts upstream remote, return empty blocklist
		if errors.Is(err, git.ErrNoUpstreamRemote) {
			logger.Log(ctx, slog.LevelWarn, "blocklist unavailable in non-rancher/charts repo, using empty blocklist",
				slog.String("branch", path.AutoCoreBranch))
			return &Blocklist{Charts: make(map[string][]string)}, nil
		}
		return nil, errors.New("load blocklist fetch branch: " + err.Error())
	}

	// Fetch blocklist.yaml from automation-core branch
	data, err := repo.ShowFileFromRemoteBranch(ctx, path.AutoCoreBranch, path.BlockList)
	if err != nil {
		return nil, errors.New("load blocklist show: " + err.Error())
	}

	// Parse YAML directly into map
	var charts map[string][]string
	if err := yaml.Unmarshal(data, &charts); err != nil {
		return nil, errors.New("load blocklist unmarshal: " + err.Error())
	}

	return &Blocklist{Charts: charts}, nil
}

func (b *Blocklist) IsBlocked(chart, version string) bool {
	versions, exists := b.Charts[chart]
	if !exists {
		return false
	}

	return slices.Contains(versions, version)
}
