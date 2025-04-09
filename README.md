# git-sweep-go

`git-sweep` is an interactive command-line tool written in Go to help you clean up old or merged Git branches in your local repository.

test

It analyzes your local branches based on their merge status (compared to a primary branch like `main` or `master`) and the time since their last commit. It then presents candidate branches in an interactive terminal UI (TUI), allowing you to select branches for safe deletion locally and optionally on their corresponding remote.

## Features

- **Branch Analysis:** Identifies local branches merged into your primary branch or branches whose last commit is older than a configurable threshold.
- **Interactive TUI:** Uses `bubbletea` to provide a user-friendly interface for selecting branches.
  - Groups branches by "Merged" and "Unmerged Old".
  - Allows selection of local branches (Space).
  - Allows selection of associated remote branches (Tab/r), only if the local branch is also selected.
  - Displays branch category and basic remote info.
- **Configuration:**
  - Loads settings from `~/.config/git-sweep/config.toml` (or path specified by `--config`).
  - Interactive first-run setup if no config file is found.
  - Configurable `age_days`, `primary_main_branch`, and `protected_branches`.
- **Safety:**
  - Uses `git branch -d` (safe delete) for merged branches.
  - Uses `git branch -D` (force delete) for unmerged branches (clearly indicated in TUI).
  - Requires explicit confirmation before executing any deletions.
  - Protects the primary main branch, branches listed in `protected_branches`, and the currently checked-out branch from being listed or deleted.
- **Dry Run Mode:** Use `--dry-run` to preview actions without making any changes.
- **Remote Awareness:** Fetches remote state (`git fetch --prune`) before analysis and handles remote deletion (`git push <remote> --delete <branch>`).

## Installation

### From GitHub Releases (Recommended for most users)

1.  Go to the [Releases page](https://github.com/bral/git-sweep-go/releases) for this project.
2.  Download the archive (`.tar.gz` or `.zip`) appropriate for your operating system and architecture.
3.  Extract the `git-sweep` executable from the archive.
4.  (Optional but recommended) Move the executable to a directory included in your system's `PATH` (e.g., `/usr/local/bin` on macOS/Linux, or add its location to the PATH environment variable on Windows).

### Using `go install` (For Go developers)

If you have Go (version 1.18+) installed and configured:

```bash
go install github.com/bral/git-sweep-go/cmd/git-sweep@latest
```

This will download, compile, and install the `git-sweep` executable into your `$GOPATH/bin` directory (or `$GOBIN`). Ensure this directory is in your system's `PATH` to run `git-sweep` directly.

### Building from Source

1.  Ensure you have Go installed (version 1.18+ recommended).
2.  Clone the repository:
    ```bash
    git clone https://github.com/bral/git-sweep-go.git
    cd git-sweep-go
    ```
3.  Build the executable:
    ```bash
    go build -o git-sweep ./cmd/git-sweep/main.go
    ```
4.  (Optional) Move the `git-sweep` executable to a directory in your system's PATH (e.g., `/usr/local/bin` or `~/bin`) to run it from anywhere.

## Usage

1.  Navigate (`cd`) into the Git repository you want to clean up.
2.  Run the executable:
    ```bash
    git-sweep [flags]
    ```
    (Or use the full path if built from source and not moved: `./git-sweep [flags]`)

### Interactive TUI

- Use **Up/Down arrows** (or **k/j**) to navigate the list of candidate branches.
- Press **Space** to toggle selection for _local_ deletion for the highlighted branch.
- Press **Tab** or **r** to toggle selection for _remote_ deletion. **Note:** Remote deletion can only be selected if the local branch is also selected.
- Press **Enter** to proceed to the confirmation screen once you have made selections.
- On the confirmation screen:
  - Press **y** or **Y** to confirm and execute the deletions.
  - Press **n**, **N**, **q**, or **Esc** to cancel and return to the selection screen.
- Press **q** or **Ctrl+C** at any time to quit.

### Flags

```
Usage:
  git-sweep [flags]

Flags:
      --age int               Override config: Max age (in days) for unmerged branches (0 uses config default).
  -c, --config string         Path to custom configuration file (default: ~/.config/git-sweep/config.toml).
      --debug                 Enable debug logging.
      --dry-run               Analyze and preview actions, but do not delete.
  -h, --help                  help for git-sweep
      --primary-main string   Override config: The single main branch name to check merge status against (empty uses config default).
      --protected strings     Override config: Comma-separated list of protected branch names.
  -r, --remote string         Specify the remote repository to fetch from and consider for remote deletions. (default "origin")
  -v, --version               version for git-sweep
```

## Configuration

`git-sweep` looks for a configuration file at `~/.config/git-sweep/config.toml` by default. You can specify a different path using the `-c` or `--config` flag.

If the configuration file is not found on the first run, `git-sweep` will guide you through an interactive setup.

**File Format:** TOML

**Example `config.toml`:**

```toml
# Example config.toml

# Age in days for a branch's last commit to be considered "old" if unmerged.
age_days = 90

# The single main branch to check merge status against.
# The tool will check if other branches have been merged into this one.
primary_main_branch = "main"

# Branches that will never be suggested for deletion, regardless of status.
# Glob patterns are NOT currently supported, use exact names.
protected_branches = ["develop", "release"]
```

**Fields:**

- `age_days` (integer, default: `90`): Branches unmerged into `primary_main_branch` whose last commit is older than this many days are considered candidates.
- `primary_main_branch` (string, default: `"main"`): The branch used as the base for merge checks.
- `protected_branches` (array of strings, default: `[]`): A list of exact branch names that should never be suggested for deletion.

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details on how to contribute to this project.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
