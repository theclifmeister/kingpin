//go:build cgo

// Command libkingpin is the engine as a C shared library (#327,
// docs/engine.md), for a native game engine (Godot, Unity, Unreal) that
// loads it in its own process: the protocol with no process and no
// socket between it and the front end. Built with
//
//	go build -buildmode=c-shared -o libkingpin.so ./cmd/libkingpin
//
// (libkingpin.dylib on macOS, kingpin.dll on Windows), it writes
// libkingpin.h beside it. The surface is four calls:
//
//	int   kingpin_open(void);                  // a session: its handle, > 0
//	char *kingpin_handle(int h, const char *line);
//	                                           // a request line in; the lines it
//	                                           // produced out, each ending in '\n',
//	                                           // the notifications first
//	void  kingpin_free(char *answer);          // every answer, once read
//	void  kingpin_close(int h);                // the session gone
//
// and kingpin_protocol() and kingpin_view(), the versions. The strings
// are the wire's, byte for byte, in UTF-8. A handle that is not open
// answers a JSON-RPC error line. The calls are safe from any thread: one
// lock serialises them, since a session is one run and steps one call at
// a time.
package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/protocol"
)

var (
	mu       sync.Mutex // a host may call from any thread
	sessions = map[C.int]*protocol.Server{}
	next     C.int
)

//export kingpin_open
func kingpin_open() C.int {
	cfg, err := content.Load()
	if err != nil {
		return 0
	}
	srv, err := protocol.NewServer(cfg)
	if err != nil {
		return 0
	}
	mu.Lock()
	defer mu.Unlock()
	next++
	sessions[next] = srv
	return next
}

//export kingpin_handle
func kingpin_handle(h C.int, line *C.char) *C.char {
	mu.Lock()
	defer mu.Unlock()
	srv, ok := sessions[h]
	if !ok {
		return C.CString(`{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"no session with that handle: kingpin_open first"}}` + "\n")
	}
	var out []byte
	for _, l := range srv.Handle([]byte(C.GoString(line))) {
		out = append(append(out, l...), '\n')
	}
	return C.CString(string(out))
}

//export kingpin_free
func kingpin_free(answer *C.char) { C.free(unsafe.Pointer(answer)) }

//export kingpin_close
func kingpin_close(h C.int) {
	mu.Lock()
	defer mu.Unlock()
	delete(sessions, h)
}

//export kingpin_protocol
func kingpin_protocol() C.int { return C.int(protocol.Version) }

//export kingpin_view
func kingpin_view() C.int { return C.int(engine.ViewVersion) }

// main is required of a c-shared build and never runs.
func main() {}
