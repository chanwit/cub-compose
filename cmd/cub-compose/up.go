package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/confighub/cub-compose/pkg/compose"
)

func newUpCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create or update config units in ConfigHub",
		Long: `The up command reads configs.yaml, clones/pulls the specified repositories,
executes the configured commands to generate config content, and creates or
updates the corresponding units in ConfigHub.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUp(dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

func runUp(dryRun bool) error {
	fmt.Printf("Loading config from %s...\n", configFile)

	// Load the compose config
	cfg, err := compose.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create executor and resolve all units
	executor, err := compose.NewExecutor()
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	// Set verbose mode
	compose.Verbose = verbose

	fmt.Println("Resolving units...")
	units, err := executor.ResolveUnits(cfg)
	if err != nil {
		return fmt.Errorf("failed to resolve units: %w", err)
	}

	fmt.Printf("Found %d units to sync\n", len(units))

	if verbose {
		for _, u := range units {
			fmt.Printf("  - %s/%s (%d bytes)", u.SpaceName, u.UnitName, len(u.Content))
			if len(u.Labels) > 0 {
				fmt.Printf(" labels=%v", u.Labels)
			}
			fmt.Println()
		}
	}

	if dryRun {
		fmt.Println("\nDry run - no changes made")
		return nil
	}

	// Create syncer and sync up
	syncer, err := compose.NewSyncer()
	if err != nil {
		return fmt.Errorf("failed to create syncer: %w", err)
	}

	fmt.Println("\nSyncing to ConfigHub...")
	if err := syncer.SyncUp(context.Background(), units); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	fmt.Println("\nDone!")
	return nil
}
