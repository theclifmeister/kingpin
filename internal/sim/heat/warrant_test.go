package heat_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
)

// signedOn is the tick's WarrantSigned, or nil; lapsedOn its
// WarrantLapsed.
func signedOn(tk *game.Tick) *events.WarrantSigned {
	for _, e := range tk.Events() {
		if ev, ok := e.(events.WarrantSigned); ok {
			return &ev
		}
	}
	return nil
}

func lapsedOn(tk *game.Tick) *events.WarrantLapsed {
	for _, e := range tk.Events() {
		if ev, ok := e.(events.WarrantLapsed); ok {
			return &ev
		}
	}
	return nil
}

// The warrant (#475): the arrest line met signs one and nothing fires;
// the next night it is served on a sale anywhere, whatever the heat,
// or on the heat still at the line; a quiet night under the line lets
// it lapse, and the line met again signs another. A task force or an
// investigation landing on the night the line is met no longer hides
// the rung: the warrant is signed beside them. warrant_days 0 is the
// arrest the night the line is met.
func TestTheWarrant(t *testing.T) {
	cfg := blind(content.MustLoad())
	s := heat.New(cfg)
	arrest := rung(cfg, content.Arrest)
	if cfg.Heat.Heat.WarrantDays != 1 {
		t.Fatalf("warrant_days %d: this test reads a night's warning", cfg.Heat.Heat.WarrantDays)
	}
	sign := func(w *game.World) {
		t.Helper()
		w.Home().Heat = over(w, s, arrest, w.Home())
		tk := step(w, s, sale(w, w.Home().ID, 10))
		ev := signedOn(tk)
		if ev == nil || ev.City != w.Home().ID || ev.Due != w.Day+1 || ev.Heat < ev.Line || enforcement(tk) != nil || w.Heat.WarrantDay != w.Day || w.Over != nil {
			t.Fatalf("the line met: signed %+v, enforcement %+v, warrant %d on day %d, over %+v", ev, enforcement(tk), w.Heat.WarrantDay, w.Day, w.Over)
		}
	}

	// Served on a sale, in the other city, with the heat under the line.
	w := world(t, cfg)
	w.SetStock(w.Home().ID, w.Products[0], 1000)
	sign(w)
	w.Home().Heat = 50
	tk := step(w, s, sale(w, w.CityOrder[1], 1))
	if ev := enforcement(tk); ev == nil || ev.Level != content.Arrest || w.Over == nil || w.Over.Cause != content.CauseArrested || w.Heat.WarrantDay != 0 {
		t.Fatalf("a sale the night it was due: enforcement %+v over %+v", ev, w.Over)
	}

	// Served on the heat still at the line, nothing sold.
	w = world(t, cfg)
	sign(w)
	w.Home().Heat = over(w, s, arrest, w.Home())
	if ev := enforcement(step(w, s)); ev == nil || ev.Level != content.Arrest || w.Over == nil {
		t.Fatalf("the heat held the night it was due: enforcement %+v over %+v", ev, w.Over)
	}

	// A quiet night under the line: it lapses, and nothing fires that
	// the heat does not call for. Met again, another is signed.
	w = world(t, cfg)
	sign(w)
	w.Home().Heat = 50
	tk = step(w, s)
	if ev := lapsedOn(tk); ev == nil || ev.Signed != w.Day-1 || ev.Heat >= ev.Line || w.Over != nil || w.Heat.WarrantDay != 0 {
		t.Fatalf("a quiet night under the line: lapsed %+v, over %+v, warrant %d", ev, w.Over, w.Heat.WarrantDay)
	}
	if ev := enforcement(tk); ev != nil && ev.Level == content.Arrest {
		t.Fatalf("a lapsed warrant arrested: %+v", ev)
	}
	sign(w)

	// The night a task force comes (#48) the line met signs the warrant
	// too; it used to skip the arrest rung whole.
	w = world(t, cfg)
	owning(w, "port")
	step(w, s) // a quiet day 1: the task force is announced on a day past 0
	w.Heat.TaskForceDay = w.Day
	w.Home().Heat = over(w, s, arrest, w.Home())
	tk = step(w, s)
	if ev := enforcement(tk); ev == nil || ev.Level != content.TaskForce || signedOn(tk) == nil || w.Heat.WarrantDay != w.Day {
		t.Fatalf("a task force night over the arrest line: enforcement %+v, signed %+v", ev, signedOn(tk))
	}

	// And the night an investigation lands (#343).
	cfgInv := investigating(content.MustLoad())
	si := heat.New(cfgInv)
	w = world(t, cfgInv)
	w.Heat.Investigation = game.Investigation{City: w.Home().ID, Kind: game.LeadCorner, Target: w.Home().Corners[0].ID, Opened: w.Day, Due: w.Day + 1}
	w.Home().Heat = over(w, si, arrest, w.Home())
	tk = step(w, si)
	if signedOn(tk) == nil || w.Heat.WarrantDay != w.Day || w.Heat.Investigation.Open() {
		t.Fatalf("an investigation night over the arrest line: signed %+v, investigation %+v", signedOn(tk), w.Heat.Investigation)
	}

	// warrant_days 0: the arrest the night the line is met.
	off := *cfg
	off.Heat.Heat.WarrantDays = 0
	so := heat.New(&off)
	w = world(t, &off)
	w.Home().Heat = over(w, so, arrest, w.Home())
	tk = step(w, so)
	if ev := enforcement(tk); ev == nil || ev.Level != content.Arrest || w.Over == nil || signedOn(tk) != nil {
		t.Fatalf("warrant_days 0: enforcement %+v over %+v", ev, w.Over)
	}
}
