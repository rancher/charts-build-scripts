package regsync

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	diffExample1 = `diff --git a/regsync.yaml b/regsync.yaml
index c584669a3..ab15c1bc5 100644
--- a/regsync.yaml
+++ b/regsync.yaml
@@ -76,6 +76,26 @@ sync:
     - v5.0.1
     - v5.0.2
     - v6.0.0
+- source: docker.io/rancher/cis-operator
+  target: '{{ env "REGISTRY_ENDPOINT" }}/rancher/cis-operator'
+  type: repository
+  tags:
+    allow:
+    - v1.1.0
+    - v1.2.0
+    - v1.3.0
 - source: docker.io/rancher/eks-operator
   target: '{{ env "REGISTRY_ENDPOINT" }}/rancher/eks-operator'
   type: repository
`

	diffExample2 = `diff --git a/regsync.yaml b/regsync.yaml
index c584669a3..ee5f53416 100644
--- a/regsync.yaml
+++ b/regsync.yaml
@@ -76,6 +76,28 @@ sync:
     - v5.0.1
     - v5.0.2
     - v6.0.0
+- source: docker.io/rancher/cis-operator
+  target: '{{ env "REGISTRY_ENDPOINT" }}/rancher/cis-operator'
+  type: repository
+  tags:
+    allow:
+    - v1.1.0
+    - v1.2.0
+    - v1.3.0
+    - v1.3.1
+    - v1.3.3
 - source: docker.io/rancher/eks-operator
   target: '{{ env "REGISTRY_ENDPOINT" }}/rancher/eks-operator'
   type: repository
@@ -1426,6 +1448,7 @@ sync:
     - v0.4.0
     - v0.5.0
     - v0.5.1
+    - v0.5.2
 - source: docker.io/rancher/shell
   target: '{{ env "REGISTRY_ENDPOINT" }}/rancher/shell'
   type: repository
`

	badDiffExample = `@@ -76,6 +76,28 @@ sync:
     - v5.0.1
     - v5.0.2
     - v6.0.0
+- source: docker.io/rancher/cis-operator
+  target: '{{ env "REGISTRY_ENDPOINT" }}/rancher/cis-operator'
+  type: repository
+  tags:
+    allow:
+    - v1.1.0
+    - v1.2.0
+    - v1.3.0
+    - v1.3.1
+    - v1.3.3
 - source: docker.io/rancher/eks-operator
   target: '{{ env "REGISTRY_ENDPOINT" }}/rancher/eks-operator'
   type: repository
@@ -1426,6 +1448,7 @@ sync:
     - v0.4.0
     - v0.5.0
     - v0.5.1
+    - v0.5.2
 - source: docker.io/rancher/shell
   target: '{{ env "REGISTRY_ENDPOINT" }}/rancher/shell'
   type: repository
`
)

func assertError(t *testing.T, err, expectedErr error) {
	if expectedErr != nil {
		assert.EqualError(t, err, expectedErr.Error())
	} else {
		assert.NoError(t, err)
	}
}

func Test_parseGitDiff(t *testing.T) {
	type input struct {
		diff string
	}
	type expected struct {
		result []insertedDiffs
		err    error
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
				diff: diffExample1,
			},
			expected: expected{
				result: []insertedDiffs{
					insertedDiffs{
						insertions: []insertion{
							insertion{
								tag:  "v1.1.0",
								line: 84,
							},
							insertion{
								tag:  "v1.2.0",
								line: 85,
							},
							insertion{
								tag:  "v1.3.0",
								line: 86,
							},
						},
					},
				},
				err: nil,
			},
		},
		{
			name: "#2",
			input: input{
				diff: diffExample2,
			},
			expected: expected{
				result: []insertedDiffs{
					insertedDiffs{
						insertions: []insertion{
							insertion{
								tag:  "v1.1.0",
								line: 84,
							},
							insertion{
								tag:  "v1.2.0",
								line: 85,
							},
							insertion{
								tag:  "v1.3.0",
								line: 86,
							},
							insertion{
								tag:  "v1.3.1",
								line: 87,
							},
							insertion{
								tag:  "v1.3.3",
								line: 88,
							},
						},
					},
					insertedDiffs{
						insertions: []insertion{
							insertion{
								tag:  "v0.5.2",
								line: 1451,
							},
						},
					},
				},
				err: nil,
			},
		},
		{
			name: "#3",
			input: input{
				diff: badDiffExample,
			},
			expected: expected{
				result: nil,
				err:    errors.New("line 25, char 581: expected file header while reading extended headers, got EOF"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insertionsToParse, err := parseGitDiff(tt.input.diff)
			require.Equal(t, tt.expected.result, insertionsToParse)
			assertError(t, err, tt.expected.err)
		})
	}

}

func Test_parseTag(t *testing.T) {
	type input struct {
		line string
	}
	type expected struct {
		result string
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
				line: "- v1.0.0",
			},
			expected: expected{
				result: "v1.0.0",
			},
		},
		{
			name: "#2",
			input: input{
				line: "- v1.0.1",
			},
			expected: expected{
				result: "v1.0.1",
			},
		},
		{
			name: "#3",
			input: input{
				line: "      - v1.1.0",
			},
			expected: expected{
				result: "v1.1.0",
			},
		},
		{
			name: "#4",
			input: input{
				line: "      - 1.1.0",
			},
			expected: expected{
				result: "1.1.0",
			},
		},
		{
			name: "#5",
			input: input{
				line: "      - 1.1.0-rc1.0.0",
			},
			expected: expected{
				result: "1.1.0-rc1.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTag(tt.input.line)
			require.Equal(t, tt.expected.result, result)
		})
	}
}

func Test_parseSourceImage(t *testing.T) {
	type input struct {
		insertedDiffs []insertedDiffs
		parsedRegsync map[int]string
	}
	type expected struct {
		result map[string][]string
		err    error
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
				insertedDiffs: []insertedDiffs{
					{
						insertions: []insertion{
							{
								tag:  "1.0.0",
								line: 100,
							},
							{
								tag:  "1.0.1",
								line: 101,
							},
							{
								tag:  "1.1.0",
								line: 102,
							},
						},
					},
				},
				parsedRegsync: map[int]string{
					95:  "- source: docker.io/rancher/cis-operator",
					96:  "  target: '{{ env REGISTRY_ENDPOINT }}/rancher/cis-operator'",
					97:  "  type: repository",
					98:  "  tags:",
					99:  "     allow:",
					100: "   - 1.0.0",
					101: "   - 1.0.1",
					102: "   - 1.1.0",
				},
			},
			expected: expected{
				result: map[string][]string{
					"rancher/cis-operator": []string{"1.0.0", "1.0.1", "1.1.0"},
				},
				err: nil,
			},
		},
		{
			name: "#2",
			input: input{
				insertedDiffs: []insertedDiffs{
					{
						insertions: []insertion{
							{
								tag:  "1.0.0",
								line: 100,
							},
							{
								tag:  "1.0.1",
								line: 101,
							},
							{
								tag:  "1.1.0",
								line: 102,
							},
						},
					},
					{
						insertions: []insertion{
							{
								tag:  "0.0.1",
								line: 100000000,
							},
						},
					},
				},
				parsedRegsync: map[int]string{
					95:  "- source: docker.io/rancher/cis-operator",
					96:  "  target: '{{ env REGISTRY_ENDPOINT }}/rancher/cis-operator'",
					97:  "  type: repository",
					98:  "  tags:",
					99:  "     allow:",
					100: "   - 1.0.0",
					101: "   - 1.0.1",
					102: "   - 1.1.0",

					99999995:  "- source: docker.io/rancher/security-scan",
					99999996:  "  target: '{{ env REGISTRY_ENDPOINT }}/rancher/sec-scan'",
					99999997:  "  type: repository",
					99999998:  "  tags:",
					99999999:  "     allow:",
					100000000: "   - 0.0.1",
				},
			},
			expected: expected{
				result: map[string][]string{
					"rancher/cis-operator":  []string{"1.0.0", "1.0.1", "1.1.0"},
					"rancher/security-scan": []string{"0.0.1"},
				},
				err: nil,
			},
		},
		{
			name: "#3",
			input: input{
				insertedDiffs: []insertedDiffs{
					{
						insertions: []insertion{
							{
								tag:  "1.0.0",
								line: 4,
							},
						},
					},
				},
				parsedRegsync: map[int]string{
					1: "  type: repository",
					2: "  tags:",
					3: "     allow:",
					4: "   - 1.0.0",
				},
			},
			expected: expected{
				result: map[string][]string{},
				err:    fmt.Errorf("source image not found around line %d", 4),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSourceImage(tt.input.insertedDiffs, tt.input.parsedRegsync)
			require.Equal(t, tt.expected.result, result)
			assertError(t, err, tt.expected.err)
		})
	}
}

func Test_removeCosignedImages(t *testing.T) {
	type input struct {
		imageTagMap    map[string][]string
		cosignedImages map[string][]string
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
				cosignedImages: map[string][]string{
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
				cosignedImages: map[string][]string{
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
				cosignedImages: map[string][]string{
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
				cosignedImages: map[string][]string{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			removeCosignedImages(tt.input.imageTagMap, tt.input.cosignedImages)
			require.Equal(t, tt.expected.result, tt.input.imageTagMap)
		})
	}
}
