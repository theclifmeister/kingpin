// Package reputation drifts the player's public face from what already
// happened today: violence and held ground for fear, what the crew were
// paid, the peace kept with the rival and the buyers' contracts delivered
// for respect, volume and headlines for notoriety. It owns
// Player.Reputation; the sims that feel it (rivals, crew, market, heat)
// each read their own knob off reputation.toml and the world. It steps
// after laundering and before news, so the headlines it counts are
// yesterday's and the band it crosses today makes today's news.
package reputation

import (
	"math"
	"slices"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the reputation simulation.
type Sim struct {
	cfg content.ReputationConfig
}

// New builds a reputation sim from the config, copying what it reads
// (#144): its own reputation.toml.
func New(cfg *content.Config) *Sim { return &Sim{cfg: cfg.Reputation} }

func (s *Sim) Name() string { return "reputation" }

// Tuning exposes the constants the UI needs to explain itself.
func (s *Sim) Tuning() content.ReputationTuning { return s.cfg.Reputation }

// Step adds today's sources, fades every axis toward the baseline, keeps
// the three inside the street's attention, then reports any band crossed.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Reputation
	r := &w.Player.Reputation
	before := *r

	var fear, respect, notoriety float64
	units := 0
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CornerStruck:
			if ev.Taken {
				fear += s.cfg.Fear.StrikeTaken
				notoriety += s.cfg.Notoriety.StrikeTaken
			} else {
				fear += s.cfg.Fear.StrikeHeld
				notoriety += s.cfg.Notoriety.StrikeHeld
			}
		case events.RivalPushed:
			fear += s.cfg.Fear.PushHeld
		case events.RivalBoosted:
			// Robbing a rival corner's takings (#70) is fear, not respect.
			if ev.Taken {
				fear += s.cfg.Fear.Boost
			}
		case events.CrewPaid:
			switch {
			case ev.Short > 0:
				respect += s.cfg.Respect.ShortPay
			case ev.Pay == events.PayGenerous:
				respect += s.cfg.Respect.GenerousPay
			}
		case events.CrewPaidOff:
			respect += s.cfg.Respect.Payoff
		case events.PlayerSold:
			units += ev.Sold
		case events.ContractDelivered:
			// A buyer's contract delivered in full is respect (#71), the
			// third source beside the payroll and a kept deal; the event
			// carries what the deck says it earns. The units are volume
			// like any other.
			respect += ev.Respect
			units += ev.Units
		case events.ContractFailed:
			// Word gets round that you did not deliver.
			respect -= ev.Respect
			notoriety += ev.Notoriety
		case events.Overdose:
			// The paper names the block, the block names you (#47).
			notoriety += s.cfg.Notoriety.Overdose
		}
	}
	if s.cfg.Notoriety.Units > 0 {
		notoriety += float64(units) / s.cfg.Notoriety.Units
	}
	// A peace kept is respect: every truce or split that held tonight,
	// counted from the night after it was struck. Tribute is not respect.
	// The rival sim has already ended the ones that ran out, so what is
	// left held.
	for _, d := range w.Rival.Deals {
		if (d.Kind == game.DealTruce || d.Kind == game.DealSplit) && d.Since < t.Day {
			respect += s.cfg.Respect.DealKept
		}
	}
	// Yesterday's headlines about you: the news sim steps after this one,
	// so today's are not written yet, and the source says whose story a
	// headline is.
	for i := len(w.Journal) - 1; i >= 0 && w.Journal[i].Day == t.Day-1; i-- {
		if slices.Contains(s.cfg.Notoriety.Sources, w.Journal[i].Source) {
			notoriety += s.cfg.Notoriety.Headline
		}
	}

	r.Fear = fade(r.Fear+fear, tun)
	r.Respect = fade(r.Respect+respect, tun)
	r.Notoriety = fade(r.Notoriety+notoriety, tun)

	// The street has only so much attention: past the total, every axis
	// gives a little to make room.
	if sum := r.Fear + r.Respect + r.Notoriety; tun.Total > 0 && sum > tun.Total {
		k := tun.Total / sum
		r.Fear *= k
		r.Respect *= k
		r.Notoriety *= k
	}

	for _, axis := range game.Axes {
		from, to := *before.Axis(axis), *r.Axis(axis)
		if band(from, tun.Band) != band(to, tun.Band) {
			t.Emit(events.ReputationShifted{Day: t.Day, Axis: axis, From: from, To: to})
		}
	}
}

// fade closes part of the gap to the baseline and clamps to 0..100.
func fade(v float64, tun content.ReputationTuning) float64 {
	v -= (v - tun.Baseline) * tun.Decay
	return math.Max(0, math.Min(100, v))
}

// band is which band of width w a value sits in; 100 sits in the top one.
func band(v, w float64) int {
	if w <= 0 {
		return 0
	}
	return int(math.Min(v, 99.999) / w)
}
