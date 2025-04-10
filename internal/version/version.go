// Package version handles version checks and update notifications.
package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bral/git-sweep-go/internal/config"
)

const (
	// GitHubReleaseURL is the URL for checking the latest release
	GitHubReleaseURL = "https://api.github.com/repos/bral/git-sweep-go/releases/latest"
	// DayInSeconds is the number of seconds in a day (for version check interval)
	DayInSeconds = 86400
)

// GitHubRelease represents the GitHub API response for releases
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Check checks if a new version is available and returns information about the update
// It follows these steps:
// 1. Checks if 24 hours have passed since last check
// 2. If so, queries GitHub API for latest version
// 3. Compares with current version
// 4. Updates config with check time and latest version
// 5. Returns information about available updates
func Check(ctx context.Context, currentVersion string, cfg *config.Config) (bool, string, string, error) {
	now := time.Now().Unix()
	hasUpdate := false
	latestVersion := ""
	releaseURL := ""

	// Check if it's been at least a day since last check
	if now-cfg.LastVersionCheck < DayInSeconds {
		// If we already know about an update, return that info
		if cfg.LatestKnownVersion != "" && cfg.LatestKnownVersion != currentVersion {
			cleanCurrent := strings.TrimPrefix(currentVersion, "v")
			cleanLatest := strings.TrimPrefix(cfg.LatestKnownVersion, "v")

			// Use proper semantic versioning comparison
			hasUpdate := false
			latestVersionParts := strings.Split(cleanLatest, ".")
			currentVersionParts := strings.Split(cleanCurrent, ".")

			// Compare each part numerically
			for i := 0; i < len(latestVersionParts) && i < len(currentVersionParts); i++ {
				latestPart, latestErr := strconv.Atoi(latestVersionParts[i])
				currentPart, currentErr := strconv.Atoi(currentVersionParts[i])

				// If conversion fails, fall back to string comparison for this part
				if latestErr != nil || currentErr != nil {
					if latestVersionParts[i] > currentVersionParts[i] {
						hasUpdate = true
						break
					} else if latestVersionParts[i] < currentVersionParts[i] {
						break
					}
					continue
				}

				if latestPart > currentPart {
					hasUpdate = true
					break
				} else if latestPart < currentPart {
					break
				}
			}

			// If all compared parts are equal but latest has more parts, it's newer
			if !hasUpdate && len(latestVersionParts) > len(currentVersionParts) {
				hasUpdate = true
			}

			if hasUpdate {
				return true, cfg.LatestKnownVersion, GitHubReleaseURL, nil
			}
		}
		return false, "", "", nil
	}

	// Get latest version from GitHub
	client := &http.Client{
		Timeout: 5 * time.Second, // Set a short timeout
	}

	req, err := http.NewRequestWithContext(ctx, "GET", GitHubReleaseURL, nil)
	if err != nil {
		return false, "", "", fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("User-Agent", "git-sweep-go/"+currentVersion)

	resp, err := client.Do(req)
	if err != nil {
		// Silently fail on network errors
		return false, "", "", nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// GitHub API error, silently fail
		return false, "", "", nil
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return false, "", "", nil
	}

	// Update config with latest check time and version
	cfg.LastVersionCheck = now
	cfg.LatestKnownVersion = release.TagName

	// Save the updated config
	_, err = config.SaveConfig(*cfg, "")
	if err != nil {
		// Just log the error, don't fail the check
		fmt.Fprintf(os.Stderr, "Warning: Failed to save version check info: %v\n", err)
	}

	// Compare versions (using proper semantic versioning comparison)
	cleanCurrent := strings.TrimPrefix(currentVersion, "v")
	cleanLatest := strings.TrimPrefix(release.TagName, "v")

	// Use proper semantic versioning comparison
	latestVersionParts := strings.Split(cleanLatest, ".")
	currentVersionParts := strings.Split(cleanCurrent, ".")

	// Compare each part numerically
	for i := 0; i < len(latestVersionParts) && i < len(currentVersionParts); i++ {
		latestPart, latestErr := strconv.Atoi(latestVersionParts[i])
		currentPart, currentErr := strconv.Atoi(currentVersionParts[i])

		// If conversion fails, fall back to string comparison for this part
		if latestErr != nil || currentErr != nil {
			if latestVersionParts[i] > currentVersionParts[i] {
				hasUpdate = true
				break
			} else if latestVersionParts[i] < currentVersionParts[i] {
				break
			}
			continue
		}

		if latestPart > currentPart {
			hasUpdate = true
			break
		} else if latestPart < currentPart {
			break
		}
	}

	// If all compared parts are equal but latest has more parts, it's newer
	if !hasUpdate && len(latestVersionParts) > len(currentVersionParts) {
		hasUpdate = true
	}

	if hasUpdate {
		latestVersion = release.TagName
		releaseURL = release.HTMLURL
	}

	return hasUpdate, latestVersion, releaseURL, nil
}

// ShowUpdateNotification displays a notification about an available update
func ShowUpdateNotification(currentVersion, latestVersion, releaseURL string) {
	// Use os.Stdout to comply with linting rules
	out := os.Stdout

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "╭─────────────────────────────────────────────────────╮")
	_, _ = fmt.Fprintln(out, "│             🚀 New Version Available! 🚀             │")
	_, _ = fmt.Fprintln(out, "├─────────────────────────────────────────────────────┤")
	_, _ = fmt.Fprintf(out, "│ Current version: %-37s │\n", currentVersion)
	_, _ = fmt.Fprintf(out, "│ Latest version:  %-37s │\n", latestVersion)
	_, _ = fmt.Fprintln(out, "│                                                     │")
	_, _ = fmt.Fprintln(out, "│ To update:                                          │")
	_, _ = fmt.Fprintln(out, "│ • Go users: go install github.com/bral/git-sweep-go │")
	_, _ = fmt.Fprintln(out, "│ • Binary: Download from GitHub releases page        │")
	_, _ = fmt.Fprintln(out, "│                                                     │")
	_, _ = fmt.Fprintf(out, "│ Release details: %-36s │\n", releaseURL)
	_, _ = fmt.Fprintln(out, "╰─────────────────────────────────────────────────────╯")
	_, _ = fmt.Fprintln(out, "")

	// Ask if user wants to update now
	_, _ = fmt.Fprint(out, "Would you like to update now? (y/n): ")
	var response string
	_, _ = fmt.Scanln(&response)

	if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
		performUpdate(latestVersion)
	}
}

// performUpdate attempts to update the application based on how it was installed
func performUpdate(latestVersion string) {
	// Use os.Stdout to comply with linting rules
	out := os.Stdout

	// Validate that latestVersion has a valid format (v1.2.3 or similar)
	if !strings.HasPrefix(latestVersion, "v") || !isValidSemVer(latestVersion[1:]) {
		_, _ = fmt.Fprintf(out, "❌ Invalid version format: %s\n", latestVersion)
		_, _ = fmt.Fprintln(out, "Please update manually:")
		printManualInstructions(out)
		return
	}

	// Try different update mechanisms

	// 1. Try go install
	_, _ = fmt.Fprintln(out, "Attempting update via go install...")
	cmd := exec.Command("go", "install", "github.com/bral/git-sweep-go@"+latestVersion)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		_, _ = fmt.Fprintln(out, "✅ Update successful! You're now using the latest version.")
		return
	}

	// If auto-update failed, provide manual instructions
	_, _ = fmt.Fprintln(out, "\nAutomatic update failed. Please update manually:")
	printManualInstructions(out)
}

// isValidSemVer returns true if the version string follows valid semver format
func isValidSemVer(version string) bool {
	// Simple regex check for semver format (x.y.z with optional pre-release/build metadata)
	matched, err := regexp.MatchString(`^(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z-]+)?(\+[0-9A-Za-z-]+)?$`, version)
	return err == nil && matched
}

// printManualInstructions prints instructions for manual updates
func printManualInstructions(out io.Writer) {
	_, _ = fmt.Fprintln(out, "- Download the latest version from GitHub:")
	_, _ = fmt.Fprintf(out, "  %s\n", "https://github.com/bral/git-sweep-go/releases/latest")
	_, _ = fmt.Fprintln(out, "- Or use your package manager to update")
}
