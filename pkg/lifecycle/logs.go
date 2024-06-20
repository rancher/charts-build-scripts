package lifecycle

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

type logs struct {
	file     *os.File
	filePath string
}

// createLogs creates a new log file and returns a logs struct with the file and file path
func createLogs(fileName string) (*logs, error) {
	filePath := fmt.Sprintf("logs/%s", fileName)

	// Create the logs directory if it doesn't exist
	err := os.MkdirAll("logs", 0755)
	if err != nil {
		logrus.Errorf("Error while creating logs directory: %s", err)
	}

	// Open the file in append mode
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &logs{file: file, filePath: filePath}, nil
}
