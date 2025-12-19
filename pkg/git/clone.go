package git

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	defaultCacheDir = ".cub-compose/repos"
)

// Manager handles git repository operations
type Manager struct {
	cacheDir string
}

// NewManager creates a new git manager
func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(home, defaultCacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Manager{cacheDir: cacheDir}, nil
}

// EnsureRepo clones or updates a repository and returns its local path
func (m *Manager) EnsureRepo(repoURL string, ref string) (string, error) {
	repoPath := m.getRepoPath(repoURL)

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		// Clone the repository
		if err := m.clone(repoURL, repoPath); err != nil {
			return "", err
		}
	} else {
		// Pull latest changes
		if err := m.pull(repoPath); err != nil {
			return "", err
		}
	}

	// Checkout specific ref if provided
	if ref != "" {
		if err := m.checkout(repoPath, ref); err != nil {
			return "", err
		}
	}

	return repoPath, nil
}

// getRepoPath returns the local path for a repository based on its URL hash
func (m *Manager) getRepoPath(repoURL string) string {
	hash := sha256.Sum256([]byte(repoURL))
	hashStr := hex.EncodeToString(hash[:8]) // use first 8 bytes
	return filepath.Join(m.cacheDir, hashStr)
}

// clone clones a repository to the specified path
func (m *Manager) clone(repoURL, destPath string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, destPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone %s: %w", repoURL, err)
	}

	return nil
}

// pull pulls latest changes in a repository
func (m *Manager) pull(repoPath string) error {
	cmd := exec.Command("git", "pull", "--ff-only")
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull in %s: %w", repoPath, err)
	}

	return nil
}

// checkout checks out a specific ref (branch or tag)
func (m *Manager) checkout(repoPath, ref string) error {
	// Fetch the ref first
	fetchCmd := exec.Command("git", "fetch", "origin", ref)
	fetchCmd.Dir = repoPath
	fetchCmd.Stdout = os.Stdout
	fetchCmd.Stderr = os.Stderr
	_ = fetchCmd.Run() // ignore error, ref might already be available

	// Checkout the ref
	cmd := exec.Command("git", "checkout", ref)
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout %s in %s: %w", ref, repoPath, err)
	}

	return nil
}

// GetRepoPath returns the cached path for a repo URL without any git operations
func (m *Manager) GetRepoPath(repoURL string) string {
	return m.getRepoPath(repoURL)
}
