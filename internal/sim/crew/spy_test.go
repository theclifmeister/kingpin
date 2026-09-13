package crew_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
)

// The spies (#45). A plant sends a member under the night it is
// queued: off the corner, not at work, their wage still paid, a
// SpyPlanted headline; every spy_days they file the faction's muscle,
// next move and till at the spy's odds; a report rolls the faction's
// spy_found, and a spy found is shot (off the roster, a body on the
// count) or comes home turned (an informant, the silent event the heat
// sim starts its clock on); a faction gone sends its spy home with
// nothing. A skill-90 spy reports the true muscle at least spy_accuracy
// of the time over 400 reports and a skill-10 spy materially less; and
// with nobody under the intel stream is never drawn.
func TestSpies(t *testing.T) {
	cfg := content.MustLoad()
	tun := cfg.Intel.Intel
	w, s := factionWorld(t, cfg)
	w.Crew.Members = []game.CrewMember{
		{ID: 901, Name: "Vee", Role: "runner", Skill: 90, Loyalty: 80, Greed: 20, Nerve: 60, Units: 100, Wage: 50},
		{ID: 902, Name: "Kay", Role: "runner", Skill: 10, Loyalty: 80, Greed: 20, Nerve: 60, Units: 100, Wage: 50},
	}
	w.Crew.NextID = 902
	if err := w.Post("oldmill", 901); err != nil {
		t.Fatal(err)
	}
	if err := w.PlantSpy("", 901); err != nil {
		t.Fatal(err)
	}
	cash := w.Cash()
	evs := step(w, s)
	if kinds(evs)["SpyPlanted"] != 1 {
		t.Fatalf("no SpyPlanted: %v", kinds(evs))
	}
	vee := w.Crew.Member(901)
	if vee.Undercover != game.FactionRival || vee.UndercoverDay != w.Day || vee.Working() || w.PostOf(901) != nil || w.Stats.Spies != 1 {
		t.Fatalf("planted: %+v post %v stats %+v", vee, w.PostOf(901), w.Stats)
	}
	if w.Cash() >= cash {
		t.Fatal("a spy's wage is not paid")
	}
	// No report before spy_days; on the day, three facts at the odds.
	never := *cfg
	never.Intel.Intel.SpyFound = map[string]float64{"expansionist": 0, "defensive": 0, "opportunist": 0, "chaotic": 0}
	s = crew.New(&never)
	for i := 1; i < tun.SpyDays; i++ {
		step(w, s)
		if len(w.Intel) != 0 {
			t.Fatalf("day %d: a report before its day: %+v", i, w.Intel)
		}
	}
	evs = step(w, s)
	if kinds(evs)["IntelGained"] != 3 || w.Stats.Reports != 1 {
		t.Fatalf("the first report: %v stats %+v", kinds(evs), w.Stats)
	}
	k := game.Known(w)
	if f, ok := k.Fact(game.FactionRival, game.FactMuscle); !ok || f.Source != game.SourceSpy || f.Confidence != tun.SpyOdds(90) {
		t.Fatalf("the muscle report %+v", f)
	}
	if c, ok := k.Move(game.FactionRival); !ok || c != "home" { // the one corner of yours it borders
		t.Fatalf("the move report %q %v", c, ok)
	}
	if c, ok := k.Stash(game.FactionRival); !ok || c != "docks" {
		t.Fatalf("the till report %q %v", c, ok)
	}
	// Accuracy by skill: 400 reports each, the skill-90 spy right at
	// least spy_accuracy of the time and the skill-10 spy well under it.
	right := func(id int) float64 {
		w, s := factionWorld(t, &never)
		w.Crew.Members = []game.CrewMember{{ID: id, Name: "S", Role: "runner", Skill: map[int]int{1: 90, 2: 10}[id], Loyalty: 80, Nerve: 60, Units: 100, Wage: 50, Undercover: game.FactionRival, UndercoverDay: 0}}
		w.Crew.NextID = id
		hits, n := 0, 0
		for w.Day < 400*tun.SpyDays {
			step(w, s)
			if (w.Day-0)%tun.SpyDays != 0 {
				continue
			}
			n++
			if f, ok := game.Known(w).Fact(game.FactionRival, game.FactMuscle); ok && int(f.Number) == w.Rival().Muscle {
				hits++
			}
		}
		return float64(hits) / float64(n)
	}
	good, bad := right(1), right(2)
	if good < tun.SpyAccuracy || bad > good-0.3 {
		t.Fatalf("a skill-90 spy was right %.2f of the time (want at least %.2f), a skill-10 spy %.2f", good, tun.SpyAccuracy, bad)
	}
	// Found and shot: off the roster, a body on the count, no informant.
	shot := *cfg
	shot.Intel.Intel.SpyFound = map[string]float64{"expansionist": 1, "defensive": 1, "opportunist": 1, "chaotic": 1}
	shot.Intel.Intel.TurnShare = 0
	w, s = factionWorld(t, &shot)
	w.Crew.Members = []game.CrewMember{{ID: 903, Name: "Dee", Role: "enforcer", Skill: 60, Loyalty: 80, Nerve: 60, Wage: 50, Undercover: game.FactionRival, UndercoverDay: 0}}
	w.Crew.NextID = 903
	for w.Day < tun.SpyDays {
		evs = step(w, s)
	}
	if kinds(evs)["SpyFound"] != 1 || w.Crew.Member(903) != nil || w.Stats.SpiesFound != 1 || w.Stats.SpiesShot != 1 || w.Stats.Bodies != 1 || w.Stats.Fallen != 1 || len(w.Crew.Fallen) != 1 {
		t.Fatalf("shot: %v member %v stats %+v", kinds(evs), w.Crew.Member(903), w.Stats)
	}
	for _, e := range evs {
		if ev, ok := e.(events.SpyFound); ok && (!ev.Dead || ev.Reports != 1 || ev.Faction != game.FactionRival) {
			t.Fatalf("the event %+v", ev)
		}
	}
	// Found and turned: home, an informant, the silent event.
	turned := shot
	turned.Intel.Intel.TurnShare = 1
	w, s = factionWorld(t, &turned)
	w.Crew.Members = []game.CrewMember{{ID: 904, Name: "Em", Role: "runner", Skill: 60, Loyalty: 80, Nerve: 60, Units: 100, Wage: 50, Undercover: game.FactionRival, UndercoverDay: 0}}
	w.Crew.NextID = 904
	for w.Day < tun.SpyDays {
		evs = step(w, s)
	}
	em := w.Crew.Member(904)
	if kinds(evs)["SpyFound"] != 1 || kinds(evs)["CrewTurnedInformant"] != 1 || em == nil || em.Undercover != "" || !em.Informant || !em.Working() || w.Stats.SpiesFound != 1 || w.Stats.SpiesShot != 0 || w.Stats.Informants != 1 {
		t.Fatalf("turned: %v member %+v stats %+v", kinds(evs), em, w.Stats)
	}
	for _, e := range evs {
		if ev, ok := e.(events.SpyFound); ok && (ev.Dead || ev.Why != "made") {
			t.Fatalf("the event %+v", ev)
		}
	}
	// A faction gone sends its spy home with nothing.
	w, s = factionWorld(t, cfg)
	w.Crew.Members = []game.CrewMember{{ID: 905, Name: "Jo", Role: "runner", Skill: 60, Loyalty: 80, Nerve: 60, Units: 100, Wage: 50, Undercover: game.FactionRival, UndercoverDay: 0}}
	w.Crew.NextID = 905
	w.Rival().Absorbed = 1
	evs = step(w, s)
	jo := w.Crew.Member(905)
	if kinds(evs)["SpyFound"] != 1 || jo == nil || jo.Undercover != "" || !jo.Working() || len(w.Intel) != 0 {
		t.Fatalf("home from a faction gone: %v %+v", kinds(evs), jo)
	}
	// Nobody under: the intel stream is never drawn, the file untouched.
	a, sa := factionWorld(t, cfg)
	b, sb := factionWorld(t, cfg)
	for i := 0; i < 3*tun.SpyDays; i++ {
		step(a, sa)
		step(b, sb)
	}
	if len(a.Intel) != 0 || a.Crew.NextID != b.Crew.NextID || len(a.Crew.Candidates) != len(b.Crew.Candidates) {
		t.Fatal("a run with nobody under is not the run it was")
	}
}
