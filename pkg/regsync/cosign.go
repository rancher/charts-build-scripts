package regsync

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/path"
	slsactl "github.com/rancherlabs/slsactl/pkg/verify"

	"github.com/sourcegraph/go-diff/diff"
)

type insertedDiffs struct {
	sourceImage string
	insertions  []insertion
}

type insertion struct {
	tag  string
	line int32
}

// checkNewImageTags will get the git diff for regsync.yaml and parse the new available image tags.
func checkNewImageTags() (map[string][]string, error) {
	rawDiff, err := getRawDiff(path.RegsyncYamlFile)
	if err != nil {
		return nil, err
	}

	insertedDiffsToParse, err := parseGitDiff(rawDiff)
	if err != nil {
		return nil, err
	}

	parsedRegsync, err := mapFileLineByValue(path.RegsyncYamlFile)
	if err != nil {
		return nil, err
	}

	return parseSourceImage(insertedDiffsToParse, parsedRegsync)
}

// getRawDiff returns the raw git diff output for a specific file
func getRawDiff(filePath string) (string, error) {
	// Run the git diff command for the specific file
	cmd := exec.Command("git", "diff", filePath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute the command
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to execute git diff: %v, %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func parseGitDiff(rawDiff string) ([]insertedDiffs, error) {
	// Parse the raw diff into structured format
	file, err := diff.ParseFileDiff([]byte(rawDiff))
	if err != nil {
		return nil, err
	}

	idfs := make([]insertedDiffs, len(file.Hunks))

	// Iterate through file diffs
	for i, hunk := range file.Hunks {
		currentLine := hunk.NewStartLine // Starting line in the new file
		idfs[i] = insertedDiffs{
			insertions: make([]insertion, 0),
		}

		// Split the hunk body into lines
		lines := bytes.Split(hunk.Body, []byte("\n"))
		for _, line := range lines {
			if len(line) > 0 && line[0] == '+' {
				if bytes.Contains(line, []byte("+++")) ||
					bytes.Contains(line, []byte("source")) ||
					bytes.Contains(line, []byte("target")) ||
					bytes.Contains(line, []byte("type")) ||
					bytes.Contains(line, []byte("tags")) ||
					bytes.Contains(line, []byte("allow")) ||
					bytes.Contains(line, []byte("sync")) ||
					bytes.Contains(line, []byte("default")) ||
					bytes.Contains(line, []byte("media")) {
					currentLine++
					continue
				}
				// Output the added line and its calculated line number
				idfs[i].insertions = append(idfs[i].insertions, insertion{
					tag:  parseTag(string(line[1:])),
					line: currentLine,
				})
				currentLine++
				continue
			}
			// Skip any line that was not inserted
			currentLine++
		}
	}
	return idfs, nil
}

// parseTag will parse the tag from the raw line value.
func parseTag(tag string) string {
	parsedTag := strings.TrimSpace(tag)
	parsedTag = strings.TrimPrefix(parsedTag, "- ")
	return parsedTag
}

// mapFileLineByValue reads a file and maps line numbers to their content.
func mapFileLineByValue(filePath string) (map[int]string, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Map to store line numbers and their content
	lineToContent := make(map[int]string)

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	lineNumber := 1
	for scanner.Scan() {
		lineToContent[lineNumber] = scanner.Text()
		lineNumber++
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	return lineToContent, nil
}

// parseSourceImage will parse the source image and its tags from the regsync.yaml.
func parseSourceImage(insertedDiffs []insertedDiffs, parsedRegsync map[int]string) (map[string][]string, error) {

	var currentSourceImage string
	var imageTags map[string][]string = make(map[string][]string)

	for _, idf := range insertedDiffs {
		for i, insertion := range idf.insertions {
			// iterate only the very first time to get the source image
			if i == 0 {
				sourceImage, err := findSourceImage(parsedRegsync, int(insertion.line))
				if err != nil {
					return imageTags, err
				}
				parsedSource := strings.TrimPrefix(sourceImage, "- source: docker.io/")
				currentSourceImage = parsedSource
				imageTags[parsedSource] = []string{}
			}
			imageTags[currentSourceImage] = append(imageTags[currentSourceImage], insertion.tag)
		}
	}

	return imageTags, nil
}

// findSourceImage will find the source image from the regsync.yaml above the line number passed.
func findSourceImage(parsedRegsync map[int]string, maxLine int) (string, error) {
	for i := maxLine; i > 0; i-- {
		lineTxt := parsedRegsync[i]
		if strings.Contains(lineTxt, "source") {
			return parsedRegsync[i], nil
		}
	}
	return "", fmt.Errorf("source image not found around line %d", maxLine)
}

// checkCosignedImages checks if the images are cosigned or not.
func checkCosignedImages(imageTags map[string][]string) (map[string][]string, error) {
	// prepare the signed-images.txt file for later manual inspection
	if err := prepareSignedImagesFile(); err != nil {
		return nil, err
	}

	baseImageRepository := "registry.suse.com/"
	cosignedImages := make(map[string][]string)

	for image, tags := range imageTags {
		for _, tag := range tags {
			imageTag := baseImageRepository + image + ":" + tag
			fmt.Printf("checking: %s\n", imageTag)
			// Close the stdout and stderr so there are no leaks
			oldStdout, oldStderr, r, w := closeStdOut()
			// Verify the image, if it's cosigned
			if err := slsactl.Verify(imageTag); err != nil {
				reopenStdOut(oldStdout, oldStderr, r, w)
				continue
			}
			reopenStdOut(oldStdout, oldStderr, r, w)

			// add it to the cosignedImages map
			cosignedImages[image] = append(cosignedImages[image], tag)

			// log and write the cosigned images to a file
			fmt.Printf("cosigned - %s\n", imageTag)
			if err := writeCosignedImages(imageTag); err != nil {
				return nil, err
			}
		}
	}

	return cosignedImages, nil
}

// prepareSignedImagesFile checks if signed-images.txt exists.
// If it exists, it erases all its content. If it does not exist, it creates it.
func prepareSignedImagesFile() error {
	// Check if the file exists
	_, err := os.Stat(path.SignedImagesFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Create the file if it does not exist
			file, err := os.Create(path.SignedImagesFile)
			if err != nil {
				return err
			}

			return file.Close()
		}
		// some other error occurred while checking the file
		return err
	}

	return os.Truncate(path.SignedImagesFile, 0)
}

// writeCosignedImages writes the imageTag to a text file called signed-images.txt.
func writeCosignedImages(imageTag string) error {
	// Open the file in append mode, create it if it doesn't exist
	file, err := os.OpenFile(path.SignedImagesFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the imageTag to the file
	if _, err := file.WriteString(imageTag + "\n"); err != nil {
		return err
	}

	return nil
}

// closeStdOut closes the stdout and stderr and returns the old ones.
func closeStdOut() (*os.File, *os.File, *os.File, *os.File) {
	// Save the current stdout and stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	// Create a pipe to discard the output
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w
	return oldStdout, oldStderr, r, w
}

// reopenStdOut reopens the stdout and stderr and reads the discarded output.
func reopenStdOut(oldStdout, oldStderr, r, w *os.File) {
	w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	io.ReadAll(r) // Read the discarded output to prevent pipe errors
}

// removeCosignedImages removes the cosigned images from the imageTagMap.
func removeCosignedImages(imageTagMap, cosignedImages map[string][]string) {
	for cosignedImage, cosignedTags := range cosignedImages {
		if _, exist := imageTagMap[cosignedImage]; exist {
			currentTags := imageTagMap[cosignedImage]
			notCosignedTags := excludeCosignedTags(currentTags, cosignedTags)
			imageTagMap[cosignedImage] = notCosignedTags
		}
	}
}

// excludeCosignedTags removes the cosigned tags from the tags slice.
func excludeCosignedTags(tags, cosignedTags []string) []string {
	var isCosigned bool = false
	var notCosignedTags []string
	for _, tag := range tags {
		for _, cTag := range cosignedTags {
			if tag == cTag {
				isCosigned = true
				break
			}
		}

		if !isCosigned {
			notCosignedTags = append(notCosignedTags, tag)
		} else {
			isCosigned = false
			continue
		}
	}
	return notCosignedTags
}
