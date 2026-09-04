package main

import (
	"time"

	"github.com/nsf/termbox-go"
)

func processKeyPress() {
	keyEvent := getKey()
	statusMsg = "" // whatever the last command reported has had its redraw

	dispatchKey(keyEvent)
}

func dispatchKey(keyEvent termbox.Event) {
	switch {
	case mode == PromptMode:
		handlePromptKey(keyEvent)
	case mode == ExplorerMode:
		handleExplorerKey(keyEvent)
	case mode == PickerMode:
		handlePickerKey(keyEvent)
	case keyEvent.Key == termbox.KeyEsc:
		esc()
	case keyEvent.Ch != 0:
		handleCharKey(keyEvent)
	default:
		handleSpecialKey(keyEvent)
	}
}

// maxCol is the rightmost column the cursor may occupy on a row. Outside Edit
// mode the cursor sits *on* a rune, the way VIM's normal mode does, so it stops
// one short of the gap past the last one that Edit mode needs in order to
// append.
func maxCol(row int) int {
	lineLen := buf.RuneLen(row)
	if mode != EditMode && lineLen > 0 {
		return lineLen - 1
	}
	return lineLen
}

func clampCol() {
	if m := maxCol(currentRow); currentCol > m {
		currentCol = m
	}
}

func handleCharKey(keyEvent termbox.Event) {
	switch mode {
	case EditMode:
		insertRune(keyEvent)
	case ReadMode, VisualMode:
		handleReadModeChar(keyEvent)
		clampCol()
	}
}

// readModeActions dispatches the single-key vim motions; keys that start a
// chord are held pending instead, see chordActions.
var readModeActions = map[rune]func(){
	'G': goToBottom,
	'H': prevBuffer,
	'L': nextBuffer,
	'I': goToStartOfLine,
	'A': goToEndOfLine,
	'a': editAfterWord,
	'h': left,
	'j': down,
	'k': up,
	'l': right,
	'w': nextWord,
	'b': prevWord,
	'e': endOfWord,
	'q': closeEditor,
	'i': editBeforeWord,
	'x': deleteRune,
	'o': openLineBelow,
	'O': openLineAbove,
	'p': pasteAfter,
	'P': pasteBefore,
	'u': undo,
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
		"gg":  goToTop,
		"gd":  goToDefinition,
		"dd":  deleteLine,
		"dw":  deleteWord,
		"de":  deleteToWordEnd,
		"db":  deleteToPrevWord,
		"yy":  yankLine,
		"yw":  yankWord,
		"ye":  yankToWordEnd,
		"yb":  yankToPrevWord,
		"zz":  centerView,
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
		" wd": closeWindow,
		"  ":  openPicker,
	}

	return chords
}

var visualChords = map[string]func(){
	"gg": goToTop,
	"zz": centerView,
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
	if (ch >= '1' && ch <= '9') || (ch == '0' && pendingCount > 0) {
		pendingCount = min(pendingCount*10+int(ch-'0'), maxCount)
		return
	}

	keys, chords, prefixes := readModeActions, chordActions, chordPrefixes
	if mode == VisualMode {
		keys, chords, prefixes = visualActions, visualChords, visualChordPrefixes
	}

	if time.Since(pendingTime) >= chordTimeout {
		pendingKeys = pendingKeys[:0]
	}

	if len(pendingKeys) > 0 {
		chord := string(pendingKeys) + string(ch)
		if action, ok := chords[chord]; ok {
			pendingKeys = pendingKeys[:0]
			runCommand(action, countAwareChords[chord])
			return
		}
		if prefixes[chord] {
			pendingKeys, pendingTime = append(pendingKeys, ch), time.Now()
			return
		}
	}
	pendingKeys = pendingKeys[:0]

	// the direct keys come first, so that Visual mode's 'd' and 'y' are
	// operators in their own right rather than the halves of a chord they are
	// in Read mode
	if action, ok := keys[ch]; ok {
		runCommand(action, countAwareKeys[ch])
		return
	}

	if prefixes[string(ch)] {
		pendingKeys, pendingTime = append(pendingKeys, ch), time.Now()
	}
}

func runCommand(action func(), countAware bool) {
	cmdCount, hadCount, pendingCount = max(pendingCount, 1), pendingCount > 0, 0
	defer func() { cmdCount, hadCount = 1, false }()

	beginChange()
	if countAware {
		action()
	} else {
		for range cmdCount {
			action()
		}
	}
	endChange()
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
	termbox.KeyArrowUp:    up,
	termbox.KeyCtrlU:      pageUp,
	termbox.KeyArrowDown:  down,
	termbox.KeyCtrlD:      pageDown, // this is a vim motion actually, but it far easier to handle it like this
	termbox.KeyArrowLeft:  left,
	termbox.KeyArrowRight: right,
	termbox.KeyPgup:       pageUp,
	termbox.KeyPgdn:       pageDown,
	termbox.KeyCtrlR:      redo,
	termbox.KeyCtrlO:      jumpBack,
}

func handleSpecialKey(keyEvent termbox.Event) {
	// Space is the leader outside Edit mode, so it has to reach the chord table
	// rather than cancel what is pending there like every other special key.
	if keyEvent.Key == termbox.KeySpace && mode != EditMode {
		handleReadModeChar(termbox.Event{Ch: ' '})
		return
	}

	pendingKeys, pendingCount = pendingKeys[:0], 0 // any special key cancels a pending chord or count

	if action, ok := windowMoveKeys[keyEvent.Key]; ok && mode != EditMode {
		action()
		return
	}

	switch keyEvent.Key {
	case termbox.KeyTab:
		if mode == EditMode {
			insertRuneNTimes(keyEvent, 4)
			break
		}
		nextBuffer()
	case termbox.KeySpace:
		insertRuneNTimes(keyEvent, 1)
	case termbox.KeyHome:
		currentCol = 0
	case termbox.KeyEnd:
		currentCol = maxCol(currentRow)
	default:
		if action, ok := specialKeyActions[keyEvent.Key]; ok {
			action()
		}
	}

	clampCol()
}

func insertRuneNTimes(keyEvent termbox.Event, n int) {
	if mode != EditMode {
		return
	}
	for range n {
		insertRune(keyEvent)
	}
}

func esc() {
	// leaving Edit mode steps back off the gap the cursor was typing into
	if mode == EditMode && currentCol > 0 {
		currentCol--
	}
	mode = ReadMode
	pendingKeys, pendingCount, hlSearch = pendingKeys[:0], 0, false
	endChange()
	clampCol()
	setCursorShape(CursorDefault)
}

func up() {
	if currentRow != 0 {
		currentRow--
	}
}

func down() {
	if currentRow < buf.LineCount()-1 {
		currentRow++
	}
}

// left and right stay on their line in Read mode, the way VIM's h and l do.
// Only Edit mode wraps, so that typing can run off one line onto the next.
func left() {
	if currentCol != 0 {
		currentCol--
		return
	}
	if mode == EditMode && currentRow > 0 {
		currentRow--
		currentCol = maxCol(currentRow)
	}
}

func right() {
	if currentCol < maxCol(currentRow) {
		currentCol++
		return
	}
	if mode == EditMode && currentRow < buf.LineCount()-1 {
		currentRow++
		currentCol = 0
	}
}

func pageUp() {
	if (currentRow - ROWS/2) > 0 {
		currentRow -= ROWS / 2
	} else {
		currentRow = 0
	}
}

func pageDown() {
	if (currentRow + ROWS/2) < buf.LineCount()-1 {
		currentRow += ROWS / 2
	} else {
		currentRow = buf.LineCount() - 1
	}
}

// centerView is VIM's zz: the cursor's line is redrawn in the middle of the
// window, keeping its column. A count names the line to centre on instead.
// Near the bottom of the buffer the window is left hanging past the last line,
// the way VIM does rather than pinning the last line to the bottom row.
func centerView() {
	if hadCount {
		currentRow = min(count()-1, buf.LineCount()-1)
		clampCol()
	}
	offsetRow = max(currentRow-ROWS/2, 0)
}

func goToTop() {
	currentRow = 0
	currentCol = 0
}

func goToBottom() {
	currentRow = buf.LineCount() - 1
	currentCol = 0
}

func goToEndOfLine() {
	currentCol = buf.RuneLen(currentRow)
	enterEditMode()
}

func goToStartOfLine() {
	currentCol = 0
	enterEditMode()
}

func editAfterWord() {
	currentCol++
	enterEditMode()
}

func editBeforeWord() {
	enterEditMode()
}

func enterEditMode() {
	beginChange()
	mode = EditMode
	setCursorShape(CursorBlinkingBar)
}

func startExPrompt() { startPrompt(':') }

// closeEditor lets the editor's loop fall out and shut the terminal down on its
// way, so quitting runs the same path whether it was 'q' or ':q' that asked.
func closeEditor() {
	quitting = true
}

func isSpace(ch rune) bool {
	return ch == ' ' || ch == '\t'
}

func isWordChar(ch rune) bool {
	return ch == '_' ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9')
}

func classOf(ch rune) CharClass {
	switch {
	case isSpace(ch):
		return ClassSpace
	case isWordChar(ch):
		return ClassWord
	default:
		return ClassPunct
	}
}

// charAt treats past-end-of-line as whitespace, so word motions see line
// breaks as word boundaries without special-casing them separately.
func charAt(row, col int) rune {
	ch, ok := buf.Rune(row, col)
	if !ok {
		return ' '
	}
	return ch
}

func stepForward(row, col int) (int, int) {
	if col < buf.RuneLen(row) {
		return row, col + 1
	}
	if row < buf.LineCount()-1 {
		return row + 1, 0
	}
	return row, col
}

func nextWord() {
	currentRow, currentCol = nextWordFrom(currentRow, currentCol)
}

func nextWordFrom(row, col int) (int, int) {
	startClass := classOf(charAt(row, col))

	if startClass != ClassSpace {
		for classOf(charAt(row, col)) == startClass {
			nr, nc := stepForward(row, col)
			if nr == row && nc == col {
				break
			}
			row, col = nr, nc
		}
	}

	for classOf(charAt(row, col)) == ClassSpace {
		nr, nc := stepForward(row, col)
		if nr == row && nc == col {
			break
		}
		row, col = nr, nc
	}

	return row, col
}

func endOfWord() {
	currentRow, currentCol = endOfWordFrom(currentRow, currentCol)
}

func endOfWordFrom(row, col int) (int, int) {
	// always advance at least one position, so pressing 'e' at the end of
	// a word moves to the end of the next one instead of staying put
	row, col = stepForward(row, col)

	for classOf(charAt(row, col)) == ClassSpace {
		nr, nc := stepForward(row, col)
		if nr == row && nc == col {
			break
		}
		row, col = nr, nc
	}

	wordClass := classOf(charAt(row, col))
	for {
		nr, nc := stepForward(row, col)
		if (nr == row && nc == col) || classOf(charAt(nr, nc)) != wordClass {
			break
		}
		row, col = nr, nc
	}

	return row, col
}

func stepBackward(row, col int) (int, int) {
	if col > 0 {
		return row, col - 1
	}
	if row > 0 {
		return row - 1, buf.RuneLen(row - 1)
	}
	return row, col
}

func prevWord() {
	currentRow, currentCol = prevWordFrom(currentRow, currentCol)
}

func prevWordFrom(row, col int) (int, int) {
	// step off the current word first, so pressing 'b' from a word-start
	// lands on the previous word instead of itself
	row, col = stepBackward(row, col)

	for classOf(charAt(row, col)) == ClassSpace {
		nr, nc := stepBackward(row, col)
		if nr == row && nc == col {
			break
		}
		row, col = nr, nc
	}

	wordClass := classOf(charAt(row, col))
	for {
		pr, pc := stepBackward(row, col)
		if (pr == row && pc == col) || classOf(charAt(pr, pc)) != wordClass {
			break
		}
		row, col = pr, pc
	}

	return row, col
}
