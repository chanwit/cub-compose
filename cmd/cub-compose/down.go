package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/confighub/cub-compose/pkg/compose"
)

func newDownCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Delete config units from ConfigHub",
		Long: `The down command reads configs.yaml and deletes the corresponding
units from ConfigHub. Units that don't exist are skipped.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDown(force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}

func runDown(force bool) error {
	fmt.Printf("Loading config from %s...\n", configFile)

	// Load the compose config
	cfg, err := compose.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get the list of units (without resolving content)
	units := compose.GetAllUnits(cfg)

	fmt.Printf("Found %d units to delete\n", len(units))
	for _, u := range units {
		fmt.Printf("  - %s/%s\n", u.SpaceName, u.UnitName)
	}

	if !force {
		fmt.Print("\nAre you sure you want to delete these units? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted")
			return nil
		}
	}

	// Create syncer and sync down
	syncer, err := compose.NewSyncer()
	if err != nil {
		return fmt.Errorf("failed to create syncer: %w", err)
	}

	fmt.Println("\nDeleting from ConfigHub...")
	if err := syncer.SyncDown(context.Background(), units); err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	fmt.Println("\nDone!")
	return nil
}
