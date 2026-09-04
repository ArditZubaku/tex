# txi

A small terminal text editor written in Go, built on [termbox-go](https://github.com/nsf/termbox-go). It's modal like VIM (separate Normal/Read, Insert/Edit and Visual modes) and currently implements a subset of VIM's motions and editing keys.

## Usage

```sh
go build -o txi .
./txi path/to/file      # opens (or creates) a file
./txi                   # starts a new, unnamed buffer (out.txt)
```

## Features

- **Modal editing** — a Read (Normal) mode for navigation, an Edit (Insert) mode for typing and a Visual mode for selecting, with the terminal cursor changing shape (block vs. blinking bar) depending on mode. The cursor sits *on* a character in Normal mode, stopping at the last one on the line like VIM does; only Insert mode reaches the column past it, where appending happens.
- **VIM-style navigation** — `hjkl`, word motions (`w`/`b`/`e`), line jumps (`I`/`A`), buffer jumps (`gg`/`G`); see the full list below.
- **Saving** — `:w` or `Ctrl-S`, in either mode. The buffer is streamed to a temporary file in the same directory and renamed over the target, so a failed write cannot truncate the original; untouched lines are copied as raw bytes, so a save costs no more memory than scrolling does. File permissions, CRLF line endings on untouched lines, and a missing trailing newline are all preserved.
- **Deleting** — `x`, `dw`, `de`, `db` and `dd` in Normal mode, `Backspace` in Insert mode, all in place: a line delete compacts the line index rather than rebuilding it, and a character delete reuses the edited line's own backing array, so deleting never costs more memory than the text it removes.
- **Line structure** — `o`/`O` open a line, `Enter` splits one at the cursor and `Backspace` at column 0 joins it back. Each shifts the line index in place rather than rebuilding it, and a split copies only the tail: the half before the cursor keeps the array it already had.
- **Yank and put** — `yy`, `yw`, `ye` and `yb` copy into VIM's unnamed register, which the delete operators fill too, and `p`/`P` put it back after or before the cursor. A line yank puts whole lines below or above the cursor; a word yank puts the run of characters back into the line the cursor is on.
- **Visual mode** — `v` selects runes and `V` whole lines, from where the mode was entered to wherever a motion has taken the cursor since; both ends are part of the selection, as VIM's `selection=inclusive` has them. `d` (or `x`) deletes it, `y` yanks it, `c` changes it, and `o` swaps the end that the next motion drags along. `v` and `V` switch between the two shapes and leave the mode when pressed on the shape it already has, and `Esc` drops the selection. The selection is drawn as a band in the theme's own colour over the text, keeping the syntax colours under it, and it takes in the line break of every line it carries on past. A run that spans lines fills the register as a run — `p` splits the line it is put into and joins the register's ends onto the halves — so a selection can be moved from one place to another whatever it covers.
- **Undo and redo** — `u` and `Ctrl-R`, with an insert session (from `i` to `Esc`) undone in one step the way VIM does it. Nothing snapshots the buffer: a change remembers only the lines the command actually touched, so the undo history costs the text that was edited rather than the size of the file. The last 500 changes are kept.
- **Counts** — a command can be prefixed with a repeat count, as in `3j`, `3x`, `2dd` or `yy3p`. Commands where the count says *how much text* rather than *how many times* (`x`, `dd`, `yy`, `p`, `P`) act on that much text in a single step, so one `u` takes the whole thing back.
- **Word-class-aware word motions** — `w`/`b`/`e` classify runs of characters into whitespace / word (`[A-Za-z0-9_]`) / punctuation, so e.g. `"foo` is treated as two words (`"` then `foo`), matching VIM's default word boundaries.
- **Search** — `/` opens a prompt on the status line, `?` opens it searching backwards, and `Enter` jumps the cursor to the first rune of the match, wrapping around the end of the buffer like VIM does. `n` and `N` repeat it with or against its direction (`3n` for the third match), `Enter` on an empty prompt reuses the last pattern, and every match on screen is lit until `Esc` clears the highlight. Patterns are literal text — spaces included, so `/quick brown` is one pattern — and case-sensitive. The search never decodes the file: it matches `bytes.Index` over the raw bytes of each line as they come through the same 64KB read window everything else uses, decoding only the text in front of a hit to turn its byte offset into a column, so scanning a 12MB / 200k-line file end to end costs **~8ms** and no more memory than scrolling through it does.
- **Relative line numbers** — a gutter on the left showing each line's distance from the cursor, with the cursor's own line carrying its absolute number instead, like VIM's `number`/`relativenumber` pair. It widens with the buffer's line count (four columns until the count reaches four digits) and the text and horizontal scrolling start after it.
- **Cursor line highlight** — the line the cursor is on is drawn as a faint band across the width of its window, gutter included, like VIM's `cursorline`, with its line number in yellow. The theme picks the shade: grey 236 of the 256-colour palette by default, gruvbox's own `bg1` under theme 2 — dark enough to sit under the text rather than compete with it, which is what the editor asks the terminal for 256-colour output for.
- **Syntax highlighting** — nine token classes, coloured by the theme in use; the default palette groups them in families so the screen reads as a handful of colours rather than a dozen: magenta for the words the language reserves (keywords, and a lighter shade for literals like `true` and `nil`), yellow for the names of things (types, and escape sequences inside strings), cyan for what can be called (a name with a `(` after it, and a lighter shade for built-ins like `len` or `print`), green for strings, red for numbers and blue for comments. The language is picked by the file's name: Go, the C family (C/C++, C#, Java, JavaScript/TypeScript, Rust, Kotlin, Swift, PHP…), and everything whose comments start with `#`, from Python and the shells to YAML and `Makefile`. A file that matches no rule is drawn plain. Each line is lexed as it is drawn, so highlighting costs a screenful of text and nothing about it scales with the size of the file; block comments are the one construct that spans lines, and one that started above the window is found by lexing at most 64 lines back.
- **Viewport scrolling** — the visible window follows the cursor both vertically and horizontally as the buffer grows past the terminal size. `zz` recentres it on the cursor's line without moving the cursor, and `40zz` centres on line 40, jumping there first; near the end of the buffer the window is left hanging past the last line rather than pinned to it, the way VIM does it.
- **Ex commands** — `:` opens the same prompt the search does, for the commands a VIM user types without thinking: `:w` (and `:w other.txt`, which writes there and carries on editing that file, like `:saveas`), `:q`, `:wq`, `:x`, `:q!` to leave unsaved changes behind, `:e other.txt` to edit another file (`:e` on its own rereads the current one), `:bnext`/`:bprev`/`:bdelete`/`:ls` for the buffer list, `:split`/`:vsplit`/`:close`/`:only` for windows, `:nohlsearch` to drop the search highlight, and a bare line number — `:42`, `:$` — to jump there. `:q` closes the window it is typed in and quits once that was the last one; on a modified buffer it refuses and says so rather than losing the changes, and names the buffer when it is one of the others that is unsaved (`E162`); the raw `q` key still quits outright, without that check. Anything unrecognised is reported (`E492`) instead of guessed at.
- **Themes** — `:theme=2` switches the whole palette, `:theme=1` goes back, and `:theme` on its own names what is in use. Three so far: **1** the editor's own colours, the only one that leaves the terminal's background alone; **2** [gruvbox](https://github.com/morhetz/gruvbox) — red keywords, yellow types, green strings and bold green functions, purple literals, aqua built-ins, grey comments over its `bg0`; **3** GitHub's dark default — red keywords, green type names, purple functions, blue literals, light-blue strings over `#0d1117`. Both are the nearest 256-colour index to each hex value the palette itself publishes. Every colour the editor draws in, syntax and interface alike, comes out of the theme in use, so a fourth one is a value in `theme.go` and nothing else.
- **File explorer** — `<leader>e` (Space, then `e`) lists the directory the file being edited lives in, over the window it is opened in the way `netrw` does rather than in a sidebar — the other windows of a split carry on showing their buffers. Directories come first and carry a trailing `/`, `..` sits on top, and `hjkl` move the way they do in the buffer: `j`/`k` walk the listing, `Enter` or `l` descends into a directory or opens a file, `h` or `-` steps back out — landing on the directory just left rather than at the top — `g`/`G` jump to the ends and `Ctrl-D`/`Ctrl-U` move half a screen at a time, as they do in the buffer. `H` shows the dotfiles it hides by default, `q` returns to the buffer, `Ctrl-H`/`J`/`K`/`L` leave it for the window that way, and a second `<leader>e` closes the explorer the same way the first opened it. `/` hands the status line to the same prompt the buffer's search uses and narrows the listing to the names matching what is typed, case-insensitively, as it is typed: `Enter` settles on that listing, `Esc` puts the whole directory back, and a second `Esc` — with nothing left to drop — closes the explorer. Moving to another directory drops the filter with it; toggling dotfiles keeps it. Opening a file adds it to the buffer list and edits it, leaving the buffer it was opened from — unsaved changes and all — a `Tab` away.

- **Buffers** — every file opened stays open, the way VIM's hidden buffers and LazyVim's buffer line do: `:e` and the explorer put the new file *beside* the one being edited rather than over it, and `Tab` walks the list, wrapping at the end. `H` and `L` are LazyVim's own step back and forward through it. A buffer keeps what belongs to it — its cursor, its viewport, its unsaved changes and its undo history — so switching away and back leaves the file exactly as it was, and editing a file already in the list switches to it rather than opening it twice. The list is drawn as a row of tabs across the top of the window, the buffer being edited picked out in the theme's own colours and an unsaved one carrying a dot; the row scrolls sideways to keep the current tab on screen when more buffers are open than fit. `:ls` names them all on the status line, marking the current one with `%`. LazyVim's `<leader>b` group is where the rest of it sits: `<leader>bn` and `<leader>bp` step through the list, `<leader>bb` goes back to the buffer last left (VIM's `:b#`), `<leader>bd` closes the current one, and `<leader>bo`, `<leader>bl` and `<leader>br` close the others — all of them, the ones to the left, or the ones to the right. Closing refuses to take unsaved changes with it, and a bulk close refuses the whole move rather than half of it, naming the buffer that stopped it (`E162`); `:bd!` is the one way to drop changes, since a chord has no `!` to add. Closing the last buffer open leaves an empty one behind.

- **Window splits** — `<leader>sh` puts two windows side by side, `<leader>sv` stacks them one above the other, and `:vsplit`/`:split` do the same from the prompt (`:sp other.txt` opens that file in the window it makes). The new window goes to the right or below and takes the cursor with it, showing the same buffer at the same place — VIM with `splitright` and `splitbelow` set, which is how LazyVim has them. Windows divide the room equally: splitting a column again gives three windows a third each rather than a half and two quarters, because a split in the direction a row or column already runs joins it rather than nesting inside it. Each window keeps its own cursor, its own viewport and the buffer it shows, so the same file can be open twice at two different places, and the buffer list, the search pattern and the registers stay shared across all of them. `Ctrl-H`/`Ctrl-J`/`Ctrl-K`/`Ctrl-L` move to the window that way — the nearest one sharing any of the rows or columns the move crosses — and are all it takes to leave a window; inside Edit mode they are left to typing, where `Ctrl-H` is the Backspace some terminals send. `<leader>wd` closes the window (`:close`, or `:q`, which quits once it was the last one) and `:only` leaves the one being worked in. A window is drawn with its own gutter and its own scrolling inside its rectangle, with a `│` or `─` in the theme's colour along the join; a split with no room for two windows of a usable size is refused (`E36`) rather than drawn unreadably, and the last window cannot be closed (`E444`).

- **Status bar** — one line at the foot of the screen for the window being worked in: its mode, file name, line count, modified/saved state, whether the register and the undo/redo stacks hold anything, the count being typed, and cursor row/column. The prompt takes the line over while a search or a `:` command is being typed, and what a command has to report — a write, a pattern that matched nothing, a refused quit — is shown there.
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
| `v` | Normal | start a selection of runes; `Esc` drops it |
| `V` | Normal | start a selection of whole lines |
| `o` | Visual | swap the end of the selection the cursor is on |
| `d` `x` | Visual | delete the selection |
| `y` | Visual | yank the selection |
| `c` | Visual | delete the selection and start typing where it was |
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
| `:e` | Normal | edit another file (`:e name`); refuses to drop unsaved changes, `:e!` overrides |
| `Tab` | Normal | switch to the next buffer, wrapping at the end |
| `H` `L` | Normal | switch to the previous / next buffer |
| `:ls` | Normal | name the open buffers on the status line |
| `:bn` `:bp` | Normal | switch to the next / previous buffer |
| `<leader>bd` | Normal | close the buffer, refusing unsaved changes |
| `<leader>bn` `<leader>bp` | Normal | switch to the next / previous buffer |
| `<leader>bb` | Normal | switch back to the buffer last left |
| `<leader>bo` | Normal | close every other buffer |
| `<leader>bl` `<leader>br` | Normal | close the buffers to the left / right |
| `:bd` | Normal | close the buffer, with `:bd!` to drop unsaved changes |
| `<leader>sh` `<leader>sv` | Normal | split the window side by side / one above the other |
| `Ctrl-H` `Ctrl-J` `Ctrl-K` `Ctrl-L` | Normal | move to the window that way |
| `<leader>wd` | Normal | close the window |
| `:vs` `:sp` | Normal | split the window (`:vs name` opens that file in it) |
| `:clo` `:on` | Normal | close the window / every other window |
| `<leader>e` | Normal | open the file explorer on the current file's directory |
| `j` `k` | Explorer | move down / up the listing |
| `Enter` `l` | Explorer | descend into the directory, or open the file |
| `h` `-` | Explorer | step out to the parent directory |
| `g` `G` | Explorer | jump to the first / last entry |
| `Ctrl-D` `Ctrl-U` | Explorer | move the selection down / up half a screen |
| `/` | Explorer | narrow the listing to the names matching what is typed |
| `H` | Explorer | show or hide dotfiles |
| `q` | Explorer | return to the buffer |
| `Ctrl-H` `Ctrl-J` `Ctrl-K` `Ctrl-L` | Explorer | leave the explorer for the window left / below / above / right |
| `Esc` | Explorer | drop the filter, or return to the buffer when there is none |
| `<leader>e` | Explorer | close the explorer |
| `:theme=2` | Normal | switch palette (`1` default, `2` gruvbox, `3` github-dark; bare `:theme` names it) |
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
- The rest of Visual mode — blockwise `Ctrl-V`, `gv`, the text objects (`vi(`, `vip`) and the operators beyond `d`/`x`/`y`/`c` (`>`, `~`, `J`, `p` over a selection)
- Regular expressions in a search pattern — `/` matches literal text, so `\v`, `*`, character classes and `:s` are not there; nor are `ignorecase`/`smartcase`, `*` and `#` (search for the word under the cursor), or a search used as an operator's motion (`d/foo`)
- The rest of the operators and text objects (`cw`, `dj`, `di(`, ...) — only `x`, `dw`, `de`, `db`, `dd`, the `y` operators and what Visual mode selects for exist so far, and they stop at the line boundary instead of running onto the next line
- Named registers (`"a`) — there is only the unnamed one
- Counts in front of an operator's motion (`d3w`) — a count goes before the whole command (`3dw`)
- Marks and macros
