package editor

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/editor/command"
	"github.com/ArditZubaku/tex/internal/editor/find"
	"github.com/ArditZubaku/tex/internal/editor/prompt"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/nsf/termbox-go"
)

func startSearchForward()  { startPrompt('/') }
func startSearchBackward() { startPrompt('?') }

func startPrompt(delimiter rune) {
	ed.Mode = state.PromptMode
	ed.Prompt.Start(delimiter)
}

func handlePromptKey(event termbox.Event) {
	switch ed.Prompt.Key(event) {
	case prompt.Closed:
		endPrompt()
		return
	case prompt.Submitted:
		submitPrompt()
		return
	}

	// the explorer's own '/' narrows its listing as the pattern is typed, so
	// what is on screen is always what Enter would settle on
	if ed.ExplorerOpen {
		filterExplorer(string(ed.Prompt.Input()))
	}
}

func submitPrompt() {
	input, delimiter := slices.Clone(ed.Prompt.Input()), ed.Prompt.Delimiter()
	endPrompt()

	if ed.ExplorerOpen {
		filterExplorer(string(input))
		return
	}

	if delimiter == ':' {
		command.Run(ed, string(input))
		return
	}
	find.CommitSearch(ed, input, delimiter == '?')
}

// endPrompt is what Esc reaches, so it leaves the explorer's listing as it was
// before the '/' was pressed; submitPrompt puts the pattern back afterwards.
func endPrompt() {
	ed.Mode = state.ReadMode
	if ed.ExplorerOpen {
		ed.Mode = state.ExplorerMode
		filterExplorer("")
	}
	ed.Prompt.Clear()
}

// promptStatus takes the status bar over while a line is being typed, and for
// the one redraw after a command reported something.
func promptStatus() (string, bool) {
	if ed.Mode == state.PromptMode {
		return ed.Prompt.Text(), true
	}

	return ed.StatusMsg, ed.StatusMsg != ""
}

func promptCol() int {
	return ed.Prompt.Width()
}
