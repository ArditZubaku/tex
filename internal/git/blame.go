// Package git is the subprocess calls txi makes of the git binary on PATH:
// who last touched a given line, and the patch behind that.
package git

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// cmdTimeout bounds git the way project.List does: the key that asks for
// this blocks the whole redraw loop, and a hung process must not hang it
// forever.
const cmdTimeout = 5 * time.Second

// Uncommitted is the Hash a Line comes back with when the line has no commit
// behind it yet — git's own sentinel, forty zeros, for its synthetic "Not
// Committed Yet" author.
const Uncommitted = "0000000000000000000000000000000000000000"

// A Line is what 'git blame' knows about one line of one file: the commit
// that put it there, who made it, when, and that commit's own summary.
type Line struct {
	Hash    string
	Author  string
	When    time.Time
	Summary string
}

// Blame is everything 'git blame' knows about line (1-indexed) of file, as
// seen from root. A line with no commit behind it yet comes back the same
// way 'git blame' itself reports it — Author "Not Committed Yet", Hash
// Uncommitted — not as an error.
func Blame(root, file string, line int) (Line, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	at := strconv.Itoa(line)
	cmd := exec.CommandContext(ctx, "git", "-C", root, "blame", "-L", at+","+at, "--line-porcelain", "--", file)

	out, err := cmd.Output()
	if err != nil {
		return Line{}, err
	}

	return parsePorcelain(out)
}

func parsePorcelain(out []byte) (Line, error) {
	lines := strings.Split(string(out), "\n")
	fields := strings.Fields(lines[0])
	if len(fields) == 0 {
		return Line{}, errors.New("git blame: no output")
	}

	result := Line{Hash: fields[0]}
	for _, l := range lines[1:] {
		switch {
		case strings.HasPrefix(l, "author "):
			result.Author = strings.TrimPrefix(l, "author ")
		case strings.HasPrefix(l, "author-time "):
			result.When = parseUnix(strings.TrimPrefix(l, "author-time "))
		case strings.HasPrefix(l, "summary "):
			result.Summary = strings.TrimPrefix(l, "summary ")
		case strings.HasPrefix(l, "\t"):
			return result, nil
		}
	}

	return result, nil
}

func parseUnix(s string) time.Time {
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}

	return time.Unix(sec, 0)
}
