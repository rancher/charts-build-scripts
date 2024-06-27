package git

import (
	"strings"
)

// CheckForValidForkRemote checks if the remote URL is a valid remote fork for the upstream URL.
func CheckForValidForkRemote(upstreamURL, remoteURL, repo string) bool {
	urlPart1, urlPart2 := extractCommonParts(upstreamURL, remoteURL)
	if urlPart1 == "https://github.com" && urlPart2 == repo {
		return true
	}

	return false
}

// extractCommonParts takes two Git URLs and returns the common prefix and suffix.
func extractCommonParts(url1, url2 string) (string, string) {
	// Split the URLs into segments
	segments1 := strings.Split(url1, "/")
	segments2 := strings.Split(url2, "/")

	// Find the common prefix
	prefixLength := 0
	for i := 0; i < len(segments1) && i < len(segments2); i++ {
		if segments1[i] == segments2[i] {
			prefixLength++
		} else {
			break
		}
	}

	// Find the common suffix
	suffixLength := 0
	for i := 0; i < len(segments1)-prefixLength && i < len(segments2)-prefixLength; i++ {
		if segments1[len(segments1)-1-i] == segments2[len(segments2)-1-i] {
			suffixLength++
		} else {
			break
		}
	}

	// Reconstruct the common prefix and suffix
	commonPrefix := strings.Join(segments1[:prefixLength], "/")
	commonSuffix := strings.Join(segments1[len(segments1)-suffixLength:], "/")

	return commonPrefix, commonSuffix
}
