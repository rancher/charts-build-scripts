package regsync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_removePrimeImageTags(t *testing.T) {
	type input struct {
		imageTagMap    map[string][]string
		primeImageTags map[string][]string
	}
	type expected struct {
		result map[string][]string
	}
	type test struct {
		name     string
		input    input
		expected expected
	}

	tests := []test{
		{
			name: "#1",
			input: input{
				imageTagMap: map[string][]string{
					"rancher/cis-operator":  {"v1.0.0", "v1.0.1", "v1.1.0"},
					"rancher/security-scan": {"v1.0.0", "v1.0.1", "v1.1.0"},
					"rancher/eks-operator":  {"v1.0.0", "v1.0.1", "v1.1.0"},
					"rancher/shell":         {"v1.0.0", "v1.0.1", "v1.1.0"},
				},
				primeImageTags: map[string][]string{
					"rancher/cis-operator":  {"v1.0.1", "v1.1.0"},
					"rancher/security-scan": {"v1.1.0"},
				},
			},
			expected: expected{
				result: map[string][]string{
					"rancher/cis-operator":  {"v1.0.0"},
					"rancher/security-scan": {"v1.0.0", "v1.0.1"},
					"rancher/eks-operator":  {"v1.0.0", "v1.0.1", "v1.1.0"},
					"rancher/shell":         {"v1.0.0", "v1.0.1", "v1.1.0"},
				},
			},
		},
		{
			name: "#2",
			input: input{
				imageTagMap: map[string][]string{
					"rancher/cis-operator":  {"v1.0.0", "v1.0.1", "v1.1.0"},
					"rancher/security-scan": {"v1.0.0", "v1.0.1", "v1.1.0"},
					"rancher/eks-operator":  {"v1.0.0", "v1.0.1", "v1.1.0"},
					"rancher/shell":         {"v1.0.0", "v1.0.1", "v1.1.0"},
				},
				primeImageTags: map[string][]string{
					"rancher/cis-operator":  {"v1.1.0"},
					"rancher/security-scan": {"v1.1.0"},
				},
			},
			expected: expected{
				result: map[string][]string{
					"rancher/cis-operator":  {"v1.0.0", "v1.0.1"},
					"rancher/security-scan": {"v1.0.0", "v1.0.1"},
					"rancher/eks-operator":  {"v1.0.0", "v1.0.1", "v1.1.0"},
					"rancher/shell":         {"v1.0.0", "v1.0.1", "v1.1.0"},
				},
			},
		},
		{
			name: "#3",
			input: input{
				imageTagMap: map[string][]string{
					"rancher/cis-operator":  {"v1.0.0"},
					"rancher/security-scan": {"v1.0.0"},
					"rancher/eks-operator":  {"v1.0.0", "v1.0.1", "v1.1.0"},
					"rancher/shell":         {"v1.0.0", "v1.0.1", "v1.1.0"},
				},
				primeImageTags: map[string][]string{
					"rancher/security-scan": {},
				},
			},
			expected: expected{
				result: map[string][]string{
					"rancher/cis-operator":  {"v1.0.0"},
					"rancher/security-scan": {"v1.0.0"},
					"rancher/eks-operator":  {"v1.0.0", "v1.0.1", "v1.1.0"},
					"rancher/shell":         {"v1.0.0", "v1.0.1", "v1.1.0"},
				},
			},
		},
		{
			name: "#4",
			input: input{
				imageTagMap: map[string][]string{
					"rancher/cis-operator":  {"v1.0.0"},
					"rancher/security-scan": {"v1.0.0"},
					"rancher/eks-operator":  {"v1.0.0", "v1.0.1", "v1.1.0"},
					"rancher/shell":         {"v1.0.0", "v1.0.1", "v1.1.0"},
				},
				primeImageTags: map[string][]string{
					"rancher/security-scan": {"2.0.0"},
				},
			},
			expected: expected{
				result: map[string][]string{
					"rancher/cis-operator":  {"v1.0.0"},
					"rancher/security-scan": {"v1.0.0"},
					"rancher/eks-operator":  {"v1.0.0", "v1.0.1", "v1.1.0"},
					"rancher/shell":         {"v1.0.0", "v1.0.1", "v1.1.0"},
				},
			},
		},
		// edge cases
		{
			name: "#5",
			input: input{
				imageTagMap: map[string][]string{
					"rancher/cis-operator": {"v1.0.0"},
				},
				primeImageTags: map[string][]string{
					"rancher/cis-operator": {"v1.0.0"},
				},
			},
			expected: expected{
				result: map[string][]string{"rancher/cis-operator": {}},
			},
		},
		{
			name: "#6",
			input: input{
				imageTagMap: map[string][]string{
					"rancher/cis-operator": {},
				},
				primeImageTags: map[string][]string{
					"rancher/cis-operator": {"v1.0.0"},
				},
			},
			expected: expected{
				result: map[string][]string{"rancher/cis-operator": {}},
			},
		},
		{
			name: "#7",
			input: input{
				imageTagMap: map[string][]string{
					"rancher/cis-operator": {"v1.0.0"},
				},
				primeImageTags: map[string][]string{
					"rancher/cis-operator": {},
				},
			},
			expected: expected{
				result: map[string][]string{"rancher/cis-operator": {"v1.0.0"}},
			},
		},
		{
			name: "#8",
			input: input{
				imageTagMap: map[string][]string{
					"rancher/cis-operator": {},
				},
				primeImageTags: map[string][]string{
					"rancher/cis-operator": {},
				},
			},
			expected: expected{
				result: map[string][]string{"rancher/cis-operator": {}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notPrimeImageTags := removePrimeImageTags(tt.input.imageTagMap, tt.input.primeImageTags)
			require.Equal(t, tt.expected.result, notPrimeImageTags)
		})
	}
}
