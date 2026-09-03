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
	matchFg, matchBg, visualBg                 termbox.Attribute

	tabBarBg, tabFg, tabActiveFg termbox.Attribute
	tabActiveBg, tabModified     termbox.Attribute
}

var themes = []theme{defaultTheme, gruvboxTheme, githubDarkTheme}

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
	// a dark blue rather than another grey: a selection over the cursor line
	// has to be told apart from the band under it
	visualBg: color256(24),

	// the buffer line leaves the terminal's background alone the way the rest
	// of this theme does; the buffer being edited is the one on a band
	tabBarBg:    termbox.ColorDefault,
	tabFg:       color256(243),
	tabActiveFg: termbox.ColorWhite,
	tabActiveBg: color256(236),
	tabModified: termbox.ColorYellow,
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
	visualBg:         color256(239), // bg2 #504945, gruvbox's own Visual

	tabBarBg:    color256(237), // bg1 #3c3836
	tabFg:       color256(243), // bg4 #7c6f64
	tabActiveFg: color256(223), // fg1, over bg0 so the current tab reads as the text below it
	tabActiveBg: color256(235), // bg0 #282828
	tabModified: color256(214), // yellow #fabd2f
}

// GitHub's dark default, at the nearest 256-colour index to each of the hex
// values its own theme publishes. It leans on fewer hues than the palettes
// above — a name that can be called is purple whether the language defines it
// or the file does, and a literal is blue whatever its type — which is GitHub's
// choice rather than a gap here.
var githubDarkTheme = theme{
	name: "github-dark",

	background: color256(233), // canvas.default #0d1117
	plain:      color256(255), // fg.default     #e6edf3

	keyword:   color256(210), // red    #ff7b72
	constant:  color256(111), // blue   #79c0ff
	typeName:  color256(114), // green  #7ee787
	escape:    color256(111), // blue   #79c0ff
	function:  color256(183), // purple #d2a8ff
	builtin:   color256(183), // purple #d2a8ff, as GitHub draws the standard library too
	stringLit: color256(153), // light blue #a5d6ff
	number:    color256(111), // blue   #79c0ff
	comment:   color256(246), // fg.muted #8b949e

	cursorLineBg:     color256(234), // canvas.subtle #161b22
	lineNumber:       color256(243), // fg.subtle     #6e7681
	cursorLineNumber: color256(255),
	endOfBuffer:      color256(243),
	statusFg:         color256(255),
	statusBg:         color256(237), // border.default #30363d
	matchFg:          color256(255),
	matchBg:          color256(130), // findMatch #9e6a03
	visualBg:         color256(24),  // selection #388bfd at the alpha GitHub draws it with

	tabBarBg:    color256(234), // canvas.subtle #161b22
	tabFg:       color256(243), // fg.subtle     #6e7681
	tabActiveFg: color256(255),
	tabActiveBg: color256(233), // canvas.default, as GitHub draws the open tab
	tabModified: color256(178), // attention.fg  #d29922
}
