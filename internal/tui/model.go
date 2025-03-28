package tui

import (
	"context" // Added for deletion context
	"fmt"
	"strings" // Added for View

	"github.com/charmbracelet/bubbles/spinner" // Added spinner
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss" // Added lipgloss

	"github.com/bral/git-sweep-go/internal/gitcmd" // Added for BranchToDelete
	"github.com/bral/git-sweep-go/internal/types"
)

// --- Styles ---
var (
	docStyle           = lipgloss.NewStyle().Margin(1, 2)
	selectedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	cursorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	helpStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	headingStyle       = lipgloss.NewStyle().Bold(true).Underline(true).MarginBottom(1)
	confirmPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	warningStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("202")) // Orange/Red for warnings
	successStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))  // Green for success
	errorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))  // Red for errors
	spinnerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("205")) // Spinner color
	protectedStyle     = lipgloss.NewStyle().Faint(true)                         // Style for protected branches ONLY
	activeStyle        = helpStyle.Copy().Faint(true)                            // Style for active branches (faint, unselectable)
	separatorStyle     = helpStyle.Copy().Faint(true)                            // Style for separator line
	categoryStyleMap   = map[types.BranchCategory]lipgloss.Style{
		// Protected category is handled separately (keyBranches)
		types.CategoryActive:      activeStyle, // Style for the label text only
		types.CategoryMergedOld:   successStyle.Copy(),
		types.CategoryUnmergedOld: warningStyle.Copy(),
	}
)

type viewState int

const (
	stateSelecting viewState = iota
	stateConfirming
	stateDeleting // Added state for showing activity during deletion
	stateResults
)

// --- Messages ---

// resultsMsg carries the deletion results back to the TUI after execution.
type resultsMsg struct {
	results []types.DeleteResult
}

// --- Model ---

// Model represents the state of the TUI application.
type Model struct {
	ctx               context.Context        // Context for git commands
	dryRun            bool                   // Is this a dry run?
	allAnalyzedBranches []types.AnalyzedBranch // Full list of analyzed branches (used for getting details by original index)
	keyBranches       []types.AnalyzedBranch // Protected: Current, Primary Main, Config-Protected (Not Selectable)
	suggestedBranches []types.AnalyzedBranch // MergedOld, UnmergedOld (Selectable Candidates)
	otherActiveBranches []types.AnalyzedBranch // Active (Not Selectable)
	listOrder         []int                  // Maps display index (key -> suggested -> other) to allAnalyzedBranches index
	cursor            int                    // Index in the combined displayed list (listOrder)
	selectedLocal     map[int]bool           // Map using original index from allAnalyzedBranches as key
	selectedRemote    map[int]bool           // Map using original index from allAnalyzedBranches as key
	viewState         viewState              // Current view state (selecting, confirming, etc.)
	results           []types.DeleteResult   // Stores the results of deletion attempts
	spinner           spinner.Model          // Spinner model
	width             int                    // Terminal width
	height            int                    // Terminal height
}

// InitialModel creates the starting model for the TUI, separating branches into three groups.
func InitialModel(ctx context.Context, analyzedBranches []types.AnalyzedBranch, dryRun bool) Model {
	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.Dot

	key := make([]types.AnalyzedBranch, 0)
	suggested := make([]types.AnalyzedBranch, 0)
	active := make([]types.AnalyzedBranch, 0)
	order := make([]int, 0, len(analyzedBranches))

	// Populate key branches first and build order map
	for i, branch := range analyzedBranches {
		if branch.Category == types.CategoryProtected {
			key = append(key, branch)
			order = append(order, i) // Store original index
		}
	}
	// Populate suggested branches second and build order map
	for i, branch := range analyzedBranches {
		if branch.Category == types.CategoryMergedOld || branch.Category == types.CategoryUnmergedOld {
			suggested = append(suggested, branch)
			order = append(order, i) // Store original index
		}
	}
	// Populate active branches third and build order map
	for i, branch := range analyzedBranches {
		if branch.Category == types.CategoryActive {
			active = append(active, branch)
			order = append(order, i) // Store original index
		}
	}

	return Model{
		ctx:               ctx,
		dryRun:            dryRun,
		allAnalyzedBranches: analyzedBranches, // Keep original full list
		keyBranches:       key,
		suggestedBranches: suggested,
		otherActiveBranches: active,
		listOrder:         order, // Store the display order mapping
		selectedLocal:     make(map[int]bool), // Key is original index
		selectedRemote:    make(map[int]bool), // Key is original index
		cursor:            0,
		viewState:         stateSelecting,
		spinner:           s,
	}
}

// Init is the first command that runs when the Bubble Tea program starts.
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick // Start the spinner ticking
}

// performDeletionCmd is a tea.Cmd that executes the branch deletions.
func performDeletionCmd(ctx context.Context, branchesToDelete []gitcmd.BranchToDelete, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		results := gitcmd.DeleteBranches(ctx, branchesToDelete, dryRun)
		return resultsMsg{results: results}
	}
}

// isSelectable checks if the branch at the given *original* index can be selected.
func (m Model) isSelectable(originalIndex int) bool {
	if originalIndex < 0 || originalIndex >= len(m.allAnalyzedBranches) {
		return false
	}
	category := m.allAnalyzedBranches[originalIndex].Category
	// Only allow selecting MergedOld and UnmergedOld (original candidates)
	return category == types.CategoryMergedOld || category == types.CategoryUnmergedOld
}

// Update handles messages and updates the model accordingly.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	// Handle terminal resize events
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Optionally, re-calculate layout or styles here if needed
		return m, nil

	case resultsMsg:
		m.results = msg.results
		m.viewState = stateResults
		return m, nil

	case spinner.TickMsg:
		if m.viewState == stateDeleting {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.viewState {

		case stateSelecting:
			totalItems := len(m.listOrder) // Use listOrder length for navigation bounds
			if totalItems == 0 {
				if msg.String() == "q" { return m, tea.Quit }
				return m, nil
			}

			switch msg.String() {
			case "q":
				return m, tea.Quit

			case "up", "k":
				if m.cursor > 0 { m.cursor-- }
			case "down", "j":
				if m.cursor < totalItems-1 { m.cursor++ }

			case " ": // Toggle local selection
				if m.cursor >= len(m.listOrder) { break } // Bounds check
				originalIndex := m.listOrder[m.cursor] // Get original index from display order
				if m.isSelectable(originalIndex) { // Check selectability using original index
					_, exists := m.selectedLocal[originalIndex]
					if exists {
						delete(m.selectedLocal, originalIndex)
						delete(m.selectedRemote, originalIndex) // Also deselect remote
					} else {
						m.selectedLocal[originalIndex] = true
					}
				}

			case "tab", "r": // Toggle remote selection
				if m.cursor >= len(m.listOrder) { break } // Bounds check
				originalIndex := m.listOrder[m.cursor]
				if m.isSelectable(originalIndex) {
					if _, localSelected := m.selectedLocal[originalIndex]; localSelected {
						branch := m.allAnalyzedBranches[originalIndex] // Use original index
						if branch.Remote != "" {
							_, remoteSelected := m.selectedRemote[originalIndex]
							if remoteSelected {
								delete(m.selectedRemote, originalIndex)
							} else {
								m.selectedRemote[originalIndex] = true
							}
						}
					}
				}

			case "enter":
				// Check selections using the maps (which use original indices)
				if len(m.selectedLocal) > 0 || len(m.selectedRemote) > 0 {
					m.viewState = stateConfirming
				}
				return m, nil
			}

		case stateConfirming:
			switch msg.String() {
			case "q", "n", "N", "esc":
				m.viewState = stateSelecting
				return m, nil
			case "y", "Y":
				m.viewState = stateDeleting
				branchesToDelete := m.GetBranchesToDelete() // This function needs to use the maps correctly
				return m, tea.Batch(
					performDeletionCmd(m.ctx, branchesToDelete, m.dryRun),
					m.spinner.Tick,
				)
			}

		case stateDeleting:
			return m, nil // Ignore keys while deleting

		case stateResults:
			return m, tea.Quit // Any key exits
		}
	}

	// Update spinner if deleting
	if m.viewState == stateDeleting {
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the UI based on the model's state.
func (m Model) View() string {
	var b strings.Builder

	switch m.viewState {

	case stateSelecting:
		title := "Branches (Space: select local, Tab/r: select remote):"
		if m.dryRun { title = warningStyle.Render("[Dry Run] ") + title }
		title += helpStyle.Render(" (Remote requires local)")
		b.WriteString(title + "\n\n")

		itemIndex := 0 // Tracks the overall item index for cursor comparison

		// --- Render Key Branches (No Header) ---
		for _, branch := range m.keyBranches {
			cursor := " "
			if m.cursor == itemIndex { cursor = cursorStyle.Render(">") }

			localCheckbox := "[-]" // Never selectable
			remoteCheckbox := "[-]"
			lineStyle := protectedStyle

			remoteInfo := "(none)"
			if branch.Remote != "" { remoteInfo = fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name) }

			categoryText := protectedStyle.Render("(Protected)")
			if branch.IsCurrent {
				categoryText = protectedStyle.Render("(Current)")
			}

			line := fmt.Sprintf("Local: %s %s | Remote: %s %s %s",
				localCheckbox, branch.Name, remoteCheckbox, remoteInfo, categoryText)

			b.WriteString(cursor + " " + lineStyle.Render(line) + "\n")
			itemIndex++
		}

		// --- Separator and Header for Suggested branches ---
		hasSuggestions := len(m.suggestedBranches) > 0
		hasActive := len(m.otherActiveBranches) > 0
		hasKeys := len(m.keyBranches) > 0

		if hasKeys && (hasSuggestions || hasActive) {
			// Add separator only if key branches exist AND others exist
			b.WriteString(separatorStyle.Render("---") + "\n")
		}

		if hasSuggestions {
			b.WriteString(headingStyle.Render("Suggested Branches (Candidates):") + "\n")
			for _, branch := range m.suggestedBranches {
				originalIndex := -1
				// Find original index by iterating through listOrder which maps display index to original index
				displayIndex := itemIndex // The current display index corresponds to this branch
				if displayIndex < len(m.listOrder) {
					originalIndex = m.listOrder[displayIndex]
				} else { continue } // Should not happen if listOrder is correct
				if originalIndex < 0 || originalIndex >= len(m.allAnalyzedBranches) { continue } // Safety check

				cursor := " "
				if m.cursor == itemIndex { cursor = cursorStyle.Render(">") }

				// These are always selectable
				localCheckbox := "[ ]"
				if _, ok := m.selectedLocal[originalIndex]; ok { localCheckbox = selectedStyle.Render("[x]") }

				remoteCheckbox := "[-]"
				remoteInfo := "(none)"
				if branch.Remote != "" {
					remoteCheckbox = "[ ]"
					remoteInfo = fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name)
					if _, ok := m.selectedRemote[originalIndex]; ok { remoteCheckbox = selectedStyle.Render("[x]") }
				}

				categoryStyle := categoryStyleMap[branch.Category]
				categoryText := categoryStyle.Render("(" + string(branch.Category) + ")")

				line := fmt.Sprintf("Local: %s %s | Remote: %s %s %s",
					localCheckbox, branch.Name, remoteCheckbox, remoteInfo, categoryText)

				// Apply styling based on cursor and category
				if m.cursor == itemIndex {
					if branch.Category == types.CategoryUnmergedOld {
						b.WriteString(cursor + " " + warningStyle.Render(selectedStyle.Render(line)) + "\n")
					} else {
						b.WriteString(cursor + " " + selectedStyle.Render(line) + "\n")
					}
				} else {
					if branch.Category == types.CategoryUnmergedOld {
						b.WriteString(cursor + " " + warningStyle.Render(line) + "\n")
					} else {
						b.WriteString(cursor + " " + line + "\n")
					}
				}
				itemIndex++
			}
		}

		// --- Separator and Header for Other Active branches ---
		if hasSuggestions && hasActive {
			// Add separator only if suggested branches exist AND active branches exist
			b.WriteString(separatorStyle.Render("---") + "\n")
		}
		if hasActive {
			b.WriteString(headingStyle.Render("Other Branches (Active / Not Selectable):") + "\n")
			for _, branch := range m.otherActiveBranches {
				cursor := " "
				if m.cursor == itemIndex { cursor = cursorStyle.Render(">") }

				// These are never selectable
				localCheckbox := "[-]"
				remoteCheckbox := "[-]"
				lineStyle := activeStyle // Use faint style

				remoteInfo := "(none)"
				if branch.Remote != "" { remoteInfo = fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name) }

				categoryText := activeStyle.Render("(" + string(branch.Category) + ")")

				line := fmt.Sprintf("Local: %s %s | Remote: %s %s %s",
					localCheckbox, branch.Name, remoteCheckbox, remoteInfo, categoryText)

				b.WriteString(cursor + " " + lineStyle.Render(line) + "\n")
				itemIndex++
			}
		}


		if itemIndex == 0 { // If no branches were rendered at all
			b.WriteString(helpStyle.Render("No branches found to display.") + "\n")
		}

		// Add selection summary to footer
		footer := fmt.Sprintf("\nSelected: %d local, %d remote | Enter: Confirm | q/Ctrl+C: Quit\n",
			len(m.selectedLocal), len(m.selectedRemote))
		b.WriteString(helpStyle.Render(footer))


	case stateConfirming:
		title := "Confirm Actions:"
		if m.dryRun { title = warningStyle.Render("[Dry Run] ") + title }
		b.WriteString(title + "\n\n")
		branchesToDelete := m.GetBranchesToDelete()
		if len(branchesToDelete) == 0 { b.WriteString("No actions selected.\n") } else {
			b.WriteString("Local Deletions:\n"); hasLocal := false
			for _, bd := range branchesToDelete { if !bd.IsRemote { style := lipgloss.NewStyle(); delType := "-d (safe)"; if !bd.IsMerged { style = warningStyle; delType = "-D (force)" }; b.WriteString(style.Render(fmt.Sprintf("  - Delete '%s' (%s)\n", bd.Name, delType))); hasLocal = true } }; if !hasLocal { b.WriteString(helpStyle.Render("  (None)\n")) }
			b.WriteString("\nRemote Deletions:\n"); hasRemote := false
			for _, bd := range branchesToDelete { if bd.IsRemote { b.WriteString(fmt.Sprintf("  - Delete remote '%s/%s'\n", bd.Remote, bd.Name)); hasRemote = true } }; if !hasRemote { b.WriteString(helpStyle.Render("  (None)\n")) }
		}
		b.WriteString("\n" + confirmPromptStyle.Render("Proceed? (y/N) "))

	case stateDeleting:
		b.WriteString(m.spinner.View())
		b.WriteString(" Processing deletions...")
		if m.dryRun { b.WriteString(warningStyle.Render(" (Dry Run)")) }

	case stateResults:
		title := "Deletion Results:"
		if m.dryRun { title = warningStyle.Render("[Dry Run] ") + title }
		b.WriteString(title + "\n\n")
		if len(m.results) > 0 {
			for _, res := range m.results { style := successStyle; status := "✅ Success"; if !res.Success { style = errorStyle; status = "❌ Failed" }; branchType := "Local"; if res.IsRemote { branchType = fmt.Sprintf("Remote (%s)", res.RemoteName) }; line := fmt.Sprintf("%s: %s %s - %s", status, branchType, res.BranchName, res.Message); b.WriteString(style.Render(line) + "\n") }
		} else { b.WriteString(helpStyle.Render("(No deletion actions were performed or results available)\n")) }
		b.WriteString(helpStyle.Render("\nPress any key to exit."))
	}

	return docStyle.Render(b.String())
}

// GetBranchesToDelete builds the list of actions based on current selections using original indices.
func (m Model) GetBranchesToDelete() []gitcmd.BranchToDelete {
	branches := make([]gitcmd.BranchToDelete, 0)
	// Iterate through the selection maps which use original indices
	for originalIndex := range m.selectedLocal {
		if originalIndex < 0 || originalIndex >= len(m.allAnalyzedBranches) { continue }
		branchInfo := m.allAnalyzedBranches[originalIndex]
		// Check if it's selectable before adding
		if m.isSelectable(originalIndex) {
			branches = append(branches, gitcmd.BranchToDelete{
				Name:     branchInfo.Name, IsRemote: false, Remote: "", IsMerged: branchInfo.IsMerged, Hash: branchInfo.CommitHash,
			})
		}
	}
	for originalIndex := range m.selectedRemote {
		if originalIndex < 0 || originalIndex >= len(m.allAnalyzedBranches) { continue }
		branchInfo := m.allAnalyzedBranches[originalIndex]
		// Check if it's selectable and has a remote before adding
		if m.isSelectable(originalIndex) && branchInfo.Remote != "" {
			branches = append(branches, gitcmd.BranchToDelete{
				Name:     branchInfo.Name, IsRemote: true, Remote:   branchInfo.Remote, IsMerged: branchInfo.IsMerged, Hash:     branchInfo.CommitHash,
			})
		}
	}
	// Remove duplicates (e.g., if local+remote selected, only one entry per type needed by DeleteBranches)
	finalBranches := make([]gitcmd.BranchToDelete, 0, len(branches))
	seen := make(map[string]bool)
	for _, btd := range branches {
		key := fmt.Sprintf("%s-%t", btd.Name, btd.IsRemote)
		if !seen[key] {
			finalBranches = append(finalBranches, btd)
			seen[key] = true
		}
	}
	return finalBranches
}
