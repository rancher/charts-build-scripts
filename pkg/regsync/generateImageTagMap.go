package regsync

// GenerateImageTagMap returns a map of container images and their tags
func GenerateImageTagMap() (map[string][]string, error) {
	imageTagMap := make(map[string][]string)

	err := walkAssetsFolder(imageTagMap)
	if err != nil {
		return imageTagMap, err
	}

	return imageTagMap, nil
}
