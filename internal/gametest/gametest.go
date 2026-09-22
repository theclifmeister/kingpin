// Package gametest is the sims' shared test fixture (#276): the one-city
// world their tests stand on and the tick that steps a sim a day over it.
// It is imported by tests only (TestOnlyTestsImportGametest): nothing
// the game or the tooling builds may read it. It imports game and events
// and nothing under internal/sim, so a sim's test may import it without
// one sim reaching another (TestSimsNeverImportEachOther).
package gametest

import (
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// A is the plain product the crew and laundering fixtures deal: $10 at
// demand 5, bought at the supplier's 0.55 of the street (#273: the
// market refuses a product with no ratio).
var A = game.StartingProduct{ID: "a", Name: "A", Price: 10, Demand: 5, SupplierRatio: 0.55}

// Weed is the product the territory, rivals and reputation fixtures
// deal: $20 at demand 60, the supplier's 0.55.
var Weed = game.StartingProduct{ID: "weed", Name: "Weed", Price: 20, Demand: 60, SupplierRatio: 0.55}

// OneCity is a world of one city, "test" named Testville, on seed with
// cash in hand and a capacity of 100, dealing products (A when none is
// given). A fixture that needs another id or name uses City.
func OneCity(seed uint64, cash int, products ...game.StartingProduct) *game.World {
	return City(seed, "test", "Testville", cash, products...)
}

// City is OneCity with the city's id and name given: the fixtures that
// stand on the file's home city (territory, rivals) name its id, the
// reputation's calls it Test.
func City(seed uint64, id, name string, cash int, products ...game.StartingProduct) *game.World {
	if len(products) == 0 {
		products = []game.StartingProduct{A}
	}
	return game.NewWorld(seed, []game.StartingCity{{ID: id, Name: name, Products: products}}, cash, 100)
}

// TickOn is day's tick over w as the clock builds it (the day's stream
// from the run's seed, and Seed set for Tick.Sub), with evs already
// emitted as if by the sims that step before the one under test.
func TickOn(w *game.World, day int, evs ...events.Event) *game.Tick {
	t := &game.Tick{Day: day, RNG: game.RNGFor(w.Seed, day), Seed: w.Seed}
	for _, e := range evs {
		t.Emit(e)
	}
	return t
}

// Tick is TickOn the day after w's.
func Tick(w *game.World, evs ...events.Event) *game.Tick { return TickOn(w, w.Day+1, evs...) }

// Step runs s over w for one day on Tick(w, evs...) and moves w's day
// on; it returns the tick, whose Events are evs and then what s emitted.
// It clears nothing: a test that wants the morning's scratch zeroed, as
// the clock does, calls w.ClearToday itself.
func Step(w *game.World, s game.Simulation, evs ...events.Event) *game.Tick {
	return run(w, s, Tick(w, evs...))
}

// StepUnseeded is Step on a tick whose Seed is zero. The crew,
// laundering, rivals and territory tests were written on such a tick,
// so every side stream they roll (Tick.Sub) is drawn from seed 0, not
// the world's; seeding it would move those tests' dice, so the zero is
// kept, and named, and no test's day changes (#276).
func StepUnseeded(w *game.World, s game.Simulation, evs ...events.Event) *game.Tick {
	t := Tick(w, evs...)
	t.Seed = 0
	return run(w, s, t)
}

func run(w *game.World, s game.Simulation, t *game.Tick) *game.Tick {
	s.Step(w, t)
	w.Day++
	return t
}
