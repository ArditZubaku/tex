package lsp

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ErrNotInstalled is the server not being on the PATH, which is not a failure:
// it is the editor going on doing what it does without one.
var ErrNotInstalled = errors.New("lsp: server not installed")

// A server told to go is given this long before it is killed. It has one file
// to write and no work to finish, so anything longer is a hung process rather
// than a slow one.
const exitGrace = 2 * time.Second

// A process is a language server running, and the pipes it talks over.
type process struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out io.ReadCloser
	log *os.File
}

// Server is the Dial that starts a real language server, given the program to
// run it as.
func Server(name string) Dial {
	return func(root string) (Transport, error) {
		bin := lookPath(name)
		if bin == "" {
			return nil, ErrNotInstalled
		}

		cmd := exec.Command(bin)
		cmd.Dir = root

		out, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		in, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}

		p := &process{cmd: cmd, in: in, out: out, log: stderrLog(name)}
		if p.log != nil {
			cmd.Stderr = p.log
		}

		if err := cmd.Start(); err != nil {
			p.closeLog()

			return nil, err
		}

		return p, nil
	}
}

func (p *process) Read(b []byte) (int, error)  { return p.out.Read(b) }
func (p *process) Write(b []byte) (int, error) { return p.in.Write(b) }

// CloseSend is what a language server takes as the end of its input, and
// therefore as the end.
func (p *process) CloseSend() error { return p.in.Close() }

// Close is what is left once it has been told to go: reaping it, or killing one
// that would not. It is called from the writer's goroutine, which is the only
// one that touches the process at all.
func (p *process) Close() error {
	done := make(chan struct{})
	go func() {
		_ = p.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(exitGrace):
		_ = p.cmd.Process.Kill()
		<-done
	}
	p.closeLog()

	return nil
}

func (p *process) closeLog() {
	if p.log != nil {
		_ = p.log.Close()
		p.log = nil
	}
}

// The terminal belongs to termbox: anything the server wrote to it would be
// drawn over the file being edited. Its complaints go to one file instead,
// which is the only place a crash of it can be read from afterwards. One file
// rather than one per run, since a session that ends well leaves nothing worth
// keeping.
func stderrLog(name string) *os.File {
	log, err := os.OpenFile(filepath.Join(os.TempDir(), "tex-"+name+".log"),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil
	}

	return log
}

// found is what the PATH was last seen to hold. A miss costs a stat per
// directory of the PATH, which is why the answer is kept — and which is why a
// server installed mid-session is picked up on the next run of the editor.
var found = map[string]string{}

func lookPath(name string) string {
	if bin, ok := found[name]; ok {
		return bin
	}

	bin, err := exec.LookPath(name)
	if err != nil {
		bin = ""
	}
	found[name] = bin

	return bin
}

// Installed says whether a server is there to be started at all, which is what
// lets a feature fall back to what the editor could always do without paying
// for a process it has no use for.
func Installed(name string) bool { return lookPath(name) != "" }
