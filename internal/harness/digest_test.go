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
// seedDigestSeed, pinned at 75bd4d0 (#144 PR 4: the shape changes of
// the issue, no number moved) and again for #193 (City.Campaign,
// DA.Backed, Law.CampaignOpen, Today.Backed and three Stats added to
// the walk; no number moved: the digest with the new fields skipped was
// the old one on every day, the boss having no clean cash to back a
// ticket with by day 60), and again for #44 (the world's incidents,
// table boxed: day 1 moved by shape alone, five zero fields added,
// World.Incidents, RouteSetting.ClosedUntil, HeatState.FederalUntil and
// FederalDecay, LawState.SnapElection and DayReport.Incident; day 25 by
// the wording of a headline that now names the chief, PressureShiftedUp's
// third; no number moved, checked field by field against d51abd1), and
// again for #47 (Player.Quality, World.BaseQuality, Corner.Repeat,
// Shipment.Quality, Supplier.Quality, Crew.Cooks and NextCook,
// Today.Cuts and five Stats added to the walk, the chemist beside the
// pool's faces once meth lists; the move is on day 1, the day the
// default quality and repeat_start are stamped, and no money number
// moved: t1 and t2 stand to the dollar and the boss's rows stand under
// #193's figures), and again for #192 (Front.Level, Invested and Grew,
// Stats.Earned and Invested, Today.Invested; no number moved: the boss's
// first level comes after day 60 on this seed).
// Again for #42 (the bought law on LawState, Route.Bought, Today.Bribes
// and Checkpoints, six Stats added to the walk; no number moved: with
// those fields skipped the digest is 0ab8bc7's on all sixty days, the
// boss paying nobody). Again for #195 (World.Offshore and QuietDays,
// Today.Reserved, Laundering.Structured, Stats.Reserved and Fees; one
// number moves from day 1: QuietDays, the count of quiet days in a row,
// which every run keeps and nothing but Retire reads; the boss reserves
// nothing by day 60 on this seed, holding under a campaign's worth).
var seedDigest = []string{
	"039175d3b483a140", "fc46d36cec2ee8e5", "fe46b9bdae9bfb9d", "53e05e04d006e3a9",
	"8d2e7baa58042923", "255669caa3876307", "5d9f81e9f2886a0f", "3c84821130994e14",
	"b97d3737da12f76b", "cdb70c8044569655", "68a15ac5c26d86f6", "0a3d2b68f7213c77",
	"69e0f3b4f805d8ed", "e37cb7067282af60", "22e9192c3997dff8", "53b835ed64231591",
	"05a35b8893c734f7", "9ddd0138e2c26fdb", "c54f0f37f628f104", "a67aad79fb5380e7",
	"ce93db38fcff48c2", "07bf236ec6a41913", "650d00750501ad8f", "599c07142485146c",
	"30f61951c016eaf4", "311322e1da953918", "f71c7db0d91a69d8", "5934b0268ea7c75c",
	"ba08f7ff466cec54", "0b8dfc238b7e151c", "991ee1cbb230e423", "7bed89a20af443dc",
	"83b7e75d250b6fa9", "c7380b39f51c10f2", "96ec3c15484aa986", "1b32c49e2a8cb9d2",
	"67eb26832eba20a3", "bbd9cb6bc0218e3f", "68e7d7e1c60ae4d7", "2e4a907e491d0a84",
	"5d289818d629387c", "11a4e9c94b74de7d", "5c2132fbba26cacd", "dc751d3cdb96a21c",
	"15bc0b8e79e3f497", "f775e0b7ebd836c0", "41b3b364e3de4dd8", "80c1963c932c90d4",
	"1cb6d0b9257cc08a", "433ef3c7af3da9a1", "7001204c71bbfebf", "a5ca02e650b12415",
	"4f8b1cdea76c6df8", "b41883a6166f8b30", "6fafe90669eb03fd", "fd59d570810223bf",
	"dfa110ad96ca6a70", "b4a6b8fb38d3f9c1", "c2976d7e8aa9bb22", "e30642d9734421cc",
}
