package git

import (
	"strings"
)

// CheckForValidForkRemote checks if the remote URL is a valid remote fork for the upstream URL.
func CheckForValidForkRemote(upstreamURL, remoteURL, repo string) bool {
	if remoteURL == "" {
		return false // Remote URL is empty
	}
	urlPart1, urlPart2 := extractCommonParts(upstreamURL, remoteURL)
	return urlPart1 == "https://github.com" && urlPart2 == repo
}

// extractCommonParts takes two Git URLs and returns the common prefix and suffix.
func extractCommonParts(url1, url2 string) (string, string) {
	// Split the URLs into segments
	segments1 := strings.Split(url1, "/")
	segments2 := strings.Split(url2, "/")

	// Find the common prefix
	prefixLength := 0
	for i := 0; i < len(segments1) && i < len(segments2); i++ {
		if segments1[i] != segments2[i] {
			break
		}
		prefixLength++
	}

	// Find the common suffix
	suffixLength := 0
	for i := 0; i < len(segments1)-prefixLength && i < len(segments2)-prefixLength; i++ {
		if segments1[len(segments1)-1-i] != segments2[len(segments2)-1-i] {
			break
		}
		suffixLength++
	}

	// Reconstruct the common prefix and suffix
	commonPrefix := strings.Join(segments1[:prefixLength], "/")
	commonSuffix := strings.Join(segments1[len(segments1)-suffixLength:], "/")

	return commonPrefix, commonSuffix
}
