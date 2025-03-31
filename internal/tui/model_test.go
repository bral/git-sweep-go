package tui

import (
	"context" // Added import
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bral/git-sweep-go/internal/types"
	tea "github.com/charmbracelet/bubbletea"
)

// Helper to create a basic model for testing
func createTestModel(branches []types.AnalyzedBranch) Model { // Use exported Model
	// Simplified context for testing
	ctx := context.Background()
	// Create a basic model, assuming not dry run for most tests
	model := InitialModel(ctx, branches, false) // Use exported InitialModel
	// Run Init() to ensure spinner is ready (though we won't test spinner ticks here)
	model.Init()
	return model
}

// Helper to simulate key presses
func simulateKeyPress(m tea.Model, key string) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

// Helper to simulate special key presses
func simulateSpecialKeyPress(m tea.Model, keyType tea.KeyType) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: keyType})
}

// Helper function to create sample analyzed branches
func createSampleBranches() []types.AnalyzedBranch {
	now := time.Now()
	ninetyDaysAgo := now.AddDate(0, 0, -91)
	sixtyDaysAgo := now.AddDate(0, 0, -60)

	return []types.AnalyzedBranch{
		// Key/Protected (Original Index 0)
		{
			BranchInfo: types.BranchInfo{Name: "main", LastCommitDate: now, Remote: "origin"},
			Category:   types.CategoryProtected, IsCurrent: true, IsProtected: true,
		},
		// Suggested (Original Index 1)
		{
			BranchInfo: types.BranchInfo{Name: "feat/merged", LastCommitDate: ninetyDaysAgo, Remote: "origin"},
			Category:   types.CategoryMergedOld, IsMerged: true,
		},
		// Suggested (Original Index 2)
		{
			BranchInfo: types.BranchInfo{Name: "feat/unmerged-old", LastCommitDate: ninetyDaysAgo, Remote: "origin"},
			Category:   types.CategoryUnmergedOld, IsMerged: false,
		},
		// Active (Original Index 3)
		{
			BranchInfo: types.BranchInfo{Name: "feat/active", LastCommitDate: sixtyDaysAgo, Remote: ""},
			Category:   types.CategoryActive, IsMerged: false,
		},
		// Suggested (Original Index 4) - No Remote
		{
			BranchInfo: types.BranchInfo{Name: "feat/merged-no-remote", LastCommitDate: ninetyDaysAgo, Remote: ""},
			Category:   types.CategoryMergedOld, IsMerged: true,
		},
	}
}

func TestTuiNavigation(t *testing.T) {
	branches := createSampleBranches()
	m := createTestModel(branches)
	// Expected order: main (0), feat/merged (1), feat/unmerged-old (2), feat/merged-no-remote (4), feat/active (3)
	// Indices in ListOrder: [0, 1, 2, 4, 3]
	totalItems := len(m.ListOrder) // Use exported ListOrder

	if m.Cursor != 0 { // Use exported Cursor
		t.Fatalf("Initial cursor expected 0, got %d", m.Cursor) // Use exported Cursor
	}

	// Move down
	mUpdated, _ := simulateSpecialKeyPress(m, tea.KeyDown)
	mAsserted, ok := mUpdated.(Model) // Use exported Model
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted
	if m.Cursor != 1 { // Use exported Cursor
		t.Errorf("Cursor after down: expected 1, got %d", m.Cursor) // Use exported Cursor
	}

	// Move down multiple times
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyDown)
	mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyDown)
	mAsserted, ok = mUpdated.(Model) // Use exported Model
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted
	if m.Cursor != 3 { // Use exported Cursor
		t.Errorf("Cursor after 3 downs: expected 3, got %d", m.Cursor) // Use exported Cursor
	}

	// Move past end
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyDown)
	mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyDown) // Try again
	mAsserted, ok = mUpdated.(Model)                             // Use exported Model
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted
	if m.Cursor != totalItems-1 { // Should be 4 // Use exported Cursor
		t.Errorf("Cursor after down past end: expected %d, got %d", totalItems-1, m.Cursor) // Use exported Cursor
	}

	// Move up
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyUp)
	mAsserted, ok = mUpdated.(Model) // Use exported Model
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted
	if m.Cursor != totalItems-2 { // Should be 3 // Use exported Cursor
		t.Errorf("Cursor after up: expected %d, got %d", totalItems-2, m.Cursor) // Use exported Cursor
	}

	// Move past beginning
	for i := 0; i < totalItems; i++ {
		mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyUp)
	}
	mAsserted, ok = mUpdated.(Model) // Use exported Model
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted
	if m.Cursor != 0 { // Use exported Cursor
		t.Errorf("Cursor after up past beginning: expected 0, got %d", m.Cursor) // Use exported Cursor
	}
}

func TestTuiSelection(t *testing.T) {
	branches := createSampleBranches()
	m := createTestModel(branches)
	// Expected order: main (0), feat/merged (1), feat/unmerged-old (2), feat/merged-no-remote (4), feat/active (3)
	// Indices in ListOrder: [0, 1, 2, 4, 3]
	// Selectable original indices: 1, 2, 4

	// 1. Try selecting protected (main, index 0) - should fail
	mUpdated, _ := simulateKeyPress(m, " ")
	mAsserted, ok := mUpdated.(Model) // Use exported Model
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted
	if len(m.SelectedLocal) != 0 { // Use exported SelectedLocal
		t.Errorf(
			"Expected 0 selected after trying to select protected, got %d",
			len(m.SelectedLocal),
		)
	}

	// 2. Move to first selectable (feat/merged, index 1) and select local
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyDown)
	mUpdated, _ = simulateKeyPress(mUpdated, " ")
	mAsserted, ok = mUpdated.(Model) // Use exported Model
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted
	originalIndex := m.ListOrder[1]      // Should be 1 // Use exported ListOrder
	if !m.SelectedLocal[originalIndex] { // Use exported SelectedLocal
		t.Errorf("Expected item at cursor 1 (original index %d) to be selected locally", originalIndex)
	}
	if len(m.SelectedLocal) != 1 { // Use exported SelectedLocal
		t.Errorf("Expected 1 selected local item, got %d", len(m.SelectedLocal)) // Use exported SelectedLocal
	}

	// 3. Verify remote is auto-selected for the same item
	if !m.SelectedRemote[originalIndex] { // Use exported SelectedRemote
		t.Errorf("Expected item at cursor 1 (original index %d) to be auto-selected remotely", originalIndex)
	}

	// 4. Move to next selectable (feat/unmerged-old, index 2) and select local
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyDown)
	mUpdated, _ = simulateKeyPress(mUpdated, " ")
	mAsserted, ok = mUpdated.(Model) // Use exported Model
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted
	originalIndex2 := m.ListOrder[2]      // Should be 2 // Use exported ListOrder
	if !m.SelectedLocal[originalIndex2] { // Use exported SelectedLocal
		t.Errorf("Expected item at cursor 2 (original index %d) to be selected locally", originalIndex2)
	}
	if len(m.SelectedLocal) != 2 { // Use exported SelectedLocal
		t.Errorf("Expected 2 selected local items, got %d", len(m.SelectedLocal)) // Use exported SelectedLocal
	}

	// 5. Verify remote is auto-selected for item 2
	if !m.SelectedRemote[originalIndex2] { // Use exported SelectedRemote
		t.Errorf("Expected item at cursor 2 (original index %d) to be auto-selected remotely", originalIndex2)
	}

	// 6. Move to non-selectable active branch (feat/active, index 4 -> original 3) and try selecting - should fail
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyDown)        // cursor 3 (original 4)
	mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyDown) // cursor 4 (original 3)
	mUpdated, _ = simulateKeyPress(mUpdated, " ")
	mAsserted, ok = mUpdated.(Model) // Use exported Model
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted
	if len(m.SelectedLocal) != 2 { // Use exported SelectedLocal
		t.Errorf(
			"Expected 2 selected local items after trying to select active, got %d",
			len(m.SelectedLocal),
		)
	}

	// 7. Move back to item 1 (original index 1) and deselect local (should deselect remote too)
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyUp)        // cursor 3 (original 4)
	mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyUp) // cursor 2 (original 2)
	mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyUp) // cursor 1 (original 1)
	mUpdated, _ = simulateKeyPress(mUpdated, " ")              // Deselect local
	mAsserted, ok = mUpdated.(Model)                           // Use exported Model
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted
	if m.SelectedLocal[originalIndex] { // Use exported SelectedLocal
		t.Errorf("Expected item at cursor 1 (original index %d) to be deselected locally", originalIndex)
	}
	if m.SelectedRemote[originalIndex] { // Use exported SelectedRemote
		t.Errorf("Expected item at cursor 1 (original index %d) "+
			"to be deselected remotely after local deselect", originalIndex)
	}
	if len(m.SelectedLocal) != 1 { // Only item 2 should be left // Use exported SelectedLocal
		t.Errorf("Expected 1 selected local item after deselect, got %d", len(m.SelectedLocal)) // Use exported SelectedLocal
	}
	if len(m.SelectedRemote) != 1 { // Item 2 should still be selected // Use exported SelectedRemote
		t.Errorf(
			"Expected 1 selected remote item after deselect, got %d",
			len(m.SelectedRemote),
		)
	}

	// 8. Move to item with no remote (feat/merged-no-remote, cursor 3, original 4) and select local
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyDown)        // cursor 2 (original 2)
	mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyDown) // cursor 3 (original 4)
	mUpdated, _ = simulateKeyPress(mUpdated, " ")                // Select local
	mAsserted, ok = mUpdated.(Model)                             // Use exported Model
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted
	originalIndex4 := m.ListOrder[3]      // Should be 4 // Use exported ListOrder
	if !m.SelectedLocal[originalIndex4] { // Use exported SelectedLocal
		t.Errorf("Expected item at cursor 3 (original index %d) to be selected locally", originalIndex4)
	}

	// 9. Try selecting remote (should fail as no remote exists)
	mUpdated, _ = simulateKeyPress(m, "r")
	mAsserted, ok = mUpdated.(Model) // Use exported Model
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted
	if m.SelectedRemote[originalIndex4] { // Use exported SelectedRemote
		t.Errorf("Expected item at cursor 3 (original index %d) to NOT be selected remotely", originalIndex4)
	}
	if len(m.SelectedRemote) != 1 { // Item 2 should still be selected // Use exported SelectedRemote
		t.Errorf(
			"Expected 1 selected remote item after trying on no-remote branch, got %d",
			len(m.SelectedRemote),
		)
	}
}

// --- TestTuiStateTransitions (Refactored) ---

// Define command types for easier assertion
type cmdType int

const (
	cmdTypeNil cmdType = iota
	cmdTypeQuit
	cmdTypeBatch // Represents tea.Batch containing performDeletionCmd and spinner.Tick
	// Add other specific command types if needed
)

// Helper to check command type
func checkCmdType(cmd tea.Cmd) cmdType {
	if cmd == nil {
		return cmdTypeNil
	}
	// Check for tea.Quit using reflection (reliable way)
	if reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(tea.Quit).Pointer() {
		return cmdTypeQuit
	}
	// Check if it's a batch command
	msg := cmd()
	if _, ok := msg.(tea.BatchMsg); ok {
		return cmdTypeBatch
	}
	return cmdTypeNil // Default or unknown
}

func TestTuiStateTransitions(t *testing.T) {
	branches := createSampleBranches()
	results := []types.DeleteResult{{BranchName: "test", Success: true}}

	testCases := []struct {
		name          string
		initialState  ViewState
		setupModel    func(m *Model) // Optional setup like selecting items
		inputMsg      tea.Msg
		expectedState ViewState
		expectedCmd   cmdType
	}{
		{
			name:          "Selecting: Enter with no selection",
			initialState:  StateSelecting,
			inputMsg:      tea.KeyMsg{Type: tea.KeyEnter},
			expectedState: StateSelecting,
			expectedCmd:   cmdTypeNil,
		},
		{
			name:         "Selecting: Select -> Enter -> Confirming",
			initialState: StateSelecting,
			setupModel: func(m *Model) {
				// Simulate moving down and selecting the first selectable item (original index 1)
				m.Cursor = 1
				originalIndex := m.ListOrder[m.Cursor]
				m.SelectedLocal[originalIndex] = true
			},
			inputMsg:      tea.KeyMsg{Type: tea.KeyEnter},
			expectedState: StateConfirming,
			expectedCmd:   cmdTypeNil,
		},
		{
			name:          "Confirming: n -> Selecting",
			initialState:  StateConfirming,
			inputMsg:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")},
			expectedState: StateSelecting,
			expectedCmd:   cmdTypeNil,
		},
		{
			name:          "Confirming: Esc -> Selecting",
			initialState:  StateConfirming,
			inputMsg:      tea.KeyMsg{Type: tea.KeyEsc},
			expectedState: StateSelecting,
			expectedCmd:   cmdTypeNil,
		},
		{
			name:         "Confirming: y -> Deleting",
			initialState: StateConfirming,
			setupModel: func(m *Model) {
				// Need something selected to trigger deletion cmd
				m.Cursor = 1
				originalIndex := m.ListOrder[m.Cursor]
				m.SelectedLocal[originalIndex] = true
			},
			inputMsg:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")},
			expectedState: StateDeleting,
			expectedCmd:   cmdTypeBatch, // Expecting the batch with deletion + tick
		},
		{
			name:          "Deleting: resultsMsg -> Results",
			initialState:  StateDeleting,
			inputMsg:      resultsMsg{results: results}, // Use internal resultsMsg
			expectedState: StateResults,
			expectedCmd:   cmdTypeNil,
		},
		{
			name:          "Results: Any Key -> Quit",
			initialState:  StateResults,
			inputMsg:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")},
			expectedState: StateResults, // State doesn't change, cmd signals quit
			expectedCmd:   cmdTypeQuit,
		},
		{
			name:          "Selecting: q -> Quit",
			initialState:  StateSelecting,
			inputMsg:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")},
			expectedState: StateSelecting, // State doesn't change
			expectedCmd:   cmdTypeQuit,
		},
		{
			name:          "Selecting: Ctrl+C -> Quit",
			initialState:  StateSelecting,
			inputMsg:      tea.KeyMsg{Type: tea.KeyCtrlC},
			expectedState: StateSelecting, // State doesn't change
			expectedCmd:   cmdTypeQuit,
		},
		// Add more cases if needed, e.g., key presses in Deleting state (should be ignored)
		{
			name:          "Deleting: Any Key -> No Change",
			initialState:  StateDeleting,
			inputMsg:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")},
			expectedState: StateDeleting,
			expectedCmd:   cmdTypeNil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := createTestModel(branches)
			m.ViewState = tc.initialState // Set initial state

			// Apply optional setup
			if tc.setupModel != nil {
				tc.setupModel(&m)
			}

			// Apply the input message
			mUpdated, cmd := m.Update(tc.inputMsg)

			// Assert the final state
			mAsserted, ok := mUpdated.(Model)
			if !ok {
				t.Fatalf("Update did not return a Model")
			}

			if mAsserted.ViewState != tc.expectedState {
				t.Errorf("Expected state %v, got %v", tc.expectedState, mAsserted.ViewState)
			}

			// Assert the command type
			actualCmdType := checkCmdType(cmd)
			if actualCmdType != tc.expectedCmd {
				t.Errorf("Expected command type %v, got %v", tc.expectedCmd, actualCmdType)
			}

			// Specific check for resultsMsg case
			if _, ok := tc.inputMsg.(resultsMsg); ok {
				if len(mAsserted.Results) != len(results) ||
					(len(results) > 0 && mAsserted.Results[0].BranchName != results[0].BranchName) {
					t.Errorf("Results not stored correctly in model after resultsMsg. Got: %+v", mAsserted.Results)
				}
			}
		})
	}
}

func TestTuiEmptyList(t *testing.T) {
	// Create model with no branches
	m := createTestModel([]types.AnalyzedBranch{})

	if m.ViewState != StateSelecting { // Use exported ViewState and StateSelecting
		t.Fatalf("Initial state not selecting")
	}
	if len(m.ListOrder) != 0 { // Use exported ListOrder
		t.Fatalf("Expected ListOrder to be empty")
	}
	if m.Cursor != 0 { // Use exported Cursor
		t.Fatalf("Expected cursor to be 0")
	}

	// --- Test Navigation ---
	t.Run("Navigation on Empty List", func(t *testing.T) {
		mUpdated, _ := simulateSpecialKeyPress(m, tea.KeyDown)
		mAsserted, ok := mUpdated.(Model) // Use exported Model
		if !ok {
			t.Fatalf("Type assertion failed for mUpdated.(Model)")
		}
		if mAsserted.Cursor != 0 { // Use exported Cursor
			t.Errorf("Cursor moved down on empty list")
		}
		mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyUp)
		mAsserted, ok = mUpdated.(Model) // Re-use mAsserted and ok // Use exported Model
		if !ok {
			t.Fatalf("Type assertion failed for mUpdated.(Model)")
		}
		if mAsserted.Cursor != 0 { // Use exported Cursor
			t.Errorf("Cursor moved up on empty list")
		}
	})

	// --- Test Selection ---
	t.Run("Selection on Empty List", func(t *testing.T) {
		mUpdated, _ := simulateKeyPress(m, " ")
		mAsserted, ok := mUpdated.(Model) // Use exported Model
		if !ok {
			t.Fatalf("Type assertion failed for mUpdated.(Model)")
		}
		if len(mAsserted.SelectedLocal) != 0 { // Use exported SelectedLocal
			t.Errorf("Local selection occurred on empty list")
		}
		mUpdated, _ = simulateKeyPress(m, "r")
		mAsserted, ok = mUpdated.(Model) // Re-use mAsserted and ok // Use exported Model
		if !ok {
			t.Fatalf("Type assertion failed for mUpdated.(Model)")
		}
		if len(mAsserted.SelectedRemote) != 0 { // Use exported SelectedRemote
			t.Errorf("Remote selection occurred on empty list")
		}
	})

	// --- Test Enter ---
	t.Run("Enter on Empty List", func(t *testing.T) {
		mUpdated, cmd := simulateSpecialKeyPress(m, tea.KeyEnter)
		mAsserted, ok := mUpdated.(Model) // Use exported Model
		if !ok {
			t.Fatalf("Type assertion failed for mUpdated.(Model)")
		}
		// Re-assign m here as well, although it's not used further in this specific test case
		m = mAsserted
		if mAsserted.ViewState != StateSelecting { // Use exported ViewState and StateSelecting
			t.Errorf("State changed on Enter with empty list")
		}
		if cmd != nil {
			t.Errorf("Command returned on Enter with empty list")
		}
	})

	// --- Test Quit ---
	t.Run("Quit on Empty List", func(t *testing.T) {
		_, cmdQ := simulateKeyPress(m, "q")
		isQuitQ := cmdQ != nil && reflect.ValueOf(cmdQ).Pointer() == reflect.ValueOf(tea.Quit).Pointer()
		if !isQuitQ {
			t.Errorf("Expected tea.Quit command after 'q' on empty list")
		}

		_, cmdCtrlC := simulateSpecialKeyPress(m, tea.KeyCtrlC)
		isQuitCtrlC := cmdCtrlC != nil && reflect.ValueOf(cmdCtrlC).Pointer() == reflect.ValueOf(tea.Quit).Pointer()
		if !isQuitCtrlC {
			t.Errorf("Expected tea.Quit command after Ctrl+C on empty list")
		}
	})
}

// TODO: Add tests verifying the returned tea.Cmd

// TestRemoteAutoSelection tests the auto-selection of remote branches when local branches are selected
func TestRemoteAutoSelection(t *testing.T) {
	branches := createSampleBranches()
	m := createTestModel(branches)

	// Move to first selectable branch (feat/merged, index 1)
	mUpdated, _ := simulateSpecialKeyPress(m, tea.KeyDown)
	mAsserted, ok := mUpdated.(Model)
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted

	// Select local branch
	mUpdated, _ = simulateKeyPress(m, " ")
	mAsserted, ok = mUpdated.(Model)
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted

	// Get the original index of the branch
	originalIndex := m.ListOrder[1] // Should be 1

	// Verify local branch is selected
	if !m.SelectedLocal[originalIndex] {
		t.Errorf("Expected local branch to be selected")
	}

	// Verify remote branch is auto-selected
	if !m.SelectedRemote[originalIndex] {
		t.Errorf("Expected remote branch to be auto-selected when local is selected")
	}

	// Deselect local branch
	mUpdated, _ = simulateKeyPress(m, " ")
	mAsserted, ok = mUpdated.(Model)
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted

	// Verify local branch is deselected
	if m.SelectedLocal[originalIndex] {
		t.Errorf("Expected local branch to be deselected")
	}

	// Verify remote branch is auto-deselected
	if m.SelectedRemote[originalIndex] {
		t.Errorf("Expected remote branch to be auto-deselected when local is deselected")
	}
}

// TestRemoteStyleRendering tests that the rendering logic applies the appropriate styles
// This is a more complex test that checks the actual rendered output
func TestRemoteStyleRendering(t *testing.T) {
	branches := createSampleBranches()
	m := createTestModel(branches)

	// Render the initial view
	initialView := m.View()

	// Move to first selectable branch (feat/merged, index 1)
	mUpdated, _ := simulateSpecialKeyPress(m, tea.KeyDown)
	mAsserted, ok := mUpdated.(Model)
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted

	// Render view with cursor on first selectable branch
	cursorView := m.View()

	// Select local branch
	mUpdated, _ = simulateKeyPress(m, " ")
	mAsserted, ok = mUpdated.(Model)
	if !ok {
		t.Fatalf("Type assertion failed for mUpdated.(Model)")
	}
	m = mAsserted

	// Render view with local branch selected
	selectedView := m.View()

	// Check that the initial view contains dimmed remote text
	// This is a simple check that just verifies the remoteDimmedStyle is being used
	// We can't easily check the exact styling in the rendered output
	if !strings.Contains(initialView, "Remote:") {
		t.Errorf("Expected initial view to contain 'Remote:' text")
	}

	// Check that the selected view contains the remote branch selected
	// Again, we can't easily check the exact styling, but we can check for the presence of the text
	if !strings.Contains(selectedView, "Remote:") {
		t.Errorf("Expected selected view to contain 'Remote:' text")
	}

	// Verify that the views are different, indicating that styling has changed
	if initialView == selectedView {
		t.Errorf("Expected initial view and selected view to be different due to styling changes")
	}
	if cursorView == selectedView {
		t.Errorf("Expected cursor view and selected view to be different due to styling changes")
	}
}
