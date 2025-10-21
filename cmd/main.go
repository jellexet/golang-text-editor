package main

import (
	"fmt"
	"github.com/jellexet/golang-text-editor/pkg/editor"
	"golang.org/x/sys/unix"
	"log"
	"os"
)

func main() {
	fd := int(os.Stdin.Fd())

	// Check if stdin is a terminal
	if _, err := unix.IoctlGetTermios(fd, unix.TCGETS); err != nil {
		log.Fatalln("Not a TTY. This editor requires a TTY to run.")
	}

	// Enable raw mode for terminal
	oldState, err := editor.EnableRawMode(fd)
	if err != nil {
		panic(err)
	}
	// Enters an alternate screen buffer
	fmt.Print("\x1b[?1049h")

	// Printing this exits the alternate screen buffer
	defer fmt.Print("\x1b[?1049h")
	defer editor.DisableRawMode(fd, oldState)

	var initialContent string
	var filename string
	if len(os.Args) > 1 {
		filename = os.Args[1]
		contentBytes, err := os.ReadFile(filename)
		// If file doesn't exist or errors, we'll just start with an empty buffer
		if err == nil {
			initialContent = string(contentBytes)
		}
	} else {
		filename = "[No Name]"
	}
	editor.InitSession(fd, filename, initialContent)

	// function to be passed as argument to ProcessKeypress()
	// It defines what to do for each keypress
	onKeypress := func() (key byte) {
		var b [1]byte
		n, err := unix.Read(fd, b[:])
		if n == 0 || err != nil {
			// On timeout (n=0) or error, return 0x00
			// editorReadKey is built to handle this.
			return 0x00
		}
		return b[0]
	}

	// Start the main editor loop
	editor.ProcessKeypress(fd, onKeypress)
}
