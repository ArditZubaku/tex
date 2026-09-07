// Package layout is the tree of rows and columns the windows are laid out in:
// where a split goes, what is left when one closes, how the room divides
// between them, and the rectangle each window is handed to draw in.
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

// A Tree is a leaf holding one window, or a node dividing its rectangle between
// its children — side by side when it is vertical, stacked when it is not.
// Splitting in the direction a node already runs adds a child to it rather than
// nesting under it, which is what makes a third split a third of the room
// rather than a quarter.
//
// Each child carries the share of the room it is owed rather than a size in
// rows or columns: a fresh split divides it equally, dragging the edge between
// two of them moves some of one child's share to its neighbour, and a terminal
// resized under all of them keeps every proportion it finds.
type Tree[T comparable] struct {
	leaf     T
	node     bool
	vertical bool
	children []*Tree[T]
	weights  []int
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

		return &Tree[T]{
			node: true, vertical: vertical,
			children: []*Tree[T]{t, Leaf(fresh)},
			weights:  []int{1, 1},
		}
	}

	for i, child := range t.children {
		if child.node || child.leaf != target {
			t.children[i] = child.InsertBeside(target, vertical, fresh)
			continue
		}
		if t.vertical == vertical {
			t.children = slices.Insert(t.children, i+1, Leaf(fresh))
			t.weights = slices.Insert(t.weights, i+1, 1)
			t.equalize()
			return t
		}
		t.children[i] = &Tree[T]{
			node: true, vertical: vertical,
			children: []*Tree[T]{child, Leaf(fresh)},
			weights:  []int{1, 1},
		}

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

	kept, weights := t.children[:0], t.weights[:0]
	for i, child := range t.children {
		pruned := child.Prune(target)
		if pruned == nil {
			continue
		}
		kept = append(kept, pruned)
		weights = append(weights, t.weights[i])
	}
	t.children, t.weights = kept, weights

	switch len(t.children) {
	case 0:
		return nil
	case 1:
		return t.children[0]
	}
	t.reduce()

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

	last := len(t.children) - 1
	for i, child := range t.children {
		rect := t.childRect(within, i)
		seps = child.Place(rect, seps, give)
		if i == last {
			break
		}
		if t.vertical {
			seps = append(seps, Separator{Row: within.Row, Col: rect.Col + rect.Cols, Length: within.Rows, Vertical: true})
			continue
		}
		seps = append(seps, Separator{Row: rect.Row + rect.Rows, Col: within.Col, Length: within.Cols})
	}

	return seps
}

// childRect is the room the ith child is owed, with a column (or a row) between
// each pair given up to the separator drawn there. The shares are measured from
// the near edge rather than one after another, so that they divide the room
// whole however little of it there is to go round.
func (t *Tree[T]) childRect(within Rect, i int) Rect {
	room := within.Rows
	if t.vertical {
		room = within.Cols
	}
	room -= len(t.children) - 1

	var cum, total int
	for j, weight := range t.weights {
		if j < i {
			cum += weight
		}
		total += weight
	}

	start := upTo(room, total, cum)
	size := upTo(room, total, cum+t.weights[i]) - start
	if t.vertical {
		return Rect{within.Row, within.Col + start + i, within.Rows, size}
	}

	return Rect{within.Row + start + i, within.Col, size, within.Cols}
}

// upTo rounds up, so that what is left over when the room does not divide
// evenly goes to the windows nearest the top or the left, as VIM's does.
func upTo(room, total, cum int) int {
	return (room*cum + total - 1) / total
}

// MoveEdge drags the separator drawn at a cell along the axis it divides, and
// says how far it went: the pointer may have run past what the windows either
// side of it can give, and a drag holds on to where the edge actually is rather
// than to where the pointer was.
func (t *Tree[T]) MoveEdge(within Rect, row, col, delta, least int) int {
	node, room, i, ok := t.edgeAt(within, row, col)
	if !ok {
		return 0
	}

	return node.shift(room, node.vertical, i, i+1, delta, least)
}

// EdgeAt says whether the cell is a separator between two windows, and which
// way it runs — what a press has to know before it can be the start of a drag.
func (t *Tree[T]) EdgeAt(within Rect, row, col int) (bool, bool) {
	node, _, _, ok := t.edgeAt(within, row, col)
	if !ok {
		return false, false
	}

	return node.vertical, true
}

// shift moves delta of the room from one child to another, clamped to what
// leaves both of them usable, and hands back what it actually moved. The shares
// are set from the sizes the children are drawn at rather than adjusted, so
// that a move of one row moves the edge exactly one row.
func (t *Tree[T]) shift(room Rect, vertical bool, grow, give, delta, least int) int {
	if grow < 0 || give < 0 || grow >= len(t.children) || give >= len(t.children) {
		return 0
	}

	sizes := make([]int, len(t.children))
	for i := range t.children {
		rect := t.childRect(room, i)
		sizes[i] = rect.Rows
		if vertical {
			sizes[i] = rect.Cols
		}
	}

	delta = max(least-sizes[grow], min(delta, sizes[give]-least))
	if delta == 0 {
		return 0
	}

	copy(t.weights, sizes)
	t.weights[grow] += delta
	t.weights[give] -= delta
	t.reduce()

	return delta
}

// edgeAt finds the node whose separator is drawn at a cell, with the room it
// divides and the child on the near side of that separator. A cell inside a
// window is no edge at all, and neither is one outside the area entirely.
func (t *Tree[T]) edgeAt(within Rect, row, col int) (*Tree[T], Rect, int, bool) {
	if t == nil || !t.node {
		return nil, Rect{}, 0, false
	}

	for i, child := range t.children {
		rect := t.childRect(within, i)
		if node, room, at, ok := child.edgeAt(rect, row, col); ok {
			return node, room, at, true
		}
		if i == len(t.children)-1 {
			break
		}
		if t.vertical && col == rect.Col+rect.Cols && row >= within.Row && row < within.Row+within.Rows {
			return t, within, i, true
		}
		if !t.vertical && row == rect.Row+rect.Rows && col >= within.Col && col < within.Col+within.Cols {
			return t, within, i, true
		}
	}

	return nil, Rect{}, 0, false
}

// equalize is what a split does to the row or column it lands in: VIM divides
// it equally again rather than halving the window that was split, which is what
// makes a third split a third of the room rather than a quarter.
func (t *Tree[T]) equalize() {
	for i := range t.weights {
		t.weights[i] = 1
	}
}

// reduce keeps the shares as small as they can be said in, since nothing but
// their ratio is ever read and a window closing leaves them adding up to less
// than the room they divide.
func (t *Tree[T]) reduce() {
	divisor := 0
	for _, weight := range t.weights {
		divisor = gcd(divisor, weight)
	}
	if divisor < 2 {
		return
	}

	for i := range t.weights {
		t.weights[i] /= divisor
	}
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}

	return a
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
