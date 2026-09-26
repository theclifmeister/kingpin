package laundering_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
)

// TestTillHoldsTheWash (#496): a playtest with two big fronts sat at
// exactly $50,000 dirty every morning, the wash taking the rest, and
// could not save for a contract, a chemist's lot or the next front. The
// till is the player's line (World.SetTill): the wash leaves it in hand;
// zero, or anything under the float, is the float, the run before.
func TestTillHoldsTheWash(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	fc := cfg.Laundering.Fronts[0]
	float := s.Float(world(0))
	for _, c := range []struct {
		name       string
		till, line int
	}{
		{"never set", 0, float},
		{"under the float", float / 2, float},
		{"raised", float + 30_000, float + 30_000},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := world(0)
			w.Fronts = []game.Front{{ID: fc.ID, Name: fc.Name}}
			w.Player.CleanCash = 1_000_000 // the upkeep is never the story
			if err := w.SetTill(c.till); err != nil {
				t.Fatal(err)
			}
			if got := s.Till(w); got != c.line {
				t.Fatalf("Till %d, want %d", got, c.line)
			}
			if got := s.Line(w); got != c.line {
				t.Fatalf("Line %d with no contract, want the till %d", got, c.line)
			}
			dirty := float + 30_000 + 700
			w.Player.DirtyCash = dirty
			through := s.Throughput(w, w.Fronts[0])
			step(w, s)
			if want := max(c.line, dirty-through); w.Player.DirtyCash != want {
				t.Fatalf("dirty %d after the wash, want %d (throughput %d)", w.Player.DirtyCash, want, through)
			}
		})
	}
	if err := world(0).SetTill(-1); err != game.ErrBadAmount {
		t.Fatalf("a negative till: %v", err)
	}
}

// TestWashKeepsTheContractsMorning (#496): the supply contracts buy at
// the top of the next night out of the dirty cash, and a big front
// washed it all down to the till first, so contracts over the till went
// short every morning. The wash's line is the till or the contracts'
// morning (World.SupplyOutlay) where that is more; a run with no
// contract, or one that costs less than the till, washes to the till.
func TestWashKeepsTheContractsMorning(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	fc := cfg.Laundering.Fronts[0]
	w := world(0)
	home := w.Home().ID
	id := w.Products[0]
	w.Houses = append(w.Houses, game.House{ID: "warehouse", City: home, Capacity: 1_000_000})
	w.Fronts = []game.Front{{ID: fc.ID, Name: fc.Name}}
	w.Player.CleanCash = 1_000_000
	w.Markup = 1.05
	price := w.SupplierPrice(home, id)
	units := int(math.Ceil(3 * float64(s.Float(w)) / price))
	if err := w.SetSupply(home, id, units); err != nil {
		t.Fatal(err)
	}
	outlay := w.SupplyOutlay()
	if want := int(math.Ceil(price * 1.05 * float64(units-w.Stock(home, id)))); outlay != want {
		t.Fatalf("SupplyOutlay %d, want %d", outlay, want)
	}
	if outlay <= s.Till(w) || s.Line(w) != outlay {
		t.Fatalf("Line %d, want the contracts' %d over the till %d", s.Line(w), outlay, s.Till(w))
	}
	w.Player.DirtyCash = outlay + 300
	step(w, s)
	if w.Player.DirtyCash != outlay {
		t.Fatalf("dirty %d after the wash, want the contracts' morning %d kept", w.Player.DirtyCash, outlay)
	}
	// Cleared, the till is the line again.
	w.ClearSupply(home, id)
	if w.SupplyOutlay() != 0 || s.Line(w) != s.Till(w) {
		t.Fatalf("no contract: outlay %d, line %d, till %d", w.SupplyOutlay(), s.Line(w), s.Till(w))
	}
}
