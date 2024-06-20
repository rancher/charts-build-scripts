package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type logs struct {
	file     *os.File
	filePath string
}

// SEPARATOR is a standard separator for logs
const SEPARATOR = "\n"

// ENDER is a standard end for the logged information
const ENDER = "\n__________________________________________________________________\n"

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

// writeHEAD writes the header of the log file with the version rules, dates and necessary information
// to help analyze the current situation on the charts versions regarding the release process.
func (l *logs) writeHEAD(versionRules *VersionRules, title string) {
	l.write(title, "INFO")
	currentTime := time.Now()
	l.write(currentTime.Format("2006-01-02 15:04:05"), "INFO")
	l.write(fmt.Sprintf("Branch Version: %.1f", versionRules.branchVersion), "INFO")
	l.write(fmt.Sprintf("minimal version: %d", versionRules.minVersion), "INFO")
	l.write(fmt.Sprintf("max version: %d", versionRules.maxVersion), "INFO")
	l.write(fmt.Sprintf("development branch: %s", versionRules.devBranch), "INFO")
	l.write(fmt.Sprintf("production branch: %s", versionRules.prodBranch), "INFO")

	rules := make(map[string]string)
	for k, v := range versionRules.rules {
		rules[fmt.Sprintf("%.1f", k)] = fmt.Sprintf("min: %s, max: %s", v.min, v.max)
	}

	rulesJSON, err := json.MarshalIndent(rules, "", "    ")
	if err != nil {
		logrus.Errorf("JSON marshaling failed: %s", err)
		l.write(fmt.Sprintf("rules: %v\n", versionRules.rules), "INFO")
	} else {
		l.write(fmt.Sprintf("rules: %s\n", rulesJSON), "INFO")
	}
}

// write writes the data to the log file and prints it to the console with customizations.
func (l *logs) write(data string, logType string) {
	switch logType {
	case "INFO":
		logrus.Info(data)
		if _, err := l.file.WriteString("INFO=" + data + "\n"); err != nil {
			logrus.Errorf("Error while writing logs: %s", err)
		}
	case "WARN":
		logrus.Warn(data)
		if _, err := l.file.WriteString("WARN=" + data + "\n"); err != nil {
			logrus.Errorf("Error while writing logs: %s", err)
		}
	case "ERROR":
		logrus.Error(data)
		if _, err := l.file.WriteString("ERROR=" + data + "\n"); err != nil {
			logrus.Errorf("Error while writing logs: %s", err)
		}
	case "SEPARATE":
		fmt.Printf(SEPARATOR)
		if _, err := l.file.WriteString(SEPARATOR); err != nil {
			logrus.Errorf("Error while writing logs: %s", err)
		}
	case "END":
		fmt.Printf(ENDER)
		if _, err := l.file.WriteString("\n" + ENDER + "\n"); err != nil {
			logrus.Errorf("Error while writing logs: %s", err)
		}
	default:
		fmt.Printf(data)
		if _, err := l.file.WriteString(data + "\n"); err != nil {
			logrus.Errorf("Error while writing logs: %s", err)
		}
	}
}
