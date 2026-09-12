package rivals_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
)

// arrived is world with the rival dug in on three corners next to the
// player, with muscle and money, so that deals have something to hold.
func arrived(t *testing.T, cfg *content.Config, seed uint64, personality string) (*game.World, *rivals.Sim) {
	t.Helper()
	w, s := world(t, cfg, seed)
	w.Rival.Personality = personality
	w.Rival.Trust = cfg.Rivals.Personality[personality].Trust
	w.Rival.Arrived, w.Rival.Muscle, w.Rival.Cash = 1, 8, 50_000
	for _, id := range []string{"railyard", "depot", "strip"} {
		c := w.Corner(id)
		c.Owner, c.Since = game.OwnerRival, 0
	}
	w.Day = 5
	return w, s
}

// Breaking a truce with a push drops trust to the floor, the rival makes
// one call to the police, and it takes no deal for distrust_days however
// often it is asked; after that it can be asked again.
func TestBetrayalFloorAndDistrust(t *testing.T) {
	cfg := content.MustLoad()
	dip := cfg.Rivals.Diplomacy
	w, s := arrived(t, cfg, 4, "defensive")
	w.Rival.Trust = 90
	w.Rival.Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: 60}, Since: w.Day, Until: w.Day + 60}}
	if err := w.SendEnforcers("railyard", events.ForcePush); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	k := kinds(evs)
	if k["DealBroken"] != 1 || k["RivalTippedPolice"] != 1 || k["CornerStruck"] != 1 {
		t.Fatalf("a push under a truce: %v", k)
	}
	if w.Rival.Trust != dip.BetrayalFloor || len(w.Rival.Deals) != 0 || w.Rival.Betrayed != w.Day || w.Stats.Betrayals != 1 {
		t.Fatalf("after the betrayal: %+v stats %+v", w.Rival, w.Stats)
	}
	betrayed := w.Day
	w.Today.Strike = nil
	accepted := 0
	for w.Day < betrayed+dip.DistrustDays+40 && accepted == 0 {
		if err := w.Propose(game.DealTruce, game.Terms{Days: dip.TruceDays[0]}); err != nil {
			t.Fatal(err)
		}
		if c := s.Chance(w, *w.Today.Proposal); (w.Day+1-betrayed < dip.DistrustDays) != (c == 0) {
			t.Fatalf("day %d: chance %.2f, betrayed on day %d, distrust %d days", w.Day, c, betrayed, dip.DistrustDays)
		}
		k := kinds(step(w, s))
		w.Today.Proposal = nil
		if k["DealAccepted"] > 0 {
			accepted = w.Day
		}
		if accepted > 0 && accepted-betrayed < dip.DistrustDays {
			t.Fatalf("a deal accepted on day %d, %d days after the betrayal on day %d", accepted, accepted-betrayed, betrayed)
		}
	}
	if accepted == 0 {
		t.Fatalf("a defensive rival took no truce in %d days after the distrust ran out", 40)
	}
}

// A hit without a deal costs trust by the force table and is no betrayal.
func TestStrikesCostTrust(t *testing.T) {
	cfg := content.MustLoad()
	w, s := arrived(t, cfg, 4, "defensive")
	before := w.Rival.Trust
	if err := w.SendEnforcers("railyard", events.ForceHit); err != nil {
		t.Fatal(err)
	}
	k := kinds(step(w, s))
	if k["DealBroken"] != 0 || w.Stats.Betrayals != 0 || w.Rival.Betrayed != 0 {
		t.Fatalf("a hit with nothing to break: %v, %+v", k, w.Rival)
	}
	if want := before - cfg.Rivals.ForceFor(events.ForceHit).Trust; math.Abs(w.Rival.Trust-want) > 1e-9 {
		t.Fatalf("trust %.1f after a hit from %.1f, want %.1f", w.Rival.Trust, before, want)
	}
}

// An offer stands offer_days and cannot be taken after; one that is taken
// is sealed the next morning on exactly the terms offered.
func TestOffersExpireAndSealAsOffered(t *testing.T) {
	cfg := content.MustLoad()
	dip := cfg.Rivals.Diplomacy
	// An opportunist with the upper hand demands tribute: nobody guards
	// the player's corner, and its muscle is high.
	w, s := arrived(t, cfg, 7, "opportunist")
	w.Rival.Muscle = 20
	w.Crew.Members = w.Crew.Members[:1] // the runner only
	// Pushed off, the player stands on the next free corner, the way the
	// harness trader does.
	stand := func() {
		if w.PostOf(game.You) == nil {
			for _, c := range w.Home().Corners {
				if c.Owner == game.OwnerNone {
					_ = w.Post(c.ID, game.You)
					break
				}
			}
		}
	}
	var offered *events.DealOffered
	for i := 0; i < 60 && offered == nil; i++ {
		stand()
		for _, e := range step(w, s) {
			if o, ok := e.(events.DealOffered); ok {
				offered = &o
			}
		}
	}
	if offered == nil {
		t.Fatal("an opportunist with the upper hand never demanded tribute")
	}
	if len(w.Offers) != 1 || w.Offers[0].ID != offered.ID || w.Offers[0].Expires != w.Day+dip.OfferDays-1 || w.Offers[0].Deal.Kind != game.DealTribute {
		t.Fatalf("offers %+v after %+v on day %d", w.Offers, offered, w.Day)
	}
	terms := w.Offers[0].Deal.Terms
	if terms.PerDay < dip.TributeMin {
		t.Fatalf("tribute of $%d a day, under the minimum", terms.PerDay)
	}
	// Sit on it until it lapses.
	for w.Day <= offered.Expires {
		stand()
		if kinds(step(w, s))["DealOffered"] > 0 {
			t.Fatalf("day %d: a second offer while one is on the table", w.Day)
		}
	}
	if _, err := w.Accept(offered.ID); err == nil {
		t.Fatal("took an offer on the morning after it lapsed")
	}
	if len(w.Offers) != 0 {
		t.Fatalf("offers %+v after the rival stepped past %d", w.Offers, offered.Expires)
	}
	// The next one, taken: sealed as offered, and paid.
	var again *events.DealOffered
	for i := 0; i < 60 && again == nil; i++ {
		stand()
		for _, e := range step(w, s) {
			if o, ok := e.(events.DealOffered); ok {
				again = &o
			}
		}
	}
	if again == nil {
		t.Fatal("no second offer")
	}
	offer := w.Offers[0]
	if _, err := w.Accept(offer.ID); err != nil {
		t.Fatal(err)
	}
	if len(w.Offers) != 0 || len(w.Today.Accepted) != 1 {
		t.Fatalf("after accepting: offers %+v accepted %+v", w.Offers, w.Today.Accepted)
	}
	cash := w.Player.DirtyCash
	evs := step(w, s)
	k := kinds(evs)
	if k["DealAccepted"] != 1 || k["TributePaid"] != 1 {
		t.Fatalf("the morning after accepting: %v", k)
	}
	d := w.Deal(game.DealTribute)
	if d == nil || fmt.Sprintf("%+v", d.Terms) != fmt.Sprintf("%+v", offer.Deal.Terms) || !d.Offered || d.Since != w.Day || d.Until != 0 {
		t.Fatalf("sealed %+v from offer %+v", d, offer.Deal)
	}
	if w.Player.DirtyCash != cash-d.Terms.PerDay || w.Stats.Tribute != d.Terms.PerDay {
		t.Fatalf("cash %d -> %d with tribute %d", cash, w.Player.DirtyCash, d.Terms.PerDay)
	}
	w.Today.Accepted = nil
	// Under tribute nothing of the player's is pushed on or undercut.
	for i := 0; i < 30; i++ {
		stand()
		k := kinds(step(w, s))
		if k["RivalPushed"]+k["CornerTaken"]+k["RivalUndercut"]+k["RivalTippedPolice"] > 0 {
			t.Fatalf("day %d under tribute: %v", w.Day, k)
		}
	}
	// A missed payment is a betrayal.
	w.Player.DirtyCash = 0
	k = kinds(step(w, s))
	if k["DealBroken"] != 1 || k["TributePaid"] != 0 || k["RivalTippedPolice"] != 1 || w.Deal(game.DealTribute) != nil {
		t.Fatalf("the morning the tribute went unpaid: %v, deals %+v", k, w.Rival.Deals)
	}
}

// The chance a proposal is taken moves the way the table says: with
// trust, with the war, with fear, against a long truce and a thin
// tribute, and a split past the line or over a corner it holds is
// refused outright.
func TestChanceFollowsTheTable(t *testing.T) {
	cfg := content.MustLoad()
	dip := cfg.Rivals.Diplomacy
	w, s := arrived(t, cfg, 4, "opportunist")
	truce := func(days int) game.Deal { return game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: days}} }
	base := s.Chance(w, truce(dip.TruceDays[1]))
	if base <= 0 || base >= 1 {
		t.Fatalf("base chance %.2f", base)
	}
	if s.Chance(w, truce(dip.TruceDays[0])) <= base || s.Chance(w, truce(dip.TruceDays[2])) >= base {
		t.Fatalf("truce length: short %.2f, standard %.2f, long %.2f", s.Chance(w, truce(dip.TruceDays[0])), base, s.Chance(w, truce(dip.TruceDays[2])))
	}
	w.Rival.Trust += 30
	if s.Chance(w, truce(dip.TruceDays[1])) <= base {
		t.Fatal("trust did not help")
	}
	w.Rival.Trust -= 30
	w.Rival.War = 60
	if s.Chance(w, truce(dip.TruceDays[1])) <= base {
		t.Fatal("a loud war did not help")
	}
	w.Rival.War = 0
	w.Player.Reputation.Fear = 80
	if s.Chance(w, truce(dip.TruceDays[1])) <= base {
		t.Fatal("fear did not help")
	}
	w.Player.Reputation.Fear = 0
	fat := game.Deal{Kind: game.DealTribute, Terms: game.Terms{PerDay: s.Cut(w, dip.TributeCuts[2])}}
	thin := game.Deal{Kind: game.DealTribute, Terms: game.Terms{PerDay: s.Cut(w, dip.TributeCuts[0])}}
	if s.Chance(w, fat) <= s.Chance(w, thin) {
		t.Fatalf("tribute: fat %.2f, thin %.2f", s.Chance(w, fat), s.Chance(w, thin))
	}
	lines := w.SplitLines()
	modest := game.Deal{Kind: game.DealSplit, Terms: game.Terms{Corners: lines[0]}}
	greedy := game.Deal{Kind: game.DealSplit, Terms: game.Terms{Corners: lines[2]}}
	if s.Chance(w, modest) <= 0 || s.Chance(w, greedy) != 0 {
		t.Fatalf("split: modest %.2f, greedy %.2f", s.Chance(w, modest), s.Chance(w, greedy))
	}
	theirs := game.Deal{Kind: game.DealSplit, Terms: game.Terms{Corners: append(append([]string(nil), lines[0]...), "railyard")}}
	if s.Chance(w, theirs) != 0 {
		t.Fatal("a split asking for a corner it holds was not refused")
	}
	if err := w.SendEnforcers("railyard", events.ForcePush); err != nil {
		t.Fatal(err)
	}
	if s.Chance(w, truce(dip.TruceDays[1])) != 0 {
		t.Fatal("a truce proposed the night the enforcers go in was not refused")
	}
}

// A chaotic rival tears deals up by personality; the trust the rival has
// is untouched, the stats say who did it.
func TestChaoticWhim(t *testing.T) {
	cfg := content.MustLoad()
	w, s := arrived(t, cfg, 2, "chaotic")
	broken := 0
	for i := 0; i < 300 && broken == 0; i++ {
		if w.Deal(game.DealTruce) == nil {
			w.Rival.Deals = append(w.Rival.Deals, game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: 60}, Since: w.Day, Until: w.Day + 60})
		}
		trust := w.Rival.Trust
		for _, e := range step(w, s) {
			if db, ok := e.(events.DealBroken); ok {
				if db.By != "rival" {
					t.Fatalf("%+v", db)
				}
				broken++
				if w.Rival.Trust < trust {
					t.Fatalf("trust fell %.1f -> %.1f over the rival's own betrayal", trust, w.Rival.Trust)
				}
			}
		}
	}
	if broken == 0 || w.Stats.BetrayedBy != broken || w.Stats.Betrayals != 0 {
		t.Fatalf("broken %d, stats %+v", broken, w.Stats)
	}
}

// A save from before the table gets the rival's trust seeded on load.
func TestMigrateDiplomacy(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg, 3)
	w.Rival.Trust = 0
	s.MigrateDiplomacy(w)
	if want := cfg.Rivals.Personality[w.Rival.Personality].Trust; w.Rival.Trust != want {
		t.Fatalf("trust %.0f after migration, want %.0f", w.Rival.Trust, want)
	}
}

// A rival run out of town has nothing to deal about: its deals end, its
// offers lapse, and no tribute is paid to nobody.
func TestRoutedRivalHasNoTable(t *testing.T) {
	cfg := content.MustLoad()
	w, s := arrived(t, cfg, 4, "defensive")
	w.Rival.Deals = []game.Deal{{Kind: game.DealTribute, Terms: game.Terms{PerDay: 300}, Since: w.Day}}
	w.Offers = []game.Offer{{ID: 1, Deal: game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: 15}}, Expires: w.Day + 4}}
	for i := range w.Home().Corners {
		if c := &w.Home().Corners[i]; c.Owner == game.OwnerRival {
			c.Owner = game.OwnerNone
		}
	}
	w.Rival.Routed = w.Day
	cash := w.Player.DirtyCash
	k := kinds(step(w, s))
	if k["DealEnded"] != 1 || k["TributePaid"] != 0 || len(w.Rival.Deals) != 0 || len(w.Offers) != 0 || w.Player.DirtyCash != cash {
		t.Fatalf("%v deals %+v offers %+v cash %d -> %d", k, w.Rival.Deals, w.Offers, cash, w.Player.DirtyCash)
	}
}
