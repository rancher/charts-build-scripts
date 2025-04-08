package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/rancher/charts-build-scripts/pkg/logger"
)

// Logs is a struct that holds the file and file path of the log file
type Logs struct {
	File     *os.File
	FilePath string
}

const separator = "\n"
const ender = "\n__________________________________________________________________\n"

// CreateLogs creates a new log file and returns a logs struct with the file and file path
func CreateLogs(ctx context.Context, fileName, detail string) (*Logs, error) {
	// get a timestamp
	currentTime := time.Now()
	now := currentTime.Format("2006-01-02T15:04")
	filePath := fmt.Sprintf("logs/%s_%s_%s", now, detail, fileName)

	// Create the logs directory if it doesn't exist
	err := os.MkdirAll("logs", 0755)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "failed to create logs directory", logger.Err(err))
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
func (l *Logs) WriteHEAD(ctx context.Context, versionRules *VersionRules, title string) {
	l.Write(ctx, title, "INFO")
	currentTime := time.Now()
	l.Write(ctx, currentTime.Format("2006-01-02 15:04:05"), "INFO")
	l.Write(ctx, fmt.Sprintf("Branch Version: %s", versionRules.BranchVersion), "INFO")
	l.Write(ctx, fmt.Sprintf("minimal version: %d", versionRules.MinVersion), "INFO")
	l.Write(ctx, fmt.Sprintf("max version: %d", versionRules.MaxVersion), "INFO")
	l.Write(ctx, fmt.Sprintf("development branch: %s", versionRules.DevBranch), "INFO")
	l.Write(ctx, fmt.Sprintf("production branch: %s", versionRules.ProdBranch), "INFO")

	rules := make(map[string]string, len(versionRules.Rules))
	for k, v := range versionRules.Rules {
		rules[fmt.Sprintf("%s", k)] = fmt.Sprintf("min: %s, max: %s", v.Min, v.Max)
	}

	rulesJSON, err := json.MarshalIndent(rules, "", "    ")
	if err != nil {
		logger.Log(ctx, slog.LevelError, "failed to marshal rules to JSON", logger.Err(err))
		l.Write(ctx, fmt.Sprintf("rules: %v\n", versionRules.Rules), "INFO")
	} else {
		l.Write(ctx, fmt.Sprintf("rules: %s\n", rulesJSON), "INFO")
	}
}

// Write writes the data to the log file and prints it to the console with customizations.
func (l *Logs) Write(ctx context.Context, data string, logType string) {
	switch logType {
	case "INFO":
		if _, err := l.File.WriteString("INFO=" + data + "\n"); err != nil {
			logger.Log(ctx, slog.LevelError, "failed to write logs", logger.Err(err))
		}
	case "WARN":
		if _, err := l.File.WriteString("WARN=" + data + "\n"); err != nil {
			logger.Log(ctx, slog.LevelError, "failed to write logs", logger.Err(err))
		}
	case "ERROR":
		if _, err := l.File.WriteString("ERROR=" + data + "\n"); err != nil {
			logger.Log(ctx, slog.LevelError, "failed to write logs", logger.Err(err))
		}
	case "SEPARATE":
		if _, err := l.File.WriteString(separator); err != nil {
			logger.Log(ctx, slog.LevelError, "failed to write logs", logger.Err(err))
		}
	case "END":
		if _, err := l.File.WriteString("\n" + ender + "\n"); err != nil {
			logger.Log(ctx, slog.LevelError, "failed to write logs", logger.Err(err))
		}
	default:
		if _, err := l.File.WriteString(data + "\n"); err != nil {
			logger.Log(ctx, slog.LevelError, "failed to write logs", logger.Err(err))
		}
	}
}

// WriteVersions receives the loaded assets versions map and writes it to the log file
// in human-readable format
func (l *Logs) WriteVersions(ctx context.Context, assetsVersions map[string][]Asset, logType string) {
	for asset, versions := range assetsVersions {
		l.Write(ctx, "", "SEPARATE")
		l.Write(ctx, asset, logType)
		for _, version := range versions {
			l.Write(ctx, version.Version, "")
		}
	}
}
