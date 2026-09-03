# txi

A small terminal text editor written in Go, built on [termbox-go](https://github.com/nsf/termbox-go). It's modal like VIM (separate Normal/Read and Insert/Edit modes) and currently implements a subset of VIM's motions and editing keys.

## Usage

```sh
go build -o txi .
./txi path/to/file      # opens (or creates) a file
./txi                   # starts a new, unnamed buffer (out.txt)
```

## Features

- **Modal editing** — a Read (Normal) mode for navigation and an Edit (Insert) mode for typing, with the terminal cursor changing shape (block vs. blinking bar) depending on mode. The cursor sits *on* a character in Normal mode, stopping at the last one on the line like VIM does; only Insert mode reaches the column past it, where appending happens.
- **VIM-style navigation** — `hjkl`, word motions (`w`/`b`/`e`), line jumps (`I`/`A`), buffer jumps (`gg`/`G`); see the full list below.
- **Saving** — `:w` or `Ctrl-S`, in either mode. The buffer is streamed to a temporary file in the same directory and renamed over the target, so a failed write cannot truncate the original; untouched lines are copied as raw bytes, so a save costs no more memory than scrolling does. File permissions, CRLF line endings on untouched lines, and a missing trailing newline are all preserved.
- **Deleting** — `x`, `dw`, `de`, `db` and `dd` in Normal mode, `Backspace` in Insert mode, all in place: a line delete compacts the line index rather than rebuilding it, and a character delete reuses the edited line's own backing array, so deleting never costs more memory than the text it removes.
- **Line structure** — `o`/`O` open a line, `Enter` splits one at the cursor and `Backspace` at column 0 joins it back. Each shifts the line index in place rather than rebuilding it, and a split copies only the tail: the half before the cursor keeps the array it already had.
- **Yank and put** — `yy`, `yw`, `ye` and `yb` copy into VIM's unnamed register, which the delete operators fill too, and `p`/`P` put it back after or before the cursor. A line yank puts whole lines below or above the cursor; a word yank puts the run of characters back into the line the cursor is on.
- **Undo and redo** — `u` and `Ctrl-R`, with an insert session (from `i` to `Esc`) undone in one step the way VIM does it. Nothing snapshots the buffer: a change remembers only the lines the command actually touched, so the undo history costs the text that was edited rather than the size of the file. The last 500 changes are kept.
- **Counts** — a command can be prefixed with a repeat count, as in `3j`, `3x`, `2dd` or `yy3p`. Commands where the count says *how much text* rather than *how many times* (`x`, `dd`, `yy`, `p`, `P`) act on that much text in a single step, so one `u` takes the whole thing back.
- **Word-class-aware word motions** — `w`/`b`/`e` classify runs of characters into whitespace / word (`[A-Za-z0-9_]`) / punctuation, so e.g. `"foo` is treated as two words (`"` then `foo`), matching VIM's default word boundaries.
- **Search** — `/` opens a prompt on the status line, `?` opens it searching backwards, and `Enter` jumps the cursor to the first rune of the match, wrapping around the end of the buffer like VIM does. `n` and `N` repeat it with or against its direction (`3n` for the third match), `Enter` on an empty prompt reuses the last pattern, and every match on screen is lit until `Esc` clears the highlight. Patterns are literal text — spaces included, so `/quick brown` is one pattern — and case-sensitive. The search never decodes the file: it matches `bytes.Index` over the raw bytes of each line as they come through the same 64KB read window everything else uses, decoding only the text in front of a hit to turn its byte offset into a column, so scanning a 12MB / 200k-line file end to end costs **~8ms** and no more memory than scrolling through it does.
- **Relative line numbers** — a gutter on the left showing each line's distance from the cursor, with the cursor's own line carrying its absolute number instead, like VIM's `number`/`relativenumber` pair. It widens with the buffer's line count (four columns until the count reaches four digits) and the text and horizontal scrolling start after it.
- **Cursor line highlight** — the line the cursor is on is drawn as a faint band across the full width of the terminal, gutter included, like VIM's `cursorline`, with its line number in yellow. The theme picks the shade: grey 236 of the 256-colour palette by default, gruvbox's own `bg1` under theme 2 — dark enough to sit under the text rather than compete with it, which is what the editor asks the terminal for 256-colour output for.
- **Syntax highlighting** — nine token classes, coloured by the theme in use; the default palette groups them in families so the screen reads as a handful of colours rather than a dozen: magenta for the words the language reserves (keywords, and a lighter shade for literals like `true` and `nil`), yellow for the names of things (types, and escape sequences inside strings), cyan for what can be called (a name with a `(` after it, and a lighter shade for built-ins like `len` or `print`), green for strings, red for numbers and blue for comments. The language is picked by the file's name: Go, the C family (C/C++, C#, Java, JavaScript/TypeScript, Rust, Kotlin, Swift, PHP…), and everything whose comments start with `#`, from Python and the shells to YAML and `Makefile`. A file that matches no rule is drawn plain. Each line is lexed as it is drawn, so highlighting costs a screenful of text and nothing about it scales with the size of the file; block comments are the one construct that spans lines, and one that started above the window is found by lexing at most 64 lines back.
- **Viewport scrolling** — the visible window follows the cursor both vertically and horizontally as the buffer grows past the terminal size. `zz` recentres it on the cursor's line without moving the cursor, and `40zz` centres on line 40, jumping there first; near the end of the buffer the window is left hanging past the last line rather than pinned to it, the way VIM does it.
- **Ex commands** — `:` opens the same prompt the search does, for the commands a VIM user types without thinking: `:w` (and `:w other.txt`, which writes there and carries on editing that file, like `:saveas`), `:q`, `:wq`, `:x`, `:q!` to leave unsaved changes behind, `:nohlsearch` to drop the search highlight, and a bare line number — `:42`, `:$` — to jump there. `:q` on a modified buffer refuses and says so rather than losing the changes; the raw `q` key still quits outright, without that check. Anything unrecognised is reported (`E492`) instead of guessed at.
- **Themes** — `:theme=2` switches the whole palette, `:theme=1` goes back, and `:theme` on its own names what is in use. Theme 1 is the editor's own colours over the terminal's background; theme 2 is [gruvbox](https://github.com/morhetz/gruvbox) at the 256-colour indices its palette documents — red keywords, yellow types, green strings and bold green functions, purple literals, aqua built-ins, grey comments, over its own `bg0` rather than the terminal's background. Every colour the editor draws in, syntax and interface alike, comes out of the theme in use, so a third one is a value in `theme.go` and nothing else.
- **Status bar** — current mode, file name, line count, modified/saved state, whether the register and the undo/redo stacks hold anything, the count being typed, and cursor row/column. The prompt takes the line over while a search or a `:` command is being typed, and what a command has to report — a write, a pattern that matched nothing, a refused quit — is shown there.
- **Constant-memory file loading** — the file is never held in memory. Opening it builds an index of where each line starts (8 bytes per line) and nothing else; lines are read through one fixed 64KB window and decoded to runes only when they're on screen or under the cursor. Opening a 23MB file of 202,000 lines and jumping to the end costs **8.8MB of RSS**, and that figure doesn't move however far you scroll — 5.2MB of it is the Go runtime floor a one-line file also pays, so the file itself accounts for 2.6MB. See [Memory model](#memory-model).

## VIM motions implemented

| Key | Mode | Action |
| ----- | ------ | -------- |
| `h` `j` `k` `l` | Normal | move left / down / up / right, stopping at the ends of the line |
| `w` | Normal | jump to the start of the next word |
| `b` | Normal | jump to the start of the previous word |
| `e` | Normal | jump to the end of the (next) word |
| `x` | Normal | delete the character under the cursor |
| `dw` | Normal | delete to the start of the next word (stops at end of line) |
| `de` | Normal | delete to the end of the current word (stops at end of line) |
| `db` | Normal | delete back to the start of the previous word (stops at the start of the line) |
| `dd` | Normal | delete the current line |
| `yy` | Normal | yank the current line |
| `yw` `ye` `yb` | Normal | yank over the matching word motion (stops at the ends of the line) |
| `p` | Normal | put the register after the cursor, or on the line below if it holds whole lines |
| `P` | Normal | put the register before the cursor, or on the line above |
| `/` | Normal | open the search prompt; `Enter` jumps to the next match, `Esc` cancels |
| `?` | Normal | the same, searching backwards |
| `n` | Normal | jump to the next match in the search's direction |
| `N` | Normal | jump to the next match against it |
| `:w` | Normal | write the buffer (`:w name` writes there and keeps editing it) |
| `:q` | Normal | quit, refusing if there are unsaved changes |
| `:q!` | Normal | quit, dropping unsaved changes |
| `:wq` `:x` | Normal | write, then quit |
| `:42` `:$` | Normal | jump to that line / the last line |
| `:noh` | Normal | clear the search highlight |
| `:theme=2` | Normal | switch palette (`1` default, `2` gruvbox; bare `:theme` names it) |
| `u` | Normal | undo the last change |
| `Ctrl-R` | Normal | redo the last undone change |
| `1`–`9` | Normal | start a count for the next command, e.g. `3p` |
| `Backspace` | Insert | delete the character before the cursor, joining onto the line above at column 0 |
| `Enter` | Insert | split the line at the cursor (in Normal mode it moves down a line) |
| `gg` | Normal | jump to the top of the buffer |
| `G` | Normal | jump to the bottom of the buffer |
| `I` | Normal | jump to start of line and enter Insert mode |
| `A` | Normal | jump to end of line and enter Insert mode |
| `o` | Normal | open an empty line below and enter Insert mode |
| `O` | Normal | open an empty line above and enter Insert mode |
| `i` | Normal | enter Insert mode before the cursor |
| `a` | Normal | enter Insert mode after the cursor |
| `Ctrl-S` | Either | save to the file that was opened |
| `zz` | Normal | redraw with the cursor's line in the middle of the window, keeping the column (`[count]zz` centres on that line) |
| `Ctrl-U` | Either | scroll up half a screen |
| `Ctrl-D` | Either | scroll down half a screen |
| `Esc` | Insert | return to Normal mode (cursor steps back a column, VIM-style) |

Arrow keys, `Home`, `End`, `PgUp`, and `PgDn` also work in either mode. Left and
right stop at the ends of the line in Normal mode; in Insert mode they wrap onto
the neighbouring line, so typing can run off one line onto the next.

## Memory model

The file is never loaded. It stays on disk with its handle open, and `buffer.go`
holds three things, none of which scale with how much of it you have visited:

| | held | 23MB / 202k-line file |
| --- | --- | --- |
| Line index (`starts`) | one byte offset per line | ~1.6MB |
| Read window (`win`) | raw bytes around the cursor | 64KB |
| Decoded line cache | the one line under the cursor | a few hundred bytes |

Opening a file makes a single indexing pass with `ReadAt` through one reusable 1MB
buffer, recording where each line begins. Reading through a small buffer rather than
walking an `mmap` is deliberate: faulting a mapping in to find its newlines makes
every page of the file resident, and macOS will not hand those pages back
(`MADV_FREE_REUSABLE` is `EPERM` on file mappings). A read leaves the data in the OS
page cache without charging it to the process.

Everything after that pass is random access through the index: `G` and `gg` are
O(1), the status bar's line count is exact, and displaying a line is one `pread`
into the window when the line falls outside it. Lines edited in Insert mode live in
an overlay map that shadows the file, so an edit is never lost when the window moves.
Lifting a line into that overlay copies it once; every keystroke after that grows or
shrinks it in place, so typing a word costs no allocation per character.

Opening a line with `o`/`O` adds an index entry that borrows the offset of the line
below it, which leaves every neighbouring line's extent exactly as it was; the new
line itself is only ever read from the overlay that shadows it.

Deleting a line drops its entry from the index and renumbers the overlay; the bytes
themselves stay on disk, unreferenced. Because a line ends where the next one starts,
the line *above* a deleted one would otherwise inherit its bytes, so that one line is
copied into the overlay — the only allocation a `dd` makes, and it is bounded by the
length of that single line rather than by the size of the file.

A line wider than the window grows it for as long as that line is on screen, then it
shrinks back. Only the index scales with file size, at 8 bytes per line.

### Goals not yet implemented

- The rest of `:` command mode — ranges (`:1,5d`), `:s`, `:e`, `:r`, `:set` and the like; `:w`, `:q`, `:wq`, `:x`, `:q!`, `:noh`, `:theme` and a bare line address are all that is there
- Visual mode
- Regular expressions in a search pattern — `/` matches literal text, so `\v`, `*`, character classes and `:s` are not there; nor are `ignorecase`/`smartcase`, `*` and `#` (search for the word under the cursor), or a search used as an operator's motion (`d/foo`)
- The rest of the operators and text objects (`c`, `cw`, `dj`, `di(`, ...) — only `x`, `dw`, `de`, `db`, `dd` and the `y` operators exist so far, and they stop at the line boundary instead of running onto the next line
- Named registers (`"a`) — there is only the unnamed one
- Counts in front of an operator's motion (`d3w`) — a count goes before the whole command (`3dw`)
- Marks and macros
