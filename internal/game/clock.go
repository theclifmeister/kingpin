package game

import (
	"math/rand/v2"

	"github.com/theclifmeister/kingpin/internal/events"
)

// Tick is the context handed to every simulation when a day ends. Events
// emitted by earlier simulations in the same tick are visible to later ones,
// which is where cross-simulation reactions live.
type Tick struct {
	Day    int
	RNG    *rand.Rand
	events []events.Event
}

// Emit records an event for this tick.
func (t *Tick) Emit(e events.Event) { t.events = append(t.events, e) }

// Events returns everything emitted so far this tick, in order.
func (t *Tick) Events() []events.Event { return t.events }

// Simulation is one independent system stepping the world forward a day.
// Implementations must be deterministic given the world and t.RNG.
type Simulation interface {
	Name() string
	Step(w *World, t *Tick)
}

// Clock advances the world one day at a time. Simulations run in the fixed
// order they were registered in:
//
//	market -> logistics -> territory -> rivals -> crew -> heat -> laundering -> news
//
// so that results are reproducible for a given seed.
type Clock struct {
	sims []Simulation
	bus  *events.Bus
}

// NewClock builds a clock that publishes each tick's events on bus.
func NewClock(bus *events.Bus, sims ...Simulation) *Clock {
	return &Clock{sims: sims, bus: bus}
}

// Sims returns the registered simulations in step order.
func (c *Clock) Sims() []Simulation { return c.sims }

// EndDay steps every simulation, clears per-day scratch, publishes the
// tick's events and returns them. It is a no-op once the run is over.
func (c *Clock) EndDay(w *World) []events.Event {
	if w.Over != nil {
		return nil
	}
	day := w.Day + 1
	t := &Tick{Day: day, RNG: RNGFor(w.Seed, day)}
	for _, s := range c.sims {
		s.Step(w, t)
		if w.Over != nil {
			break
		}
	}
	w.Day = day
	w.Orders = map[string]SellOrder{}
	w.Buys = nil
	w.LieLow = false
	w.Strike = nil
	w.Investigation = nil
	w.UpgradesToday = nil
	w.Crew.HiredToday = nil
	w.Crew.FiredToday = nil
	w.Crew.PaidOffToday = nil
	for _, m := range w.Market {
		m.BoughtToday = 0
	}
	if w.Cash() > w.Stats.PeakCash {
		w.Stats.PeakCash = w.Cash()
	}
	t.Emit(events.DayEnded{Day: day})
	if c.bus != nil {
		for _, e := range t.events {
			c.bus.Publish(e)
		}
	}
	return t.events
}
