package registries

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
)

// errUnsupportedPlatform indicates trivy could not find an image for the runner's platform (Linux).
var errUnsupportedPlatform = errors.New("image doesn't support the runner's platform")

// fallbackPlatform is retried when the image does not support runner's default platform
const fallbackPlatform = "windows/amd64"

// SeverityCounts holds the number of CVEs found for each severity level.
type SeverityCounts struct {
	Critical int64 `json:"critical"`
	High     int64 `json:"high"`
	Medium   int64 `json:"medium"`
	Low      int64 `json:"low"`
	Unknown  int64 `json:"unknown"`
}

// ImageCVEResult holds the CVE counts found for a single image:tag.
type ImageCVEResult struct {
	Repository string         `json:"repository"`
	Tag        string         `json:"tag"`
	CVECounts  SeverityCounts `json:"CVEs"`
	Skipped    bool           `json:"skipped,omitempty"`
	SkipReason string         `json:"skipReason,omitempty"`
}

// CVEReport is the top-level output of CheckChartCVEs.
type CVEReport struct {
	Chart           string           `json:"chart"`
	Version         string           `json:"version"`
	CVECounts       SeverityCounts   `json:"CVEs"`
	Images          []ImageCVEResult `json:"images"`
	PreviousVersion string           `json:"previousVersion,omitempty"`
	Delta           *SeverityCounts  `json:"delta,omitempty"`
}

// trivyReport represents a subset of Trivy's JSON output.
type trivyReport struct {
	Results []struct {
		Vulnerabilities []struct {
			Severity string `json:"Severity"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
}

// CheckChartCVEs scans every image used by chart/version with Trivy, and if a previous
// released version with the same major exists, scans it too and reports the CVE delta.
func CheckChartCVEs(ctx context.Context, repoRoot, chart, version string) (CVEReport, error) {
	report := CVEReport{Chart: chart, Version: version}

	counts, images, err := scanChartVersion(ctx, repoRoot, chart, version)
	if err != nil {
		return report, fmt.Errorf("scanning %s:%s: %w", chart, version, err)
	}
	report.CVECounts = counts
	report.Images = images

	previousVersion, found, err := findPreviousSameMajorVersion(ctx, repoRoot, chart, version)
	if err != nil {
		return report, fmt.Errorf("finding previous version: %w", err)
	}
	if !found {
		return report, nil
	}
	report.PreviousVersion = previousVersion

	previousCounts, _, err := scanChartVersion(ctx, repoRoot, chart, previousVersion)
	if err != nil {
		return report, fmt.Errorf("scanning %s:%s: %w", chart, previousVersion, err)
	}
	report.Delta = &SeverityCounts{
		Critical: counts.Critical - previousCounts.Critical,
		High:     counts.High - previousCounts.High,
		Medium:   counts.Medium - previousCounts.Medium,
		Low:      counts.Low - previousCounts.Low,
		Unknown:  counts.Unknown - previousCounts.Unknown,
	}

	return report, nil
}

// scanChartVersion collects every image used by chart/version and scans each one with trivy,
// returning the aggregated severity counts and the per-image breakdown.
func scanChartVersion(ctx context.Context, repoRoot, chart, version string) (SeverityCounts, []ImageCVEResult, error) {
	var total SeverityCounts
	var images []ImageCVEResult

	chartImages, err := collectChartImages(ctx, repoRoot, chart, version)
	if err != nil {
		return total, nil, fmt.Errorf("collecting chart images: %w", err)
	}

	for repository, tags := range chartImages {
		for _, tag := range tags {
			counts, skipped, err := scanImage(ctx, repository, tag)
			if err != nil {
				return total, nil, fmt.Errorf("scanning %s:%s: %w", repository, tag, err)
			}

			result := ImageCVEResult{Repository: repository, Tag: tag, CVECounts: counts}
			if skipped {
				result.Skipped = true
				result.SkipReason = fmt.Sprintf("image does not support the default platform or %s", fallbackPlatform)
				images = append(images, result)
				continue
			}

			images = append(images, result)
			total.Critical += counts.Critical
			total.High += counts.High
			total.Medium += counts.Medium
			total.Low += counts.Low
			total.Unknown += counts.Unknown
		}
	}

	return total, images, nil
}

// findPreviousSameMajorVersion returns the highest released version of chart that is lower
// than version and shares its major version, if any.
// Helm's LoadIndexFile sorts the index.yaml in descending order.
func findPreviousSameMajorVersion(ctx context.Context, repoRoot, chart, version string) (string, bool, error) {
	index, err := helm.OpenIndexYaml(ctx, filesystem.GetFilesystem(repoRoot))
	if err != nil {
		return "", false, fmt.Errorf("opening index.yaml: %w", err)
	}

	currentVer, err := semver.NewVersion(version)
	if err != nil {
		return "", false, nil
	}

	for _, chartVersion := range index.Entries[chart] {
		v, err := semver.NewVersion(chartVersion.Version)
		if err != nil || v.Prerelease() != "" || !v.LessThan(currentVer) {
			continue
		}
		if v.Major() != currentVer.Major() {
			return "", false, nil
		}
		return chartVersion.Version, true, nil
	}

	return "", false, nil
}

// scanImage runs trivy against repository:tag and returns its CVE counts by severity.
// If the image does not support the default platform, it retries against fallbackPlatform.
// If neither platform is supported, it returns skipped=true instead of an error.
var scanImage = func(ctx context.Context, repository, tag string) (counts SeverityCounts, skipped bool, err error) {
	output, err := runTrivy(ctx, repository, tag, "")
	if errors.Is(err, errUnsupportedPlatform) {
		output, err = runTrivy(ctx, repository, tag, fallbackPlatform)
		if errors.Is(err, errUnsupportedPlatform) {
			return SeverityCounts{}, true, nil
		}
	}
	if err != nil {
		return SeverityCounts{}, false, err
	}
	counts, err = parseTrivyReport(output)
	return counts, false, err
}

// trivyBinary is the trivy executable to invoke; overridden in tests.
var trivyBinary = "trivy"

// runTrivy shells out to the trivy CLI and returns its raw JSON report for repository:tag.
// If platform is non-empty, it is passed to trivy's --platform flag.
func runTrivy(ctx context.Context, repository, tag, platform string) ([]byte, error) {
	args := []string{"image", "--format", "json", "--quiet"}
	if platform != "" {
		args = append(args, "--platform", platform)
	}
	args = append(args, fmt.Sprintf("%s:%s", repository, tag))

	cmd := exec.CommandContext(ctx, trivyBinary, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(stderr.String(), "no child with platform") {
			return nil, errUnsupportedPlatform
		}
		return nil, fmt.Errorf("running trivy on %s:%s: %w: %s", repository, tag, err, strings.TrimSpace(stderr.String()))
	}

	return output, nil
}

// parseTrivyReport tallies CVE counts by severity from trivy's JSON output.
func parseTrivyReport(data []byte) (SeverityCounts, error) {
	var report trivyReport
	if err := json.Unmarshal(data, &report); err != nil {
		return SeverityCounts{}, fmt.Errorf("parsing trivy report: %w", err)
	}

	var counts SeverityCounts
	for _, result := range report.Results {
		for _, vuln := range result.Vulnerabilities {
			switch vuln.Severity {
			case "CRITICAL":
				counts.Critical++
			case "HIGH":
				counts.High++
			case "MEDIUM":
				counts.Medium++
			case "LOW":
				counts.Low++
			case "UNKNOWN":
				counts.Unknown++
			}
		}
	}

	return counts, nil
}
