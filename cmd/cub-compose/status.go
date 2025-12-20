package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/confighub/cub-compose/pkg/compose"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show connection status and verify authentication",
		Long: `The status command verifies that cub-compose can connect to ConfigHub
using the credentials from ~/.confighub/

It displays the current context information and tests the API connection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus()
		},
	}

	return cmd
}

func runStatus() error {
	// Load config and get context info
	info, err := compose.GetContextInfo()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Context:      %s\n", info.ContextName)
	fmt.Printf("Server:       %s\n", info.ServerURL)
	fmt.Printf("Organization: %s\n", info.OrganizationName)
	fmt.Printf("User:         %s\n", info.User)

	// Try to authenticate
	syncer, err := compose.NewSyncer()
	if err != nil {
		fmt.Printf("Auth:         FAILED\n")
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Test the connection by listing spaces
	err = syncer.TestConnection(context.Background())
	if err != nil {
		fmt.Printf("Auth:         FAILED\n")
		return fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Printf("Auth:         OK\n")
	return nil
}
