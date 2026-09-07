// Package lsp is a language server as a process and a protocol: finding it,
// starting it, framing JSON-RPC over its input and output, and the subset of
// the protocol the editor actually uses. It knows nothing of the editor — what
// it hands back is rows, columns and text.
//
// The editor has one goroutine and a great deal of package-level scratch that
// assumes it. What follows is what keeps that true, and is meant to be checked
// against rather than admired:
//
//  1. Nothing off the loop's goroutine touches the editor's own state or a
//     *buffer.Buffer. A buffer is not merely unsynchronised: reading a line
//     moves its window and its decoded-line cache.
//  2. The loop's goroutine marshals every message before handing it over, so
//     what crosses to the writer is bytes nobody else holds.
//  3. The reader's goroutine owns each message until it sends it and never
//     afterwards. Params and Result are left raw, so that decoding one happens
//     on the loop's side.
//  4. The writer's goroutine owns the process and its input; the reader owns
//     its output. Neither touches the other's half, which is why the writer
//     waits on the reader before it reaps.
//  5. Waking the loop is the only thing a goroutine off it does to the editor,
//     and all that is is a non-blocking send on a channel of capacity one.
//  6. Sending to the server never blocks: a server that is not reading is a
//     server that is going, and the frame is dropped. Every sender is therefore
//     a reconciler, which asks again on the next frame.
//  7. Nothing here may log. The editor installs no slog handler, so a log line
//     goes to the terminal that termbox owns and is drawn over the file.
package lsp
