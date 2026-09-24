// Package engine is the game as one object a front end drives (#293,
// docs/engine.md): a Session assembles a run the one way (the sims in
// sim.Default's step order, a clock, a bus), owns its world and its
// lifecycle (a new run, a load, a save, the day) and publishes the
// day's events. The TUI and the harness play through it, and so will
// any other front end, so there is one assembly path to keep right.
//
// The session sits above the clock: the sims still never call each
// other, and events stay their only channel. It never reads the wall
// clock and never draws a seed (the caller passes it, #50), and it
// starts no goroutine.
package engine

import (
	"errors"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// ErrNoRun is what a call that needs a run returns before NewRun, Load
// or Attach has given the session one.
var ErrNoRun = errors.New("engine: no run")

// Session is one run's engine: the config, the sims, the clock, the
// bus and the world they step.
type Session struct {
	cfg   *content.Config
	set   *sim.Set
	clock *game.Clock
	bus   *events.Bus
	w     *game.World
	saved []game.Preset // the front end's saved presets (#357, UsePresets)

	// street is what the street sold for on the night that ended on
	// streetDay (the tick's PlayerSold revenue, the number the
	// businessman's count reads, #347): kept from the last EndDay so the
	// legit plan can say it; a run loaded since reads the report's flow.
	street, streetDay int
}

// New builds the sims from cfg in their step order, a clock over them
// and a bus the clock publishes each day's events on. The session has
// no run until NewRun, Load or Attach.
func New(cfg *content.Config) (*Session, error) {
	set, sims, err := sim.Default(cfg)
	if err != nil {
		return nil, err
	}
	bus := events.NewBus()
	return &Session{cfg: cfg, set: set, clock: game.NewClock(bus, sims...), bus: bus}, nil
}

// Config is the tuning the session was built from.
func (s *Session) Config() *content.Config { return s.cfg }

// Rules is what a front end reads a cost, a price or a preview through
// (#297): one interface a sim, each listing the sim's read methods a
// front end may call and nothing that steps or seeds it.
func (s *Session) Rules() Rules {
	return Rules{
		Market:     s.set.Market,
		Logistics:  s.set.Logistics,
		Territory:  s.set.Territory,
		Rivals:     s.set.Rivals,
		Crew:       s.set.Crew,
		Heat:       s.set.Heat,
		Law:        s.set.Law,
		Laundering: s.set.Laundering,
	}
}

// Sims is the constructed sims themselves, for the harness and the
// tests that pin them. A front end reads them through Rules and acts
// through the session's commands (TestFrontEndsUseTheSession).
func (s *Session) Sims() *sim.Set { return s.set }

// World is the run the session drives, nil before it has one.
func (s *Session) World() *game.World { return s.w }

// NewRun starts a fresh run from the seed as the start (#50): the
// character, the hard DA, the daily, sim.NewWorldWith's.
func (s *Session) NewRun(seed uint64, start game.Start) *game.World {
	s.w = sim.NewWorldWith(s.cfg, seed, start)
	return s.w
}

// Attach drives a world the caller built: the harness's and the
// tests', which start theirs from sim.NewWorld and a config of their
// own.
func (s *Session) Attach(w *game.World) { s.w = w }

// Load continues the run saved in the slot, migrated to the current
// schema through the sims' chain (docs/saves.md). A slot that does not
// load leaves the session's run as it was.
func (s *Session) Load(slot int) (*game.World, error) {
	w, err := game.Load(slot, s.set.Migrations()...)
	if err != nil {
		return nil, err
	}
	s.w = w
	return w, nil
}

// Save writes the run to the slot.
func (s *Session) Save(slot int) error {
	if s.w == nil {
		return ErrNoRun
	}
	return game.Save(slot, s.w)
}

// ExportSave is the run as a save's bytes (game.Encode), for a front end
// that keeps its saves itself (#327): a browser has no slots.
func (s *Session) ExportSave() ([]byte, error) {
	if s.w == nil {
		return nil, ErrNoRun
	}
	return game.Encode(s.w)
}

// ImportSave makes a save's bytes the run, as Load makes a slot's:
// upgraded with the sims' migrations, and refused when it cannot be
// read.
func (s *Session) ImportSave(b []byte) (*game.World, error) {
	w, err := game.Decode(b, s.set.Migrations()...)
	if err != nil {
		return nil, err
	}
	s.w = w
	return w, nil
}

// EndDay steps every sim once over the run (game.Clock.EndDay),
// publishes the tick's events to the subscribers and returns them. It
// is a no-op once the run is over, or before there is one.
func (s *Session) EndDay() []events.Event {
	if s.w == nil {
		return nil
	}
	evs := s.clock.EndDay(s.w)
	if len(evs) == 0 {
		return evs
	}
	s.street, s.streetDay = 0, s.w.Day
	for _, e := range evs {
		if ev, ok := e.(events.PlayerSold); ok {
			s.street += ev.Revenue
		}
	}
	return evs
}

// Subscribe registers h for every event the session publishes, in the
// order the day emitted them, on the caller's goroutine.
func (s *Session) Subscribe(h events.Handler) { s.bus.Subscribe(h) }
