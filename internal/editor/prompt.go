package editor

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

// A prompt is the status line handed over for a line of input: VIM's '/' and
// '?' searches and its ':' commands, which differ only in what pressing Enter
// does with what was typed.
var (
	promptChar  rune
	promptInput []rune
)

func startPrompt(delimiter rune) {
	ed.Mode = state.PromptMode
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
	if ed.ExplorerOpen && ed.Mode == state.PromptMode {
		filterExplorer(string(promptInput))
	}
}

func submitPrompt() {
	input, delimiter := slices.Clone(promptInput), promptChar
	endPrompt()

	if ed.ExplorerOpen {
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
	ed.Mode = state.ReadMode
	if ed.ExplorerOpen {
		ed.Mode = state.ExplorerMode
		filterExplorer("")
	}
	promptInput = promptInput[:0]
}

// promptStatus takes the status bar over while a line is being typed, and for
// the one redraw after a command reported something.
func promptStatus() (string, bool) {
	if ed.Mode == state.PromptMode {
		return string(promptChar) + string(promptInput), true
	}

	return ed.StatusMsg, ed.StatusMsg != ""
}

func promptCol() int {
	return runewidth.StringWidth(string(promptInput)) + 1
}
