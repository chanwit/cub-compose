package config

// ComposeConfig represents the root structure of configs.yaml
type ComposeConfig struct {
	Configs []RepoConfig `yaml:"configs"`
}

// RepoConfig represents a Git repository with its spaces
type RepoConfig struct {
	Repo       string            `yaml:"repo"`
	Ref        string            `yaml:"ref,omitempty"`        // branch or tag
	UnitLabels map[string]string `yaml:"unitLabels,omitempty"` // labels for all units in this repo
	Spaces     map[string]*Space `yaml:"spaces"`
}

// Space represents a ConfigHub space containing units
type Space struct {
	Units map[string]*Unit `yaml:"units"`
}

// Unit represents a config unit with its source definition
type Unit struct {
	Dir    string            `yaml:"dir"`              // directory relative to repo root
	Cmd    string            `yaml:"cmd,omitempty"`    // command to execute (e.g., "kubectl kustomize .")
	Files  []string          `yaml:"files,omitempty"`  // files to read (alternative to cmd)
	Labels map[string]string `yaml:"labels,omitempty"` // labels for this unit
}

// ResolvedUnit contains the resolved data for a unit
type ResolvedUnit struct {
	RepoURL   string
	SpaceName string
	UnitName  string
	Dir       string
	Cmd       string
	Labels    map[string]string // merged labels (repo + unit)
	Content   []byte            // resolved config content after cmd execution or file read
}
