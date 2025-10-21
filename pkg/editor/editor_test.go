package editor

import (
	"strings"
	"testing"

	"github.com/jellexet/golang-text-editor/pkg/buffer"
)

// helper: create callback returning bytes from seq sequentially, then 0
func makeCallback(seq []byte) func() byte {
	i := 0
	return func() byte {
		if i >= len(seq) {
			return 0
		}
		b := seq[i]
		i++
		return b
	}
}

func resetSessionForTest() {
	session = Session{}
	// provide safe defaults so functions using screenCols/Rows don't panic
	session.screenRows = 24
	session.screenCols = 80
}

// editorReadKey tests
func TestEditorReadKey_PrintableAndEscAndArrow(t *testing.T) {
	resetSessionForTest()

	t.Run("printable char", func(t *testing.T) {
		cb := makeCallback([]byte{'a'})
		got := editorReadKeypress(cb)
		if got != int('a') {
			t.Fatalf("expected %d got %d", int('a'), got)
		}
	})

	t.Run("esc then timeout", func(t *testing.T) {
		cb := makeCallback([]byte{Esc, 0})
		got := editorReadKeypress(cb)
		if got != int(Esc) {
			t.Fatalf("expected Esc (%d) got %d", int(Esc), got)
		}
	})

	t.Run("arrow up sequence", func(t *testing.T) {
		cb := makeCallback([]byte{Esc, '[', 'A'})
		got := editorReadKeypress(cb)
		if got != ArrowUp {
			t.Fatalf("expected ArrowUp (%d) got %d", ArrowUp, got)
		}
	})
}

// handleInsert, handleBackspace, handleUndo/Redo tests
func TestInsertBackspaceUndoRedo(t *testing.T) {
	resetSessionForTest()

	// start with "hello"
	session.rope = buffer.NewRope("hello")
	session.cursorIdx = session.rope.Length()
	updateCursorPosition()

	handleInsert(" world")
	// After insert
	if session.rope.String() != "hello world" {
		t.Fatalf("insert failed: got %q", session.rope.String())
	}
	if session.cursorIdx != len("hello world") {
		t.Fatalf("cursorIdx after insert wrong: got %d expected %d", session.cursorIdx, len("hello world"))
	}
	if len(session.undoStack) == 0 || session.undoStack[len(session.undoStack)-1].actionType != "insert" {
		t.Fatalf("undo stack not updated after insert")
	}

	// Backspace: remove 'd'
	handleBackspace()
	if session.rope.String() != "hello worl" {
		t.Fatalf("backspace failed: got %q", session.rope.String())
	}
	if session.cursorIdx != len("hello worl") {
		t.Fatalf("cursorIdx after backspace wrong: got %d", session.cursorIdx)
	}
	// Last undo action should be delete
	last := session.undoStack[len(session.undoStack)-1]
	if last.actionType != "delete" || last.content == "" {
		t.Fatalf("undo stack did not record delete: %+v", last)
	}

	// Undo the delete (should reinsert 'd')
	handleUndo()
	if session.rope.String() != "hello world" {
		t.Fatalf("undo delete failed: got %q", session.rope.String())
	}

	// Undo the insert (should remove " world")
	handleUndo()
	if session.rope.String() != "hello" {
		t.Fatalf("undo insert failed: got %q", session.rope.String())
	}

	// Redo (should reapply insert)
	handleRedo()
	if session.rope.String() != "hello world" {
		t.Fatalf("redo insert failed: got %q", session.rope.String())
	}
}

// editorMoveCursor tests across lines and bounds
func TestEditorMoveCursor_MultiLine(t *testing.T) {
	resetSessionForTest()

	session.rope = buffer.NewRope("one\ntwo\nthree")
	// Set cursor to end of first line (after 'e')
	session.cursorIdx = strings.Index(session.rope.String(), "\n") // index of newline
	updateCursorPosition()
	if session.cursorRow != 1 {
		t.Fatalf("expected cursorRow 1 got %d", session.cursorRow)
	}

	// Move right: should go to beginning of next line
	editorMoveCursor(ArrowRight)
	if session.cursorRow != 2 {
		t.Fatalf("expected move to row 2 got %d", session.cursorRow)
	}
	// Move left: should go back to end of previous line
	editorMoveCursor(ArrowLeft)
	if session.cursorRow != 1 {
		t.Fatalf("expected back to row 1 got %d", session.cursorRow)
	}

	// Place cursor on second line, col past length then up should clamp
	session.cursorIdx = strings.Index(session.rope.String(), "two") + len("two")
	updateCursorPosition() // at end of "two"
	// Move up
	editorMoveCursor(ArrowUp)
	// After moving up, ensure cursorCol does not exceed prev line length +1
	if session.cursorRow != 1 {
		t.Fatalf("expected row 1 after ArrowUp got %d", session.cursorRow)
	}

	// Move down from row 1 to row 2, then to row 3 and ensure indexes valid
	session.cursorIdx = 0
	updateCursorPosition()
	editorMoveCursor(ArrowDown)
	if session.cursorRow != 2 {
		t.Fatalf("expected row 2 after ArrowDown got %d", session.cursorRow)
	}
	editorMoveCursor(ArrowDown)
	if session.cursorRow != 3 {
		t.Fatalf("expected row 3 after second ArrowDown got %d", session.cursorRow)
	}
	// Bounds check: moving down at last line should not change
	editorMoveCursor(ArrowDown)
	if session.cursorRow != 3 {
		t.Fatalf("expected row 3 to remain at bottom got %d", session.cursorRow)
	}
}

// getLines and getLineStartIndex tests
func TestGetLinesAndStartIndex(t *testing.T) {
	resetSessionForTest()

	session.rope = buffer.NewRope("a\nbb\nccc")
	lines := getLines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines got %d", len(lines))
	}
	if lines[0] != "a" || lines[1] != "bb" || lines[2] != "ccc" {
		t.Fatalf("unexpected lines: %+v", lines)
	}

	// start indices: row1 -> 0, row2 -> 2 (1 char + newline), row3 -> 5 (1 +1 +2 +1)
	if getLineStartIndex(1) != 0 {
		t.Fatalf("start index row1 expected 0 got %d", getLineStartIndex(1))
	}
	if getLineStartIndex(2) != 2 {
		t.Fatalf("start index row2 expected 2 got %d", getLineStartIndex(2))
	}
	if getLineStartIndex(3) != 5 {
		t.Fatalf("start index row3 expected 5 got %d", getLineStartIndex(3))
	}
}

// handleSearch test (simulate typing "lo" then Return, then Ctrl-N to cycle)
func TestHandleSearch_FindsAndCycles(t *testing.T) {
	resetSessionForTest()

	content := "hello\nworld\nhello"
	session.rope = buffer.NewRope(content)
	session.cursorIdx = 0
	updateCursorPosition()

	// The handleSearch function:
	// 1. Prompts for query (consumes: 'l', 'o', Return)
	// 2. Finds all instances and cycles through them
	// 3. After each instance, waits for Ctrl-N to continue or any other key to stop

	// There are 2 instances of "lo" in the content (at index 3 and 15)
	// We need: prompt input + Ctrl-N after first match + any key to exit after second match
	seq := []byte{
		'l', 'o', Return, // prompt input
		CtrlN, // continue to next match
		'q',   // exit after second match (any regular char works)
	}
	cb := makeCallback(seq)

	// Mock fd parameter (not actually used for I/O in test)
	mockFd := 0

	handleSearch(mockFd, cb)

	// After cycling through both matches, cursor should be at the second occurrence
	lastOcc := strings.LastIndex(content, "lo")
	if session.cursorIdx != lastOcc {
		t.Fatalf("search did not move cursor to last occurrence: got %d want %d", session.cursorIdx, lastOcc)
	}
}

// Test search not found
func TestHandleSearch_NotFound(t *testing.T) {
	resetSessionForTest()

	session.rope = buffer.NewRope("hello world")
	session.cursorIdx = 0
	updateCursorPosition()
	oldIdx := session.cursorIdx

	// Search for something that doesn't exist
	seq := []byte{'x', 'y', 'z', Return}
	cb := makeCallback(seq)

	handleSearch(0, cb)

	// Cursor should be restored to original position
	if session.cursorIdx != oldIdx {
		t.Fatalf("cursor moved after failed search: got %d want %d", session.cursorIdx, oldIdx)
	}

	// Status message should indicate not found
	if !strings.Contains(session.statusMessage, "Not found") {
		t.Fatalf("expected 'Not found' in status message, got %q", session.statusMessage)
	}
}

// Test search cancellation
func TestHandleSearch_Cancel(t *testing.T) {
	resetSessionForTest()

	session.rope = buffer.NewRope("hello world")
	session.cursorIdx = 5
	updateCursorPosition()

	// Press Esc to cancel
	seq := []byte{Esc}
	cb := makeCallback(seq)

	handleSearch(0, cb)

	// Status message should indicate cancellation
	if !strings.Contains(session.statusMessage, "canceled") {
		t.Fatalf("expected 'canceled' in status message, got %q", session.statusMessage)
	}
}

// editorDrawPrompt should return "" on Esc
func TestEditorDrawPrompt_EscCancel(t *testing.T) {
	resetSessionForTest()

	cb := makeCallback([]byte{Esc})
	result := editorDrawPrompt("Prompt:", cb)
	if result != "" {
		t.Fatalf("expected empty result on Esc cancel, got %q", result)
	}
}
