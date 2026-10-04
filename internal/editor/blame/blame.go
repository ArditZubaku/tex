// Package blame is who wrote the line under the cursor, shown in the same box
// a language server's hover answers in, and the patch behind it, run in a
// real pty beside the window so git's own pager renders it.
package blame

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ArditZubaku/tex/internal/editor/state"
	"github.com/ArditZubaku/tex/internal/editor/view"
	"github.com/ArditZubaku/tex/internal/git"
	"github.com/ArditZubaku/tex/internal/project"
)

// currentLine is the commit behind the cursor's own line, read off the file
// as last saved — the way 'git blame' itself always has, so an edit not yet
// written can leave it answering for the wrong line, the same gap every such
// plugin leaves open.
func currentLine(e *state.Editor) (root string, line git.Line, err error) {
	root = project.Root(e.SourceFile)
	line, err = git.Blame(root, e.SourceFile, e.Row+1)

	return root, line, err
}

// ShowLine is ' gb': git's own answer for who last touched the cursor's line
// and when.
func ShowLine(e *state.Editor) {
	_, line, err := currentLine(e)
	if err != nil {
		e.StatusMsg = "git blame: " + reason(err)

		return
	}

	e.Hov.Show(format(line), e.ScreenArea())
}

// ShowDiff is ' gs': the patch the commit behind the cursor's line made to
// this file — 'git show <hash> -- file' — run in a pty in a split to the
// right, so git's own configured pager renders and pages it exactly as it
// would typed by hand, colour included.
func ShowDiff(e *state.Editor) {
	root, line, err := currentLine(e)
	if err != nil {
		e.StatusMsg = "git blame: " + reason(err)

		return
	}
	if line.Hash == git.Uncommitted {
		e.StatusMsg = "this line has not been committed yet"

		return
	}

	view.OpenGitShow(e, root, line.Hash, e.SourceFile)
}

func format(l git.Line) string {
	hash := l.Hash
	if len(hash) > 7 {
		hash = hash[:7]
	}

	text := fmt.Sprintf("%s %s, %s", hash, l.Author, relative(l.When))
	if l.Summary != "" {
		text += " — " + l.Summary
	}

	return text
}

func relative(t time.Time) string {
	d := max(time.Since(t), 0)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d/(24*time.Hour)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d/(30*24*time.Hour)))
	default:
		return fmt.Sprintf("%dy ago", int(d/(365*24*time.Hour)))
	}
}

func reason(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		first, _, _ := bytes.Cut(exitErr.Stderr, []byte("\n"))

		return strings.TrimPrefix(strings.TrimSpace(string(first)), "fatal: ")
	}

	return err.Error()
}
