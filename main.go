package main

import (
	"bufio"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"unicode"
)

// enableRawMode sets the terminal connected to the given file descriptor (fd)
// into "raw mode". This disables canonical input processing (line buffering)
// and input echo, as well as carriage-return to newline translation.
//
// It returns the original terminal state, which should be restored before
// the program exits. If an error occurs, nil and the error are returned.
//
// Example usage:
//
//	fd := int(os.Stdin.Fd())
//	oldState, err := enableRawMode(fd)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer unix.IoctlSetTermios(fd, unix.TCSETS, oldState)
func enableRawMode(fd int) (*unix.Termios, error) {
	oldState, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}

	newState := *oldState
	newState.Lflag &^= unix.ECHO | unix.ICANON                             // disable echo and canonical mode
	newState.Lflag &^= unix.ISIG | unix.IEXTEN                             // disable Ctrl-C and Ctrl-Z (SIGINT - SIGSTP) and Ctrl-V
	newState.Iflag &^= unix.IXON                                           // disable Ctrl-S and Ctrl-Q (XOFF - XON Stop and Start output)
	newState.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP // ICRNL disbles carriage return to newline
	newState.Oflag &^= unix.OPOST                                          // disable output processing. Use \r\n for newline when printing

	//	newState.Cc[unix.VMIN] = 0  // Control Character field. VMIN sets the minimum number of bytes before stdin returns
	//	newState.Cc[unix.VTIME] = 1 // VTIME is the time in milliseconds for which stdin returns

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return nil, err
	}

	return oldState, nil
}

// disableRawMode resets the terminal to the previous previous state
//
// It returns an error if it fails
func disableRawMode(fd int, prevState *unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TCSETS, prevState)
}

// processKeypress() has a main loop that processes a key and prints it if possible
//
// callback is the functions that provides the key to be processed
//
// It returns if the key pressed is Ctrl-q
func processKeypress(callback func() (key byte)) {
	const CtrlQ byte = 0x11 // Ctrl-q

	for {
		key := callback()
		if unicode.IsControl(rune(key)) {
			switch key {
			case CtrlQ:
				return
			default:
				fmt.Printf("Control character %U cannot be printed\r\n", key)
			}
		}
		fmt.Printf("You typed: %#U \r\n", key)
	}
}

// Constants used as arguments for the clearScreen function
const (
	line        rune = '0'
	belowCursor rune = '1'
	screen      rune = '2'
)

// clearScreen() clears the screen
func clearScreen(element rune) {
	// \x1b is 27 in hex. When followed by [ it means it follows an escape sequence.
	// In this case J clears an element in the screen,
	// the argument 2 clears all the screen, 1 below the cursor and 0 the line
	clearScreenCmd := fmt.Sprintf("\x1b[%cJ", element)
	fmt.Print(clearScreenCmd)
}

// moveCursorTopLeft() repositions the cursor on the top left corner
func moveCursorTopLeft() {
	const moveCursorTop = "\x1b[H"
	fmt.Print(moveCursorTop)
}

// getWindowSize() returns rows and columns of the current window
func getWindowSize(fd int) (rows, cols uint16) {
	winSize, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		panic(err)
	}
	return winSize.Row, winSize.Col
}

// drawTildes() draws a ~ for each row above the end of the file
func drawTildes(fd int) {
	rows, _ := getWindowSize(fd)
	for row := uint16(1); row < rows-1; row++ {
		fmt.Print("~\r\n")
	}
}

func main() {
	fd := int(os.Stdin.Fd())

	oldState, err := enableRawMode(fd)
	if err != nil {
		panic(err)
	}
	defer disableRawMode(fd, oldState)
	defer clearScreen(screen)
	defer moveCursorTopLeft()

	onKeypress := func() (key byte) {
		reader := bufio.NewReader(os.Stdin)
		key, _ = reader.ReadByte()
		return key
	}

	clearScreen(screen)
	drawTildes(fd)
	moveCursorTopLeft()
	fmt.Printf("Write something. press \"Ctrl-q\" to exit the program\r\n")
	processKeypress(onKeypress)
}
