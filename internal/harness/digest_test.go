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
// alone: no sim reads it. The night the lead first said the corners
// have a ceiling (#476, Progression.Ceiling) is the port alert's: the
// news sim stamps it off the flows' history, and no sim reads it.
var unwalked = map[string]bool{"World.Flows": true, "DayReport.Flow": true, "World.Ambition": true, "Progression.Ceiling": true}

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
// Again for #419 (a push's CornerTaken says the odds, the muscle and
// the guard, and the digest names an idle enforcer): the move on day 23
// is the report's and the journal's words alone. With World.Report and
// World.Journal set aside the digest is main's on all sixty days, and
// managed, boss, war and aggressive (20 runs, 200 days) print what main
// prints.
// Again for #458 (Front.Unpaid added to the walk): the move on day 16,
// the boss's first front, is shape alone. With Unpaid set aside the
// digest is main's on all sixty days; the boss's fronts never go short.
// Again for #459 (a route short of its target and sending nothing says
// why, RouteIdle, and a supply contract out of cash says what the wash
// took): the move on day 1 is the report's words alone, the boss's
// Channel on with nothing in the Bayport stash to send. With
// World.Report and World.Journal set aside the digest is main's on all
// sixty days.
// Again for #465 (the words match what happened): the move on day 10 is
// the journal's and the report's words alone (a big night's headline no
// longer says "awash in cheap", days 10, 24, 26, 35 and 55; the push
// held off opens TERRITORY, not TIER, day 56) and one count: from day
// 56 Stats.CornersLost counts the two corners that went back to the
// street that night unworked (1 -> 3), the summary's ground. With
// World.Report, World.Journal and Stats.CornersLost set aside the
// digest is main's on all sixty days.
// Again for #463 (a buyer's report line points `on the market screen
// (2)`, the spelling every pointer has, where it said `on the market
// (2)`): the days a buyer's offer is reported move by the report's
// words alone; no sim, roll or number changed.
// Again for #470 (supply contracts short of cash or room fill by
// margin, World.ByMargin): the move on day 59 is the first morning the
// boss's lieutenant's contracts in Eastside, short of cash, put the
// pills before the weed (a dollar brought back more on the pills that
// morning); on days 57 and 58 they were short too and the weed still
// ranked first, so the fifty-eight days before it are main's.
// Again for #473 (a buyer is a phrase: `You took the order from …`,
// `The offer from … lapsed`): the move on day 8 and the days after it
// that take or lose a buyer's order is the report's words alone; the
// days between keep main's digest, and no number moved.
// Again for #475 (the arrest line signs a warrant, HeatState.WarrantDay)
// and #479 (tip_evidence 0.2 -> 0.1): the move on day 1 is the new
// field's shape alone. The boss never meets the arrest line in sixty
// days and never tips; with HeatState.WarrantDay set aside the digest
// is main's on all sixty days.
// Again for #478 (the nightly sweep offshore, LaunderingState.Sweep):
// day 1 by shape alone; with the field left out of the walk every day
// is main's digest, and no number moved.
// Again for #501 (a card is not dealt again within repeat_gap days,
// DilemmaState.Dealt) and #502 (the report's lines say what happened):
// day 1 by the new field's shape, day 8 by the lead's flow line (`Profit
// ran ×5.1 the week's nightly average: +$27K last night against
// +$5,339`, a night against a night) and the report's words after it.
// With Dealt, the report and the journal left out of the walk, every
// one of the sixty days is main's digest (so walked on 92db052 too):
// the boss's one card is dealt on day 5 as before, and no number moved.
var seedDigest = []string{
	"2518d9c83a763111", "a2b1742ab91fec2b", "67b6b53c3feb8b69", "666f68d8f9bf95e0",
	"673d6f2ac28e0bfa", "2d719ef8f54e510b", "1dcdd9b831e243f2", "54f20eb08c035340",
	"288823bb723ba671", "02a2174f54055aaa", "b5bb8993f463964c", "6d8daced39949cb3",
	"ae64610c235c502b", "d7568fee74c03577", "88196cf8b7a735fe", "62f6b69af10fc181",
	"72cdd3089b450a2d", "72369db10af91bdb", "f3d9d3ee2ddda8df", "142f3e6e393dd534",
	"96ffcafd37a177be", "6f9d27d5f6af881b", "2f5ba68ad66f6139", "ce03bcf5b3ee0825",
	"d6370a2ec0f94be4", "2c9f3081926a2fcf", "ce93b028e4375d48", "fe73b39302c68204",
	"8b3e9b1da1a87efd", "ed19620205f395f3", "80d9751465eaf105", "3ae81f63f60d63ac",
	"9cb067cc10951416", "531042690fec4908", "a5abb982ce9fed29", "964742725c84967f",
	"bccc05c3a2daf7ce", "81830ed84a69efbc", "d07576e14d94e122", "87c03c2443318ffa",
	"42feb3f4b4cf8d60", "e96922e539904cd1", "d5f7f1b6eccdb1d0", "8ac629c39a55ad1c",
	"4eca2df082850b39", "de1a829bf7e60ca8", "9574c01131230dbc", "b367c8d58d1a0c5c",
	"551b3c9981bfc6a4", "9922b0c5d24d8976", "0dd1ee4990e08f96", "625f84572bc2d36e",
	"7b9cc50b597ae364", "a8d272f97acfa70e", "0ef02d771b21fbaf", "17c70a406fc3a50a",
	"1b5c16073d3ff651", "6daed785492d21d0", "901be4377e1ffb91", "0c960ca09d29f40d",
}
