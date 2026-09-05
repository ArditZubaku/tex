// Package theme is every colour the editor draws in, gathered so that a second
// palette is a value rather than a second copy of the rendering code.
package theme

import "github.com/nsf/termbox-go"

// A theme is every colour the editor draws in, gathered so that a second
// palette is a value rather than a second copy of the rendering code. Themes
// are addressed by position: ':theme=1' is the first one below.
type Palette struct {
	Name string

	Background termbox.Attribute
	Plain      termbox.Attribute

	Keyword, Constant, TypeName, Escape        termbox.Attribute
	Function, Builtin, StringLit, Number       termbox.Attribute
	Comment                                    termbox.Attribute
	CursorLineBg, LineNumber, CursorLineNumber termbox.Attribute
	EndOfBuffer, StatusFg, StatusBg            termbox.Attribute
	MatchFg, MatchBg, VisualBg                 termbox.Attribute

	TabBarBg, TabFg, TabActiveFg termbox.Attribute
	TabActiveBg, TabModified     termbox.Attribute
	Separator                    termbox.Attribute
}

// Themes are addressed by position, the way ':theme=1' names the first one.
var Themes = []Palette{defaultTheme, gruvboxTheme, githubDarkTheme}

// Default is the palette the editor starts in.
func Default() Palette { return Themes[2] }

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
var defaultTheme = Palette{
	Name: "default",

	Background: termbox.ColorDefault,
	Plain:      termbox.ColorDefault,

	Keyword:   termbox.ColorMagenta,
	Constant:  termbox.ColorLightMagenta,
	TypeName:  termbox.ColorYellow,
	Escape:    termbox.ColorLightYellow,
	Function:  termbox.ColorCyan,
	Builtin:   termbox.ColorLightCyan,
	StringLit: termbox.ColorGreen,
	Number:    termbox.ColorLightRed,
	Comment:   termbox.ColorLightBlue,

	// grey 236, a few steps up from black: enough to find the line by, where
	// the 8-colour palette's own dark grey reads as a selection over the text
	CursorLineBg:     color256(236),
	LineNumber:       termbox.ColorBlue,
	CursorLineNumber: termbox.ColorYellow,
	EndOfBuffer:      termbox.ColorBlue,
	StatusFg:         termbox.ColorBlack,
	StatusBg:         termbox.ColorWhite,
	MatchFg:          termbox.ColorBlack,
	MatchBg:          termbox.ColorYellow,
	// a dark blue rather than another grey: a selection over the cursor line
	// has to be told apart from the band under it
	VisualBg: color256(24),

	// the buffer line leaves the terminal's background alone the way the rest
	// of this theme does; the buffer being edited is the one on a band
	TabBarBg:    termbox.ColorDefault,
	TabFg:       color256(243),
	TabActiveFg: termbox.ColorWhite,
	TabActiveBg: color256(236),
	TabModified: termbox.ColorYellow,
	Separator:   color256(240),
}

// Gruvbox, at the 256-colour indices its own palette documents for terminals,
// and with the group each colour is taken from named alongside: the theme paints
// its own background rather than borrowing the terminal's, since half of what
// makes it gruvbox is bg0 under the text.
var gruvboxTheme = Palette{
	Name: "gruvbox",

	Background: color256(235), // bg0 #282828
	Plain:      color256(223), // fg1 #ebdbb2

	Keyword:   color256(167),                    // red    #fb4934
	Constant:  color256(175),                    // purple #d3869b
	TypeName:  color256(214),                    // yellow #fabd2f
	Escape:    color256(208),                    // orange #fe8019
	Function:  color256(142) | termbox.AttrBold, // green  #b8bb26, bold as gruvbox draws it
	Builtin:   color256(108),                    // aqua   #8ec07c
	StringLit: color256(142),                    // green  #b8bb26
	Number:    color256(175),                    // purple #d3869b
	Comment:   color256(245),                    // gray   #928374

	CursorLineBg:     color256(237), // bg1 #3c3836
	LineNumber:       color256(243), // bg4 #7c6f64
	CursorLineNumber: color256(214), // yellow, gruvbox's CursorLineNr
	EndOfBuffer:      color256(243),
	StatusFg:         color256(223),
	StatusBg:         color256(239), // bg2 #504945
	MatchFg:          color256(235),
	MatchBg:          color256(214),
	VisualBg:         color256(239), // bg2 #504945, gruvbox's own Visual

	TabBarBg:    color256(237), // bg1 #3c3836
	TabFg:       color256(243), // bg4 #7c6f64
	TabActiveFg: color256(223), // fg1, over bg0 so the current tab reads as the text below it
	TabActiveBg: color256(235), // bg0 #282828
	TabModified: color256(214), // yellow #fabd2f
	Separator:   color256(239), // bg2 #504945, gruvbox's own VertSplit
}

// GitHub's dark default, at the nearest 256-colour index to each of the hex
// values its own theme publishes. It leans on fewer hues than the palettes
// above — a name that can be called is purple whether the language defines it
// or the file does, and a literal is blue whatever its type — which is GitHub's
// choice rather than a gap here.
var githubDarkTheme = Palette{
	Name: "github-dark",

	Background: color256(233), // canvas.default #0d1117
	Plain:      color256(255), // fg.default     #e6edf3

	Keyword:   color256(210), // red    #ff7b72
	Constant:  color256(111), // blue   #79c0ff
	TypeName:  color256(114), // green  #7ee787
	Escape:    color256(111), // blue   #79c0ff
	Function:  color256(183), // purple #d2a8ff
	Builtin:   color256(183), // purple #d2a8ff, as GitHub draws the standard library too
	StringLit: color256(153), // light blue #a5d6ff
	Number:    color256(111), // blue   #79c0ff
	Comment:   color256(246), // fg.muted #8b949e

	CursorLineBg:     color256(234), // canvas.subtle #161b22
	LineNumber:       color256(243), // fg.subtle     #6e7681
	CursorLineNumber: color256(255),
	EndOfBuffer:      color256(243),
	StatusFg:         color256(255),
	StatusBg:         color256(237), // border.default #30363d
	MatchFg:          color256(255),
	MatchBg:          color256(130), // findMatch #9e6a03
	VisualBg:         color256(24),  // selection #388bfd at the alpha GitHub draws it with

	TabBarBg:    color256(234), // canvas.subtle #161b22
	TabFg:       color256(243), // fg.subtle     #6e7681
	TabActiveFg: color256(255),
	TabActiveBg: color256(233), // canvas.default, as GitHub draws the open tab
	TabModified: color256(178), // attention.fg  #d29922
	Separator:   color256(237), // border.default #30363d
}
