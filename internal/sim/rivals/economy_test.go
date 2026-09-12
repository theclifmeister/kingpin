package rivals_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The rival's money is priced in corner-days (#139): its wage, its fee,
// its claim and the chest it arrives with are so many days of what a
// standard corner at home earns it, so doubling the street price
// doubles every one of them, and the port's product (no_supply at
// home) is in none of them, nor in its take.
func TestCostsAreCornerDays(t *testing.T) {
	cfg := content.MustLoad()
	tun := cfg.Rivals.Rivals
	w, s := world(t, cfg, 3)
	w.Rival.Personality = "defensive"
	m := w.Product(w.Home().ID, "weed")
	std := m.Demand * m.Price
	day := std * tun.Margin
	if got := s.Standard(w); math.Abs(got-std) > 1e-9 {
		t.Fatalf("standard corner %.2f, want %.2f", got, std)
	}
	if got := s.CornerDay(w); math.Abs(got-day) > 1e-9 {
		t.Fatalf("corner-day %.2f, want %.2f", got, day)
	}
	heads := cfg.Rivals.Personality["defensive"].MusclePerCorner
	if s.Wage(w) != int(math.Round(tun.MuscleWage/heads*day)) || s.Fee(w) != int(math.Round(tun.MuscleFee*day)) || s.ClaimCost(w) != int(math.Round(tun.ClaimCost*day)) {
		t.Fatalf("wage %d fee %d claim %d on a corner-day of %.0f", s.Wage(w), s.Fee(w), s.ClaimCost(w), day)
	}
	if w.Rival.Cash != int(math.Round(tun.StartCash*day)) {
		t.Fatalf("arrived with %d, want %.0f corner-days of %.0f", w.Rival.Cash, tun.StartCash, day)
	}
	wage, fee, claim := s.Wage(w), s.Fee(w), s.ClaimCost(w)
	m.Price *= 2
	if s.Wage(w) != 2*wage || s.Fee(w) != 2*fee || s.ClaimCost(w) != 2*claim {
		t.Fatalf("the price doubled and the costs went %d/%d/%d -> %d/%d/%d", wage, fee, claim, s.Wage(w), s.Fee(w), s.ClaimCost(w))
	}
	// The port's product is not the rival's trade.
	w.Corner("docks").Owner = game.OwnerRival
	income := s.Income(w)
	w.Home().Market["designer"] = &game.ProductMarket{Price: 2500, Demand: 40, NoSupply: true}
	w.Products = append(w.Products, "designer")
	if s.Income(w) != income || s.Standard(w) != 2*std || s.CornerIncome(w, *w.Corner("docks")) != income {
		t.Fatalf("designer at home moved the rival's take %d -> %d, the standard corner %.0f -> %.0f, the corner's %d", income, s.Income(w), 2*std, s.Standard(w), s.CornerIncome(w, *w.Corner("docks")))
	}
	w.Home().Market["designer"].NoSupply = false
	if s.Income(w) <= income {
		t.Fatalf("designer the street sells is not the rival's take: %d", s.Income(w))
	}
}

// Muscle is what the take pays for (#139), with no dice in it: a
// corner squeezed by a price war cuts the take, the wages the take no
// longer covers run up arrears, and at a full wage owed a head walks,
// one a day, never one of the two it came with; with the squeeze off
// the surplus pays the arrears down and it recruits the head back out
// of the chest. A rival that could not pay its muscle from the chest
// either is down to what the chest holds.
func TestArrearsWalkAHead(t *testing.T) {
	cfg := content.MustLoad()
	w, s := warWorld(t, cfg, 4, "defensive")
	// Two more corners, and the muscle its take pays for exactly.
	for _, id := range []string{"railyard", "oldmill"} {
		w.Corner(id).Owner, w.Corner(id).Since = game.OwnerRival, 1
	}
	afford := s.Afford(w)
	if afford < 3 {
		t.Fatalf("three corners pay for %d heads", afford)
	}
	w.Rival.Muscle = afford
	step(w, s)
	if w.Rival.Muscle != afford || w.Rival.Arrears != 0 {
		t.Fatalf("in peace: muscle %d (afford %d), arrears %.0f", w.Rival.Muscle, afford, w.Rival.Arrears)
	}
	// The war: a third off the biggest corner, every morning, as the
	// market would leave it. The take no longer covers the payroll.
	days := 0
	for w.Rival.Muscle == afford && days < 30 {
		squeeze(w, "docks", 0.34)
		short := float64(s.Wages(w) - s.Income(w)) // at today's price: its own undercutting drags it a little each night
		if short <= 0 {
			t.Fatalf("a third off the docks and the take still covers %d heads", w.Rival.Muscle)
		}
		before := w.Rival.Arrears
		step(w, s)
		days++
		if w.Rival.Muscle == afford && math.Abs(w.Rival.Arrears-(before+short)) > 1 {
			t.Fatalf("day %d: arrears %.0f, want %.0f + %.0f", days, w.Rival.Arrears, before, short)
		}
	}
	if w.Rival.Muscle != afford-1 || days == 0 || days > 30 {
		t.Fatalf("under the war: muscle %d after %d days (was %d)", w.Rival.Muscle, days, afford)
	}
	if wage := s.Wage(w); w.Rival.Arrears >= float64(wage) {
		t.Fatalf("a head walked and %.0f is still owed against a wage of %d", w.Rival.Arrears, wage)
	}
	// Peace (the market leaves no squeeze): the surplus pays the arrears
	// down and the head is hired back.
	squeeze(w, "docks", 0)
	for i := 0; i < 30 && (w.Rival.Arrears > 0 || w.Rival.Muscle < afford); i++ {
		step(w, s)
	}
	if w.Rival.Arrears != 0 || w.Rival.Muscle != afford {
		t.Fatalf("after the war: muscle %d (afford %d), arrears %.0f", w.Rival.Muscle, afford, w.Rival.Arrears)
	}
	// Never under the two it came with, whatever it owes.
	w.Rival.Muscle = cfg.Rivals.Rivals.StartMuscle
	w.Rival.Arrears = float64(10 * s.Wage(w))
	step(w, s)
	if w.Rival.Muscle < cfg.Rivals.Rivals.StartMuscle {
		t.Fatalf("owing ten wages it let one of the first %d go: %d", cfg.Rivals.Rivals.StartMuscle, w.Rival.Muscle)
	}
	// An empty chest is the last resort: down to what it holds.
	w.Rival.Muscle, w.Rival.Cash, w.Rival.Arrears = 20, 0, 0
	step(w, s)
	if w.Rival.Muscle >= 20 || w.Rival.Cash < 0 {
		t.Fatalf("with no chest: muscle %d, cash %d", w.Rival.Muscle, w.Rival.Cash)
	}
}
