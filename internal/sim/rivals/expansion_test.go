package rivals_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
)

// away is the hub the expansion tests earn in: the second city.
const away = "bayport"

// expansionWorld is a two-city run with n factions at home, the rivals
// sim alone stepping it.
func expansionWorld(t *testing.T, n int, seed uint64) (*game.World, *rivals.Sim, *content.Config) {
	t.Helper()
	cfg := table(n)
	w := sim.NewWorld(cfg, seed)
	if w.Cities[away] == nil {
		t.Fatalf("no %s in the run: %v", away, w.CityOrder)
	}
	return w, rivals.New(cfg), cfg
}

// night steps the rivals sim over one day with the player's take in the
// hub, as the market would report it, and zeroes the day's scratch as
// the clock does.
func night(w *game.World, s *rivals.Sim, take int) []events.Event {
	var evs []events.Event
	if take > 0 {
		evs = append(evs, events.PlayerSold{Day: w.Day + 1, City: away, Product: gametest.Weed.ID, Sold: 1, Revenue: take})
	}
	got := gametest.Step(w, s, evs...).Events()
	w.ClearToday(w.Day)
	return got
}

func first[E events.Event](evs []events.Event) (E, bool) {
	for _, e := range evs {
		if ev, ok := e.(E); ok {
			return ev, true
		}
	}
	var zero E
	return zero, false
}

// The stages (#341): a take over the line in a city nobody lives in
// draws the last seat still in the wings, telegraphed: scouts the night
// the window crosses take_min, recruiting scout_days later, arrival
// arrive_days after that on a free corner there, and the faction is at
// home in the hub from then on.
func TestExpansionStages(t *testing.T) {
	w, s, cfg := expansionWorld(t, 4, 3)
	e := cfg.Rivals.Expansion
	daily := e.TakeMin/e.WindowDays + 1
	last := w.Rivals[3]
	var scout, recruit, arrive int
	for day := 1; day <= 60 && arrive == 0; day++ {
		evs := night(w, s, daily)
		if ev, ok := first[events.RivalScouting](evs); ok {
			if scout != 0 || ev.Faction != last.Faction() || ev.City != away || ev.Arrive != day+e.ScoutDays+e.ArriveDays || ev.Recruit != day+e.ScoutDays {
				t.Fatalf("day %d: scouting %+v (scouted before on day %d)", day, ev, scout)
			}
			scout = day
			if !last.Scouting() || last.Home != away || w.Stats.Moves != 1 {
				t.Fatalf("day %d: the seat is not on its way: %+v", day, *last)
			}
			if f, ok := game.Known(w).Fact(last.Faction(), game.FactScout); !ok || f.Value != away {
				t.Fatalf("day %d: no scout fact filed: %+v", day, f)
			}
		}
		if ev, ok := first[events.RivalRecruiting](evs); ok {
			recruit = day
			if ev.Bite != e.Bite || ev.Faction != last.Faction() {
				t.Fatalf("day %d: recruiting %+v", day, ev)
			}
		}
		if ev, ok := first[events.RivalMovedIn](evs); ok && ev.Faction == last.Faction() {
			arrive = day
			if c := w.Corner(ev.Corner); c == nil || c.City != away || c.FactionID() != last.Faction() {
				t.Fatalf("day %d: arrived on %+v", day, c)
			}
		}
	}
	if scout != e.WindowDays || recruit != scout+e.ScoutDays || arrive != recruit+e.ArriveDays {
		t.Fatalf("scouts on day %d, recruiting on %d, arrival on %d; want %d, %d, %d", scout, recruit, arrive, e.WindowDays, e.WindowDays+e.ScoutDays, e.WindowDays+e.ScoutDays+e.ArriveDays)
	}
	if last.Scouting() || w.CityOf(last).ID != away || w.Stats.Expanded != 1 || !last.Alive() {
		t.Fatalf("after the arrival: %+v, stats %d", *last, w.Stats.Expanded)
	}
	if _, ok := game.Known(w).Fact(last.Faction(), game.FactScout); ok {
		t.Fatal("the scout fact outlived the arrival")
	}
	// A city a faction lives in draws no second one.
	for day := 0; day < 20; day++ {
		if _, ok := first[events.RivalScouting](night(w, s, daily)); ok {
			t.Fatalf("a second faction drawn to %s", away)
		}
	}
}

// With every seat arrived, the strongest faction at home splits a cell
// off (#341): a new faction at the end of the table with start_muscle
// heads, the parent's temper and connect; the take falling back before
// the recruiting sends it home, the cell gone and its heads back with
// the parent, the window started over.
func TestExpansionSplitsACellAndItGoesHome(t *testing.T) {
	w, s, cfg := expansionWorld(t, 3, 5)
	e := cfg.Rivals.Expansion
	home := w.Home().Corners
	for i, r := range w.Rivals {
		seat(w, r, home[i].ID, 6)
	}
	seat(w, w.Rival(), home[3].ID, 6) // the rival at home holds two: the strongest
	parent := w.Rival()
	var cell *game.RivalState
	for day := 1; day <= e.WindowDays; day++ {
		night(w, s, e.TakeMin/e.WindowDays+1)
	}
	if len(w.Rivals) != 4 {
		t.Fatalf("no cell split off: %d factions", len(w.Rivals))
	}
	cell = w.Rivals[3]
	if cell.Cell != parent.Faction() || !cell.Scouting() || cell.Personality != parent.Personality || cell.Leader == "" || cell.Muscle != cfg.Rivals.Rivals.StartMuscle || cell.Cash <= 0 {
		t.Fatalf("the cell: %+v", *cell)
	}
	var went int
	for day := 0; day < e.ScoutDays && went == 0; day++ {
		evs := night(w, s, 0)
		if ev, ok := first[events.RivalWithdrew](evs); ok && ev.Faction == cell.Faction() {
			went = w.Day
		}
		if _, ok := first[events.RivalRecruiting](evs); ok {
			t.Fatal("it recruited with the take gone")
		}
	}
	if went == 0 {
		t.Fatal("the scouts never went home")
	}
	if len(w.Rivals) != 3 || w.Faction(cell.Faction()) != nil || w.Takes[away] != nil || w.Stats.Withdrew != 1 || cell.Muscle != 0 {
		t.Fatalf("after going home: %d factions, the cell %+v, window %v", len(w.Rivals), *cell, w.Takes[away])
	}
	if _, ok := game.Known(w).Fact(cell.Faction(), game.FactScout); ok {
		t.Fatal("the scout fact outlived the withdrawal")
	}
}

// Hitting the scouts (#341): an enforcer to send, once a faction; the
// move is set back setback_days and it holds a grudge.
func TestHitTheScouts(t *testing.T) {
	w, s, cfg := expansionWorld(t, 4, 3)
	e := cfg.Rivals.Expansion
	daily := e.TakeMin/e.WindowDays + 1
	for day := 1; day <= e.WindowDays; day++ {
		night(w, s, daily)
	}
	r := w.Rivals[3]
	if !r.Scouting() {
		t.Fatalf("not scouting: %+v", *r)
	}
	if err := w.HitScouts(r.Faction()); err != game.ErrNoEnforcers {
		t.Fatalf("with nobody to send: %v", err)
	}
	if err := w.HitScouts(w.Rival().Faction()); err != game.ErrNotScouting {
		t.Fatalf("the rival at home is not scouting: %v", err)
	}
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 9, Name: "Tank", Role: game.RoleEnforcer, Skill: 50, Loyalty: 70, Nerve: 50})
	due := s.ArriveDay(w, r)
	if err := w.HitScouts(r.Faction()); err != nil {
		t.Fatal(err)
	}
	evs := night(w, s, daily)
	ev, ok := first[events.ScoutsHit](evs)
	if !ok || ev.Setback != e.SetbackDays || ev.Arrive != due+e.SetbackDays || s.ArriveDay(w, r) != due+e.SetbackDays || r.Grudge != 1 {
		t.Fatalf("the hit: %+v, due %d, grudge %d", ev, s.ArriveDay(w, r), r.Grudge)
	}
	if err := w.HitScouts(r.Faction()); err != game.ErrScoutsHit {
		t.Fatalf("a second hit: %v", err)
	}
}

// A city with no free corner left (#341): the faction offers a tribute
// the night it recruits, and, the tribute refused, pushes its way in
// arrive_grace days after it was due; taken, it is paid every night and
// waits at peace.
func TestNoRoomAndTheTribute(t *testing.T) {
	for _, pay := range []bool{false, true} {
		w, s, cfg := expansionWorld(t, 4, 3)
		e := cfg.Rivals.Expansion
		cs := w.Cities[away].Corners
		for i := range cs {
			cs[i].Owner, cs[i].Yours = game.OwnerPlayer, true
		}
		w.Player.DirtyCash = 10_000_000
		daily := e.TakeMin/e.WindowDays + 1
		r := w.Rivals[3]
		offered, paid, forced := false, 0, 0
		for day := 1; day <= 80; day++ {
			evs := night(w, s, daily)
			if ev, ok := first[events.DealOffered](evs); ok && ev.Faction == r.Faction() {
				offered = true
				if ev.Deal != game.DealTribute {
					t.Fatalf("offered %s", ev.Deal)
				}
				if pay {
					if _, err := w.Accept(w.Offers[len(w.Offers)-1].ID); err != nil {
						t.Fatal(err)
					}
				}
			}
			for _, e := range evs {
				if ev, ok := e.(events.TributePaid); ok && ev.Faction == r.Faction() {
					paid++
				}
				if ev, ok := e.(events.CornerTaken); ok && ev.Faction == r.Faction() && ev.From == game.OwnerPlayer {
					forced = day
				}
			}
			if r.Arrived > 0 {
				break
			}
		}
		if !offered {
			t.Fatalf("pay %v: no tribute offered", pay)
		}
		switch {
		case pay && (paid == 0 || r.Arrived != 0 || forced != 0):
			t.Fatalf("paid: %d tributes, arrived %d, forced in on %d", paid, r.Arrived, forced)
		case !pay && (forced == 0 || r.Arrived != forced):
			t.Fatalf("refused: forced in on day %d, arrived %d", forced, r.Arrived)
		case !pay && forced < e.WindowDays+e.ScoutDays+e.ArriveDays+cfg.Rivals.Pace.ArriveGrace:
			t.Fatalf("forced in on day %d, before arrive_grace ran out", forced)
		}
	}
}

// The duel never expands and keeps no window (#341).
func TestTheDuelNeverExpands(t *testing.T) {
	cfg := duel()
	w := sim.NewWorld(cfg, 3)
	s := rivals.New(cfg)
	for day := 1; day <= 30; day++ {
		if _, ok := first[events.RivalScouting](night(w, s, cfg.Rivals.Expansion.TakeMin)); ok {
			t.Fatal("the duel drew a faction")
		}
	}
	if w.Takes != nil || len(w.Rivals) != 1 {
		t.Fatalf("the duel kept a window %v or grew a table of %d", w.Takes, len(w.Rivals))
	}
}

// While the crown waits only on clocks the money draws no new faction
// (#495): you hold the share at home, the rest of the table is gone bar
// one faction run out and back on a claim it has not yet kept
// settle_days, and a take over the line in the hub sends no scouts; the
// same faction settled on its corner splits a cell off to it.
func TestNoNewFactionWhileTheCrownWaits(t *testing.T) {
	for _, clock := range []bool{true, false} {
		w, s, cfg := expansionWorld(t, 3, 3)
		e := cfg.Rivals.Expansion
		for i, r := range w.Rivals {
			landless(w, r)
			r.Arrived = 1
			if i > 0 {
				r.Absorbed = 1
			}
		}
		home := w.Home()
		f := w.Rivals[0]
		seat(w, f, home.Corners[0].ID, 10)
		if clock {
			f.Routed, f.Spell = 1, 1
		}
		n := int(cfg.Rivals.Endings.KingpinShare*float64(len(home.Corners))) + 1
		daily := e.TakeMin/e.WindowDays + 1
		scouted := false
		for range e.WindowDays + 3 {
			// The ground as it stands, whatever the night moved: the
			// faction on its one corner, the share yours.
			landless(w, f)
			home.Corners[0].Owner, home.Corners[0].Faction = game.OwnerRival, f.Faction()
			for i := 1; i <= n; i++ {
				home.Corners[i].Owner, home.Corners[i].Faction = game.OwnerPlayer, ""
			}
			if s.CrownClock(w) != clock {
				t.Fatalf("clock %v: the crown's clock reads %v on day %d", clock, !clock, w.Day)
			}
			if _, ok := first[events.RivalScouting](night(w, s, daily)); ok {
				scouted = true
			}
		}
		if scouted == clock {
			t.Fatalf("clock %v: scouts sent %v, %d factions", clock, scouted, len(w.Rivals))
		}
	}
}
