// Command kingpin-client is the reference front end (#300): it starts
// kingpind, plays a run through the protocol alone (protocol.Play: a
// greedy dealer that buys, sells everything at the aggressive dial and
// answers every card with its first choice) until it ends, and prints
// each event as it arrived and how the run ended. It is the protocol's
// acceptance test in a form a person can run:
//
//	go build -o /tmp/kingpind ./cmd/kingpind
//	go run ./cmd/kingpin-client -server /tmp/kingpind -seed 7
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/theclifmeister/kingpin/internal/protocol"
)

func main() {
	server := flag.String("server", "kingpind", "the kingpind binary to start")
	seed := flag.Uint64("seed", 7, "the run's seed")
	days := flag.Int("days", 400, "the most days to play; the run usually ends first")
	quiet := flag.Bool("q", false, "print only how the run ended")
	flag.Parse()

	cmd := exec.Command(*server)
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		fail(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		fail(err)
	}
	if err := cmd.Start(); err != nil {
		fail(err)
	}
	c := protocol.NewRPCClient(in, out)
	v, err := protocol.Play(c, *seed, *days)
	if err != nil {
		fail(err)
	}
	if !*quiet {
		for _, raw := range c.Events() {
			var e struct {
				Kind string          `json:"kind"`
				Day  int             `json:"day"`
				Cue  json.RawMessage `json:"cue"`
			}
			if json.Unmarshal(raw, &e) != nil {
				continue
			}
			if len(e.Cue) > 0 {
				fmt.Printf("day %3d  %-20s cue %s\n", e.Day, e.Kind, e.Cue) // what a renderer animates (#301)
			} else {
				fmt.Printf("day %3d  %s\n", e.Day, e.Kind)
			}
		}
	}
	in.Close()
	_ = cmd.Wait()
	if v.Over == nil {
		fmt.Printf("still playing on day %d with %d events: no ending in %d days\n", v.Day, len(c.Events()), *days)
		os.Exit(2)
	}
	fmt.Printf("the run ended on day %d: %s (%d events, net worth $%d)\n", v.Over.Day, v.Over.Cause, len(c.Events()), v.You.NetWorth)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "kingpin-client:", err)
	os.Exit(1)
}
