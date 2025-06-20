package filesystem

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/logger"
	yamlV2 "gopkg.in/yaml.v2"
	yamlV3 "gopkg.in/yaml.v3"
)

// safeDecodeYaml will attempt to decode a yaml file in-memory.
// The 1st attempt will always be using yaml.v3, that has stricter format rules.
// If 'ignoreFormat' is true, it can try a 2nd attempt using yaml.v2.
// see 'decodeErrorsToIgnore' function below for a list of legacy errors allowed to be skipped.
func safeDecodeYaml(ctx context.Context, reader StreamReader, data interface{}, ignoreFormat bool) error {
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
