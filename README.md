# golang-text-editor
A simple terminal-based text editor written in Golang as an excercise

I have taken inspiration from [here](https://viewsourcecode.org/snaptoken/kilo/index.html).
If something is hard to follow in the source code, the linked website explains
all the details.


## Features

  * **File Handling**: Open existing files or create new ones.
  * **Save**: Save your work to disk (`Ctrl-S`).
  * **Text Editing**: Basic insertion (typing) and deletion (Backspace).
  * **Navigation**: Cursor navigation using Arrow Keys (Up, Down, Left, Right).
  * **Undo/Redo**: Undo (`Ctrl-Z`) and Redo (`Ctrl-R`) your last actions.
* **Search**: Finds text in the buffer (`Ctrl-F`).

## Keybindings

| Key | Action |
| --- | --- |
| **Arrow Keys** | Move cursor |
| **Backspace** | Delete character before cursor |
| **Ctrl-S** | Save file (prompts for filename if new) |
| **Ctrl-F** | Search for text |
| **Ctrl-Z** | Undo last action |
| **Ctrl-R** | Redo last action |
| **Ctrl-Q** | Quit the editor |

## Build

```bash
go build -o go-editor main.go
```

## Usage

**To open an existing file:**

```bash
./go-editor my_file.txt
```

**To start a new, empty buffer:**

```bash
./go-editor
```
