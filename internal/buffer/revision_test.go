package buffer

import "testing"

func TestEveryMutatorCountsAsAChange(t *testing.T) {
	mutate := map[string]func(*Buffer){
		"SetLine":     func(b *Buffer) { b.SetLine(0, []rune("x")) },
		"InsertRune":  func(b *Buffer) { b.InsertRune(0, 0, 'x') },
		"DeleteRunes": func(b *Buffer) { b.DeleteRunes(0, 0, 1) },
		"InsertLine":  func(b *Buffer) { b.InsertLine(1) },
		"SplitLine":   func(b *Buffer) { b.SplitLine(0, 1) },
		"JoinLine":    func(b *Buffer) { b.JoinLine(0) },
		"DeleteLine":  func(b *Buffer) { b.DeleteLine(0) },
	}

	for name, change := range mutate {
		t.Run(name, func(t *testing.T) {
			b := Open(writeTemp(t, "abc\ndef\n"))
			defer b.Close()

			before := b.Revision()
			change(b)

			if b.Revision() <= before {
				t.Errorf("Revision() = %d after %s, want more than %d", b.Revision(), name, before)
			}
		})
	}
}

func TestReadingTheBufferIsNotAChange(t *testing.T) {
	b := Open(writeTemp(t, "abc\ndef\n"))
	defer b.Close()

	b.SetLine(0, []rune("xyz"))
	after := b.Revision()

	b.Line(1)
	b.Raw(1)
	b.RuneLen(0)
	b.LineInto(1, nil)
	b.Rune(0, 0)
	b.LineCount()

	if b.Revision() != after {
		t.Errorf("Revision() = %d after reading, want %d", b.Revision(), after)
	}
}

func TestATooShortMutationCountsAsNothing(t *testing.T) {
	b := Open(writeTemp(t, "abc\n"))
	defer b.Close()

	before := b.Revision()

	b.SetLine(9, []rune("x"))
	b.DeleteRunes(0, 2, 1)
	b.DeleteLine(-1)
	b.JoinLine(0)

	if b.Revision() != before {
		t.Errorf("Revision() = %d, want %d: nothing changed", b.Revision(), before)
	}
}

// A count that started again from nothing would read as a copy of the text
// that is already current, leaving whatever holds one stale for as long as the
// buffer is open.
func TestTheChangeCountSurvivesASaveAndAReload(t *testing.T) {
	path := writeTemp(t, "abc\ndef\n")
	b := Open(path)
	defer b.Close()

	b.SetLine(0, []rune("xyz"))
	edited := b.Revision()

	if err := b.Save(path); err != nil {
		t.Fatal(err)
	}
	saved := b.Revision()
	if saved <= edited {
		t.Fatalf("Revision() = %d after a save, want more than %d", saved, edited)
	}

	b.Reload(path)
	if b.Revision() <= saved {
		t.Errorf("Revision() = %d after a reload, want more than %d", b.Revision(), saved)
	}
}

func TestTheChangeCountSurvivesASaveThatCouldNotReopenTheFile(t *testing.T) {
	path := writeTemp(t, "abc\n")
	b := Open(path)
	defer b.Close()

	b.DeleteRunes(0, 0, 3) // an empty file is the branch reopen hands to Reload
	edited := b.Revision()

	if err := b.Save(path); err != nil {
		t.Fatal(err)
	}

	if b.Revision() <= edited {
		t.Errorf("Revision() = %d, want more than %d", b.Revision(), edited)
	}
}
