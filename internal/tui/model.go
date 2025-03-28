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
	// headingStyle       = lipgloss.NewStyle().Bold(true).Underline(true).MarginBottom(1) // No longer needed
	confirmPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	warningStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("202")) // Orange/Red for warnings
	successStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))  // Green for success
	errorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))  // Red for errors
	spinnerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("205")) // Spinner color
	protectedStyle     = lipgloss.NewStyle().Faint(true)                         // Style for protected/active branches
	categoryStyleMap   = map[types.BranchCategory]lipgloss.Style{
		types.CategoryProtected:   helpStyle.Copy().Faint(true),
		types.CategoryActive:      helpStyle.Copy().Faint(true),
		types.CategoryMergedOld:   successStyle.Copy(), // Use success style for safe-to-delete merged
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
	allAnalyzedBranches []types.AnalyzedBranch // Full list of analyzed branches
	cursor            int                    // Index in the allAnalyzedBranches list
	selectedLocal     map[int]bool           // Map using direct index as key
	selectedRemote    map[int]bool           // Map using direct index as key
	viewState         viewState              // Current view state (selecting, confirming, etc.)
	results           []types.DeleteResult   // Stores the results of deletion attempts
	spinner           spinner.Model          // Spinner model
	// TODO: Add dimensions for layout
}

// InitialModel creates the starting model for the TUI, using all analyzed branches.
func InitialModel(ctx context.Context, analyzedBranches []types.AnalyzedBranch, dryRun bool) Model {
	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.Dot

	return Model{
		ctx:               ctx,
		dryRun:            dryRun,
		allAnalyzedBranches: analyzedBranches, // Store the full list
		selectedLocal:     make(map[int]bool), // Key is direct index
		selectedRemote:    make(map[int]bool), // Key is direct index
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

// isSelectable checks if the branch at the given index is a candidate for deletion.
func (m Model) isSelectable(index int) bool {
	if index < 0 || index >= len(m.allAnalyzedBranches) {
		return false
	}
	category := m.allAnalyzedBranches[index].Category
	return category == types.CategoryMergedOld || category == types.CategoryUnmergedOld
}

// Update handles messages and updates the model accordingly.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

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
			totalItems := len(m.allAnalyzedBranches)
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

			case " ": // Toggle local selection only if selectable
				if m.isSelectable(m.cursor) {
					_, exists := m.selectedLocal[m.cursor]
					if exists {
						delete(m.selectedLocal, m.cursor)
						delete(m.selectedRemote, m.cursor) // Also deselect remote
					} else {
						m.selectedLocal[m.cursor] = true
					}
				}

			case "tab", "r": // Toggle remote selection only if selectable and local is selected
				if m.isSelectable(m.cursor) {
					if _, localSelected := m.selectedLocal[m.cursor]; localSelected {
						branch := m.allAnalyzedBranches[m.cursor]
						if branch.Remote != "" {
							_, remoteSelected := m.selectedRemote[m.cursor]
							if remoteSelected {
								delete(m.selectedRemote, m.cursor)
							} else {
								m.selectedRemote[m.cursor] = true
							}
						}
					}
				}

			case "enter":
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
				branchesToDelete := m.GetBranchesToDelete()
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

		if len(m.allAnalyzedBranches) == 0 {
			b.WriteString(helpStyle.Render("No branches found.") + "\n")
		} else {
			for i, branch := range m.allAnalyzedBranches {
				cursor := " "
				if m.cursor == i { cursor = cursorStyle.Render(">") }

				isCandidate := m.isSelectable(i)
				lineStyle := lipgloss.NewStyle() // Default style
				localCheckbox := "[-]" // Default disabled checkbox
				remoteCheckbox := "[-]"

				if isCandidate {
					localCheckbox = "[ ]" // Enable checkbox
					if _, ok := m.selectedLocal[i]; ok { localCheckbox = selectedStyle.Render("[x]") }

					if branch.Remote != "" {
						remoteCheckbox = "[ ]" // Enable remote checkbox
						if _, ok := m.selectedRemote[i]; ok { remoteCheckbox = selectedStyle.Render("[x]") }
					}
				} else {
					lineStyle = protectedStyle // Apply faint style to non-candidates
				}

				remoteInfo := "(none)"
				if branch.Remote != "" { remoteInfo = fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name) }

				categoryStyle := categoryStyleMap[branch.Category]
				line := fmt.Sprintf("Local: %s %s | Remote: %s %s %s",
					localCheckbox, branch.Name, remoteCheckbox, remoteInfo, categoryStyle.Render("("+string(branch.Category)+")"))

				// Apply faint style if not a candidate, otherwise apply selection style if cursor is on it
				if !isCandidate {
					b.WriteString(cursor + " " + lineStyle.Render(line) + "\n")
				} else if m.cursor == i {
					b.WriteString(cursor + " " + selectedStyle.Render(line) + "\n")
				} else {
					b.WriteString(cursor + " " + line + "\n")
				}
			}
		}

		b.WriteString(helpStyle.Render("\nEnter: Confirm | q/Ctrl+C: Quit\n"))

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

// GetBranchesToDelete builds the list of actions based on current selections using direct indices.
func (m Model) GetBranchesToDelete() []gitcmd.BranchToDelete {
	branches := make([]gitcmd.BranchToDelete, 0)
	// Iterate through the selection maps which use direct indices
	for index := range m.selectedLocal {
		if index < 0 || index >= len(m.allAnalyzedBranches) { continue } // Bounds check
		branchInfo := m.allAnalyzedBranches[index]
		// Double check it's actually a candidate before adding
		if branchInfo.Category == types.CategoryMergedOld || branchInfo.Category == types.CategoryUnmergedOld {
			branches = append(branches, gitcmd.BranchToDelete{
				Name:     branchInfo.Name, IsRemote: false, Remote: "", IsMerged: branchInfo.IsMerged, Hash: branchInfo.CommitHash,
			})
		}
	}
	for index := range m.selectedRemote {
		if index < 0 || index >= len(m.allAnalyzedBranches) { continue } // Bounds check
		branchInfo := m.allAnalyzedBranches[index]
		// Double check it's actually a candidate and has a remote
		if (branchInfo.Category == types.CategoryMergedOld || branchInfo.Category == types.CategoryUnmergedOld) && branchInfo.Remote != "" {
			branches = append(branches, gitcmd.BranchToDelete{
				Name:     branchInfo.Name, IsRemote: true, Remote:   branchInfo.Remote, IsMerged: branchInfo.IsMerged, Hash:     branchInfo.CommitHash,
			})
		}
	}
	return branches
}
