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

// unwalked is what the digest leaves out: the report's cash flow
// (#351), the night's money by category and the history of it. It is
// the news sim's reading of the numbers the walk already hashes (the
// piles, the report's lines and CASH BEFORE), no sim reads it, and a
// change to its shape or its categories is a report's, never a number's.
// The plan the player pinned (#347, World.Ambition) is the player's
// alone: no sim reads it.
var unwalked = map[string]bool{"World.Flows": true, "DayReport.Flow": true, "World.Ambition": true}

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
			if f := v.Type().Field(i); f.IsExported() && !unwalked[v.Type().Name()+"."+f.Name] {
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
// Again for #358 (Card.Hide added to the walk; the move is on day 5,
// the first morning a card is pending, by shape alone and no number
// moved: with the field skipped the digest is the one before on all
// sixty days).
// Again for #351 (the report's cash flow, World.Flows and
// DayReport.Flow, which the walk leaves out, unwalked): days 9, 24, 26
// and 47 move by the report's words alone, a robbery's MONEY line
// naming the corner and the city (`Robbed on Riverside in Eastside`)
// where it read `Robbed on the corner`; days 57 to 60 by its numbers,
// the lieutenant's skim no longer counted twice (once off
// LieutenantActed and again inside the night's CrewSkimmed, so day 57's
// MISSING FROM THE COUNT read $188 for $94 and CASH BEFORE sat $94
// high; the flow reconciling pile by pile is what found it). No number
// moved: with World.Report set aside on both sides the digest is
// main's on all sixty days (checked again on top of #358).
// Again for #343 (HeatState.Trail and Investigation added to the walk;
// the move is on day 1 by shape alone and no number moved: with the two
// fields skipped the digest is the list before it on all sixty days,
// investigations shipping off).
// Again for #344 (Front.City added to the walk, stamped where a front
// is bought, and the fronts' roles): the move is on day 16, the boss's
// first front (the laundromat, which carries no effect), by shape
// alone; under harness.NoFrontRoles the digest is the file's to day 53
// and the roles first move a number on day 54, and cmd/balance -roles
// off prints main's trace to the dollar.
// Again for #346 (CrewMember.Trait, Lived, Captain and Budget added to
// the walk): the move on day 1 is shape alone, and the first number
// moves on day 46, the night the boss's first hire has served [traits]
// days and shows a trait: with the four fields skipped while zero the
// digest is f3dc3ee's on days 1 to 45, and under harness.NoTraits it
// is f3dc3ee's on all sixty.
// Again for #341 (RivalState.ScoutingCity, ScoutDay, Recruited,
// ScoutsHit and Cell, World.Takes, Today.HitScouts and three Stats added
// to the walk; the move is on day 1 by shape alone and no number moved:
// with the fields skipped the digest is the one before on all sixty
// days, the duel never expanding and never keeping the window).
// Again for #354 (the morning's lead, DayReport.Lead, and its lines in
// the journal under the digest's source): every day from day 1 moves by
// the report's and the journal's words alone, the first morning's lead
// being the corner the boss claimed. No number moved: with World.Report
// and World.Journal set aside the digest with the lead written is the
// digest with it left out on every day, the duel and the table, seeds
// 7, 1, 2 and 3, 150 days each; TestLeadDoesNotMoveTheRun holds it.
// Again for #379 (World.DelegatedHit added to the walk, the lieutenant's
// hit on a faction's scouts; and the restaurant's sale_heat_mul 0.9 in
// its city): the move on day 1 is shape alone, and the first number
// moves on day 55, the boss's restaurant cutting a sale's heat at home.
// With World.DelegatedHit skipped and the restaurant as it was the
// digest is the list before it on all sixty days (the duel never
// expands, so neither the lieutenant's hit nor harness.HitScoutsIn
// ever acts on it).
// Again for #343 switched on (heat.toml [investigation] enabled): the
// move is on day 1, HeatState.Trail now tallied from the first sale,
// and no other number moved: with Trail and Investigation skipped on
// both sides the digest on is the digest off on all sixty days, the
// boss peaking at 53.9 heat on this seed, under the sting line, so no
// investigation opens (checked on top of #379).
// Again for #391 (World.Exports and five Stats added to the walk): the
// move on day 1 is shape alone and no number moved. With Exports and
// the Stats.Export* fields set aside the world is main's on every day
// of this seed's boss to day 197 and its net worth main's on all 300:
// from day 198 the Dutchman's book is announced at $8M peak clean (it
// was $10M, the tunnel's line now), an Unlocked in the journal and a
// key in Laundering.Offered, and the boss buys no asset (checked again
// on top of #389).
// Again for #392 (World.Trophies and TrophiesLost, Progression.Rich and
// four Stats added to the walk): the move on day 1 is shape alone and
// no number moved. With those set aside the world is main's to day 214
// and its net worth main's on all 300: from day 215 the boss is over
// the rich list's first line, a headline in the journal and the line
// stamped; its pile never reaches the rot line.
// Again for #395 (Today.CashedOut and two Stats added to the walk):
// the move on day 1 is shape alone. With the three set aside the digest
// is main's on all sixty days; the boss never cashes out.
// Again for #399 (World.ReignSlip and Reigns added to the walk): the
// move on day 1 is shape alone. With the two set aside the digest is
// main's on all sixty days; the boss never reigns.
// Again for #425 (CrewState.Named added to the walk): the move on day 1
// is shape alone. With Named set aside the digest is main's to day 55:
// on day 56 the pool deals a fresh name where main dealt one a member
// who had left once wore. managed, boss and aggressive (20 runs, 200
// days) print what main prints.
var seedDigest = []string{
	"0a9d256ac4067b0b", "d733bc202d78a82d", "d4957c0996824e2f", "854723905af8046c",
	"512fd46840037822", "6d2c090948bb7171", "aa02e7ea3955331a", "14f526cef234b2a7",
	"a4a83c27f7dfe8c6", "d508573e8e6375b4", "fb972410bb305b00", "bad158def77769d0",
	"879d09a4c37af2f1", "1d0a0e8c6300a8f3", "e2daadfcc794e8bb", "3cc33241ff16d133",
	"c1711ea2ab8a9d65", "5afadc26b3deaac7", "26376d966b8c1d93", "572c922927ab5eed",
	"41baf3fd2ea2bb0a", "ac565b3c19e08742", "5c7264a75eed3eb2", "716ef6508a17245d",
	"34e67524c0ef3c07", "7862b637fc0fc925", "d11ca2b69dd686b7", "b470293e739b0617",
	"1dd18b9dabb5164d", "bc2b4f369dc096e2", "cf85c0bd7893dcd6", "696b1ddf9c9e3d3e",
	"79eceb1dbc85d028", "4efd09b43534ecb1", "54173c0bbe82d32d", "bb6934052351ee38",
	"809b1b89c2d0b6e9", "630dd1cd4f6430c0", "6ac6720f96416f93", "c2353c29d1d8a550",
	"a6521f6697c0de42", "d1a17e73a5398d69", "a6a7fc9cc9d5d8c6", "d505926353315154",
	"445630f8e5d3bc7d", "72bcd2145bb6d10b", "b061372f3ab39dfa", "5a6bf8f70930e876",
	"8d7b8ca778a8cccf", "47504e322e5b08d2", "80ff3d78ea9d2da2", "728c4431a401f697",
	"3ca28b57c5545244", "4cdcfc9324d95463", "cd5b176c7667f441", "db2eeaa226a3a302",
	"e46f7c6860b029b1", "5e67b1445679f64f", "e8adbfa42cbb23b1", "aebaad94dcdec9de",
}
