package main

import (
	"os"

	"github.com/ArditZubaku/txi/internal/editor"
)

func main() {
	editor.Run(os.Args[1:])
}
