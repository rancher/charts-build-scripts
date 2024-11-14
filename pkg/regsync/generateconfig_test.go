package regsync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_SlsaYaml(t *testing.T) {

	type input struct {
		imageTagMap      map[string][]string
		readSlsaYamlMock func() ([]string, error)
	}

	type expected struct {
		err         error
		imageTagMap map[string][]string
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
					"image1": {"tag1"},
					"image2": {"tag2"},
					"image3": {"tag3"},
				},
				readSlsaYamlMock: func() ([]string, error) {
					return []string{"image1"}, nil
				},
			},
			expected: expected{
				err: nil,
				imageTagMap: map[string][]string{
					"image2": {"tag2"},
					"image3": {"tag3"},
				},
			},
		},
		{
			name: "#2",
			input: input{
				imageTagMap: map[string][]string{
					"image1": {"tag1", "tag2"},
					"image2": {"tag2"},
					"image3": {"tag3", "tag4"},
				},
				readSlsaYamlMock: func() ([]string, error) {
					return []string{"image1", "image2"}, nil
				},
			},
			expected: expected{
				err: nil,
				imageTagMap: map[string][]string{
					"image3": {"tag3", "tag4"},
				},
			},
		},
		{
			name: "#3",
			input: input{
				imageTagMap: map[string][]string{
					"image1": {"tag1", "tag2"},
				},
				readSlsaYamlMock: func() ([]string, error) {
					return []string{"image1", "image2"}, nil
				},
			},
			expected: expected{
				err:         nil,
				imageTagMap: map[string][]string{},
			},
		},
		{
			name: "#4",
			input: input{
				imageTagMap: map[string][]string{},
				readSlsaYamlMock: func() ([]string, error) {
					return []string{"image1", "image2"}, nil
				},
			},
			expected: expected{
				err:         nil,
				imageTagMap: map[string][]string{},
			},
		},
		{
			name: "#5",
			input: input{
				imageTagMap: map[string][]string{},
				readSlsaYamlMock: func() ([]string, error) {
					return []string{}, nil
				},
			},
			expected: expected{
				err:         nil,
				imageTagMap: map[string][]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := removeSlsaImages(tt.input.imageTagMap, tt.input.readSlsaYamlMock)
			require.NoError(t, err)
			require.Equal(t, tt.expected.imageTagMap, tt.input.imageTagMap)

		})
	}
}
