package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"
)

// The client's own end of a net.Pipe. There is no half-close on one, so saying
// there is nothing more to send closes it outright, which is what the server
// side reads as the end either way.
type pipe struct{ net.Conn }

func (p pipe) CloseSend() error { return p.Close() }

// A server is the far end of the pipe, driven by the test a frame at a time.
type server struct {
	t    *testing.T
	conn net.Conn
	wire *bufio.Reader
}

func (s *server) next() Message {
	s.t.Helper()

	frame, err := readFrame(s.wire)
	if err != nil {
		s.t.Fatalf("reading what the client sent: %v", err)
	}

	var msg Message
	if err := json.Unmarshal(frame, &msg); err != nil {
		s.t.Fatalf("the client sent %q, which is not a message: %v", frame, err)
	}

	return msg
}

func (s *server) put(frame []byte, err error) {
	s.t.Helper()

	if err != nil {
		s.t.Fatal(err)
	}

	out := bufio.NewWriter(s.conn)
	if err := writeFrame(out, frame); err != nil {
		s.t.Fatal(err)
	}
	if err := out.Flush(); err != nil {
		s.t.Fatal(err)
	}
}

func (s *server) answer(id json.RawMessage, result any) {
	s.t.Helper()

	raw, err := json.Marshal(result)
	if err != nil {
		s.t.Fatal(err)
	}
	s.put(marshalResponse(id, raw, nil))
}

func (s *server) refuse(id json.RawMessage, failure *ResponseError) {
	s.t.Helper()

	s.put(marshalResponse(id, nil, failure))
}

func (s *server) notify(method string, params any) {
	s.t.Helper()

	s.put(marshalRequest(nil, method, params))
}

func (s *server) ask(id json.RawMessage, method string, params any) {
	s.t.Helper()

	s.put(marshalRequest(id, method, params))
}

type harness struct {
	t      *testing.T
	client *Client
	server *server
	woken  chan struct{}
}

func dialed(t *testing.T) *harness {
	t.Helper()

	Reset()
	t.Cleanup(Reset)

	near, far := net.Pipe()
	// One deadline for the whole test, set before either end can close: a
	// net.Pipe refuses to take one once it has, so setting it per read would
	// fail on exactly the reads that come after a shutdown.
	if err := far.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}

	h := &harness{
		t:      t,
		server: &server{t: t, conn: far, wire: bufio.NewReader(far)},
		woken:  make(chan struct{}, 1),
	}
	h.client = New(func(string) (Transport, error) { return pipe{near}, nil }, h.wake)

	t.Cleanup(func() {
		h.client.Stop()
		_ = far.Close()
	})

	if err := h.client.Start(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	return h
}

// The real waker coalesces on a channel of one and never blocks; so does this,
// and waiting on it is what makes an answer arriving observable at all.
func (h *harness) wake() {
	select {
	case h.woken <- struct{}{}:
	default:
	}
}

func (h *harness) poll() {
	h.t.Helper()

	select {
	case <-h.woken:
	case <-time.After(2 * time.Second):
		h.t.Fatal("the loop was never woken")
	}
	h.client.Poll()
}

// shake is the handshake done with, which every test but the handshake's own
// starts from.
func (h *harness) shake(encoding string) {
	h.t.Helper()

	asked := h.server.next()
	if asked.Method != methodInitialize {
		h.t.Fatalf("first message was %q, want %q", asked.Method, methodInitialize)
	}

	h.server.answer(asked.ID, initializeResult{
		Capabilities: serverCapability{PositionEncoding: encoding},
	})
	h.poll()

	if got := h.server.next(); got.Method != methodInitialized {
		h.t.Fatalf("after the handshake the client sent %q, want %q", got.Method, methodInitialized)
	}
}

// stopped is the clock, held still so that a deadline passes when the test says
// it does rather than when the suite happens to be slow.
func held(t *testing.T) func(time.Duration) {
	t.Helper()

	at := time.Now()
	now = func() time.Time { return at }
	t.Cleanup(func() { now = time.Now })

	return func(by time.Duration) { at = at.Add(by) }
}

func TestTheHandshakeIsWhatMakesAServerWorthAsking(t *testing.T) {
	h := dialed(t)

	if h.client.Ready() {
		t.Fatal("the client was ready before the server had answered anything")
	}
	if err := h.client.Request("textDocument/definition", nil, nil); !errors.Is(err, ErrStopped) {
		t.Fatalf("asking before the handshake gave %v, want %v", err, ErrStopped)
	}

	h.shake("")

	if !h.client.Ready() {
		t.Fatal("the client was not ready with the handshake answered")
	}
}

func TestTheServerChoosesHowColumnsAreCounted(t *testing.T) {
	for _, tc := range []struct {
		chose string
		want  Encoding
	}{
		{chose: "", want: UTF16},
		{chose: "utf-16", want: UTF16},
		{chose: "utf-8", want: UTF8},
	} {
		t.Run(tc.chose, func(t *testing.T) {
			h := dialed(t)
			h.shake(tc.chose)

			if got := h.client.Encoding(); got != tc.want {
				t.Errorf("counting columns as %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAnEncodingNeitherEndOfferedIsRefusedRatherThanGuessedAt(t *testing.T) {
	h := dialed(t)

	asked := h.server.next()
	h.server.answer(asked.ID, initializeResult{
		Capabilities: serverCapability{PositionEncoding: "utf-32"},
	})
	h.poll()

	if h.client.Ready() {
		t.Fatal("a server counting columns a third way was taken as ready")
	}

	var unsupported *UnsupportedEncodingError
	if !errors.As(h.client.Err(), &unsupported) {
		t.Fatalf("the client failed with %v, want an unsupported encoding", h.client.Err())
	}
}

func TestAHandshakeNeverAnsweredLeavesTheClientWithNoServer(t *testing.T) {
	pass := held(t)
	h := dialed(t)

	h.client.Poll()
	if h.client.Err() != nil {
		t.Fatalf("the client gave up at once: %v", h.client.Err())
	}

	pass(startTimeout)
	h.client.Poll()

	if !errors.Is(h.client.Err(), ErrNoAnswer) {
		t.Fatalf("the client failed with %v, want %v", h.client.Err(), ErrNoAnswer)
	}
	if h.client.Ready() {
		t.Fatal("a server that never answered was taken as ready")
	}
}

func TestARequestNeverAnsweredIsAnsweredForExactlyOnce(t *testing.T) {
	pass := held(t)
	h := dialed(t)
	h.shake("")

	answers := []error{}
	if err := h.client.Request("textDocument/definition", nil, func(_ json.RawMessage, err error) {
		answers = append(answers, err)
	}); err != nil {
		t.Fatal(err)
	}
	h.server.next()

	pass(RequestTimeout)
	h.client.Poll()
	h.client.Poll()

	if len(answers) != 1 || !errors.Is(answers[0], ErrNoAnswer) {
		t.Fatalf("the caller was answered %v, want one %v", answers, ErrNoAnswer)
	}
}

func TestAnAnswerToARequestAlreadyGivenUpOnIsDropped(t *testing.T) {
	pass := held(t)
	h := dialed(t)
	h.shake("")

	answers := 0
	if err := h.client.Request("textDocument/definition", nil, func(json.RawMessage, error) {
		answers++
	}); err != nil {
		t.Fatal(err)
	}
	asked := h.server.next()

	pass(RequestTimeout)
	h.client.Poll()

	h.server.answer(asked.ID, []string{})
	h.poll()

	if answers != 1 {
		t.Fatalf("the caller was answered %d times, want once", answers)
	}
}

func TestAServersOwnErrorReachesTheCaller(t *testing.T) {
	h := dialed(t)
	h.shake("")

	var got error
	if err := h.client.Request("textDocument/definition", nil, func(_ json.RawMessage, err error) {
		got = err
	}); err != nil {
		t.Fatal(err)
	}

	asked := h.server.next()
	h.server.refuse(asked.ID, &ResponseError{Code: CodeInternalError, Message: "no package"})
	h.poll()

	var failure *ResponseError
	if !errors.As(got, &failure) || failure.Code != CodeInternalError {
		t.Fatalf("the caller was answered %v, want the server's own error", got)
	}
}

// A server left waiting on a reply it was promised stops making progress, and
// the id goes back as it came: a server is free to number its own requests with
// strings.
func TestEveryRequestTheServerMakesIsAnswered(t *testing.T) {
	for _, tc := range []struct {
		method string
		want   int
	}{
		{method: methodRegisterCapability},
		{method: methodUnregisterCapability},
		{method: "window/showMessageRequest", want: CodeMethodNotFound},
	} {
		t.Run(tc.method, func(t *testing.T) {
			h := dialed(t)
			h.shake("")

			h.server.ask(json.RawMessage(`"a-string-id"`), tc.method, nil)
			h.poll()

			reply := h.server.next()
			if string(reply.ID) != `"a-string-id"` {
				t.Errorf("replied to id %s, want the server's own", reply.ID)
			}
			switch {
			case tc.want == 0 && reply.Error != nil:
				t.Errorf("%s was refused with %v", tc.method, reply.Error)
			case tc.want != 0 && (reply.Error == nil || reply.Error.Code != tc.want):
				t.Errorf("%s was answered %v, want code %d", tc.method, reply.Error, tc.want)
			}
		})
	}
}

func TestANotificationTheClientDoesNotAnswerGoesToTheHandler(t *testing.T) {
	h := dialed(t)
	h.shake("")

	var method string
	var params json.RawMessage
	h.client.Handler = func(m string, p json.RawMessage) { method, params = m, p }

	h.server.notify("window/logMessage", map[string]any{"uri": "file:///x.go"})
	h.poll()

	if method != "window/logMessage" {
		t.Fatalf("the handler was given %q", method)
	}
	if string(params) != `{"uri":"file:///x.go"}` {
		t.Fatalf("the handler was given params %s, want them as they arrived", params)
	}
}

func TestAServerThatDiesLeavesNothingWaitingOnIt(t *testing.T) {
	h := dialed(t)
	h.shake("")

	var got error
	if err := h.client.Request("textDocument/definition", nil, func(_ json.RawMessage, err error) {
		got = err
	}); err != nil {
		t.Fatal(err)
	}
	h.server.next()

	if err := h.server.conn.Close(); err != nil {
		t.Fatal(err)
	}
	h.poll()

	if got == nil {
		t.Fatal("the caller was left waiting on a server that had gone")
	}
	if h.client.Ready() {
		t.Fatal("a server that had gone was still taken as ready")
	}
	if err := h.client.Request("textDocument/definition", nil, nil); err == nil {
		t.Fatal("asking a server that had gone was allowed")
	}
}

func TestStoppingTellsTheServerToGoBeforeClosingItsInput(t *testing.T) {
	h := dialed(t)
	h.shake("")

	done := make(chan []string, 1)
	go func() {
		var said []string
		for range 2 {
			said = append(said, h.server.next().Method)
		}
		if _, err := h.server.wire.ReadByte(); err == nil {
			said = append(said, "and then went on talking")
		}
		done <- said
	}()

	h.client.Stop()

	select {
	case said := <-done:
		if len(said) != 2 || said[0] != methodShutdown || said[1] != methodExit {
			t.Fatalf("stopping said %v, want %q then %q then nothing",
				said, methodShutdown, methodExit)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stopping never reached the server")
	}

	h.client.Stop() // stopping twice is what quitting with a dead server does
}

func TestAServerThatIsNotInstalledIsNotAFailure(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	const missing = "tex-no-such-language-server"
	if Installed(missing) {
		t.Skipf("something called %q is on the PATH", missing)
	}

	_, err := Server(missing)(t.TempDir())
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("dialing something not installed gave %v, want %v", err, ErrNotInstalled)
	}
}

func TestNothingIsSentToAServerThatIsNotReady(t *testing.T) {
	h := dialed(t)

	h.client.Notify("textDocument/didOpen", nil)

	asked := h.server.next()
	if asked.Method != methodInitialize {
		t.Fatalf("the client sent %q before the handshake was answered", asked.Method)
	}
}

func TestAFrameThatIsNotAMessageDoesNotStopTheReader(t *testing.T) {
	h := dialed(t)

	out := bufio.NewWriter(h.server.conn)
	go func() {
		_ = writeFrame(out, []byte("this is not JSON at all"))
		_ = out.Flush()
	}()

	asked := h.server.next()
	h.server.answer(asked.ID, initializeResult{})
	h.poll()

	if !h.client.Ready() {
		t.Fatalf("the reader gave up on a frame it could not read: %v", h.client.Err())
	}
}
