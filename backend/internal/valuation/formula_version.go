package valuation

const (
	FormulaVersionLegacy = "hours-ratio-v1"
	FormulaVersionV2     = "hours-reciprocal-v2"
)

func normalizeFormulaVersion(version string) string {
	if version == FormulaVersionV2 {
		return FormulaVersionV2
	}
	return FormulaVersionLegacy
}

func timeDecayExponent(kHours, kBrand float64, formulaVersion string) float64 {
	if kHours <= 0 || kBrand <= 0 {
		return 1
	}
	if normalizeFormulaVersion(formulaVersion) == FormulaVersionV2 {
		return 1 / (kHours * kBrand)
	}
	return kHours / kBrand
}
