package main

import "github.com/nsf/termbox-go"

// A theme is every colour the editor draws in, gathered so that a second
// palette is a value rather than a second copy of the rendering code. Themes
// are addressed by position: ':theme=1' is the first one below.
type theme struct {
	name string

	background termbox.Attribute
	plain      termbox.Attribute

	keyword, constant, typeName, escape        termbox.Attribute
	function, builtin, stringLit, number       termbox.Attribute
	comment                                    termbox.Attribute
	cursorLineBg, lineNumber, cursorLineNumber termbox.Attribute
	endOfBuffer, statusFg, statusBg            termbox.Attribute
	matchFg, matchBg                           termbox.Attribute
}

var themes = []theme{defaultTheme, gruvboxTheme}

var active = themes[0]

// color256 turns a palette index into the attribute termbox wants, which
// numbers colours from 1 so that zero can mean "whatever the terminal uses".
func color256(index int) termbox.Attribute {
	return termbox.Attribute(index + 1)
}

// The default theme gives related token classes neighbouring hues — magenta
// for the words the language reserves, yellow for the names of things, cyan for
// what can be called, red for literal numbers — so the screen reads as a few
// colour families rather than a dozen unrelated colours. Every one of them is a
// bright shade, which keeps it legible over the cursor line's grey band as well
// as over the terminal's own background, which it leaves alone.
var defaultTheme = theme{
	name: "default",

	background: termbox.ColorDefault,
	plain:      termbox.ColorDefault,

	keyword:   termbox.ColorMagenta,
	constant:  termbox.ColorLightMagenta,
	typeName:  termbox.ColorYellow,
	escape:    termbox.ColorLightYellow,
	function:  termbox.ColorCyan,
	builtin:   termbox.ColorLightCyan,
	stringLit: termbox.ColorGreen,
	number:    termbox.ColorLightRed,
	comment:   termbox.ColorLightBlue,

	// grey 236, a few steps up from black: enough to find the line by, where
	// the 8-colour palette's own dark grey reads as a selection over the text
	cursorLineBg:     color256(236),
	lineNumber:       termbox.ColorBlue,
	cursorLineNumber: termbox.ColorYellow,
	endOfBuffer:      termbox.ColorBlue,
	statusFg:         termbox.ColorBlack,
	statusBg:         termbox.ColorWhite,
	matchFg:          termbox.ColorBlack,
	matchBg:          termbox.ColorYellow,
}

// Gruvbox, at the 256-colour indices its own palette documents for terminals,
// and with the group each colour is taken from named alongside: the theme paints
// its own background rather than borrowing the terminal's, since half of what
// makes it gruvbox is bg0 under the text.
var gruvboxTheme = theme{
	name: "gruvbox",

	background: color256(235), // bg0 #282828
	plain:      color256(223), // fg1 #ebdbb2

	keyword:   color256(167),                    // red    #fb4934
	constant:  color256(175),                    // purple #d3869b
	typeName:  color256(214),                    // yellow #fabd2f
	escape:    color256(208),                    // orange #fe8019
	function:  color256(142) | termbox.AttrBold, // green  #b8bb26, bold as gruvbox draws it
	builtin:   color256(108),                    // aqua   #8ec07c
	stringLit: color256(142),                    // green  #b8bb26
	number:    color256(175),                    // purple #d3869b
	comment:   color256(245),                    // gray   #928374

	cursorLineBg:     color256(237), // bg1 #3c3836
	lineNumber:       color256(243), // bg4 #7c6f64
	cursorLineNumber: color256(214), // yellow, gruvbox's CursorLineNr
	endOfBuffer:      color256(243),
	statusFg:         color256(223),
	statusBg:         color256(239), // bg2 #504945
	matchFg:          color256(235),
	matchBg:          color256(214),
}
