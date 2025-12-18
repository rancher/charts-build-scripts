package filesystem

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/logger"
	yamlV2 "gopkg.in/yaml.v2"
	yamlV3 "gopkg.in/yaml.v3"
)

// StreamReader func() type is callback function for custom filesystem file loading behavior
type StreamReader func() (io.ReadCloser, error)

// LoadYamlFile is a generic function that loads a YAML file and decodes it
// efficiently (especially for large files) using a streaming approach
// into a struct of type YamlFields, specified by the caller.
// YamlFields is expected to be a struct suitable for YAML unmarshalling.
//   - filepath: entire file path to the .yaml file
//   - ignoreFormat: ignore legacy yaml format errors
func LoadYamlFile[YamlFields any](ctx context.Context, filepath string, ignoreFormat bool) (*YamlFields, error) {
	/*
		See the example of a valid YamlFields struct for regsync.yaml

		type Config struct {
			Version  int         `yaml:"version"`
			Creds    interface{} `yaml:"creds"`
			Defaults interface{} `yaml:"defaults"`
		}
	*/

	reader := func() (io.ReadCloser, error) {
		return os.Open(filepath)
	}
	var yamlFields *YamlFields

	logger.Log(ctx, slog.LevelDebug, "decoding", slog.String("filepath", filepath))

	if err := SafeDecodeYaml(ctx, reader, &yamlFields, false); err != nil {
		return nil, err
	}

	return yamlFields, nil
}

// SafeDecodeYaml will attempt to decode a yaml file in-memory.
// The 1st attempt will always be using yaml.v3, that has stricter format rules.
// If 'ignoreFormat' is true, it can try a 2nd attempt using yaml.v2.
// see 'decodeErrorsToIgnore' function below for a list of legacy errors allowed to be skipped.
func SafeDecodeYaml(ctx context.Context, reader StreamReader, data interface{}, ignoreFormat bool) error {
	// stream1 is the first attempt of reading the file with yaml.v3 package
	stream1, err := reader()
	if err != nil {
		logger.Log(ctx, slog.LevelError, "stream1 failed", logger.Err(err))
		return err
	}

	var stream1Err error
	func() {
		defer stream1.Close()
		decoder1 := yamlV3.NewDecoder(stream1)
		stream1Err = decoder1.Decode(data)
	}()

	// different error handling to avoid nested if/else/swith
	if stream1Err == nil || errors.Is(stream1Err, io.EOF) {
		return nil
	}

	if !ignoreFormat {
		logger.Log(ctx, slog.LevelError, "safe decode failed", logger.Err(stream1Err))
		return stream1Err
	}
	logger.Log(ctx, slog.LevelWarn, "unsafe decode in progress")

	// Check if this is a legacy yaml error to ignore
	if ignoreError := decodeErrorsToIgnore(stream1Err); !ignoreError {
		logger.Log(ctx, slog.LevelError, "safe decode exception", logger.Err(stream1Err))
		return stream1Err
	}

	// We are ignoring the previous error, Execute fallback with yaml.v2
	stream2, err := reader()
	if err != nil {
		logger.Log(ctx, slog.LevelError, "stream2 failed", logger.Err(err))
		return err
	}

	var stream2Err error
	func() {
		defer stream2.Close()
		decoder2 := yamlV2.NewDecoder(stream2)
		stream2Err = decoder2.Decode(data)
	}()

	if stream2Err != nil && stream2Err != io.EOF {
		logger.Log(ctx, slog.LevelError, "unsafe decode failed", logger.Err(stream2Err))
		return stream2Err
	}

	return nil
}

// decodeErrorsToIgnore will check for yaml.v3 errors format errors that can be skipped.
// errors like 'mapping key already defined' can be skipped for old charts which were already released;
// This kind of error should not be skipped for newer charts.
func decodeErrorsToIgnore(err error) (ignore bool) {
	ignore = false
	var yamlV3TypeError *yamlV3.TypeError

	if errors.As(err, &yamlV3TypeError) {
		for _, errMsg := range yamlV3TypeError.Errors {
			if strings.Contains(errMsg, "mapping key") && strings.Contains(errMsg, "already defined") {
				ignore = true
				return
			}
		}
	}

	return ignore
}

// CreateAndOpenYamlFile creates a new yaml file or opens an existing one.
// The behavior is controlled by the truncate flag.
//
// Parameters:
//   - ctx: The context for logging.
//   - filePath: The full path to the yaml file.
//   - truncate: If true, the file is created or truncated.
//     If false, it's opened for read/write, or created if it doesn't exist.
func CreateAndOpenYamlFile(ctx context.Context, filePath string, truncate bool) (*os.File, error) {
	flags := os.O_RDWR | os.O_CREATE

	if truncate {
		logger.Log(ctx, slog.LevelWarn, "truncating", slog.String("file", filePath))
		flags |= os.O_TRUNC
	}

	const permissions = 0644

	file, err := os.OpenFile(filePath, flags, permissions)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "", slog.Group("open failure", filePath, flags, logger.Err(err)))
		return nil, err
	}

	return file, nil
}

// UpdateYamlFile encodes any data structure to a YAML file
func UpdateYamlFile(file *os.File, data any) error {
	encoder := yamlV3.NewEncoder(file)
	encoder.SetIndent(2)
	return encoder.Encode(data)
}
