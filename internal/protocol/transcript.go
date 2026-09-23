package protocol

import (
	"bytes"
	"io"

	"github.com/theclifmeister/kingpin/internal/content"
)

// Transcript is the reference game (Play) on a server in this process,
// recorded: every request line the client sent, in order, and every line
// the server answered, each ending in a newline. The game's moves depend
// only on what the server answered, so an embedding of the engine that
// is handed the same requests must answer the same bytes (#327): the
// WASM and the C builds are checked by replaying it.
func Transcript(seed uint64, days int) (requests [][]byte, answers []byte, err error) {
	srv, err := NewServer(content.MustLoad())
	if err != nil {
		return nil, nil, err
	}
	rec := &recorder{srv: srv}
	if _, err := Play(NewRPCClient(rec, &rec.out), seed, days); err != nil {
		return nil, nil, err
	}
	return rec.requests, rec.answers.Bytes(), nil
}

// recorder hands each request line to the server and keeps it, and what
// the server answered for the client to read and for the transcript.
type recorder struct {
	srv      *Server
	requests [][]byte
	out      bytes.Buffer // what the client has yet to read
	answers  bytes.Buffer // every answer
}

var _ io.Writer = (*recorder)(nil)

func (r *recorder) Write(p []byte) (int, error) {
	line := bytes.TrimRight(p, "\n")
	r.requests = append(r.requests, bytes.Clone(line))
	for _, a := range r.srv.Handle(line) {
		r.out.Write(a)
		r.out.WriteByte('\n')
		r.answers.Write(a)
		r.answers.WriteByte('\n')
	}
	return len(p), nil
}
