package main

import (
	"os"

	"github.com/ArditZubaku/tex/internal/editor"
)

func main() {
	editor.Run(os.Args[1:])
}
