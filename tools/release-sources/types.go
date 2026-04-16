package main

// SourceData holds the result of fetching one data source.
type SourceData struct {
	Name      string
	Available bool
	Repos     map[string]bool // set of repo names present in this source
	Detail    string          // display string, e.g. "releng/pyxis-repo-configs @ main"
}

// Sources holds all five data sources plus per-repo metadata.
type Sources struct {
	Pyxis     SourceData
	OBD       SourceData
	ImageRefs SourceData
	Stage     SourceData
	Prod      SourceData

	// OBD per-repo metadata: repo name -> delivery_repo_names list
	OBDDelivery map[string][]string

	// image-references per-repo metadata: repo name -> from image URL
	ImgRefFrom map[string]string
}

// Issue represents a problem found during comparison.
type Issue struct {
	Severity  string // "error" or "warning"
	Repo      string
	Message   string
	InSources string // which sources have it (for context)
}
