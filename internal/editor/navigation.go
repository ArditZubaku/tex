package editor

import (
	"time"

	"github.com/ArditZubaku/tex/internal/editor/command"
	"github.com/ArditZubaku/tex/internal/editor/edit"
	"github.com/ArditZubaku/tex/internal/editor/explorer"
	"github.com/ArditZubaku/tex/internal/editor/find"
	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/nsf/termbox-go"
)

func processKeyPress() {
	keyEvent := screen.Key()
	ed.StatusMsg = "" // whatever the last command reported has had its redraw

	dispatchKey(keyEvent)
}

func dispatchKey(keyEvent termbox.Event) {
	switch {
	case ed.Mode == state.PromptMode:
		handlePromptKey(keyEvent)
	case ed.Mode == state.ExplorerMode:
		explorer.Key(ed, keyEvent)
	case ed.Mode == state.PickerMode:
		find.PickerKey(ed, keyEvent)
	case keyEvent.Key == termbox.KeyEsc:
		esc()
	case keyEvent.Ch != 0:
		handleCharKey(keyEvent)
	default:
		handleSpecialKey(keyEvent)
	}
}

func handleCharKey(keyEvent termbox.Event) {
	switch ed.Mode {
	case state.EditMode:
		edit.InsertRune(ed, keyEvent)
	case state.ReadMode, state.VisualMode:
		handleReadModeChar(keyEvent)
		ed.ClampCol()
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
	'q': (*state.Editor).Close,
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
	}
	for _, ch := range "hjklwbeG" {
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
		" e":  explorer.Open,
		" bb": view.AlternateBuffer,
		" bd": view.CloseCurrentBuffer,
		" bn": view.NextBuffer,
		" bp": view.PrevBuffer,
		" bo": view.CloseOtherBuffers,
		" bl": view.CloseBuffersLeft,
		" br": view.CloseBuffersRight,
		" sh": view.SplitRight,
		" sv": view.SplitBelow,
		" ss": find.OpenSymbols,
		" sS": find.OpenWorkspaceSymbols,
		" wd": view.CloseWindow,
		"  ":  find.OpenFiles,
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

// A count-aware command reads count() itself, because the count says how much
// text it works on rather than how many times it runs; everything else is
// simply run that many times.
var (
	countAwareKeys   = map[rune]bool{'x': true, 'p': true, 'P': true}
	countAwareChords = map[string]bool{"dd": true, "yy": true, "zz": true}
)

func handleReadModeChar(keyEvent termbox.Event) {
	ch := keyEvent.Ch

	// a leading '0' is VIM's jump to column 0, not the start of a count
	if (ch >= '1' && ch <= '9') || (ch == '0' && ed.PendingCount > 0) {
		ed.PendingCount = min(ed.PendingCount*10+int(ch-'0'), state.MaxCount)
		return
	}

	keys, chords, prefixes := readModeActions, chordActions, chordPrefixes
	if ed.Mode == state.VisualMode {
		keys, chords, prefixes = visualActions, visualChords, visualChordPrefixes
	}

	if time.Since(ed.PendingTime) >= state.ChordTimeout {
		ed.PendingKeys = ed.PendingKeys[:0]
	}

	if len(ed.PendingKeys) > 0 {
		chord := string(ed.PendingKeys) + string(ch)
		if action, ok := chords[chord]; ok {
			ed.PendingKeys = ed.PendingKeys[:0]
			runCommand(action, countAwareChords[chord])
			return
		}
		if prefixes[chord] {
			ed.PendingKeys, ed.PendingTime = append(ed.PendingKeys, ch), time.Now()
			return
		}
	}
	ed.PendingKeys = ed.PendingKeys[:0]

	// the direct keys come first, so that Visual mode's 'd' and 'y' are
	// operators in their own right rather than the halves of a chord they are
	// in Read mode
	if action, ok := keys[ch]; ok {
		runCommand(action, countAwareKeys[ch])
		return
	}

	if prefixes[string(ch)] {
		ed.PendingKeys, ed.PendingTime = append(ed.PendingKeys, ch), time.Now()
	}
}

func runCommand(action func(*state.Editor), countAware bool) {
	ed.CmdCount, ed.HadCount, ed.PendingCount = max(ed.PendingCount, 1), ed.PendingCount > 0, 0
	defer func() { ed.CmdCount, ed.HadCount = 1, false }()

	ed.BeginChange()
	if countAware {
		action(ed)
	} else {
		for range ed.CmdCount {
			action(ed)
		}
	}
	ed.EndChange()
}

var specialKeyActions = map[termbox.Key]func(*state.Editor){
	termbox.KeyCtrlS:      command.Save,
	termbox.KeyEnter:      edit.Enter,
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

func handleSpecialKey(keyEvent termbox.Event) {
	// Space is the leader outside Edit mode, so it has to reach the chord table
	// rather than cancel what is pending there like every other special key.
	if keyEvent.Key == termbox.KeySpace && ed.Mode != state.EditMode {
		handleReadModeChar(termbox.Event{Ch: ' '})
		return
	}

	ed.PendingKeys, ed.PendingCount = ed.PendingKeys[:0], 0 // any special key cancels a pending chord or count

	if action, ok := view.MoveKeys[keyEvent.Key]; ok && ed.Mode != state.EditMode {
		action(ed)
		return
	}

	switch keyEvent.Key {
	case termbox.KeyTab:
		if ed.Mode == state.EditMode {
			insertRuneNTimes(keyEvent, 4)
			break
		}
		view.NextBuffer(ed)
	case termbox.KeySpace:
		insertRuneNTimes(keyEvent, 1)
	case termbox.KeyHome:
		ed.Col = 0
	case termbox.KeyEnd:
		ed.Col = ed.MaxCol(ed.Row)
	default:
		if action, ok := specialKeyActions[keyEvent.Key]; ok {
			action(ed)
		}
	}

	ed.ClampCol()
}

func insertRuneNTimes(keyEvent termbox.Event, n int) {
	if ed.Mode != state.EditMode {
		return
	}
	for range n {
		edit.InsertRune(ed, keyEvent)
	}
}

func esc() {
	// leaving Edit mode steps back off the gap the cursor was typing into
	if ed.Mode == state.EditMode && ed.Col > 0 {
		ed.Col--
	}
	ed.Mode = state.ReadMode
	ed.PendingKeys, ed.PendingCount, ed.HlSearch = ed.PendingKeys[:0], 0, false
	ed.EndChange()
	ed.ClampCol()
	state.SetCursorShape(state.CursorDefault)
}

func startExPrompt(e *state.Editor) { e.StartPrompt(':') }
