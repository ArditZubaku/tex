// Package prompt is the line of input the status bar is handed over for: VIM's
// '/' and '?' searches and its ':' commands, which differ only in what pressing
// Enter does with what was typed.
package prompt

import (
	"slices"

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
	label     string
	input     []rune
	cursor    int
}

func (l *Line) Start(delimiter rune) { l.StartWith(delimiter, "") }

// StartWith opens the line on text already typed, which is what '<leader>cr'
// hands the name it is about to rename.
func (l *Line) StartWith(delimiter rune, input string) { l.StartLabelled(delimiter, "", input) }

// StartLabelled opens the line under a word rather than under its delimiter,
// for a prompt a delimiter would say nothing about. The delimiter is still what
// tells one prompt from another once Enter is pressed.
func (l *Line) StartLabelled(delimiter rune, label, input string) {
	l.delimiter, l.label = delimiter, label
	l.input = append(l.input[:0], []rune(input)...)
	l.cursor = len(l.input)
}

func (l *Line) Delimiter() rune { return l.delimiter }

func (l *Line) Input() []rune { return l.input }

func (l *Line) Text() string { return l.prefix() + string(l.input) }

func (l *Line) Width() int { return runewidth.StringWidth(l.Text()) }

// CursorWidth is where the cursor sits within Text(), in terminal columns.
func (l *Line) CursorWidth() int {
	return runewidth.StringWidth(l.prefix()) + runewidth.StringWidth(string(l.input[:l.cursor]))
}

func (l *Line) prefix() string {
	if l.label != "" {
		return l.label
	}

	return string(l.delimiter)
}

func (l *Line) Clear() { l.input, l.cursor = l.input[:0], 0 }

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
		if l.cursor > 0 {
			l.input = slices.Delete(l.input, l.cursor-1, l.cursor)
			l.cursor--
		}
	case termbox.KeyArrowLeft:
		if l.cursor > 0 {
			l.cursor--
		}
	case termbox.KeyArrowRight:
		if l.cursor < len(l.input) {
			l.cursor++
		}
	case termbox.KeyHome:
		l.cursor = 0
	case termbox.KeyEnd:
		l.cursor = len(l.input)
	case termbox.KeyCtrlU:
		l.input = slices.Delete(l.input, 0, l.cursor)
		l.cursor = 0
	case termbox.KeySpace:
		l.insert(' ')
	default:
		if event.Ch != 0 {
			l.insert(event.Ch)
		}
	}

	return Handled
}

func (l *Line) insert(r rune) {
	l.input = slices.Insert(l.input, l.cursor, r)
	l.cursor++
}
