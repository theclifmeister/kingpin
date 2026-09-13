package law_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/law"
)

// The chief's fact (#45): observed, the temper is filed at full
// confidence and never fades; a cop paid files it at cop_accuracy
// before that, a wrong word naming one of the other tempers, right
// within ± 0.05 of the accuracy over 400 seeds; the observed fact is
// not replaced by a cop's; and a chief replaced takes the fact with
// them, so the new one reads Unknown until seen.
func TestChiefFact(t *testing.T) {
	cfg := content.MustLoad()
	tun := cfg.Intel.Intel
	s := law.New(cfg)
	// Observed by time in office: the fact at 1, once, with its event.
	w := sim.NewWorld(cfg, 5)
	w.Law.Chief.Since = 0
	day := cfg.Law.Law.ObserveDays
	tk := tick(w, day)
	s.Step(w, tk)
	if !w.Law.Chief.Observed || kinds(tk)["IntelGained"] != 1 {
		t.Fatalf("observed %v events %v", w.Law.Chief.Observed, kinds(tk))
	}
	if f, ok := game.Known(w).Fact(game.SubjectChief, game.FactPersonality); !ok || f.Value != w.Law.Chief.Personality || f.Confidence != 1 || f.Stale != 0 || f.Source != game.SourceSeen {
		t.Fatalf("the chief's fact %+v", f)
	}
	tk = tick(w, day+1)
	s.Step(w, tk)
	if kinds(tk)["IntelGained"] != 0 || len(w.Intel) != 1 {
		t.Fatalf("filed twice: %v %+v", kinds(tk), w.Intel)
	}
	// A cop paid with the chief observed adds nothing; before that, the
	// word at the accuracy, right the right share of the time.
	w.Player.DirtyCash = 100_000
	if err := w.PayCop(tun.CopPrice); err != nil {
		t.Fatal(err)
	}
	tk = tick(w, day+2)
	s.Step(w, tk)
	if f, _ := game.Known(w).Fact(game.SubjectChief, game.FactPersonality); f.Confidence != 1 || f.Source != game.SourceSeen {
		t.Fatalf("a cop's word replaced the observed fact: %+v", f)
	}
	right, wrong := 0, 0
	for seed := uint64(1); seed <= 400; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Law.Chief.Since = 0
		w.Player.DirtyCash = 100_000
		if err := w.PayCop(tun.CopPrice); err != nil {
			t.Fatal(err)
		}
		tk := tick(w, 1)
		s.Step(w, tk)
		f, ok := game.Known(w).Fact(game.SubjectChief, game.FactPersonality)
		if !ok || f.Source != game.SourceCop || math.Abs(f.Confidence-tun.CopAccuracy) > 1e-9 || f.Stale == 0 {
			t.Fatalf("seed %d: the cop's word %+v %v", seed, f, ok)
		}
		if f.Value == w.Law.Chief.Personality {
			right++
			continue
		}
		wrong++
		known := false
		for _, p := range content.ChiefPersonalities {
			known = known || p == f.Value
		}
		if !known {
			t.Fatalf("seed %d: %q is no temper", seed, f.Value)
		}
	}
	if got := float64(right) / float64(right+wrong); math.Abs(got-tun.CopAccuracy) > 0.05 {
		t.Fatalf("the cop was right about the chief %.3f of the time, want %.2f ± 0.05", got, tun.CopAccuracy)
	}
	// A chief replaced takes the fact with them.
	w = sim.NewWorld(cfg, 9)
	w.Law.Chief.Since = 0
	tk = tick(w, day)
	s.Step(w, tk)
	if _, ok := game.Known(w).Fact(game.SubjectChief, game.FactPersonality); !ok {
		t.Fatal("not observed")
	}
	tk = tick(w, day+1, events.Incident{Day: day + 1, ID: "chief_resigns", City: w.Home().ID, Chief: w.Law.Chief.Name, NewChief: true})
	s.Step(w, tk)
	if kinds(tk)["ChiefReplaced"] != 1 {
		t.Fatalf("no replacement: %v", kinds(tk))
	}
	if _, ok := game.Known(w).Fact(game.SubjectChief, game.FactPersonality); ok || w.Law.Chief.Observed {
		t.Fatalf("the new chief is known: %+v observed %v", w.Intel, w.Law.Chief.Observed)
	}
	if game.Known(w).Chief() != game.Unknown {
		t.Fatal("Chief() does not read Unknown")
	}
}
