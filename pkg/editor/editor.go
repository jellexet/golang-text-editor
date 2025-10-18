package editor

import (
	"fmt"
	"github.com/jellexet/golang-text-editor/pkg/buffer"
	"golang.org/x/sys/unix"
	"os"
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
	rope            *buffer.Rope
	undoStack       []Action
	redoStack       []Action
	cursorIdx       int // linear index in the rope
	cursorRow       int // 1-indexed row (screen position)
	cursorCol       int // 1-indexed column (screen position)
	screenRows      uint16
	screenCols      uint16
	filename        string // Name of the file being edited
	statusMessage   string // For showing messages like "Not found"
	lastSearchQuery string // For "find next"
}

// The session global variable
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

	// Read() will block for at most 100ms (1/10th of a second)
	// If no key is pressed, it returns 0 bytes.
	newState.Cc[unix.VMIN] = 0
	newState.Cc[unix.VTIME] = 1

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
	CtrlF byte = 0x06
	CtrlQ byte = 0x11
	CtrlR byte = 0x12
	CtrlS byte = 0x13
	CtrlZ byte = 0x1A
	Esc   byte = 0x1B
)

// Special character constants
const (
	Return    byte = 0x0D
	Backspace byte = 0x7F
)

// Arrow key constants
const (
	ArrowUp    = 1000
	ArrowDown  = 1001
	ArrowLeft  = 1002
	ArrowRight = 1003
)

// Screen clearing constants
const (
	Line        rune = '0'
	BelowCursor rune = '1'
	Screen      rune = '2'
)

// Initialize session with rope and screen dimensions
func InitSession(fd int, filename string, initialContent string) {
	session.rope = buffer.NewRope(initialContent)
	session.filename = filename
	session.cursorIdx = 0
	session.cursorRow = 1
	session.cursorCol = 1
	session.undoStack = []Action{}
	session.redoStack = []Action{}
	rows, cols := getWindowSize(fd)
	session.screenRows = rows
	session.screenCols = cols
	updateCursorPosition()
}

// editorReadKey reads a key from stdin, intelligently handling multi-byte
// ANSI escape sequences. This is necessary because special keys, like the
// arrow keys, are not sent as a single byte.
//
// For example:
//   - Arrow Up    is sent as 3 bytes: \x1b [ A
//   - Arrow Down  is sent as 3 bytes: \x1b [ B
//   - ...and so on.
//
// This function reads the first byte. If it's '\x1b' (Esc), it uses the
// non-blocking callback (which respects the VMIN/VTIME timeout) to
// check for the subsequent bytes ('[' and 'A'/'B'/'C'/'D').
//
// This allows it to distinguish between a user just pressing the 'Esc' key
// (where the subsequent reads will time out) and a user pressing an arrow
// key (where the sequence is read successfully).
func editorReadKey(callback func() byte) int {
	c := callback()

	// If we time out (c == 0), just return 0
	if c == 0 {
		return 0
	}

	// If it's not an escape sequence, return the key
	if c != Esc {
		return int(c)
	}

	// It's an escape key. Try to read the next two bytes.
	// These will also time out if no byte is available.
	seq1 := callback()
	if seq1 == 0 {
		return int(Esc) // Just an Esc key was pressed
	}

	seq2 := callback()
	if seq2 == 0 {
		return int(Esc) // Incomplete sequence, treat as Esc
	}

	// Check for \x1b[... sequences
	if seq1 == '[' {
		switch seq2 {
		case 'A':
			return ArrowUp
		case 'B':
			return ArrowDown
		case 'C':
			return ArrowRight
		case 'D':
			return ArrowLeft
		}
	}

	// If it's not a recognized sequence, just return Esc
	return int(Esc)
}

// ProcessKeypress handles keyboard input and updates editor state
func ProcessKeypress(fd int, callback func() (key byte)) {

	// Initial screen draw
	refreshScreen(fd)

	for {
		key := editorReadKey(callback)

		if key == 0 {
			continue
		}

		// Handle arrow keys
		if key >= 1000 {
			switch key {
			case ArrowUp:
				editorMoveCursor(ArrowUp)
			case ArrowDown:
				editorMoveCursor(ArrowDown)
			case ArrowLeft:
				editorMoveCursor(ArrowLeft)
			case ArrowRight:
				editorMoveCursor(ArrowRight)
			}
			refreshScreen(fd)
			continue
		}

		// Handle control characters
		c := byte(key)
		switch c {
		case CtrlQ:
			return
		case CtrlF:
			handleSearch(callback)
			refreshScreen(fd)
		case CtrlR:
			handleRedo()
			refreshScreen(fd)
		case CtrlS:
			handleSave(callback)
			refreshScreen(fd)
		case CtrlZ:
			handleUndo()
			refreshScreen(fd)
		case Backspace:
			handleBackspace()
			refreshScreen(fd)
		case Return:
			handleInsert("\n")
			refreshScreen(fd)
		default:
			// Regular character
			if c >= 32 && c < 127 {
				handleInsert(string(c))
				refreshScreen(fd)
			}
		}
	}
}

// editorMoveCursor moves the cursor based on arrow key
func editorMoveCursor(key int) {
	lines := getLines()
	currentLine := ""
	if session.cursorRow > 0 && session.cursorRow <= len(lines) {
		currentLine = lines[session.cursorRow-1]
	}

	switch key {
	case ArrowLeft:
		if session.cursorCol > 1 {
			session.cursorCol--
			session.cursorIdx--
		} else if session.cursorRow > 1 {
			// Move to end of previous line
			session.cursorRow--
			prevLine := lines[session.cursorRow-1]
			session.cursorCol = len(prevLine) + 1
			session.cursorIdx--
		}

	case ArrowRight:
		if session.cursorCol <= len(currentLine) {
			session.cursorCol++
			session.cursorIdx++
		} else if session.cursorRow < len(lines) {
			// Move to start of next line
			session.cursorRow++
			session.cursorCol = 1
			session.cursorIdx++
		}

	case ArrowUp:
		if session.cursorRow > 1 {
			session.cursorRow--
			// Adjust column if new line is shorter
			prevLine := lines[session.cursorRow-1]
			if session.cursorCol > len(prevLine)+1 {
				session.cursorCol = len(prevLine) + 1
			}
			// Recalculate cursorIdx
			session.cursorIdx = getLineStartIndex(session.cursorRow) + session.cursorCol - 1
		}

	case ArrowDown:
		if session.cursorRow < len(lines) {
			session.cursorRow++
			// Adjust column if new line is shorter
			if session.cursorRow <= len(lines) {
				nextLine := lines[session.cursorRow-1]
				if session.cursorCol > len(nextLine)+1 {
					session.cursorCol = len(nextLine) + 1
				}
			}
			// Recalculate cursorIdx
			session.cursorIdx = getLineStartIndex(session.cursorRow) + session.cursorCol - 1
		}
	}

	// Bounds check
	if session.cursorIdx < 0 {
		session.cursorIdx = 0
	}
	if session.cursorIdx > session.rope.Length() {
		session.cursorIdx = session.rope.Length()
	}
}

// handleInsert inserts a character at cursor position
func handleInsert(s string) {
	if session.rope == nil || session.rope.Length() == 0 {
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
		}
	}

	session.cursorIdx += len(s)
	updateCursorPosition()
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

// Saves the current buffer content to a file.
func handleSave(callback func() byte) {
	if session.filename == "[No Name]" {
		filename := editorDrawPrompt("Save as (Esc to cancel):", callback)
		if filename == "" {
			session.statusMessage = "Save canceled"
			return
		}
		session.filename = filename
	}

	content := session.rope.String()

	err := os.WriteFile(session.filename, []byte(content), 0644)
	if err != nil {
		session.statusMessage = fmt.Sprintf("Error saving file: %v", err)
		return
	}

	session.statusMessage = fmt.Sprintf("Saved %d bytes to %s", len(content), session.filename)
}

// Draws a prompt on the status bar and waits for user input
func editorDrawPrompt(prompt string, callback func() byte) string {
	var input string
	for {
		// Display the prompt on the status line
		msg := fmt.Sprintf("%s %s", prompt, input)

		var buf strings.Builder
		buf.WriteString(fmt.Sprintf("\x1b[%d;1H", session.screenRows)) // Go to status line
		buf.WriteString("\x1b[7m")                                     // Inverted colors
		buf.WriteString(msg)
		buf.WriteString("\x1b[K") // Clear rest of line
		buf.WriteString("\x1b[m") // Reset colors
		// Move cursor to end of input
		buf.WriteString(fmt.Sprintf("\x1b[%d;%dH", session.screenRows, len(msg)+1))
		buf.WriteString("\x1b[?25h") // Show cursor
		fmt.Print(buf.String())

		key := editorReadKey(callback)

		switch key {
		case int(Return):
			return input // Done
		case int(Esc):
			return "" // Canceled
		case int(Backspace):
			if len(input) > 0 {
				input = input[:len(input)-1]
			}
		case 0, ArrowUp, ArrowDown, ArrowLeft, ArrowRight:
			// Ignore timeouts and arrow keys in prompt mode
			continue
		default:
			if key >= 32 && key < 127 { // Printable char
				input += string(byte(key))
			}
		}
	}
}

// Prompts user for search query and moves cursor to result
func handleSearch(callback func() byte) {
	// Save cursor position in case of cancel/not found
	oldCursorIdx := session.cursorIdx

	query := editorDrawPrompt("Search (Esc to cancel):", callback)

	if query == "" {
		// User hit Esc
		session.statusMessage = "Search canceled"
		return
	}

	session.lastSearchQuery = query // Save for next time

	text := session.rope.String()

	// Start search from *after* the current cursor position
	searchFrom := session.cursorIdx + 1
	if searchFrom >= session.rope.Length() {
		searchFrom = 0 // Wrap around if at end
	}

	idx := strings.Index(text[searchFrom:], query)

	if idx != -1 {
		// Found
		session.cursorIdx = searchFrom + idx // Adjust index
		updateCursorPosition()
		session.statusMessage = "" // Clear status
	} else {
		// Not found from cursor. Try from beginning.
		idx = strings.Index(text, query)
		if idx != -1 {
			// Found at beginning
			session.cursorIdx = idx
			updateCursorPosition()
			session.statusMessage = "Search wrapped to top"
		} else {
			// Not found at all
			session.statusMessage = "Not found: " + query
			session.cursorIdx = oldCursorIdx // Restore cursor
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
	var buf strings.Builder

	// Hide cursor during refresh
	buf.WriteString("\x1b[?25l")
	// Clear screen and move cursor to top-left
	buf.WriteString("\x1b[2J")
	buf.WriteString("\x1b[H")

	lines := getLines()
	rows, _ := getWindowSize(fd)

	// Draw content lines (leave one row for status bar)
	for i := 0; i < int(rows)-1; i++ {
		if i < len(lines) {
			buf.WriteString(lines[i])
		} else {
			buf.WriteString("~")
		}
		buf.WriteString("\x1b[K") // Clear rest of the line
		buf.WriteString("\r\n")
	}

	// Draw status bar (inverted colors)
	var statusMsg string
	if session.statusMessage != "" {
		// Show a temporary message (e.g., "Not found")
		statusMsg = session.statusMessage
		session.statusMessage = "" // Clear it after displaying once
	} else {
		// Show default status
		statusMsg = fmt.Sprintf("File: %s | Row:%d Col:%d | Ctrl-Q:Quit Ctrl-S:Save Ctrl-F:Find",
			session.filename, session.cursorRow, session.cursorCol)
	}

	// Truncate status if too long
	if len(statusMsg) > int(session.screenCols) {
		statusMsg = statusMsg[:session.screenCols]
	}

	buf.WriteString("\x1b[7m") // Inverted colors
	buf.WriteString(statusMsg)
	// Pad with spaces to fill the line
	for i := len(statusMsg); i < int(session.screenCols); i++ {
		buf.WriteString(" ")
	}
	buf.WriteString("\x1b[m") // Reset colors

	// Move cursor to correct position
	buf.WriteString(fmt.Sprintf("\x1b[%d;%dH", session.cursorRow, session.cursorCol))
	// Show cursor
	buf.WriteString("\x1b[?25h")

	// Write everything at once
	fmt.Print(buf.String())
}

// ClearScreen clears the screen
func ClearScreen(element rune) {
	fmt.Printf("\x1b[%cJ", element)
}

// MoveCursorTopLeft moves cursor to top left
func MoveCursorTopLeft() {
	fmt.Print("\x1b[H")
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
