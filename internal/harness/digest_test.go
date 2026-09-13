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
	t.Parallel()
	// The duel (#43, harness.OneFaction): the digest pins the sims on the
	// one rival's dice, the run every number before #43 was pinned on;
	// TestOneFactionIsTheOldRun says the duel is byte-for-byte the old
	// run, and this is where a moved number names its day.
	cfg := OneFaction(content.MustLoad())
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
// Again for #43 (the table, on the duel's dice, harness.OneFaction:
// World.Rival became the slice World.Rivals of one, RivalState gained
// Home, Grudges, Trusts, Ally, Against, LostToYou, LastTakenBy,
// Absorbed, AbsorbedBy, Fragmented and Fragments, Deal, Offer,
// ScoutOrder, PoachOrder and Lead a Faction, CrewMember a Former and
// four Stats; the move is on day 1 by shape alone and no number moved:
// cmd/balance -factions 1 prints main's trace to the dollar on every
// day of five policies over three seeds, TestOneFactionIsTheOldRun).
// Again for #48 (World.Assets and AssetsLost, Today.AssetsBought,
// HeatState.TaskForceDay, WatchUntil, LineUntil and LineMul,
// Supplier.Owned, ShockUntil and ShockMul, six Stats added to the walk;
// one number moves from day 1: Stats.PeakClean, the clean high-water
// mark the assets unlock on, a copy of a maximum every run already
// held; and the report's tier lines reword twice, `tier 2 of 5` on day
// 2 and tier 4's `Next` on day 54, the day this seed enters
// Distribution; with those set aside the digest is aea739e's on all
// sixty days before #43 and 8b22ab1's after it, so no money number
// moved: TestNoAssetIsTheOldRun reads the same day by day to the
// tier-4 checkpoint; re-pinned on the merged tree).
// Again for #194 (Corner.Deed, Today.DeedsBought, Law.Forfeited and
// four Stats; the move is on day 1 by shape alone and no number moved:
// the boss buys its first deed after day 100 on this seed, and
// cmd/balance -deeds off prints 74a0b60's trace to the dollar on boss,
// cartel and distributor over three seeds to day 200, with deeds on
// the boss's trace holding to day 125 at the earliest).
// Again for #45 (World.Intel, Today.Cop and Spy, CrewMember.Undercover
// and UndercoverDay, DayReport.Intel and eight Stats added to the walk,
// RivalState.Known taken out of it and SchemaVersion 16; the move is on
// day 1 by shape alone and no number moved: with those fields set aside
// on both sides the digest is dff4a0a's on all sixty days, the boss
// paying no cop and planting nobody, and the facts the night writes off
// its own events living in Intel alone).
var seedDigest = []string{
	"8f3fd5637f283009", "2e7641631daee8e3", "74d404f94f4523c1", "3b6a962d7fb6e2db",
	"13ac34c7f9933402", "298a6b9aa9c389c0", "d5a21d8a63f14124", "9bf2406180ab378e",
	"120b476436a759e2", "665d3c349533e335", "69806a1bce745e9e", "273db5acae20e9ec",
	"881210255f020f76", "fc53575388a93de1", "3ae127c75e343a45", "958918d0a8dc26d9",
	"55b78e9fbdc0ac3d", "553efea26b036022", "5c5435f710c51fee", "60cdbfb523e87000",
	"b4c30ee21258736c", "5280c6ef05e003dd", "85aa18c39bdfc914", "4acfb08bc75e8797",
	"0c52d2689e64f9af", "b75f390bf6ff0e65", "dbd59f77b0287687", "e4bf7bf4d67ebc20",
	"d871a351a9abbd41", "a4e59cf2b3a5d182", "d590b207c4820f5b", "88592b7af0dbbdfb",
	"1e06b17c39f8a9d0", "14ab8f2e84487e15", "e92271e48126ad8a", "12381ea0b7e57bbc",
	"fdb61cbce63e404f", "de57417da98cadda", "30cd8ea5cd17d802", "8a65204c61e3892c",
	"b8b3857eac812685", "ee5d831f51bb0a42", "bda6d2ed77bf2486", "8d202cf946a48fd0",
	"784ab9a242e91490", "4680d1ff257e9016", "e74ce54ef757e355", "c0f8720c9f89ccb5",
	"4cabd08ea8e3cebe", "fedbad4884f2a98a", "1e1affe84d60f969", "05d5bcf0d38aa87e",
	"cec33c79e4c4c168", "6646bd0981524c88", "d303220ce2b43fac", "ee0b2e0ab4c7e0f6",
	"5c2ca7fcf92d5c7c", "e70c7db6873a6b7d", "8823815b36720bb4", "18d3a750103f6814",
}
