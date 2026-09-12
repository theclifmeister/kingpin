package game

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestStockAccessors: AddStock and TakeStock are the stash's only
// writers (#144): a take is clamped at what is there and returns what it
// took, an add of a negative count is a take, SetStock puts a figure
// there for a test or a migration, and StashOf is a copy that changes
// nothing when written to.
func TestStockAccessors(t *testing.T) {
	w := NewWorld(1, []StartingCity{{ID: "test", Name: "Test", Products: []StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}}}, 500, 100)
	if got := w.Stock("test", "a"); got != 0 {
		t.Fatalf("a fresh stash holds %d", got)
	}
	w.AddStock("test", "a", 30)
	if took := w.TakeStock("test", "a", 12); took != 12 || w.Stock("test", "a") != 18 {
		t.Fatalf("took %d, left %d; want 12 and 18", took, w.Stock("test", "a"))
	}
	if took := w.TakeStock("test", "a", 50); took != 18 || w.Stock("test", "a") != 0 {
		t.Fatalf("over-take took %d, left %d; want 18 and 0", took, w.Stock("test", "a"))
	}
	if took := w.TakeStock("test", "a", -5); took != 0 || w.Stock("test", "a") != 0 {
		t.Fatalf("a negative take took %d, left %d; want nothing", took, w.Stock("test", "a"))
	}
	w.AddStock("test", "a", 7)
	w.AddStock("test", "a", -3)
	if got := w.Stock("test", "a"); got != 4 {
		t.Fatalf("add then negative add left %d; want 4", got)
	}
	w.SetStock("test", "a", -9)
	if got := w.Stock("test", "a"); got != 0 {
		t.Fatalf("SetStock under zero left %d; want 0", got)
	}
	w.SetStock("test", "a", 25)
	view := w.StashOf("test")
	view["a"] = 999
	view["b"] = 1
	if got := w.Stock("test", "a"); got != 25 || w.Stock("test", "b") != 0 {
		t.Fatalf("writing the copy reached the stash: a %d, b %d", got, w.Stock("test", "b"))
	}
	// A city and a product the world has never heard of are a stash too:
	// the map is made on first use, as the road's landing needs.
	w.AddStock("port", "z", 3)
	if got := w.Stock("port", "z"); got != 3 {
		t.Fatalf("a new city's stash holds %d; want 3", got)
	}
	// The accessors guard a nil map, as a save from before the cities
	// leaves it.
	var bare World
	bare.AddStock("x", "y", 2)
	if bare.TakeStock("x", "y", 1) != 1 || bare.Stock("x", "y") != 1 {
		t.Fatal("a nil stash was not made on first use")
	}
}

// TestStashHasNoWriters: no source file outside game/world.go writes into
// a stash map (#144). The map is Player.Stash (exported for gob, its name
// and type kept so no schema bump); the only way in or out of it is
// AddStock, TakeStock and SetStock in world.go, so #73 and #47 can change
// what it means or holds in one place. The check is a grep over every
// non-test Go file under cmd/ and internal/: no element assignment
// through Player.Stash or a Stash-derived map (`[...] =`, `+=`, `-=`,
// `++`, `--`, `delete(`), whether written straight or through a local
// bound from one.
func TestStashHasNoWriters(t *testing.T) {
	root := filepath.Join("..", "..")
	var (
		// a local bound from the map: `stash := w.StashOf(c)`,
		// `s = w.Player.Stash[c]`, `for _, stash := range w.Player.Stash`
		bind = regexp.MustCompile(`(?:(\w+)\s*:?=\s*|for\s+\w+\s*,\s*(\w+)\s*:?=\s*range\s+)[\w.]*(?:Player\.Stash|StashOf\(|stash\()`)
		// a direct element write: `Player.Stash[c][p] = n`, `StashOf(c)[p] += n`
		direct = regexp.MustCompile(`(?:Player\.Stash|StashOf\([^)]*\)|\.stash\([^)]*\))(?:\[[^\]]*\])+\s*(?:\+\+|--|[+\-*/]?=[^=])`)
		del    = regexp.MustCompile(`delete\(\s*[\w.]*(?:Player\.Stash|StashOf\([^)]*\)|\.stash\([^)]*\))`)
	)
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if rel != "." && !strings.HasPrefix(rel, "cmd") && !strings.HasPrefix(rel, "internal") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || rel == filepath.Join("internal", "game", "world.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		locals := map[string]bool{}
		for i, l := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "//") {
				continue
			}
			for _, m := range bind.FindAllStringSubmatch(l, -1) {
				for _, name := range m[1:] {
					if name != "" && name != "_" {
						locals[name] = true
					}
				}
			}
			bad := direct.MatchString(l) || del.MatchString(l)
			for name := range locals {
				if regexp.MustCompile(`\b`+name+`(?:\[[^\]]*\])+\s*(?:\+\+|--|[+\-*/]?=[^=])`).MatchString(l) || regexp.MustCompile(`delete\(\s*`+name+`\s*,`).MatchString(l) {
					bad = true
				}
			}
			if bad {
				offenders = append(offenders, rel+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(l))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("stash written outside game/world.go (use AddStock / TakeStock):\n  %s", strings.Join(offenders, "\n  "))
	}
}
