package compose

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/confighub/cub-compose/pkg/config"
)

// LoadConfig loads and parses a configs.yaml file
func LoadConfig(path string) (*config.ComposeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg config.ComposeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// validateConfig validates the config structure
func validateConfig(cfg *config.ComposeConfig) error {
	if len(cfg.Configs) == 0 {
		return fmt.Errorf("no configs defined")
	}

	for i, repo := range cfg.Configs {
		if repo.Repo == "" {
			return fmt.Errorf("config[%d]: repo is required", i)
		}

		if len(repo.Spaces) == 0 {
			return fmt.Errorf("config[%d]: no spaces defined for repo %s", i, repo.Repo)
		}

		for spaceName, space := range repo.Spaces {
			// Allow empty spaces (no units) - they will be skipped during sync
			if space == nil || len(space.Units) == 0 {
				continue
			}

			for unitName, unit := range space.Units {
				if unit.Dir == "" {
					return fmt.Errorf("config[%d]: unit %s/%s: dir is required", i, spaceName, unitName)
				}
				if unit.Cmd == "" && len(unit.Files) == 0 {
					return fmt.Errorf("config[%d]: unit %s/%s: either 'cmd' or 'files' is required", i, spaceName, unitName)
				}
			}
		}
	}

	return nil
}

// GetAllUnits returns a flat list of all units from the config
func GetAllUnits(cfg *config.ComposeConfig) []config.ResolvedUnit {
	var units []config.ResolvedUnit

	for _, repo := range cfg.Configs {
		for spaceName, space := range repo.Spaces {
			for unitName, unit := range space.Units {
				units = append(units, config.ResolvedUnit{
					RepoURL:   repo.Repo,
					SpaceName: spaceName,
					UnitName:  unitName,
					Dir:       unit.Dir,
					Cmd:       unit.Cmd,
				})
			}
		}
	}

	return units
}
