package complete

import (
	"strconv"
	"testing"
)

// Every keystroke against an open menu narrows the candidates already in hand,
// which is what keeps a keystroke from being a request. It has to cost nothing.
func BenchmarkRetype(b *testing.B) {
	items := make([]Item, 0, 300)
	for i := range 300 {
		items = append(items, Item{
			Label:  "Println" + strconv.Itoa(i),
			Detail: "func(a ...any) (n int, err error)",
			Text:   "Println" + strconv.Itoa(i),
		})
	}

	var m Menu
	m.Show(items, 0, 0, nil, false)
	query := []rune("prln")

	b.ReportAllocs()
	for range b.N {
		m.Retype(query)
	}
}
