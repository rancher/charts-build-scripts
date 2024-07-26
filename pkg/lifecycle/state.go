package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/sirupsen/logrus"
)

// SaveState will save the lifecycle-status state to state.json file at charts repo
func (s *Status) SaveState() error {
	logrus.Info("saving app state to state.json file")

	// Marshal the Project struct into JSON
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		err = fmt.Errorf("error marshalling project to JSON: %w", err)
		logrus.Error(err)
		return err
	}

	// Write the JSON data to a file
	err = os.WriteFile(s.StateFile, data, 0644)
	if err != nil {
		err = fmt.Errorf("error writing JSON to file: %w", err)
		logrus.Error(err)
		return err
	}

	logrus.Info("saved state file successfully")
	return nil
}

// LoadState will load the lifecycle-status state from an existing state.json file at charts repo
func LoadState(rootFs billy.Filesystem) (*Status, error) {
	logrus.Info("loading previous state from state.json file")

	// get the absolute path for the state.json file
	stateFilePath := filesystem.GetAbsPath(rootFs, path.RepositoryStateFile)
	s := &Status{
		StateFile: stateFilePath,
	}

	exist, err := s.checkStateFileExist()
	if err != nil {
		err = fmt.Errorf("failed to load state file: %w", err)
		logrus.Error(err)
		return nil, err
	}
	if !exist {
		err = fmt.Errorf("state file does not exist")
		logrus.Error(err)
		return nil, err
	}

	file, _ := os.Open(s.StateFile)
	// Read the file content
	data, err := io.ReadAll(file)
	if err != nil {
		err = fmt.Errorf("Error reading file: %w", err)
		logrus.Error(err)
		return nil, err
	}

	// Unmarshal the JSON data into the struct
	err = json.Unmarshal(data, &s)
	if err != nil {
		err = fmt.Errorf("Error unmarshalling JSON: %w", err)
		logrus.Error(err)
		return nil, err
	}

	logrus.Info("loaded state.json successfully")
	return s, nil
}

// checkStateFileExist will check if the state.json file exists at the charts repo
func (s *Status) checkStateFileExist() (bool, error) {
	logrus.Info("checking if state.json file exists")

	_, err := os.Stat(s.StateFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logrus.Info("state.json file does not exist")
			return false, nil
		}

		err = fmt.Errorf("failed to check if state.json exists: %w", err)
		logrus.Error(err)
		return false, err
	}

	logrus.Info("state.json exists")
	return true, nil
}

// createStateFile will create a new state file at the charts repo
func (s *Status) createStateFile() error {
	stateFilePath := filesystem.GetAbsPath(s.ld.rootFs, path.RepositoryStateFile)

	_, err := os.Create(stateFilePath)
	if err != nil {
		err = fmt.Errorf("failed to create state file: %w", err)
		logrus.Error(err)
		return err
	}

	s.StateFile = stateFilePath
	return nil
}

// initState will create a new state file and save the state to it
func (s *Status) initState() error {

	err := s.createStateFile()
	if err != nil {
		err = fmt.Errorf("failed to init state file: %w", err)
		logrus.Error(err)
		return err
	}

	err = s.SaveState()
	if err != nil {
		err = fmt.Errorf("failed to init state file: %w", err)
		logrus.Error(err)
		return err
	}

	return nil
}
