package main

import (
	"bufio"
	"github.com/jellexet/golang-text-editor/pkg/editor"
	"os"
)

func main() {
	fd := int(os.Stdin.Fd())

	// Enable raw mode for terminal
	oldState, err := editor.EnableRawMode(fd)
	if err != nil {
		panic(err)
	}

	// Ensure cleanup on exit
	defer editor.DisableRawMode(fd, oldState)
	defer editor.ClearScreen(editor.Screen)
	defer editor.MoveCursorTopLeft()

	// Callback function to read keyboard input
	onKeypress := func() (key byte) {
		reader := bufio.NewReader(os.Stdin)
		key, _ = reader.ReadByte()
		return key
	}

	// Initial screen setup
	editor.ClearScreen(editor.Screen)
	editor.DrawTildes(fd)
	editor.MoveCursorTopLeft()

	// Start the main editor loop
	editor.ProcessKeypress(onKeypress)
}
