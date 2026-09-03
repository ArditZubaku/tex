package main

import (
	"slices"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

// A prompt is the status line handed over for a line of input: VIM's '/' and
// '?' searches and its ':' commands, which differ only in what pressing Enter
// does with what was typed.
var (
	promptChar  rune
	promptInput []rune
	statusMsg   string
)

func startPrompt(delimiter rune) {
	mode = PromptMode
	promptChar, promptInput = delimiter, promptInput[:0]
}

func handlePromptKey(event termbox.Event) {
	switch event.Key {
	case termbox.KeyEsc:
		endPrompt()
	case termbox.KeyEnter:
		submitPrompt()
	case termbox.KeyBackspace, termbox.KeyBackspace2:
		// backspacing off the delimiter leaves the prompt, as it does in VIM
		if len(promptInput) == 0 {
			endPrompt()
			return
		}
		promptInput = promptInput[:len(promptInput)-1]
	case termbox.KeyCtrlU:
		promptInput = promptInput[:0]
	case termbox.KeySpace:
		promptInput = append(promptInput, ' ')
	default:
		if event.Ch != 0 {
			promptInput = append(promptInput, event.Ch)
		}
	}

	// the explorer's own '/' narrows its listing as the pattern is typed, so
	// what is on screen is always what Enter would settle on
	if explorerOpen && mode == PromptMode {
		filterExplorer(string(promptInput))
	}
}

func submitPrompt() {
	input, delimiter := slices.Clone(promptInput), promptChar
	endPrompt()

	if explorerOpen {
		filterExplorer(string(input))
		return
	}

	if delimiter == ':' {
		runExCommand(string(input))
		return
	}
	commitSearch(input, delimiter == '?')
}

// endPrompt is what Esc reaches, so it leaves the explorer's listing as it was
// before the '/' was pressed; submitPrompt puts the pattern back afterwards.
func endPrompt() {
	mode = ReadMode
	if explorerOpen {
		mode = ExplorerMode
		filterExplorer("")
	}
	promptInput = promptInput[:0]
}

// promptStatus takes the status bar over while a line is being typed, and for
// the one redraw after a command reported something.
func promptStatus() (string, bool) {
	if mode == PromptMode {
		return string(promptChar) + string(promptInput), true
	}

	return statusMsg, statusMsg != ""
}

func promptCol() int {
	return runewidth.StringWidth(string(promptInput)) + 1
}
