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

// handleSearch test (simulate typing "lo" then Return)
func TestHandleSearch_FindsAndWraps(t *testing.T) {
	resetSessionForTest()

	content := "hello\nworld\nhello"
	session.rope = buffer.NewRope(content)
	session.cursorIdx = 0
	updateCursorPosition()

	// Simulate typing "lo" then Return
	seq := []byte{'l', 'o', Return}
	cb := makeCallback(seq)

	handleSearch(cb)
	// After searching from cursorIdx 0, first match of "lo" after position 1 is at "hello" index 3
	foundIdx := strings.Index(content, "lo")
	if session.cursorIdx != foundIdx {
		t.Fatalf("search did not move cursor to first occurrence: got %d want %d", session.cursorIdx, foundIdx)
	}

	// Now move cursor to after last occurrence and search to cause wrap
	lastOcc := strings.LastIndex(content, "lo")
	session.cursorIdx = lastOcc + 1
	updateCursorPosition()
	cb2 := makeCallback([]byte{'l', 'o', Return})
	handleSearch(cb2)
	// Because searching starts from cursorIdx+1, should wrap and find earlier occurrence at index foundIdx
	if session.cursorIdx != foundIdx {
		t.Fatalf("search wrap did not move cursor to wrapped occurrence: got %d want %d", session.cursorIdx, foundIdx)
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
