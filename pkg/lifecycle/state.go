package lifecycle

import (
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
)

// SaveState will save the lifecycle-status state to state.json file at charts repo
func (s *Status) SaveState() error {
	// Marshal the Project struct into JSON
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Write the JSON data to a file
	if err := os.WriteFile(s.StateFile, data, 0644); err != nil {
		return err
	}

	return nil
}

// LoadState will load the lifecycle-status state from an existing state.json file at charts repo
func LoadState(rootFs billy.Filesystem) (*Status, error) {
	// get the absolute path for the state.json file
	stateFilePath := filesystem.GetAbsPath(rootFs, path.RepositoryStateFile)
	s := &Status{
		StateFile: stateFilePath,
	}

	exist, err := s.checkStateFileExist()
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, err
	}

	// Read the file content
	file, _ := os.Open(s.StateFile)
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON data into the struct
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	return s, nil
}

// checkStateFileExist will check if the state.json file exists at the charts repo
func (s *Status) checkStateFileExist() (bool, error) {
	if _, err := os.Stat(s.StateFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// createStateFile will create a new state file at the charts repo
func (s *Status) createStateFile() error {
	stateFilePath := filesystem.GetAbsPath(s.ld.rootFs, path.RepositoryStateFile)

	if _, err := os.Create(stateFilePath); err != nil {
		return err
	}

	s.StateFile = stateFilePath
	return nil
}

// initState will create a new state file and save the state to it
func (s *Status) initState() error {
	if err := s.createStateFile(); err != nil {
		return err
	}

	if err := s.SaveState(); err != nil {
		return err
	}

	return nil
}
