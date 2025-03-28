package main

import (
	"bufio"   // Added for setup input
	"context" // Added for git commands
	"errors"  // Added for error checking
	"fmt"
	"os"

	"github.com/bral/git-sweep-go/internal/analyze"
	"github.com/bral/git-sweep-go/internal/config" // Added config import
	"github.com/bral/git-sweep-go/internal/gitcmd" // Added gitcmd import
	"github.com/bral/git-sweep-go/internal/tui"    // Added tui import
	"github.com/bral/git-sweep-go/internal/types"
	tea "github.com/charmbracelet/bubbletea" // Added bubbletea import
	"github.com/spf13/cobra"
)

// Global config variable to be used by the command logic
var appConfig config.Config
var isDebug bool // Global variable to store debug flag state

// logDebugf prints only if the --debug flag is set.
func logDebugf(format string, a ...any) {
	if isDebug {
		fmt.Printf(format, a...)
	}
}

// logDebugln prints only if the --debug flag is set.
func logDebugln(a ...any) {
	if isDebug {
		fmt.Println(a...)
	}
}

// printDryRunActions prints the actions that would be taken for the candidates.
func printDryRunActions(candidates []types.AnalyzedBranch) {
	fmt.Println("[Dry Run] Proposed Actions:")
	fmt.Println("\nLocal Deletions:")
	hasLocal := false
	for _, branch := range candidates {
		// Assume all candidates would be selected in a real run for dry-run output
		delType := "-d (safe)"
		if !branch.IsMerged {
			delType = "-D (force)"
		}
		fmt.Printf("  - Delete '%s' (%s)\n", branch.Name, delType)
		hasLocal = true
	}
	if !hasLocal { fmt.Println("  (None)") }

	fmt.Println("\nRemote Deletions:")
	hasRemote := false
	for _, branch := range candidates {
		// Assume remote is selected if local is and remote exists
		if branch.Remote != "" {
			fmt.Printf("  - Delete remote '%s/%s'\n", branch.Remote, branch.Name)
			hasRemote = true
		}
	}
	if !hasRemote { fmt.Println("  (None)") }
	fmt.Println("\n(Dry run complete, no changes made)")
}


var rootCmd = &cobra.Command{
	Use:     "git-sweep",
	Version: "0.1.0", // Initial version
	Short:   "git-sweep helps clean up old Git branches interactively",
	Long: `git-sweep analyzes your local Git repository for branches that are
merged or haven't been updated recently. It presents these branches
in an interactive terminal UI, allowing you to select and delete them
safely (both locally and optionally on the remote).`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Get debug flag value early
		isDebug, _ = cmd.Flags().GetBool("debug")

		logDebugln("Starting PersistentPreRunE...")
		customConfigPath, _ := cmd.Flags().GetString("config")
		logDebugf("Custom config path flag: %q\n", customConfigPath)

		var err error
		appConfig, err = config.LoadConfig(customConfigPath)

		if err != nil {
			if errors.Is(err, config.ErrConfigNotFound) {
				// Config not found, run first-time setup
				fmt.Println("Configuration file not found. Starting first-time setup...")
				reader := bufio.NewReader(os.Stdin)
				appConfig, err = config.FirstRunSetup(reader, os.Stdout)
				if err != nil {
					return fmt.Errorf("failed during first-time setup: %w", err)
				}

				// Save the newly created config
				savedPath, saveErr := config.SaveConfig(appConfig, customConfigPath)
				if saveErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to save configuration to %q: %v\n", savedPath, saveErr)
				} else {
					fmt.Printf("Configuration saved to %q\n", savedPath)
				}
				fmt.Println("Setup complete. Continuing execution...")
				err = nil
			} else {
				return fmt.Errorf("failed to load configuration: %w", err)
			}
		} else {
			logDebugln("Configuration loaded successfully.")
		}

		// Apply command-line overrides AFTER loading/setup
		logDebugln("Applying flag overrides...")
		if ageOverride, _ := cmd.Flags().GetInt("age"); ageOverride > 0 {
			logDebugf("Overriding AgeDays with flag value: %d\n", ageOverride)
			appConfig.AgeDays = ageOverride
		}
		if mainOverride, _ := cmd.Flags().GetString("primary-main"); mainOverride != "" {
			logDebugf("Overriding PrimaryMainBranch with flag value: %q\n", mainOverride)
			appConfig.PrimaryMainBranch = mainOverride
		}
		if protectedOverride, _ := cmd.Flags().GetStringSlice("protected"); len(protectedOverride) > 0 {
			logDebugf("Overriding ProtectedBranches with flag value: %v\n", protectedOverride)
			appConfig.ProtectedBranches = protectedOverride
			appConfig.ProtectedBranchMap = make(map[string]bool)
			for _, branch := range appConfig.ProtectedBranches {
				appConfig.ProtectedBranchMap[branch] = true
			}
		}

		if appConfig.ProtectedBranchMap == nil {
			logDebugln("ProtectedBranchMap was nil, initializing.")
			appConfig.ProtectedBranchMap = make(map[string]bool)
			for _, branch := range appConfig.ProtectedBranches {
				appConfig.ProtectedBranchMap[branch] = true
			}
		}
		logDebugln("Finished PersistentPreRunE.")
		return nil // No error from pre-run
	},
	Run: func(cmd *cobra.Command, args []string) {
		logDebugf("Configuration loaded. AgeDays: %d, Main: %s, Protected: %v\n",
			appConfig.AgeDays, appConfig.PrimaryMainBranch, appConfig.ProtectedBranches)
		logDebugln("\nExecuting git-sweep main logic...")

		// --- Core Workflow Steps ---
		ctx := context.Background() // Use background context for now

		// 2. Check Environment
		logDebugln("Checking environment...")
		inGitRepo, err := gitcmd.IsInGitRepo(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking Git repository status: %v\n", err)
			os.Exit(1)
		}
		if !inGitRepo {
			fmt.Fprintln(os.Stderr, "Error: Not inside a Git repository.")
			os.Exit(1)
		}
		logDebugln("-> Environment check passed.")

		// 3. Fetch Remote State
		remoteName, _ := cmd.Flags().GetString("remote")
		logDebugf("Fetching remote state for '%s'...\n", remoteName)
		err = gitcmd.FetchAndPrune(ctx, remoteName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to fetch remote state for '%s': %v\n", remoteName, err)
		} else {
			logDebugln("-> Remote fetch complete.")
		}


		// 4. Gather Branch Data
		logDebugln("Gathering branch data...")
		allBranches, err := gitcmd.GetAllLocalBranchInfo(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error gathering local branch info: %v\n", err)
			os.Exit(1)
		}
		if len(allBranches) == 0 {
			fmt.Println("No local branches found. Nothing to do.")
			os.Exit(0)
		}

		mainHash, err := gitcmd.GetMainBranchHash(ctx, appConfig.PrimaryMainBranch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting hash for primary main branch '%s': %v\n", appConfig.PrimaryMainBranch, err)
			fmt.Fprintln(os.Stderr, "Please ensure the 'primary_main_branch' in your config or flag exists.")
			os.Exit(1)
		}

		mergedBranchesMap, err := gitcmd.GetMergedBranches(ctx, mainHash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error determining merged branches against hash %s: %v\n", mainHash, err)
			os.Exit(1)
		}
		logDebugf("-> Found %d local branches. Primary main branch '%s' hash: %s. Found %d merged branches.\n",
			len(allBranches), appConfig.PrimaryMainBranch, mainHash, len(mergedBranchesMap))


		// 5. Analyze Branches
		logDebugln("Analyzing branches...")
		currentBranch, err := gitcmd.GetCurrentBranchName(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not determine current branch: %v\n", err)
			currentBranch = ""
		} else if currentBranch != "" {
			logDebugf("-> Current branch detected: %s (will be protected)\n", currentBranch)
		}
		analyzedBranches := analyze.AnalyzeBranches(allBranches, mergedBranchesMap, appConfig, currentBranch)
		logDebugln("-> Branch analysis complete.")


		// 6. Filter Candidates
		logDebugln("Filtering candidates...")
		candidates := make([]types.AnalyzedBranch, 0)
		for _, branch := range analyzedBranches {
			if branch.Category == types.CategoryMergedOld || branch.Category == types.CategoryUnmergedOld {
				candidates = append(candidates, branch)
			}
		}

		if len(candidates) == 0 {
			fmt.Println("-> No candidate branches found for cleanup. Exiting.")
			os.Exit(0)
		}
		logDebugf("-> Found %d candidate branches for cleanup.\n", len(candidates))

		// Check for Dry Run *before* launching TUI
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			printDryRunActions(candidates)
			os.Exit(0) // Exit after printing dry run actions
		}

		// 7. Launch Interactive TUI (only if not dry run)
		logDebugln("Launching TUI...")
		initialModel := tui.InitialModel(ctx, candidates, dryRun) // dryRun will be false here
		p := tea.NewProgram(initialModel)

		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}

		// 8. Execute Deletions (Handled within TUI via tea.Cmd)
		// 9. Display Results (Handled within TUI)

		logDebugln("\nExiting git-sweep.") // Final message only in debug

	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Define flags based on PROJECT_PLAN.md Section 10
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging.")
	rootCmd.PersistentFlags().Bool("dry-run", false, "Analyze and preview actions, but do not delete.")
	rootCmd.PersistentFlags().StringP("config", "c", "", "Path to custom configuration file (default: ~/.config/git-sweep/config.toml).")
	rootCmd.PersistentFlags().StringP("remote", "r", "origin", "Specify the remote repository to fetch from and consider for remote deletions.")
	rootCmd.PersistentFlags().Int("age", 0, "Override config: Max age (in days) for unmerged branches (0 uses config default).")
	rootCmd.PersistentFlags().String("primary-main", "", "Override config: The single main branch name to check merge status against (empty uses config default).")
	rootCmd.PersistentFlags().StringSlice("protected", []string{}, "Override config: Comma-separated list of protected branch names.")
}
