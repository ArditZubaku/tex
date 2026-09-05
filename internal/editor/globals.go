package editor

import "github.com/ArditZubaku/tex/internal/editor/state"

// ed is the editor being run. Every command below works on it rather than on
// state of its own, which is what lets each of them live in the package it
// belongs to.
var ed = state.New()
