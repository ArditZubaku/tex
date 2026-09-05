// Package register is VIM's unnamed register: whatever was last yanked or
// deleted, held either as whole lines (yy, dd, V) or as a run of runes (yw, x,
// v), which spans more than one line only when a Visual selection did.
package register

import "slices"

type Register struct {
	lines    [][]rune
	linewise bool
}

func Linewise(lines [][]rune) Register { return Register{lines: lines, linewise: true} }
func Charwise(lines [][]rune) Register { return Register{lines: lines} }

func (r Register) Empty() bool       { return len(r.lines) == 0 }
func (r Register) IsLinewise() bool  { return r.linewise }
func (r Register) Content() [][]rune { return r.lines }

// Repeated is a counted put of a charwise register: the copies run into each
// other, so putting a two-line register twice leaves three lines, not four.
func (r Register) Repeated(n int) [][]rune {
	if n <= 1 {
		return r.lines
	}

	out := make([][]rune, 0, (len(r.lines)-1)*n+1)
	for range n {
		if len(out) == 0 {
			out = append(out, slices.Clone(r.lines[0]))
		} else {
			out[len(out)-1] = append(out[len(out)-1], r.lines[0]...)
		}
		for _, line := range r.lines[1:] {
			out = append(out, slices.Clone(line))
		}
	}

	return out
}
