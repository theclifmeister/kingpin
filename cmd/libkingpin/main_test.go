//go:build cgo

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/protocol"
)

// replay is a C program linked against the library: it opens a session,
// hands it every request line on stdin and writes every line it
// answered to stdout, then reports the versions and what a closed
// handle answers on stderr.
const replay = `#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "libkingpin.h"

int main(void) {
	int h = kingpin_open();
	if (h <= 0) { fprintf(stderr, "no session\n"); return 1; }
	char *line = NULL;
	size_t cap = 0;
	ssize_t n;
	while ((n = getline(&line, &cap, stdin)) > 0) {
		if (line[n-1] == '\n') line[n-1] = 0;
		if (line[0] == 0) continue;
		char *answer = kingpin_handle(h, line);
		fputs(answer, stdout);
		kingpin_free(answer);
	}
	free(line);
	kingpin_close(h);
	char *gone = kingpin_handle(h, "{}");
	fprintf(stderr, "protocol %d view %d closed %s", kingpin_protocol(), kingpin_view(), gone);
	kingpin_free(gone);
	return 0;
}
`

// TestCIsTheEngine (#327): a C program linked against the library built
// with -buildmode=c-shared, handed the reference game's requests,
// answers the bytes the server in this process answered: every event,
// every view, every response. A closed handle answers an error, and the
// versions are the protocol's and the view's.
func TestCIsTheEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the library")
	}
	cc, err := exec.LookPath("cc")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("no cc on the runner: CI links the library")
		}
		t.Skip("no C compiler")
	}
	t.Parallel()
	dir := t.TempDir()
	lib := "libkingpin.so"
	if runtime.GOOS == "darwin" {
		lib = "libkingpin.dylib"
	}
	build := exec.Command("go", "build", "-buildmode=c-shared", "-o", filepath.Join(dir, lib), ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "replay.c")
	if err := os.WriteFile(src, []byte(replay), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "replay")
	if out, err := exec.Command(cc, "-o", bin, src, "-I", dir, "-L", dir, "-lkingpin", "-Wl,-rpath,"+dir).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s", err, out)
	}

	requests, want, err := protocol.Transcript(7, 400)
	if err != nil {
		t.Fatal(err)
	}
	run := exec.Command(bin)
	run.Env = append(os.Environ(), "KINGPIN_HOME="+t.TempDir())
	run.Stdin = bytes.NewReader(append(bytes.Join(requests, []byte("\n")), '\n'))
	var stdout, stderr bytes.Buffer
	run.Stdout, run.Stderr = &stdout, &stderr
	if err := run.Run(); err != nil {
		t.Fatalf("the C program: %v\n%s", err, stderr.String())
	}
	t.Logf("%d requests replayed, %d bytes answered", len(requests), stdout.Len())
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Errorf("the library answered %d bytes, the engine %d: not the same game", stdout.Len(), len(want))
	}
	wantTail := fmt.Sprintf("protocol %d view %d closed ", protocol.Version, engine.ViewVersion)
	if got := stderr.String(); !strings.HasPrefix(got, wantTail) || !strings.Contains(got, `"code":-32600`) {
		t.Errorf("the versions and a closed handle: %q", got)
	}
}
