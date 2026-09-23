package protocol

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

// The WebSocket transport (#326, RFC 6455): the protocol's messages, one
// JSON-RPC message a text frame, for a browser client or a game engine
// that would rather open a socket than start a process. It is framing
// and nothing else: ServeWS hands each message to Server.Handle, the
// loop stdio's Serve runs, and sends what it returns a frame each, the
// notifications before the response as on stdio.
//
// It is the standard library alone, no dependency, and like the rest of
// the tree it starts no goroutine: a connection is read and answered on
// the caller's goroutine, and cmd/kingpind serves one connection at a
// time. A second client waits until the first hangs up.

// wsGUID is RFC 6455's, hashed with the client's key into the accept.
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// maxMessage is the largest message either end takes, stdio's line cap.
const maxMessage = 16 << 20

// The opcodes.
const (
	opCont  = 0x0
	opText  = 0x1
	opBin   = 0x2
	opClose = 0x8
	opPing  = 0x9
	opPong  = 0xA
)

// The close codes the transport sends.
const (
	closeNormal   = 1000
	closeProtocol = 1002
	closeData     = 1003 // a binary frame: the protocol is text
	closeUTF8     = 1007
	closeTooBig   = 1009
)

// WS is one end of a WebSocket connection. The server's end comes from
// AcceptWS, the client's from DialWS. It is an io.Reader and an
// io.Writer of lines, so an RPCClient speaks the protocol over it as it
// does over a pipe: a line written is a message sent, and a message
// received reads as a line.
type WS struct {
	r      *bufio.Reader
	w      *bufio.Writer
	client bool   // masks what it sends, as RFC 6455 has a client do
	line   []byte // a message received and not yet all read
	closed bool   // a close frame has gone out
}

// ErrClosed is a read on a connection the other end closed.
var ErrClosed = errors.New("websocket: closed")

// wsError is a frame the transport refuses: the close code it sends.
type wsError struct {
	code int
	msg  string
}

func (e *wsError) Error() string { return fmt.Sprintf("websocket: %s (%d)", e.msg, e.code) }

// AcceptWS reads a client's opening handshake from r and answers it on
// w. A request that is not a WebSocket upgrade to /, and one from a web
// page anywhere but this machine (an Origin whose host is not loopback:
// a page on another site must not drive the game), is answered with an
// HTTP error and refused.
func AcceptWS(r *bufio.Reader, w io.Writer) (*WS, error) {
	req, err := http.ReadRequest(r)
	if err != nil {
		return nil, fmt.Errorf("websocket: the handshake: %w", err)
	}
	refuse := func(status int, extra, msg string) (*WS, error) {
		fmt.Fprintf(w, "HTTP/1.1 %d %s\r\nContent-Type: text/plain; charset=utf-8\r\nConnection: close\r\n%s\r\n%s\n", status, http.StatusText(status), extra, msg)
		return nil, fmt.Errorf("websocket: refused %d: %s", status, msg)
	}
	switch {
	case req.Method != http.MethodGet:
		return refuse(http.StatusMethodNotAllowed, "", "a WebSocket opens with a GET")
	case req.URL.Path != "/":
		return refuse(http.StatusNotFound, "", "kingpind serves /")
	case !hasToken(req.Header, "Connection", "upgrade") || !hasToken(req.Header, "Upgrade", "websocket"):
		return refuse(http.StatusBadRequest, "", "not a WebSocket upgrade")
	case req.Header.Get("Sec-WebSocket-Version") != "13":
		return refuse(http.StatusUpgradeRequired, "Sec-WebSocket-Version: 13\r\n", "WebSocket version 13 only")
	}
	key := req.Header.Get("Sec-WebSocket-Key")
	if k, err := base64.StdEncoding.DecodeString(key); err != nil || len(k) != 16 {
		return refuse(http.StatusBadRequest, "", "no Sec-WebSocket-Key")
	}
	if o := req.Header.Get("Origin"); o != "" && !loopbackOrigin(o) {
		return refuse(http.StatusForbidden, "", "kingpind serves pages on this machine alone")
	}
	if _, err := fmt.Fprintf(w, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", acceptKey(key)); err != nil {
		return nil, err
	}
	return &WS{r: r, w: bufio.NewWriter(w)}, nil
}

// DialWS opens the client's end over conn, a connection to host: the
// handshake for path, and the server's answer checked.
func DialWS(conn io.ReadWriter, host, path string) (*WS, error) {
	var k [16]byte
	if _, err := rand.Read(k[:]); err != nil {
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(k[:])
	if _, err := fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", path, host, key); err != nil {
		return nil, err
	}
	r := bufio.NewReaderSize(conn, 1<<16)
	resp, err := http.ReadResponse(r, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket: the handshake: %w", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("websocket: the server answered %s: %s", resp.Status, bytes.TrimSpace(body))
	}
	if resp.Header.Get("Sec-WebSocket-Accept") != acceptKey(key) {
		return nil, errors.New("websocket: the server's accept does not match the key")
	}
	return &WS{r: r, w: bufio.NewWriter(conn), client: true}, nil
}

func acceptKey(key string) string {
	h := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h[:])
}

// hasToken reports whether a comma-separated header holds token, in any
// case.
func hasToken(h http.Header, name, token string) bool {
	for _, v := range h.Values(name) {
		for _, t := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(t), token) {
				return true
			}
		}
	}
	return false
}

// loopbackOrigin reports whether a page's origin is on this machine.
func loopbackOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	h := u.Hostname()
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// ReadMessage is the next text message, its fragments joined. A ping is
// answered and a pong dropped on the way. A close is answered and ends
// the connection: io.EOF. A frame the transport refuses closes it with
// the code for why and is the error.
func (c *WS) ReadMessage() ([]byte, error) {
	var msg []byte
	in := false // inside a fragmented message
	for {
		fin, op, payload, err := c.readFrame()
		if err != nil {
			return nil, c.fail(err)
		}
		switch op {
		case opPing:
			if err := c.writeFrame(opPong, payload); err != nil {
				return nil, err
			}
			continue
		case opPong:
			continue
		case opClose:
			code := closeNormal
			if len(payload) >= 2 {
				code = int(binary.BigEndian.Uint16(payload))
			}
			if !c.closed {
				_ = c.Close(code, "")
			}
			return nil, io.EOF
		case opBin:
			return nil, c.fail(&wsError{closeData, "a binary frame: the protocol is text"})
		case opText:
			if in {
				return nil, c.fail(&wsError{closeProtocol, "a new message inside a fragmented one"})
			}
			msg, in = payload, true
		case opCont:
			if !in {
				return nil, c.fail(&wsError{closeProtocol, "a continuation with no message"})
			}
			if len(msg)+len(payload) > maxMessage {
				return nil, c.fail(&wsError{closeTooBig, "a message over 16 MiB"})
			}
			msg = append(msg, payload...)
		default:
			return nil, c.fail(&wsError{closeProtocol, fmt.Sprintf("opcode %#x", op)})
		}
		if fin {
			if !utf8.Valid(msg) {
				return nil, c.fail(&wsError{closeUTF8, "a text message that is not UTF-8"})
			}
			return msg, nil
		}
	}
}

// fail closes the connection with the code a refused frame carries and
// returns the error.
func (c *WS) fail(err error) error {
	var we *wsError
	if errors.As(err, &we) && !c.closed {
		_ = c.Close(we.code, we.msg)
	}
	return err
}

// readFrame reads one frame, unmasked. A client's frames must be masked
// and a server's must not; a control frame is whole and short.
func (c *WS) readFrame() (fin bool, op byte, payload []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(c.r, h[:]); err != nil {
		if errors.Is(err, io.EOF) {
			err = ErrClosed
		}
		return
	}
	fin, op = h[0]&0x80 != 0, h[0]&0x0F
	if h[0]&0x70 != 0 {
		return fin, op, nil, &wsError{closeProtocol, "a reserved bit set"}
	}
	masked := h[1]&0x80 != 0
	if masked == c.client {
		return fin, op, nil, &wsError{closeProtocol, "a frame masked the wrong way for its side"}
	}
	n := uint64(h[1] & 0x7F)
	switch n {
	case 126:
		var ext [2]byte
		if _, err = io.ReadFull(c.r, ext[:]); err != nil {
			return
		}
		n = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err = io.ReadFull(c.r, ext[:]); err != nil {
			return
		}
		n = binary.BigEndian.Uint64(ext[:])
	}
	if op >= opClose && (n > 125 || !fin) {
		return fin, op, nil, &wsError{closeProtocol, "a control frame fragmented or over 125 bytes"}
	}
	if n > maxMessage {
		return fin, op, nil, &wsError{closeTooBig, "a message over 16 MiB"}
	}
	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(c.r, mask[:]); err != nil {
			return
		}
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(c.r, payload); err != nil {
		return
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return fin, op, payload, nil
}

// writeFrame sends one whole frame, masked if this is the client's end.
func (c *WS) writeFrame(op byte, payload []byte) error {
	h := []byte{0x80 | op, 0}
	switch n := len(payload); {
	case n < 126:
		h[1] = byte(n)
	case n <= 0xFFFF:
		h[1] = 126
		h = binary.BigEndian.AppendUint16(h, uint16(n))
	default:
		h[1] = 127
		h = binary.BigEndian.AppendUint64(h, uint64(n))
	}
	if c.client {
		h[1] |= 0x80
		var mask [4]byte
		if _, err := rand.Read(mask[:]); err != nil {
			return err
		}
		h = append(h, mask[:]...)
		masked := make([]byte, len(payload))
		for i, b := range payload {
			masked[i] = b ^ mask[i%4]
		}
		payload = masked
	}
	if _, err := c.w.Write(h); err != nil {
		return err
	}
	if _, err := c.w.Write(payload); err != nil {
		return err
	}
	return c.w.Flush()
}

// WriteMessage sends one text message in one frame.
func (c *WS) WriteMessage(msg []byte) error { return c.writeFrame(opText, msg) }

// Close sends a close frame with the code and the reason. It does not
// wait for the other end's: the caller hangs up.
func (c *WS) Close(code int, reason string) error {
	if c.closed {
		return nil
	}
	c.closed = true
	p := binary.BigEndian.AppendUint16(nil, uint16(code))
	if len(reason) > 123 {
		reason = reason[:123]
	}
	return c.writeFrame(opClose, append(p, reason...))
}

// Write sends each line of p as a message: an RPCClient's request.
func (c *WS) Write(p []byte) (int, error) {
	for _, line := range bytes.Split(p, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		if err := c.WriteMessage(line); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// Read reads the messages received as lines: an RPCClient's answers.
func (c *WS) Read(p []byte) (int, error) {
	if len(c.line) == 0 {
		msg, err := c.ReadMessage()
		if err != nil {
			return 0, err
		}
		c.line = append(msg, '\n')
	}
	n := copy(p, c.line)
	c.line = c.line[n:]
	return n, nil
}

// ServeWS serves s over one connection until the client closes it: each
// message is a request, and what it produces goes back a frame a line.
func ServeWS(c *WS, s *Server) error {
	for {
		msg, err := c.ReadMessage()
		if errors.Is(err, io.EOF) || errors.Is(err, ErrClosed) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(bytes.TrimSpace(msg)) == 0 {
			continue
		}
		for _, line := range s.Handle(msg) {
			if err := c.WriteMessage(line); err != nil {
				return err
			}
		}
	}
}
