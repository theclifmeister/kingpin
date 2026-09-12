package harness

import (
	"reflect"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// incidents lists what a run dealt, as "day:id".
func incidents(res Result) []string {
	var out []string
	for _, e := range res.Events {
		if ev, ok := e.(events.Incident); ok {
			out = append(out, string(rune('0'+ev.Day/100))+string(rune('0'+ev.Day/10%10))+string(rune('0'+ev.Day%10))+":"+ev.ID)
		}
	}
	return out
}

// Every incident in the table fires at least once across 200 seeds x
// 300 days with the table on, and none of them renders with a hole in
// it: the headline under the world source and the report's line. A
// rich idle player, so the rows that read the operation's size
// (peak_cash_min) are eligible and nothing ends the run.
func TestEveryIncidentFires(t *testing.T) {
	cfg := content.MustLoad()
	if len(cfg.Incidents.Table) < 11 {
		t.Fatalf("the table has %d rows; the issue asks for the starter eleven (rival_leader_killed waits for #43)", len(cfg.Incidents.Table))
	}
	seen := map[string]int{}
	holes := 0
	for seed := uint64(1); seed <= 200; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Player.DirtyCash = 300_000
		res, err := Play(cfg, w, 300, Idle, Options{Incidents: true})
		if err != nil {
			t.Fatal(err)
		}
		if res.Over != nil {
			t.Fatalf("seed %d: the idle player's run ended: %+v", seed, res.Over)
		}
		fired := map[int]events.Incident{}
		for _, e := range res.Events {
			if ev, ok := e.(events.Incident); ok {
				seen[ev.ID]++
				if _, twice := fired[ev.Day]; twice {
					t.Fatalf("seed %d: two incidents on day %d", seed, ev.Day)
				}
				fired[ev.Day] = ev
			}
		}
		if len(fired) != len(res.World.Incidents.Fired) {
			t.Fatalf("seed %d: %d incidents fired, %d recorded", seed, len(fired), len(res.World.Incidents.Fired))
		}
		for _, h := range res.World.Journal {
			if h.Source != "world" {
				continue
			}
			if _, ok := fired[h.Day]; !ok {
				t.Fatalf("seed %d: a world headline on day %d with no incident: %q", seed, h.Day, h.Text)
			}
			if hole(h.Text) {
				holes++
				t.Errorf("seed %d day %d: headline %q has a hole", seed, h.Day, h.Text)
			}
		}
		if holes > 10 {
			t.Fatal("stopping at ten holes")
		}
	}
	var missing []string
	for _, inc := range cfg.Incidents.Table {
		if seen[inc.ID] == 0 {
			missing = append(missing, inc.ID)
		}
	}
	if len(missing) > 0 {
		t.Errorf("incidents never dealt across 200 seeds x 300 days: %v (seen %v)", missing, seen)
	}
}

// hole is a template slot that did not fill.
func hole(s string) bool {
	return strings.Contains(s, "<no value>") || strings.Contains(s, "{{") || strings.Contains(s, "  ") || strings.HasPrefix(s, " ") || strings.Contains(s, " in .") || strings.Contains(s, "Chief ;") || strings.Contains(s, "DA ;")
}

// The report opens with the incident the morning it fires, and the line
// reads clean.
func TestIncidentOpensTheReport(t *testing.T) {
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, 3)
	w.Player.DirtyCash = 300_000
	lines := 0
	_, err := Play(cfg, w, 120, func(w *game.World) {
		r := w.Report
		if r == nil {
			return
		}
		fired := len(w.Incidents.Fired) > 0 && w.Incidents.Fired[len(w.Incidents.Fired)-1].Day == w.Day
		if fired != (len(r.Incident) > 0) {
			t.Fatalf("day %d: incident fired %v, report has %v", w.Day, fired, r.Incident)
		}
		for _, l := range r.Incident {
			lines++
			if hole(l) || !strings.HasSuffix(l, ".") {
				t.Errorf("day %d: report line %q", w.Day, l)
			}
		}
	}, Options{Incidents: true})
	if err != nil {
		t.Fatal(err)
	}
	if lines == 0 {
		t.Fatal("no incident in 120 days")
	}
}

// The incident stream for a seed is the seed's: two players who do
// nothing alike see the same incidents on the same days (the pacing and
// the pick roll on the incidents' own stream whatever happens, as the
// deck's do; only a row's trigger reads the player), and a run stopped
// on the morning of an incident, saved and continued plays out exactly
// like one that never stopped.
func TestIncidentStreamIsTheSeeds(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 3; seed++ {
		a, err := Play(cfg, sim.NewWorld(cfg, seed), 200, Idle, Options{Incidents: true})
		if err != nil {
			t.Fatal(err)
		}
		b, err := Play(cfg, sim.NewWorld(cfg, seed), 200, Hide, Options{Incidents: true})
		if err != nil {
			t.Fatal(err)
		}
		if ia, ib := incidents(a), incidents(b); !reflect.DeepEqual(ia, ib) {
			t.Fatalf("seed %d: idle and hiding see different incidents:\n%v\n%v", seed, ia, ib)
		}
		if len(incidents(a)) < 200/(cfg.Incidents.Incidents.MaxGap+2) {
			t.Fatalf("seed %d: only %d incidents in 200 days", seed, len(incidents(a)))
		}
	}

	// The save on the morning of one.
	live := *cfg
	_, sims, err := sim.Default(&live)
	if err != nil {
		t.Fatal(err)
	}
	policy := Crewed(cfg, 40)
	play := func(w *game.World, days int) {
		clock := game.NewClock(nil, sims...)
		for d := 0; d < days && w.Over == nil; d++ {
			policy(w)
			clock.EndDay(w)
		}
	}
	a := sim.NewWorld(cfg, 11)
	play(a, 90)
	b := sim.NewWorld(cfg, 11)
	clock := game.NewClock(nil, sims...)
	for len(b.Incidents.Fired) == 0 {
		policy(b)
		clock.EndDay(b)
	}
	if err := game.Save(1, b); err != nil {
		t.Fatal(err)
	}
	b2, err := game.Load(1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b2.Incidents, b.Incidents) {
		t.Fatalf("the incidents did not come back: %+v vs %+v", b2.Incidents, b.Incidents)
	}
	play(b2, 90-b2.Day)
	if !reflect.DeepEqual(a.Incidents.Fired, b2.Incidents.Fired) {
		t.Fatalf("incidents differ after a save on one:\n%v\n%v", a.Incidents.Fired, b2.Incidents.Fired)
	}
	if a.Cash() != b2.Cash() || a.Day != b2.Day || a.NetWorth() != b2.NetWorth() {
		t.Fatalf("worlds differ after a save on an incident: day %d $%d vs day %d $%d", a.Day, a.Cash(), b2.Day, b2.Cash())
	}
}

// A run with the table boxed is the run before the world sim existed:
// nothing fires, nothing is recorded, no route is ever shut, and the
// harness's default (Run, RunFrom, RunWith) boxes it. TestSeedDigest
// pins the rest of the world for it.
func TestNoIncidentsIsTheOldRun(t *testing.T) {
	cfg := content.MustLoad()
	for _, run := range []struct {
		name string
		play func(w *game.World) (Result, error)
	}{
		{"Run", func(w *game.World) (Result, error) { return Run(cfg, w.Seed, 150, Boss(cfg, 40, "")) }},
		{"RunWith", func(w *game.World) (Result, error) { return RunWith(cfg, w, 150, Boss(cfg, 40, ""), Decline) }},
	} {
		res, err := run.play(sim.NewWorld(cfg, 7))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Events {
			if ev, ok := e.(events.Incident); ok {
				t.Fatalf("%s: %+v in a run with the table boxed", run.name, ev)
			}
		}
		if w := res.World; !reflect.DeepEqual(w.Incidents, game.IncidentState{}) || w.Heat.FederalUntil != 0 || w.Law.SnapElection != 0 {
			t.Fatalf("%s: the world carries incident state with the table boxed: %+v", run.name, w.Incidents)
		}
		for id, rs := range res.World.Routes {
			if rs.ClosedUntil != 0 {
				t.Fatalf("%s: route %s was shut with the table boxed", run.name, id)
			}
		}
	}
}

// With the table on, the difficulty ordering and the money bands hold:
// incidents are weather, not difficulty. The medians with and without
// are logged so the PR can name the delta.
func TestIncidentsAreWeather(t *testing.T) {
	cfg := content.MustLoad()
	median := func(xs []int) int {
		sortInts(xs)
		return xs[len(xs)/2]
	}
	for _, row := range []struct {
		name   string
		day    int
		policy func() Policy
		lo, hi int
	}{
		{"managed", TierDays[0], func() Policy { return Managed(cfg, 40) }, 50_000, 200_000},
		{"crewed", TierDays[1], func() Policy { return Crewed(cfg, 40) }, 500_000, 2_000_000},
	} {
		var with, without []int
		indicted := 0
		for seed := uint64(1); seed <= 20; seed++ {
			a, err := Play(cfg, sim.NewWorld(cfg, seed), row.day, row.policy(), Options{Incidents: true})
			if err != nil {
				t.Fatal(err)
			}
			b, err := Play(cfg, sim.NewWorld(cfg, seed), row.day, row.policy(), Options{})
			if err != nil {
				t.Fatal(err)
			}
			with = append(with, a.NetWorthAt(row.day))
			without = append(without, b.NetWorthAt(row.day))
			if a.Over != nil {
				indicted++
			}
		}
		mw, mo := median(with), median(without)
		t.Logf("%s at day %d: median net worth %d with incidents, %d without (%+.1f%%), %d of 20 runs ended", row.name, row.day, mw, mo, 100*float64(mw-mo)/float64(mo), indicted)
		if mw < row.lo || mw > row.hi {
			t.Errorf("%s at day %d: median %d with incidents is outside the tier's band %d-%d", row.name, row.day, mw, row.lo, row.hi)
		}
		if indicted > 2 {
			t.Errorf("%s: %d of 20 runs ended with incidents on; they are weather, not difficulty", row.name, indicted)
		}
	}
	// The ordering: aggressive is indicted with the table on as without,
	// and quiet survives it.
	ended := 0
	for seed := uint64(1); seed <= 10; seed++ {
		a, err := Play(cfg, sim.NewWorld(cfg, seed), Horizon, Trader(cfg, events.DialAggressive), Options{Incidents: true})
		if err != nil {
			t.Fatal(err)
		}
		if a.Over != nil {
			ended++
		}
		q, err := Play(cfg, sim.NewWorld(cfg, seed), Horizon, Trader(cfg, events.DialQuiet), Options{Incidents: true})
		if err != nil {
			t.Fatal(err)
		}
		if q.Over != nil {
			t.Errorf("seed %d: the quiet trader's run ended with incidents on: %+v", seed, q.Over)
		}
	}
	if ended < 8 {
		t.Errorf("only %d of 10 aggressive runs ended with incidents on", ended)
	}
}

// A snap election is held the morning it is due and resets the term; a
// chief who resigns is replaced the same morning, and the headline names
// the one who left.
func TestSnapElectionAndResignation(t *testing.T) {
	cfg := content.MustLoad()
	only := func(id string) *content.Config {
		c := *cfg
		c.Incidents.Table = nil
		for _, inc := range cfg.Incidents.Table {
			if inc.ID == id {
				inc.MinDay, inc.MinGap, inc.Once = 1, 0, true
				c.Incidents.Table = append(c.Incidents.Table, inc)
			}
		}
		if len(c.Incidents.Table) != 1 {
			t.Fatalf("no incident %s in the table", id)
		}
		return &c
	}

	// The election.
	c := only("election_called")
	snap := c.Incidents.Table[0].Effects.ElectionCalled
	res, err := Play(c, sim.NewWorld(c, 5), 60, Idle, Options{Incidents: true})
	if err != nil {
		t.Fatal(err)
	}
	called, held := 0, 0
	for _, e := range res.Events {
		switch ev := e.(type) {
		case events.Incident:
			called = ev.Day
			if ev.Election != snap || ev.DA == "" {
				t.Fatalf("incident %+v", ev)
			}
		case events.DAElected:
			held = ev.Day
		}
	}
	if called == 0 || held != called+snap {
		t.Fatalf("election called on day %d, held on day %d; want %d", called, held, called+snap)
	}
	if w := res.World; w.Law.DA.ElectedDay != held || w.Law.SnapElection != 0 {
		t.Fatalf("after the snap election: elected day %d, snap %d", w.Law.DA.ElectedDay, w.Law.SnapElection)
	}
	if next := cfg.Law.Law.TermDays; next > 0 && held >= next {
		t.Fatalf("the snap election on day %d came after the term's own on day %d", held, next)
	}

	// The chief.
	c = only("chief_resigns")
	res, err = Play(c, sim.NewWorld(c, 5), 60, Idle, Options{Incidents: true})
	if err != nil {
		t.Fatal(err)
	}
	old := ""
	day, replaced := 0, 0
	for _, e := range res.Events {
		switch ev := e.(type) {
		case events.Incident:
			day, old = ev.Day, ev.Chief
		case events.ChiefReplaced:
			if ev.Why == "resigned" {
				replaced = ev.Day
				if ev.Old != old {
					t.Fatalf("the chief who resigned was %s, the one replaced %s", old, ev.Old)
				}
			}
		}
	}
	if day == 0 || replaced != day {
		t.Fatalf("the chief resigned on day %d and was replaced on day %d", day, replaced)
	}
	if res.World.Law.Chief.Name == old || res.World.Law.Chief.Since != day {
		t.Fatalf("the chief after the resignation: %+v (was %s)", res.World.Law.Chief, old)
	}
	named := false
	for _, h := range res.World.Journal {
		if h.Day == day && h.Source == "world" {
			named = strings.Contains(h.Text, old)
			if !named {
				t.Fatalf("the headline %q does not name %s, who resigned", h.Text, old)
			}
		}
	}
	if !named {
		t.Fatal("no world headline the morning the chief resigned")
	}
}

func sortInts(xs []int) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j-1] > xs[j]; j-- {
			xs[j-1], xs[j] = xs[j], xs[j-1]
		}
	}
}
