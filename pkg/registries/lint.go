package registries

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	pkggit "github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/logger"
)

const rancherNamespace = "rancher/"

// LintWarning represents a violation of image block rules in a chart's values.yaml.
type LintWarning struct {
	Asset      string // path to the .tgz file
	YAMLPath   string // dot-notation path to the offending node, e.g. "httpHeaderInjectorWebhook.proxy.image"
	Repository string // the repository value found
	Tag        string // the tag value found (empty string if missing/null)
	Reason     string // "orphan_repository" | "wrong_namespace"
}

// LintImageTags enforces three rules on every image block found in the
// values.yaml of each PR-changed .tgz asset:
//
//  1. repository present + tag missing/empty/null → block (orphan_repository)
//  2. repository present + tag valid + repository not rancher/* → block (wrong_namespace)
//  3. repository present + tag valid + repository is rancher/* → pass
func LintImageTags(ctx context.Context, baseBranch, tgzOverride string) ([]LintWarning, error) {
	logger.Log(ctx, slog.LevelInfo, "starting Lint Image Tags")
	logger.Log(ctx, slog.LevelInfo, "baseBranch", slog.String("baseBranch", baseBranch))
	logger.Log(ctx, slog.LevelInfo, "tgzOverride", slog.String("tgzOverride", tgzOverride))

	var tgzPaths []string

	if tgzOverride != "" {
		// Direct path provided — bypass git diff (useful for local testing)
		logger.Log(ctx, slog.LevelInfo, "linting single tgz directly", slog.String("tgz", tgzOverride))
		tgzPaths = []string{tgzOverride}
	} else {
		if baseBranch == "" {
			return nil, errors.New("lint-images: --base-branch or BASE_BRANCH required")
		}
		logger.Log(ctx, slog.LevelInfo, "linting image tags", slog.String("baseBranch", baseBranch))

		changedFiles, err := pkggit.GetChangedFiles(ctx, ".", baseBranch)
		if err != nil {
			return nil, err
		}
		tgzPaths, err = parseChangedTgzFiles(ctx, changedFiles)
		if err != nil {
			return nil, err
		}

		if len(tgzPaths) == 0 {
			logger.Log(ctx, slog.LevelInfo, "no changed .tgz files in this PR, skipping lint")
			return nil, nil
		}
	}

	var warnings []LintWarning
	for _, tgzPath := range tgzPaths {
		w, err := lintTgz(ctx, tgzPath)
		if err != nil {
			return nil, err
		}
		warnings = append(warnings, w...)
	}

	logger.Log(ctx, slog.LevelInfo, "lint complete", slog.Int("warnings", len(warnings)))
	return warnings, nil
}

// parseChangedTgzFiles filters a set of git changes down to assets/**/*.tgz
// files that were added or modified (deletions are ignored).
func parseChangedTgzFiles(ctx context.Context, changes object.Changes) ([]string, error) {
	var tgzFiles []string
	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return nil, fmt.Errorf("failed to read change action: %w", err)
		}
		if action == merkletrie.Delete {
			continue
		}
		name := change.To.Name
		if strings.HasPrefix(name, "assets/") && strings.HasSuffix(name, ".tgz") {
			logger.Log(ctx, slog.LevelDebug, "changed tgz", slog.String("path", name), slog.String("action", action.String()))
			tgzFiles = append(tgzFiles, name)
		}
	}
	logger.Log(ctx, slog.LevelInfo, "changed tgz files", slog.Int("count", len(tgzFiles)))
	return tgzFiles, nil
}

// lintTgz processes a single .tgz asset: decodes values.yaml and runs
// traverseViolations to collect all rule violations.
func lintTgz(ctx context.Context, tgzPath string) ([]LintWarning, error) {
	logger.Log(ctx, slog.LevelDebug, "linting", slog.String("tgz", tgzPath))

	valuesSlice, err := filesystem.DecodeValueYamlInTgz(ctx, tgzPath, []string{"values.yaml", "values.yml"})
	if err != nil {
		return nil, err
	}

	violations := make(map[string]LintWarning) // yamlPath → violation
	for _, values := range valuesSlice {
		traverseViolations(values, "", tgzPath, violations)
	}

	if len(violations) == 0 {
		return nil, nil
	}

	warnings := make([]LintWarning, 0, len(violations))
	for _, v := range violations {
		logger.Log(ctx, slog.LevelDebug, "violation found",
			slog.String("tgz", tgzPath),
			slog.String("path", v.YAMLPath),
			slog.String("repository", v.Repository),
			slog.String("reason", v.Reason),
		)
		warnings = append(warnings, v)
	}

	return warnings, nil
}

// traverseViolations walks a decoded values.yaml tree and collects every image
// block that violates one of the three lint rules.
//
// Rules (applied in order, traversal stops at the first match per node):
//   - repository present + tag missing/empty/null → orphan_repository
//   - repository present + tag valid + not rancher/* → wrong_namespace
//   - repository present + tag valid + rancher/* → pass, stop
//   - no repository → keep traversing children
func traverseViolations(data interface{}, path, asset string, violations map[string]LintWarning) {
	switch value := data.(type) {
	case map[string]interface{}:
		repo, repoExists := value["repository"]
		tag, tagExists := value["tag"]

		repoStr, repoIsStr := repo.(string)
		tagStr, tagIsStr := tag.(string)

		hasRepo := repoExists && repoIsStr && repoStr != ""
		hasTag := tagExists && tagIsStr && tagStr != ""

		if hasRepo && !hasTag {
			violations[path] = LintWarning{
				Asset:      asset,
				YAMLPath:   path,
				Repository: repoStr,
				Tag:        tagStr,
				Reason:     "orphan_repository",
			}
			return
		}

		if hasRepo && hasTag {
			if !strings.HasPrefix(repoStr, rancherNamespace) {
				violations[path] = LintWarning{
					Asset:      asset,
					YAMLPath:   path,
					Repository: repoStr,
					Tag:        tagStr,
					Reason:     "wrong_namespace",
				}
			}
			// Either valid or wrong_namespace — stop traversing this node
			return
		}

		// No repository at this level — keep traversing children
		for k, v := range value {
			childPath := k
			if path != "" {
				childPath = path + "." + k
			}
			traverseViolations(v, childPath, asset, violations)
		}

	case []interface{}:
		for i, v := range value {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			traverseViolations(v, childPath, asset, violations)
		}
	}
}
