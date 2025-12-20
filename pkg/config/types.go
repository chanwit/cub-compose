package config

// ComposeConfig represents the root structure of configs.yaml
type ComposeConfig struct {
	Project      string            `yaml:"project,omitempty"`       // project name, adds Project label to all entities
	SpacePrefix  string            `yaml:"space-prefix,omitempty"`  // prefix for all space names
	CommonLabels map[string]string `yaml:"common-labels,omitempty"` // labels for all entities (spaces and units)
	Configs      []RepoConfig      `yaml:"configs"`
}

// RepoConfig represents a Git repository with its spaces
type RepoConfig struct {
	Repo       string            `yaml:"repo"`
	Ref        string            `yaml:"ref,omitempty"`         // branch or tag
	UnitLabels map[string]string `yaml:"unit-labels,omitempty"` // labels for all units in this repo
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

// ResolvedSpace contains the resolved data for a space
type ResolvedSpace struct {
	Name   string            // full space name (with prefix applied)
	Labels map[string]string // merged labels (project + common)
}

// ResolvedUnit contains the resolved data for a unit
type ResolvedUnit struct {
	RepoURL   string
	SpaceName string            // full space name (with prefix applied)
	UnitName  string
	Dir       string
	Cmd       string
	Labels    map[string]string // merged labels (project + common + repo + unit)
	Content   []byte            // resolved config content after cmd execution or file read
}
