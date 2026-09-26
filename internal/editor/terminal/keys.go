package terminal

import "github.com/nsf/termbox-go"

// cursorSeqs and appCursorSeqs are the arrows and Home/End in xterm's two
// conventions: CSI normally, SS3 once the shell has asked for DECCKM, which
// full-screen programs — vim among them — do and a plain prompt does not.
var (
	cursorSeqs = map[termbox.Key]string{
		termbox.KeyArrowUp: "\x1b[A", termbox.KeyArrowDown: "\x1b[B",
		termbox.KeyArrowRight: "\x1b[C", termbox.KeyArrowLeft: "\x1b[D",
		termbox.KeyHome: "\x1b[H", termbox.KeyEnd: "\x1b[F",
	}
	appCursorSeqs = map[termbox.Key]string{
		termbox.KeyArrowUp: "\x1bOA", termbox.KeyArrowDown: "\x1bOB",
		termbox.KeyArrowRight: "\x1bOC", termbox.KeyArrowLeft: "\x1bOD",
		termbox.KeyHome: "\x1bOH", termbox.KeyEnd: "\x1bOF",
	}
	navSeqs = map[termbox.Key]string{
		termbox.KeyPgup: "\x1b[5~", termbox.KeyPgdn: "\x1b[6~",
		termbox.KeyInsert: "\x1b[2~", termbox.KeyDelete: "\x1b[3~",
		termbox.KeyF1: "\x1bOP", termbox.KeyF2: "\x1bOQ", termbox.KeyF3: "\x1bOR", termbox.KeyF4: "\x1bOS",
		termbox.KeyF5: "\x1b[15~", termbox.KeyF6: "\x1b[17~", termbox.KeyF7: "\x1b[18~", termbox.KeyF8: "\x1b[19~",
		termbox.KeyF9: "\x1b[20~", termbox.KeyF10: "\x1b[21~", termbox.KeyF11: "\x1b[23~", termbox.KeyF12: "\x1b[24~",
	}
)

// Encode is one key event as the bytes the shell expects it to have typed.
// A rune goes as itself; everything else that termbox already gives a literal
// byte value — the Ctrl combinations, Enter, Tab, Esc, Backspace, Space —
// goes as that byte, the same as a real terminal would send it. What is left
// is the keys termbox has no byte for at all, which is what the tables above
// are for.
func Encode(keyEvent termbox.Event, appCursor bool) []byte {
	if keyEvent.Ch != 0 {
		return []byte(string(keyEvent.Ch))
	}

	seqs := cursorSeqs
	if appCursor {
		seqs = appCursorSeqs
	}
	if seq, ok := seqs[keyEvent.Key]; ok {
		return []byte(seq)
	}
	if seq, ok := navSeqs[keyEvent.Key]; ok {
		return []byte(seq)
	}

	return []byte{byte(keyEvent.Key)}
}
