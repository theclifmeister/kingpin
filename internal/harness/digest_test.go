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
// seedDigestSeed, pinned at 75bd4d0 (#144 PR 4: the shape changes of
// the issue, no number moved).
var seedDigest = []string{
	"e90fbd1fbbf62c22", "19df1355b08f9597", "9154d884c0da1ea0", "db154f3e5342aeca",
	"420c9b23568cd03e", "94e8497eee162a42", "4336738d6ff6aefd", "d1f1cebb131f5552",
	"b2dd15097d1b4293", "e2de70949848be75", "2aa5736f4b0062ec", "6709c6a31686580d",
	"ec9cb299a2028226", "841a8860285021b5", "42a21726b95cea22", "98dd699eabeba272",
	"d8c2e0a32701e6a3", "0a783eebea47c706", "81191270c8cc1817", "ec3c00947e4c22bf",
	"19e5f568f87cab99", "097990e7da8d1b12", "6d09f6b97e1d4418", "7d01372c6ada777f",
	"818d542c72d35cfd", "b1e55301d3fbfe6c", "8d291524c4c7d2ce", "5a6bac84b4fcaf94",
	"7622c215e2ccfc6e", "2598371449fb3c29", "2755ec2f0e7293c0", "175269f6366b56c4",
	"0ecf26f8087baf00", "42b949c5697d5a25", "8dcf881903c4b942", "db9300d420a99bc2",
	"57a5ec311e82b788", "dc4f439b6d44d987", "2796bc69f732716c", "7c41e7f2c762ae1e",
	"ca15906ccae3b9d0", "d44479fd39157961", "4d3279705dadcd72", "f29a45504f275af3",
	"b3a85b37f00db54e", "51b07fc6f15e9ca8", "57ba18fa5c54a8fc", "9d9287f7d079a326",
	"aff3a4088f35834a", "0c17ed94c9436c0d", "3ca4825460d4fd8f", "1939204831e99937",
	"a20a49fe81f2cd64", "9ae63a0a1aba7620", "8d8318718984b864", "ad3a3422e437be20",
	"2cf633a0ea3f9187", "568df82e4cf17487", "fc671b51589200a3", "420afd827558d624",
}
