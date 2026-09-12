package game

import (
	"errors"
	"io/fs"
	"math"
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
	w.AddStock("test", "a", 30, 0)
	if took := w.TakeStock("test", "a", 12); took != 12 || w.Stock("test", "a") != 18 {
		t.Fatalf("took %d, left %d; want 12 and 18", took, w.Stock("test", "a"))
	}
	if took := w.TakeStock("test", "a", 50); took != 18 || w.Stock("test", "a") != 0 {
		t.Fatalf("over-take took %d, left %d; want 18 and 0", took, w.Stock("test", "a"))
	}
	if took := w.TakeStock("test", "a", -5); took != 0 || w.Stock("test", "a") != 0 {
		t.Fatalf("a negative take took %d, left %d; want nothing", took, w.Stock("test", "a"))
	}
	w.AddStock("test", "a", 7, 0)
	w.AddStock("test", "a", -3, 0)
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
	w.AddStock("port", "z", 3, 0)
	if got := w.Stock("port", "z"); got != 3 {
		t.Fatalf("a new city's stash holds %d; want 3", got)
	}
	// The accessors guard a nil map, as a save from before the cities
	// leaves it.
	var bare World
	bare.AddStock("x", "y", 2, 0)
	if bare.TakeStock("x", "y", 1) != 1 || bare.Stock("x", "y") != 1 {
		t.Fatal("a nil stash was not made on first use")
	}
}

// TestLotQuality: a lot's quality (#47) is the mean by units of what
// came in, a take leaves it alone, an empty lot and a stash set by hand
// read the default, a zero quality in is the default, the figure never
// leaves 0..100, and a house-first landing and a street-first draw keep
// the one number the city holds.
func TestLotQuality(t *testing.T) {
	w := NewWorld(1, []StartingCity{{ID: "test", Name: "Test", Products: []StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}}}, 500, 100)
	if q := w.Quality("test", "a"); q != StreetQuality {
		t.Fatalf("an empty lot reads %v; want the default %v", q, StreetQuality)
	}
	w.BaseQuality = 60
	if q := w.Quality("test", "a"); q != 60 {
		t.Fatalf("an empty lot reads %v; want the stamped default 60", q)
	}
	w.AddStock("test", "a", 100, 40)
	if q := w.Quality("test", "a"); q != 40 {
		t.Fatalf("the first lot in reads %v; want 40", q)
	}
	w.AddStock("test", "a", 100, 80)
	if q := w.Quality("test", "a"); q != 60 {
		t.Fatalf("100 at 40 and 100 at 80 read %v; want 60", q)
	}
	w.TakeStock("test", "a", 150)
	if q := w.Quality("test", "a"); q != 60 {
		t.Fatalf("a take moved the quality to %v", q)
	}
	w.AddStock("test", "a", 50, 0) // zero is the default
	if q := w.Quality("test", "a"); q != 60 {
		t.Fatalf("50 at 60 into 50 at the default 60 read %v", q)
	}
	w.AddStock("test", "a", 100, 500)
	if q := w.Quality("test", "a"); q > 100 {
		t.Fatalf("quality left the scale: %v", q)
	}
	w.SetStock("test", "b", 10)
	if q := w.Quality("test", "b"); q != 60 {
		t.Fatalf("a stash set by hand reads %v; want the default", q)
	}
	// Houses (#73): a landing goes house-first and a draw street-first,
	// and the quality is the city's one number either way.
	w.Houses = append(w.Houses, House{ID: "h", City: "test", Capacity: 30})
	w.SetStock("test", "c", 0)
	w.AddStock("test", "c", 50, 20) // 30 in the house, 20 on the street
	w.AddStock("test", "c", 50, 80) // all on the street
	if q, st, h := w.Quality("test", "c"), w.Street("test", "c"), w.House("h").Stock["c"]; q != 50 || st != 70 || h != 30 {
		t.Fatalf("landing: quality %v street %d house %d; want 50, 70, 30", q, st, h)
	}
	w.TakeStock("test", "c", 80) // the street's 70, then 10 of the house
	if q, st, h := w.Quality("test", "c"), w.Street("test", "c"), w.House("h").Stock["c"]; q != 50 || st != 0 || h != 20 {
		t.Fatalf("draw: quality %v street %d house %d; want 50, 0, 20", q, st, h)
	}
	w.MoveStock("test", "h", Street, "c", 20)
	if q := w.Quality("test", "c"); q != 50 {
		t.Fatalf("a move within the city moved the quality to %v", q)
	}
}

// TestCut (#47): a cut adds exactly units x ratio units at nothing, so
// the quality falls by the ratio (the pure weight is conserved), costs
// the cut's units in dirty cash, is refused past the product's most,
// past the room the city has and with nothing to cut, and a chemist's
// bonus puts quality back, never over what it was.
func TestCut(t *testing.T) {
	w := NewWorld(1, []StartingCity{{ID: "test", Name: "Test", Products: []StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}}}, 5000, 1000)
	w.Player.Location = "test"
	w.AddStock("test", "a", 100, 60)
	if _, err := w.Cut("test", "a", 0.5, 0.4, 2, 0, ""); err == nil {
		t.Fatal("a cut past the most was allowed")
	}
	if _, err := w.Cut("test", "b", 0.5, 1, 2, 0, ""); err == nil {
		t.Fatal("a cut of nothing was allowed")
	}
	rec, err := w.Cut("test", "a", 0.5, 1, 2, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Added != 50 || w.Stock("test", "a") != 150 || w.Player.DirtyCash != 5000-100 {
		t.Fatalf("cut: added %d, stock %d, cash %d; want 50, 150, 4900", rec.Added, w.Stock("test", "a"), w.Player.DirtyCash)
	}
	if q := w.Quality("test", "a"); q != 40 || rec.From != 60 || rec.To != 40 {
		t.Fatalf("cut: quality %v (%v -> %v); want 40 (60 -> 40)", q, rec.From, rec.To)
	}
	if len(w.Today.Cuts) != 1 || w.Stats.Cut != 50 || w.Stats.CutCost != 100 {
		t.Fatalf("cut not recorded: %+v %+v", w.Today.Cuts, w.Stats)
	}
	// The chemist's hand: the cut keeps the bonus, never over the start.
	rec, err = w.Cut("test", "a", 0.5, 1, 2, 30, "Doc")
	if err != nil {
		t.Fatal(err)
	}
	if q := w.Quality("test", "a"); q != 40 || rec.Chemist != "Doc" {
		t.Fatalf("a chemist's cut of 40 by 0.5 with 30 back reads %v; want 40 (capped at what it was)", q)
	}
	rec, err = w.Cut("test", "a", 1, 1, 2, 5, "Doc")
	if err != nil {
		t.Fatal(err)
	}
	if q := w.Quality("test", "a"); math.Abs(q-25) > 1e-9 {
		t.Fatalf("a chemist's cut of 40 by 1.0 with 5 back reads %v; want 25", q)
	}
	// The room: a cut past what the city holds is refused.
	w.Player.CarryLimit = 10
	if _, err := w.Cut("test", "a", 1, 1, 2, 0, ""); err == nil {
		t.Fatal("a cut past the room was allowed")
	}
	w.Player.CarryLimit, w.Player.DirtyCash = 100000, 1
	if _, err := w.Cut("test", "a", 1, 1, 2, 0, ""); err == nil {
		t.Fatal("a cut the till cannot pay was allowed")
	}
}

// TestCookOrder (#47): a cook needs a chemist and a product the file
// cooks, is for at most a batch, one a product a city a day, refused
// past the room with what is on its way counted, paid on the order, and
// lands at the quality it was ordered at.
func TestCookOrder(t *testing.T) {
	w := NewWorld(1, []StartingCity{{ID: "test", Name: "Test", Products: []StartingProduct{{ID: "a", Name: "A", Price: 10, Demand: 5}}}}, 5000, 100)
	w.Player.Location = "test"
	if _, err := w.CookOrder("test", "a", 10, 30, 3, 70, 80, ""); !errors.Is(err, ErrNoChemist) {
		t.Fatalf("a cook with no chemist: %v", err)
	}
	if _, err := w.CookOrder("test", "a", 10, 0, 3, 70, 80, "Doc"); !errors.Is(err, ErrNotCooked) {
		t.Fatalf("a cook of a product nobody cooks: %v", err)
	}
	if _, err := w.CookOrder("test", "a", 90, 30, 3, 70, 80, "Doc"); !errors.Is(err, ErrBatch) {
		t.Fatalf("a cook past the batch: %v", err)
	}
	k, err := w.CookOrder("test", "a", 60, 30, 3, 70, 80, "Doc")
	if err != nil {
		t.Fatal(err)
	}
	if k.Ready != 3 || k.Quality != 70 || k.Cost != 1800 || w.Player.DirtyCash != 3200 || w.Crew.Cooking("test", "a") != 60 {
		t.Fatalf("cook: %+v, cash %d, cooking %d", k, w.Player.DirtyCash, w.Crew.Cooking("test", "a"))
	}
	if _, err := w.CookOrder("test", "a", 10, 30, 3, 70, 80, "Doc"); !errors.Is(err, ErrCooking) {
		t.Fatalf("a second cook today: %v", err)
	}
	w.Day = 1
	if _, err := w.CookOrder("test", "a", 60, 30, 3, 70, 80, "Doc"); err == nil {
		t.Fatal("a cook past the room, with the first on its way, was allowed")
	}
	if k, err := w.CookOrder("test", "a", 40, 30, 3, 70, 80, "Doc"); err != nil || k.ID != 2 {
		t.Fatalf("a cook that fits: %v %+v", err, k)
	}
}

// TestStashHasNoWriters: no source file outside game/world.go and
// game/houses.go writes into a stash map (#144). The map is Player.Stash
// (exported for gob, its name and type kept so no schema bump) and, since
// #73, a house's Stock, and since #47 the lot's quality beside it,
// Player.Quality; the only way in or out of them is AddStock,
// TakeStock, TakeStreet, TakeFromHouse, MoveStock, SetStock, Cut and
// SetQuality, so what they mean or hold changes in one place. The check
// is a grep over every non-test Go file under cmd/ and internal/: no
// element assignment through Player.Stash, Player.Quality, a
// Stash-derived map or a house's Stock (`[...] =`, `+=`, `-=`, `++`,
// `--`, `delete(`), whether written straight or through a local bound
// from one.
func TestStashHasNoWriters(t *testing.T) {
	root := filepath.Join("..", "..")
	var (
		// a local bound from the map: `stash := w.StashOf(c)`,
		// `s = w.Player.Stash[c]`, `for _, stash := range w.Player.Stash`
		bind = regexp.MustCompile(`(?:(\w+)\s*:?=\s*|for\s+\w+\s*,\s*(\w+)\s*:?=\s*range\s+)[\w.]*(?:Player\.Stash|Player\.Quality|StashOf\(|stash\(|quality\()`)
		// a direct element write: `Player.Stash[c][p] = n`, `StashOf(c)[p] += n`
		direct = regexp.MustCompile(`(?:Player\.Stash|Player\.Quality|StashOf\([^)]*\)|StreetOf\([^)]*\)|\.stash\([^)]*\)|\.quality\([^)]*\)|\.Stock)(?:\[[^\]]*\])+\s*(?:\+\+|--|[+\-*/]?=[^=])`)
		del    = regexp.MustCompile(`delete\(\s*[\w.]*(?:Player\.Stash|Player\.Quality|StashOf\([^)]*\)|StreetOf\([^)]*\)|\.stash\([^)]*\)|\.quality\([^)]*\)|\.Stock)`)
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
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || rel == filepath.Join("internal", "game", "world.go") || rel == filepath.Join("internal", "game", "houses.go") {
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
