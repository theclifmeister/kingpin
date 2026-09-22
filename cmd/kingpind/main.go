// Command kingpind serves the game to a front end in another process
// (#300, docs/engine.md): JSON-RPC 2.0 over stdin and stdout, one
// message a line, the methods and notifications in
// internal/protocol/schema.json. It has no run until the client calls
// new_run or load, and exits when stdin closes. Saves go where the
// game's do (KINGPIN_HOME).
package main

import (
	"fmt"
	"os"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/protocol"
)

func main() {
	cfg, err := content.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kingpind:", err)
		os.Exit(1)
	}
	srv, err := protocol.NewServer(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kingpind:", err)
		os.Exit(1)
	}
	if err := protocol.Serve(os.Stdin, os.Stdout, srv); err != nil {
		fmt.Fprintln(os.Stderr, "kingpind:", err)
		os.Exit(1)
	}
}
