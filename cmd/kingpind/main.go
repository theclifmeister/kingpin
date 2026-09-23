// Command kingpind serves the game to a front end in another process
// (#300, docs/engine.md): JSON-RPC 2.0, the methods and notifications in
// internal/protocol/schema.json. By default it speaks over stdin and
// stdout, one message a line, and exits when stdin closes. With -listen
// it serves WebSocket instead (#326), one message a text frame, on a
// loopback address: it prints the URL to connect to on stdout, serves
// one connection at a time, each its own session, and runs until it is
// killed. Either way a session has no run until the client calls
// new_run or load. Saves go where the game's do (KINGPIN_HOME).
//
//	kingpind                          # stdio
//	kingpind -listen 127.0.0.1:7777   # ws://127.0.0.1:7777/
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/protocol"
)

func main() {
	listen := flag.String("listen", "", "serve WebSocket on this loopback address (host:port; port 0 picks one) instead of stdio")
	flag.Parse()
	if *listen != "" {
		fail(serveWS(*listen)) // it serves until it is killed, or fails
	}
	cfg, err := content.Load()
	if err != nil {
		fail(err)
	}
	srv, err := protocol.NewServer(cfg)
	if err != nil {
		fail(err)
	}
	if err := protocol.Serve(os.Stdin, os.Stdout, srv); err != nil {
		fail(err)
	}
}

// serveWS listens on addr and serves each connection in turn, a fresh
// session each, and returns only when it cannot go on. The address
// must be loopback: the protocol has no authentication, and remote play
// is not what it is for.
func serveWS(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host == "" {
		host = "127.0.0.1"
		addr = host + addr
	}
	if ip := net.ParseIP(host); !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("-listen %s: kingpind serves this machine alone (a loopback address): the protocol has no authentication", addr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Printf("ws://%s/\n", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		if err := serveConn(conn); err != nil {
			fmt.Fprintln(os.Stderr, "kingpind:", err)
		}
	}
}

// serveConn is one client, from the handshake until it hangs up.
func serveConn(conn net.Conn) error {
	defer conn.Close()
	// A client gets ten seconds to finish the handshake, so one that
	// connects and says nothing cannot hold the line.
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	ws, err := protocol.AcceptWS(bufio.NewReader(conn), conn)
	if err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Time{})
	cfg, err := content.Load()
	if err != nil {
		return err
	}
	srv, err := protocol.NewServer(cfg)
	if err != nil {
		return err
	}
	if err := protocol.ServeWS(ws, srv); err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "kingpind:", err)
	os.Exit(1)
}
