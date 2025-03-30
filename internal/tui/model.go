// Package tui implements the interactive terminal user interface using Bubble Tea.
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
	errorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red for errors
	spinnerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("205")) // Spinner color
	forceDeleteStyle   = errorStyle.Bold(true).Reverse(true)                   // Style for force delete warnings
	protectedStyle     = lipgloss.NewStyle().Faint(true)                       // Style for protected branches ONLY
	// Style for active branches (faint, unselectable)
	activeStyle      = helpStyle.Faint(true)
	separatorStyle   = helpStyle.Faint(true) // Style for separator line
	categoryStyleMap = map[types.BranchCategory]lipgloss.Style{
		// Protected category is handled separately (keyBranches)
		types.CategoryActive:      activeStyle,  // Style for the label text only
		types.CategoryMergedOld:   successStyle, // Removed .Copy()
		types.CategoryUnmergedOld: warningStyle, // Removed .Copy()
	}
)

// ViewState represents the different views the TUI can be in.
type ViewState int // Renamed from viewState

const (
	// StateSelecting is the initial state for branch selection.
	StateSelecting ViewState = iota // Renamed from stateSelecting
	// StateConfirming is the state for confirming deletions.
	StateConfirming // Renamed from stateConfirming
	// StateDeleting is the state shown while deletions are in progress.
	StateDeleting // Renamed from stateDeleting
	// StateResults is the state showing the outcome of deletions.
	StateResults // Renamed from stateResults

	// Constants for UI elements (kept internal)
	checkboxUnselectable = "[-]"
	checkboxUnchecked    = "[ ]"
	remoteNone           = "(none)"
)

// --- Messages ---

// resultsMsg carries the deletion results back to the TUI after execution.
// Kept internal as it's only used within the TUI update loop.
type resultsMsg struct {
	results []types.DeleteResult
}

// --- Model ---

// Model represents the state of the TUI application.
type Model struct { // Renamed from model
	Ctx                 context.Context        `json:"-"` // Context for git commands (ignore in JSON if ever needed)
	DryRun              bool                   `json:"dryRun"`
	AllAnalyzedBranches []types.AnalyzedBranch `json:"-"` // Full list (ignore in JSON)
	KeyBranches         []types.AnalyzedBranch `json:"-"` // Protected (ignore in JSON)
	SuggestedBranches   []types.AnalyzedBranch `json:"-"` // Candidates (ignore in JSON)
	OtherActiveBranches []types.AnalyzedBranch `json:"-"` // Active (ignore in JSON)
	ListOrder           []int                  `json:"-"` // Maps display index to original index (ignore in JSON)
	Cursor              int                    `json:"cursor"`
	SelectedLocal       map[int]bool           `json:"selectedLocal"`  // Map using original index
	SelectedRemote      map[int]bool           `json:"selectedRemote"` // Map using original index
	ViewState           ViewState              `json:"viewState"`      // Renamed from viewState
	Results             []types.DeleteResult   `json:"results"`
	Spinner             spinner.Model          `json:"-"` // Spinner model (ignore in JSON)
	Width               int                    `json:"width"`
	Height              int                    `json:"height"`
}

// InitialModel creates the starting model for the TUI, separating branches into three groups.
func InitialModel(
	ctx context.Context,
	analyzedBranches []types.AnalyzedBranch,
	dryRun bool,
) Model { // Renamed from initialModel
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
		Ctx:                 ctx,
		DryRun:              dryRun,
		AllAnalyzedBranches: analyzedBranches, // Keep original full list
		KeyBranches:         key,
		SuggestedBranches:   suggested,
		OtherActiveBranches: active,
		ListOrder:           order,              // Store the display order mapping
		SelectedLocal:       make(map[int]bool), // Key is original index
		SelectedRemote:      make(map[int]bool), // Key is original index
		Cursor:              0,
		ViewState:           StateSelecting, // Renamed from stateSelecting
		Spinner:             s,
	}
}

// Init is the first command that runs when the Bubble Tea program starts.
func (m Model) Init() tea.Cmd {
	return m.Spinner.Tick // Start the spinner ticking
}

// performDeletionCmd is a tea.Cmd that executes the branch deletions.
// Kept internal as it's only used within the TUI update loop.
func performDeletionCmd(ctx context.Context, branchesToDelete []gitcmd.BranchToDelete, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		results := gitcmd.DeleteBranches(ctx, branchesToDelete, dryRun)
		return resultsMsg{results: results}
	}
}

// isSelectable checks if the branch at the given *original* index can be selected.
// Kept internal as it's only used within the TUI update loop.
func (m Model) isSelectable(originalIndex int) bool {
	if originalIndex < 0 || originalIndex >= len(m.AllAnalyzedBranches) {
		return false
	}
	category := m.AllAnalyzedBranches[originalIndex].Category
	// Only allow selecting MergedOld and UnmergedOld (original candidates)
	return category == types.CategoryMergedOld || category == types.CategoryUnmergedOld
}

// --- Update Logic ---

// Update handles messages and updates the model accordingly.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case resultsMsg: // Internal message type
		m.Results = msg.results
		m.ViewState = StateResults
		return m, nil

	case spinner.TickMsg:
		// Only update spinner if in deleting state
		if m.ViewState == StateDeleting {
			m.Spinner, cmd = m.Spinner.Update(msg)
			return m, cmd
		}
		return m, nil // Ignore spinner ticks in other states

	case tea.KeyMsg:
		// Global Quit
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Delegate key handling based on state
		switch m.ViewState {
		case StateSelecting:
			return m.updateSelecting(msg)
		case StateConfirming:
			return m.updateConfirming(msg)
		case StateDeleting:
			return m.updateDeleting(msg)
		case StateResults:
			return m.updateResults(msg)
		}
	}

	return m, nil
}

// updateSelecting handles key presses when in the selecting state.
func (m Model) updateSelecting(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalItems := len(m.ListOrder)
	if totalItems == 0 {
		if msg.String() == "q" {
			return m, tea.Quit
		}
		return m, nil // Ignore other keys if list is empty
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit

	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < totalItems-1 {
			m.Cursor++
		}

	case " ": // Toggle local selection
		if m.Cursor >= len(m.ListOrder) {
			break // Bounds check
		}
		originalIndex := m.ListOrder[m.Cursor]
		if m.isSelectable(originalIndex) {
			_, exists := m.SelectedLocal[originalIndex]
			if exists {
				delete(m.SelectedLocal, originalIndex)
				delete(m.SelectedRemote, originalIndex) // Also deselect remote
			} else {
				m.SelectedLocal[originalIndex] = true
			}
		}

	case "tab", "r": // Toggle remote selection
		if m.Cursor >= len(m.ListOrder) {
			break // Bounds check
		}
		originalIndex := m.ListOrder[m.Cursor]
		if m.isSelectable(originalIndex) {
			if _, localSelected := m.SelectedLocal[originalIndex]; localSelected {
				branch := m.AllAnalyzedBranches[originalIndex]
				if branch.Remote != "" {
					_, remoteSelected := m.SelectedRemote[originalIndex]
					if remoteSelected {
						delete(m.SelectedRemote, originalIndex)
					} else {
						m.SelectedRemote[originalIndex] = true
					}
				}
			}
		}

	case "enter":
		if len(m.SelectedLocal) > 0 || len(m.SelectedRemote) > 0 {
			m.ViewState = StateConfirming
		}
		return m, nil // No command needed here
	}

	return m, nil
}

// updateConfirming handles key presses when in the confirming state.
func (m Model) updateConfirming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "n", "N", "esc":
		m.ViewState = StateSelecting
		return m, nil
	case "y", "Y":
		m.ViewState = StateDeleting
		branchesToDelete := m.GetBranchesToDelete()
		return m, tea.Batch(
			performDeletionCmd(m.Ctx, branchesToDelete, m.DryRun),
			m.Spinner.Tick, // Ensure spinner keeps ticking
		)
	}
	return m, nil
}

// updateDeleting handles key presses when in the deleting state (currently ignores them).
func (m Model) updateDeleting(_ tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ignore key presses while deleting
	return m, nil
}

// updateResults handles key presses when in the results state (any key quits).
func (m Model) updateResults(_ tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Any key press quits
	return m, tea.Quit
}

// --- View Helper Functions ---

// renderKeyBranches renders the non-selectable key branches (Protected, Current).
// Kept internal as it's only called by View.
func (m Model) renderKeyBranches(b *strings.Builder, itemIndex *int) {
	for _, branch := range m.KeyBranches {
		cursor := " "
		if m.Cursor == *itemIndex {
			cursor = cursorStyle.Render(">")
		}

		localCheckbox := checkboxUnselectable // Never selectable
		remoteCheckbox := checkboxUnselectable
		lineStyle := protectedStyle

		remoteInfo := remoteNone
		if branch.Remote != "" {
			remoteInfo = fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name)
		}

		categoryText := protectedStyle.Render("(Protected)")
		if branch.IsCurrent {
			categoryText = protectedStyle.Render("(Current)")
		}

		line := fmt.Sprintf("Local: %s %s | Remote: %s %s %s",
			localCheckbox, branch.Name, remoteCheckbox, remoteInfo, categoryText)

		b.WriteString(cursor + " " + lineStyle.Render(line) + "\n")
		*itemIndex++ // Increment the shared index
	}
}

// renderSuggestedBranches renders the selectable suggested branches (MergedOld, UnmergedOld).
// Kept internal as it's only called by View.
func (m Model) renderSuggestedBranches(b *strings.Builder, itemIndex *int) {
	for _, branch := range m.SuggestedBranches {
		var originalIndex int // Declare variable, will be assigned below
		// Find original index by iterating through listOrder which maps display index to original index
		displayIndex := *itemIndex // The current display index corresponds to this branch
		if displayIndex < len(m.ListOrder) {
			originalIndex = m.ListOrder[displayIndex] // Assign value here
		} else {
			continue // Should not happen if listOrder is correct
		}
		if originalIndex < 0 || originalIndex >= len(m.AllAnalyzedBranches) {
			continue // Safety check
		}

		cursor := " "
		if m.Cursor == *itemIndex {
			cursor = cursorStyle.Render(">")
		}

		// These are always selectable
		localCheckbox := checkboxUnchecked // Default to unchecked
		if _, ok := m.SelectedLocal[originalIndex]; ok {
			localCheckbox = selectedStyle.Render("[x]")
		}

		remoteCheckbox := checkboxUnselectable
		remoteInfo := remoteNone
		if branch.Remote != "" {
			remoteCheckbox = checkboxUnchecked
			remoteInfo = fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name)
			if _, ok := m.SelectedRemote[originalIndex]; ok {
				remoteCheckbox = selectedStyle.Render("[x]")
			}
		}

		categoryStyle := categoryStyleMap[branch.Category]
		categoryText := categoryStyle.Render("(" + string(branch.Category) + ")")

		line := fmt.Sprintf("Local: %s %s | Remote: %s %s %s",
			localCheckbox, branch.Name, remoteCheckbox, remoteInfo, categoryText)

		// Apply styling based on cursor and category
		if m.Cursor == *itemIndex {
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
		*itemIndex++ // Increment the shared index
	}
}

// renderOtherActiveBranches renders the non-selectable active branches.
// Kept internal as it's only called by View.
func (m Model) renderOtherActiveBranches(b *strings.Builder, itemIndex *int) {
	for _, branch := range m.OtherActiveBranches {
		cursor := " "
		if m.Cursor == *itemIndex {
			cursor = cursorStyle.Render(">")
		}

		// These are never selectable
		localCheckbox := checkboxUnselectable
		remoteCheckbox := checkboxUnselectable
		lineStyle := activeStyle // Use faint style

		remoteInfo := remoteNone
		if branch.Remote != "" {
			remoteInfo = fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name)
		}

		categoryText := activeStyle.Render("(" + string(branch.Category) + ")")

		line := fmt.Sprintf("Local: %s %s | Remote: %s %s %s",
			localCheckbox, branch.Name, remoteCheckbox, remoteInfo, categoryText)

		b.WriteString(cursor + " " + lineStyle.Render(line) + "\n")
		*itemIndex++ // Increment the shared index
	}
}

// renderSelectingState renders the branch selection view
func (m Model) renderSelectingState(b *strings.Builder) {
	title := "Branches (Space: select local, Tab/r: select remote):"
	if m.DryRun {
		title = warningStyle.Render("[Dry Run] ") + title
	}
	title += helpStyle.Render(" (Remote requires local)")
	b.WriteString(title + "\n\n")

	itemIndex := 0 // Tracks the overall item index for cursor comparison

	// --- Render Key Branches ---
	m.renderKeyBranches(b, &itemIndex)

	// --- Separator and Header for Suggested branches ---
	hasSuggestions := len(m.SuggestedBranches) > 0
	hasActive := len(m.OtherActiveBranches) > 0
	hasKeys := len(m.KeyBranches) > 0

	if hasKeys && (hasSuggestions || hasActive) {
		// Add separator only if key branches exist AND others exist
		b.WriteString(separatorStyle.Render("---") + "\n")
	}
	if hasSuggestions {
		b.WriteString(headingStyle.Render("Suggested Branches (Candidates):") + "\n")
		m.renderSuggestedBranches(b, &itemIndex)
	}

	// --- Separator and Header for Other Active branches ---
	if hasSuggestions && hasActive {
		// Add separator only if suggested branches exist AND active branches exist
		b.WriteString(separatorStyle.Render("---") + "\n")
	}
	if hasActive {
		b.WriteString(headingStyle.Render("Other Branches (Active / Not Selectable):") + "\n")
		m.renderOtherActiveBranches(b, &itemIndex)
	}

	if itemIndex == 0 { // If no branches were rendered at all
		b.WriteString(helpStyle.Render("No branches found to display.") + "\n")
	}

	// Add selection summary to footer
	footer := fmt.Sprintf("\nSelected: %d local, %d remote | Enter: Confirm | q/Ctrl+C: Quit\n",
		len(m.SelectedLocal), len(m.SelectedRemote))
	b.WriteString(helpStyle.Render(footer))
}

// renderConfirmingState renders the confirmation view
func (m Model) renderConfirmingState(b *strings.Builder) {
	title := "Confirm Actions:"
	if m.DryRun {
		title = warningStyle.Render("[Dry Run] ") + title
	}
	b.WriteString(title + "\n\n")
	branchesToDelete := m.GetBranchesToDelete()
	hasForceDeletes := false

	if len(branchesToDelete) == 0 {
		b.WriteString("No actions selected.\n")
	} else {
		b.WriteString("Local Deletions:\n")
		hasLocal := false
		for _, bd := range branchesToDelete {
			if !bd.IsRemote {
				style := lipgloss.NewStyle()
				delType := "-d (safe)"
				if !bd.IsMerged {
					style = forceDeleteStyle
					delType = "-D (FORCE)"
					hasForceDeletes = true
				}
				b.WriteString(style.Render(fmt.Sprintf("  - Delete '%s' (%s)\n", bd.Name, delType)))
				hasLocal = true
			}
		}
		if !hasLocal {
			b.WriteString(helpStyle.Render("  (None)\n"))
		}

		b.WriteString("\nRemote Deletions:\n")
		hasRemote := false
		for _, bd := range branchesToDelete {
			if bd.IsRemote {
				fmt.Fprintf(b, "  - Delete remote '%s/%s'\n", bd.Remote, bd.Name)
				hasRemote = true
			}
		}
		if !hasRemote {
			b.WriteString(helpStyle.Render("  (None)\n"))
		}
	}

	if hasForceDeletes {
		b.WriteString("\n" + warningStyle.Render(
			"WARNING: Branches marked with '-D (FORCE)' contain unmerged work and will be permanently lost!") + "\n")
	}

	b.WriteString("\n" + confirmPromptStyle.Render("Proceed? (y/N) "))
}

// renderDeletingState renders the deletion in progress view
func (m Model) renderDeletingState(b *strings.Builder) {
	b.WriteString(m.Spinner.View())
	b.WriteString(" Processing deletions...")
	if m.DryRun {
		b.WriteString(warningStyle.Render(" (Dry Run)"))
	}
}

// renderResultsState renders the results view
func (m Model) renderResultsState(b *strings.Builder) {
	title := "Deletion Results:"
	if m.DryRun {
		title = warningStyle.Render("[Dry Run] ") + title
	}
	b.WriteString(title + "\n\n")
	if len(m.Results) > 0 {
		for _, res := range m.Results {
			style := successStyle
			status := "✅ Success"
			if !res.Success {
				style = errorStyle
				status = "❌ Failed"
			}
			branchType := "Local"
			if res.IsRemote {
				branchType = fmt.Sprintf("Remote (%s)", res.RemoteName)
			}
			hashInfo := ""
			if res.Success && res.DeletedHash != "" {
				hashInfo = fmt.Sprintf(" (was %s)", res.DeletedHash)
			}
			line := fmt.Sprintf("%s: %s %s%s - %s", status, branchType, res.BranchName, hashInfo, res.Message)
			b.WriteString(style.Render(line) + "\n")
		}
	} else {
		b.WriteString(helpStyle.Render("(No deletion actions were performed or results available)\n"))
	}
	b.WriteString(helpStyle.Render("\nPress any key to exit."))
}

// View renders the UI based on the model's state.
func (m Model) View() string {
	var b strings.Builder

	switch m.ViewState {
	case StateSelecting:
		m.renderSelectingState(&b)
	case StateConfirming:
		m.renderConfirmingState(&b)
	case StateDeleting:
		m.renderDeletingState(&b)
	case StateResults:
		m.renderResultsState(&b)
	}

	return docStyle.Render(b.String())
}

// GetBranchesToDelete builds the list of actions based on current selections using original indices.
// Kept internal as it's only called by View and Update.
func (m Model) GetBranchesToDelete() []gitcmd.BranchToDelete {
	branches := make([]gitcmd.BranchToDelete, 0)
	// Iterate through the selection maps which use original indices
	for originalIndex := range m.SelectedLocal {
		if originalIndex < 0 || originalIndex >= len(m.AllAnalyzedBranches) {
			continue
		}
		branchInfo := m.AllAnalyzedBranches[originalIndex]
		// Check if it's selectable before adding
		if m.isSelectable(originalIndex) {
			branches = append(branches, gitcmd.BranchToDelete{
				Name: branchInfo.Name, IsRemote: false, Remote: "", IsMerged: branchInfo.IsMerged, Hash: branchInfo.CommitHash,
			})
		}
	}
	for originalIndex := range m.SelectedRemote {
		if originalIndex < 0 || originalIndex >= len(m.AllAnalyzedBranches) {
			continue
		}
		branchInfo := m.AllAnalyzedBranches[originalIndex]
		// Check if it's selectable and has a remote before adding
		if m.isSelectable(originalIndex) && branchInfo.Remote != "" {
			branches = append(branches, gitcmd.BranchToDelete{
				Name:     branchInfo.Name,
				IsRemote: true,
				Remote:   branchInfo.Remote,
				IsMerged: branchInfo.IsMerged,
				Hash:     branchInfo.CommitHash,
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
