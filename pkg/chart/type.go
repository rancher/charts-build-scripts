package chart

// Type represents the type of chart that is being exported
type Type string

// BaseChartType is a type that describes the main chart
func BaseChartType() Type {
	return "chart"
}

// CRDChartType is a type that describes the CRD chart
func CRDChartType() Type {
	return "crd-chart"
}
