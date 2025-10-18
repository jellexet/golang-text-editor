package main

import (
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

	defer editor.DisableRawMode(fd, oldState)
	defer editor.ClearScreen(editor.Screen)
	defer editor.MoveCursorTopLeft()

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

	onKeypress := func() (key byte) {
		var b [1]byte
		n, err := unix.Read(fd, b[:])
		if n == 0 || err != nil {
			// On timeout (n=0) or error, return a 0 byte
			// editorReadKey is built to handle this.
			return 0x00
		}
		// Return the single byte read
		return b[0]
	}

	// Initial screen setup
	editor.ClearScreen(editor.Screen)
	editor.DrawTildes(fd)
	editor.MoveCursorTopLeft()

	// Start the main editor loop
	editor.ProcessKeypress(fd, onKeypress)
}
