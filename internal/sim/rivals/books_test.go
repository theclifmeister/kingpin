package rivals_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
)

// The books (#70): scouting, boosting, tipping and buying off muscle.

// find is the first event of a kind in a night's events, or nil.
func find[E events.Event](evs []events.Event) *E {
	for _, e := range evs {
		if ev, ok := e.(E); ok {
			return &ev
		}
	}
	return nil
}

// A boost takes take of the corner's day of income off the rival's
// chest and into your dirty cash, exactly, and never the ground; it
// draws the boost's heat and war whatever the roll, and lands at the
// strike's odds.
func TestBoostTakesTheTakingsNotTheGround(t *testing.T) {
	cfg := content.MustLoad()
	w, s := warWorld(t, cfg, 5, "defensive")
	w.Rival.Muscle = 2
	b := cfg.Rivals.Boost
	docks := w.Corner("docks")
	if err := w.Boost("fourth", events.ForceHit); err == nil {
		t.Fatal("boosted your own corner")
	}
	if err := w.Boost("docks", events.ForceHit); err != nil {
		t.Fatal(err)
	}
	if w.Today.Strike == nil || !w.Today.Strike.Boost || w.Today.Strike.Force != events.ForceHit {
		t.Fatalf("the boost is not the night's strike order: %+v", w.Today.Strike)
	}
	// A strike queued after it replaces it: one order a night.
	if err := w.SendEnforcers("docks", events.ForceWarn); err != nil || w.Today.Strike.Boost {
		t.Fatalf("a strike did not replace the boost: %v %+v", err, w.Today.Strike)
	}
	taken, tries := false, 0
	for !taken && tries < 50 {
		if err := w.Boost("docks", events.ForceHit); err != nil {
			t.Fatal(err)
		}
		take := s.BoostTake(w, *docks)
		if take <= 0 || take != int(math.Round(b.Take*float64(s.CornerIncome(w, *docks))/cfg.Rivals.Rivals.Margin)) {
			t.Fatalf("take %d against a corner worth %d a day to them", take, s.CornerIncome(w, *docks))
		}
		cash, war, trust := w.Player.DirtyCash, w.Rival.War, w.Rival.Trust
		evs := step(w, s)
		w.Today.Strike = nil
		tries++
		ev := find[events.RivalBoosted](evs)
		if ev == nil {
			t.Fatalf("day %d: no RivalBoosted: %v", w.Day, kinds(evs))
		}
		if find[events.CornerStruck](evs) != nil {
			t.Fatal("a boost is not a strike")
		}
		if docks.Owner != game.OwnerRival {
			t.Fatalf("a boost took the corner: %+v", *docks)
		}
		if ev.Heat != s.BoostHeat(docks) || ev.Heat != b.Heat*docks.Heat || ev.Force != events.ForceHit {
			t.Fatalf("boost event %+v", *ev)
		}
		if want := math.Min(100, war+b.War) * (1 - cfg.Rivals.Rivals.WarDecay); w.Rival.War < want-1e-9 {
			t.Fatalf("war %.1f from %.1f, want +%.0f less the decay", w.Rival.War, war, b.War)
		}
		if w.Rival.Trust != math.Max(0, trust-cfg.Rivals.Force["hit"].Trust) {
			t.Fatalf("trust %.1f from %.1f", w.Rival.Trust, trust)
		}
		taken = ev.Taken
		if taken {
			if ev.Cash != take || w.Player.DirtyCash != cash+take || ev.Toll != b.Loyalty || ev.Hurt != 0 {
				t.Fatalf("landed: %+v, cash %d -> %d, take %d", *ev, cash, w.Player.DirtyCash, take)
			}
		} else if ev.Cash != 0 || w.Player.DirtyCash != cash || ev.Toll != b.FailLoss {
			t.Fatalf("held off: %+v, cash %d -> %d", *ev, cash, w.Player.DirtyCash)
		}
	}
	if !taken {
		t.Fatalf("%d boosts at hit against muscle 2 and never took the takings", tries)
	}
	if w.Stats.Boosts != tries || w.Stats.Boosted == 0 || w.Stats.Strikes != 0 || w.Stats.CornersWon != 0 {
		t.Fatalf("stats %+v", w.Stats)
	}
}

// A boost that fails against a rival with more heads than fail_muscle
// hurts an enforcer: the crew sim reads Hurt off the event; the toll is
// fail_loss.
func TestFailedBoostHurts(t *testing.T) {
	cfg := content.MustLoad()
	w, s := warWorld(t, cfg, 7, "defensive")
	b := cfg.Rivals.Boost
	w.Rival.Muscle = b.FailMuscle + 40 // a wall
	for i := range w.Crew.Members {
		w.Crew.Members[i].Skill = 1
	}
	if err := w.Boost("docks", events.ForceWarn); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	ev := find[events.RivalBoosted](evs)
	if ev == nil || ev.Taken {
		t.Fatalf("a warn boost against 44 heads landed: %+v", ev)
	}
	if ev.Toll != b.FailLoss || ev.Hurt != b.FailHurt {
		t.Fatalf("a failed boost: %+v, want toll %.0f hurt %d", *ev, b.FailLoss, b.FailHurt)
	}
}

// A boost at push or hit under a live deal is a betrayal as a strike is;
// at warn it is not.
func TestBoostAtPushBreaksThePeace(t *testing.T) {
	cfg := content.MustLoad()
	for _, force := range []events.Force{events.ForceWarn, events.ForcePush} {
		w, s := warWorld(t, cfg, 3, "defensive")
		w.Rival.Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: 30}, Since: 1, Until: 31}}
		if err := w.Boost("docks", force); err != nil {
			t.Fatal(err)
		}
		evs := step(w, s)
		broke := find[events.DealBroken](evs) != nil
		if broke != (force != events.ForceWarn) {
			t.Fatalf("a boost at %s: deal broken %v", force, broke)
		}
		if broke && (w.Rival.Betrayed != w.Day || w.Stats.Betrayals != 1 || find[events.RivalTippedPolice](evs) == nil) {
			t.Fatalf("the betrayal at %s: %+v stats %+v events %v", force, w.Rival, w.Stats, kinds(evs))
		}
	}
}

// A tip raises the rival's heat, costs its trust and holds its grudge;
// past the notice line the police raid the corner: it goes back to the
// street, raid_muscle of its muscle is gone and its heat comes down.
// Under a truce a tip is a betrayal.
func TestTipsBringARaid(t *testing.T) {
	cfg := content.MustLoad()
	tp := cfg.Rivals.Tip
	w, s := warWorld(t, cfg, 11, "defensive")
	w.Rival.Muscle = 2 // the heads it came with: none walks for wages
	if err := w.Tip("fourth"); err != game.ErrNotRivals {
		t.Fatalf("tipped on your own corner: %v", err)
	}
	if err := w.Tip("docks"); err != nil {
		t.Fatal(err)
	}
	if err := w.Tip("docks"); err != game.ErrTipped {
		t.Fatalf("tipped twice: %v", err)
	}
	w.CancelTip()
	raided := false
	for night := 1; night <= 10 && !raided; night++ {
		if err := w.Tip("docks"); err != nil {
			t.Fatal(err)
		}
		heat, trust := w.Rival.Heat, w.Rival.Trust
		evs := step(w, s)
		w.Today.Tipoff = nil
		ev := find[events.PoliceTipped](evs)
		if ev == nil || ev.Betrayal {
			t.Fatalf("night %d: %+v %v", night, ev, kinds(evs))
		}
		if w.Rival.Trust != math.Max(0, trust-tp.Trust) {
			t.Fatalf("night %d: trust %.1f from %.1f", night, w.Rival.Trust, trust)
		}
		if find[events.PoliceTipped](evs) != nil && w.Stats.Tips != night {
			t.Fatalf("night %d: stats %+v", night, w.Stats)
		}
		if r := find[events.RivalRaided](evs); r != nil {
			raided = true
			want := max(1, int(math.Round(2*tp.RaidMuscle)))
			if r.Muscle != want || w.Rival.Muscle != 2-want || w.Corner("docks").Owner != game.OwnerNone {
				t.Fatalf("the raid: %+v muscle %d corner %+v", *r, w.Rival.Muscle, *w.Corner("docks"))
			}
			if heat+tp.Heat < tp.PoliceNotice {
				t.Fatalf("raided at heat %.0f + %.0f, under the notice line %.0f", heat, tp.Heat, tp.PoliceNotice)
			}
			// The grudge may have been paid back with a call tonight;
			// the heads in the van are away.
			if w.Rival.Heat >= tp.PoliceNotice || w.Stats.RivalRaids != 1 || w.Rival.LastRaid != w.Day || w.Rival.Grudge+w.Rival.Tips < tp.Grudge || w.Rival.Away != want {
				t.Fatalf("after the raid: heat %.1f stats %+v rival %+v", w.Rival.Heat, w.Stats, w.Rival)
			}
		} else if w.Rival.Heat <= heat || ev.RivalHeat != math.Min(100, heat+tp.Heat) {
			t.Fatalf("night %d: heat %.1f from %.1f, event %.1f", night, w.Rival.Heat, heat, ev.RivalHeat)
		}
	}
	if !raided {
		t.Fatalf("ten tips and no raid: heat %.1f", w.Rival.Heat)
	}
	// Under a truce a tip is a betrayal: trust to the floor, a call made.
	w, s = warWorld(t, cfg, 11, "defensive")
	w.Rival.Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: 30}, Since: 1, Until: 31}}
	if err := w.Tip("docks"); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	ev := find[events.PoliceTipped](evs)
	if ev == nil || !ev.Betrayal || find[events.DealBroken](evs) == nil || find[events.RivalTippedPolice](evs) == nil {
		t.Fatalf("a tip under a truce: %+v %v", ev, kinds(evs))
	}
	if w.Rival.Trust != cfg.Rivals.Diplomacy.BetrayalFloor || w.Rival.Betrayed != w.Day || w.Stats.Betrayals != 1 {
		t.Fatalf("after the betrayal: %+v", w.Rival)
	}
	// A tip on a corner that is no longer the rival's is dropped.
	w.Corner("docks").Owner = game.OwnerNone
	if err := w.Tip("docks"); err != game.ErrNotRivals {
		t.Fatalf("tipped on a free corner: %v", err)
	}
}

// The rival's own pushes raise its heat only while it has some: a run
// that never tips keeps it at zero whatever it does.
func TestRivalHeatNeedsATipToStart(t *testing.T) {
	cfg := content.MustLoad()
	w, s := warWorld(t, cfg, 13, "expansionist")
	w.Rival.Muscle = 10
	for i := 0; i < 40; i++ {
		step(w, s)
		if w.Rival.Heat != 0 {
			t.Fatalf("day %d: heat %.2f with no tip", w.Day, w.Rival.Heat)
		}
	}
	tp := cfg.Rivals.Tip
	w.Rival.Heat = tp.Heat
	pushed := false
	for i := 0; i < 40 && !pushed; i++ {
		before := w.Rival.Heat
		evs := step(w, s)
		pushes := kinds(evs)["RivalPushed"]
		for _, e := range evs {
			if ev, ok := e.(events.CornerTaken); ok && ev.From == game.OwnerPlayer && ev.Handed == "" {
				pushes++
			}
		}
		if pushes > 0 {
			pushed = true
			if w.Rival.Heat <= before*(1-tp.Decay) {
				t.Fatalf("%d pushes and heat %.2f from %.2f", pushes, w.Rival.Heat, before)
			}
		} else if math.Abs(w.Rival.Heat-before*(1-tp.Decay)) > 1e-9 {
			t.Fatalf("no push and heat %.2f from %.2f", w.Rival.Heat, before)
		}
	}
}

// Buying off muscle sends heads home and never to you: at the odds the
// heads leave, never more than it has (the rest of the money comes
// back), the rival none the wiser; failing, the money is gone and it
// holds a grudge. The price is a corner-day figure plus a share of the
// chest per head, cut by respect.
func TestBuyOffSendsHeadsHome(t *testing.T) {
	cfg := content.MustLoad()
	p := cfg.Rivals.Poach
	w, s := warWorld(t, cfg, 17, "defensive")
	w.Rival.Muscle = 2 // the heads it came with: none walks for wages
	crew := len(w.Crew.Members)
	price := s.MusclePrice(w)
	want := p.MusclePrice*s.CornerDay(w) + p.CashShare*float64(w.Rival.Cash)/2
	if price != int(math.Round(want)) {
		t.Fatalf("price %d, want %.0f", price, want)
	}
	w.Player.Reputation.Respect = 100
	if got := s.MusclePrice(w); got != int(math.Round(want*(1-cfg.Reputation.Effects.PoachPriceCut))) || got >= price {
		t.Fatalf("at respect 100 the price is %d, was %d", got, price)
	}
	w.Player.Reputation.Respect = 0
	if err := w.BuyOff(0, price); err != game.ErrBadUnits {
		t.Fatalf("bought nothing: %v", err)
	}
	w.Player.DirtyCash = price - 1
	if err := w.BuyOff(1, price); err == nil {
		t.Fatal("bought on money you do not have")
	}
	w.Player.DirtyCash = 100_000_000
	// Landing: the order is capped at what the rival has, the rest comes
	// back.
	sure := *cfg
	sure.Rivals.Poach.Odds = 1
	s = newSim(&sure)
	if err := w.BuyOff(6, 6*price); err != nil {
		t.Fatal(err)
	}
	if err := w.BuyOff(1, price); err != game.ErrPoaching {
		t.Fatalf("bought twice: %v", err)
	}
	if w.Player.DirtyCash != 100_000_000-6*price {
		t.Fatalf("paid %d", 100_000_000-w.Player.DirtyCash)
	}
	grudge := w.Rival.Grudge
	evs := step(w, s)
	w.Today.Poach = nil
	ev := find[events.RivalMusclePoached](evs)
	if ev == nil || ev.Failed || ev.Got != 2 || ev.Wanted != 6 || ev.Refund != 6*price*4/6 {
		t.Fatalf("landed: %+v", ev)
	}
	if w.Rival.Muscle != 0 || len(w.Crew.Members) != crew || w.Player.DirtyCash != 100_000_000-6*price+ev.Refund || w.Stats.Poached != 2 {
		t.Fatalf("after: muscle %d crew %d cash %d stats %+v", w.Rival.Muscle, len(w.Crew.Members), w.Player.DirtyCash, w.Stats)
	}
	// The heads sent home are out of its reach: it wants that many
	// fewer, and one comes back every away_days.
	if w.Rival.Away != 2 || w.Rival.AwayDay != w.Day || s.Want(w) != max(0, int(math.Round(cfg.Rivals.Personality["defensive"].MusclePerCorner*2))-2) {
		t.Fatalf("away %d since day %d, wants %d", w.Rival.Away, w.Rival.AwayDay, s.Want(w))
	}
	for i := 0; i < p.AwayDays; i++ {
		step(w, s)
	}
	if w.Rival.Away != 1 || w.Rival.AwayDay != w.Day {
		t.Fatalf("after %d days away %d since day %d (day %d)", p.AwayDays, w.Rival.Away, w.Rival.AwayDay, w.Day)
	}
	w.Rival.Away, w.Rival.AwayDay = 0, 0
	if w.Rival.Grudge+w.Rival.Tips != grudge {
		t.Fatalf("a buy-off that landed left a grudge: %d from %d", w.Rival.Grudge, grudge)
	}
	// Failing: the money is gone and it knows.
	never := *cfg
	never.Rivals.Poach.Odds = 0
	s = newSim(&never)
	w.Rival.Muscle = 2
	cash := w.Player.DirtyCash
	if err := w.BuyOff(2, 2*price); err != nil {
		t.Fatal(err)
	}
	grudge = w.Rival.Grudge
	evs = step(w, s)
	w.Today.Poach = nil
	ev = find[events.RivalMusclePoached](evs)
	if ev == nil || !ev.Failed || ev.Got != 0 || ev.Refund != 0 {
		t.Fatalf("failed: %+v", ev)
	}
	if w.Rival.Muscle != 2 || w.Player.DirtyCash != cash-2*price || w.Rival.Grudge+w.Rival.Tips < grudge+p.Grudge {
		t.Fatalf("after the failure: muscle %d cash %d grudge %d", w.Rival.Muscle, w.Player.DirtyCash, w.Rival.Grudge)
	}
	// Calling it off returns the money.
	if err := w.BuyOff(1, price); err != nil {
		t.Fatal(err)
	}
	w.CancelBuyOff()
	if w.Today.Poach != nil || w.Player.DirtyCash != cash-2*price {
		t.Fatalf("called off: %+v cash %d", w.Today.Poach, w.Player.DirtyCash)
	}
}

func newSim(cfg *content.Config) *rivals.Sim {
	return rivals.New(cfg)
}

// A scout reads the books as the night leaves them into Known, a
// snapshot nothing else writes: it stays put while the rival moves on,
// goes stale after stale_days, and a scout that reads nothing costs the
// money, stamps nothing and makes the next look likelier. A second look
// the same night is refused.
func TestScoutReadsASnapshot(t *testing.T) {
	cfg := content.MustLoad()
	bk := cfg.Rivals.Books
	w, s := warWorld(t, cfg, 19, "expansionist")
	w.Rival.Muscle = 5
	if w.Rival.Known.Read() {
		t.Fatal("the books are read before any scout")
	}
	if got, want := s.ScoutOdds(w), bk.ScoutBase+bk.ScoutSkill*0.8; math.Abs(got-want) > 1e-9 {
		t.Fatalf("odds %.2f, want %.2f with the best enforcer at 80", got, want)
	}
	// Nothing read: Known untouched, the next look likelier.
	never := *cfg
	never.Rivals.Poach.Odds = 0
	never.Rivals.Books.ScoutBase = -10
	s = newSim(&never)
	cash := w.Cash()
	if err := w.Scout(s.ScoutCost()); err != nil {
		t.Fatal(err)
	}
	if err := w.Scout(s.ScoutCost()); err != game.ErrScouting {
		t.Fatalf("scouted twice: %v", err)
	}
	if w.Cash() != cash-bk.ScoutCost {
		t.Fatalf("paid %d", cash-w.Cash())
	}
	evs := step(w, s)
	w.Today.Scouting = nil
	if ev := find[events.RivalScouted](evs); ev == nil || ev.Read || ev.Cost != bk.ScoutCost {
		t.Fatalf("read nothing: %+v", ev)
	}
	if w.Rival.Known.Read() || w.Rival.Scouted != 1 || w.Stats.Scouts != 1 {
		t.Fatalf("after an empty night: %+v scouted %d", w.Rival.Known, w.Rival.Scouted)
	}
	s = newSim(cfg)
	if got, want := s.ScoutOdds(w), bk.ScoutBase+bk.ScoutSkill*0.8+bk.ScoutLearn; math.Abs(got-want) > 1e-9 {
		t.Fatalf("odds after an empty night %.2f, want %.2f", got, want)
	}
	// Read: the snapshot is the night's closing books.
	sure := *cfg
	sure.Rivals.Books.ScoutBase = 1
	s = newSim(&sure)
	if err := w.Scout(s.ScoutCost()); err != nil {
		t.Fatal(err)
	}
	evs = step(w, s)
	w.Today.Scouting = nil
	if ev := find[events.RivalScouted](evs); ev == nil || !ev.Read {
		t.Fatalf("read: %+v", ev)
	}
	k := w.Rival.Known
	if !k.Read() || k.Day != w.Day || k.Cash != w.Rival.Cash || k.Muscle != w.Rival.Muscle || k.Income != s.Income(w) || k.Wages != s.Wages(w) || w.Rival.Scouted != 0 {
		t.Fatalf("the snapshot %+v against cash %d muscle %d income %d wages %d", k, w.Rival.Cash, w.Rival.Muscle, s.Income(w), s.Wages(w))
	}
	if s.Stale(w, w.Day) {
		t.Fatal("stale the night it was read")
	}
	// It stays put while the rival moves on, and goes stale.
	w.Rival.Cash += 12345
	for i := 0; i < bk.StaleDays; i++ {
		step(w, s)
		if w.Rival.Known != k {
			t.Fatalf("day %d: the snapshot moved: %+v", w.Day, w.Rival.Known)
		}
	}
	if !s.Stale(w, w.Day) || k.Age(w.Day) != bk.StaleDays {
		t.Fatalf("not stale after %d days: age %d", bk.StaleDays, k.Age(w.Day))
	}
}
