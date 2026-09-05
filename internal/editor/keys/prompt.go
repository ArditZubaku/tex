package keys

import (
	"slices"

	"github.com/ArditZubaku/tex/internal/editor/command"
	"github.com/ArditZubaku/tex/internal/editor/explorer"
	"github.com/ArditZubaku/tex/internal/editor/find"
	"github.com/ArditZubaku/tex/internal/editor/prompt"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/nsf/termbox-go"
)

func startSearchForward(e *state.Editor)  { e.StartPrompt('/') }
func startSearchBackward(e *state.Editor) { e.StartPrompt('?') }

func handlePromptKey(e *state.Editor, event termbox.Event) {
	switch e.Prompt.Key(event) {
	case prompt.Closed:
		endPrompt(e)
		return
	case prompt.Submitted:
		submitPrompt(e)
		return
	}

	// the explorer's own '/' narrows its listing as the pattern is typed, so
	// what is on screen is always what Enter would settle on
	if e.ExplorerOpen {
		explorer.Filter(e, string(e.Prompt.Input()))
	}
}

func submitPrompt(e *state.Editor) {
	input, delimiter := slices.Clone(e.Prompt.Input()), e.Prompt.Delimiter()
	endPrompt(e)

	if e.ExplorerOpen {
		explorer.Filter(e, string(input))
		return
	}

	if delimiter == ':' {
		command.Run(e, string(input))
		return
	}
	find.CommitSearch(e, input, delimiter == '?')
}

// endPrompt is what Esc reaches, so it leaves the explorer's listing as it was
// before the '/' was pressed; submitPrompt puts the pattern back afterwards.
func endPrompt(e *state.Editor) {
	e.Mode = state.ReadMode
	if e.ExplorerOpen {
		e.Mode = state.ExplorerMode
		explorer.Filter(e, "")
	}
	e.Prompt.Clear()
}
