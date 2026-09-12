package harness

import (
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// TestSeedDigest (#144): the boss on one pinned seed, sixty days, the
// whole world hashed at the end of every one of them. A change that
// moves any number, any day, fails here and names the first day it
// moved, so a stream shift (a draw added to the home stream, a sim
// slotted between two others, a seed roll moved) or a balance change
// reads as one diff and not as a dozen medians, and a shape change (a
// field moved, a constructor reshaped) that claims to be byte-for-byte
// the old run has a test that says so. The failure prints the new
// list to paste in: a PR that moves a number pins the new one and
// names the day and the reason.
//
// The digest walks the world by reflection in a fixed order (struct
// fields in declaration order, map keys sorted), hashing every
// exported value; a float is rounded to six decimals first, so the
// fused multiply-add an arm64 build is allowed to use (Go's spec
// permits it; amd64 does not) cannot move the hash between a laptop
// and CI while a real change to any number does.
func TestSeedDigest(t *testing.T) {
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, seedDigestSeed)
	_, sims, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	clock := game.NewClock(nil, sims...)
	policy := Boss(cfg, 40, "")
	var got []string
	for day := 1; day <= seedDigestDays; day++ {
		if w.Over != nil {
			break
		}
		policy(w)
		clock.EndDay(w)
		got = append(got, digest(w))
	}
	if len(seedDigest) != seedDigestDays {
		t.Fatalf("seed %d: %d days pinned, want %d; paste this in:\n\nvar seedDigest = []string{\n%s\n}", seedDigestSeed, len(seedDigest), seedDigestDays, list(got))
	}
	moved := -1
	for i := range seedDigest {
		if i >= len(got) || got[i] != seedDigest[i] {
			moved = i
			break
		}
	}
	if moved < 0 {
		return
	}
	was := "(the run ended)"
	if moved < len(got) {
		was = got[moved]
	}
	t.Errorf("seed %d: the world moved on day %d (digest %s, was %s); every later day follows. If the move is meant, name the day and the reason in the PR and paste this in:\n\nvar seedDigest = []string{\n%s\n}", seedDigestSeed, moved+1, was, seedDigest[moved], list(got))
}

// list renders the digests as the Go literal the test pins.
func list(ds []string) string {
	var b strings.Builder
	for i, d := range ds {
		if i%4 == 0 {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("\t")
		} else {
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "%q,", d)
	}
	return b.String()
}

// digest is the world's hash: FNV-1a over a walk of every exported
// value in a fixed order, floats to six decimals.
func digest(w *game.World) string {
	h := fnv.New64a()
	walk(h, reflect.ValueOf(w).Elem())
	return fmt.Sprintf("%016x", h.Sum64())
}

func walk(h interface{ Write([]byte) (int, error) }, v reflect.Value) {
	put := func(s string) { _, _ = h.Write([]byte(s)); _, _ = h.Write([]byte{0}) }
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			put("nil")
			return
		}
		walk(h, v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if f := v.Type().Field(i); f.IsExported() {
				put(f.Name)
				walk(h, v.Field(i))
			}
		}
	case reflect.Map:
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface()) })
		put(fmt.Sprintf("map%d", len(keys)))
		for _, k := range keys {
			put(fmt.Sprint(k.Interface()))
			walk(h, v.MapIndex(k))
		}
	case reflect.Slice, reflect.Array:
		put(fmt.Sprintf("len%d", v.Len()))
		for i := 0; i < v.Len(); i++ {
			walk(h, v.Index(i))
		}
	case reflect.Float32, reflect.Float64:
		put(fmt.Sprintf("%.6f", v.Float()))
	default:
		put(fmt.Sprint(v.Interface()))
	}
}

// seedDigestSeed is the run TestSeedDigest plays, seedDigestDays how
// far: sixty days is the boss into tier 3 with every sim awake (the
// crew, the road, the fronts, the rival's war and books, the law's
// first election), short enough to run in a second.
const (
	seedDigestSeed = 7
	seedDigestDays = 60
)

// seedDigest is the world after each of the boss's first sixty days on
// seedDigestSeed, pinned at 0d7a1d3 for #192 (three zero fields on the
// world, Front.Level, Invested and Grew, Stats.Earned and Invested,
// Today.Invested, move the hash from day 1; no number moved in the
// sixty days, the boss's first level comes later).
var seedDigest = []string{
	"ccc89093ccc2dba4", "ffba6d21eb4fde95", "d2ed882cc1cbe1a4", "00a3329199f91862",
	"c79d9975d41720ca", "80ad7b764b965088", "a88a242c9466bf7b", "efd2240fb7d2cb72",
	"6bb0454dc7aab9c1", "ff19651313dd2cf5", "d76c2211d5c79dd8", "003953313aead8bd",
	"4691b504f6f3aba7", "ab0fff92f16ec836", "9a222110c984a36b", "a98536ee0b84b503",
	"c35478c7ad9d912e", "ba62e65a25a6b95b", "6d0ff05893c40856", "3a97f81914f978f6",
	"e016fe2181c0f1f8", "45105f57d5b1eae3", "ba80bc9e4539f5d1", "ed3c47c57fd527da",
	"443fffb01850a82c", "be6177dcc71fe91e", "f875f939afe200b0", "17bc32a7899a8862",
	"1a1684e2dd2d6978", "32424aef8179a43d", "7e2774dbabd63906", "e1e1729da262a3d6",
	"47d06eaf2499139a", "a573320b95329de9", "220a4e3b6bf4b254", "85890f85c75775d8",
	"8abb0dbf79413d08", "dec5cf20f7903c99", "d453e0c74290bc26", "310e17421899752a",
	"8b4b09f2d59a5012", "2f3bd5db2da1d757", "36d6ad7e952a4196", "9e763d09b0214ce5",
	"c543a7b0ea7ab9fc", "8d71a99d0bb497b0", "b0879f37e22c278c", "6d051c11c9c3f2c2",
	"e8d4ff8f987e805a", "0ca4a1e72a8d69b3", "332a438a128a75bf", "0bfe90e81dc1e195",
	"c6081861294f52e2", "22a703aa4edcd928", "90a7b3707147c800", "d9e9870a5249dc72",
	"2ba543228a77a5b5", "e9f03fdc359dafe1", "1d928299ff9d78ab", "4eee3e24ca61db08",
}
