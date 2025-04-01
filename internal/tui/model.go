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
	progressStyle = helpStyle
	// Unused styles after refactoring to simpler indicators
	// progressMarkerStyle = selectedStyle
	// progressInfoStyle   = helpStyle
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

// Unused after refactoring to simpler indicators
// // Helper function to render the compact progress indicator
// func renderCompactIndicator(start, viewportSize, total int, width int) string {
// 	// Handle case where everything fits
// 	if total <= viewportSize {
// 		return progressInfoStyle.Render("All branches visible")
// 	}
//
// 	// Calculate position (ensure we don't divide by zero)
// 	maxStart := max(0, total-viewportSize)
// 	position := float64(start) / float64(maxStart)
//
// 	// Numerical portion
// 	nums := fmt.Sprintf("%d-%d/%d",
// 		start+1,
// 		min(start+viewportSize, total),
// 		total)
//
// 	// Visual bar portion - use a fixed width for the bar
// 	barWidth := 10
// 	markerPos := int(position * float64(barWidth))
//
// 	bar := "["
// 	for i := 0; i < barWidth; i++ {
// 		if i == markerPos {
// 			bar += progressMarkerStyle.Render("⬤") // Position marker with emphasis
// 		} else {
// 			bar += "·" // Bar dots
// 		}
// 	}
// 	bar += "]"
//
// 	// Percentage
// 	percentage := fmt.Sprintf("(%d%%)", int(position*100))
//
// 	// Help text that fits remaining space
// 	helpText := " | PgUp/PgDn to scroll"
//
// 	// Check if we have room for Home/End text
// 	if width >= len(nums)+len(bar)+len(percentage)+len(helpText)+20 {
// 		helpText += " | Home/End to jump"
// 	}
//
// 	return progressStyle.Render(nums+" "+bar+" "+percentage) +
// 		progressInfoStyle.Render(helpText)
// }

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
	// Only the suggested section is scrollable now
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
		// Other branches don't scroll anymore, so we don't need to account for
		// them in the available height calculation

		// Calculate available height after accounting for headers, footers, etc.
		availableHeight := max(3, m.Height-fixedHeightEstimate)

		// Set fixed viewport sizes to match user requirements
		keyHeight := min(len(m.KeyBranches), 3) // Show up to 3 key branches
		// Other branches are fixed at 5 maximum, no viewport needed

		// Give remaining space to suggested branches, but cap at 10
		// Initial values
		var suggestedHeight int
		var otherHeight int

		// Guarantee minimums if items exist
		if suggestedTotal > 0 {
			suggestedHeight = 1
		}
		// We always show up to 5 other branches if they exist
		otherHeight = min(5, otherTotal)

		// Calculate height remaining *after* guaranteeing minimums (and keyHeight)
		// availableHeight is now defined
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

			// Other branches are fixed at max 5, no need to distribute remaining height to them
		}

		// Final cap: ensure height doesn't exceed total items (might be redundant now, but safe)
		suggestedHeight = min(suggestedHeight, suggestedTotal)
		// Other branches are fixed at max 5

		// Debug logging removed

		// Update viewport sizes and totals
		if m.Viewports == nil {
			m.Viewports = make(map[Section]ViewportState)
		}

		// Key branches viewport
		keyViewport := m.Viewports[SectionKey]
		keyViewport.Size = keyHeight
		keyViewport.Total = len(m.KeyBranches)
		m.Viewports[SectionKey] = keyViewport

		// Suggested branches viewport - max 10 visible items
		suggestedViewport := m.Viewports[SectionSuggested]
		suggestedViewport.Size = suggestedHeight
		suggestedViewport.Total = len(m.SuggestedBranches)
		m.Viewports[SectionSuggested] = suggestedViewport

		// Other branches are fixed at 5 visible items maximum, no scrolling
		// We don't need viewport for this section anymore since it doesn't scroll

		// Ensure cursor is still visible after resize
		m = m.ensureCursorVisible() // Use the ensureCursorVisible that returns Model

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

// moveUp moves the cursor up and handles scrolling logic
func (m Model) moveUp() Model {
	if m.Cursor <= 0 {
		return m // Already at the top
	}

	// Move cursor up
	m.Cursor--

	// Get current section based on the new cursor position
	cursorSection := m.getCurrentSection()

	// Update current section
	m.CurrentSection = cursorSection

	// Auto-scroll viewport if cursor is in the suggested section
	if cursorSection == SectionSuggested {
		// Get the index within the suggested section
		sectionIndex := 0
		for i := 0; i < len(m.KeyBranches); i++ {
			if m.Cursor == i {
				break
			}
			sectionIndex++
		}

		// If cursor is now outside viewport, scroll up
		viewport := m.Viewports[SectionSuggested]
		if sectionIndex < viewport.Start {
			viewport.Start = max(0, sectionIndex)
			m.Viewports[SectionSuggested] = viewport
		}
	}

	return m
}

// moveDown moves the cursor down and handles scrolling logic
func (m Model) moveDown(_ int) Model {
	// Calculate the last valid cursor position
	// This is the last suggested branch, ensuring we don't move into Other Branches
	lastValidCursor := len(m.KeyBranches) + len(m.SuggestedBranches) - 1

	// If we're already at the last valid position or beyond, do nothing
	if m.Cursor >= lastValidCursor {
		return m
	}

	// Move cursor down (but never beyond the suggested branches)
	m.Cursor++

	// Get current section based on the new cursor position
	cursorSection := m.getCurrentSection()

	// Update current section - should always be Key or Suggested
	m.CurrentSection = cursorSection

	// Auto-scroll viewport if cursor is in the suggested section
	if cursorSection == SectionSuggested {
		// Get the index within the suggested section
		sectionIndex := 0
		for i := len(m.KeyBranches); i < len(m.KeyBranches)+len(m.SuggestedBranches); i++ {
			if m.Cursor == i {
				break
			}
			sectionIndex++
		}

		// If cursor is now outside viewport, scroll down
		viewport := m.Viewports[SectionSuggested]
		if sectionIndex >= viewport.Start+viewport.Size {
			viewport.Start = max(0, sectionIndex-viewport.Size+1)
			m.Viewports[SectionSuggested] = viewport
		}
	}

	return m
}

// pageUp scrolls the current section up one page
func (m Model) pageUp() Model {
	cursorSection := m.getCurrentSection()

	// Only scroll in the suggested section
	if cursorSection == SectionSuggested {
		viewport := m.Viewports[SectionSuggested]
		// Move viewport up by page size
		viewport.Start = max(0, viewport.Start-viewport.Size)
		m.Viewports[SectionSuggested] = viewport

		// Move cursor to top of viewport if it's now outside
		newCursorPos := len(m.KeyBranches) + viewport.Start
		if m.Cursor < newCursorPos || m.Cursor >= newCursorPos+viewport.Size {
			m.Cursor = newCursorPos
		}

		// Update current section
		m.CurrentSection = m.getCurrentSection()
	}

	return m
}

// pageDown scrolls the current section down one page
func (m Model) pageDown() Model {
	cursorSection := m.getCurrentSection()

	// Only scroll in the suggested section
	if cursorSection == SectionSuggested {
		viewport := m.Viewports[SectionSuggested]
		// Move viewport down by page size
		maxStart := max(0, viewport.Total-viewport.Size)
		viewport.Start = min(maxStart, viewport.Start+viewport.Size)
		m.Viewports[SectionSuggested] = viewport

		// Move cursor to top of viewport
		newCursorPos := len(m.KeyBranches) + viewport.Start
		if m.Cursor < newCursorPos || m.Cursor >= newCursorPos+viewport.Size {
			m.Cursor = newCursorPos
		}

		// Update current section
		m.CurrentSection = m.getCurrentSection()
	}

	return m
}

// jumpToSectionStart jumps to the first item in the current section
func (m Model) jumpToSectionStart() Model {
	cursorSection := m.getCurrentSection()

	// Only jump in the suggested section
	if cursorSection == SectionSuggested {
		viewport := m.Viewports[SectionSuggested]
		viewport.Start = 0
		m.Viewports[SectionSuggested] = viewport

		// Move cursor to first item in section
		m.Cursor = len(m.KeyBranches)

		// Update current section
		m.CurrentSection = m.getCurrentSection()
	}

	return m
}

// jumpToSectionEnd jumps to the last item in the current section
func (m Model) jumpToSectionEnd() Model {
	cursorSection := m.getCurrentSection()

	// Only jump in the suggested section
	if cursorSection == SectionSuggested {
		viewport := m.Viewports[SectionSuggested]
		maxStart := max(0, viewport.Total-viewport.Size)
		viewport.Start = maxStart
		m.Viewports[SectionSuggested] = viewport

		// Move cursor to last visible item
		lastVisible := min(
			len(m.KeyBranches)+viewport.Start+viewport.Size-1,
			len(m.KeyBranches)+len(m.SuggestedBranches)-1,
		)
		m.Cursor = lastVisible

		// Update current section
		m.CurrentSection = m.getCurrentSection()
	}

	return m
}

// toggleLocalSelection toggles local branch selection
func (m Model) toggleLocalSelection() Model {
	if m.Cursor >= len(m.ListOrder) {
		return m // Bounds check
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

	return m
}

// toggleRemoteSelection toggles remote branch selection
func (m Model) toggleRemoteSelection() Model {
	if m.Cursor >= len(m.ListOrder) {
		return m // Bounds check
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

	return m
}

// getCurrentSection determines the section for the current cursor position
func (m Model) getCurrentSection() Section {
	// Calculate section boundaries
	firstSuggestedIndex := len(m.KeyBranches)
	lastSuggestedIndex := firstSuggestedIndex + len(m.SuggestedBranches) - 1

	// Determine section based on cursor position
	switch {
	case m.Cursor < firstSuggestedIndex:
		return SectionKey
	case m.Cursor <= lastSuggestedIndex:
		return SectionSuggested
	default:
		// This shouldn't happen anymore, but return Suggested as fallback
		return SectionSuggested
	}
}

// ensureCursorVisible makes sure the cursor is visible in the current viewport.
func (m Model) ensureCursorVisible() Model {
	if m.Cursor < 0 {
		return m
	}

	// Get section for current cursor
	cursorSection := m.getCurrentSection()

	// Only handle suggested section with viewport logic (Other section doesn't scroll and is not selectable)
	if cursorSection != SectionSuggested {
		return m
	}

	// Get the base index and viewport for this section
	baseIndex := m.getSectionBaseIndex(cursorSection)
	viewport, ok := m.Viewports[cursorSection]
	if !ok || viewport.Total <= viewport.Size {
		return m // No viewport or everything fits
	}

	// Get cursor position within section
	sectionIndex := m.Cursor - baseIndex

	// Cursor is above viewport, adjust viewport to show cursor
	if sectionIndex < viewport.Start {
		viewport.Start = sectionIndex
		m.Viewports[cursorSection] = viewport
	}

	// Cursor is below viewport, adjust viewport to show cursor
	if sectionIndex >= viewport.Start+viewport.Size {
		viewport.Start = sectionIndex - viewport.Size + 1
		m.Viewports[cursorSection] = viewport
	}

	return m
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

// updateSelecting handles key presses when in the selecting state.
func (m Model) updateSelecting(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalItems := len(m.ListOrder)
	if totalItems == 0 {
		if msg.String() == "q" {
			return m, tea.Quit
		}
		return m, nil // Ignore other keys if list is empty
	}

	// Initialize current section
	m.CurrentSection = m.getCurrentSection()

	// Handle different key presses
	switch msg.String() {
	case "q":
		return m, tea.Quit

	case "up", "k":
		m = m.moveUp()

	case "down", "j":
		m = m.moveDown(totalItems)

	case "pgup":
		m = m.pageUp()

	case "pgdown":
		m = m.pageDown()

	case "home":
		m = m.jumpToSectionStart()

	case "end":
		m = m.jumpToSectionEnd()

	case " ": // Toggle local selection
		m = m.toggleLocalSelection()

	case "tab", "r": // Toggle remote selection
		m = m.toggleRemoteSelection()

	case "enter":
		if len(m.SelectedLocal) > 0 || len(m.SelectedRemote) > 0 {
			m.ViewState = StateConfirming
		}
		return m, nil // No command needed here
	}

	// Ensure cursor is visible in the viewport
	m = m.ensureCursorVisible()

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
	for i, branch := range m.KeyBranches { // Capture index i
		cursor := " "
		// Calculate display index based on loop index (key branches are first)
		displayIndex := i
		if m.Cursor == displayIndex {
			cursor = cursorStyle.Render(">")
		}

		// Normalize cursor width for consistent alignment
		if cursor == " " {
			cursor = "  " // Two spaces when no cursor
		} else {
			cursor += " " // Cursor with one space
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

		b.WriteString(cursor + lineStyle.Render(line) + "\n")
		*itemIndex++ // Increment the shared index
	}
}

// renderSuggestedBranches renders the selectable suggested branches (MergedOld, UnmergedOld).
// Kept internal as it's only called by View. The itemIndex parameter tracks the global item index.
func (m Model) renderSuggestedBranches(b *strings.Builder, _ *int) {
	// Get viewport state for suggested branches
	viewport := m.Viewports[SectionSuggested]

	// Remove debug output for production version

	// Show "More above" indicator only if needed
	if viewport.Start > 0 {
		// Add more spaces to properly align with branch content
		b.WriteString("    " + helpStyle.Render("-- More branches above --") + "\n")
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

		// Get cursor with consistent width for alignment
		cursor := " "
		if m.Cursor == displayIndex {
			cursor = cursorStyle.Render(">")
		}

		// Normalize cursor width for consistent alignment
		if cursor == " " {
			cursor = "  " // Two spaces when no cursor
		} else {
			cursor += " " // Cursor with one space
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

		// Construct the line with appropriate styling for each part
		// Use a consistent formatting for the line to ensure alignment
		// Build remote text with the declared variables
		remoteText := fmt.Sprintf("Remote: %s %s", remoteCheckbox, remoteInfo)

		line := fmt.Sprintf("Local: %s %s | %s %s",
			localCheckbox, branch.Name, remoteText, categoryText)

		// Apply styling based on cursor and category
		if m.Cursor == displayIndex {
			if branch.Category == types.CategoryUnmergedOld {
				// For unmerged branches, apply warning style
				// But preserve the remote dimming/brightening
				parts := strings.SplitN(line, " | ", 2)
				if len(parts) == 2 {
					localPart := warningStyle.Render(selectedStyle.Render(parts[0]))
					// Don't apply warning style to the remote part to preserve dimming
					b.WriteString(cursor + localPart + " | " + parts[1] + "\n")
				} else {
					// Fallback if split fails
					b.WriteString(cursor + warningStyle.Render(selectedStyle.Render(line)) + "\n")
				}
			} else {
				// For merged branches, apply selected style to local part only
				parts := strings.SplitN(line, " | ", 2)
				if len(parts) == 2 {
					localPart := selectedStyle.Render(parts[0])
					// Don't apply selected style to the remote part to preserve dimming
					b.WriteString(cursor + localPart + " | " + parts[1] + "\n")
				} else {
					// Fallback if split fails
					b.WriteString(cursor + selectedStyle.Render(line) + "\n")
				}
			}
		} else {
			if branch.Category == types.CategoryUnmergedOld {
				// For unmerged branches not under cursor
				parts := strings.SplitN(line, " | ", 2)
				if len(parts) == 2 {
					localPart := warningStyle.Render(parts[0])
					// Don't apply warning style to the remote part to preserve dimming
					b.WriteString(cursor + localPart + " | " + parts[1] + "\n")
				} else {
					// Fallback if split fails
					b.WriteString(cursor + warningStyle.Render(line) + "\n")
				}
			} else {
				// Regular branches not under cursor
				b.WriteString(cursor + line + "\n")
			}
		}
		// --- End Simplified Rendering ---

		// Increment the shared index after rendering
		// No need to increment shared index
	}

	// Removed padding loop - let dynamic height handle layout

	// Always reserve space for "More below" indicator
	if viewport.Start+viewport.Size < viewport.Total {
		// Add more spaces to properly align with branch content
		b.WriteString("    " + helpStyle.Render("-- More branches below --\n"))
	}
	// Removed the unconditional blank line here

	// Show pagination indicator if there are more branches than can fit
	if viewport.Total > viewport.Size {
		// Create a right-aligned indicator
		nums := fmt.Sprintf("%d-%d/%d",
			viewport.Start+1,
			min(viewport.Start+viewport.Size, viewport.Total),
			viewport.Total)

		// Add a fixed amount of spaces to better right-align the numbers
		b.WriteString("                                                          " + progressStyle.Render(nums) + "\n")
	}
}

// renderOtherActiveBranches renders the non-selectable active branches.
// Kept internal as it's only called by View.
func (m Model) renderOtherActiveBranches(b *strings.Builder, _ *int) {
	// Always limit to exactly 5 branches (or fewer if there are less than 5)
	maxBranches := 5
	totalBranches := len(m.OtherActiveBranches)

	// Show only the first 5 branches, no scrolling
	visibleBranches := min(maxBranches, totalBranches)

	// Render only the first 5 branches
	for i := 0; i < visibleBranches; i++ {
		branch := m.OtherActiveBranches[i]

		// Other branches are never selectable and never show a cursor
		cursor := "  " // Always use two spaces (no cursor)

		// These are never selectable
		localCheckbox := checkboxUnselectable
		remoteCheckbox := checkboxUnselectable
		lineStyle := activeStyle // Use faint style

		// Format remote info
		remoteInfo := remoteNoneStyle.Render(remoteNone)
		if branch.Remote != "" {
			remoteInfo = remoteDimmedStyle.Render(fmt.Sprintf("(%s/%s)", branch.Remote, branch.Name))
		}

		categoryText := activeStyle.Render("(" + string(branch.Category) + ")")

		line := fmt.Sprintf("Local: %s %s | Remote: %s %s %s",
			localCheckbox, branch.Name, remoteCheckbox, remoteInfo, categoryText)

		b.WriteString(cursor + lineStyle.Render(line) + "\n")

		// We don't increment itemIndex here since these branches are not part of the
		// navigation order and can't be selected
	}

	// If there are more branches than we're showing, just add a note
	if totalBranches > maxBranches {
		totalHidden := totalBranches - maxBranches
		b.WriteString("    " + helpStyle.Render(fmt.Sprintf("... and %d more branches not shown", totalHidden)) + "\n")
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
	hasKeys := len(m.KeyBranches) > 0 // Restore check

	// Restore conditional separator logic
	if hasKeys && (hasSuggestions || hasActive) {
		b.WriteString(separatorStyle.Render("---") + "\n")
	}
	if hasSuggestions {
		b.WriteString(headingStyle.Render("Suggested Branches (Candidates):") + "\n")
		m.renderSuggestedBranches(b, &itemIndex)
	}

	// --- Separator and Header for Other Active branches ---
	// Restore conditional separator logic
	if hasSuggestions && hasActive {
		b.WriteString(separatorStyle.Render("---") + "\n")
	}
	// Restore rendering 'Other' section
	if hasActive {
		b.WriteString(headingStyle.Render("Other Branches (Active / Not Selectable):") + "\n")
		m.renderOtherActiveBranches(b, &itemIndex)
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
// The duplicate ensureCursorVisible method has been removed
// There's another implementation at line 669
