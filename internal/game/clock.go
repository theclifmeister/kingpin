package game

import (
	"hash/fnv"
	"math/rand/v2"

	"github.com/theclifmeister/kingpin/internal/events"
)

// Tick is the context handed to every simulation when a day ends. Events
// emitted by earlier simulations in the same tick are visible to later ones,
// which is where cross-simulation reactions live.
type Tick struct {
	Day    int
	RNG    *rand.Rand // the day's stream, shared by every sim in step order
	Seed   uint64     // the run's seed, for Sub
	events []events.Event
	subs   map[string]*rand.Rand
}

// Sub is a side stream of the day's randomness named for what it rolls
// (a city's market, the road), derived from the run's seed, the day and
// the name, so that what happens away from home never shifts the home
// stream: a run that never leaves the first city replays the same
// whether or not the second exists. Sims draw the home city's dice from
// RNG and everything else from a Sub.
func (t *Tick) Sub(name string) *rand.Rand {
	if r := t.subs[name]; r != nil {
		return r
	}
	if t.subs == nil {
		t.subs = map[string]*rand.Rand{}
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	r := rand.New(rand.NewPCG(t.Seed^h.Sum64(), uint64(t.Day)*0x9E3779B97F4A7C15+1))
	t.subs[name] = r
	return r
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
//	market -> logistics -> territory -> rivals -> crew -> heat -> law -> laundering -> reputation -> news
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
// Of the day's receipts it keeps what the supply contracts bought this
// morning (#113), so the cart can show and return them through the day;
// the buys made by hand and yesterday's contract receipts go.
func (c *Clock) EndDay(w *World) []events.Event {
	if w.Over != nil {
		return nil
	}
	day := w.Day + 1
	t := &Tick{Day: day, RNG: RNGFor(w.Seed, day), Seed: w.Seed}
	for _, s := range c.sims {
		s.Step(w, t)
		if w.Over != nil {
			break
		}
	}
	w.Day = day
	w.ClearToday(day)
	w.Dilemmas.Answered = nil
	w.Crew.HiredToday = nil
	w.Crew.FiredToday = nil
	w.Crew.PaidOffToday = nil
	for _, c := range w.Cities {
		for _, m := range c.Market {
			m.BoughtToday = 0
		}
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

// ClearToday zeroes the player's per-day scratch (World.Today) as one
// unit, the way the clock does after every EndDay: every order, choice
// and errand queued since the morning is gone, and the sell orders' map
// is fresh for the next day's. The one exception is #113's kept
// contract receipts: of Buys it keeps what the supply contracts bought
// in the tick that brings day, so the cart can show and return them
// through the day (supplied). Nothing else on World is per-day: the
// sims' own tallies (Dilemmas.Answered, Crew.HiredToday, FiredToday and
// PaidOffToday, a market's BoughtToday) stay on their sims' state and
// the clock clears them beside it.
func (w *World) ClearToday(day int) {
	w.Today = Today{Orders: map[string]SellOrder{}, Buys: supplied(w.Today.Buys, day)}
}

// supplied is the receipts the clock keeps into the morning of day: the
// supply contracts' buys made in the tick that brings it, with Prior
// cleared, since the market has reset the supplier price since and a
// return must not walk it back to yesterday's. Nil when there are none,
// as the scratch was before the contracts.
func supplied(buys []Purchase, day int) []Purchase {
	var kept []Purchase
	for _, b := range buys {
		if b.Contract && b.Day == day {
			b.Prior = 0
			kept = append(kept, b)
		}
	}
	return kept
}
