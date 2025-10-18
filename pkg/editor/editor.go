package editor

import (
	"fmt"
	"github.com/jellexet/golang-text-editor/pkg/buffer"
	"golang.org/x/sys/unix"
	"strings"
)

// Action represents an editing action for undo/redo
type Action struct {
	actionType string // "insert" or "delete"
	position   int    // position in rope
	content    string // content that was inserted or deleted
}

// Session contains the information to display the text, undo-redo and edit the text
type Session struct {
	rope       *buffer.Rope
	undoStack  []Action
	redoStack  []Action
	cursorIdx  int // linear index in the rope
	cursorRow  int // 1-indexed row
	cursorCol  int // 1-indexed column
	screenRows uint16
	screenCols uint16
}

var session Session

// EnableRawMode sets the terminal into raw mode
func EnableRawMode(fd int) (*unix.Termios, error) {
	oldState, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}

	newState := *oldState
	newState.Lflag &^= unix.ECHO | unix.ICANON
	newState.Lflag &^= unix.ISIG | unix.IEXTEN
	newState.Iflag &^= unix.IXON
	newState.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP
	newState.Oflag &^= unix.OPOST

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return nil, err
	}

	return oldState, nil
}

// DisableRawMode resets the terminal to previous state
func DisableRawMode(fd int, prevState *unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TCSETS, prevState)
}

// Control character constants
const (
	CtrlQ byte = 0x11
	CtrlZ byte = 0x1A
	CtrlR byte = 0x12
	Esc   byte = 0x1B
)

// Special character constants
const (
	Return    byte = 0x0D
	Backspace byte = 0x7F
	Newline   byte = 0x0A
)

// Screen clearing constants
const (
	Line        rune = '0'
	BelowCursor rune = '1'
	Screen      rune = '2'
)

// Initialize session with rope and screen dimensions
func InitSession(fd int) {
	session.rope = buffer.NewRope("")
	session.cursorIdx = 0
	session.cursorRow = 1
	session.cursorCol = 1
	session.undoStack = []Action{}
	session.redoStack = []Action{}
	rows, cols := getWindowSize(fd)
	session.screenRows = rows
	session.screenCols = cols
}

// ProcessKeypress handles keyboard input and updates editor state
func ProcessKeypress(callback func() (key byte)) {
	fd := int(unix.Stdin)
	InitSession(fd)

	for {
		key := callback()

		// Handle escape sequences (arrow keys)
		if key == Esc {
			seq1 := callback()
			seq2 := callback()

			if seq1 == '[' {
				switch seq2 {
				case 'A': // Up arrow
					handleArrowUp()
				case 'B': // Down arrow
					handleArrowDown()
				case 'C': // Right arrow
					handleArrowRight()
				case 'D': // Left arrow
					handleArrowLeft()
				}
			}
			refreshScreen(fd)
			continue
		}

		// Handle control characters
		switch key {
		case CtrlQ:
			return
		case CtrlZ:
			handleUndo()
			refreshScreen(fd)
		case CtrlR:
			handleRedo()
			refreshScreen(fd)
		case Backspace:
			handleBackspace()
			refreshScreen(fd)
		case Return:
			handleInsert("\n")
			refreshScreen(fd)
		default:
			// Regular character
			if key >= 32 && key < 127 {
				handleInsert(string(key))
				refreshScreen(fd)
			}
		}
	}
}

// handleInsert inserts a character at cursor position
func handleInsert(s string) {
	if session.rope == nil {
		session.rope = buffer.NewRope(s)
	} else {
		newRope, err := session.rope.Insert(session.cursorIdx, s)
		if err == nil {
			// Record action for undo
			action := Action{
				actionType: "insert",
				position:   session.cursorIdx,
				content:    s,
			}
			session.undoStack = append(session.undoStack, action)
			session.redoStack = []Action{} // Clear redo stack on new action

			session.rope = newRope
			session.cursorIdx += len(s)
			updateCursorPosition()
		}
	}
}

// handleBackspace deletes character before cursor
func handleBackspace() {
	if session.cursorIdx > 0 {
		// Get the character being deleted for undo
		deletedChar, _ := session.rope.Index(session.cursorIdx - 1)

		newRope, err := session.rope.Delete(session.cursorIdx-1, session.cursorIdx)
		if err == nil {
			// Record action for undo
			action := Action{
				actionType: "delete",
				position:   session.cursorIdx - 1,
				content:    string(deletedChar),
			}
			session.undoStack = append(session.undoStack, action)
			session.redoStack = []Action{} // Clear redo stack

			session.rope = newRope
			session.cursorIdx--
			updateCursorPosition()
		}
	}
}

// handleUndo undoes the last action
func handleUndo() {
	if len(session.undoStack) == 0 {
		return
	}

	// Pop last action
	action := session.undoStack[len(session.undoStack)-1]
	session.undoStack = session.undoStack[:len(session.undoStack)-1]

	// Perform reverse operation
	if action.actionType == "insert" {
		// Undo insert by deleting
		newRope, err := session.rope.Delete(action.position, action.position+len(action.content))
		if err == nil {
			session.rope = newRope
			session.cursorIdx = action.position
		}
	} else if action.actionType == "delete" {
		// Undo delete by inserting
		newRope, err := session.rope.Insert(action.position, action.content)
		if err == nil {
			session.rope = newRope
			session.cursorIdx = action.position + len(action.content)
		}
	}

	// Add to redo stack
	session.redoStack = append(session.redoStack, action)
	updateCursorPosition()
}

// handleRedo redoes the last undone action
func handleRedo() {
	if len(session.redoStack) == 0 {
		return
	}

	// Pop last undone action
	action := session.redoStack[len(session.redoStack)-1]
	session.redoStack = session.redoStack[:len(session.redoStack)-1]

	// Perform the action again
	if action.actionType == "insert" {
		newRope, err := session.rope.Insert(action.position, action.content)
		if err == nil {
			session.rope = newRope
			session.cursorIdx = action.position + len(action.content)
		}
	} else if action.actionType == "delete" {
		newRope, err := session.rope.Delete(action.position, action.position+len(action.content))
		if err == nil {
			session.rope = newRope
			session.cursorIdx = action.position
		}
	}

	// Add back to undo stack
	session.undoStack = append(session.undoStack, action)
	updateCursorPosition()
}

// Arrow key handlers
func handleArrowLeft() {
	if session.cursorIdx > 0 {
		session.cursorIdx--
		updateCursorPosition()
	}
}

func handleArrowRight() {
	if session.cursorIdx < session.rope.Length() {
		session.cursorIdx++
		updateCursorPosition()
	}
}

func handleArrowUp() {
	lines := getLines()
	if session.cursorRow > 1 {
		session.cursorRow--
		// Try to maintain column position
		if session.cursorRow-1 < len(lines) {
			lineStart := getLineStartIndex(session.cursorRow)
			lineLen := len(lines[session.cursorRow-1])
			if session.cursorCol-1 <= lineLen {
				session.cursorIdx = lineStart + session.cursorCol - 1
			} else {
				session.cursorIdx = lineStart + lineLen
				session.cursorCol = lineLen + 1
			}
		}
	}
}

func handleArrowDown() {
	lines := getLines()
	if session.cursorRow < len(lines) {
		session.cursorRow++
		// Try to maintain column position
		if session.cursorRow-1 < len(lines) {
			lineStart := getLineStartIndex(session.cursorRow)
			lineLen := len(lines[session.cursorRow-1])
			if session.cursorCol-1 <= lineLen {
				session.cursorIdx = lineStart + session.cursorCol - 1
			} else {
				session.cursorIdx = lineStart + lineLen
				session.cursorCol = lineLen + 1
			}
		}
	}
}

// updateCursorPosition updates row and column based on linear index
func updateCursorPosition() {
	text := session.rope.String()
	row := 1
	col := 1

	for i := 0; i < session.cursorIdx && i < len(text); i++ {
		if text[i] == '\n' {
			row++
			col = 1
		} else {
			col++
		}
	}

	session.cursorRow = row
	session.cursorCol = col
}

// getLines splits the rope content into lines
func getLines() []string {
	text := session.rope.String()
	if text == "" {
		return []string{""}
	}
	return strings.Split(text, "\n")
}

// getLineStartIndex returns the starting index of a given row (1-indexed)
func getLineStartIndex(row int) int {
	lines := getLines()
	idx := 0
	for i := 0; i < row-1 && i < len(lines); i++ {
		idx += len(lines[i]) + 1 // +1 for newline
	}
	return idx
}

// refreshScreen redraws the entire screen
func refreshScreen(fd int) {
	ClearScreen(Screen)
	MoveCursorTopLeft()

	lines := getLines()
	rows, _ := getWindowSize(fd)

	// Draw content lines
	for i := 0; i < len(lines) && i < int(rows)-1; i++ {
		fmt.Print(lines[i] + "\r\n")
	}

	// Draw tildes for empty lines
	for i := len(lines); i < int(rows)-1; i++ {
		fmt.Print("~\r\n")
	}

	// Draw status bar
	fmt.Printf("\r\n\x1b[7mRow:%d Col:%d Idx:%d Len:%d | Ctrl-Q:Quit Ctrl-Z:Undo Ctrl-R:Redo\x1b[m",
		session.cursorRow, session.cursorCol, session.cursorIdx, session.rope.Length())

	// Move cursor to correct position
	moveCursor(session.cursorRow, session.cursorCol)
}

// ClearScreen clears the screen
func ClearScreen(element rune) {
	fmt.Printf("\x1b[%cJ", element)
}

// MoveCursorTopLeft moves cursor to top left
func MoveCursorTopLeft() {
	fmt.Print("\x1b[H")
}

// moveCursor moves cursor to specific position
func moveCursor(row, col int) {
	fmt.Printf("\x1b[%d;%dH", row, col)
}

// DrawTildes draws tildes for empty lines
func DrawTildes(fd int) {
	rows, _ := getWindowSize(fd)
	for row := uint16(1); row < rows; row++ {
		fmt.Print("~\r\n")
	}
}

// getWindowSize returns terminal dimensions
func getWindowSize(fd int) (rows, cols uint16) {
	winSize, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return 24, 80 // default fallback
	}
	return winSize.Row, winSize.Col
}
