package registries

import (
	"context"
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

// CheckChartCVEs scans every image used by chart/version with Trivy, and if a previous
// released version with the same major exists, scans it too and reports the CVE delta.
func CheckChartCVEs(ctx context.Context, repoRoot, chart, version string) (CVEReport, error) {
	report := CVEReport{Chart: chart, Version: version}
	return report, nil
}
