# cub-compose

> **Note:** This is an experimental project and not an official ConfigHub project.

A docker-compose inspired CLI for managing ConfigHub config units from multiple Git repositories.

## What It Is

cub-compose reads a `configs.yaml` file that defines:
- Git repositories containing your configuration
- Spaces (logical groupings) in ConfigHub
- Units (config items) with their source definitions

It then clones the repos, generates config content (via commands like `kubectl kustomize` or by reading files), and syncs everything to ConfigHub.

## How It Works

```
configs.yaml
     │
     ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ Clone repos │ ──▶ │ Execute cmd │ ──▶ │  ConfigHub  │
│   (git)     │     │ or read     │     │    API      │
└─────────────┘     │   files     │     └─────────────┘
                    └─────────────┘
```

1. **Clone/Pull** - Repositories are cloned to `~/.cub-compose/repos/`
2. **Generate** - For each unit, run the specified command or read files
3. **Sync** - Create or update units in ConfigHub via the API

Authentication uses existing `cub` CLI credentials from `~/.confighub/`.

## Installation

```bash
go install github.com/confighub/cub-compose/cmd/cub-compose@latest
```

Or build from source:

```bash
git clone https://github.com/confighub/cub-compose
cd cub-compose
go build -o cub-compose ./cmd/cub-compose
```

## Usage

```bash
# Create or update all units
cub-compose up

# Preview changes without applying
cub-compose up --dry-run

# Delete all units
cub-compose down

# Use a custom config file
cub-compose -f my-configs.yaml up

# Verbose output
cub-compose -v up
```

## Configuration

Create a `configs.yaml` file:

```yaml
configs:
- repo: https://github.com/org/apps
  ref: main                    # optional: branch or tag
  unitLabels:                  # optional: labels for all units in this repo
    Tier: App
  spaces:
    production:
      units:
        backend:
          labels:              # optional: unit-specific labels
            Component: Backend
          dir: ./components/backend/production
          cmd: kubectl kustomize .
        frontend:
          labels:
            Component: Frontend
          dir: ./components/frontend/production
          cmd: kubectl kustomize .
        update-policies:
          dir: ./update-policies
          files:               # alternative to cmd: read files directly
          - backend-memcached.yaml
          - backend-redis.yaml
          - frontend-podinfo.yaml
    staging:
      units:
        backend:
          labels:
            Component: Backend
          dir: ./components/backend/staging
          cmd: kubectl kustomize .
```

### Config Fields

| Field | Description |
|-------|-------------|
| `repo` | Git repository URL |
| `ref` | Branch or tag (optional, defaults to default branch) |
| `unitLabels` | Labels applied to all units in this repo |
| `spaces` | Map of space names to their units |
| `units` | Map of unit names to their definitions |
| `dir` | Directory relative to repo root |
| `cmd` | Command to execute (e.g., `kubectl kustomize .`) |
| `files` | List of files to read (alternative to `cmd`) |
| `labels` | Unit-specific labels (merged with `unitLabels`) |

## Commands

### `up`

Creates or updates config units in ConfigHub.

- Spaces are auto-created if they don't exist
- Units are created or updated based on whether they already exist
- Use `--dry-run` to preview without making changes

### `down`

Deletes config units from ConfigHub.

- Requires `--force` flag for safety
- Skips units/spaces that don't exist

## Prerequisites

1. Install and authenticate with the `cub` CLI:
   ```bash
   cub auth login
   ```

2. Ensure `kubectl` is available if using `kubectl kustomize` commands

## Security

- Directory traversal attacks are prevented using Go's `os.Root` API
- Commands are executed in sandboxed directories within cloned repos
- Authentication tokens are read from the secure `~/.confighub/` directory

## License

MIT
