package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configFile string
	verbose    bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "cub-compose",
		Short: "Docker-compose inspired CLI for ConfigHub",
		Long: `cub-compose manages ConfigHub config units from multiple Git repositories.

It reads a configs.yaml file that defines repos, spaces, and units,
then executes commands (like kubectl kustomize) to generate config content
and syncs it to ConfigHub.

Authentication uses existing cub CLI credentials from ~/.confighub/`,
	}

	rootCmd.PersistentFlags().StringVarP(&configFile, "file", "f", "configs.yaml", "Path to configs.yaml file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	rootCmd.AddCommand(newUpCmd())
	rootCmd.AddCommand(newDownCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
