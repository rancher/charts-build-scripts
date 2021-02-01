package helm

import (
	"strings"
)

// ExportAnnotation is an annotation that is modified on an export
type ExportAnnotation interface {
	dropRCVersion(val string) (newValWithoutRC string, updated bool)
}

// AutoInstallAnnotation corresponds to catalog.cattle.io/auto-install
type AutoInstallAnnotation struct{}

func (a *AutoInstallAnnotation) dropRCVersion(val string) (newValWithoutRC string, updated bool) {
	if strings.HasSuffix(val, "=match") {
		return val, false
	}
	newValWithoutRC = strings.SplitN(val, "-rc", 2)[0]
	return newValWithoutRC, val != newValWithoutRC
}

var (
	exportAnnotations = map[string]ExportAnnotation{
		"catalog.cattle.io/auto-install": &AutoInstallAnnotation{},
	}
)
