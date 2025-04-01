// Package tui implements the interactive terminal user interface using Bubble Tea.
package tui

import (
	"context" // Added for deletion context
	"fmt"

	// "os"      // Removed debug logging import
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
	activeStyle    = helpStyle.Faint(true)
	separatorStyle = helpStyle.Faint(true) // Style for separator line
	// New styles for remote selection states
	remoteDimmedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Faint(true) // Dimmed style for unavailable remotes
	remoteNoneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Faint(true).
			Italic(true) // Style for non-existent remotes

	// Progress indicator styles
	progressStyle       = helpStyle
	progressMarkerStyle = selectedStyle
	progressInfoStyle   = helpStyle
	categoryStyleMap    = map[types.BranchCategory]lipgloss.Style{
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

// --- Section Types ---

// Section represents a logical section of branches in the UI
type Section int

const (
	// SectionKey represents the protected/key branches section
	SectionKey Section = iota
	// SectionSuggested represents the suggested branches section
	SectionSuggested
	// SectionOther represents the other active branches section
	SectionOther
)

// ViewportState tracks scrolling state for a specific section
type ViewportState struct {
	Start int // First visible item index
	Size  int // Number of visible items
	Total int // Total items in section
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

	// Viewport management
	Viewports      map[Section]ViewportState `json:"-"` // Viewport state for each section
	CurrentSection Section                   `json:"-"` // Currently active section
}

// Helper function to render the compact progress indicator
func renderCompactIndicator(start, viewportSize, total int, width int) string {
	// Handle case where everything fits
	if total <= viewportSize {
		return progressInfoStyle.Render("All branches visible")
	}

	// Calculate position (ensure we don't divide by zero)
	maxStart := max(0, total-viewportSize)
	position := float64(start) / float64(maxStart)

	// Numerical portion
	nums := fmt.Sprintf("%d-%d/%d",
		start+1,
		min(start+viewportSize, total),
		total)

	// Visual bar portion - use a fixed width for the bar
	barWidth := 10
	markerPos := int(position * float64(barWidth))

	bar := "["
	for i := 0; i < barWidth; i++ {
		if i == markerPos {
			bar += progressMarkerStyle.Render("⬤") // Position marker with emphasis
		} else {
			bar += "·" // Bar dots
		}
	}
	bar += "]"

	// Percentage
	percentage := fmt.Sprintf("(%d%%)", int(position*100))

	// Help text that fits remaining space
	helpText := " | PgUp/PgDn to scroll"

	// Check if we have room for Home/End text
	if width >= len(nums)+len(bar)+len(percentage)+len(helpText)+20 {
		helpText += " | Home/End to jump"
	}

	return progressStyle.Render(nums+" "+bar+" "+percentage) +
		progressInfoStyle.Render(helpText)
}

// Helper function to get the section for a branch
func (m Model) getBranchSection(originalIndex int) Section {
	if originalIndex < 0 || originalIndex >= len(m.AllAnalyzedBranches) {
		return SectionSuggested // Default
	}

	branch := m.AllAnalyzedBranches[originalIndex]

	switch branch.Category {
	case types.CategoryProtected:
		return SectionKey
	case types.CategoryActive:
		return SectionOther
	case types.CategoryMergedOld, types.CategoryUnmergedOld:
		return SectionSuggested
	default:
		return SectionSuggested // Fallback for any future categories
	}
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

	// Initialize viewports for each section
	viewports := map[Section]ViewportState{
		SectionKey: {
			Start: 0,
			Size:  len(key),
			Total: len(key),
		},
		SectionSuggested: {
			Start: 0,
			Size:  len(suggested), // Initial size, will be adjusted by WindowSizeMsg
			Total: len(suggested),
		},
		SectionOther: {
			Start: 0,
			Size:  len(active),
			Total: len(active),
		},
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
		Viewports:           viewports,
		CurrentSection:      SectionSuggested, // Default to suggested section
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

		// Estimate fixed heights: Header (3 lines), Footer (2 lines), Section Headers (1 line each = 3), Spacing (3 lines)
		// Total fixed = 3 + 2 + 3 + 3 = 11 lines (Adjust estimate as needed)
		// Add 1 line per scroll indicator if sections are scrollable (max 2 extra lines)
		fixedHeightEstimate := 11
		// Check totals before accessing viewports, as they might be nil initially
		suggestedTotal := len(m.SuggestedBranches)
		otherTotal := len(m.OtherActiveBranches)
		if suggestedTotal > 0 { // Assume suggested might scroll if it exists
			fixedHeightEstimate++
		}
		if otherTotal > 0 { // Assume other might scroll if it exists
			fixedHeightEstimate++
		}

		availableHeight := max(3, m.Height-fixedHeightEstimate) // Min 1 line per section

		// Allocate space:
		// 1. Key Branches: Fixed max height (e.g., 5 lines)
		keyHeight := min(len(m.KeyBranches), 5)
		// remainingHeight := availableHeight - keyHeight // Removed, replaced by remainingHeightAfterMins logic

		// 2. Allocate remaining height: Guarantee minimums first, then distribute.
		suggestedHeight := 0
		otherHeight := 0

		// Guarantee minimums if items exist
		if suggestedTotal > 0 {
			suggestedHeight = 1
		}
		if otherTotal > 0 {
			otherHeight = 1
		}

		// Calculate height remaining *after* guaranteeing minimums (and keyHeight)
		remainingHeightAfterMins := availableHeight - keyHeight - suggestedHeight - otherHeight

		// Distribute positive remaining height, prioritizing suggested
		if remainingHeightAfterMins > 0 {
			suggestedMaxHeightPref := 10 // Max preferred lines for suggested (including the guaranteed 1)

			// How much more suggested *needs* (beyond the guaranteed 1)
			neededSuggested := max(0, suggestedTotal-suggestedHeight)
			// How much more suggested *can* get based on preference (beyond the guaranteed 1)
			preferredSuggested := max(0, suggestedMaxHeightPref-suggestedHeight)

			// How much to actually add to suggested
			addSuggested := min(remainingHeightAfterMins, min(neededSuggested, preferredSuggested))

			suggestedHeight += addSuggested
			remainingHeightAfterMins -= addSuggested

			// Give the rest to other
			if remainingHeightAfterMins > 0 {
				neededOther := max(0, otherTotal-otherHeight) // How much more other needs
				addOther := min(remainingHeightAfterMins, neededOther)
				otherHeight += addOther
				// remainingHeightAfterMins -= addOther // No need to track further
			}
		}

		// Final cap: ensure height doesn't exceed total items (might be redundant now, but safe)
		suggestedHeight = min(suggestedHeight, suggestedTotal)
		otherHeight = min(otherHeight, otherTotal)

		// Debug logging removed

		// Update viewport sizes and totals
		if m.Viewports == nil {
			m.Viewports = make(map[Section]ViewportState)
		}

		// --- Key Viewport ---
		keyViewport := m.Viewports[SectionKey] // Get existing or zero value
		keyViewport.Size = keyHeight           // Set calculated size
		keyViewport.Total = len(m.KeyBranches) // Update total
		// Ensure Start is valid after resize
		keyViewport.Start = min(keyViewport.Start, max(0, keyViewport.Total-keyViewport.Size))
		m.Viewports[SectionKey] = keyViewport

		// --- Suggested Viewport ---
		suggestedViewport := m.Viewports[SectionSuggested] // Get existing or zero value
		suggestedViewport.Size = suggestedHeight           // Set calculated size
		suggestedViewport.Total = suggestedTotal           // Update total
		// Ensure Start is valid after resize
		suggestedViewport.Start = min(suggestedViewport.Start, max(0, suggestedViewport.Total-suggestedViewport.Size))
		m.Viewports[SectionSuggested] = suggestedViewport

		// --- Other Viewport ---
		otherViewport := m.Viewports[SectionOther] // Get existing or zero value
		otherViewport.Size = otherHeight           // Set calculated size
		otherViewport.Total = otherTotal           // Update total
		// Ensure Start is valid after resize
		otherViewport.Start = min(otherViewport.Start, max(0, otherViewport.Total-otherViewport.Size))
		m.Viewports[SectionOther] = otherViewport

		// Ensure cursor is still visible after resize
		m.ensureCursorVisible() // Add a helper function for this

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

// getSectionBaseIndex returns the base cursor index for a section
func (m Model) getSectionBaseIndex(section Section) int {
	switch section {
	case SectionSuggested:
		return len(m.KeyBranches)
	case SectionOther:
		return len(m.KeyBranches) + len(m.SuggestedBranches)
	case SectionKey:
		return 0
	default:
		return 0
	}
}

// getCurrentSection determines the section for the current cursor position
func (m Model) getCurrentSection() Section {
	if m.Cursor < len(m.ListOrder) {
		return m.getBranchSection(m.ListOrder[m.Cursor])
	}
	return SectionSuggested // Default
}

// updateCurrentSection updates the current section based on cursor position
func (m *Model) updateCurrentSection() {
	m.CurrentSection = m.getCurrentSection()
}

// isScrollableSection returns true if a section can be scrolled
func (m Model) isScrollableSection(section Section) bool {
	return section == SectionSuggested || section == SectionOther
}

// moveUp moves the cursor up one position
func (m *Model) moveUp() {
	if m.Cursor <= 0 {
		return
	}

	// Determine section before moving cursor
	sectionBeforeMove := m.getCurrentSection()

	// Move cursor up
	m.Cursor--

	// Auto-scroll viewport if cursor moves out of view
	if viewport, ok := m.Viewports[sectionBeforeMove]; ok && viewport.Total > viewport.Size {
		baseIndex := m.getSectionBaseIndex(sectionBeforeMove)
		sectionIndex := m.Cursor - baseIndex

		// If cursor moved above the visible area of the section
		if sectionIndex >= 0 && sectionIndex < viewport.Start {
			viewport.Start = sectionIndex
			m.Viewports[sectionBeforeMove] = viewport
		}
	}

	// Update current section
	m.updateCurrentSection()
}

// moveDown moves the cursor down one position
func (m *Model) moveDown(totalItems int) {
	if m.Cursor >= totalItems-1 {
		return
	}

	// Determine section before moving cursor
	sectionBeforeMove := m.getCurrentSection()

	// Move cursor down
	m.Cursor++

	// Auto-scroll viewport if cursor moves out of view
	if viewport, ok := m.Viewports[sectionBeforeMove]; ok && viewport.Total > viewport.Size {
		baseIndex := m.getSectionBaseIndex(sectionBeforeMove)
		sectionIndex := m.Cursor - baseIndex

		// If cursor moved below the visible area of the section
		if sectionIndex >= 0 && sectionIndex >= viewport.Start+viewport.Size {
			viewport.Start = sectionIndex - viewport.Size + 1
			m.Viewports[sectionBeforeMove] = viewport
		}
	}

	// Update current section
	m.updateCurrentSection()
}

// pageUp scrolls the current section up one page
func (m *Model) pageUp() {
	// Determine section before scrolling
	sectionBeforeScroll := m.getCurrentSection()

	// Scroll the section's viewport up
	if viewport, ok := m.Viewports[sectionBeforeScroll]; ok && viewport.Total > viewport.Size {
		if m.isScrollableSection(sectionBeforeScroll) {
			viewport.Start = max(0, viewport.Start-viewport.Size)
			m.Viewports[sectionBeforeScroll] = viewport

			// Move cursor to the top of the new viewport page
			baseIndex := m.getSectionBaseIndex(sectionBeforeScroll)
			m.Cursor = baseIndex + viewport.Start
		}
	}

	// Update current section
	m.updateCurrentSection()
}

// pageDown scrolls the current section down one page
func (m *Model) pageDown() {
	// Determine section before scrolling
	sectionBeforeScroll := m.getCurrentSection()

	// Scroll the section's viewport down
	if viewport, ok := m.Viewports[sectionBeforeScroll]; ok && viewport.Total > viewport.Size {
		if m.isScrollableSection(sectionBeforeScroll) {
			maxStart := max(0, viewport.Total-viewport.Size)
			viewport.Start = min(maxStart, viewport.Start+viewport.Size)
			m.Viewports[sectionBeforeScroll] = viewport

			// Move cursor to the top of the new viewport page
			baseIndex := m.getSectionBaseIndex(sectionBeforeScroll)
			m.Cursor = baseIndex + viewport.Start
		}
	}

	// Update current section
	m.updateCurrentSection()
}

// jumpToSectionStart jumps to the first item in the current section
func (m *Model) jumpToSectionStart() {
	// Determine section before jumping
	sectionBeforeJump := m.getCurrentSection()

	// Jump to the first item in that section
	if viewport, ok := m.Viewports[sectionBeforeJump]; ok && viewport.Total > 0 {
		if m.isScrollableSection(sectionBeforeJump) {
			viewport.Start = 0
			m.Viewports[sectionBeforeJump] = viewport
		}

		// Move cursor to the first item of the section
		baseIndex := m.getSectionBaseIndex(sectionBeforeJump)
		m.Cursor = baseIndex
	}

	// Update current section
	m.updateCurrentSection()
}

// jumpToSectionEnd jumps to the last item in the current section
func (m *Model) jumpToSectionEnd() {
	// Determine section before jumping
	sectionBeforeJump := m.getCurrentSection()

	// Jump to the last item in that section
	if viewport, ok := m.Viewports[sectionBeforeJump]; ok && viewport.Total > 0 {
		if m.isScrollableSection(sectionBeforeJump) {
			maxStart := max(0, viewport.Total-viewport.Size)
			viewport.Start = maxStart
			m.Viewports[sectionBeforeJump] = viewport
		}

		// Move cursor to the last item of the section
		baseIndex := m.getSectionBaseIndex(sectionBeforeJump)
		lastItemIndexInSection := viewport.Total - 1
		m.Cursor = baseIndex + lastItemIndexInSection
	}

	// Update current section
	m.updateCurrentSection()
}

// toggleLocalSelection toggles local branch selection
func (m *Model) toggleLocalSelection() {
	if m.Cursor >= len(m.ListOrder) {
		return
	}

	originalIndex := m.ListOrder[m.Cursor]
	if m.isSelectable(originalIndex) {
		_, exists := m.SelectedLocal[originalIndex]
		if exists {
			delete(m.SelectedLocal, originalIndex)
			delete(m.SelectedRemote, originalIndex) // Also deselect remote
		} else {
			m.SelectedLocal[originalIndex] = true

			// Auto-select remote if it exists
			branch := m.AllAnalyzedBranches[originalIndex]
			if branch.Remote != "" {
				m.SelectedRemote[originalIndex] = true
			}
		}
	}
}

// toggleRemoteSelection toggles remote branch selection
func (m *Model) toggleRemoteSelection() {
	if m.Cursor >= len(m.ListOrder) {
		return
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
		m.moveUp()

	case "down", "j":
		m.moveDown(totalItems)

	case "pgup":
		m.pageUp()

	case "pgdown":
		m.pageDown()

	case "home":
		m.jumpToSectionStart()

	case "end":
		m.jumpToSectionEnd()

	case " ": // Toggle local selection
		m.toggleLocalSelection()

	case "tab", "r": // Toggle remote selection
		m.toggleRemoteSelection()

	case "enter":
		if len(m.SelectedLocal) > 0 || len(m.SelectedRemote) > 0 {
			m.ViewState = StateConfirming
		}
		return m, nil // No command needed here
	}

	// Ensure cursor is visible after any potential move/scroll
	m.ensureCursorVisible()

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
func (m Model) renderKeyBranches(b *strings.Builder) { // Removed itemIndex
	for i, branch := range m.KeyBranches { // Capture index i
		cursor := " "
		// Calculate display index based on loop index (key branches are first)
		displayIndex := i
		if m.Cursor == displayIndex {
			cursor = cursorStyle.Render(">")
		}

		localCheckbox := checkboxUnselectable // Never selectable
		remoteCheckbox := checkboxUnselectable
		lineStyle := protectedStyle

		// Restore remote info display using styles
		// Correct remote info display using styles and correct check
		remoteInfo := remoteNoneStyle.Render(remoteNone)
		if branch.Remote != "" { // Check if remote name is configured
			// Now check if it actually exists upstream (implicitly via HasRemote in analysis, but we use Remote field here)
			// Assuming analysis sets Remote correctly based on upstream existence check
			// If Remote is set, we assume it exists for display purposes here.
			// A more robust check might involve passing HasRemote if available
			remoteInfo = remoteDimmedStyle.Render(fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name))
		} // No specific style needed for "remote defined but doesn't exist" based on current styles

		categoryText := protectedStyle.Render("(Protected)")
		if branch.IsCurrent {
			categoryText = protectedStyle.Render("(Current)")
		}

		line := fmt.Sprintf("Local: %s %s | Remote: %s %s %s",
			localCheckbox, branch.Name, remoteCheckbox, remoteInfo, categoryText)

		b.WriteString(cursor + " " + lineStyle.Render(line) + "\n")
		// No need to increment shared index
	}
}

// renderSuggestedBranches renders the selectable suggested branches (MergedOld, UnmergedOld).
// Kept internal as it's only called by View.
func (m Model) renderSuggestedBranches(b *strings.Builder) {
	// Get viewport state for suggested branches
	// viewport := m.Viewports[SectionSuggested] // Already declared earlier

	// Add a simple debug line here
	b.WriteString("--- DEBUG: Rendering Suggested Section ---\n")
	viewport := m.Viewports[SectionSuggested]

	// Debug logging removed

	// Debug output removed for production

	// Show "More above" indicator only if needed
	if viewport.Start > 0 {
		b.WriteString(helpStyle.Render("   ↑ More branches above ↑") + "\n")
	}
	// Removed the unconditional blank line here

	// Only render branches that are in the current viewport
	visibleEnd := min(viewport.Start+viewport.Size, len(m.SuggestedBranches))

	// Removed unused branchLinesToRender calculation

	// Render the visible branches
	for i := viewport.Start; i < visibleEnd; i++ {
		if i >= len(m.SuggestedBranches) {
			break // Safety check
		}

		branch := m.SuggestedBranches[i] // Revert to original approach

		// Calculate the actual display index for this branch
		displayIndex := len(m.KeyBranches) + i
		// *itemIndex = displayIndex // Removed - Shared index no longer used this way

		// Find original index from ListOrder
		if displayIndex >= len(m.ListOrder) {
			continue // Should not happen if ListOrder is correct
		}
		originalIndex := m.ListOrder[displayIndex]

		if originalIndex < 0 || originalIndex >= len(m.AllAnalyzedBranches) {
			continue // Safety check
		}
		// Reverted: Fetch branch data using originalIndex instead of m.SuggestedBranches[i]

		cursor := " "
		if m.Cursor == displayIndex {
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
		if m.Cursor == displayIndex {
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
		// --- End Simplified Rendering ---

		// Increment the shared index after rendering
		// No need to increment shared index
	}

	// Removed padding loop - let dynamic height handle layout

	// Always reserve space for "More below" indicator
	if viewport.Start+viewport.Size < viewport.Total {
		b.WriteString(helpStyle.Render("   ↓ More branches below ↓") + "\n")
	}
	// Removed the unconditional blank line here

	// Show pagination indicator if there are more branches than can fit
	if viewport.Total > viewport.Size {
		indicator := renderCompactIndicator(viewport.Start, viewport.Size, viewport.Total, m.Width)
		b.WriteString(indicator + "\n")
	}
}

// renderOtherActiveBranches renders the non-selectable active branches.
// Kept internal as it's only called by View.
func (m Model) renderOtherActiveBranches(b *strings.Builder) { // Removed itemIndex
	for i, branch := range m.OtherActiveBranches { // Capture index i
		cursor := " "
		// Calculate display index based on loop index and previous sections
		displayIndex := len(m.KeyBranches) + len(m.SuggestedBranches) + i
		if m.Cursor == displayIndex {
			cursor = cursorStyle.Render(">")
		}

		// These are never selectable
		localCheckbox := checkboxUnselectable
		remoteCheckbox := checkboxUnselectable
		lineStyle := activeStyle // Use faint style

		// Restore remote info display using styles
		// Correct remote info display using styles and correct check
		remoteInfo := remoteNoneStyle.Render(remoteNone)
		if branch.Remote != "" { // Check if remote name is configured
			// Assume analysis sets Remote correctly based on upstream existence check
			remoteInfo = remoteDimmedStyle.Render(fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name))
		} // No specific style needed for "remote defined but doesn't exist"

		categoryText := activeStyle.Render("(" + string(branch.Category) + ")")

		line := fmt.Sprintf("Local: %s %s | Remote: %s %s %s",
			localCheckbox, branch.Name, remoteCheckbox, remoteInfo, categoryText)

		b.WriteString(cursor + " " + lineStyle.Render(line) + "\n")
		// No need to increment shared index
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

	// itemIndex := 0 // Removed - No longer needed

	// --- Render Key Branches ---
	m.renderKeyBranches(b) // Removed itemIndex

	// --- Separator and Header for Suggested branches ---
	hasSuggestions := len(m.SuggestedBranches) > 0
	hasActive := len(m.OtherActiveBranches) > 0
	hasKeys := len(m.KeyBranches) > 0 // Restore check

	// Restore conditional separator logic
	if hasKeys && (hasSuggestions || hasActive) {
		b.WriteString(separatorStyle.Render("---") + "\n")
	}
	if hasSuggestions {
		b.WriteString(headingStyle.Render("Suggested Branches (Candidates):") + "\n")
		b.WriteString("--- DEBUG: BEFORE renderSuggestedBranches ---\n") // DEBUG
		m.renderSuggestedBranches(b)
		b.WriteString("--- DEBUG: AFTER renderSuggestedBranches ---\n") // DEBUG
	}

	// --- Separator and Header for Other Active branches ---
	// Restore conditional separator logic
	if hasSuggestions && hasActive {
		b.WriteString(separatorStyle.Render("---") + "\n")
	}
	// Restore rendering 'Other' section
	if hasActive {
		b.WriteString(headingStyle.Render("Other Branches (Active / Not Selectable):") + "\n")
		m.renderOtherActiveBranches(b) // Removed itemIndex
	}

	if len(m.ListOrder) == 0 { // Check if the overall list order is empty
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

// --- Helper Functions ---

// ensureCursorVisible adjusts the viewport start position for the current section
// so that the cursor is within the visible area. Should be called after
// operations that might change viewport size or cursor position relative to it.
func (m *Model) ensureCursorVisible() {
	// Use a pointer receiver to modify the model directly
	if m.Cursor < 0 || m.Cursor >= len(m.ListOrder) {
		return // Cursor out of bounds
	}

	cursorSection := m.getBranchSection(m.ListOrder[m.Cursor])
	viewport, ok := m.Viewports[cursorSection]
	// Use pointer to modify map value directly
	if !ok {
		return
	} // Viewport not found for section

	if viewport.Size <= 0 || viewport.Total <= viewport.Size {
		return // Not a scrollable section or viewport invalid/not needed
	}

	// Calculate the base index for this section
	baseIndex := 0
	switch cursorSection {
	case SectionSuggested:
		baseIndex = len(m.KeyBranches)
	case SectionOther:
		baseIndex = len(m.KeyBranches) + len(m.SuggestedBranches)
	case SectionKey:
		return // Key section doesn't scroll, no need to adjust viewport
	default: // Unknown section
		return
	}

	// Calculate cursor's index relative to the section start
	sectionIndex := m.Cursor - baseIndex
	if sectionIndex < 0 {
		return
	} // Should not happen if baseIndex logic is correct

	// Determine newStart based on cursor position relative to viewport
	newStart := viewport.Start // Initialize with current value
	switch {
	case sectionIndex < viewport.Start: // Cursor above viewport
		newStart = sectionIndex
	case sectionIndex >= viewport.Start+viewport.Size: // Cursor below viewport
		newStart = sectionIndex - viewport.Size + 1
		// default: // Cursor is already visible, newStart remains viewport.Start
	}

	// Clamp Start value to valid range
	newStart = min(max(0, newStart), max(0, viewport.Total-viewport.Size))

	// Only update if the start position actually changed
	if newStart != viewport.Start {
		viewport.Start = newStart
		m.Viewports[cursorSection] = viewport // Update the viewport in the map
	}
}
