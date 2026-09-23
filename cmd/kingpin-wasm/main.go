//go:build js && wasm

// Command kingpin-wasm is the engine as WebAssembly (#327,
// docs/engine.md), for a browser or a web game engine: the protocol
// with no process and no socket between it and the front end. Built
// with
//
//	GOOS=js GOARCH=wasm go build -o kingpin.wasm ./cmd/kingpin-wasm
//
// and run with Go's wasm_exec.js ($(go env GOROOT)/lib/wasm), it sets
// one global, kingpin:
//
//	kingpin.protocol      // protocol.Version
//	kingpin.view          // engine.ViewVersion
//	const s = kingpin.open()
//	s.handle(line)        // a request line in, the lines it produced out:
//	                      // an array of strings, the notifications first
//
// Each open is a session of its own. A browser has no save slots:
// export_save and import_save carry the save as base64 for the page to
// keep where it likes.
package main

import (
	"syscall/js"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/protocol"
)

func main() {
	open := js.FuncOf(func(js.Value, []js.Value) any {
		cfg, err := content.Load() // its own tuning, as each kingpind connection has
		if err != nil {
			return js.Global().Get("Error").New("kingpin: " + err.Error())
		}
		srv, err := protocol.NewServer(cfg)
		if err != nil {
			return js.Global().Get("Error").New("kingpin: " + err.Error())
		}
		handle := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) != 1 || args[0].Type() != js.TypeString {
				return js.Global().Get("Error").New("kingpin: handle takes one request line, a string")
			}
			lines := srv.Handle([]byte(args[0].String()))
			out := make([]any, len(lines))
			for i, l := range lines {
				out[i] = string(l)
			}
			return out
		})
		return map[string]any{"handle": handle}
	})
	js.Global().Set("kingpin", map[string]any{
		"protocol": protocol.Version,
		"view":     engine.ViewVersion,
		"open":     open,
	})
	select {} // the page calls in; the module lives as long as it does
}
