package heat_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
)

// A cop paid (#45) files the police's next move where you stand: the
// rung the ladder stands at and the first night it can fire (Next), at
// cop_accuracy for the price, less in proportion under it. Over 500
// nights on 500 seeds the word is right within cop_accuracy ± 0.05, a
// wrong word is a rung beside the truth or a few days out, never off
// the ladder, and a night with no cop files nothing and rolls nothing.
func TestCopIsRightAtTheAccuracy(t *testing.T) {
	cfg := content.MustLoad()
	tun := cfg.Intel.Intel
	s := heat.New(cfg)
	right, wrong := 0, 0
	for seed := uint64(1); seed <= 500; seed++ {
		w := world(t, cfg)
		w.Seed = seed
		w.Player.DirtyCash = 100_000
		here := w.Here()
		here.Heat = float64(20 + seed%70) // every rung's neighbourhood
		if seed%3 == 0 {
			w.Heat.LastResponse[content.Sting] = w.Day // a cooldown to read
		}
		if err := w.PayCop(tun.CopPrice); err != nil {
			t.Fatal(err)
		}
		tk := step(w, s)
		level, from := s.Next(w, here, w.Day)
		f, ok := game.Known(w).Fact(here.ID, game.FactResponse)
		if !ok || f.Source != game.SourceCop || !near(f.Confidence, tun.CopAccuracy) || f.Day != w.Day {
			t.Fatalf("seed %d: the cop's word %+v %v", seed, f, ok)
		}
		gained := false
		for _, e := range tk.Events() {
			if ev, isGain := e.(events.IntelGained); isGain && ev.FactKind == game.FactResponse {
				gained = true
			}
		}
		if !gained {
			t.Fatalf("seed %d: no IntelGained for the cop's word", seed)
		}
		if content.Rank(f.Value) == 0 {
			t.Fatalf("seed %d: a rung off the ladder %q", seed, f.Value)
		}
		if f.Value == level && int(f.Number) == from {
			right++
			continue
		}
		wrong++
		if f.Value != level && int(f.Number) != from {
			t.Fatalf("seed %d: wrong on both counts: %s d%.0f against %s d%d", seed, f.Value, f.Number, level, from)
		}
		if f.Value != level && int(math.Abs(float64(content.Rank(f.Value)-content.Rank(level)))) != 1 {
			t.Fatalf("seed %d: %s is not the rung beside %s", seed, f.Value, level)
		}
		if f.Value == level && (int(f.Number) <= from || int(f.Number) > from+tun.CopSlip) {
			t.Fatalf("seed %d: d%.0f is not 1..%d nights past d%d", seed, f.Number, tun.CopSlip, from)
		}
	}
	if got := float64(right) / float64(right+wrong); math.Abs(got-tun.CopAccuracy) > 0.05 {
		t.Fatalf("the cop was right %.3f of the time over %d nights, want %.2f ± 0.05", got, right+wrong, tun.CopAccuracy)
	}
	// Half the price is half the accuracy; the file says so.
	w := world(t, cfg)
	w.Player.DirtyCash = 100_000
	if err := w.PayCop(tun.CopPrice / 2); err != nil {
		t.Fatal(err)
	}
	step(w, s)
	if f, ok := game.Known(w).Fact(w.Here().ID, game.FactResponse); !ok || !near(f.Confidence, tun.CopAccuracy/2) {
		t.Fatalf("half the price: %+v", f)
	}
	// No cop, no fact, and nothing off the intel stream: a night with
	// and without the feature is the same night.
	a, b := world(t, cfg), world(t, cfg)
	step(a, s)
	step(b, heat.New(cfg))
	if len(a.Intel) != 0 || a.Here().Heat != b.Here().Heat {
		t.Fatalf("a night with no cop: %+v", a.Intel)
	}
}

// Next is the truth the cop's word is measured against: the highest
// rung the heat is at or over, the patrol under every line, and the
// first night the cooldown lets it fire, tomorrow with none.
func TestNextRung(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	w := world(t, cfg)
	here := w.Here()
	w.Day = 10
	here.Heat = 0
	if level, from := s.Next(w, here, w.Day); level != content.Patrol || from != 11 {
		t.Fatalf("cold: %s d%d", level, from)
	}
	here.Heat = s.Threshold(w, rung(cfg, content.Sting), here) + 1
	if level, from := s.Next(w, here, w.Day); level != content.Sting || from != 11 {
		t.Fatalf("over the sting line: %s d%d", level, from)
	}
	w.Heat.LastResponse[content.Sting] = 9
	if level, from := s.Next(w, here, w.Day); level != content.Sting || from != 9+s.CooldownDays(w, content.Sting) {
		t.Fatalf("in the cooldown: %s d%d, want d%d", level, from, 9+s.CooldownDays(w, content.Sting))
	}
	here.Heat = 100
	w.Heat.LastResponse[content.Arrest] = 10
	if level, from := s.Next(w, here, w.Day); level != content.Arrest || from != 11 {
		t.Fatalf("the arrest has no cooldown: %s d%d", level, from)
	}
}
