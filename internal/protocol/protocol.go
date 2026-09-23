// Package protocol serves an engine.Session to a front end in another
// process (#300, docs/engine.md): JSON-RPC 2.0, one message a line,
// over any reader and writer (cmd/kingpind wires stdin and stdout). A
// request names a method and passes its parameters by position; the
// server answers every request with a response, and before it sends the
// notifications the call caused: each event the day published
// (`event`), then the view (`view`) after any call that changed the run.
//
// The server is one loop on the caller's goroutine: it reads a line,
// runs it and writes what it produced, so it holds no lock and starts
// no goroutine (TestNoGoroutineInTheTree walks this package too).
package protocol

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
)

// Version is the protocol's: the methods, their parameters and the
// notifications. The view inside carries its own, engine.ViewVersion.
// 2 added the quotes, rules.<sim>.<method> (#325); 3 export_save and
// import_save (#327).
const Version = 3

// The error codes: JSON-RPC 2.0's, and the game's own in its
// server-error range.
const (
	CodeParse          = -32700 // the line is not JSON
	CodeInvalidRequest = -32600 // not a request
	CodeNoMethod       = -32601 // no such method
	CodeInvalidParams  = -32602 // the parameters do not fit the method
	CodeInternal       = -32603 // the engine panicked; the run may be damaged
	CodeRefused        = -32000 // the game refused the action: the message says why, in the game's words
	CodeNoRun          = -32001 // the method needs a run and there is none: new_run or load first
)

// Request is a call. An id is echoed in the response; a request without
// one is still answered, with a null id.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is the answer to a request: a result or an error.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is a refused or failed call.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return fmt.Sprintf("%s (%d)", e.Message, e.Code) }

// Refused reports whether err is the game saying no (CodeRefused): a
// move the rules do not allow, never a fault in the call.
func Refused(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == CodeRefused
}

// Notification is what the server sends unasked: `event` with an
// EventParams, `view` with an engine.View.
type Notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// EventParams is an event as the wire carries it: its kind (the
// events.Event Kind, stable), the day it happened, the event itself and,
// for one a front end animates, its cue (#301, engine.CueOf): the ids a
// renderer moves its sprites by.
type EventParams struct {
	Kind    string       `json:"kind"`
	Day     int          `json:"day"`
	Payload events.Event `json:"payload"`
	Cue     *engine.Cue  `json:"cue,omitempty"`
}

// eventParams is an event's params.
func eventParams(e events.Event, day int) EventParams {
	p := EventParams{Kind: e.Kind(), Day: day, Payload: e}
	if c, ok := engine.CueOf(e); ok {
		p.Cue = &c
	}
	return p
}

// EventJSON is an event as an `event` notification's params carry it,
// byte for byte: a client in the same process encodes its events with
// it to compare them with the wire's.
func EventJSON(e events.Event, day int) (json.RawMessage, error) {
	return json.Marshal(eventParams(e, day))
}

// Server is one session served.
type Server struct {
	sess    *engine.Session
	pending []any // the notifications the call in hand has caused
}

// NewServer builds a session from cfg and serves it. It has no run
// until a client calls new_run or load.
func NewServer(cfg *content.Config) (*Server, error) {
	sess, err := engine.New(cfg)
	if err != nil {
		return nil, err
	}
	s := &Server{sess: sess}
	sess.Subscribe(func(e events.Event) {
		// The clock publishes after it has moved the world to the tick's
		// day, so the world's day is the event's.
		s.pending = append(s.pending, Notification{JSONRPC: "2.0", Method: "event", Params: eventParams(e, sess.World().Day)})
	})
	return s, nil
}

// Session is the session the server serves.
func (s *Server) Session() *engine.Session { return s.sess }

// Handle runs one request line and returns the lines to send back, in
// order: the notifications it caused, then its response.
func (s *Server) Handle(line []byte) [][]byte {
	s.pending = nil
	resp := s.handle(line)
	var out [][]byte
	for _, n := range s.pending {
		out = append(out, mustMarshal(n))
	}
	s.pending = nil
	return append(out, mustMarshal(resp))
}

func (s *Server) handle(line []byte) (resp Response) {
	resp = Response{JSONRPC: "2.0", ID: json.RawMessage("null")}
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		resp.Error = &Error{Code: CodeParse, Message: "not JSON: " + err.Error()}
		return resp
	}
	if len(req.ID) > 0 {
		resp.ID = req.ID
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		resp.Error = &Error{Code: CodeInvalidRequest, Message: `not a JSON-RPC 2.0 request: want "jsonrpc": "2.0" and a method`}
		return resp
	}
	m, ok := methods[req.Method]
	if !ok {
		resp.Error = &Error{Code: CodeNoMethod, Message: "no method " + req.Method}
		return resp
	}
	if m.run && s.sess.World() == nil {
		resp.Error = &Error{Code: CodeNoRun, Message: "no run: call new_run or load first"}
		return resp
	}
	var params []json.RawMessage
	if len(req.Params) > 0 && string(req.Params) != "null" {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &Error{Code: CodeInvalidParams, Message: "params are positional: a JSON array"}
			return resp
		}
	}
	defer func() {
		if p := recover(); p != nil {
			resp.Result = nil
			resp.Error = &Error{Code: CodeInternal, Message: fmt.Sprint("the engine panicked: ", p)}
		}
	}()
	result, err := m.call(s, params)
	if err != nil {
		var e *Error
		if !errors.As(err, &e) {
			e = &Error{Code: CodeRefused, Message: err.Error()}
		}
		resp.Error = e
		return resp
	}
	raw, err := json.Marshal(result)
	if err != nil {
		resp.Error = &Error{Code: CodeInternal, Message: "the result does not encode: " + err.Error()}
		return resp
	}
	resp.Result = raw
	if m.changes && s.sess.World() != nil {
		s.pending = append(s.pending, Notification{JSONRPC: "2.0", Method: "view", Params: s.sess.View()})
	}
	return resp
}

// Serve reads requests from r a line at a time until it ends and writes
// what each produces to w, flushed after every request so a client that
// waits for its answer gets it.
func Serve(r io.Reader, w io.Writer, s *Server) error {
	in := bufio.NewScanner(r)
	in.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	out := bufio.NewWriter(w)
	for in.Scan() {
		if len(in.Bytes()) == 0 {
			continue
		}
		for _, line := range s.Handle(in.Bytes()) {
			if _, err := out.Write(append(line, '\n')); err != nil {
				return err
			}
		}
		if err := out.Flush(); err != nil {
			return err
		}
	}
	return in.Err()
}

func mustMarshal(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		raw, _ = json.Marshal(Response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &Error{Code: CodeInternal, Message: "does not encode: " + err.Error()}})
	}
	return raw
}
