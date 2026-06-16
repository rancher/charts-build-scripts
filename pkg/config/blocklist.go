package config

import (
	"context"
	"slices"
)

type Blocklist struct {
	Charts map[string][]string
}

func LoadBlockList(ctx context.Context) (*Blocklist, error) {
	// TODO: implementation
	return &Blocklist{}, nil
}

func (b *Blocklist) IsBlocked(chart, version string) bool {
	versions, exists := b.Charts[chart]
	if !exists {
		return false
	}

	return slices.Contains(versions, version)
}
