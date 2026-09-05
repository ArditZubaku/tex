// Package prompt is the line of input the status bar is handed over for: VIM's
// '/' and '?' searches and its ':' commands, which differ only in what pressing
// Enter does with what was typed.
package prompt

import (
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

type Action int

const (
	Handled Action = iota
	Closed
	Submitted
)

type Line struct {
	delimiter rune
	input     []rune
}

func (l *Line) Start(delimiter rune) {
	l.delimiter, l.input = delimiter, l.input[:0]
}

func (l *Line) Delimiter() rune { return l.delimiter }

func (l *Line) Input() []rune { return l.input }

func (l *Line) Text() string { return string(l.delimiter) + string(l.input) }

func (l *Line) Width() int { return runewidth.StringWidth(string(l.input)) + 1 }

func (l *Line) Clear() { l.input = l.input[:0] }

func (l *Line) Key(event termbox.Event) Action {
	switch event.Key {
	case termbox.KeyEsc:
		return Closed
	case termbox.KeyEnter:
		return Submitted
	case termbox.KeyBackspace, termbox.KeyBackspace2:
		// backspacing off the delimiter leaves the prompt, as it does in VIM
		if len(l.input) == 0 {
			return Closed
		}
		l.input = l.input[:len(l.input)-1]
	case termbox.KeyCtrlU:
		l.input = l.input[:0]
	case termbox.KeySpace:
		l.input = append(l.input, ' ')
	default:
		if event.Ch != 0 {
			l.input = append(l.input, event.Ch)
		}
	}

	return Handled
}
