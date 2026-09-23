package protocol

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
)

// buildKingpind builds cmd/kingpind into a temporary directory.
func buildKingpind(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "kingpind")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/kingpind")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatal(err)
	}
	return bin
}

// dialWS connects a client to a kingpind listening at host.
func dialWS(t *testing.T, host string) (*RPCClient, *WS, net.Conn) {
	t.Helper()
	conn, err := net.Dial("tcp", host)
	if err != nil {
		t.Fatal(err)
	}
	ws, err := DialWS(conn, host, "/")
	if err != nil {
		t.Fatal(err)
	}
	return NewRPCClient(ws, ws), ws, conn
}

// TestOverWebSocket (#326): the reference game played against a real
// kingpind -listen over a real socket is the direct run, byte for byte:
// every event and the last view. A second client after the first hangs
// up gets a session of its own, with no run until it starts one.
func TestOverWebSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("builds kingpind")
	}
	t.Parallel()
	bin := buildKingpind(t)
	cmd := exec.Command(bin, "-listen", "127.0.0.1:0")
	cmd.Env = append(os.Environ(), "KINGPIN_HOME="+t.TempDir())
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	first, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	host := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(first), "ws://"), "/")
	if host == "" || !strings.HasPrefix(first, "ws://127.0.0.1:") {
		t.Fatalf("kingpind printed %q, want its ws:// URL", first)
	}

	c, ws, conn := dialWS(t, host)
	v, err := Play(c, 7, playDays)
	if err != nil {
		t.Fatal(err)
	}
	vc := finalView(t, c)
	if v.Over == nil {
		t.Fatalf("no ending in %d days over WebSocket", playDays)
	}
	if err := ws.Close(closeNormal, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.ReadMessage(); !errors.Is(err, io.EOF) {
		t.Errorf("the server's answer to a close: %v, want its close", err)
	}
	conn.Close()
	d := newDirect(t)
	if _, err := Play(d, 7, playDays); err != nil {
		t.Fatal(err)
	}
	sameRun(t, d, c, finalView(t, d), vc)

	c2, ws2, conn2 := dialWS(t, host)
	var e *Error
	if err := c2.Call("view", nil, nil); !errors.As(err, &e) || e.Code != CodeNoRun {
		t.Errorf("the second client's view: %v, want no run: a session of its own", err)
	}
	_ = ws2.Close(closeNormal, "")
	conn2.Close()
}

// TestListenIsLoopbackOnly (#326): kingpind refuses to listen anywhere
// but this machine: the protocol has no authentication.
func TestListenIsLoopbackOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("builds kingpind")
	}
	t.Parallel()
	bin := buildKingpind(t)
	for _, addr := range []string{"0.0.0.0:0", "[::]:0", "192.0.2.1:0"} {
		out, err := exec.Command(bin, "-listen", addr).CombinedOutput()
		if err == nil || !strings.Contains(string(out), "loopback") {
			t.Errorf("-listen %s: %v %q, want refused for loopback", addr, err, out)
		}
	}
}

// handshake is a client's opening request with the headers given.
func handshake(method, path string, headers ...string) string {
	return method + " " + path + " HTTP/1.1\r\nHost: localhost\r\n" + strings.Join(headers, "\r\n") + "\r\n\r\n"
}

var goodHeaders = []string{"Upgrade: websocket", "Connection: keep-alive, Upgrade", "Sec-WebSocket-Version: 13", "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ=="}

// TestHandshake (#326): the accept is RFC 6455's for its sample key, a
// page on this machine may connect, and everything else is answered
// with its HTTP error.
func TestHandshake(t *testing.T) {
	t.Parallel()
	with := func(extra ...string) []string { return append(append([]string{}, goodHeaders...), extra...) }
	without := func(drop string) []string {
		var out []string
		for _, h := range goodHeaders {
			if !strings.HasPrefix(h, drop) {
				out = append(out, h)
			}
		}
		return out
	}
	for _, c := range []struct {
		name, req, status string
	}{
		{"good", handshake("GET", "/", goodHeaders...), "101"},
		{"a page on localhost", handshake("GET", "/", with("Origin: http://localhost:8080")...), "101"},
		{"a page on 127.0.0.1", handshake("GET", "/", with("Origin: http://127.0.0.1")...), "101"},
		{"a page elsewhere", handshake("GET", "/", with("Origin: https://example.com")...), "403"},
		{"a POST", handshake("POST", "/", goodHeaders...), "405"},
		{"another path", handshake("GET", "/other", goodHeaders...), "404"},
		{"no upgrade", handshake("GET", "/", without("Upgrade")...), "400"},
		{"version 8", handshake("GET", "/", append(without("Sec-WebSocket-Version"), "Sec-WebSocket-Version: 8")...), "426"},
		{"no key", handshake("GET", "/", without("Sec-WebSocket-Key")...), "400"},
	} {
		var out bytes.Buffer
		_, err := AcceptWS(bufio.NewReader(strings.NewReader(c.req)), &out)
		if got := strings.Fields(out.String()); len(got) < 2 || got[1] != c.status {
			t.Errorf("%s: answered %q, want %s", c.name, out.String(), c.status)
		}
		if (err == nil) != (c.status == "101") {
			t.Errorf("%s: err %v", c.name, err)
		}
		if c.status == "101" && !strings.Contains(out.String(), "Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=\r\n") {
			t.Errorf("%s: not RFC 6455's accept: %q", c.name, out.String())
		}
	}
}

// pipe is a server's end reading what a client's end wrote, and the
// client's end reading what the server wrote back.
type pipe struct {
	toServer, toClient bytes.Buffer
	client, server     *WS
}

func newPipe() *pipe {
	p := &pipe{}
	p.client = &WS{r: bufio.NewReader(&p.toClient), w: bufio.NewWriter(&p.toServer), client: true}
	p.server = &WS{r: bufio.NewReader(&p.toServer), w: bufio.NewWriter(&p.toClient)}
	return p
}

// frame is a client frame by hand: masked, with fin and the opcode.
func frame(fin bool, op byte, payload []byte) []byte {
	b := []byte{op, 0x80}
	if fin {
		b[0] |= 0x80
	}
	switch {
	case len(payload) < 126:
		b[1] |= byte(len(payload))
	default:
		b[1] |= 126
		b = binary.BigEndian.AppendUint16(b, uint16(len(payload)))
	}
	mask := []byte{1, 2, 3, 4}
	b = append(b, mask...)
	for i, c := range payload {
		b = append(b, c^mask[i%4])
	}
	return b
}

// closeCode is the code of the close frame the server sent last, or 0.
func (p *pipe) closeCode(t *testing.T) int {
	t.Helper()
	for {
		fin, op, payload, err := p.client.readFrame()
		if err != nil {
			return 0
		}
		if op == opClose && fin && len(payload) >= 2 {
			return int(binary.BigEndian.Uint16(payload))
		}
	}
}

// TestFrames (#326): a message in fragments is one message, a ping
// between them is answered, a close is echoed and ends the read, and
// a frame the transport refuses closes the connection with its code.
func TestFrames(t *testing.T) {
	t.Parallel()
	p := newPipe()
	p.toServer.Write(frame(false, opText, []byte(`{"jsonrpc":`)))
	p.toServer.Write(frame(true, opPing, []byte("hi")))
	p.toServer.Write(frame(false, opCont, []byte(`"2.0",`)))
	p.toServer.Write(frame(true, opCont, []byte(`"method":"view"}`)))
	long := bytes.Repeat([]byte("x"), 300) // a 16-bit length
	p.toServer.Write(frame(true, opText, long))
	p.toServer.Write(frame(true, opClose, binary.BigEndian.AppendUint16(nil, closeNormal)))
	msg, err := p.server.ReadMessage()
	if err != nil || string(msg) != `{"jsonrpc":"2.0","method":"view"}` {
		t.Fatalf("the fragments read %q, %v", msg, err)
	}
	if msg, err := p.server.ReadMessage(); err != nil || !bytes.Equal(msg, long) {
		t.Fatalf("the long message read %d bytes, %v", len(msg), err)
	}
	if _, err := p.server.ReadMessage(); !errors.Is(err, io.EOF) {
		t.Fatalf("the close read %v, want io.EOF", err)
	}
	if _, op, payload, err := p.client.readFrame(); err != nil || op != opPong || string(payload) != "hi" {
		t.Errorf("the ping's answer: op %#x %q %v", op, payload, err)
	}
	if code := p.closeCode(t); code != closeNormal {
		t.Errorf("the close echoed %d", code)
	}

	for _, c := range []struct {
		name  string
		frame []byte
		code  int
	}{
		{"unmasked", []byte{0x81, 0x02, 'h', 'i'}, closeProtocol},
		{"binary", frame(true, opBin, []byte{1}), closeData},
		{"not UTF-8", frame(true, opText, []byte{0xff, 0xfe}), closeUTF8},
		{"a continuation of nothing", frame(true, opCont, []byte("x")), closeProtocol},
		{"a reserved bit", append([]byte{0xC1}, frame(true, opText, []byte("x"))[1:]...), closeProtocol},
		{"a ping in pieces", frame(false, opPing, []byte("x")), closeProtocol},
		{"over 16 MiB", []byte{0x81, 0xFF, 0, 0, 0, 0, 0x02, 0, 0, 0}, closeTooBig},
		{"an unknown opcode", frame(true, 0x3, nil), closeProtocol},
	} {
		p := newPipe()
		p.toServer.Write(c.frame)
		if _, err := p.server.ReadMessage(); err == nil {
			t.Errorf("%s: read without an error", c.name)
		}
		if code := p.closeCode(t); code != c.code {
			t.Errorf("%s: closed with %d, want %d", c.name, code, c.code)
		}
	}
}

// TestServeWS (#326): what a request produces goes back a frame a line,
// the notifications before the response, as on stdio; the client's end
// reads them as the lines an RPCClient reads.
func TestServeWS(t *testing.T) {
	t.Parallel()
	p := newPipe()
	for _, req := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"new_run","params":[7,"",false]}`,
		`{"jsonrpc":"2.0","id":2,"method":"end_day"}`,
	} {
		if err := p.client.WriteMessage([]byte(req)); err != nil {
			t.Fatal(err)
		}
	}
	srv, err := NewServer(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	if err := ServeWS(p.server, srv); err != nil {
		t.Fatal(err)
	}
	var methods []string
	for {
		msg, err := p.client.ReadMessage()
		if err != nil {
			break
		}
		var m struct {
			Method string          `json:"method"`
			ID     json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal(msg, &m); err != nil {
			t.Fatalf("a frame that is not one JSON message: %q", msg)
		}
		if m.Method == "" {
			m.Method = "response " + string(m.ID)
		}
		if len(methods) == 0 || methods[len(methods)-1] != m.Method {
			methods = append(methods, m.Method)
		}
	}
	want := []string{"view", "response 1", "event", "view", "response 2"}
	if strings.Join(methods, ",") != strings.Join(want, ",") {
		t.Errorf("the frames came %v, want %v", methods, want)
	}
}
