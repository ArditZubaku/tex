package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func framed(t *testing.T, bodies ...string) *bufio.Reader {
	t.Helper()

	var wire bytes.Buffer
	w := bufio.NewWriter(&wire)
	for _, body := range bodies {
		if err := writeFrame(w, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	return frames(&wire)
}

func TestAFrameIsWrittenAndReadBack(t *testing.T) {
	r := framed(t, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, `{"jsonrpc":"2.0","id":1,"result":{}}`)

	for _, want := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","id":1,"result":{}}`,
	} {
		frame, err := readFrame(r)
		if err != nil {
			t.Fatal(err)
		}
		if string(frame) != want {
			t.Errorf("frame = %q, want %q", frame, want)
		}
	}

	if _, err := readFrame(r); !errors.Is(err, io.EOF) {
		t.Errorf("err = %v, want EOF once the stream has ended", err)
	}
}

func TestAFrameCarriesExactlyTheBytesItPromised(t *testing.T) {
	// the body of one frame runs straight into the header of the next, so a
	// reader that took a line at a time rather than the promised length would
	// swallow the frame after this one
	r := framed(t, "{\"a\":\"line\\nbreak\"}", `{"b":2}`)

	first, err := readFrame(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != "{\"a\":\"line\\nbreak\"}" {
		t.Fatalf("frame = %q", first)
	}

	second, err := readFrame(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != `{"b":2}` {
		t.Errorf("frame = %q, want the frame after it intact", second)
	}
}

func TestAHeaderTheProtocolDoesNotDefineIsSkipped(t *testing.T) {
	wire := strings.NewReader("Content-Type: application/vscode-jsonrpc; charset=utf-8\r\nContent-Length: 7\r\n\r\n{\"b\":2}")

	frame, err := readFrame(frames(wire))
	if err != nil {
		t.Fatal(err)
	}
	if string(frame) != `{"b":2}` {
		t.Errorf("frame = %q", frame)
	}
}

func TestAFrameWithNoLengthAndOneTooLargeAreBothRefused(t *testing.T) {
	if _, err := readFrame(frames(strings.NewReader("Content-Type: x\r\n\r\n{}"))); err == nil {
		t.Error("a message with no Content-Length was accepted")
	}

	_, err := readFrame(frames(strings.NewReader("Content-Length: 99999999999\r\n\r\n")))
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Errorf("err = %v, want ErrFrameTooLarge", err)
	}

	if _, err := readFrame(frames(strings.NewReader("Content-Length: seven\r\n\r\n{}"))); err == nil {
		t.Error("a Content-Length that is not a number was accepted")
	}
}

func TestAFrameCutShortDoesNotComeBackAsHalfAMessage(t *testing.T) {
	if _, err := readFrame(frames(strings.NewReader("Content-Length: 20\r\n\r\n{\"a\":1}"))); err == nil {
		t.Error("a frame shorter than it promised was accepted")
	}
}

func TestANotificationIsARequestWithNoIDOnIt(t *testing.T) {
	frame, err := marshalRequest(nil, "initialized", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(frame, []byte(`"id"`)) {
		t.Errorf("notification = %s, want no id on it", frame)
	}

	frame, err = marshalRequest(requestID(3), "textDocument/hover", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(frame, []byte(`"id":3`)) {
		t.Errorf("request = %s, want the id it was given", frame)
	}
}

// A server is free to number its own requests with strings, and putting one
// through an int would answer a question it never asked.
func TestAReplyCarriesTheServersOwnIDBackByteForByte(t *testing.T) {
	frame, err := marshalResponse(json.RawMessage(`"reg-1"`), json.RawMessage("null"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(frame, []byte(`"id":"reg-1"`)) {
		t.Errorf("reply = %s, want the string id back as it came", frame)
	}

	frame, err = marshalResponse(requestID(2), nil, &ResponseError{Code: CodeMethodNotFound, Message: "workspace/configuration"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(frame, []byte(`"code":-32601`)) {
		t.Errorf("reply = %s, want the error code on it", frame)
	}
}

func TestWhichOfTheThreeKindsAMessageIsShowsInItsFields(t *testing.T) {
	cases := []struct {
		body       string
		isRequest  bool
		isResponse bool
	}{
		{`{"jsonrpc":"2.0","id":1,"method":"client/registerCapability"}`, true, false},
		{`{"jsonrpc":"2.0","id":1,"result":{}}`, false, true},
		{`{"jsonrpc":"2.0","method":"textDocument/publishDiagnostics"}`, false, false},
		{`{"jsonrpc":"2.0","id":"reg-1","method":"client/registerCapability"}`, true, false},
	}

	for _, one := range cases {
		var m Message
		if err := json.Unmarshal([]byte(one.body), &m); err != nil {
			t.Fatal(err)
		}
		if m.IsRequest() != one.isRequest || m.IsResponse() != one.isResponse {
			t.Errorf("%s: request=%v response=%v, want %v and %v",
				one.body, m.IsRequest(), m.IsResponse(), one.isRequest, one.isResponse)
		}
	}
}
