package rivals_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// The crown's "crews down" read (#472) is the rule absorb applies, not
// a guess at it: a faction another took the last corner off reads
// GoneOn as the day it is absorbed, one routed by you and broke the day
// it scatters, one that can afford a claim strand_days after its rout,
// and one that holds a corner, pays homage or is gone reads as such.
func TestDownIsTheAbsorbRule(t *testing.T) {
	cfg := table(3)
	w, s := world(t, cfg, 4)
	f1, f2, f3 := w.Rivals[0], w.Rivals[1], w.Rivals[2]
	seat(w, f1, "oldmill", cfg.Rivals.Rivals.StartMuscle)
	absorb, strand := cfg.Rivals.Factions.AbsorbDays, cfg.Rivals.Factions.StrandDays

	if d := s.Down(w, f1); d.Counts || d.Corners != 1 || d.GoneOn != 0 {
		t.Fatalf("a faction on a corner: %+v", d)
	}
	if d := s.Down(w, f3); d.Counts || d.Due != s.ArriveDay(w, f3) || d.GoneOn != d.Due+strand {
		t.Fatalf("a seat in the wings: %+v (arrives day %d)", d, s.ArriveDay(w, f3))
	}

	// Taken by another faction: absorbed on GoneOn, whatever its chest.
	f2.Arrived, f2.Observed, f2.Muscle, f2.Cash = 1, true, 3, 1_000_000
	f2.Routed, f2.LastTakenBy = 30, f1.Faction()
	w.Day = 30 + absorb - 5
	want := s.Down(w, f2)
	if want.Counts || want.Since != 30 || want.GoneOn != 30+absorb || want.Rich {
		t.Fatalf("taken by a faction: %+v", want)
	}
	for !f2.Gone() {
		if w.Day > 30+absorb+5 {
			t.Fatalf("never absorbed; the read said day %d", want.GoneOn)
		}
		step(w, s)
	}
	if f2.Absorbed != want.GoneOn {
		t.Fatalf("absorbed on day %d, the read said day %d", f2.Absorbed, want.GoneOn)
	}
	if d := s.Down(w, f2); !d.Counts {
		t.Fatalf("an absorbed faction does not count: %+v", d)
	}

	// Routed by you and broke: it scatters on GoneOn.
	landless(w, f3)
	f3.Arrived, f3.Observed, f3.Muscle, f3.Cash = 1, true, 0, 0
	f3.Routed, f3.LastTakenBy = w.Day, ""
	want = s.Down(w, f3)
	if want.Rich || want.GoneOn != f3.Routed+absorb {
		t.Fatalf("routed by you and broke: %+v", want)
	}
	for !f3.Gone() {
		if w.Day > want.GoneOn+5 {
			t.Fatalf("never scattered; the read said day %d", want.GoneOn)
		}
		f3.Cash = 0
		step(w, s)
	}
	if f3.Absorbed != want.GoneOn || f3.AbsorbedBy != "" {
		t.Fatalf("scattered on day %d by %q, the read said day %d", f3.Absorbed, f3.AbsorbedBy, want.GoneOn)
	}

	// Routed by you with the chest for a claim: strand_days, not absorb.
	f1.Cash = 10 * s.ClaimCost(w, f1)
	landless(w, f1)
	f1.Routed, f1.LastTakenBy = w.Day, ""
	if d := s.Down(w, f1); !d.Rich || d.GoneOn != f1.Routed+strand {
		t.Fatalf("routed by you with the money for a claim: %+v", d)
	}

	// Paying you homage counts.
	f1.Deals = []game.Deal{{Kind: game.DealHomage, Terms: game.Terms{PerDay: 10}, Since: w.Day, Faction: f1.Faction()}}
	if d := s.Down(w, f1); !d.Counts {
		t.Fatalf("a faction paying homage does not count: %+v", d)
	}
}

// landless takes every corner a faction holds back to the street.
func landless(w *game.World, r *game.RivalState) {
	for _, cid := range w.CityOrder {
		cs := w.Cities[cid].Corners
		for i := range cs {
			if cs[i].FactionID() == r.Faction() {
				cs[i].Owner, cs[i].Faction = game.OwnerNone, ""
			}
		}
	}
}

// A claim it did not keep does not restart a run-out faction's clock
// (#495): routed broke, it claims a corner and loses it again inside
// settle_days, and it scatters absorb_days after the first rout, as the
// read says the morning it loses it; one that keeps its claim
// settle_days has settled, and its next rout starts a new clock.
func TestAClaimNotKeptKeepsTheClock(t *testing.T) {
	cfg := table(3)
	settle, absorb := cfg.Rivals.Factions.SettleDays, cfg.Rivals.Factions.AbsorbDays
	if settle <= 0 {
		t.Skip("settle_days is boxed")
	}
	for _, hold := range []int{5, settle + 1} {
		w, s := world(t, cfg, 4)
		f := w.Rivals[2]
		for _, o := range w.Rivals[:2] { // nobody else to push it off its claim
			landless(w, o)
			o.Arrived, o.Absorbed = 1, 1
		}
		landless(w, f)
		f.Arrived, f.Observed, f.Muscle, f.Cash = 1, true, 0, 0
		w.Day = 40
		f.Routed, f.LastTakenBy = w.Day, ""
		first := w.Day
		for range 5 {
			f.Cash = 0
			step(w, s)
		}
		if f.Gone() || f.RunOut() != first {
			t.Fatalf("hold %d: landless 5 nights, gone %v, run out day %d, want %d", hold, f.Gone(), f.RunOut(), first)
		}
		c := w.Corner("oldmill")
		c.Owner, c.Faction, c.Runner, c.Enforcer, c.Since = game.OwnerRival, f.Faction(), 0, 0, w.Day
		for range hold {
			step(w, s)
			if c.FactionID() != f.Faction() {
				t.Fatalf("hold %d: the corner changed hands on day %d", hold, w.Day)
			}
		}
		if d := s.Down(w, f); d.Corners != 1 || (hold < settle) != (d.Settles > 0) {
			t.Fatalf("hold %d: on its claim %+v", hold, d)
		}
		landless(w, f)
		f.Routed, f.Cash = w.Day, 0
		want := first
		if hold >= settle {
			want = w.Day
		}
		d := s.Down(w, f)
		if d.Since != want || d.GoneOn != max(w.Day+1, want+absorb) {
			t.Fatalf("hold %d: routed again on day %d: %+v, want the clock from day %d", hold, w.Day, d, want)
		}
		for !f.Gone() {
			if w.Day > d.GoneOn+5 {
				t.Fatalf("hold %d: never scattered; the read said day %d", hold, d.GoneOn)
			}
			f.Cash = 0
			step(w, s)
		}
		if f.Absorbed != d.GoneOn {
			t.Fatalf("hold %d: scattered on day %d, the read said day %d", hold, f.Absorbed, d.GoneOn)
		}
	}
}
