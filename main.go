package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/eiannone/keyboard"
)

type Editor struct {
	content  [][]rune
	cursorX  int
	cursorY  int
	filename string
	modified bool
}

func NewEditor(filename string) (*Editor, error) {
	editor := &Editor{
		content:  make([][]rune, 1),
		filename: filename,
	}
	editor.content[0] = make([]rune, 0)

	// Try to read existing file
	if _, err := os.Stat(filename); err == nil {
		content, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		lines := strings.Split(string(content), "\n")
		editor.content = make([][]rune, len(lines))
		for i, line := range lines {
			editor.content[i] = []rune(line)
		}
	}

	return editor, nil
}

func (e *Editor) insertRune(r rune) {
	line := e.content[e.cursorY]
	if e.cursorX == len(line) {
		e.content[e.cursorY] = append(line, r)
	} else {
		newLine := make([]rune, len(line)+1)
		copy(newLine, line[:e.cursorX])
		newLine[e.cursorX] = r
		copy(newLine[e.cursorX+1:], line[e.cursorX:])
		e.content[e.cursorY] = newLine
	}
	e.cursorX++
	e.modified = true
}

func (e *Editor) insertNewline() {
	currentLine := e.content[e.cursorY]
	rightPart := make([]rune, len(currentLine[e.cursorX:]))
	copy(rightPart, currentLine[e.cursorX:])
	e.content[e.cursorY] = currentLine[:e.cursorX]

	// Insert new line
	e.content = append(e.content[:e.cursorY+1], e.content[e.cursorY:]...)
	e.content[e.cursorY+1] = rightPart

	e.cursorY++
	e.cursorX = 0
	e.modified = true
}

func (e *Editor) backspace() {
	if e.cursorX > 0 {
		line := e.content[e.cursorY]
		e.content[e.cursorY] = append(line[:e.cursorX-1], line[e.cursorX:]...)
		e.cursorX--
		e.modified = true
	} else if e.cursorY > 0 {
		// Merge with previous line
		prevLine := e.content[e.cursorY-1]
		currentLine := e.content[e.cursorY]
		e.cursorX = len(prevLine)
		e.content[e.cursorY-1] = append(prevLine, currentLine...)
		e.content = append(e.content[:e.cursorY], e.content[e.cursorY+1:]...)
		e.cursorY--
		e.modified = true
	}
}

func (e *Editor) save() error {
	var content strings.Builder
	for i, line := range e.content {
		content.WriteString(string(line))
		if i < len(e.content)-1 {
			content.WriteRune('\n')
		}
	}
	err := os.WriteFile(e.filename, []byte(content.String()), 0644)
	if err == nil {
		e.modified = false
	}
	return err
}

func (e *Editor) render() {
	// Get terminal size
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, _ := cmd.Output()
	rows, _ := strconv.Atoi(strings.Split(string(out), " ")[0])
	visibleLines := rows - 1 // Leave one line for status

	// Calculate viewport
	startLine := 0
	if len(e.content) > visibleLines {
		startLine = e.cursorY - visibleLines/2
		if startLine < 0 {
			startLine = 0
		}
		if startLine > len(e.content)-visibleLines {
			startLine = len(e.content) - visibleLines
		}
	}

	// Clear screen
	fmt.Print("\033[2J")
	fmt.Print("\033[H")

	// Render visible content
	for i := 0; i < visibleLines && i+startLine < len(e.content); i++ {
		lineNum := i + startLine
		line := e.content[lineNum]
		fmt.Printf("\033[90m%3d \033[0m", lineNum+1)
		fmt.Println(string(line))
	}

	// Move cursor to position (accounting for line number width)
	fmt.Printf("\033[%d;%dH", e.cursorY-startLine+1, e.cursorX+5)
}

func cleanup() {
	// Reset terminal to normal mode
	fmt.Print("\033[?25h") // Show cursor
	fmt.Print("\033[0m")   // Reset all attributes
	fmt.Print("\033[H")    // Move to home position
	fmt.Print("\033[2J")   // Clear screen
	keyboard.Close()
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: dm <filename>")
		os.Exit(1)
	}

	editor, err := NewEditor(os.Args[1])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Initialize keyboard
	if err := keyboard.Open(); err != nil {
		fmt.Printf("Error opening keyboard: %v\n", err)
		os.Exit(1)
	}

	// Set up proper terminal cleanup
	defer cleanup()

	for {
		editor.render()

		char, key, err := keyboard.GetKey()
		if err != nil {
			fmt.Printf("Error reading keyboard: %v\n", err)
			os.Exit(1)
		}

		switch key {
		case keyboard.KeyCtrlQ:
			if !editor.modified {
				return
			}
			// Ask to save if modified
			fmt.Print("\033[2J")
			fmt.Print("\033[H")
			fmt.Print("File has unsaved changes. Save before quitting? (y/n): ")
			var response string
			for {
				char, key, _ := keyboard.GetKey()
				if key == keyboard.KeyEnter {
					if response == "y" {
						if err := editor.save(); err != nil {
							fmt.Printf("Error saving: %v\n", err)
							continue
						}
						return
					} else if response == "n" {
						return
					}
					// Invalid input, ask again
					fmt.Print("\nPlease enter 'y' or 'n': ")
					response = ""
				} else if key == keyboard.KeyBackspace || key == keyboard.KeyBackspace2 {
					if len(response) > 0 {
						response = response[:len(response)-1]
						fmt.Print("\b \b") // Move back, clear character, move back again
					}
				} else if char != 0 {
					response += string(char)
					fmt.Print(string(char))
				}
			}
		case keyboard.KeyCtrlS:
			if err := editor.save(); err != nil {
				fmt.Printf("Error saving: %v\n", err)
			}
		case keyboard.KeyArrowLeft:
			if editor.cursorX > 0 {
				editor.cursorX--
			}
		case keyboard.KeyArrowRight:
			if editor.cursorX < len(editor.content[editor.cursorY]) {
				editor.cursorX++
			}
		case keyboard.KeyArrowUp:
			if editor.cursorY > 0 {
				editor.cursorY--
				if editor.cursorX > len(editor.content[editor.cursorY]) {
					editor.cursorX = len(editor.content[editor.cursorY])
				}
			}
		case keyboard.KeyArrowDown:
			if editor.cursorY < len(editor.content)-1 {
				editor.cursorY++
				if editor.cursorX > len(editor.content[editor.cursorY]) {
					editor.cursorX = len(editor.content[editor.cursorY])
				}
			}
		case keyboard.KeyEnter:
			editor.insertNewline()
		case keyboard.KeyBackspace, keyboard.KeyBackspace2:
			editor.backspace()
		case keyboard.KeySpace:
			editor.insertRune(' ')
		default:
			if char != 0 {
				editor.insertRune(char)
			}
		}
	}
}
