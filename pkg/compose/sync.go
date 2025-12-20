package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	pkgconfig "github.com/confighub/cub-compose/pkg/config"
	goclientnew "github.com/confighub/sdk/openapi/goclient-new"
	"github.com/confighub/sdk/workerapi"
)

const (
	configHubDir     = ".confighub"
	defaultServerURL = "https://hub.confighub.com"
)

// resolveTokenPath resolves a token file path, handling ~ prefix like the SDK does
func resolveTokenPath(home, tokenFile string) string {
	if filepath.IsAbs(tokenFile) {
		return tokenFile
	}
	// Handle ~ prefix (e.g., ~/.confighub/tokens/context.json)
	if strings.HasPrefix(tokenFile, "~") {
		return filepath.Join(home, tokenFile[1:])
	}
	// Default to tokens directory with just the filename
	return filepath.Join(home, configHubDir, "tokens", filepath.Base(tokenFile))
}

// CubConfig represents the cub CLI config structure
type CubConfig struct {
	APIVersion     string       `yaml:"apiVersion"`
	Kind           string       `yaml:"kind"`
	CurrentContext string       `yaml:"currentContext"`
	Contexts       []CubContext `yaml:"contexts"`
}

// CubContext represents a context in the cub config
type CubContext struct {
	Name       string        `yaml:"name"`
	Coordinate CubCoordinate `yaml:"coordinate"`
	Settings   CubSettings   `yaml:"settings"`
	Metadata   CubMetadata   `yaml:"metadata"`
}

// CubCoordinate represents a context coordinate
type CubCoordinate struct {
	ServerURL      string `yaml:"serverURL"`
	OrganizationID string `yaml:"organizationID"`
	User           string `yaml:"user"`
}

// CubSettings represents context settings
type CubSettings struct {
	DefaultSpace string `yaml:"defaultSpace"`
}

// CubMetadata represents context metadata
type CubMetadata struct {
	TokenFile        string `yaml:"tokenFile"`
	OrganizationName string `yaml:"organizationName"`
}

// TokenData represents the token file structure
type TokenData struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

// Syncer handles synchronization with ConfigHub
type Syncer struct {
	client    *goclientnew.ClientWithResponses
	serverURL string
}

// NewSyncer creates a new syncer using the cub CLI credentials
func NewSyncer() (*Syncer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(home, configHubDir, "config.yaml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cub config (run 'cub auth login' first): %w", err)
	}

	var config CubConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse cub config: %w", err)
	}

	// Find the current context
	var currentCtx *CubContext
	for i := range config.Contexts {
		if config.Contexts[i].Name == config.CurrentContext {
			currentCtx = &config.Contexts[i]
			break
		}
	}
	if currentCtx == nil {
		return nil, fmt.Errorf("current context %q not found in config", config.CurrentContext)
	}

	// Load the token - handle ~ prefix like the SDK does
	tokenPath := resolveTokenPath(home, currentCtx.Metadata.TokenFile)
	tokenData, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token (run 'cub auth login' first): %w", err)
	}

	var token TokenData
	if err := json.Unmarshal(tokenData, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Create the API client
	serverURL := currentCtx.Coordinate.ServerURL
	if serverURL == "" {
		serverURL = defaultServerURL
	}

	client, err := goclientnew.NewClientWithResponses(serverURL+"/api", func(c *goclientnew.Client) error {
		c.RequestEditors = append(c.RequestEditors, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
			return nil
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return &Syncer{
		client:    client,
		serverURL: serverURL,
	}, nil
}

// SyncUp creates or updates spaces and units in ConfigHub
func (s *Syncer) SyncUp(ctx context.Context, spaces []pkgconfig.ResolvedSpace, units []pkgconfig.ResolvedUnit) error {
	// Cache space IDs to avoid redundant lookups/creates
	spaceCache := make(map[string]goclientnew.UUID)

	// Build a map of space labels for quick lookup
	spaceLabels := make(map[string]map[string]string)
	for _, space := range spaces {
		spaceLabels[space.Name] = space.Labels
	}

	for _, unit := range units {
		fmt.Printf("Syncing %s/%s...\n", unit.SpaceName, unit.UnitName)

		// Get or create the space
		spaceID, ok := spaceCache[unit.SpaceName]
		if !ok {
			var err error
			labels := spaceLabels[unit.SpaceName]
			spaceID, err = s.ensureSpace(ctx, unit.SpaceName, labels)
			if err != nil {
				return fmt.Errorf("failed to ensure space %s: %w", unit.SpaceName, err)
			}
			spaceCache[unit.SpaceName] = spaceID
		}

		// Check if unit exists
		existingUnit, err := s.getUnitBySlug(ctx, spaceID, unit.UnitName)
		if err != nil {
			return fmt.Errorf("failed to check unit %s: %w", unit.UnitName, err)
		}

		if existingUnit != nil {
			// Update existing unit
			err = s.updateUnit(ctx, spaceID, existingUnit.UnitID, unit)
		} else {
			// Create new unit
			err = s.createUnit(ctx, spaceID, unit)
		}

		if err != nil {
			return err
		}

		fmt.Printf("  ✓ %s/%s synced\n", unit.SpaceName, unit.UnitName)
	}

	return nil
}

// SyncDown deletes units from ConfigHub
func (s *Syncer) SyncDown(ctx context.Context, units []pkgconfig.ResolvedUnit) error {
	// Cache space IDs to avoid redundant lookups
	spaceCache := make(map[string]goclientnew.UUID)
	missingSpaces := make(map[string]bool)

	for _, unit := range units {
		fmt.Printf("Deleting %s/%s...\n", unit.SpaceName, unit.UnitName)

		// Check if we already know this space is missing
		if missingSpaces[unit.SpaceName] {
			fmt.Printf("  ! Space %s not found, skipping\n", unit.SpaceName)
			continue
		}

		// Get the space ID by slug (lookup only, don't create)
		spaceID, ok := spaceCache[unit.SpaceName]
		if !ok {
			var err error
			spaceID, err = s.getSpaceID(ctx, unit.SpaceName)
			if err != nil {
				missingSpaces[unit.SpaceName] = true
				fmt.Printf("  ! Space %s not found, skipping\n", unit.SpaceName)
				continue
			}
			spaceCache[unit.SpaceName] = spaceID
		}

		// Check if unit exists
		existingUnit, err := s.getUnitBySlug(ctx, spaceID, unit.UnitName)
		if err != nil {
			return fmt.Errorf("failed to check unit %s: %w", unit.UnitName, err)
		}

		if existingUnit == nil {
			fmt.Printf("  ! Unit %s not found, skipping\n", unit.UnitName)
			continue
		}

		// Delete the unit
		resp, err := s.client.DeleteUnitWithResponse(ctx, spaceID, existingUnit.UnitID)
		if err != nil {
			return fmt.Errorf("failed to delete unit %s: %w", unit.UnitName, err)
		}

		if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
			return fmt.Errorf("failed to delete unit %s: %s", unit.UnitName, resp.Status())
		}

		fmt.Printf("  ✓ %s/%s deleted\n", unit.SpaceName, unit.UnitName)
	}

	return nil
}

// getSpaceID looks up a space by slug (lookup only, returns error if not found)
func (s *Syncer) getSpaceID(ctx context.Context, spaceSlug string) (goclientnew.UUID, error) {
	where := fmt.Sprintf("Slug = '%s'", spaceSlug)
	params := &goclientnew.ListSpacesParams{
		Where: &where,
	}

	resp, err := s.client.ListSpacesWithResponse(ctx, params)
	if err != nil {
		return goclientnew.UUID{}, err
	}

	if resp.StatusCode() != http.StatusOK {
		return goclientnew.UUID{}, fmt.Errorf("failed to list spaces: %s", resp.Status())
	}

	if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		return goclientnew.UUID{}, fmt.Errorf("space %q not found", spaceSlug)
	}

	extSpace := (*resp.JSON200)[0]
	if extSpace.Space == nil {
		return goclientnew.UUID{}, fmt.Errorf("space %q has no Space data", spaceSlug)
	}
	return extSpace.Space.SpaceID, nil
}

// ensureSpace looks up a space by slug; creates it if it doesn't exist
func (s *Syncer) ensureSpace(ctx context.Context, spaceSlug string, labels map[string]string) (goclientnew.UUID, error) {
	// Try to find existing space
	where := fmt.Sprintf("Slug = '%s'", spaceSlug)
	params := &goclientnew.ListSpacesParams{
		Where: &where,
	}

	resp, err := s.client.ListSpacesWithResponse(ctx, params)
	if err != nil {
		return goclientnew.UUID{}, err
	}

	if resp.StatusCode() != http.StatusOK {
		return goclientnew.UUID{}, fmt.Errorf("failed to list spaces: %s", resp.Status())
	}

	// Space exists, return its ID
	if resp.JSON200 != nil && len(*resp.JSON200) > 0 {
		extSpace := (*resp.JSON200)[0]
		if extSpace.Space == nil {
			return goclientnew.UUID{}, fmt.Errorf("space %q has no Space data", spaceSlug)
		}
		return extSpace.Space.SpaceID, nil
	}

	// Space doesn't exist, create it
	fmt.Printf("  Creating space %s...\n", spaceSlug)
	createBody := goclientnew.Space{
		Slug:        spaceSlug,
		DisplayName: spaceSlug,
	}

	// Add labels if present
	if len(labels) > 0 {
		createBody.Labels = labels
	}

	createResp, err := s.client.CreateSpaceWithResponse(ctx, nil, createBody)
	if err != nil {
		return goclientnew.UUID{}, fmt.Errorf("failed to create space: %w", err)
	}

	if createResp.StatusCode() != http.StatusOK && createResp.StatusCode() != http.StatusCreated {
		return goclientnew.UUID{}, fmt.Errorf("failed to create space: %s", createResp.Status())
	}

	if createResp.JSON200 == nil {
		return goclientnew.UUID{}, fmt.Errorf("no space returned after creation")
	}

	return createResp.JSON200.SpaceID, nil
}

// getUnitBySlug looks up a unit by slug within a space
func (s *Syncer) getUnitBySlug(ctx context.Context, spaceID goclientnew.UUID, unitSlug string) (*goclientnew.Unit, error) {
	where := fmt.Sprintf("Slug = '%s'", unitSlug)
	params := &goclientnew.ListUnitsParams{
		Where: &where,
	}

	resp, err := s.client.ListUnitsWithResponse(ctx, spaceID, params)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to list units: %s", resp.Status())
	}

	if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		return nil, nil // Not found
	}

	extUnit := (*resp.JSON200)[0]
	return extUnit.Unit, nil
}

// createUnit creates a new unit
func (s *Syncer) createUnit(ctx context.Context, spaceID goclientnew.UUID, unit pkgconfig.ResolvedUnit) error {
	toolchainType := string(workerapi.ToolchainKubernetesYAML)
	body := goclientnew.Unit{
		Slug:          unit.UnitName,
		DisplayName:   unit.UnitName,
		Data:          string(unit.Content),
		ToolchainType: toolchainType,
	}

	// Add labels if present
	if len(unit.Labels) > 0 {
		body.Labels = unit.Labels
	}

	resp, err := s.client.CreateUnitWithResponse(ctx, spaceID, nil, body)
	if err != nil {
		return fmt.Errorf("failed to create unit: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusCreated {
		return fmt.Errorf("failed to create unit: %s", resp.Status())
	}

	return nil
}

// updateUnit updates an existing unit's data
func (s *Syncer) updateUnit(ctx context.Context, spaceID, unitID goclientnew.UUID, unit pkgconfig.ResolvedUnit) error {
	toolchainType := string(workerapi.ToolchainKubernetesYAML)
	body := goclientnew.Unit{
		Slug:          unit.UnitName,
		DisplayName:   unit.UnitName,
		Data:          string(unit.Content),
		ToolchainType: toolchainType,
	}

	// Add labels if present
	if len(unit.Labels) > 0 {
		body.Labels = unit.Labels
	}

	resp, err := s.client.UpdateUnitWithResponse(ctx, spaceID, unitID, nil, body)
	if err != nil {
		return fmt.Errorf("failed to update unit: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("failed to update unit: %s", resp.Status())
	}

	return nil
}
