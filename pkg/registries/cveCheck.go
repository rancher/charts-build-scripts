package registries

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

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
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	CVECounts  SeverityCounts
}

// CVEReport is the top-level output of CheckChartCVEs.
type CVEReport struct {
	Chart           string `json:"chart"`
	Version         string `json:"version"`
	CVECounts       SeverityCounts
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

	return report, nil
}

// scanChartVersion collects every image used by chart/version and scans each one with trivy,
// returning the aggregated severity counts and the per-image breakdown.
func scanChartVersion(ctx context.Context, repoRoot, chart, version string) (SeverityCounts, []ImageCVEResult, error) {
	var total SeverityCounts
	images := []ImageCVEResult{}

	chartImages, err := collectChartImages(ctx, repoRoot, chart, version)
	if err != nil {
		return total, nil, fmt.Errorf("collecting chart images: %w", err)
	}

	for repository, tags := range chartImages {
		for _, tag := range tags {
			counts, err := scanImage(ctx, repository, tag)
			if err != nil {
				return total, nil, fmt.Errorf("scanning %s:%s: %w", repository, tag, err)
			}
			images = append(images, ImageCVEResult{Repository: repository, Tag: tag, CVECounts: counts})
			total.Critical += counts.Critical
			total.High += counts.High
			total.Medium += counts.Medium
			total.Low += counts.Low
			total.Unknown += counts.Unknown
		}
	}

	return total, images, nil
}

// scanImage runs trivy against repository:tag and returns its CVE counts by severity.
var scanImage = func(ctx context.Context, repository, tag string) (SeverityCounts, error) {
	output, err := runTrivy(ctx, repository, tag)
	if err != nil {
		return SeverityCounts{}, err
	}
	return parseTrivyReport(output)
}

// runTrivy shells out to the trivy CLI and returns its raw JSON report for repository:tag.
func runTrivy(ctx context.Context, repository, tag string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "trivy", "image", "--format", "json", "--quiet", fmt.Sprintf("%s:%s", repository, tag))

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running trivy on %s:%s: %w", repository, tag, err)
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
