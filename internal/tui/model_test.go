package tui

import (
	"context" // Added import
	"reflect"
	"testing"
	"time"

	"github.com/bral/git-sweep-go/internal/types"
	tea "github.com/charmbracelet/bubbletea"
)

// Helper to create a basic model for testing
func createTestModel(branches []types.AnalyzedBranch) Model {
	// Simplified context for testing
	ctx := context.Background()
	// Create a basic model, assuming not dry run for most tests
	model := InitialModel(ctx, branches, false)
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
		{BranchInfo: types.BranchInfo{Name: "main", LastCommitDate: now, Remote: "origin"}, Category: types.CategoryProtected, IsCurrent: true, IsProtected: true},
		// Suggested (Original Index 1)
		{BranchInfo: types.BranchInfo{Name: "feat/merged", LastCommitDate: ninetyDaysAgo, Remote: "origin"}, Category: types.CategoryMergedOld, IsMerged: true},
		// Suggested (Original Index 2)
		{BranchInfo: types.BranchInfo{Name: "feat/unmerged-old", LastCommitDate: ninetyDaysAgo, Remote: "origin"}, Category: types.CategoryUnmergedOld, IsMerged: false},
		// Active (Original Index 3)
		{BranchInfo: types.BranchInfo{Name: "feat/active", LastCommitDate: sixtyDaysAgo, Remote: ""}, Category: types.CategoryActive, IsMerged: false},
		// Suggested (Original Index 4) - No Remote
		{BranchInfo: types.BranchInfo{Name: "feat/merged-no-remote", LastCommitDate: ninetyDaysAgo, Remote: ""}, Category: types.CategoryMergedOld, IsMerged: true},
	}
}

func TestTuiNavigation(t *testing.T) {
	branches := createSampleBranches()
	m := createTestModel(branches)
	// Expected order: main (0), feat/merged (1), feat/unmerged-old (2), feat/merged-no-remote (4), feat/active (3)
	// Indices in listOrder: [0, 1, 2, 4, 3]
	totalItems := len(m.listOrder) // Should be 5

	if m.cursor != 0 {
		t.Fatalf("Initial cursor expected 0, got %d", m.cursor)
	}

	// Move down
	mUpdated, _ := simulateSpecialKeyPress(m, tea.KeyDown)
	m = mUpdated.(Model)
	if m.cursor != 1 {
		t.Errorf("Cursor after down: expected 1, got %d", m.cursor)
	}

	// Move down multiple times
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyDown)
	mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyDown)
	m = mUpdated.(Model)
	if m.cursor != 3 {
		t.Errorf("Cursor after 3 downs: expected 3, got %d", m.cursor)
	}

	// Move past end
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyDown)
	mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyDown) // Try again
	m = mUpdated.(Model)
	if m.cursor != totalItems-1 { // Should be 4
		t.Errorf("Cursor after down past end: expected %d, got %d", totalItems-1, m.cursor)
	}

	// Move up
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyUp)
	m = mUpdated.(Model)
	if m.cursor != totalItems-2 { // Should be 3
		t.Errorf("Cursor after up: expected %d, got %d", totalItems-2, m.cursor)
	}

	// Move past beginning
	for i := 0; i < totalItems; i++ {
		mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyUp)
	}
	m = mUpdated.(Model)
	if m.cursor != 0 {
		t.Errorf("Cursor after up past beginning: expected 0, got %d", m.cursor)
	}
}

func TestTuiSelection(t *testing.T) {
	branches := createSampleBranches()
	m := createTestModel(branches)
	// Expected order: main (0), feat/merged (1), feat/unmerged-old (2), feat/merged-no-remote (4), feat/active (3)
	// Indices in listOrder: [0, 1, 2, 4, 3]
	// Selectable original indices: 1, 2, 4

	// 1. Try selecting protected (main, index 0) - should fail
	mUpdated, _ := simulateKeyPress(m, " ")
	m = mUpdated.(Model)
	if len(m.selectedLocal) != 0 {
		t.Errorf("Expected 0 selected after trying to select protected, got %d", len(m.selectedLocal))
	}

	// 2. Move to first selectable (feat/merged, index 1) and select local
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyDown)
	mUpdated, _ = simulateKeyPress(mUpdated, " ")
	m = mUpdated.(Model)
	originalIndex := m.listOrder[1] // Should be 1
	if !m.selectedLocal[originalIndex] {
		t.Errorf("Expected item at cursor 1 (original index %d) to be selected locally", originalIndex)
	}
	if len(m.selectedLocal) != 1 {
		t.Errorf("Expected 1 selected local item, got %d", len(m.selectedLocal))
	}

	// 3. Select remote for the same item (should work)
	mUpdated, _ = simulateKeyPress(m, "r") // or simulateSpecialKeyPress(m, tea.KeyTab)
	m = mUpdated.(Model)
	if !m.selectedRemote[originalIndex] {
		t.Errorf("Expected item at cursor 1 (original index %d) to be selected remotely", originalIndex)
	}

	// 4. Move to next selectable (feat/unmerged-old, index 2) and select local
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyDown)
	mUpdated, _ = simulateKeyPress(mUpdated, " ")
	m = mUpdated.(Model)
	originalIndex2 := m.listOrder[2] // Should be 2
	if !m.selectedLocal[originalIndex2] {
		t.Errorf("Expected item at cursor 2 (original index %d) to be selected locally", originalIndex2)
	}
	if len(m.selectedLocal) != 2 {
		t.Errorf("Expected 2 selected local items, got %d", len(m.selectedLocal))
	}

	// 5. Try selecting remote for item 2 (should work)
	mUpdated, _ = simulateKeyPress(m, "r")
	m = mUpdated.(Model)
	if !m.selectedRemote[originalIndex2] {
		t.Errorf("Expected item at cursor 2 (original index %d) to be selected remotely", originalIndex2)
	}

	// 6. Move to non-selectable active branch (feat/active, index 4 -> original 3) and try selecting - should fail
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyDown) // cursor 3 (original 4)
	mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyDown) // cursor 4 (original 3)
	mUpdated, _ = simulateKeyPress(mUpdated, " ")
	m = mUpdated.(Model)
	if len(m.selectedLocal) != 2 {
		t.Errorf("Expected 2 selected local items after trying to select active, got %d", len(m.selectedLocal))
	}

	// 7. Move back to item 1 (original index 1) and deselect local (should deselect remote too)
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyUp) // cursor 3 (original 4)
	mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyUp) // cursor 2 (original 2)
	mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyUp) // cursor 1 (original 1)
	mUpdated, _ = simulateKeyPress(mUpdated, " ") // Deselect local
	m = mUpdated.(Model)
	if m.selectedLocal[originalIndex] {
		t.Errorf("Expected item at cursor 1 (original index %d) to be deselected locally", originalIndex)
	}
	if m.selectedRemote[originalIndex] {
		t.Errorf("Expected item at cursor 1 (original index %d) to be deselected remotely after local deselect", originalIndex)
	}
	if len(m.selectedLocal) != 1 { // Only item 2 should be left
		t.Errorf("Expected 1 selected local item after deselect, got %d", len(m.selectedLocal))
	}
	if len(m.selectedRemote) != 1 { // Only item 2 should be left
		t.Errorf("Expected 1 selected remote item after deselect, got %d", len(m.selectedRemote))
	}

	// 8. Move to item with no remote (feat/merged-no-remote, cursor 3, original 4) and select local
	mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyDown) // cursor 2 (original 2)
	mUpdated, _ = simulateSpecialKeyPress(mUpdated, tea.KeyDown) // cursor 3 (original 4)
	mUpdated, _ = simulateKeyPress(mUpdated, " ") // Select local
	m = mUpdated.(Model)
	originalIndex4 := m.listOrder[3] // Should be 4
	if !m.selectedLocal[originalIndex4] {
		t.Errorf("Expected item at cursor 3 (original index %d) to be selected locally", originalIndex4)
	}

	// 9. Try selecting remote (should fail as no remote exists)
	mUpdated, _ = simulateKeyPress(m, "r")
	m = mUpdated.(Model)
	if m.selectedRemote[originalIndex4] {
		t.Errorf("Expected item at cursor 3 (original index %d) to NOT be selected remotely", originalIndex4)
	}
	if len(m.selectedRemote) != 1 { // Still only item 2 selected remotely
		t.Errorf("Expected 1 selected remote item after trying on no-remote branch, got %d", len(m.selectedRemote))
	}
}

func TestTuiStateTransitions(t *testing.T) {
	branches := createSampleBranches()

	// --- Test Case 1: Enter with no selection -> No change ---
	t.Run("Enter with no selection", func(t *testing.T) {
		m := createTestModel(branches)
		if m.viewState != stateSelecting { t.Fatalf("Initial state not selecting") }

		mUpdated, cmd := simulateSpecialKeyPress(m, tea.KeyEnter)
		m = mUpdated.(Model)

		if m.viewState != stateSelecting {
			t.Errorf("Expected state selecting after enter with no selection, got %v", m.viewState)
		}
		if cmd != nil {
			t.Errorf("Expected nil cmd after enter with no selection, got %T", cmd)
		}
	})

	// --- Test Case 2: Select -> Enter -> Confirming ---
	t.Run("Select -> Enter -> Confirming", func(t *testing.T) {
		m := createTestModel(branches)
		mUpdated, _ := simulateSpecialKeyPress(m, tea.KeyDown) // Move to selectable
		mUpdated, _ = simulateKeyPress(mUpdated, " ")      // Select local
		m = mUpdated.(Model)
		if len(m.selectedLocal) == 0 { t.Fatalf("Setup failed: No item selected") }

		mUpdated, cmd := simulateSpecialKeyPress(m, tea.KeyEnter)
		m = mUpdated.(Model)

		if m.viewState != stateConfirming {
			t.Errorf("Expected state confirming after enter with selection, got %v", m.viewState)
		}
		if cmd != nil {
			t.Errorf("Expected nil cmd after enter with selection, got %T", cmd)
		}
	})

	// --- Test Case 3: Confirming -> n -> Selecting ---
	t.Run("Confirming -> n -> Selecting", func(t *testing.T) {
		m := createTestModel(branches)
		m.viewState = stateConfirming // Force state

		mUpdated, cmd := simulateKeyPress(m, "n")
		m = mUpdated.(Model)

		if m.viewState != stateSelecting {
			t.Errorf("Expected state selecting after 'n' from confirming, got %v", m.viewState)
		}
		if cmd != nil {
			t.Errorf("Expected nil cmd after 'n' from confirming, got %T", cmd)
		}
	})

	// --- Test Case 4: Confirming -> Esc -> Selecting ---
	t.Run("Confirming -> Esc -> Selecting", func(t *testing.T) {
		m := createTestModel(branches)
		m.viewState = stateConfirming // Force state

		mUpdated, cmd := simulateSpecialKeyPress(m, tea.KeyEsc)
		m = mUpdated.(Model)

		if m.viewState != stateSelecting {
			t.Errorf("Expected state selecting after 'Esc' from confirming, got %v", m.viewState)
		}
		if cmd != nil {
			t.Errorf("Expected nil cmd after 'Esc' from confirming, got %T", cmd)
		}
	})

	// --- Test Case 5: Confirming -> y -> Deleting (and check cmd) ---
	t.Run("Confirming -> y -> Deleting", func(t *testing.T) {
		m := createTestModel(branches)
		// Select something first
		mUpdated, _ := simulateSpecialKeyPress(m, tea.KeyDown) // Move to selectable (original index 1)
		mUpdated, _ = simulateKeyPress(mUpdated, " ")      // Select local
		m = mUpdated.(Model)
		m.viewState = stateConfirming // Force state

		mUpdated, cmd := simulateKeyPress(m, "y")
		m = mUpdated.(Model)

		if m.viewState != stateDeleting {
			t.Errorf("Expected state deleting after 'y' from confirming, got %v", m.viewState)
		}
		// Check if a command was returned (should be a tea.Batch containing performDeletionCmd and spinner.Tick)
		if cmd == nil {
			t.Errorf("Expected a command after 'y' from confirming, got nil")
		}
		// Note: Directly checking the type of the function within tea.Cmd or tea.Batch is complex.
		// We rely on the fact that *some* command is returned. More advanced testing could use interfaces or mocks.
	})

	// --- Test Case 6: Deleting -> resultsMsg -> Results ---
	t.Run("Deleting -> resultsMsg -> Results", func(t *testing.T) {
		m := createTestModel(branches)
		m.viewState = stateDeleting // Force state

		results := []types.DeleteResult{{BranchName: "test", Success: true}}
		mUpdated, cmd := m.Update(resultsMsg{results: results})
		m = mUpdated.(Model)

		if m.viewState != stateResults {
			t.Errorf("Expected state results after resultsMsg, got %v", m.viewState)
		}
		if cmd != nil {
			t.Errorf("Expected nil cmd after resultsMsg, got %T", cmd)
		}
		if len(m.results) != 1 || m.results[0].BranchName != "test" {
			t.Errorf("Results not stored correctly in model")
		}
	})

	// --- Test Case 7: Results -> Any Key -> Quit ---
	t.Run("Results -> Any Key -> Quit", func(t *testing.T) {
		m := createTestModel(branches)
		m.viewState = stateResults // Force state

		mUpdated, cmd := simulateKeyPress(m, "a") // Simulate pressing 'a'

		// Check if the returned command is tea.Quit by comparing function pointers
		isQuit := cmd != nil && reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(tea.Quit).Pointer()

		if !isQuit {
			t.Errorf("Expected tea.Quit command after key press in results state, got %T", cmd)
		}
		// Model state doesn't change here, the command signals exit
		if mUpdated.(Model).viewState != stateResults {
			t.Errorf("Model state should remain results")
		}
	})

	// --- Test Case 8: Selecting -> q -> Quit ---
	t.Run("Selecting -> q -> Quit", func(t *testing.T) {
		m := createTestModel(branches)
		mUpdated, cmd := simulateKeyPress(m, "q")

		// Check if the returned command is tea.Quit by comparing function pointers
		isQuit := cmd != nil && reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(tea.Quit).Pointer()

		if !isQuit {
			t.Errorf("Expected tea.Quit command after 'q' in selecting state, got %T", cmd)
		}
		if mUpdated.(Model).viewState != stateSelecting {
			t.Errorf("Model state should remain selecting")
		}
	})

	// --- Test Case 9: Selecting -> Ctrl+C -> Quit ---
	t.Run("Selecting -> Ctrl+C -> Quit", func(t *testing.T) {
		m := createTestModel(branches)
		mUpdated, cmd := simulateSpecialKeyPress(m, tea.KeyCtrlC)

		// Check if the returned command is tea.Quit by comparing function pointers
		isQuit := cmd != nil && reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(tea.Quit).Pointer()

		if !isQuit {
			t.Errorf("Expected tea.Quit command after Ctrl+C, got %T", cmd)
		}
		if mUpdated.(Model).viewState != stateSelecting {
			t.Errorf("Model state should remain selecting")
		}
	})

}

func TestTuiEmptyList(t *testing.T) {
	// Create model with no branches
	m := createTestModel([]types.AnalyzedBranch{})

	if m.viewState != stateSelecting { t.Fatalf("Initial state not selecting") }
	if len(m.listOrder) != 0 { t.Fatalf("Expected listOrder to be empty") }
	if m.cursor != 0 { t.Fatalf("Expected cursor to be 0") }

	// --- Test Navigation ---
	t.Run("Navigation on Empty List", func(t *testing.T) {
		mUpdated, _ := simulateSpecialKeyPress(m, tea.KeyDown)
		if mUpdated.(Model).cursor != 0 {
			t.Errorf("Cursor moved down on empty list")
		}
		mUpdated, _ = simulateSpecialKeyPress(m, tea.KeyUp)
		if mUpdated.(Model).cursor != 0 {
			t.Errorf("Cursor moved up on empty list")
		}
	})

	// --- Test Selection ---
	t.Run("Selection on Empty List", func(t *testing.T) {
		mUpdated, _ := simulateKeyPress(m, " ")
		if len(mUpdated.(Model).selectedLocal) != 0 {
			t.Errorf("Local selection occurred on empty list")
		}
		mUpdated, _ = simulateKeyPress(m, "r")
		if len(mUpdated.(Model).selectedRemote) != 0 {
			t.Errorf("Remote selection occurred on empty list")
		}
	})

	// --- Test Enter ---
	t.Run("Enter on Empty List", func(t *testing.T) {
		mUpdated, cmd := simulateSpecialKeyPress(m, tea.KeyEnter)
		if mUpdated.(Model).viewState != stateSelecting {
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
