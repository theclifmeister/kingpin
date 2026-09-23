//go:build !js

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/protocol"
)

// replay is a Node script: it loads the module with Go's wasm_exec.js,
// opens a session, hands it every request line of the transcript and
// writes every line it answered, then checks the globals.
const replay = `
"use strict";
const fs = require("fs");
const [wasmExec, wasm, requests, answers] = process.argv.slice(2);
globalThis.require = require;
globalThis.fs = fs;
globalThis.TextEncoder ??= require("util").TextEncoder;
globalThis.TextDecoder ??= require("util").TextDecoder;
globalThis.performance ??= require("perf_hooks").performance;
globalThis.crypto ??= require("crypto");
require(wasmExec);
const go = new Go();
WebAssembly.instantiate(fs.readFileSync(wasm), go.importObject).then((r) => {
	go.run(r.instance);
	const k = globalThis.kingpin;
	const s = k.open();
	const out = [];
	for (const line of fs.readFileSync(requests, "utf8").split("\n")) {
		if (line === "") continue;
		for (const a of s.handle(line)) out.push(a + "\n");
	}
	fs.writeFileSync(answers, out.join(""));
	const bad = k.open().handle(42);
	console.log(JSON.stringify({protocol: k.protocol, view: k.view, refused: bad instanceof Error}));
	process.exit(0);
}).catch((e) => { console.error(e); process.exit(1); });
`

// TestWASMIsTheEngine (#327): the module built for js/wasm and run under
// Node, handed the reference game's requests, answers the bytes the
// server in this process answered: every event, every view, every
// response. It carries the protocol's and the view's versions and
// refuses a request that is not a string.
func TestWASMIsTheEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the module")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("no node on the runner: CI replays the module under it")
		}
		t.Skip("no node to run the module")
	}
	t.Parallel()
	dir := t.TempDir()
	wasm := filepath.Join(dir, "kingpin.wasm")
	build := exec.Command("go", "build", "-o", wasm, ".")
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatal(err)
	}
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatal(err)
	}
	wasmExec := filepath.Join(strings.TrimSpace(string(goroot)), "lib", "wasm", "wasm_exec.js")

	requests, want, err := protocol.Transcript(7, 400)
	if err != nil {
		t.Fatal(err)
	}
	reqs := filepath.Join(dir, "requests")
	if err := os.WriteFile(reqs, append(bytes.Join(requests, []byte("\n")), '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "replay.js")
	if err := os.WriteFile(script, []byte(replay), 0o644); err != nil {
		t.Fatal(err)
	}
	answers := filepath.Join(dir, "answers")
	run := exec.Command(node, script, wasmExec, wasm, reqs, answers)
	run.Env = append(os.Environ(), "KINGPIN_HOME="+t.TempDir())
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	got, err := os.ReadFile(answers)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("the module answered %d bytes, the engine %d: not the same game", len(got), len(want))
	}
	t.Logf("%d requests replayed, %d bytes answered alike", len(requests), len(got))
	if len(requests) < 50 {
		t.Errorf("only %d requests: the reference game is not being played", len(requests))
	}
	wantGlobals := `{"protocol":` + strconv.Itoa(protocol.Version) + `,"view":` + strconv.Itoa(engine.ViewVersion) + `,"refused":true}`
	if strings.TrimSpace(string(out)) != wantGlobals {
		t.Errorf("the globals: %s, want %s", out, wantGlobals)
	}
}
