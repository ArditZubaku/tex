package register

import "testing"

func TestTextJoinsCharwiseLinesWithNoTrailingBreak(t *testing.T) {
	r := Charwise([][]rune{[]rune("foo"), []rune("bar")})

	if got, want := r.Text(), "foo\nbar"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestTextAddsATrailingBreakForLinewise(t *testing.T) {
	r := Linewise([][]rune{[]rune("foo"), []rune("bar")})

	if got, want := r.Text(), "foo\nbar\n"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestTextOfAnEmptyLineYankedIsOneLineBreak(t *testing.T) {
	r := Linewise([][]rune{{}})

	if got, want := r.Text(), "\n"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestTextOfAnEmptyRegisterIsEmpty(t *testing.T) {
	if got := (Register{}).Text(); got != "" {
		t.Errorf("Text() = %q, want empty", got)
	}
}
