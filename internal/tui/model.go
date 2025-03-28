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
	confirmPromptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	warningStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("202")) // Orange/Red for warnings
	successStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))  // Green for success
	errorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))  // Red for errors
	spinnerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("205")) // Spinner color
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
	ctx                context.Context        // Context for git commands
	dryRun             bool                   // Is this a dry run?
	originalCandidates []types.AnalyzedBranch // Original list of candidates passed in
	mergedBranches     []types.AnalyzedBranch // Filtered list: MergedOld
	unmergedBranches   []types.AnalyzedBranch // Filtered list: UnmergedOld
	listOrder          []int                  // Maps display index to originalCandidates index
	cursor             int                    // Index in the combined displayed list (listOrder)
	selectedLocal      map[int]bool           // Map using *original index* as key
	selectedRemote     map[int]bool           // Map using *original index* as key
	viewState          viewState              // Current view state (selecting, confirming, etc.)
	results            []types.DeleteResult   // Stores the results of deletion attempts
	spinner            spinner.Model          // Spinner model
	// TODO: Add dimensions for layout
}

// InitialModel creates the starting model for the TUI, grouping branches.
func InitialModel(ctx context.Context, candidates []types.AnalyzedBranch, dryRun bool) Model {
	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.Dot

	merged := make([]types.AnalyzedBranch, 0)
	unmerged := make([]types.AnalyzedBranch, 0)
	order := make([]int, 0, len(candidates)) // Pre-allocate slice

	// Group branches and store original indices
	for i, branch := range candidates {
		if branch.Category == types.CategoryMergedOld {
			merged = append(merged, branch)
			order = append(order, i) // Store original index
		} else if branch.Category == types.CategoryUnmergedOld {
			// Append unmerged later to maintain order
		}
	}
	// Append unmerged branches and their original indices
	for i, branch := range candidates {
		if branch.Category == types.CategoryUnmergedOld {
			unmerged = append(unmerged, branch)
			order = append(order, i) // Store original index
		}
	}


	return Model{
		ctx:                ctx,
		dryRun:             dryRun,
		originalCandidates: candidates, // Store original list
		mergedBranches:     merged,
		unmergedBranches:   unmerged,
		listOrder:          order, // Store the display order mapping
		selectedLocal:      make(map[int]bool), // Key is original index
		selectedRemote:     make(map[int]bool), // Key is original index
		cursor:             0,
		viewState:          stateSelecting,
		spinner:            s,
	}
}

// Init is the first command that runs when the Bubble Tea program starts.
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick // Start the spinner ticking
}

// performDeletionCmd is a tea.Cmd that executes the branch deletions.
func performDeletionCmd(ctx context.Context, branchesToDelete []gitcmd.BranchToDelete, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		// This function runs in a separate goroutine.
		results := gitcmd.DeleteBranches(ctx, branchesToDelete, dryRun)
		return resultsMsg{results: results} // Send the results back to the Update loop
	}
}

// Update handles messages and updates the model accordingly.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd // Command to potentially return

	switch msg := msg.(type) {

	// Handle results message
	case resultsMsg:
		m.results = msg.results
		m.viewState = stateResults
		return m, nil // Stop spinner implicitly by not returning Tick

	// Handle spinner tick
	case spinner.TickMsg:
		// Only update spinner if we are in the deleting state
		if m.viewState == stateDeleting {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil


	// Handle key presses
	case tea.KeyMsg:
		// Global quit keys
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.viewState {

		// --- Selecting State ---
		case stateSelecting:
			totalItems := len(m.listOrder)
			if totalItems == 0 { // Handle case with no candidates
				if msg.String() == "q" { return m, tea.Quit }
				return m, nil
			}

			switch msg.String() {
			case "q":
				return m, tea.Quit

			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < totalItems-1 {
					m.cursor++
				}

			case " ": // Toggle local selection
				originalIndex := m.listOrder[m.cursor] // Get original index from display order
				_, exists := m.selectedLocal[originalIndex]
				if exists {
					delete(m.selectedLocal, originalIndex)
					delete(m.selectedRemote, originalIndex) // Also deselect remote
				} else {
					m.selectedLocal[originalIndex] = true
				}

			case "tab", "r": // Toggle remote selection
				originalIndex := m.listOrder[m.cursor]
				if _, localSelected := m.selectedLocal[originalIndex]; localSelected {
					branch := m.originalCandidates[originalIndex] // Get branch details from original list
					if branch.Remote != "" {
						_, remoteSelected := m.selectedRemote[originalIndex]
						if remoteSelected {
							delete(m.selectedRemote, originalIndex)
						} else {
							m.selectedRemote[originalIndex] = true
						}
					}
				}

			case "enter":
				if len(m.selectedLocal) > 0 || len(m.selectedRemote) > 0 {
					m.viewState = stateConfirming
				}
				return m, nil
			}

		// --- Confirming State ---
		case stateConfirming:
			switch msg.String() {
			case "q", "n", "N", "esc": // Cancel confirmation
				m.viewState = stateSelecting
				return m, nil

			case "y", "Y": // Confirm deletion
				m.viewState = stateDeleting // Switch to deleting state
				branchesToDelete := m.GetBranchesToDelete()
				return m, tea.Batch(
					performDeletionCmd(m.ctx, branchesToDelete, m.dryRun),
					m.spinner.Tick,
				)
			}

		// --- Deleting State ---
		case stateDeleting:
			// Ignore key presses while deleting, wait for resultsMsg
			return m, nil

		// --- Results State ---
		case stateResults:
			// Any key exits from results view
			return m, tea.Quit
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

	// --- Selecting View ---
	case stateSelecting:
		title := "Select branches to delete (Space: local, Tab/r: remote):"
		if m.dryRun {
			title = warningStyle.Render("[Dry Run] ") + title
		}
		// Add concise note about remote selection dependency here
		title += helpStyle.Render(" (Tab/r requires local selection first)")
		b.WriteString(title + "\n\n")


		itemIndex := 0 // Tracks the overall item index for cursor comparison

		// Render Merged Branches
		if len(m.mergedBranches) > 0 {
			b.WriteString(headingStyle.Render("Merged Branches (Safe to delete):") + "\n")
			for _, branch := range m.mergedBranches {
				originalIndex := -1
				// Find original index (less efficient, consider storing it in AnalyzedBranch if needed often)
				for idx, orig := range m.originalCandidates {
					if orig.CommitHash == branch.CommitHash && orig.Name == branch.Name { // Use Hash+Name as unique ID
						originalIndex = idx
						break
					}
				}
				if originalIndex == -1 { continue } // Should not happen

				cursor := " "
				if m.cursor == itemIndex { cursor = cursorStyle.Render(">") }

				localChecked := "[ ]"
				if _, ok := m.selectedLocal[originalIndex]; ok { localChecked = selectedStyle.Render("[x]") }

				remoteChecked := "[ ]"; remoteInfo := "(none)"
				if branch.Remote != "" {
					remoteInfo = fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name)
					if _, ok := m.selectedRemote[originalIndex]; ok { remoteChecked = selectedStyle.Render("[x]") }
				} else { remoteChecked = helpStyle.Render("[-]") }

				line := fmt.Sprintf("Local: %s %s | Remote: %s %s", localChecked, branch.Name, remoteChecked, remoteInfo)
				if m.cursor == itemIndex { b.WriteString(cursor + " " + selectedStyle.Render(line) + "\n") } else { b.WriteString(cursor + " " + line + "\n") }
				itemIndex++
			}
			b.WriteString("\n") // Add space between groups
		}

		// Render Unmerged Branches
		if len(m.unmergedBranches) > 0 {
			b.WriteString(headingStyle.Render("Unmerged Old Branches (Requires force delete -D):") + "\n")
			for _, branch := range m.unmergedBranches {
				originalIndex := -1
				for idx, orig := range m.originalCandidates {
					if orig.CommitHash == branch.CommitHash && orig.Name == branch.Name {
						originalIndex = idx
						break
					}
				}
				if originalIndex == -1 { continue }

				cursor := " "
				if m.cursor == itemIndex { cursor = cursorStyle.Render(">") }

				localChecked := "[ ]"
				if _, ok := m.selectedLocal[originalIndex]; ok { localChecked = selectedStyle.Render("[x]") }

				remoteChecked := "[ ]"; remoteInfo := "(none)"
				if branch.Remote != "" {
					remoteInfo = fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name)
					if _, ok := m.selectedRemote[originalIndex]; ok { remoteChecked = selectedStyle.Render("[x]") }
				} else { remoteChecked = helpStyle.Render("[-]") }

				line := fmt.Sprintf("Local: %s %s | Remote: %s %s", localChecked, branch.Name, remoteChecked, remoteInfo)
				// Apply warning style to unmerged lines
				line = warningStyle.Render(line)
				if m.cursor == itemIndex { b.WriteString(cursor + " " + selectedStyle.Render(line) + "\n") } else { b.WriteString(cursor + " " + line + "\n") }
				itemIndex++
			}
			b.WriteString("\n")
		}

		if itemIndex == 0 { // If no candidates were rendered
			b.WriteString(helpStyle.Render("No candidate branches found for cleanup.") + "\n")
		}

		help := "\nEnter: Confirm | q/Ctrl+C: Quit\n" +
			"Note: Remote branch (Tab/r) can only be selected if local branch (Space) is also selected."
		b.WriteString(helpStyle.Render(help))

	// --- Confirming View ---
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

	// --- Deleting View ---
	case stateDeleting:
		b.WriteString(m.spinner.View())
		b.WriteString(" Processing deletions...")
		if m.dryRun { b.WriteString(warningStyle.Render(" (Dry Run)")) }

	// --- Results View ---
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
	// Iterate through the original selection maps which use original indices
	for originalIndex := range m.selectedLocal {
		branchInfo := m.originalCandidates[originalIndex]
		branches = append(branches, gitcmd.BranchToDelete{
			Name:     branchInfo.Name,
			IsRemote: false,
			Remote:   "",
			IsMerged: branchInfo.IsMerged,
			Hash:     branchInfo.CommitHash,
		})
	}
	for originalIndex := range m.selectedRemote {
		branchInfo := m.originalCandidates[originalIndex]
		if branchInfo.Remote != "" { // Ensure remote exists
			branches = append(branches, gitcmd.BranchToDelete{
				Name:     branchInfo.Name,
				IsRemote: true,
				Remote:   branchInfo.Remote,
				IsMerged: branchInfo.IsMerged,
				Hash:     branchInfo.CommitHash,
			})
		}
	}
	// Note: This might create duplicate entries if both local and remote are selected,
	// but DeleteBranches handles them as separate operations. Consider sorting or optimizing later if needed.
	return branches
}

// ConfirmedDeletion is less relevant now.
// func (m Model) ConfirmedDeletion() bool { return m.Confirmed }
