package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"

	"github.com/go-git/go-billy/v5"
	"github.com/sirupsen/logrus"
)

// GeneratePatch generates the patch between the files at srcPath and dstPath and outputs it to patchPath
// It returns whether the patch was generated or any errors that were encountered
func GeneratePatch(fs billy.Filesystem, patchPath, srcPath, dstPath string) (bool, error) {
	// TODO(aiyengar2): find a better library to actually generate and apply patches
	// There doesn't seem to be any existing library at the moment that can work with unified patches
	pathToDiffCmd, err := exec.LookPath("diff")
	if err != nil {
		return false, fmt.Errorf("Cannot generate patch file if GNU diff is not available")
	}

	var buf bytes.Buffer
	cmd := exec.Command(pathToDiffCmd, "-uN", "-x *.tgz", "-x *.lock", srcPath, dstPath)
	cmd.Dir = fs.Root()
	cmd.Stdout = &buf

	if err = cmd.Run(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		// Exit code of 1 indicates that a difference was observed, so it is expected
		if !ok || exitErr.ExitCode() != 1 {
			logrus.Errorf("\n%s", &buf)
			return false, fmt.Errorf("Unable to generate patch with error: %s", err)
		}
	}

	if buf.Len() == 0 {
		return false, nil
	}
	// Patch exists
	patchFile, err := CreateFileAndDirs(fs, patchPath)
	if err != nil {
		return false, err
	}
	defer patchFile.Close()
	if _, err = removeTimestamps(&buf).WriteTo(patchFile); err != nil {
		return false, fmt.Errorf("Unable to write diff to file: %s", err)
	}
	return true, nil
}

// ApplyPatch applies a patch file located at patchPath to the destDir on the filesystem
func ApplyPatch(fs billy.Filesystem, patchPath, destDir string) error {
	// TODO(aiyengar2): find a better library to actually generate and apply patches
	// There doesn't seem to be any existing library at the moment that can work with unified patches
	pathToPatchCmd, err := exec.LookPath("patch")
	if err != nil {
		return fmt.Errorf("Cannot generate patch file if GNU patch is not available")
	}

	var buf bytes.Buffer
	patchFile, err := fs.Open(patchPath)
	if err != nil {
		return err
	}
	defer patchFile.Close()

	cmd := exec.Command(pathToPatchCmd, "-E", "-p1")
	cmd.Dir = GetAbsPath(fs, destDir)
	cmd.Stdin = patchFile
	cmd.Stdout = &buf

	if err = cmd.Run(); err != nil {
		logrus.Errorf("\n%s", &buf)
		err = fmt.Errorf("Unable to generate patch with error: %s", err)
	}
	return err
}

// removeTimestamps removes timestamps from a given patch file
func removeTimestamps(in *bytes.Buffer) *bytes.Buffer {
	var out []byte
	var timestampPath string
	s := bufio.NewScanner(in)
	for s.Scan() {
		line := s.Text()
		if n, err := fmt.Sscanf(line, "--- %s", &timestampPath); n == 1 && err == nil {
			line = fmt.Sprintf("--- %s", timestampPath)
		}
		if n, err := fmt.Sscanf(line, "+++ %s", &timestampPath); n == 1 && err == nil {
			line = fmt.Sprintf("+++ %s", timestampPath)
		}
		out = append(out, (line + "\n")...)
	}
	return bytes.NewBuffer(out)
}
