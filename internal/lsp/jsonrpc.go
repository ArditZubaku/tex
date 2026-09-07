package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// A Message is any of the three things that come down the wire — a request, a
// response to one, or a notification — since which of them it is only shows in
// which fields arrived. Params and Result stay raw so that decoding either one
// happens on the goroutine that asked for it rather than on the reader's.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

// IsResponse is an answer to something asked from this end, which is told from a
// request made by the other end by there being no method on it.
func (m *Message) IsResponse() bool { return m.Method == "" && len(m.ID) > 0 }

// IsRequest is the other end asking for something, and expecting a reply: one
// left waiting on a reply it was promised stops making progress.
func (m *Message) IsRequest() bool { return m.Method != "" && len(m.ID) > 0 }

type ResponseError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *ResponseError) Error() string {
	return strconv.Itoa(e.Code) + ": " + e.Message
}

// The codes the editor sends back. A request it has no answer for gets
// MethodNotFound rather than silence, which is what lets the server carry on
// without whatever it asked for.
const (
	CodeMethodNotFound = -32601
	CodeInternalError  = -32603
)

const version = "2.0"

// A frame past this is not something a language server meant to send, and
// believing the header would allocate whatever it claimed.
const maxFrame = 32 << 20

// ErrFrameTooLarge is a Content-Length no server would have written, which a
// caller has to tell from a read that simply ended.
var ErrFrameTooLarge = errors.New("lsp: frame too large")

const header = "Content-Length: "

// readFrame is one message off the wire: headers to a blank line, then exactly
// as many bytes as Content-Length promised. Only that header is read — the
// content type is the one other header the protocol defines and it has one
// legal value — but the rest are still skipped rather than tripped over.
func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if !strings.HasPrefix(line, header) {
			continue
		}

		length, err = strconv.Atoi(strings.TrimSpace(line[len(header):]))
		if err != nil {
			return nil, fmt.Errorf("lsp: bad Content-Length: %w", err)
		}
	}

	switch {
	case length < 0:
		return nil, errors.New("lsp: message with no Content-Length")
	case length > maxFrame:
		return nil, fmt.Errorf("%w: %d bytes", ErrFrameTooLarge, length)
	}

	frame := make([]byte, length)
	if _, err := io.ReadFull(r, frame); err != nil {
		return nil, err
	}

	return frame, nil
}

// writeFrame puts one already-marshalled message on the wire. It does not
// flush: a burst of them is one write to the pipe, and the writer's own loop
// flushes once it has nothing left to send.
func writeFrame(w *bufio.Writer, frame []byte) error {
	var head [len(header) + 24]byte
	out := append(head[:0], header...)
	out = strconv.AppendInt(out, int64(len(frame)), 10)
	out = append(out, "\r\n\r\n"...)

	if _, err := w.Write(out); err != nil {
		return err
	}
	_, err := w.Write(frame)

	return err
}

// request is what goes out. It is built here rather than reusing Message so
// that an omitted id is the difference between a request and a notification,
// which Message's own omitempty cannot express for a raw id of "null".
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  any             `json:"params,omitempty"`
}

// response is a reply to something the server asked. The id goes back byte for
// byte: a server is free to number its own requests with strings, and putting
// one through an int would answer a question it never asked.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

func marshalRequest(id json.RawMessage, method string, params any) ([]byte, error) {
	return json.Marshal(request{JSONRPC: version, ID: id, Method: method, Params: params})
}

func marshalResponse(id json.RawMessage, result json.RawMessage, failure *ResponseError) ([]byte, error) {
	return json.Marshal(response{JSONRPC: version, ID: id, Result: result, Error: failure})
}

// requestID numbers what this end asks. It is a raw number rather than a string
// because that is what every server logs, which is what makes a trace readable.
func requestID(n int) json.RawMessage {
	return json.RawMessage(strconv.Itoa(n))
}

// frames is what a test reads a written stream back through, and what the
// reader's own loop is built on.
func frames(r io.Reader) *bufio.Reader { return bufio.NewReaderSize(r, readBuffer) }

const readBuffer = 64 << 10
