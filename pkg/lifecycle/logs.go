package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// Logs is a struct that holds the file and file path of the log file
type Logs struct {
	File     *os.File
	FilePath string
}

const separator = "\n"
const ender = "\n__________________________________________________________________\n"

// CreateLogs creates a new log file and returns a logs struct with the file and file path
func CreateLogs(fileName, detail string) (*Logs, error) {
	// get a timestamp
	currentTime := time.Now()
	now := currentTime.Format("2006-01-02T15:04")
	filePath := fmt.Sprintf("logs/%s_%s_%s", now, detail, fileName)

	// Create the logs directory if it doesn't exist
	err := os.MkdirAll("logs", 0755)
	if err != nil {
		logrus.Errorf("Error while creating logs directory: %s", err)
		return nil, err
	}

	// Open the file in append mode
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &Logs{File: file, FilePath: filePath}, nil
}

// WriteHEAD writes the header of the log file with the version rules, dates and necessary information
// to help analyze the current situation on the charts versions regarding the release process.
func (l *Logs) WriteHEAD(versionRules *VersionRules, title string) {
	l.Write(title, "INFO")
	currentTime := time.Now()
	l.Write(currentTime.Format("2006-01-02 15:04:05"), "INFO")
	l.Write(fmt.Sprintf("Branch Version: %.1f", versionRules.BranchVersion), "INFO")
	l.Write(fmt.Sprintf("minimal version: %d", versionRules.MinVersion), "INFO")
	l.Write(fmt.Sprintf("max version: %d", versionRules.MaxVersion), "INFO")
	l.Write(fmt.Sprintf("development branch: %s", versionRules.DevBranch), "INFO")
	l.Write(fmt.Sprintf("production branch: %s", versionRules.ProdBranch), "INFO")

	rules := make(map[string]string, len(versionRules.Rules))
	for k, v := range versionRules.Rules {
		rules[fmt.Sprintf("%.1f", k)] = fmt.Sprintf("min: %s, max: %s", v.min, v.max)
	}

	rulesJSON, err := json.MarshalIndent(rules, "", "    ")
	if err != nil {
		logrus.Errorf("JSON marshaling failed: %s", err)
		l.Write(fmt.Sprintf("rules: %v\n", versionRules.Rules), "INFO")
	} else {
		l.Write(fmt.Sprintf("rules: %s\n", rulesJSON), "INFO")
	}
}

// Write writes the data to the log file and prints it to the console with customizations.
func (l *Logs) Write(data string, logType string) {
	switch logType {
	case "INFO":
		logrus.Info(data)
		if _, err := l.File.WriteString("INFO=" + data + "\n"); err != nil {
			logrus.Errorf("Error while writing logs: %s", err)
		}
	case "WARN":
		logrus.Warn(data)
		if _, err := l.File.WriteString("WARN=" + data + "\n"); err != nil {
			logrus.Errorf("Error while writing logs: %s", err)
		}
	case "ERROR":
		logrus.Error(data)
		if _, err := l.File.WriteString("ERROR=" + data + "\n"); err != nil {
			logrus.Errorf("Error while writing logs: %s", err)
		}
	case "SEPARATE":
		fmt.Printf(separator)
		if _, err := l.File.WriteString(separator); err != nil {
			logrus.Errorf("Error while writing logs: %s", err)
		}
	case "END":
		fmt.Printf(ender)
		if _, err := l.File.WriteString("\n" + ender + "\n"); err != nil {
			logrus.Errorf("Error while writing logs: %s", err)
		}
	default:
		fmt.Println(data)
		if _, err := l.File.WriteString(data + "\n"); err != nil {
			logrus.Errorf("Error while writing logs: %s", err)
		}
	}
}

// WriteVersions receives the loaded assets versions map and writes it to the log file
// in human-readable format
func (l *Logs) WriteVersions(assetsVersions map[string][]Asset, logType string) {
	for asset, versions := range assetsVersions {
		l.Write("", "SEPARATE")
		l.Write(asset, logType)
		for _, version := range versions {
			l.Write(version.Version, "")
		}
	}
}
