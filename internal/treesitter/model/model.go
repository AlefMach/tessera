package model

type FileIndex struct {
	Path           string        `json:"path"`
	Language       string        `json:"language"`
	Symbols        []SymbolIndex `json:"symbols"`
	Imports        []string      `json:"imports"`
	Exports        []string      `json:"exports"`
	HasTestsNearby bool          `json:"has_tests_nearby"`
	Fallback       string        `json:"fallback,omitempty"`
}

type SymbolIndex struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}
