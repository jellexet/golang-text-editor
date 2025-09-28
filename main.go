package main

import (
	"bufio"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
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
	newState.Lflag &^= unix.ECHO | unix.ICANON // disable echo and canonical mode
	newState.Iflag &^= unix.ICRNL              // disable Carriage Return to Newline translation

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return nil, err
	}

	return oldState, nil
}

func main() {
	fd := int(os.Stdin.Fd())

	oldState, err := enableRawMode(fd)
	if err != nil {
		panic(err)
	}
	defer unix.IoctlSetTermios(fd, unix.TCSETS, oldState) // restore on exit

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Write something...")

	for {
		char, _ := reader.ReadByte()
		if char == 'q' {
			break
		}
		fmt.Printf("You typed: %#U \n", char)
	}
}
