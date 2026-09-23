// Command kingpin-web is the graphical client (#328, docs/web.md): the
// game drawn in a browser, sprites on a canvas animated by the engine's
// cues, played through the protocol alone. The page runs the engine
// itself as WebAssembly (#327), so it needs no server once it is built:
//
//	go run ./cmd/kingpin-web                 # build it and serve it on http://127.0.0.1:8080/
//	go run ./cmd/kingpin-web -addr :9000     # elsewhere
//	go run ./cmd/kingpin-web -out site/      # write the static site and exit
//
// The site is the files under web/ (embedded), kingpin.wasm built from
// ./cmd/kingpin-wasm and Go's wasm_exec.js, which must come from the
// toolchain that built the module.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed web
var web embed.FS

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "where to serve the site")
	out := flag.String("out", "", "write the static site to this directory and exit")
	flag.Parse()
	dir := *out
	if dir == "" {
		tmp, err := os.MkdirTemp("", "kingpin-web")
		if err != nil {
			fail(err)
		}
		defer os.RemoveAll(tmp)
		dir = tmp
	}
	if err := Build(dir); err != nil {
		fail(err)
	}
	if *out != "" {
		fmt.Println("the site is in", dir)
		return
	}
	host := *addr
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	fmt.Printf("kingpin: http://%s/\n", host)
	if err := http.ListenAndServe(*addr, http.FileServer(http.Dir(dir))); err != nil {
		fail(err)
	}
}

// Build writes the site to dir: the page, the module and its loader.
func Build(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	site, err := fs.Sub(web, "web")
	if err != nil {
		return err
	}
	// os.CopyFS refuses a file already there; a rebuild overwrites.
	err = fs.WalkDir(site, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		to := filepath.Join(dir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		b, err := fs.ReadFile(site, path)
		if err != nil {
			return err
		}
		return os.WriteFile(to, b, 0o644)
	})
	if err != nil {
		return err
	}
	build := exec.Command("go", "build", "-o", filepath.Join(dir, "kingpin.wasm"), "github.com/theclifmeister/kingpin/cmd/kingpin-wasm")
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("building the module: %w", err)
	}
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return err
	}
	loader, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(goroot)), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "wasm_exec.js"), loader, 0o644)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "kingpin-web:", err)
	os.Exit(1)
}
