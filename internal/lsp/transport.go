package lsp

import "io"

// A Transport is where the frames go and come from. A language server's process
// is one; a pair of pipes a test drives both ends of is another, which is what
// lets the client be tested on a machine with no server installed.
type Transport interface {
	io.Reader
	io.Writer

	// CloseSend says there is nothing more coming, which is what a language
	// server takes as the end. Its output goes on being read until it stops.
	CloseSend() error

	// Close is what is left once it has: reaping the process, or killing one
	// that would not go.
	Close() error
}

// A Dial is how one is made, given the directory the server is to run in.
type Dial func(root string) (Transport, error)
