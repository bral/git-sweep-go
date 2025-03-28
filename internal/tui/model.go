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
	confirmPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
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
	ctx            context.Context        // Context for git commands
	dryRun         bool                   // Is this a dry run?
	candidates     []types.AnalyzedBranch // Branches to display
	cursor         int                    // Index of the currently selected branch
	selectedLocal  map[int]bool           // Map to track selected branches for local deletion (key is index)
	selectedRemote map[int]bool           // Map to track selected branches for remote deletion (key is index)
	viewState      viewState              // Current view state (selecting, confirming, etc.)
	results        []types.DeleteResult   // Stores the results of deletion attempts
	spinner        spinner.Model          // Spinner model
	// TODO: Add dimensions for layout
}

// InitialModel creates the starting model for the TUI.
func InitialModel(ctx context.Context, candidates []types.AnalyzedBranch, dryRun bool) Model {
	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.Dot // Or choose another spinner type

	return Model{
		ctx:            ctx, // Store context
		dryRun:         dryRun, // Store dryRun flag
		candidates:     candidates,
		selectedLocal:  make(map[int]bool),
		selectedRemote: make(map[int]bool),
		cursor:         0,
		viewState:      stateSelecting,
		spinner:        s,
		// Initialize other fields as needed
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
		// Otherwise, ignore the tick
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
			switch msg.String() {
			case "q":
				return m, tea.Quit

			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.candidates)-1 {
					m.cursor++
				}

			case " ": // Toggle local selection
				_, exists := m.selectedLocal[m.cursor]
				if exists {
					delete(m.selectedLocal, m.cursor)
					delete(m.selectedRemote, m.cursor)
				} else {
					m.selectedLocal[m.cursor] = true
				}

			case "tab", "r": // Toggle remote selection
				if _, localSelected := m.selectedLocal[m.cursor]; localSelected {
					branch := m.candidates[m.cursor]
					if branch.Remote != "" {
						_, remoteSelected := m.selectedRemote[m.cursor]
						if remoteSelected {
							delete(m.selectedRemote, m.cursor)
						} else {
							m.selectedRemote[m.cursor] = true
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
				// Return the command to perform deletions AND the spinner tick command
				return m, tea.Batch(
					performDeletionCmd(m.ctx, branchesToDelete, m.dryRun),
					m.spinner.Tick, // Ensure spinner keeps ticking
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

	// If no other specific command was returned, update the spinner
	// (this handles the case where Update is called for other reasons, e.g., window resize)
	// Only tick if deleting
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
		b.WriteString(helpStyle.Render(title) + "\n\n")


		for i, branch := range m.candidates {
			cursor := " "
			if m.cursor == i {
				cursor = cursorStyle.Render(">")
			}

			localChecked := "[ ]"
			if _, ok := m.selectedLocal[i]; ok {
				localChecked = selectedStyle.Render("[x]")
			}

			remoteChecked := "[ ]"
			remoteInfo := "(none)"
			if branch.Remote != "" {
				remoteInfo = fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name)
				if _, ok := m.selectedRemote[i]; ok {
					remoteChecked = selectedStyle.Render("[x]")
				}
			} else {
				remoteChecked = helpStyle.Render("[-]")
			}

			line := fmt.Sprintf("Local: %s %s | Remote: %s %s (%s)",
				localChecked, branch.Name, remoteChecked, remoteInfo, branch.Category)

			if m.cursor == i {
				b.WriteString(cursor + " " + selectedStyle.Render(line) + "\n")
			} else {
				b.WriteString(cursor + " " + line + "\n")
			}
		}

		b.WriteString(helpStyle.Render("\nEnter: Confirm | q/Ctrl+C: Quit\n"))

	// --- Confirming View ---
	case stateConfirming:
		title := "Confirm Actions:"
		if m.dryRun {
			title = warningStyle.Render("[Dry Run] ") + title
		}
		b.WriteString(title + "\n\n")

		branchesToDelete := m.GetBranchesToDelete()

		if len(branchesToDelete) == 0 {
			b.WriteString("No actions selected.\n")
		} else {
			b.WriteString("Local Deletions:\n")
			hasLocal := false
			for _, bd := range branchesToDelete {
				if !bd.IsRemote {
					delType := "-d (safe)"
					style := lipgloss.NewStyle()
					if !bd.IsMerged {
						delType = "-D (force)"
						style = warningStyle
					}
					b.WriteString(style.Render(fmt.Sprintf("  - Delete '%s' (%s)\n", bd.Name, delType)))
					hasLocal = true
				}
			}
			if !hasLocal { b.WriteString(helpStyle.Render("  (None)\n")) }

			b.WriteString("\nRemote Deletions:\n")
			hasRemote := false
			for _, bd := range branchesToDelete {
				if bd.IsRemote {
					b.WriteString(fmt.Sprintf("  - Delete remote '%s/%s'\n", bd.Remote, bd.Name))
					hasRemote = true
				}
			}
			if !hasRemote { b.WriteString(helpStyle.Render("  (None)\n")) }
		}

		b.WriteString("\n" + confirmPromptStyle.Render("Proceed? (y/N) "))

	// --- Deleting View ---
	case stateDeleting:
		b.WriteString(m.spinner.View()) // Render the spinner
		b.WriteString(" Processing deletions...")
		if m.dryRun {
			b.WriteString(warningStyle.Render(" (Dry Run)"))
		}


	// --- Results View ---
	case stateResults:
		title := "Deletion Results:"
		if m.dryRun {
			title = warningStyle.Render("[Dry Run] ") + title
		}
		b.WriteString(title + "\n\n")

		if len(m.results) > 0 {
			for _, res := range m.results {
				var style lipgloss.Style
				var status string
				if res.Success {
					style = successStyle
					status = "✅ Success"
				} else {
					style = errorStyle
					status = "❌ Failed"
				}
				branchType := "Local"
				if res.IsRemote {
					branchType = fmt.Sprintf("Remote (%s)", res.RemoteName)
				}
				line := fmt.Sprintf("%s: %s %s - %s", status, branchType, res.BranchName, res.Message)
				b.WriteString(style.Render(line) + "\n")
			}
		} else {
			b.WriteString(helpStyle.Render("(No deletion actions were performed or results available)\n"))
		}
		b.WriteString(helpStyle.Render("\nPress any key to exit."))

	}

	// Apply overall margin
	return docStyle.Render(b.String())
}

// GetBranchesToDelete builds the list of actions based on current selections.
func (m Model) GetBranchesToDelete() []gitcmd.BranchToDelete {
	branches := make([]gitcmd.BranchToDelete, 0)
	processedLocal := make(map[int]bool)

	for index := range m.selectedRemote {
		branchInfo := m.candidates[index]
		branches = append(branches, gitcmd.BranchToDelete{
			Name:     branchInfo.Name,
			IsRemote: true,
			Remote:   branchInfo.Remote,
			IsMerged: branchInfo.IsMerged,
			Hash:     branchInfo.CommitHash,
		})
		processedLocal[index] = true
	}

	for index := range m.selectedLocal {
		if !processedLocal[index] {
			branchInfo := m.candidates[index]
			branches = append(branches, gitcmd.BranchToDelete{
				Name:     branchInfo.Name,
				IsRemote: false,
				Remote:   "",
				IsMerged: branchInfo.IsMerged,
				Hash:     branchInfo.CommitHash,
			})
		}
	}
	return branches
}

// ConfirmedDeletion is less relevant now as the TUI handles the full flow.
// func (m Model) ConfirmedDeletion() bool {
// 	return m.Confirmed // Kept for potential future use but likely removable
// }
