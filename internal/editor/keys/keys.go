// Package keys is the dispatcher: it says which command each key names, in the
// mode it arrives in, and holds a chord or a count until the keys that finish
// it come.
package keys

import (
	"time"

	"github.com/nsf/termbox-go"

	"github.com/ArditZubaku/tex/internal/editor/command"
	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/explorer"
	"github.com/ArditZubaku/tex/internal/editor/find"
	"github.com/ArditZubaku/tex/internal/editor/rename"
	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
)

func Read(e *state.Editor) {
	keyEvent, isKey := screen.Key()

	Handle(e, keyEvent, isKey)
}

// Handle is one event off the terminal. Anything that was not a key is a frame
// owed and nothing else: a chord half typed, a count, what the last command
// reported and the box a server's answer went in all outlive a resize, and
// outlive the interrupt that answer wakes the loop with.
func Handle(e *state.Editor, keyEvent termbox.Event, isKey bool) {
	if keyEvent.Type == termbox.EventMouse {
		e.Hov.Clear()
		view.Mouse(e, keyEvent)

		return
	}
	if !isKey {
		return
	}
	// whatever the last command reported has had its redraw
	e.StatusMsg = ""
	e.Hov.Clear()

	Dispatch(e, keyEvent)
}

func Dispatch(e *state.Editor, keyEvent termbox.Event) {
	sawCR := e.SawCR
	e.SawCR = keyEvent.Ch == 0 && keyEvent.Key == termbox.KeyEnter
	if sawCR && keyEvent.Ch == 0 && keyEvent.Key == termbox.KeyCtrlJ {
		return
	}

	switch {
	case e.Mode == state.PromptMode:
		handlePromptKey(e, keyEvent)
	case e.Mode == state.ExplorerMode:
		explorer.Key(e, keyEvent)
	case e.Mode == state.PickerMode:
		find.PickerKey(e, keyEvent)
	case keyEvent.Key == termbox.KeyEsc:
		esc(e)
	case keyEvent.Ch != 0:
		handleCharKey(e, keyEvent)
	default:
		handleSpecialKey(e, keyEvent)
	}
}

func handleCharKey(e *state.Editor, keyEvent termbox.Event) {
	switch e.Mode {
	case state.EditMode:
		edit.InsertRune(e, keyEvent)
	case state.ReadMode, state.VisualMode:
		handleReadModeChar(e, keyEvent)
		e.ClampCol()
	}
}

// readModeActions dispatches the single-key vim motions; keys that start a
// chord are held pending instead, see chordActions.
var readModeActions = map[rune]func(*state.Editor){
	'G': (*state.Editor).GoToBottom,
	'H': view.PrevBuffer,
	'L': view.NextBuffer,
	'I': (*state.Editor).GoToStartOfLine,
	'A': (*state.Editor).GoToEndOfLine,
	'a': (*state.Editor).EditAfterWord,
	'h': (*state.Editor).Left,
	'j': (*state.Editor).Down,
	'k': (*state.Editor).Up,
	'l': (*state.Editor).Right,
	'w': (*state.Editor).NextWord,
	'b': (*state.Editor).PrevWord,
	'e': (*state.Editor).EndOfWord,
	'$': (*state.Editor).EndOfLine,
	'q': command.Quit,
	'i': (*state.Editor).EditBeforeWord,
	'x': edit.DeleteRune,
	'o': edit.OpenLineBelow,
	'O': edit.OpenLineAbove,
	'p': edit.PasteAfter,
	'P': edit.PasteBefore,
	'u': (*state.Editor).Undo,
	'/': startSearchForward,
	'?': startSearchBackward,
	'n': find.NextMatch,
	'N': find.PrevMatch,
	':': startExPrompt,
	'v': edit.StartVisualChar,
	'V': edit.StartVisualLine,
	'K': find.ShowHover,
}

// Visual mode reuses Read mode's motions — a motion there drags the far end of
// the selection along with it — and puts operators that act on the selection
// where Read mode's own are. A key that means nothing while selecting is left
// out rather than doing what it does in Read mode.
var visualActions = visualKeys()

func visualKeys() map[rune]func(*state.Editor) {
	actions := map[rune]func(*state.Editor){
		'v': edit.ToggleVisualChar,
		'V': edit.ToggleVisualLine,
		'o': edit.SwapVisualEnds,
		'd': edit.DeleteSelection,
		'x': edit.DeleteSelection,
		'c': edit.ChangeSelection,
		'y': edit.YankSelection,
		'p': edit.PasteSelection,
	}
	for _, ch := range "hjklwbeG$" {
		actions[ch] = readModeActions[ch]
	}

	return actions
}

// A chord is the keys it takes to name one command, in the order they are
// typed: VIM's own two-key operators, and the leader sequences LazyVim puts its
// buffer commands under.
var chordActions = chordKeys()

func chordKeys() map[string]func(*state.Editor) {
	chords := map[string]func(*state.Editor){
		"gg":  (*state.Editor).GoToTop,
		"gd":  find.GoToDefinition,
		"gr":  find.OpenReferences,
		"dd":  edit.DeleteLine,
		"dw":  edit.DeleteWord,
		"de":  edit.DeleteToWordEnd,
		"db":  edit.DeleteToPrevWord,
		"yy":  edit.YankLine,
		"yw":  edit.YankWord,
		"ye":  edit.YankToWordEnd,
		"yb":  edit.YankToPrevWord,
		"zz":  (*state.Editor).CenterView,
		"]d":  find.NextDiagnostic,
		"[d":  find.PrevDiagnostic,
		" e":  explorer.Open,
		" bb": view.AlternateBuffer,
		" bd": view.CloseCurrentBuffer,
		" bn": view.NextBuffer,
		" bp": view.PrevBuffer,
		" bo": view.CloseOtherBuffers,
		" bl": view.CloseBuffersLeft,
		" br": view.CloseBuffersRight,
		" cr": rename.Start,
		" sh": view.SplitRight,
		" sv": view.SplitBelow,
		" ss": find.OpenSymbols,
		" sS": find.OpenWorkspaceSymbols,
		" wd": view.CloseWindow,
		"  ":  find.OpenFiles,
		" /":  find.OpenGrep,
	}

	return chords
}

var visualChords = map[string]func(*state.Editor){
	"gg": (*state.Editor).GoToTop,
	"zz": (*state.Editor).CenterView,
}

// A chord's own prefixes do nothing on their own; they wait for the keys that
// finish it. Deriving them from the table is what lets a chord be as long as it
// likes without a second list to keep in step.
var (
	chordPrefixes       = prefixesOf(chordActions)
	visualChordPrefixes = prefixesOf(visualChords)
)

func prefixesOf(chords map[string]func(*state.Editor)) map[string]bool {
	prefixes := make(map[string]bool)
	for chord := range chords {
		keys := []rune(chord)
		for i := 1; i < len(keys); i++ {
			prefixes[string(keys[:i])] = true
		}
	}

	return prefixes
}

// A count-aware command reads CmdCount itself, because the count says how much
// text it works on rather than how many times it runs; everything else is
// simply run that many times.
var (
	countAwareKeys   = map[rune]bool{'x': true, 'p': true, 'P': true}
	countAwareChords = map[string]bool{"dd": true, "yy": true, "zz": true}
)

func handleReadModeChar(e *state.Editor, keyEvent termbox.Event) {
	ch := keyEvent.Ch

	// a leading '0' is VIM's jump to column 0, not the start of a count
	if (ch >= '1' && ch <= '9') || (ch == '0' && e.PendingCount > 0) {
		e.PendingCount = min(e.PendingCount*10+int(ch-'0'), state.MaxCount)
		return
	}

	keys, chords, prefixes := readModeActions, chordActions, chordPrefixes
	if e.Mode == state.VisualMode {
		keys, chords, prefixes = visualActions, visualChords, visualChordPrefixes
	}

	if time.Since(e.PendingTime) >= state.ChordTimeout {
		e.PendingKeys = e.PendingKeys[:0]
	}

	if len(e.PendingKeys) > 0 {
		chord := string(e.PendingKeys) + string(ch)
		if action, ok := chords[chord]; ok {
			e.PendingKeys = e.PendingKeys[:0]
			runCommand(e, action, countAwareChords[chord])
			return
		}
		if prefixes[chord] {
			e.PendingKeys, e.PendingTime = append(e.PendingKeys, ch), time.Now()
			return
		}
	}
	e.PendingKeys = e.PendingKeys[:0]

	// the direct keys come first, so that Visual mode's 'd' and 'y' are
	// operators in their own right rather than the halves of a chord they are
	// in Read mode
	if action, ok := keys[ch]; ok {
		runCommand(e, action, countAwareKeys[ch])
		return
	}

	if prefixes[string(ch)] {
		e.PendingKeys, e.PendingTime = append(e.PendingKeys, ch), time.Now()
	}
}

func runCommand(e *state.Editor, action func(*state.Editor), countAware bool) {
	e.CmdCount, e.HadCount, e.PendingCount = max(e.PendingCount, 1), e.PendingCount > 0, 0

	e.BeginChange()
	if countAware {
		action(e)
	} else {
		for range e.CmdCount {
			action(e)
		}
	}
	e.EndChange()

	e.CmdCount, e.HadCount = 1, false
}

var specialKeyActions = map[termbox.Key]func(*state.Editor){
	termbox.KeyCtrlS:      command.Save,
	termbox.KeyEnter:      edit.Enter,
	termbox.KeyCtrlJ:      edit.Enter, // a pasted Unix line ending, absent CRLF's CR
	termbox.KeyBackspace:  edit.Backspace,
	termbox.KeyBackspace2: edit.Backspace,
	termbox.KeyArrowUp:    (*state.Editor).Up,
	termbox.KeyCtrlU:      (*state.Editor).PageUp,
	termbox.KeyArrowDown:  (*state.Editor).Down,
	termbox.KeyCtrlD:      (*state.Editor).PageDown, // this is a vim motion actually, but it far easier to handle it like this
	termbox.KeyArrowLeft:  (*state.Editor).Left,
	termbox.KeyArrowRight: (*state.Editor).Right,
	termbox.KeyPgup:       (*state.Editor).PageUp,
	termbox.KeyPgdn:       (*state.Editor).PageDown,
	termbox.KeyCtrlR:      (*state.Editor).Redo,
	termbox.KeyCtrlO:      find.JumpBack,
}

func handleSpecialKey(e *state.Editor, keyEvent termbox.Event) {
	// Space is the leader outside Edit mode, so it has to reach the chord table
	// rather than cancel what is pending there like every other special key.
	if keyEvent.Key == termbox.KeySpace && e.Mode != state.EditMode {
		handleReadModeChar(e, termbox.Event{Ch: ' '})
		return
	}

	e.PendingKeys, e.PendingCount = e.PendingKeys[:0], 0 // any special key cancels a pending chord or count

	if action, ok := view.MoveKeys[keyEvent.Key]; ok && e.Mode != state.EditMode {
		action(e)
		return
	}

	switch keyEvent.Key {
	case termbox.KeyTab:
		if e.Mode == state.EditMode {
			insertRuneNTimes(e, keyEvent, 4)
			break
		}
		view.NextBuffer(e)
	case termbox.KeySpace:
		insertRuneNTimes(e, keyEvent, 1)
	case termbox.KeyHome:
		e.Col = 0
	case termbox.KeyEnd:
		e.Col = e.MaxCol(e.Row)
	case termbox.KeyCtrlV:
		if e.Mode == state.VisualMode {
			runCommand(e, edit.PasteSelection, true)
		} else {
			runCommand(e, edit.PasteAfter, true)
		}
	default:
		if action, ok := specialKeyActions[keyEvent.Key]; ok {
			action(e)
		}
	}

	e.ClampCol()
}

func insertRuneNTimes(e *state.Editor, keyEvent termbox.Event, n int) {
	if e.Mode != state.EditMode {
		return
	}
	for range n {
		edit.InsertRune(e, keyEvent)
	}
}

func esc(e *state.Editor) {
	// leaving Edit mode steps back off the gap the cursor was typing into
	if e.Mode == state.EditMode && e.Col > 0 {
		e.Col--
	}
	e.Mode = state.ReadMode
	e.PendingKeys, e.PendingCount, e.HlSearch = e.PendingKeys[:0], 0, false
	e.Note.Clear()
	e.EndChange()
	e.ClampCol()
	state.SetCursorShape(state.CursorDefault)
}

func startExPrompt(e *state.Editor) { e.StartPrompt(':') }
