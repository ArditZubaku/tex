// Package layout is the tree of rows and columns the windows are laid out in:
// where a split goes, what is left when one closes, and the rectangle each
// window is handed to draw in.
package layout

import "slices"

// Rect is the part of the screen one window has to itself.
type Rect struct {
	Row, Col, Rows, Cols int
}

// A Separator is the line drawn between two windows, kept from the placing pass
// so that drawing them is one walk over the gaps rather than a second one over
// the tree.
type Separator struct {
	Row, Col, Length int
	Vertical         bool
}

// A Tree is a leaf holding one window, or a node dividing its rectangle equally
// between its children — side by side when it is vertical, stacked when it is
// not. Splitting in the direction a node already runs adds a child to it rather
// than nesting under it, which is what makes a third split a third of the room
// rather than a quarter.
type Tree[T comparable] struct {
	leaf     T
	node     bool
	vertical bool
	children []*Tree[T]
}

func Leaf[T comparable](of T) *Tree[T] {
	return &Tree[T]{leaf: of}
}

// Leaves walks them in the order they lie on screen.
func (t *Tree[T]) Leaves(out []T) []T {
	if t == nil {
		return out
	}
	if !t.node {
		return append(out, t.leaf)
	}

	for _, child := range t.children {
		out = child.Leaves(out)
	}

	return out
}

func (t *Tree[T]) InsertBeside(target T, vertical bool, fresh T) *Tree[T] {
	if !t.node {
		if t.leaf != target {
			return t
		}

		return &Tree[T]{node: true, vertical: vertical, children: []*Tree[T]{t, Leaf(fresh)}}
	}

	for i, child := range t.children {
		if child.node || child.leaf != target {
			t.children[i] = child.InsertBeside(target, vertical, fresh)
			continue
		}
		if t.vertical == vertical {
			t.children = slices.Insert(t.children, i+1, Leaf(fresh))
			return t
		}
		t.children[i] = &Tree[T]{node: true, vertical: vertical, children: []*Tree[T]{child, Leaf(fresh)}}

		return t
	}

	return t
}

// Prune drops a leaf and collapses whatever nothing is left dividing.
func (t *Tree[T]) Prune(target T) *Tree[T] {
	if !t.node {
		if t.leaf == target {
			return nil
		}

		return t
	}

	kept := t.children[:0]
	for _, child := range t.children {
		if pruned := child.Prune(target); pruned != nil {
			kept = append(kept, pruned)
		}
	}
	t.children = kept

	switch len(t.children) {
	case 0:
		return nil
	case 1:
		return t.children[0]
	}

	return t
}

// Place hands every leaf its rectangle and appends the separators between them
// to seps, which the caller keeps between frames so that a layout pass costs no
// allocation of its own.
func (t *Tree[T]) Place(within Rect, seps []Separator, give func(T, Rect)) []Separator {
	if !t.node {
		give(t.leaf, within)
		return seps
	}

	count := len(t.children)
	if t.vertical {
		room := within.Cols - (count - 1) // a column between each pair carries the separator
		col := within.Col
		for i, child := range t.children {
			width := share(room, count, i)
			seps = child.Place(Rect{within.Row, col, within.Rows, width}, seps, give)
			col += width
			if i < count-1 {
				seps = append(seps, Separator{Row: within.Row, Col: col, Length: within.Rows, Vertical: true})
				col++
			}
		}

		return seps
	}

	room := within.Rows - (count - 1)
	row := within.Row
	for i, child := range t.children {
		height := share(room, count, i)
		seps = child.Place(Rect{row, within.Col, height, within.Cols}, seps, give)
		row += height
		if i < count-1 {
			seps = append(seps, Separator{Row: row, Col: within.Col, Length: within.Cols})
			row++
		}
	}

	return seps
}

// share hands the remainder to the first windows, so that the room divides
// whole however many are sharing it.
func share(room, count, i int) int {
	size := room / count
	if i < room%count {
		size++
	}

	return size
}

// Gap is how far another rectangle lies in the given direction, and whether it
// lies that way at all: one sharing no rows with this one is off to a side
// rather than above or below it, which is what keeps a move up from landing
// there.
func (r Rect) Gap(to Rect, dRow, dCol int) (int, bool) {
	var gap int
	switch {
	case dCol < 0:
		gap = r.Col - (to.Col + to.Cols)
	case dCol > 0:
		gap = to.Col - (r.Col + r.Cols)
	case dRow < 0:
		gap = r.Row - (to.Row + to.Rows)
	default:
		gap = to.Row - (r.Row + r.Rows)
	}

	if gap < 0 || !r.overlaps(to, dCol != 0) {
		return 0, false
	}

	return gap, true
}

func (r Rect) overlaps(other Rect, rows bool) bool {
	if rows {
		return other.Row < r.Row+r.Rows && r.Row < other.Row+other.Rows
	}

	return other.Col < r.Col+r.Cols && r.Col < other.Col+other.Cols
}
