package diff

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/sirupsen/logrus"
)

func checkDependencyPackage(pathToCmd string) error {
	cmd := exec.Command(pathToCmd, "--version")
	out, err := cmd.Output()
	if err != nil {
		return err
	}

	version := string(out)

	if strings.Contains(version, "Apple") || strings.Contains(version, "FreeBSD") {
		return errors.New("detected Apple/FreeBSD version of patch. This will lead to compatibility issues. Install GNU patch: https://github.com/rancher/charts-build-scripts/issues/130")
	}

	return nil
}

// GeneratePatch generates the patch between the files at srcPath and dstPath and outputs it to patchPath
// It returns whether the patch was generated or any errors that were encountered
func GeneratePatch(fs billy.Filesystem, patchPath, srcPath, dstPath string) (bool, error) {
	// TODO(aiyengar2): find a better library to actually generate and apply patches
	// There doesn't seem to be any existing library at the moment that can work with unified patches
	pathToDiffCmd, err := exec.LookPath("diff")
	if err != nil {
		return false, fmt.Errorf("cannot generate patch file if GNU diff is not available")
	}

	if err := checkDependencyPackage(pathToDiffCmd); err != nil {
		return false, err
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
			return false, fmt.Errorf("unable to generate patch with error: %s", err)
		}
	}

	if buf.Len() == 0 {
		return false, nil
	}
	// Patch exists
	patchFile, err := filesystem.CreateFileAndDirs(fs, patchPath)
	if err != nil {
		return false, err
	}
	defer patchFile.Close()
	if _, err = removeTimestamps(&buf).WriteTo(patchFile); err != nil {
		return false, fmt.Errorf("unable to write diff to file: %s", err)
	}
	return true, nil
}

// ApplyPatch applies a patch file located at patchPath to the destDir on the filesystem
func ApplyPatch(fs billy.Filesystem, patchPath, destDir string) error {
	// TODO(aiyengar2): find a better library to actually generate and apply patches
	// There doesn't seem to be any existing library at the moment that can work with unified patches
	pathToPatchCmd, err := exec.LookPath("patch")
	if err != nil {
		return fmt.Errorf("cannot generate patch file if GNU patch is not available")
	}

	if err := checkDependencyPackage(pathToPatchCmd); err != nil {
		return err
	}

	var buf bytes.Buffer
	patchFile, err := fs.Open(patchPath)
	if err != nil {
		return err
	}
	defer patchFile.Close()

	cmd := exec.Command(pathToPatchCmd, "-E", "-p1")
	cmd.Dir = filesystem.GetAbsPath(fs, destDir)
	cmd.Stdin = patchFile
	cmd.Stdout = &buf

	if err = cmd.Run(); err != nil {
		logrus.Errorf("\n%s", &buf)
		err = fmt.Errorf("unable to generate patch with error: %s", err)
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
