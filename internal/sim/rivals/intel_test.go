package rivals_test

import (
	"fmt"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
)

// What the factions let you know (#45), no dice: a strike files the
// muscle you met within observe_band at observe_confidence, a faction
// that has shown its hand files its temper at 1 (once), a scout's count
// stands over a band while it lives, and OddsAt reads the picker's
// odds at a count the file holds.
func TestObservationFiles(t *testing.T) {
	cfg := duel()
	tun := cfg.Intel.Intel
	w, s := warWorld(t, cfg, 19, "expansionist")
	w.Rival().Observed = false
	w.Rival().Muscle = 7
	if len(w.Intel) != 0 {
		t.Fatal("a fresh world knows something")
	}
	if err := w.SendEnforcers("docks", events.ForceWarn); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	w.Today.Strike = nil
	k := game.Known(w)
	lo, hi, f, ok := k.Muscle(game.FactionRival)
	wantLo, wantHi := game.MuscleBand(w.Rival().Muscle, tun.ObserveBand)
	if !ok || lo != wantLo || hi != wantHi || f.Source != game.SourceSeen || f.Confidence != tun.ObserveConfidence || f.Value != fmt.Sprintf("%d–%d", wantLo, wantHi) {
		t.Fatalf("the band after a strike: %d–%d %+v (muscle %d)", lo, hi, f, w.Rival().Muscle)
	}
	if p := k.Personality(game.FactionRival); p != "expansionist" {
		t.Fatalf("the temper after a strike: %q", p)
	}
	if n := kinds(evs)["IntelGained"]; n != 2 {
		t.Fatalf("%d facts gained, want the band and the temper: %v", n, kinds(evs))
	}
	// The temper is filed once; a quiet night files nothing.
	evs = step(w, s)
	if kinds(evs)["IntelGained"] != 0 || len(w.Intel) != 2 {
		t.Fatalf("a quiet night: %v %+v", kinds(evs), w.Intel)
	}
	// A count from the scout stands over a band while it lives.
	sure := *cfg
	sure.Rivals.Books.ScoutBase = 1
	s = newSim(&sure)
	if err := w.Scout(s.ScoutCost()); err != nil {
		t.Fatal(err)
	}
	step(w, s)
	w.Today.Scouting = nil
	if word := game.Known(w).MuscleWord(game.FactionRival); word != fmt.Sprint(w.Rival().Muscle) {
		t.Fatalf("after the scout: %q", word)
	}
	if err := w.SendEnforcers("docks", events.ForceWarn); err != nil {
		t.Fatal(err)
	}
	step(w, s)
	w.Today.Strike = nil
	if _, _, f, _ := game.Known(w).Muscle(game.FactionRival); f.Source != game.SourceBooks {
		t.Fatalf("a band replaced the scout's count: %+v", f)
	}
	// OddsAt at the truth is Odds; at more muscle, less.
	r := w.Rival()
	if got, want := s.OddsOnAt(w, r, nil, events.ForcePush, r.Muscle), s.Odds(w, r, events.ForcePush); got != want {
		t.Fatalf("OddsOnAt the truth %.3f, Odds %.3f", got, want)
	}
	var yours *game.Corner
	for i := range w.Home().Corners {
		if w.Home().Corners[i].Owner == game.OwnerPlayer {
			yours = &w.Home().Corners[i]
		}
	}
	if s.OddsOnAt(w, r, nil, events.ForcePush, r.Muscle+3) >= s.Odds(w, r, events.ForcePush) || s.PushOddsAt(w, r, yours, 1) >= s.PushOddsAt(w, r, yours, 9) {
		t.Fatal("more muscle does not read as worse odds")
	}
}

// The feed (#45): a faction that trusts you under feed_trust plants a
// lie at feed_chance a night off the intel stream, a road with its dial
// off and nothing on it or one of its own corners, filed as a contact's
// word at feed_confidence with the faction hidden in Planted; a boost
// on the corner it named finds the till empty at the usual heat and
// war, the fact names the faction and IntelFalse is emitted; a faction
// over the line plants nothing, and a night with the feed boxed is the
// night it was on the home stream.
func TestTheFeed(t *testing.T) {
	cfg := duel()
	cfg.Intel.Intel.FeedChance = 1
	tun := cfg.Intel.Intel
	w, s := warWorld(t, cfg, 23, "defensive")
	w.Rival().Trust = tun.FeedTrust - 1
	evs := step(w, s)
	if kinds(evs)["IntelGained"] != 1 || w.Stats.Lures != 1 || len(w.Intel) != 1 {
		t.Fatalf("the first night: %v stats %+v file %+v", kinds(evs), w.Stats, w.Intel)
	}
	f := w.Intel[0]
	if f.Source != game.SourceContact || f.Planted != game.FactionRival || f.Confidence != tun.FeedConfidence || !f.Lure() {
		t.Fatalf("the lie %+v", f)
	}
	switch f.Kind {
	case game.FactRisk:
		if w.Route(f.Subject).Dial != events.RouteOff || f.Number != tun.FeedRisk {
			t.Fatalf("a road lie on a road you are on: %+v dial %v", f, w.Route(f.Subject).Dial)
		}
	case game.FactStash:
		if c := w.Corner(f.Value); c == nil || c.Owner != game.OwnerRival || c.FactionID() != game.FactionRival || f.Subject != game.FactionRival {
			t.Fatalf("a till lie off its corners: %+v (%+v)", f, w.Corner(f.Value))
		}
	default:
		t.Fatalf("a lie of kind %q", f.Kind)
	}
	// Every night under the line plants (a road once; the till's corner
	// can be named again); over it nothing.
	for i := 0; i < 5; i++ {
		step(w, s)
	}
	if w.Stats.Lures < 4 {
		t.Fatalf("%d lies over six nights at chance 1", w.Stats.Lures)
	}
	w.Rival().Trust = tun.FeedTrust
	lures := w.Stats.Lures
	step(w, s)
	if w.Stats.Lures != lures {
		t.Fatal("a faction over the line fed you")
	}
	// The till lie bites a boost: the roll is made, the till is empty,
	// the heat and the war land, the source names the faction.
	w, _ = warWorld(t, cfg, 23, "defensive")
	w.Rival().Trust = 50
	w.Learn(game.Fact{Subject: game.FactionRival, Kind: game.FactStash, Value: "docks", Confidence: 0.9, Day: w.Day, Source: game.SourceContact, Stale: 0.05, Forget: 0.2, Planted: game.FactionRival})
	rich := *cfg
	rich.Rivals.Force = map[string]content.ForceConfig{}
	for k, fc := range cfg.Rivals.Force {
		fc.Flip = 1
		rich.Rivals.Force[k] = fc
	}
	s = newSim(&rich)
	if err := w.Boost("docks", events.ForceWarn); err != nil {
		t.Fatal(err)
	}
	cash, war := w.Player.DirtyCash, w.Rival().War
	evs = step(w, s)
	w.Today.Strike = nil
	var boosted *events.RivalBoosted
	for _, e := range evs {
		if ev, ok := e.(events.RivalBoosted); ok {
			boosted = &ev
		}
	}
	if boosted == nil || boosted.Taken || boosted.Cash != 0 || w.Player.DirtyCash != cash || w.Rival().War <= war || boosted.Heat <= 0 {
		t.Fatalf("the boost on the lie: %+v dirty %d -> %d war %.1f -> %.1f", boosted, cash, w.Player.DirtyCash, war, w.Rival().War)
	}
	if kinds(evs)["IntelFalse"] != 1 || w.Stats.Bitten != 1 {
		t.Fatalf("the bite: %v stats %+v", kinds(evs), w.Stats)
	}
	if f, ok := game.Known(w).Fact(game.FactionRival, game.FactStash); !ok || f.Source != game.FactionRival || f.Lure() || w.Lure(game.FactionRival, game.FactStash) != nil {
		t.Fatalf("after the bite %+v", f)
	}
	// Boxed, the night is the night it was: the feed rolls on its own
	// stream and a faction over the line rolls nothing.
	a, sa := warWorld(t, cfg, 31, "chaotic")
	b, _ := warWorld(t, cfg, 31, "chaotic")
	a.Rival().Trust, b.Rival().Trust = 5, 5
	sb := rivals.New(func() *content.Config { c := *cfg; c.Intel.Intel.FeedChance = 0; return &c }())
	for i := 0; i < 10; i++ {
		step(a, sa)
		step(b, sb)
	}
	if a.Rival().Cash != b.Rival().Cash || a.Rival().Muscle != b.Rival().Muscle || a.RivalHeld() != b.RivalHeld() || b.Stats.Lures != 0 || a.Stats.Lures == 0 {
		t.Fatalf("the feed moved the rival's dice: %+v against %+v, lies %d/%d", *a.Rival(), *b.Rival(), a.Stats.Lures, b.Stats.Lures)
	}
}
