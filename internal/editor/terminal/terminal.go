// Package terminal is a shell running behind the editor's own screen: a pty,
// a VT100/xterm emulator reading what comes back off it, and the snapshot a
// window draws from that. It knows nothing of windows or keys, only of a
// session and the cells it currently holds.
package terminal

import (
	"bufio"
	"os"
	"os/exec"
	"sync/atomic"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

// Session is one shell, its pty and the emulator parsing what it prints.
// Everything but Write and the two lifecycle methods locks the emulator for
// as long as it takes to read from it, since the reader goroutine is parsing
// into the same state at the same time.
type Session struct {
	pty  *os.File
	cmd  *exec.Cmd
	term vt10x.Terminal

	exited atomic.Bool
}

var wake func()

// Wake is how a session tells the editor's loop a frame is owed, exactly as
// internal/lsp's own Wake does: nothing here may draw, so it only asks.
func Wake(ask func()) { wake = ask }

// Start is a shell sized to cols by rows, $SHELL if it says one and /bin/sh
// otherwise.
func Start(cols, rows int) (*Session, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	return start(exec.Command(shell), cols, rows)
}

func start(cmd *exec.Cmd, cols, rows int) (*Session, error) {
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")

	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, err
	}

	s := &Session{
		pty: f, cmd: cmd,
		// WithWriter is not for the shell's output, only for what the emulator
		// itself replies to the shell with — a cursor-position report, an OSC
		// colour query — without which a program asking for either would hang.
		term: vt10x.New(vt10x.WithSize(cols, rows), vt10x.WithWriter(f)),
	}
	go s.readLoop()

	return s, nil
}

// readLoop is the only place Parse is called, and Parse is the only thing
// that ever blocks on the pty, so this is the one goroutine that owns reading
// it. It ends the same way whether the shell exited on its own or Kill closed
// the pty out from under it: either way Parse returns an error, and Wait is
// called exactly once, here, so the child is reaped without a second path.
func (s *Session) readLoop() {
	r := bufio.NewReader(s.pty)
	for {
		if err := s.term.Parse(r); err != nil {
			_ = s.cmd.Wait()
			s.exited.Store(true)
			if wake != nil {
				wake()
			}

			return
		}
		if wake != nil {
			wake()
		}
	}
}

func (s *Session) Write(p []byte) {
	_, _ = s.pty.Write(p)
}

// Resize is the pty's own size and the emulator's idea of it kept together;
// vt10x's Resize already locks itself, so this must not lock around it too.
func (s *Session) Resize(cols, rows int) {
	_ = pty.Setsize(s.pty, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	s.term.Resize(cols, rows)
}

func (s *Session) Exited() bool { return s.exited.Load() }

func (s *Session) Kill() {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.pty.Close()
}

// Snapshot is every cell of the terminal as it stands this frame, locked for
// exactly as long as the copy out of vt10x's own state takes.
func (s *Session) Snapshot() [][]Cell {
	s.term.Lock()
	defer s.term.Unlock()

	cols, rows := s.term.Size()
	out := make([][]Cell, rows)
	for y := range rows {
		row := make([]Cell, cols)
		for x := range cols {
			row[x] = cellOf(s.term.Cell(x, y))
		}
		out[y] = row
	}

	return out
}

func (s *Session) Cursor() (col, row int, visible bool) {
	s.term.Lock()
	defer s.term.Unlock()

	c := s.term.Cursor()

	return c.X, c.Y, s.term.CursorVisible()
}

func (s *Session) Size() (cols, rows int) {
	s.term.Lock()
	defer s.term.Unlock()

	return s.term.Size()
}

// AppCursor is whether the shell has asked for DECCKM, the cursor keys
// sending their SS3 form rather than CSI — vim does, a plain prompt does not.
func (s *Session) AppCursor() bool {
	s.term.Lock()
	defer s.term.Unlock()

	return s.term.Mode()&vt10x.ModeAppCursor != 0
}
