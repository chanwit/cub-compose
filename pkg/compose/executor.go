package compose

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/confighub/cub-compose/pkg/config"
	"github.com/confighub/cub-compose/pkg/git"
)

// Executor handles command execution for units
type Executor struct {
	gitManager *git.Manager
}

// NewExecutor creates a new executor
func NewExecutor() (*Executor, error) {
	gitMgr, err := git.NewManager()
	if err != nil {
		return nil, err
	}

	return &Executor{gitManager: gitMgr}, nil
}

// ResolveUnits clones repos and executes commands for all units
func (e *Executor) ResolveUnits(cfg *config.ComposeConfig) ([]config.ResolvedUnit, error) {
	var resolved []config.ResolvedUnit

	// Process each repo
	for _, repoCfg := range cfg.Configs {
		repoPath, err := e.gitManager.EnsureRepo(repoCfg.Repo, repoCfg.Ref)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure repo %s: %w", repoCfg.Repo, err)
		}

		// Process each space
		for spaceName, space := range repoCfg.Spaces {
			if space == nil || len(space.Units) == 0 {
				continue
			}

			// Process each unit
			for unitName, unit := range space.Units {
				var content []byte
				var err error

				// Use files or cmd to get content
				if len(unit.Files) > 0 {
					content, err = e.readFiles(repoPath, unit.Dir, unit.Files)
				} else if unit.Cmd != "" {
					content, err = e.executeCommand(repoPath, unit.Dir, unit.Cmd)
				} else {
					return nil, fmt.Errorf("unit %s/%s: either 'cmd' or 'files' is required", spaceName, unitName)
				}

				if err != nil {
					return nil, fmt.Errorf("failed to resolve %s/%s: %w", spaceName, unitName, err)
				}

				// Merge labels: repo-level unitLabels + unit-level labels
				labels := make(map[string]string)
				for k, v := range repoCfg.UnitLabels {
					labels[k] = v
				}
				for k, v := range unit.Labels {
					labels[k] = v
				}

				resolved = append(resolved, config.ResolvedUnit{
					RepoURL:   repoCfg.Repo,
					SpaceName: spaceName,
					UnitName:  unitName,
					Dir:       unit.Dir,
					Cmd:       unit.Cmd,
					Labels:    labels,
					Content:   content,
				})
			}
		}
	}

	return resolved, nil
}

// Verbose controls whether to print detailed execution info
var Verbose bool

// readFiles reads and concatenates multiple files from a directory using os.Root for safe path handling
func (e *Executor) readFiles(repoPath, dir string, files []string) ([]byte, error) {
	// Open repo as root to prevent directory traversal
	repoRoot, err := os.OpenRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repo root: %w", err)
	}
	defer repoRoot.Close()

	// Open the target directory within the repo root (validates path is within repo)
	dirRoot, err := repoRoot.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid dir %q: %w", dir, err)
	}
	defer dirRoot.Close()

	if Verbose {
		fmt.Printf("  Reading files from: %s/%s\n", repoPath, dir)
	}

	var result bytes.Buffer
	for i, file := range files {
		if Verbose {
			fmt.Printf("    - %s\n", file)
		}

		// Open file safely within the directory root
		f, err := dirRoot.Open(file)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", file, err)
		}

		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file, err)
		}

		if i > 0 {
			// Add YAML document separator between files
			result.WriteString("---\n")
		}
		result.Write(data)

		// Ensure there's a newline at the end
		if len(data) > 0 && data[len(data)-1] != '\n' {
			result.WriteByte('\n')
		}
	}

	return result.Bytes(), nil
}

// executeCommand executes a command in the specified directory and returns stdout
func (e *Executor) executeCommand(repoPath, dir, cmdStr string) ([]byte, error) {
	// Use os.OpenRoot to validate the path is within the repo (prevents traversal)
	repoRoot, err := os.OpenRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repo root: %w", err)
	}

	// Validate the target directory exists and is within repo
	dirRoot, err := repoRoot.OpenRoot(dir)
	if err != nil {
		repoRoot.Close()
		return nil, fmt.Errorf("invalid dir %q: %w", dir, err)
	}
	dirRoot.Close()
	repoRoot.Close()

	// Now safe to use the joined path for command execution
	workDir := filepath.Join(repoPath, dir)

	if Verbose {
		fmt.Printf("  Executing: %s\n", cmdStr)
		fmt.Printf("  Working dir: %s\n", workDir)
	}

	// Parse the command string
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("command failed in %s: %w\nstderr: %s", workDir, err, stderr.String())
	}

	return stdout.Bytes(), nil
}
