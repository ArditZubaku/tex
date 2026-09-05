package editor

import (
	"time"

	"github.com/ArditZubaku/tex/internal/editor/screen"
	"github.com/ArditZubaku/tex/internal/editor/state"
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
		handleExplorerKey(keyEvent)
	case ed.Mode == state.PickerMode:
		handlePickerKey(keyEvent)
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
		insertRune(keyEvent)
	case state.ReadMode, state.VisualMode:
		handleReadModeChar(keyEvent)
		ed.ClampCol()
	}
}

// readModeActions dispatches the single-key vim motions; keys that start a
// chord are held pending instead, see chordActions.
var readModeActions = map[rune]func(){
	'G': ed.GoToBottom,
	'H': prevBuffer,
	'L': nextBuffer,
	'I': ed.GoToStartOfLine,
	'A': ed.GoToEndOfLine,
	'a': ed.EditAfterWord,
	'h': ed.Left,
	'j': ed.Down,
	'k': ed.Up,
	'l': ed.Right,
	'w': ed.NextWord,
	'b': ed.PrevWord,
	'e': ed.EndOfWord,
	'q': closeEditor,
	'i': ed.EditBeforeWord,
	'x': deleteRune,
	'o': openLineBelow,
	'O': openLineAbove,
	'p': pasteAfter,
	'P': pasteBefore,
	'u': ed.Undo,
	'/': startSearchForward,
	'?': startSearchBackward,
	'n': nextMatch,
	'N': prevMatch,
	':': startExPrompt,
	'v': startVisualChar,
	'V': startVisualLine,
}

// Visual mode reuses Read mode's motions — a motion there drags the far end of
// the selection along with it — and puts operators that act on the selection
// where Read mode's own are. A key that means nothing while selecting is left
// out rather than doing what it does in Read mode.
var visualActions = visualKeys()

func visualKeys() map[rune]func() {
	actions := map[rune]func(){
		'v': toggleVisualChar,
		'V': toggleVisualLine,
		'o': swapVisualEnds,
		'd': deleteSelection,
		'x': deleteSelection,
		'c': changeSelection,
		'y': yankSelection,
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

func chordKeys() map[string]func() {
	chords := map[string]func(){
		"gg":  ed.GoToTop,
		"gd":  goToDefinition,
		"gr":  openReferences,
		"dd":  deleteLine,
		"dw":  deleteWord,
		"de":  deleteToWordEnd,
		"db":  deleteToPrevWord,
		"yy":  yankLine,
		"yw":  yankWord,
		"ye":  yankToWordEnd,
		"yb":  yankToPrevWord,
		"zz":  ed.CenterView,
		" e":  openExplorer,
		" bb": alternateBuffer,
		" bd": closeCurrentBuffer,
		" bn": nextBuffer,
		" bp": prevBuffer,
		" bo": closeOtherBuffers,
		" bl": closeBuffersLeft,
		" br": closeBuffersRight,
		" sh": splitRight,
		" sv": splitBelow,
		" ss": openSymbols,
		" sS": openWorkspaceSymbols,
		" wd": closeWindow,
		"  ":  openPicker,
	}

	return chords
}

var visualChords = map[string]func(){
	"gg": ed.GoToTop,
	"zz": ed.CenterView,
}

// A chord's own prefixes do nothing on their own; they wait for the keys that
// finish it. Deriving them from the table is what lets a chord be as long as it
// likes without a second list to keep in step.
var (
	chordPrefixes       = prefixesOf(chordActions)
	visualChordPrefixes = prefixesOf(visualChords)
)

func prefixesOf(chords map[string]func()) map[string]bool {
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

func runCommand(action func(), countAware bool) {
	ed.CmdCount, ed.HadCount, ed.PendingCount = max(ed.PendingCount, 1), ed.PendingCount > 0, 0
	defer func() { ed.CmdCount, ed.HadCount = 1, false }()

	ed.BeginChange()
	if countAware {
		action()
	} else {
		for range ed.CmdCount {
			action()
		}
	}
	ed.EndChange()
}

// Ctrl-hjkl move between windows, which is all it takes to leave one. Only
// outside Edit mode: Ctrl-H is also the Backspace that terminals sending 0x08
// rather than 0x7F give, and typing has first call on it.
var windowMoveKeys = map[termbox.Key]func(){
	termbox.KeyCtrlH: focusLeft,
	termbox.KeyCtrlJ: focusDown,
	termbox.KeyCtrlK: focusUp,
	termbox.KeyCtrlL: focusRight,
}

var specialKeyActions = map[termbox.Key]func(){
	termbox.KeyCtrlS:      saveFile,
	termbox.KeyEnter:      enter,
	termbox.KeyBackspace:  backspace,
	termbox.KeyBackspace2: backspace,
	termbox.KeyArrowUp:    ed.Up,
	termbox.KeyCtrlU:      ed.PageUp,
	termbox.KeyArrowDown:  ed.Down,
	termbox.KeyCtrlD:      ed.PageDown, // this is a vim motion actually, but it far easier to handle it like this
	termbox.KeyArrowLeft:  ed.Left,
	termbox.KeyArrowRight: ed.Right,
	termbox.KeyPgup:       ed.PageUp,
	termbox.KeyPgdn:       ed.PageDown,
	termbox.KeyCtrlR:      ed.Redo,
	termbox.KeyCtrlO:      jumpBack,
}

func handleSpecialKey(keyEvent termbox.Event) {
	// Space is the leader outside Edit mode, so it has to reach the chord table
	// rather than cancel what is pending there like every other special key.
	if keyEvent.Key == termbox.KeySpace && ed.Mode != state.EditMode {
		handleReadModeChar(termbox.Event{Ch: ' '})
		return
	}

	ed.PendingKeys, ed.PendingCount = ed.PendingKeys[:0], 0 // any special key cancels a pending chord or count

	if action, ok := windowMoveKeys[keyEvent.Key]; ok && ed.Mode != state.EditMode {
		action()
		return
	}

	switch keyEvent.Key {
	case termbox.KeyTab:
		if ed.Mode == state.EditMode {
			insertRuneNTimes(keyEvent, 4)
			break
		}
		nextBuffer()
	case termbox.KeySpace:
		insertRuneNTimes(keyEvent, 1)
	case termbox.KeyHome:
		ed.Col = 0
	case termbox.KeyEnd:
		ed.Col = ed.MaxCol(ed.Row)
	default:
		if action, ok := specialKeyActions[keyEvent.Key]; ok {
			action()
		}
	}

	ed.ClampCol()
}

func insertRuneNTimes(keyEvent termbox.Event, n int) {
	if ed.Mode != state.EditMode {
		return
	}
	for range n {
		insertRune(keyEvent)
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

func startExPrompt() { startPrompt(':') }

// closeEditor lets the editor's loop fall out and shut the terminal down on its
// way, so quitting runs the same path whether it was 'q' or ':q' that asked.
func closeEditor() {
	ed.Quitting = true
}
