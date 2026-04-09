package canary_test

import (
	"path/filepath"
	"runtime"
)

// fixturePath returns the absolute path to a file in testdata/ relative to
// this source file, so tests work regardless of where go test is invoked from.
func fixturePath(name string) string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "testdata", name)
}
