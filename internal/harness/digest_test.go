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
// Again for #227 (World.Reign added to the walk; no number moved: with
// the field skipped the digest is the list before it on all sixty days,
// the boss holding no reign by day 60 on this seed).
// Again for #228 (Law.Favours, Law.FavourOwed and Stats.Favours in the
// walk; no number moved: with the fields skipped the digest is the list
// before it on all sixty days, the boss paying nobody by day 60 here).
// Again for #229 (World.War in the walk; no number moved: with the
// field skipped the digest is the list before it on all sixty days, the
// boss declaring no war).
// Again for #231 (Stats.Taxed in the walk; no number moved: with the
// field skipped the digest is the list before it on all sixty days, the
// boss holding no city by day 60).
// Again for #233, by copy alone, on day 56: the report's opening line
// under TIER the night the rival's push on the boss's front line was
// held off (World.Report is in the walk); with the report set aside the
// digest is the list before it on all sixty days, on both trees, and no
// number moved.
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
// Again for #49 (World.LegitDays, Ending.Who and Stats.Score added to
// the walk; the move is on day 1 by shape alone and no number moved:
// with the three fields skipped the digest is 2be03a7's on all sixty
// days, the boss's legit income under zero and nothing ending, and
// cmd/balance prints 2be03a7's trace to the dollar on boss, laundered
// and distributor to day 200, TestNoEndingIsTheOldRun).
// Again for #342 (DilemmaState.Owes added to the walk; the move is on
// day 1 by shape alone and no number moved: with the field skipped the
// digest is the list before it on all sixty days, the digest's run
// answering no card).
// Again for #343 (HeatState.Trail and Investigation added to the walk;
// the move is on day 1 by shape alone and no number moved: with the two
// fields skipped the digest is the list before it on all sixty days,
// investigations shipping off).
var seedDigest = []string{
	"f989e8db98e65762", "dd55377a42e034ca", "69db755c70ab443a", "861273f85d9b03b8",
	"8ba5d82fe527ffa9", "ea7ecc31df968503", "632f22a010dd0c85", "ce72c39263210a87",
	"1038b5ea4f63e82d", "573362dde5eefad0", "664203b6891299b1", "e4484e583c651973",
	"67f240154b252873", "8f50251c5762ef94", "33a781d796327666", "672ee62f550c0fbe",
	"82cc8d081be9b592", "1851133e8fd883d3", "2425809e973d2821", "327b0b8c7108f16b",
	"179aef0ef92333bb", "87b2debcde585efa", "0b33004902f88b4f", "d87a537eb4cb504a",
	"f70a97e109602460", "85be42f937b5bafe", "12a8d523bcfa5db0", "38d880e33ee4ffe7",
	"931ba59b5e7126cc", "834222abaef88261", "55e87ac07400baae", "3be66b8aadb4ec1c",
	"6435369c359c4d41", "6e700c188bde6344", "2b54cdf8d6f318ef", "b941504f43bd0fa7",
	"05eeac7f2bd7c8ae", "eae011dd8255898f", "2399d00bab594f73", "86f4fc50d36fa439",
	"bb7c08009433637e", "094f216a3a9c22f5", "3880b617fa1934b3", "7ea3e9956724df13",
	"4df7d0436d1d1173", "3cd0378d2c3dd1ed", "1e5a8f5613ce1f78", "227e04ea54ac7dfe",
	"b329e68a26246015", "62f85073a537026f", "a76571fef5250c00", "75cd226208faf615",
	"347eefbbf21958d1", "4538da56f51cb031", "0c96b118cce3d42d", "9c07e7a066d2a2ac",
	"b9ef3f811eae6d37", "2149e057bbfee44e", "93fc39f3ae671091", "47082689ce6e671d",
}
