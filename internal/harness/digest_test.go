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
	cfg.Incidents.Table = nil // the weather stays boxed (#44), as it is in every harness run: the digest pins the sims
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
// seedDigestSeed, pinned at #44 (the world's incidents, table boxed:
// day 1 moved by shape alone, five zero fields added, World.Incidents,
// RouteSetting.ClosedUntil, HeatState.FederalUntil and FederalDecay,
// LawState.SnapElection and DayReport.Incident; day 25 by the wording
// of a headline that now names the chief, PressureShiftedUp's third;
// no number moved, checked field by field against 0d7a1d3).
var seedDigest = []string{
	"db63512d3e5f66a0", "7487c2c1bf408eff", "f3fe6268ca995d8c", "95a698a372f74434",
	"87750d2b4b68599c", "c5b293bcfe64e536", "f0fbb2285d211cf9", "43cde4fc0d00b98a",
	"5353172c2da865bd", "9d2b765574aac4c5", "e0a4c6b1413efe24", "0e49a325ab9b337d",
	"eb671f21ec89a19a", "700e39edc4d27735", "c1e12c2025ca1bb8", "6962309a9f65f8e8",
	"f25c5cf93174fec5", "b24565713ce4b1ca", "f40605f80bbb4fa1", "f1564e25f83af639",
	"4023233dbc4098dd", "a4e48d33632c9e36", "6cea8d8b693768d8", "2b584616668b8af7",
	"32a428424f9fc48f", "de3ca01c643dec8f", "ce44361577946839", "62b04f77069e3dca",
	"6260e9a7f52039d4", "827606081b50a5b9", "2af900dc5622b996", "23072bcca450f60a",
	"faf8d8a3b8bdadf6", "7e46da9a1e586793", "e10529a57bc5be40", "c53d896a44c80336",
	"e20eee559a2f3f8c", "01c35f00a4ff0073", "da33a54acaf2f69c", "10aa6cf5de41883c",
	"506fbbda94c35eb0", "a5a139e0207af171", "fb78386800c10a50", "148b47c16a9f78b7",
	"de8e1e0fa8a30ca2", "931652625117d542", "b404f02b5c416a78", "35895cc8f2c5d0f0",
	"b0d54b8ae007cda4", "e7b168bf1670e97b", "e59dfd6a0d908381", "b5d61b96d1a3aab9",
	"0d71109b8657dcfc", "e95e737edcdfc45e", "34f1cc02445499ea", "4a55a7bf183cd0d4",
	"6e3ecc32a8d39941", "e9694c2e237d4741", "fdda989d9a302efd", "433f03c40237be6e",
}
