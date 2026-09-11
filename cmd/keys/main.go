// Command keys prints the README's key table from the UI's key table
// (internal/ui/keys.go), so the two never disagree: `go run ./cmd/keys`
// prints the markdown, `go run ./cmd/keys -w` writes it into README.md
// between the `<!-- keys -->` markers. TestReadmeMatchesKeys holds the
// README to it.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/theclifmeister/kingpin/internal/ui"
)

func main() {
	write := flag.Bool("w", false, "write the table into README.md")
	path := flag.String("readme", "README.md", "the README to write")
	flag.Parse()
	table := ui.ReadmeKeys()
	if !*write {
		fmt.Print(table)
		return
	}
	b, err := os.ReadFile(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out, ok := ui.SpliceReadmeKeys(string(b), table)
	if !ok {
		fmt.Fprintf(os.Stderr, "%s: no %s / %s markers\n", *path, ui.ReadmeKeysBegin, ui.ReadmeKeysEnd)
		os.Exit(1)
	}
	if strings.TrimSpace(out) == strings.TrimSpace(string(b)) {
		return
	}
	if err := os.WriteFile(*path, []byte(out), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
