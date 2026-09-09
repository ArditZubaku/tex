# tex

A small terminal text editor written in Go, built on [termbox-go](https://github.com/nsf/termbox-go). It's modal like VIM (separate Normal/Read, Insert/Edit and Visual modes) and currently implements a subset of VIM's motions and editing keys.

https://github.com/user-attachments/assets/620b49ad-a62f-47fb-b0f7-113c9e453f8e


## Usage

```sh
go build -o tex .
./tex path/to/file      # opens (or creates) a file
./tex                   # starts a new, unnamed buffer (out.txt)
```

### Ctrl-hjkl inside tmux

Anything that binds `Ctrl-h`/`j`/`k`/`l` to pane switching — `vim-tmux-navigator`
and the hand-rolled versions of it — takes those keys before tex ever sees them,
because the check it forwards on matches vim and nvim by name. Teaching it about
tex is one block in `~/.tmux.conf`, after `run '~/.tmux/plugins/tpm/tpm'` so that
it wins over the plugin's own bindings:

```tmux
is_vim="ps -o state=,comm= -t '#{pane_tty}' | awk '\$1 !~ /^[TXZ]/ {print \$2}' | grep -iqE '(^|/)(view|l?n?vim?x?|tex)(-wrapped)?(diff)?\$'"
bind -n C-h if-shell "$is_vim" "send-keys C-h" "select-pane -L"
bind -n C-j if-shell "$is_vim" "send-keys C-j" "select-pane -D"
bind -n C-k if-shell "$is_vim" "send-keys C-k" "select-pane -U"
bind -n C-l if-shell "$is_vim" "send-keys C-l" "select-pane -R"
```

It asks the pane's tty what is running on it rather than reading
`pane_current_command`, which says `make` when the editor was started through
`make run`, and skips stopped processes so a suspended editor gives the keys
back to tmux.

### Git hooks

The hooks live in `.githooks/` and are tracked, but git only looks there once a
clone has been told to:

```sh
make hooks      # git config core.hooksPath .githooks
```

`pre-push` runs `make lint` and refuses the push if it reports anything.

## Layout

What the editor is built out of are packages of their own, so that the
boundaries are the compiler's to keep rather than a convention. What anything
could use sits under `internal/`; what only the editor has any use for sits
under `internal/editor/`:

```
main.go                 hands the arguments to the editor and nothing else
internal/
  chars/                VIM's word/punct/space split
  theme/                the palettes, and every colour drawn in one
  buffer/               the file: line index, read window, edit overlay
  syntax/               the lexer that colours a line, and says which of it is code
  fuzzy/                the subsequence match the pickers narrow with
  gutter/               how wide the line numbers are, and what they read
  motion/               where w, e and b land
  search/               a literal pattern matched over a buffer
  decl/                 what a declaration looks like: gd, gr and the symbols
  project/              the tree the file sits in, and the files beside it
  lsp/                  a language server: the framing, the handshake, the documents, the lookups
  layout/               the window tree: splits, closes, the shares they divide and the edges between them
  editor/               the loop: draw a frame, read a key, repeat
    screen/             putting text on the terminal, and taking a key off it
    history/            the changes u and Ctrl-R step through
    register/           what was last yanked or deleted
    picker/             the popup: a list narrowed to what is typed
    filetree/           the directory listing <leader>e draws
    tabbar/             the buffer line along the top
    prompt/             the line of input / ? and : are typed on
    hover/              the box K's answer goes in, and its markup as plain text
    complete/           the menu of candidates under the word being typed
    state/              the editor being run: cursor, mode, buffer, room
    edit/               typing, deleting, yank and put, the Visual selection
    rename/             <leader>cr: a name, and how far it reaches
    view/               the buffer list and the window tree
    diag/               what a language server said about a line, and where it moved to
    command/            the ':' commands, and the file writing they share
    find/               search, gd, gr, K, completion, the symbols, the diagnostics, the project grep, the popup
    explorer/           <leader>e: the file tree as the editor drives it
    render/             one frame: windows, buffer line, status line
    keys/               which command each key, chord and count names
    edtest/             the harness the packages' own tests share
```

Nothing below `editor` imports it, and nothing imports `editor` but `main`, so
the dependencies run one way. Under `internal/`, `chars`, `theme`, `buffer`,
`gutter`, `project` and `layout` depend on nothing of the editor's, `syntax` on
`chars` and `theme`, `fuzzy` and `decl` on `chars`, `search` and `lsp` on
`buffer`, and `motion` on both. Under `editor/`, `screen`, `history`, `register`,
`picker`, `filetree`, `tabbar`, `prompt` and `hover` are leaves in the same way;
`state` holds them and is what every command below takes as its one argument —
`edit` first, then `view` on top of it, then `rename` and `diag`, with `command`,
`find` and `explorer` above them and `render` and `keys` reading all of them.

What is left in `editor` itself is the loop: it makes the one `state.Editor`,
draws a frame from it and hands the next key to `keys`. Every test lives in the
package whose behaviour it asserts, outside it where the shared harness would
otherwise close a cycle.

## Features

- **Modal editing** — a Read (Normal) mode for navigation, an Edit (Insert) mode for typing and a Visual mode for selecting, with the terminal cursor changing shape (block vs. blinking bar) depending on mode. The cursor sits *on* a character in Normal mode, stopping at the last one on the line like VIM does; only Insert mode reaches the column past it, where appending happens.
- **VIM-style navigation** — `hjkl`, word motions (`w`/`b`/`e`), line jumps (`I`/`A`), buffer jumps (`gg`/`G`); see the full list below.
- **Saving** — `:w` or `Ctrl-S`, in either mode. The buffer is streamed to a temporary file in the same directory and renamed over the target, so a failed write cannot truncate the original; untouched lines are copied as raw bytes, so a save costs no more memory than scrolling does. The index of where the new file's lines begin is built as the file is written, so the buffer left behind never reads back what it has just written. File permissions, CRLF line endings on untouched lines, and a missing trailing newline are all preserved.
- **Format on save** — a file written with `:w`, `:wq` or `Ctrl-S` is handed to whatever formats its language, and the result is read straight back into the buffer, so the formatting appears under the cursor rather than only on disk. Four languages so far, each taking the first of its formatters that is actually installed: **Go** (`gofumpt`, `goimports`, `gofmt`), **Rust** (`rustfmt`), and **JavaScript**/**TypeScript** (`prettier`, `biome`) — a JavaScript project's own `node_modules/.bin` is looked in before the `PATH`, since that is where `prettier` almost always is. A language with nothing installed for it, and a file of any other kind, is written exactly as it was. The formatter runs from inside the file's own directory, so `rustfmt.toml`, `.prettierrc` and the rest are found the way the tool's own CLI finds them.

  It costs a save what the formatter itself costs and nothing else: the lookup for a formatter on the `PATH` is made once and remembered, a file already formatted is left alone and not reread — `gofmt` over a 300-line file takes about **3ms**, which is under a frame either way — and only a file the formatter actually rewrote is read back, which the modification time says without reading either version. A formatter that hangs is given five seconds and then killed, since the editor's loop blocks on the keyboard, and a file too large for one to be worth waiting on (4MB) is written unformatted. **The write always lands first.** A formatter that refuses the file — while it is being typed into, almost always a syntax error rather than anything else — leaves what was written exactly where it is, and its complaint goes in a box in the top-right corner rather than onto the status line, which carries the write as usual:

  ```
   broken.go
  1   package main                                       ┌─ gofmt ───────────────────────────────────┐
    1                                                    │ broken.go:6:20: expected '}', found 'EOF' │
    2 import "fmt"                                       └───────────────────────────────────────────┘
    3
    4 func main() {
    5  fmt.Println("hi")
  ```

  The box is titled with the formatter that raised it, sized to the message and wrapped to fit, and comes down on `Esc`, on the next write, or once six seconds have passed — nothing wakes the editor, so it is the next keypress that takes an expired one off the screen, which leaves it up for as long as nobody is at the keyboard, which is when it is being read. A save is never lost to a half-finished line. Undo survives a reformat that only moved text within its lines, which is what most of them are; one that added or removed a line is the one case that drops the history, since replaying it would put lines back at rows that have moved. `:wa` formats every buffer it writes, on the same terms.
- **Deleting** — `x`, `dw`, `de`, `db` and `dd` in Normal mode, `Backspace` in Insert mode, all in place: a line delete compacts the line index rather than rebuilding it, and a character delete reuses the edited line's own backing array, so deleting never costs more memory than the text it removes.
- **Auto-pairs** — typing `(`, `[` or `{` in Insert mode writes the closing bracket after the cursor, so the pair is never left half-written. Typing the closer the editor already wrote steps over it rather than doubling it, and `Backspace` between an empty pair takes both brackets at once. A closer typed where there is nothing of its own kind to step over is inserted like any other character, which is what leaves unbalanced text still typeable.
- **Line structure** — `o`/`O` open a line, `Enter` splits one at the cursor and `Backspace` at column 0 joins it back. Each shifts the line index in place rather than rebuilding it, and a split copies only the tail: the half before the cursor keeps the array it already had.
- **Yank and put** — `yy`, `yw`, `ye` and `yb` copy into VIM's unnamed register, which the delete operators fill too, and `p`/`P` put it back after or before the cursor. A line yank puts whole lines below or above the cursor; a word yank puts the run of characters back into the line the cursor is on. Every one of them also reaches the system clipboard through `OSC 52`, the escape sequence a terminal reads it from directly rather than a tool like `pbcopy` that would need installing and cannot cross an SSH session; a terminal that does not understand it just discards the sequence. A linewise yank carries a trailing line break, a charwise one does not — pasting either outside the editor reads the same as pasting it back inside.
- **Visual mode** — `v` selects runes and `V` whole lines, from where the mode was entered to wherever a motion has taken the cursor since; both ends are part of the selection, as VIM's `selection=inclusive` has them. `d` (or `x`) deletes it, `y` yanks it, `c` changes it, and `o` swaps the end that the next motion drags along. `v` and `V` switch between the two shapes and leave the mode when pressed on the shape it already has, and `Esc` drops the selection. The selection is drawn as a band in the theme's own colour over the text, keeping the syntax colours under it, and it takes in the line break of every line it carries on past. A run that spans lines fills the register as a run — `p` splits the line it is put into and joins the register's ends onto the halves — so a selection can be moved from one place to another whatever it covers. `p` (or `Ctrl-V`) over a selection replaces it with the register instead of putting alongside it: the selection is removed the way `d` removes it, and what the register held before that goes in its place — the register itself is left holding the removed text, same trade VIM makes for it.
- **Undo and redo** — `u` and `Ctrl-R`, with an insert session (from `i` to `Esc`) undone in one step the way VIM does it. Nothing snapshots the buffer: a change remembers only the lines the command actually touched, so the undo history costs the text that was edited rather than the size of the file. The last 500 changes are kept.
- **Counts** — a command can be prefixed with a repeat count, as in `3j`, `3x`, `2dd` or `yy3p`. Commands where the count says *how much text* rather than *how many times* (`x`, `dd`, `yy`, `p`, `P`) act on that much text in a single step, so one `u` takes the whole thing back.
- **Word-class-aware word motions** — `w`/`b`/`e` classify runs of characters into whitespace / word (`[A-Za-z0-9_]`) / punctuation, so e.g. `"foo` is treated as two words (`"` then `foo`), matching VIM's default word boundaries.
- **Search** — `/` opens a prompt on the status line, `?` opens it searching backwards, and `Enter` jumps the cursor to the first rune of the match, wrapping around the end of the buffer like VIM does. `n` and `N` repeat it with or against its direction (`3n` for the third match), `Enter` on an empty prompt reuses the last pattern, and every match on screen is lit until `Esc` clears the highlight. Patterns are literal text — spaces included, so `/quick brown` is one pattern — and case-sensitive. The search never decodes the file: it matches `bytes.Index` over the raw bytes of each line as they come through the same 64KB read window everything else uses, decoding only the text in front of a hit to turn its byte offset into a column, so scanning a 12MB / 200k-line file end to end costs **~8ms** and no more memory than scrolling through it does.
- **Relative line numbers** — a gutter on the left showing each line's distance from the cursor, with the cursor's own line carrying its absolute number instead, like VIM's `number`/`relativenumber` pair. It widens with the buffer's line count (four columns until the count reaches four digits) and the text and horizontal scrolling start after it.
- **Cursor line highlight** — the line the cursor is on is drawn as a faint band across the width of its window, gutter included, like VIM's `cursorline`, with its line number in yellow. The theme picks the shade: grey 236 of the 256-colour palette by default, gruvbox's own `bg1` under theme 2 — dark enough to sit under the text rather than compete with it, which is what the editor asks the terminal for 256-colour output for.
- **Syntax highlighting** — nine token classes, coloured by the theme in use; the default palette groups them in families so the screen reads as a handful of colours rather than a dozen: magenta for the words the language reserves (keywords, and a lighter shade for literals like `true` and `nil`), yellow for the names of things (types, and escape sequences inside strings), cyan for what can be called (a name with a `(` after it, and a lighter shade for built-ins like `len` or `print`), green for strings, red for numbers and blue for comments. The language is picked by the file's name: Go, the C family (C/C++, C#, Java, JavaScript/TypeScript, Rust, Kotlin, Swift, PHP…), and everything whose comments start with `#`, from Python and the shells to YAML and `Makefile`. A file that matches no rule is drawn plain. Each line is lexed as it is drawn, so highlighting costs a screenful of text and nothing about it scales with the size of the file; block comments are the one construct that spans lines, and one that started above the window is found by lexing at most 64 lines back.
- **Viewport scrolling** — the visible window follows the cursor both vertically and horizontally as the buffer grows past the terminal size. `zz` recentres it on the cursor's line without moving the cursor, and `40zz` centres on line 40, jumping there first; near the end of the buffer the window is left hanging past the last line rather than pinned to it, the way VIM does it.
- **Ex commands** — `:` opens the same prompt the search does, for the commands a VIM user types without thinking: `:w` (and `:w other.txt`, which writes there and carries on editing that file, like `:saveas`), `:q`, `:wq`, `:x`, `:q!` to leave unsaved changes behind, `:e other.txt` to edit another file (`:e` on its own rereads the current one), `:bnext`/`:bprev`/`:bdelete`/`:ls` for the buffer list, `:split`/`:vsplit`/`:close`/`:only` for windows, `:nohlsearch` to drop the search highlight, `:rename` for the identifier under the cursor, `:wa` to write every buffer that has unsaved changes, `:qa` to quit however many windows are open, and a bare line number — `:42`, `:$` — to jump there. `:q` closes the window it is typed in and quits once that was the last one. Anything unrecognised is reported (`E492`) instead of guessed at.
- **Themes** — `:theme=2` switches the whole palette, `:theme=1` goes back, and `:theme` on its own names what is in use. Three so far: **1** the editor's own colours, the only one that leaves the terminal's background alone; **2** [gruvbox](https://github.com/morhetz/gruvbox) — red keywords, yellow types, green strings and bold green functions, purple literals, aqua built-ins, grey comments over its `bg0`; **3** GitHub's dark default (the one the editor starts in) — red keywords, green type names, purple functions, blue literals, light-blue strings over `#0d1117`. Both are the nearest 256-colour index to each hex value the palette itself publishes. Every colour the editor draws in, syntax and interface alike, comes out of the theme in use, so a fourth one is a value in `internal/theme` and nothing else.
- **Quitting asks about unsaved changes** — `q`, `:q` on the last window and `:qa` all go through one confirmation: every buffer holding changes that are not on disk is named on the status line — `save changes to a.go, b.go? [y]es/[n]o/[c]ancel` — and the editor goes only once that has been answered. It is the whole buffer list that is asked about, not the file on screen: a file left behind by `Tab`, by the picker or by `<leader>cr` holds its changes just the same, and three names is as many as the line has room for, so the rest are counted (`and 2 more`). `y` writes all of them, `n` gives them up, and `c` — or `Esc`, or anything else typed — stays where it was with the changes intact. A write that fails keeps the editor open and reports why (`E212`), since the changes it could not put on disk are still the only copy. `:q!` and `:qa!` skip the question and drop the changes, which is what the `!` has always meant.

- **File explorer** — `<leader>e` (Space, then `e`) lists the directory the file being edited lives in, over the window it is opened in the way `netrw` does rather than in a sidebar — the other windows of a split carry on showing their buffers. Directories come first and carry a trailing `/`, `..` sits on top, and `hjkl` move the way they do in the buffer: `j`/`k` walk the listing, `Enter` or `l` descends into a directory or opens a file, `h` or `-` steps back out — landing on the directory just left rather than at the top — `g`/`G` jump to the ends and `Ctrl-D`/`Ctrl-U` move half a screen at a time, as they do in the buffer. `a` asks for a name on the status line and makes it in the directory being listed — a trailing `/` makes a directory, anything else a file, and a path (`internal/editor/keys.go`) makes the directories on the way to it as well, then moves the listing to whichever directory holds what was made with the new entry selected; a name that already exists is left alone. `yy` picks the file under the selection up and `p` writes it out again — asking what to call the copy, on the name it already has, so that pasting it back into its own directory is a rename and pasting it into another is an `Enter` — and `d` deletes what is selected, but only once the confirmation it puts on the status line has been answered with a `y`. A directory is not copied at all and is deleted only when it is empty; the yanked file survives moving to another directory, which is what makes `yy`, `h`/`l` and `p` a copy across the tree. `H` shows the dotfiles it hides by default, `q` returns to the buffer, `Ctrl-H`/`J`/`K`/`L` leave it for the window that way, and a second `<leader>e` closes the explorer the same way the first opened it. `/` hands the status line to the same prompt the buffer's search uses and narrows the listing to the names matching what is typed, case-insensitively, as it is typed: `Enter` settles on that listing, `Esc` puts the whole directory back, and a second `Esc` — with nothing left to drop — closes the explorer. Moving to another directory drops the filter with it; toggling dotfiles keeps it. Opening a file adds it to the buffer list and edits it, leaving the buffer it was opened from — unsaved changes and all — a `Tab` away.

- **Buffers** — every file opened stays open, the way VIM's hidden buffers and LazyVim's buffer line do: `:e` and the explorer put the new file *beside* the one being edited rather than over it, and `Tab` walks the list, wrapping at the end. `H` and `L` are LazyVim's own step back and forward through it. A buffer keeps what belongs to it — its cursor, its viewport, its unsaved changes and its undo history — so switching away and back leaves the file exactly as it was, and editing a file already in the list switches to it rather than opening it twice. The list is drawn as a row of tabs across the top of the window, the buffer being edited picked out in the theme's own colours and an unsaved one carrying a dot; the row scrolls sideways to keep the current tab on screen when more buffers are open than fit. `:ls` names them all on the status line, marking the current one with `%`. LazyVim's `<leader>b` group is where the rest of it sits: `<leader>bn` and `<leader>bp` step through the list, `<leader>bb` goes back to the buffer last left (VIM's `:b#`), `<leader>bd` closes the current one, and `<leader>bo`, `<leader>bl` and `<leader>br` close the others — all of them, the ones to the left, or the ones to the right. Closing refuses to take unsaved changes with it, and a bulk close refuses the whole move rather than half of it, naming the buffer that stopped it (`E162`); `:bd!` is the one way to drop changes, since a chord has no `!` to add. Closing the last buffer open leaves an empty one behind.

- **Window splits** — `<leader>sh` puts two windows side by side, `<leader>sv` stacks them one above the other, and `:vsplit`/`:split` do the same from the prompt (`:sp other.txt` opens that file in the window it makes). The new window goes to the right or below and takes the cursor with it, showing the same buffer at the same place — VIM with `splitright` and `splitbelow` set, which is how LazyVim has them. Windows divide the room equally: splitting a column again gives three windows a third each rather than a half and two quarters, because a split in the direction a row or column already runs joins it rather than nesting inside it. What each window keeps is its share of the room rather than a size in rows, so a terminal resized under a layout hands every window the same proportion of the new screen it had of the old one. **The line between two windows is dragged with the mouse**: pressing on it takes hold of it, moving with the button down moves it, and letting go leaves it there; the line of a split nested inside a column moves that split alone, and one dragged as far as the window beside it can give stops there rather than being refused. What is followed is the edge rather than the pointer, so a pointer dragged past that point leaves the edge where it stopped rather than owing it the difference on the way back. Asking the terminal for the mouse at all is asking for all of it, so a plain drag no longer selects text: holding `Shift` while dragging is what every terminal worth the name leaves its own selection on. Each window keeps its own cursor, its own viewport and the buffer it shows, so the same file can be open twice at two different places, and the buffer list, the search pattern and the registers stay shared across all of them. `Ctrl-H`/`Ctrl-J`/`Ctrl-K`/`Ctrl-L` move to the window that way — the nearest one sharing any of the rows or columns the move crosses — and are all it takes to leave a window; inside Edit mode they are left to typing, where `Ctrl-H` is the Backspace some terminals send (a multiplexer of its own may want [teaching about tex](#ctrl-hjkl-inside-tmux) first). `<leader>wd` closes the window (`:close`, or `:q`, which quits once it was the last one) and `:only` leaves the one being worked in. A window is drawn with its own gutter and its own scrolling inside its rectangle, with a `│` or `─` in the theme's colour along the join; a split with no room for two windows of a usable size is refused (`E36`) rather than drawn unreadably, and the last window cannot be closed (`E444`).

- **File picker** — `<leader><leader>` opens a popup over the middle of the screen listing every file under the project root — the repository the file being edited sits in, or its own directory when it is in none — with hidden directories left out, since `.git` alone holds more files than the tree being worked on. What is typed narrows the listing as a fuzzy match rather than a prefix, so `bfg` finds `buffers.go`, and what matched is ranked the way a picker is usually meant: the letters together, at the start of a word, and in the name rather than the directories leading to it. `Ctrl-N`/`Ctrl-P` (or the arrow keys) walk the listing, `Enter` opens the file settled on as a buffer, and `Esc` — or a backspace with nothing left to delete — closes the popup, leaving the buffer underneath untouched.

- **Project search** — `<leader>/` asks for a pattern on the status line and runs [ripgrep](https://github.com/BurntSushi/ripgrep) over the project root for it, listing every match in the same popup the file picker and `gr` use: `file:line: the line itself`, narrowed further by what is typed and opened with `Enter`. The pattern is ripgrep's own regex rather than the buffer's own literal `/`, case-insensitive unless it carries a capital itself, and `.gitignore` and hidden files stay out of it the same as the file picker leaves them out by hand. **There is no fallback yet**: a machine with no `rg` on its `PATH`, or a pattern ripgrep refuses, is told so on the status line rather than left waiting or answered some slower way.

- **Language server** — optional, and nothing about the editor needs one. A server found on the `PATH` — `gopls` for a Go file with a `go.mod` above it, `rust-analyzer` under a `Cargo.toml`, `typescript-language-server` under a `tsconfig.json`, `jsconfig.json` or `package.json` — is started once for the project the file sits in, spoken to over JSON-RPC on its own standard input, handed the documents that are open, and shut down with the editor. There is one server per language rather than one in all, so a Go backend and the TypeScript frontend beside it both have theirs; each is pinned to the root it was started in, which is what stops a `gd` into the standard library spinning up a second copy of the same language server on it. What it answers is what `gd` and `gr`, `<leader>ss` and `<leader>sS`, `K`, the completion menu and the diagnostics use, each described below. With no server installed every one of them falls back to reading the text, which is what they all did before there was a server at all, and a server that dies is reported once and not started again.

- **Symbols** — `<leader>ss` is LazyVim's document symbols: the declarations of the file being edited, listed in the picker's own popup in the order they appear, each with the kind it is. With a language server running they are the ones it knows about and the kinds are its own, which is a parser's answer rather than a pattern's. With none they are read off the text the way `gd` reads a declaration, in the shapes they take across the languages the editor highlights — `Function`, `Method`, `Struct`, `Interface`, `Class`, `Enum`, `Trait`, `Type`, `Constant`, `Variable` — so a Go method carries the receiver it hangs off (`Buffer.Line`) and a `def` indented inside a `class` is listed as readily as one at the margin. Go's `var (` and `const (` blocks are followed into, since that is where a Go file keeps most of its globals and each name on a line of one — `ROWS, COLS int` — is its own symbol. What is typed narrows the listing as a fuzzy match over both the kind and the name, and `Enter` goes to the name itself rather than the head of its line, which counts as a jump so `Ctrl-O` comes back from it. A declaration has to start its line, past whatever the language lets stand in front of one (`export`, `pub`, `async`, `static`…): that is what keeps a list of a language's own keywords inside a string literal from reading as a screenful of declarations. It is still text and not a parser: a line of a multi-line string that happens to open with `const` is read as one.

  `<leader>sS` is the same listing widened to the project — from the server when there is one, and otherwise the file being edited first, then the files of the same kind beside it, under the same root the file picker walks and with the same limits `gd` and `gr` keep to (hidden directories left out, nothing larger than a megabyte). Every row carries the file and line it was found on, so what is typed narrows on the name and the file alike, and `Enter` opens that file as a buffer before going to the symbol. This repository lists 770 symbols across its 30-odd Go files in about 50ms from the text, since the walk is the same one the picker already makes.

  Project-wide symbols are the one place the two do not fit each other: the protocol's own are query-driven, where the popup lists everything and narrows locally. So the server is asked for the whole project with an empty query — `gopls` answers that in full — and a server that will not is a server the text listing answers for, which is the same listing as before.

- **Go to definition and references** — `gd` on an identifier jumps to where it is declared. A language server knows which `count` of the four in the file is the one under the cursor, and is asked when there is one; what follows is what answers when there is not. VIM's own `gd` is a local declaration, and so is the text's: the search goes *up* from the cursor to the start of the top-level construct it sits in and takes the nearest line declaring the name — the parameter it was passed as, the local it was assigned from — rather than a field of the same name three hundred lines below. A mention reached through a `.` is not one of them, so `b.file` never answers for `file`. Failing that the whole file is searched in the shapes a declaration takes across the languages the editor highlights — `func`/`fn`/`def`/`function`, `type`/`class`/`struct`/`interface`, a parameter list, `var`/`let`/`const`, then a `:=` or `=` binding — strongest form first, with the plain first mention of the word as the last resort. Failing that too the files beside it of the same kind are read (under the project root, hidden directories left out, nothing larger than a megabyte) and the one holding it is opened as a buffer. `Ctrl-O` goes back to where the jump left from, across files as well, and the row is centred when the jump lands off screen. Nothing found says so (`E388`), as does a cursor on no identifier at all (`E349`). `gr` is the other half of it: every mention of the identifier — from the server when there is one, and otherwise the file being edited first, then the ones of the same kind beside it — listed in the picker's own popup as `file:line: the line itself`, narrowed by what is typed and opened with `Enter`, which counts as a jump so `Ctrl-O` comes back from it too. A server tells the two apart that the text cannot: `gr` on a method finds the calls to *that* method rather than every line in the project with the same word in it.

  **A lookup never holds the keyboard.** The request goes out, the editor carries on, and the answer lands a few frames later; nothing is drawn in between, since a warm server answers in tens of milliseconds and a message that appears for that long is a flicker rather than news. An answer to a lookup already given up on — the cursor has moved, or another `gd` has been pressed since — is dropped rather than acted on, because acting on it would move the cursor over whatever was started instead. And a server that finds nothing, refuses, or never answers at all leaves the text to answer: a hung one degrades to the paragraphs above after two seconds rather than to nothing at all.

- **Completion** — a menu of what a language server offers for the word being typed, under the word itself. It opens on its own once three characters of a word are down, and on the characters the server asked to be woken on (`.` for Go); `Ctrl-N` asks for it outright, and says so when there is nothing to ask. `Ctrl-N`/`Ctrl-P` and the arrows move through it, `Enter`, `Tab` or `Ctrl-Y` settles on a candidate, `Ctrl-E` or `Esc` takes it down without leaving Insert mode. A pasted line ending is not an accept: with no bracketed-paste mode the only thing telling a paste from typing is that a pasted Unix newline arrives as `Ctrl-J` where the `Enter` key itself arrives as a CR, so `Ctrl-J` goes on splitting the line the way it always has. Candidates are listed in the server's own ranking — which knows that a field of the receiver beats a package of the same first letter — with the signature or type that explains each one to the right of it, cut before the name is when the menu is narrow.

  A candidate replaces exactly what the server says it replaces, which is how a `.` typed in TypeScript is replaced along with the member chosen after it while a Go `.` is not, and the edits it names elsewhere in the file go in with it. Those are what an import is, and all three languages do it: `gopls` offers `Println` in a file that has not imported `fmt` and adds the `import "fmt"`, opening a block around a single import already there if that is what it takes; `rust-analyzer` adds the `use std::any::Any;`; `typescript-language-server` adds the `import { computeTotal } from "./helper";`. The cursor follows whatever those pushed down or along. It is one undo.

  The last two hold those edits back until they know which candidate was settled on — a server answering a thousand candidates cannot work out an import line for every one of them — so they are asked a second question about the one chosen. That question goes out *after* the name is already in the buffer, since a round trip with the keyboard held is a round trip felt: the import lands a frame or two behind the name. An answer that arrives once the file has gained or lost a line is dropped, because the rows it names have moved and an import written into the wrong one is worse than no import at all.

  **A keystroke is not a request.** The server is asked when a word becomes worth asking about and when a trigger character is typed; everything after that narrows the answer already in hand, locally, at about 110µs and zero allocations over two thousand candidates. The narrowing is over everything the server sent rather than a shortlist of it, and it has to be: `typescript-language-server` answers a bare prefix with a thousand candidates and ranks the ones needing an import *last*, so a list cut to the best few hundred would be a list with every auto-import cut out of it. A server is asked again only once it has said its own answer was cut short *and* a fifth of a second has passed — the same throttle the documents go out under, since a completion sends the document before it asks and asking oftener would pay for the same send twice. A word the server had nothing for is not asked about again, and a language whose server offers no completion at all is never asked in the first place.

- **Hover** — `K` on an identifier is what a language server knows about it, in a box beside the cursor: the signature, the type, and the doc comment above the declaration. It is VIM's own `K` with the server where the man page used to be, and there is nothing behind it in the text — a doc comment read off the file is what `gd` is for, and `gd` goes to the file rather than quoting it back at you, so with no server running `K` says so and leaves it there.

  The box goes *under* the cursor's line, since one drawn over it would hide the thing being asked about, and above it when there is no room below; it is sized to the answer and pulled left to stay on the screen. What the server sends is markup, and a terminal has no bold and no headings to draw it with, so the fences come off and what is left is the text: a fenced signature keeps the line breaks it was written with — it is code, and rewrapping it across a comma reads worse than cutting it — while the prose is joined into a paragraph and wrapped to the box's own width. An answer longer than the box is cut rather than scrolled. It comes down on the next key pressed, whatever that key is, the way the status line's own message does; nothing about it is waited for, so the box appears a frame or two after `K` and the keyboard is never held.

- **Rename** — `<leader>cr` on an identifier is LazyVim's rename with the same text behind it rather than a language server: the `:` line opens on `:rename target` with the name already typed, so it is edited into the new one rather than retyped, and `Enter` renames it as far as it reaches. What a rename has to get right is *which* mentions are the same thing rather than the same spelling, and that is read off the declaration:

  - a **local** — a parameter, or anything declared indented inside a construct — reaches the block it was declared in and no further, so renaming a `count` inside one function leaves the `count` in the next one alone;
  - a **package-level** name reaches its own package bare, and the rest of the project only through its package name: renaming `Run` in `command.go` rewrites `command.Run` wherever it is called and leaves `editor.Run` — and every other unrelated `Run` — exactly as it was. A Go `_test` package beside it counts as the rest of the project, since that is how it refers to what it tests;
  - a name reached **through a value** (`b.file`) reaches its own package either way, bare and qualified, since nothing in the text says which values are of the type that holds it.

  The package a file belongs to is what it says it belongs to (`package command`, wherever its doc comment ends), or the directory it sits in for the languages that have no such line. Mentions are whole words, so renaming `file` leaves `filename` and `myfile` alone, and only the ones in code count: the same lexer that colours the screen says which columns are comment or string, and prose that merely names `editor.Run` is left saying what it said. The two exceptions are the ones that would otherwise be wrong or surprising — the doc comment directly above the declaration itself, which is renamed with it (`// Run is the editor:` becomes `// Start is the editor:`), and the mention the cursor is on, which is renamed wherever it sits, so `<leader>cr` works from a name in a comment as readily as from the code. It is still text and not a parser: an import alias is not followed.

  **Nothing is written.** A file the rename reached joins the buffer list with its changes unsaved — the way `:e` and `gd` put a file there — and only if it actually changed, so it can be read through with `Tab`, put back with its own `u` (one undo step per file), and written when it is right: `:w` for one, `:wa` for all of them at once. The cursor never leaves the file the rename was asked for in, and stays on the name it was on, wherever a shorter or longer name has moved it along the line. The status line reports how far it went — `renamed target to handle: 9 changes in 4 files, none written (:wa)`, `2 changes on 2 lines` when it never left the buffer, or `3 changes, local to this block`. A new name that is not an identifier is refused (`E474`), as is a rename with no name to give (`E471`) or no identifier under the cursor (`E349`). `:rename handle` is the same command typed out, which is what the chord is a shortcut for.

- **Diagnostics from a language server** — the errors and warnings a server publishes, drawn under the text. The server is spoken to over JSON-RPC on its own standard input the way any editor speaks to one: found on the `PATH` the same way a formatter is, started once for the project the file being edited sits in, handed the document, and shut down with the editor. **Nothing is required.** With no server installed there are no diagnostics and nothing else about the editor changes — `gd`, `gr`, the symbol lists and `<leader>cr` go on reading the text, which is what they have always done. Go, Rust, TypeScript and JavaScript are what is sent so far. Another language is one entry in one table — the program to run and what it wants on its command line, the files that mark the root of a project of that kind, and what the protocol calls the language — and nothing above that entry is language-specific, the completion trigger characters included, since those are the server's own rather than a list here.

  A diagnostic is drawn as an underline in the severity's own colour — red for an error, yellow for a warning, blue for a hint — over the text as it already is, so the syntax colours stay legible under it, and the cursor line's band and a visual selection still win, since those are things being done rather than things being reported. The line number in the gutter is recoloured to match, which is what makes a diagnostic below the window findable; nothing widens, so there is no sign column and the text does not shift along when an error appears. A span across lines underlines from its start column on the first row, whole rows in the middle and up to the end column on the last; a range of no width takes the single rune it points at; and a span longer than eight rows — a syntax error the server could not place, reported over the rest of the file — draws only its first eight, since underlining a screenful says nothing.

  The status line carries the count, `[2E 1W]`, in the run of flags beside `[Copy]` and `[Undo]`, read off where the cursor is each frame rather than stored. The worst message on the line the cursor is on goes in the box in the top-right corner instead, titled with its severity — refreshed every frame, so it lasts exactly as long as the cursor sits on the line and is gone the moment it leaves — and it never interrupts a box a formatter's refusal or a dead server has just raised, picking back up once that one's own six seconds run out.

  `]d` and `[d` walk the file's diagnostics forward and back, wrapping at the ends the way `n` and `N` do — and like them they are a move rather than a jump, so `Ctrl-O` is left meaning where you last came from and the window is not recentred out from under you. `:diag` lists all of them in the picker's own popup, worst first and then by file, line and column, each row marked `E`/`W`/`I`/`H` with the file, the line and the message; what is typed narrows the listing, and `Enter` opens that file and goes to the diagnostic. A message that runs over several lines is put back onto one, since a picker row and a status line each have one.

  **A diagnostic that has stopped being true does not have to be redrawn to be right.** The server reports against a version of the document, and the document goes on being typed into while it thinks. So an underline moves with the line it is pinned to: a line inserted above it shifts it down, a line deleted above it shifts it up, and the notes on a deleted line go with it. The one case that drops them is the line being typed on, where the columns could have moved anywhere — an underline three runes out is worse than none, and it is the one line whose errors are already known about. An undo, a redo or a formatter having been over the file drops that file's notes: too much moved to follow, and the server's next answer is about 200ms away.

  Which is the other half of it: **the diagnostic appears without a key being pressed.** The editor's loop blocks on the keyboard, so a server answering into an idle editor has to interrupt it, which is what the one goroutine parked in the terminal library is for; the document itself is sent no more often than every 200ms, however fast the typing is. It is streamed to the server through the same 64KB window everything else reads through rather than being loaded, and a file over a megabyte is not sent at all — the editor's own memory is what the section below is about, and handing a document over is not the place to give it up. What the server writes to its own standard error goes to a file in the temporary directory: the terminal belongs to the editor, and anything drawn on it by something else lands over the file being edited.

  **The one cost that is real is the server's own memory, and it is not small.** The 8.8MB a 23MB file costs is unchanged by any of this — every number in [Memory model](#memory-model) still holds, because the editor still never loads the file. A language server is another matter: on a real module `gopls` holds the type information for every package it loads, which is **300MB to 1GB**, and `rust-analyzer` is no cheaper — no amount of care on this side changes that. One server runs per language and each is pinned to the root it was started in, so a session touching two Go modules, or following a `gd` into the standard library, still pays for one of them rather than three. So it is a trade rather than a free feature, and one made per project and only when the binary is there: what the editor costs is what it costs, and what a language server costs is what a language server costs. `gopls -remote=auto` shares one of them across every editor open on the machine, which is the right eventual answer to it.

  A server that dies is reported once in the box in the corner, its underlines come off, and it is not started again — a language server restarting itself in a loop against a project it cannot load is worse than not having one. Everything the editor does from the text goes on working, which is the state it is in anyway when `gopls` was never there to begin with.

- **Status bar** — one line at the foot of the screen for the window being worked in: its mode, file name, line count, modified/saved state, whether the register and the undo/redo stacks hold anything, the count being typed, and cursor row/column. The prompt takes the line over while a search or a `:` command is being typed, and what a command has to report — a write, a pattern that matched nothing, a cancelled quit — is shown there.
- **Constant-memory file loading** — the file is never held in memory. Opening it builds an index of where each line starts (8 bytes per line) and nothing else; lines are read through one fixed 64KB window and decoded to runes only when they're on screen or under the cursor. Opening a 23MB file of 202,000 lines and jumping to the end costs **8.8MB of RSS**, and that figure doesn't move however far you scroll — 5.2MB of it is the Go runtime floor a one-line file also pays, so the file itself accounts for 2.6MB. See [Memory model](#memory-model).

## VIM motions implemented

| Key | Mode | Action |
| ----- | ------ | -------- |
| `h` `j` `k` `l` | Normal | move left / down / up / right, stopping at the ends of the line |
| `w` | Normal | jump to the start of the next word |
| `b` | Normal | jump to the start of the previous word |
| `e` | Normal | jump to the end of the (next) word |
| `$` | Normal | jump to the end of the line |
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
| `p` `Ctrl-V` | Visual | replace the selection with the register, leaving the register holding what was replaced |
| `/` | Normal | open the search prompt; `Enter` jumps to the next match, `Esc` cancels |
| `?` | Normal | the same, searching backwards |
| `n` | Normal | jump to the next match in the search's direction |
| `N` | Normal | jump to the next match against it |
| `:w` | Normal | write the buffer, formatting it first (`:w name` writes there and keeps editing it) |
| `q` `:q` | Normal | quit, asking about unsaved changes first |
| `:qa` | Normal | the same, however many windows are open |
| `:q!` `:qa!` | Normal | quit, dropping unsaved changes |
| `:wq` `:x` | Normal | write and format, then quit |
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
| drag a `│` or `─` | Normal | move the line between two windows with the mouse |
| `:vs` `:sp` | Normal | split the window (`:vs name` opens that file in it) |
| `:clo` `:on` | Normal | close the window / every other window |
| `gd` | Normal | jump to where the identifier under the cursor is declared, asking a language server first |
| `gr` | Normal | list every mention of the identifier under the cursor, asking a language server first |
| `<leader>cr` | Normal | rename the identifier under the cursor as far as it reaches |
| `:rename` | Normal | the same, typed out (`:rename handle`) |
| `:wa` | Normal | write and format every buffer with unsaved changes |
| `K` | Normal | show what a language server knows about the identifier under the cursor |
| `Ctrl-N` | Insert | open the completion menu on the word being typed, and step down it once it is up |
| `Ctrl-P` | Insert | step up the completion menu |
| `Enter` `Tab` `Ctrl-Y` | Insert | settle on the candidate the completion menu has selected |
| `Ctrl-E` `Esc` | Insert | take the completion menu down, staying in Insert mode |
| `Ctrl-O` | Normal | go back to where the last jump left from |
| `<leader><leader>` | Normal | open the file picker on the project root |
| `<leader>/` | Normal | search the project with ripgrep, listing matches in the same popup |
| `<leader>ss` | Normal | list the declarations of the file being edited |
| `<leader>sS` | Normal | list the declarations of the whole project, or of every file of the same kind beside it |
| `]d` `[d` | Normal | jump to the next / previous diagnostic in the file, wrapping at the ends |
| `:diag` | Normal | list every diagnostic a language server has published, worst first |
| any letter | Picker | narrow the listing to the files matching what is typed |
| `Ctrl-N` `Ctrl-P` | Picker | move down / up the listing |
| `Enter` | Picker | open the file settled on |
| `Esc` | Picker | close the picker |
| `<leader>e` | Normal | open the file explorer on the current file's directory |
| `j` `k` | Explorer | move down / up the listing |
| `Enter` `l` | Explorer | descend into the directory, or open the file |
| `h` `-` | Explorer | step out to the parent directory |
| `g` `G` | Explorer | jump to the first / last entry |
| `Ctrl-D` `Ctrl-U` | Explorer | move the selection down / up half a screen |
| `/` | Explorer | narrow the listing to the names matching what is typed |
| `a` | Explorer | create a file, a directory (trailing `/`), or a path of them |
| `yy` | Explorer | pick the file under the selection up for `p` |
| `p` | Explorer | copy the yanked file in, asking what to call it |
| `d` | Explorer | delete the entry, once the confirmation is answered `y` |
| `H` | Explorer | show or hide dotfiles |
| `q` | Explorer | return to the buffer |
| `Ctrl-H` `Ctrl-J` `Ctrl-K` `Ctrl-L` | Explorer | leave the explorer for the window left / below / above / right |
| `Esc` | Explorer | drop the filter, or return to the buffer when there is none |
| `<leader>e` | Explorer | close the explorer |
| `:theme=2` | Normal | switch palette (`1` editor's own, `2` gruvbox, `3` github-dark, default; bare `:theme` names it) |
| `u` | Normal | undo the last change |
| `Ctrl-R` | Normal | redo the last undone change |
| `1`–`9` | Normal | start a count for the next command, e.g. `3p` |
| `(` `[` `{` | Insert | insert the bracket and its closing pair, leaving the cursor between them |
| `)` `]` `}` | Insert | step over the closing bracket when it is the one already under the cursor, insert it otherwise |
| `Backspace` | Insert | delete the character before the cursor, taking the closing bracket too when it sits in an empty pair, and joining onto the line above at column 0 |
| `Tab` | Insert | insert one space (with a completion menu up it settles on a candidate instead) |
| `Enter` | Insert | split the line at the cursor (in Normal mode it moves down a line) |
| `Ctrl-J` | Insert | the same as `Enter` — with no bracketed-paste mode, a pasted line ending arrives as `Enter`'s CR, this LF, or both together, and all three split the line exactly once |
| `gg` | Normal | jump to the top of the buffer |
| `G` | Normal | jump to the bottom of the buffer |
| `I` | Normal | jump to start of line and enter Insert mode |
| `A` | Normal | jump to end of line and enter Insert mode |
| `o` | Normal | open an empty line below and enter Insert mode |
| `O` | Normal | open an empty line above and enter Insert mode |
| `i` | Normal | enter Insert mode before the cursor |
| `a` | Normal | enter Insert mode after the cursor |
| `Ctrl-S` | Either | save to the file that was opened, formatting it first |
| `zz` | Normal | redraw with the cursor's line in the middle of the window, keeping the column (`[count]zz` centres on that line) |
| `Ctrl-U` | Either | scroll up half a screen |
| `Ctrl-D` | Either | scroll down half a screen |
| `Ctrl-V` | Normal | the same as `p` |
| `Esc` | Insert | return to Normal mode (cursor steps back a column, VIM-style), taking down the error box with it |

Arrow keys, `Home`, `End`, `PgUp`, and `PgDn` also work in either mode. Left and
right stop at the ends of the line in Normal mode; in Insert mode they wrap onto
the neighbouring line, so typing can run off one line onto the next.

## Memory model

The file is never loaded. It stays on disk with its handle open, and `internal/buffer`
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
an overlay that shadows the file, so an edit is never lost when the window moves.
Lifting a line into that overlay copies it once; every keystroke after that grows or
shrinks it in place, so typing a word costs no allocation per character.

The overlay is a list in row order rather than a map. Reading a line finds its entry
by binary search, and inserting or deleting one renumbers the entries below it by
walking a run of them — where a map made that an allocation, a sort and two map
operations for every line already edited, so an `Enter` cost more the longer the
session had gone on.

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

Drawing a frame holds nothing per line either. Every visible line is decoded into one
array that outlives the frame and coloured into one more, and both are reused by every
row of every redraw. The block-comment state above the window is settled by looking
back a bounded number of lines, and a line whose raw bytes cannot hold the delimiter
that would change that state is skipped without being decoded at all — which is most
of them. A screenful of a 200,000-line file is drawn with no allocation per line of
text: what is left is the status bar and the buffer line, a handful of short strings.

### Goals not yet implemented

- The rest of `:` command mode — ranges (`:1,5d`), `:s`, `:r`, `:set` and the like; the write and quit family (`:w`, `:q`, `:qa`, `:wq`, `:x`, `:q!`), `:e`, the buffer commands (`:bn`, `:bp`, `:bd`, `:ls`), the window commands (`:sp`, `:vs`, `:clo`, `:on`), `:noh`, `:theme` and a bare line address are all that is there
- The rest of Visual mode — blockwise selection (its own key now that `Ctrl-V` pastes), `gv`, the text objects (`vi(`, `vip`) and the operators beyond `d`/`x`/`y`/`c`/`p` (`>`, `~`, `J`)
- Regular expressions in a search pattern — `/` matches literal text, so `\v`, `*`, character classes and `:s` are not there; nor are `ignorecase`/`smartcase`, `*` and `#` (search for the word under the cursor), or a search used as an operator's motion (`d/foo`)
- The rest of the operators and text objects (`cw`, `dj`, `di(`, ...) — only `x`, `dw`, `de`, `db`, `dd`, the `y` operators and what Visual mode selects for exist so far, and they stop at the line boundary instead of running onto the next line
- Named registers (`"a`) — there is only the unnamed one
- Counts in front of an operator's motion (`d3w`) — a count goes before the whole command (`3dw`)
- Marks and macros
