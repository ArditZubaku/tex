// Package buffer is the file being edited as the editor holds it: an index of
// where every line starts, one window of raw bytes around the cursor, and an
// overlay of the lines that have been changed. The file itself is never loaded.
package buffer

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"unicode/utf8"
)

// Buffer reads a file from disk on demand, holding only an index of where each
// line starts plus one fixed-size window of raw bytes around the cursor.
type Buffer struct {
	file  *os.File
	size  int64
	count int

	starts          []int64 // byte offset of each line's first byte
	endsWithNewline bool

	// win holds raw bytes for lines [winFrom, winTo), starting at byte winBase.
	// Slices into it are invalidated by the next fill.
	win            []byte
	winBase        int64
	winFrom, winTo int
	cacheRow       int
	cacheLine      []rune
	cacheOK        bool

	overlay map[int][]rune // lines edited in Insert mode, shadowing the file
}

// WindowBytes is how much of the file is held in memory at once, which the
// tests size their fixtures against.
const (
	WindowBytes = 64 << 10
	windowBack  = 32 // lines kept behind the cursor, so scrolling back up rarely re-reads
	scanChunk   = 1 << 20
)

func NewEmpty() *Buffer {
	// starts stays parallel to count even with no file behind it, so inserting
	// and deleting lines needs no separate empty-buffer case. Every line of
	// such a buffer lives in the overlay, so the index is never read through.
	return &Buffer{
		count:   1,
		starts:  []int64{0},
		overlay: map[int][]rune{0: {}},
		winFrom: -1,
		winTo:   -1,
	}
}

func Open(name string) *Buffer {
	file, err := os.Open(name)
	if err != nil {
		slog.Error("Failed to open file", "error", err)
		return NewEmpty()
	}

	info, err := file.Stat()
	if err != nil {
		slog.Error("Failed to stat file", "path", name, "error", err)
		closeFile(file)
		return NewEmpty()
	}

	starts := buildIndex(file, info.Size())
	if len(starts) == 0 {
		closeFile(file)
		return NewEmpty()
	}

	last := make([]byte, 1)
	if _, err := file.ReadAt(last, info.Size()-1); err != nil {
		slog.Error("Failed to read final byte", "path", name, "error", err)
		closeFile(file)
		return NewEmpty()
	}

	return &Buffer{
		file:            file,
		size:            info.Size(),
		count:           len(starts),
		starts:          starts,
		endsWithNewline: last[0] == '\n',
		win:             make([]byte, 0, WindowBytes),
		winFrom:         -1,
		winTo:           -1,
		overlay:         make(map[int][]rune),
	}
}

func closeFile(file *os.File) {
	if err := file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		slog.Error("Failed to close file", "path", file.Name(), "error", err)
	}
}

// buildIndex records the offset of every line start, reading through one
// reusable buffer so the scan costs a fixed scanChunk rather than the file size.
// Splitting matches bufio.Scanner: '\n' terminates, a trailing '\n' adds no
// empty line, and a '\r' before it is stripped at read time.
func buildIndex(r io.ReaderAt, size int64) []int64 {
	if size == 0 {
		return nil
	}

	const guessedLineLen = 96

	starts := make([]int64, 1, size/guessedLineLen+1)
	scratch := make([]byte, scanChunk)

	for off := int64(0); off < size; {
		n, err := r.ReadAt(scratch, off)
		for i := 0; i < n; {
			j := bytes.IndexByte(scratch[i:n], '\n')
			if j < 0 {
				break
			}
			i += j + 1
			if off+int64(i) < size {
				starts = append(starts, off+int64(i))
			}
		}

		if n == 0 {
			break
		}
		off += int64(n)

		if err != nil {
			if !errors.Is(err, io.EOF) {
				slog.Error("Failed to index file", "offset", off, "error", err)
			}
			break
		}
	}

	return starts
}

// Save writes the buffer to path through a temporary file in the same
// directory and renames it over the target, so a failed write can never
// truncate the original. Unedited lines are copied as raw bytes straight out of
// the read window, so saving holds no more memory than scrolling does, whatever
// the file size. The buffer is then reopened against what was written, which
// drops the overlay and rebuilds the index.
func (b *Buffer) Save(path string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tex-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // a no-op once the rename lands

	if info, err := os.Stat(path); err == nil {
		if err := tmp.Chmod(info.Mode().Perm()); err != nil {
			slog.Error("Failed to carry over file permissions", "path", path, "error", err)
		}
	}

	// bufio's error is sticky, so it is enough to check it once at the Flush.
	w := bufio.NewWriterSize(tmp, WindowBytes)
	for i := range b.count {
		if line, ok := b.overlay[i]; ok {
			for _, ch := range line {
				_, _ = w.WriteRune(ch)
			}
		} else {
			_, _ = w.Write(b.rawLine(i))
		}
		if i < b.count-1 || b.endsWithNewline {
			_ = w.WriteByte('\n')
		}
	}

	if err := w.Flush(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}

	b.Reload(path)

	return nil
}

// Reload reads the file again from scratch, dropping the window, the index and
// every edit the overlay held. It is what a save needs, since the old handle
// still points at the file the rename replaced, and what a formatter rewriting
// the file underneath the buffer leaves it needing.
func (b *Buffer) Reload(path string) {
	b.Close()
	*b = *Open(path)
}

func (b *Buffer) Close() {
	if b == nil || b.file == nil {
		return
	}
	closeFile(b.file)
	b.file = nil
}

func (b *Buffer) LineCount() int {
	return b.count
}

// lineEnd is exclusive and omits the terminator.
func (b *Buffer) lineEnd(i int) int64 {
	if i+1 < b.count {
		return b.starts[i+1] - 1
	}
	if b.endsWithNewline {
		return b.size - 1
	}
	return b.size
}

func (b *Buffer) fillWindow(i int) {
	from := max(0, i-windowBack)
	base := b.starts[from]

	// The window must hold the requested line whole, however long that line is.
	length := min(int64(WindowBytes), b.size-base)
	if need := b.lineEnd(i) - base; need > length {
		length = need
	}

	oversized := int64(cap(b.win)) > 4*WindowBytes && length <= WindowBytes
	if int64(cap(b.win)) < length || oversized {
		b.win = make([]byte, length)
	}
	b.win = b.win[:length]

	n, err := b.file.ReadAt(b.win, base)
	if err != nil && !errors.Is(err, io.EOF) {
		slog.Error("Failed to read line window", "offset", base, "error", err)
	}
	b.win = b.win[:n]

	b.winBase, b.winFrom = base, from
	b.winTo = from
	for b.winTo < b.count && b.lineEnd(b.winTo) <= base+int64(n) {
		b.winTo++
	}
}

// rawLine aliases the window: the next fill invalidates the result.
func (b *Buffer) rawLine(i int) []byte {
	if i < b.winFrom || i >= b.winTo {
		b.fillWindow(i)
		if i < b.winFrom || i >= b.winTo {
			return nil
		}
	}

	return b.win[b.starts[i]-b.winBase : b.lineEnd(i)-b.winBase]
}

// raw drops the '\r' of a CRLF pair, which rawLine keeps so that saving an
// untouched line writes back the bytes it was read as.
func (b *Buffer) Raw(i int) []byte {
	line := b.rawLine(i)
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}

	return line
}

// Line 's result is owned by the Buffer; copy before modifying (see SetLine).
func (b *Buffer) Line(i int) []rune {
	if i < 0 || i >= b.count {
		return nil
	}
	if line, ok := b.overlay[i]; ok {
		return line
	}
	if b.cacheOK && b.cacheRow == i {
		return b.cacheLine
	}

	line := bytes.Runes(b.Raw(i))
	b.cacheRow, b.cacheLine, b.cacheOK = i, line, true

	return line
}

// LineInto is Line for a caller that reads one line after another and keeps
// none of them: the runes go into scratch, which is grown when it is too small
// and returned so that the next line reuses it. Nothing is cached, and an
// edited line is copied out rather than aliased, so what comes back is the
// caller's to read and the overlay's to keep.
func (b *Buffer) LineInto(i int, scratch []rune) []rune {
	if i < 0 || i >= b.count {
		return scratch[:0]
	}
	if line, ok := b.overlay[i]; ok {
		return append(scratch[:0], line...)
	}

	return appendRunes(scratch[:0], b.Raw(i))
}

// appendRunes is bytes.Runes into a buffer the caller owns. One byte cannot
// decode to more than one rune, so len(raw) is capacity enough for all of them.
func appendRunes(dst []rune, raw []byte) []rune {
	if cap(dst) < len(raw) {
		dst = make([]rune, len(raw))
	}
	dst = dst[:len(raw)]

	// almost every line is ASCII from end to end, and widening one is a loop
	// with nothing in it but a load and a store
	n := 0
	for ; n < len(raw); n++ {
		if raw[n] >= utf8.RuneSelf {
			break
		}
		dst[n] = rune(raw[n])
	}

	for i := n; i < len(raw); n++ {
		ch, size := utf8.DecodeRune(raw[i:])
		dst[n] = ch
		i += size
	}

	return dst[:n]
}

// editedLine reports the overlay's copy of line i, the one that shadows the
// file, so a caller can tell a line it has to read as runes from one it can
// still read as raw bytes.
func (b *Buffer) EditedLine(i int) ([]rune, bool) {
	line, ok := b.overlay[i]
	return line, ok
}

// RuneLen avoids decoding: for unedited lines it counts runes over the raw bytes.
func (b *Buffer) RuneLen(i int) int {
	if i < 0 || i >= b.count {
		return 0
	}
	if line, ok := b.overlay[i]; ok {
		return len(line)
	}
	if b.cacheOK && b.cacheRow == i {
		return len(b.cacheLine)
	}

	return utf8.RuneCount(b.Raw(i))
}

func (b *Buffer) Rune(i, col int) (rune, bool) {
	line := b.Line(i)
	if col < 0 || col >= len(line) {
		return 0, false
	}

	return line[col], true
}

func (b *Buffer) SetLine(i int, line []rune) {
	if i < 0 || i >= b.count {
		return
	}

	b.overlay[i] = line
	if b.cacheOK && b.cacheRow == i {
		b.cacheOK = false
	}
}

// InsertRune inserts ch at col in line i. The first insert into an unedited
// line has to copy it out of the read window, but from then on the line grows
// amortized in the overlay, so typing a run of characters into it does not
// reallocate on every keystroke.
func (b *Buffer) InsertRune(i, col int, ch rune) {
	line := b.Line(i)
	col = min(max(col, 0), len(line))

	if edited, ok := b.overlay[i]; ok {
		b.overlay[i] = slices.Insert(edited, col, ch)
		return
	}

	updated := make([]rune, len(line)+1)
	copy(updated, line[:col])
	updated[col] = ch
	copy(updated[col+1:], line[col:])
	b.SetLine(i, updated)
}

// DeleteRunes removes runes [from, to) from line i. An already-edited line is
// compacted in place; an unedited one is copied out at its new, shorter length,
// so a delete never allocates more than the line it shrinks.
func (b *Buffer) DeleteRunes(i, from, to int) {
	line := b.Line(i)
	from, to = max(from, 0), min(to, len(line))
	if from >= to {
		return
	}

	if edited, ok := b.overlay[i]; ok {
		b.overlay[i] = append(edited[:from], edited[to:]...)
		return
	}

	updated := make([]rune, len(line)-(to-from))
	copy(updated, line[:from])
	copy(updated[from:], line[to:])
	b.SetLine(i, updated)
}

// InsertLine adds an empty line at index i, pushing the lines from i down. The
// new line exists only in the overlay, but the index still needs an entry for
// it: it gets the offset the line now below it starts at, which leaves the
// extent of every neighbouring line exactly as it was. Nothing ever reads the
// file through that duplicated offset, since the overlay always shadows it.
func (b *Buffer) InsertLine(i int) {
	if i < 0 || i > b.count {
		return
	}
	b.cacheOK = false

	off := b.size
	switch {
	case i < b.count:
		off = b.starts[i]
	case !b.endsWithNewline:
		// the line above ends at size, not at a terminator before it
		off++
	}
	b.starts = slices.Insert(b.starts, i, off)
	b.count++

	shifted := make([]int, 0, len(b.overlay))
	for row := range b.overlay {
		if row >= i {
			shifted = append(shifted, row)
		}
	}
	// descending, so each line moves into a slot already vacated
	slices.SortFunc(shifted, func(a, b int) int { return b - a })
	for _, row := range shifted {
		b.overlay[row+1] = b.overlay[row]
		delete(b.overlay, row)
	}
	b.overlay[i] = nil

	// window line numbers no longer match the file after the shift
	b.winFrom, b.winTo = -1, -1
}

// SplitLine breaks line i at col, moving what follows onto a new line below.
// Only the tail is copied: the head keeps the array it already had, so typing
// on after a split does not start from a fresh allocation.
func (b *Buffer) SplitLine(i, col int) {
	if i < 0 || i >= b.count {
		return
	}

	line := b.Line(i)
	col = min(max(col, 0), len(line))
	tail := slices.Clone(line[col:])

	b.InsertLine(i + 1)
	b.overlay[i], b.overlay[i+1] = line[:col], tail
}

// JoinLine appends line i+1 to line i and drops it. The result has to live in
// the overlay: the two lines are contiguous on disk apart from the terminator
// between them, and the index cannot express a line with a hole in it.
func (b *Buffer) JoinLine(i int) {
	if i < 0 || i+1 >= b.count {
		return
	}

	line, ok := b.overlay[i]
	if !ok {
		line = slices.Clone(b.Line(i))
	}

	b.overlay[i] = append(line, b.Line(i+1)...)
	b.DeleteLine(i + 1)
}

// DeleteLine drops line i by compacting the index in place and re-keying the
// overlay, so nothing proportional to the rest of the file is rebuilt. The
// deleted bytes stay on disk, simply unreferenced by the index.
func (b *Buffer) DeleteLine(i int) {
	if i < 0 || i >= b.count {
		return
	}
	b.cacheOK = false

	// VIM leaves an empty line behind rather than an empty buffer
	if b.count == 1 {
		b.overlay[i] = b.overlay[i][:0]
		return
	}

	// A line ends where the next one starts, so dropping an index entry would
	// hand the deleted bytes to the line above it. Pin that line into the
	// overlay instead, where its content no longer depends on the index.
	if i > 0 {
		if _, ok := b.overlay[i-1]; !ok {
			b.overlay[i-1] = slices.Clone(b.Line(i - 1))
		}
	}

	b.starts = append(b.starts[:i], b.starts[i+1:]...)
	b.count--

	delete(b.overlay, i)
	if len(b.overlay) > 0 {
		shifted := make([]int, 0, len(b.overlay))
		for row := range b.overlay {
			if row > i {
				shifted = append(shifted, row)
			}
		}
		// ascending, so each line moves into a slot already vacated
		slices.Sort(shifted)
		for _, row := range shifted {
			b.overlay[row-1] = b.overlay[row]
			delete(b.overlay, row)
		}
	}

	// window line numbers no longer match the file after the shift
	b.winFrom, b.winTo = -1, -1
}
