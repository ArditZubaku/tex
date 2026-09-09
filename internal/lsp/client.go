package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"time"
)

// ErrStopped is a client with no server behind it: not started, told to stop,
// or one whose server died. Every caller of a server has something it could
// always do without one, and this is what says to go and do that.
var ErrStopped = errors.New("lsp: no server running")

// ErrNoAnswer is a request whose deadline passed. It is answered for rather
// than left, so that a caller with a fallback runs it exactly once.
var ErrNoAnswer = errors.New("lsp: server did not answer")

const (
	// RequestTimeout is what a warm server answers a definition in several
	// times over. Past it the server is not slow, it is gone.
	RequestTimeout = 2 * time.Second

	// The handshake is the one genuinely slow call: a cold server reads the
	// whole module before it answers.
	startTimeout = 30 * time.Second

	messageQueue = 64
	writeBuffer  = 64 << 10
)

var now = time.Now

type phase int

const (
	phaseOff phase = iota
	phaseStarting
	phaseReady
	phaseStopping
	phaseFailed
)

type waiting struct {
	deadline time.Time
	answer   func(json.RawMessage, error)
}

// A Client is one language server, seen from the editor's loop. Every method on
// it belongs to the loop's goroutine; what the reader and the writer are given
// is channels and a transport, and nothing else.
type Client struct {
	dial Dial
	wake func()

	// Handler is where a notification the client does not answer itself goes —
	// the diagnostics a server publishes, and whatever else comes to be wanted.
	// It runs on the loop's goroutine, out of Poll.
	Handler func(method string, params json.RawMessage)

	transport Transport
	encoding  Encoding

	// What the server said about completion during the handshake: whether it
	// offers any, the characters it asked to be woken on, and whether it will
	// answer a second question about one candidate.
	completes bool
	resolves  bool
	triggers  string

	in       chan Message
	out      chan []byte
	gone     chan error
	readDone chan struct{}
	stopped  chan struct{}

	phase   phase
	failure error
	nextID  int
	pending map[string]waiting
}

// New is a client with nothing running. The dial is what decides whether Start
// gets a process or a pair of pipes a test drives the far end of.
func New(dial Dial, wake func()) *Client {
	if wake == nil {
		wake = func() {}
	}

	return &Client{dial: dial, wake: wake}
}

// Start brings a server up in root and sends it the handshake. It returns as
// soon as the process is running: the client is not Ready until the server has
// answered, which is when what it can do and how it counts columns are known.
func (c *Client) Start(root string) error {
	if c.phase != phaseOff {
		return nil
	}

	transport, err := c.dial(root)
	if err != nil {
		c.phase, c.failure = phaseFailed, err

		return err
	}

	c.transport, c.encoding, c.phase = transport, UTF16, phaseStarting
	c.in, c.out = make(chan Message, messageQueue), make(chan []byte, messageQueue)
	c.gone, c.readDone, c.stopped = make(chan error, 1), make(chan struct{}), make(chan struct{})
	c.pending = map[string]waiting{}

	go c.read(transport, c.in, c.gone, c.readDone, c.stopped)
	go c.write(transport, c.out, c.readDone)

	if err := c.ask(methodInitialize, handshake(root), startTimeout, c.shook); err != nil {
		c.fail(err)

		return err
	}

	return nil
}

func (c *Client) shook(result json.RawMessage, err error) {
	if err != nil {
		c.fail(err)

		return
	}

	encoding, err := encodingFrom(result)
	if err != nil {
		c.fail(err)

		return
	}

	c.encoding, c.phase = encoding, phaseReady
	c.triggers, c.resolves, c.completes = triggersFrom(result)
	c.post(methodInitialized, struct{}{})
}

// Stop tells the server to go and hands the process to the writer to reap. The
// shutdown request is sent without waiting for its answer: the editor is on its
// way out, and a server that would rather not go is killed after a grace.
func (c *Client) Stop() {
	if c.phase != phaseStarting && c.phase != phaseReady {
		return // never started, already stopped, or a server that died on its own
	}

	c.nextID++
	if frame, err := marshalRequest(requestID(c.nextID), methodShutdown, nil); err == nil {
		c.send(frame)
	}
	c.post(methodExit, nil)
	c.teardown(phaseStopping)
}

func (c *Client) fail(err error) {
	if c.phase != phaseStarting && c.phase != phaseReady {
		return
	}

	c.failure = err
	c.teardown(phaseFailed)
}

func (c *Client) teardown(to phase) {
	c.phase, c.transport = to, nil
	close(c.stopped)
	close(c.out)
	c.in, c.out = nil, nil

	failure := c.stopErr()
	for id, wait := range c.pending {
		delete(c.pending, id)
		wait.answer(nil, failure)
	}
}

// Ready is a server that has answered the handshake, and so is one worth asking
// anything of.
func (c *Client) Ready() bool { return c.phase == phaseReady }

// Encoding is how the server counts columns, which it chose during the
// handshake out of the two offered.
func (c *Client) Encoding() Encoding { return c.encoding }

// Completes is a server that offers completion, and Triggers the characters it
// asked to be woken on. A server that offers none is not asked, which is what
// keeps a request per keystroke out of a language nobody has a server for.
func (c *Client) Completes() bool { return c.completes }

func (c *Client) Triggers() string { return c.triggers }

// Resolves is a server that fills the rest of a candidate in when asked about
// that one candidate — the import line it needs, most of all.
func (c *Client) Resolves() bool { return c.resolves }

// Err is why there is no server, for the one report a crash is worth.
func (c *Client) Err() error {
	if c.phase == phaseFailed {
		return c.failure
	}

	return nil
}

// Poll takes everything the reader has left and acts on it: answers to what was
// asked, replies to what the server asks, and the notifications the Handler
// takes. It is called once a frame, and never blocks.
func (c *Client) Poll() {
	for c.in != nil && c.took() {
	}
	c.reap()
}

// The messages come before the death: a server's last frames are usually the
// diagnostics for what killed it, and they are worth having.
func (c *Client) took() bool {
	select {
	case msg := <-c.in:
		c.route(msg)

		return true
	default:
	}

	select {
	case err := <-c.gone:
		c.fail(err)
	default:
	}

	return false
}

func (c *Client) route(msg Message) {
	switch {
	case msg.IsResponse():
		c.answered(msg)
	case msg.IsRequest():
		c.asked(msg)
	case msg.Method != "" && c.Handler != nil:
		c.Handler(msg.Method, msg.Params)
	}
}

func (c *Client) answered(msg Message) {
	wait, ok := c.pending[string(msg.ID)]
	if !ok {
		return // an answer to something already given up on
	}
	delete(c.pending, string(msg.ID))

	if msg.Error != nil {
		wait.answer(nil, msg.Error)

		return
	}
	wait.answer(msg.Result, nil)
}

// A server left waiting on a reply it was promised stops making progress:
// gopls registers its file watching this way and then goes quiet, which reads
// as "diagnostics worked for ten seconds and then stopped".
func (c *Client) asked(msg Message) {
	switch msg.Method {
	case methodRegisterCapability, methodUnregisterCapability:
		c.reply(msg.ID, json.RawMessage("null"), nil)
	default:
		c.reply(msg.ID, nil, &ResponseError{
			Code:    CodeMethodNotFound,
			Message: msg.Method + " is not implemented",
		})
	}
}

func (c *Client) reply(id, result json.RawMessage, failure *ResponseError) {
	if frame, err := marshalResponse(id, result, failure); err == nil {
		c.send(frame)
	}
}

// Request asks the server something. The answer comes back later, out of Poll,
// into an editor that may have moved on since — which is the caller's to
// notice. A deadline passing counts as an answer of ErrNoAnswer.
func (c *Client) Request(method string, params any, answer func(json.RawMessage, error)) error {
	if c.phase != phaseReady {
		return c.stopErr()
	}

	return c.ask(method, params, RequestTimeout, answer)
}

func (c *Client) ask(
	method string, params any, within time.Duration, answer func(json.RawMessage, error),
) error {
	c.nextID++
	id := requestID(c.nextID)

	frame, err := marshalRequest(id, method, params)
	if err != nil {
		return err
	}

	c.pending[string(id)] = waiting{deadline: now().Add(within), answer: answer}
	c.send(frame)

	return nil
}

// Notify tells the server something. There is no answer and so nothing to
// report: a notification that did not fit is one the next frame's reconciling
// sends again.
func (c *Client) Notify(method string, params any) {
	if c.phase != phaseReady {
		return
	}
	c.post(method, params)
}

func (c *Client) post(method string, params any) {
	if frame, err := marshalRequest(nil, method, params); err == nil {
		c.send(frame)
	}
}

// A server that is not reading is a server that is going: the frame goes
// nowhere rather than stopping the loop on it. Every sender here is a
// reconciler, and asks again on the next frame.
func (c *Client) send(frame []byte) {
	if c.out == nil {
		return
	}

	select {
	case c.out <- frame:
	default:
	}
}

func (c *Client) stopErr() error {
	if c.failure != nil {
		return c.failure
	}

	return ErrStopped
}

func (c *Client) reap() {
	at := now()
	for id, wait := range c.pending {
		if at.Before(wait.deadline) {
			continue
		}
		delete(c.pending, id)
		wait.answer(nil, ErrNoAnswer)
	}
}

func (c *Client) read(
	transport Transport,
	in chan<- Message,
	gone chan<- error,
	readDone chan<- struct{},
	stopped <-chan struct{},
) {
	defer close(readDone)

	wire := frames(transport)
	for {
		frame, err := readFrame(wire)
		if err != nil {
			select {
			case gone <- err:
			default:
			}
			c.wake() // a server dying is a frame owed too: what was waiting on it is not

			return
		}

		var msg Message
		if err := json.Unmarshal(frame, &msg); err != nil {
			continue // a frame that is not a message says nothing worth stopping for
		}

		select {
		case in <- msg:
		case <-stopped:
			return
		}
		c.wake()
	}
}

func (c *Client) write(transport Transport, out <-chan []byte, readDone <-chan struct{}) {
	to := bufio.NewWriterSize(transport, writeBuffer)
	for frame := range out {
		if err := writeFrame(to, frame); err != nil {
			break
		}
		// A burst of frames is one write to the pipe: the flush waits until
		// there is nothing queued behind it.
		if len(out) == 0 {
			if err := to.Flush(); err != nil {
				break
			}
		}
	}

	_ = transport.CloseSend()

	// The reader owns the server's output and Close reaps the process out from
	// under it, so the two are kept apart by waiting here.
	select {
	case <-readDone:
	case <-time.After(exitGrace):
	}
	_ = transport.Close()
}
