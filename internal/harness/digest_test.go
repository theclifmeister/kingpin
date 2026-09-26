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
// Again for #496 and #503 (the player's till, LaunderingState.Till;
// the wash keeping the contracts' morning; a stashed shipment's fare
// out of the float; SellOrder.All): day 1 by the till's shape alone,
// on top of #501's and #502's moves above (walked with both).
// The boss's cash, clean, offshore, wash, stock and road read main's on
// all sixty days: its first route fare paid out of the float, and its
// contracts' morning over the till, come later than day 60.
// Again for #497 (a lieutenant counts only the stick-ups since they took
// the city, CrewMember.Stickups) and #506 (a truce's end said a day
// ahead, DealEnding): day 1 by the field's shape; with it left out of
// the walk every day is main's digest bar day 52, the morning the
// boss's truce with Dutch holds one more night and the report's
// TERRITORY says so: the report's words, and no number moved
// (walked on top of #501, #502, #496 and #503).
var seedDigest = []string{
	"27d53ecb5ef284d6", "70e6ad0e878812ce", "abc3940b35b60b3c", "7554e68d843fe5b9",
	"3d72ab675f9c4a97", "c33aec642d2281a6", "8f7e777f698d270d", "5f1f045c608b531b",
	"f1e590eb3fc650dc", "c596eda151996c4d", "047c2a45bdb24fcd", "de9ec6f6dcdca872",
	"8c2e16136ef2b28c", "5b460091f0019736", "0c1dbce9c79928fb", "7f76cdc7cef63b9c",
	"ab9602235958cc58", "6e6a59be419c741c", "45ed6009a2623b24", "245c95f0e8b5f441",
	"f30088dd2a7573bf", "04633e11222961d6", "8146712bfabdf7d2", "c9238e09682eb154",
	"bc39520d84dc887f", "9771e8c66fcb442c", "ec623f501c675e35", "ff2556c0255ce805",
	"e96dc712dcddc464", "3e7be16871edfc20", "003317276bfbfe98", "8c06cc87d5b9bb13",
	"815ee195657baa01", "36ba1c8c0133a20b", "684d73e9f9c49416", "a2fc0cc0f022444a",
	"2e84d76a4c062775", "15c61f977b1dac1f", "bd9d837cef2b9241", "a03295dd9506eecd",
	"3e35233b543f8a25", "00945e4a296fb9ee", "7ca6d63fce86b231", "b1a2d5b63e2dfd21",
	"a833fd24a72fd9fe", "85e22a24fc47fd05", "7cd23bc76e94cf7b", "0b079a650be5772b",
	"2a809f3ddaa8b70d", "f513c375b3ca5869", "6e53ff283194c087", "1f1fd74bdd1ce468",
	"92d10c0e9c6172a3", "e48f6fed6d3d7203", "096169700c2cf632", "4bf29b76ae22874e",
	"6cd32496793136fd", "f4a891db2b16b3ce", "a072ac04b3b8b887", "556517244cb87bcf",
}
