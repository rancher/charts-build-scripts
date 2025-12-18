package filesystem

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/logger"
)

// WalkAssetsFolderTgzFiles will walk and retrieve all .tgz files on the assets folder.
func WalkAssetsFolderTgzFiles(ctx context.Context) ([]string, error) {
	assetsTgzs := make([]string, 0)

	filepath.Walk("./assets/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Log(ctx, slog.LevelError, "error while walking over assets", logger.Err(err))
			return err
		}

		if strings.HasSuffix(info.Name(), ".tgz") {
			assetsTgzs = append(assetsTgzs, path)
		}

		return nil
	})

	return assetsTgzs, nil
}

// DecodeTgzValuesYamlMap will untar into memory the given .tgz file paths.
func DecodeTgzValuesYamlMap(ctx context.Context, assetsTgzs []string) (map[string][]map[string]interface{}, error) {
	valuesYamlMap := make(map[string][]map[string]interface{})

	for _, tgzPath := range assetsTgzs {
		values, err := DecodeValueYamlInTgz(ctx, tgzPath, []string{"values.yaml", "values.yml"})
		if err != nil {
			return nil, err
		}

		valuesYamlMap[tgzPath] = values
	}
	return valuesYamlMap, nil
}

// DecodeValueYamlInTgz will untar into-memory a given .tgz file and map it, normalizing the fields
// as strings enabling O(1) operations when searching for a target key which corresponds to a yaml field.
func DecodeValueYamlInTgz(ctx context.Context, tgzPath string, fileNames []string) ([]map[string]interface{}, error) {
	logger.Log(ctx, slog.LevelDebug, "untar/decode", slog.String("tgz", tgzPath))

	// open .tgz
	tgz, err := os.Open(tgzPath)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "open compressed file failure", slog.String("tgzPath", tgzPath))
		return nil, err
	}
	defer tgz.Close()

	// reader of compressed file
	gzr, err := gzip.NewReader(tgz)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "read compressed file failure", slog.String("tgzPath", tgzPath))
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var valuesSlice []map[string]interface{}
	for {
		header, err := tr.Next()
		switch {
		case err != nil && err != io.EOF:
			logger.Log(ctx, slog.LevelError, "untar failed", logger.Err(err))
			return nil, err

		// End of File
		case err == io.EOF:
			return valuesSlice, nil

		// valid values.(yaml || yml) file
		case header.Typeflag == tar.TypeReg && isTargetFile(header.Name, fileNames):
			var values *map[string]interface{}

			// Read the current tar entry's content into a buffer
			tarContent, err := io.ReadAll(tr)
			if err != nil {
				logger.Log(ctx, slog.LevelError, "tar buffer failure", logger.Err(err), slog.String("tgz", tgzPath))
				return valuesSlice, err
			}

			// Define the custom StreamReader for this buffered tar entry
			streamReader := func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(tarContent)), nil
			}

			// decode into values opened streamReader buffer
			if err := SafeDecodeYaml(ctx, streamReader, &values, true); err != nil {
				logger.Log(ctx, slog.LevelError, "yaml decode failure", logger.Err(err), slog.String("tgz", tgzPath))
				return nil, err
			}

			// There are empty files like CRD's
			if values != nil {
				normalizedValues := normalizeMapStructure(values).(map[string]interface{})
				valuesSlice = append(valuesSlice, normalizedValues)
			}

		default:
			continue
		}
	}
}

// isTargetFile checks if the current file is a values.(yaml||yml) or not
func isTargetFile(path string, fileNames []string) bool {
	basename := filepath.Base(path)

	for _, fileCompare := range fileNames {
		if basename == fileCompare {
			return true
		}
	}

	return false
}

// normalizeMapStructure receives any interface, checks for specific types and convert to map[string]interface{}.
// If it is a normal different type, it will try to convert it to a string.
func normalizeMapStructure(data interface{}) interface{} {
	if data == nil {
		return nil
	}

	switch value := data.(type) {
	// Already desired type, recurse on child values
	case *map[string]interface{}:
		strKeyMap := make(map[string]interface{})
		for k, v := range *value {
			strKeyMap[k] = normalizeMapStructure(v)
		}
		return strKeyMap

	// Already desired type, recurse on child values
	case map[string]interface{}:
		strKeyMap := make(map[string]interface{})
		for k, v := range value {
			strKeyMap[k] = normalizeMapStructure(v)
		}
		return strKeyMap

	// Some child values cannot have the proper map[string] inferred, convert them
	case map[interface{}]interface{}:
		strKeyMap := make(map[string]interface{})
		for k, v := range value {
			keyStr := fmt.Sprintf("%v", k)
			strKeyMap[keyStr] = normalizeMapStructure(v)
		}
		return strKeyMap

	// Process each element on the slice, it have can have child maps
	case []interface{}:
		processedSlice := make([]interface{}, len(value))
		for i, item := range value {
			processedSlice[i] = normalizeMapStructure(item)
		}
		return processedSlice

	// Normalize values
	case string:
		return value
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", value)
	case float32, float64:
		return fmt.Sprintf("%g", value) // %g compact representation for floats
	default:
		return fmt.Sprintf("%v", value)
	}
}
