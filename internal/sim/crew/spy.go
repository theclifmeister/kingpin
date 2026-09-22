package crew

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The spies (#45, intel.toml). PlantSpy sends a member under with a
// faction (CrewMember.Undercover): from that night they stand on no
// corner, sell nothing, guard nothing and drive nothing (Working and
// Fit are false), while the wage runs on. Every spy_days under they
// file a report, three facts at the spy's accuracy (spy_accuracy at
// spy_skill, scaled by skill): the faction's muscle (a wrong count is
// off by up to spy_slip heads), the corner it moves on next (the tell,
// or the corner of yours it borders with nobody guarding it) and the
// corner its till is fattest on (World.Fattest, the truth a boost
// takes from). Each report rolls the faction's spy_found by its temper:
// found, turn_share of them come home turned (an informant, #13, the
// heat sim's clock started by the silent event; the paper says only
// that they were made) and the rest are shot (the crew sim's own death,
// fall, off the roster and onto the count; no headline names your
// corner, the body is theirs to lose). A faction gone (#43) sends its
// spy home with nothing. Everything rolls on Tick.Sub("intel"), after
// the rivals sim's feed on the same stream, so a run with no spy under
// draws nothing.

// Intel exposes the intel tuning for the UI and the harness.
func (s *Sim) Intel() content.IntelTuning { return s.intel }

// spies runs tonight's plant and every spy's night under.
func (s *Sim) spies(w *game.World, t *game.Tick) {
	tun := s.intel
	if o := w.Today.Spy; o != nil {
		if m, r := w.Crew.Member(o.Member), w.Faction(o.Faction); m != nil && r != nil && r.Alive() && m.Undercover == "" && m.Fit(t.Day) {
			w.Recall(m.ID)
			m.Undercover, m.UndercoverDay = r.Faction(), t.Day
			w.Stats.Spies++
			t.Emit(events.SpyPlanted{Day: t.Day, ID: m.ID, Name: m.Name, Role: m.Role, Rival: r.Leader, Faction: r.Faction()})
		}
	}
	var under, home []int
	for _, m := range w.Crew.Members {
		if m.Undercover != "" && m.UndercoverDay != t.Day {
			under = append(under, m.ID)
		}
	}
	for _, id := range under {
		m := w.Crew.Member(id) // a spy shot tonight leaves the roster (fall), so look each up
		if m == nil {
			continue
		}
		r := w.Faction(m.Undercover)
		if r == nil || !r.Alive() {
			home = append(home, m.ID)
			continue
		}
		if tun.SpyDays <= 0 || (t.Day-m.UndercoverDay)%tun.SpyDays != 0 {
			continue
		}
		rng := t.Sub(game.StreamIntel)
		s.report(w, t, rng, m, r)
		if rng.Float64() >= tun.Found(r.Personality) {
			continue
		}
		w.Stats.SpiesFound++
		ev := events.SpyFound{Day: t.Day, ID: m.ID, Name: m.Name, Role: m.Role, Rival: r.Leader, Faction: r.Faction(), Why: "made", Reports: (t.Day - m.UndercoverDay) / tun.SpyDays}
		if rng.Float64() < tun.TurnShare {
			m.Undercover, m.UndercoverDay = "", 0
			if !m.Informant {
				m.Informant = true
				w.Stats.Informants++
				t.Emit(events.CrewTurnedInformant{Day: t.Day, ID: m.ID, Name: m.Name})
			}
			t.Emit(ev)
			continue
		}
		ev.Dead = true
		w.Stats.SpiesShot++
		s.fall(w, t, *m, nil)
		t.Emit(ev)
	}
	for _, id := range home {
		if m := w.Crew.Member(id); m != nil {
			r := factionOf(w, m.Undercover)
			ev := events.SpyFound{Day: t.Day, ID: m.ID, Name: m.Name, Role: m.Role, Rival: r.Leader, Faction: r.Faction(), Why: "gone"}
			if tun.SpyDays > 0 {
				ev.Reports = (t.Day - m.UndercoverDay) / tun.SpyDays
			}
			m.Undercover, m.UndercoverDay = "", 0
			t.Emit(ev)
		}
	}
}

// report files what a spy saw tonight: the muscle, the next move and
// the till, each true at the spy's odds and filed at them.
func (s *Sim) report(w *game.World, t *game.Tick, rng game.Rand, m *game.CrewMember, r *game.RivalState) {
	tun := s.intel
	p := tun.SpyOdds(m.Skill)
	w.Stats.Reports++
	learn := func(kind, value string, n float64, name string) {
		f := game.Fact{Subject: r.Faction(), Kind: kind, Value: value, Number: n, Confidence: p, Day: t.Day, Source: game.SourceSpy, Stale: tun.StaleRate, Forget: tun.Forget}
		w.Learn(f)
		t.Emit(events.IntelGained{Day: t.Day, Subject: f.Subject, FactKind: kind, Value: value, Confidence: p, Source: f.Source, Name: name})
	}
	// The muscle: the count, or one off by up to spy_slip heads.
	muscle := r.Muscle
	if rng.Float64() >= p {
		slip := 1 + rng.IntN(max(1, tun.SpySlip))
		if rng.IntN(2) == 0 && muscle-slip >= 0 {
			muscle -= slip
		} else {
			muscle += slip
		}
	}
	learn(game.FactMuscle, fmt.Sprint(muscle), float64(muscle), r.Leader)
	// The next move: the corner it has spoken for, else the corner of
	// yours it borders with nobody guarding it, else any it borders.
	if c := s.nextMove(w, r); c != nil {
		move := c
		if rng.Float64() >= p {
			if yours := s.yours(w, r); len(yours) > 0 {
				move = yours[rng.IntN(len(yours))]
			}
		}
		learn(game.FactMove, move.ID, 0, r.Leader)
	}
	// The till: the corner moving the most trade, or another of its.
	if c := w.Fattest(r.Faction()); c != nil {
		till := c
		if rng.Float64() >= p {
			if theirs := s.theirCorners(w, r); len(theirs) > 1 {
				till = theirs[rng.IntN(len(theirs))]
			}
		}
		learn(game.FactStash, till.ID, 0, r.Leader)
	}
}

// nextMove is where a faction moves next as a spy reads it: the corner
// it is eyeing (#69) while it is still free, else the first of your
// corners it borders with no enforcer on it, else the first it borders.
func (s *Sim) nextMove(w *game.World, r *game.RivalState) *game.Corner {
	if r.Eyeing != "" {
		if c := w.Corner(r.Eyeing); c != nil && c.Owner == game.OwnerNone {
			return c
		}
	}
	yours := s.yours(w, r)
	for _, c := range yours {
		if c.Enforcer == 0 {
			return c
		}
	}
	if len(yours) > 0 {
		return yours[0]
	}
	return nil
}

// yours is the player's corners the faction borders, in map order.
func (s *Sim) yours(w *game.World, r *game.RivalState) []*game.Corner {
	var out []*game.Corner
	for _, cid := range w.CityOrder {
		cs := w.Cities[cid].Corners
		for i := range cs {
			if cs[i].Owner == game.OwnerPlayer && w.ContestedBy(cs[i], r.Faction()) {
				out = append(out, &cs[i])
			}
		}
	}
	return out
}

// theirCorners is the faction's corners, in map order.
func (s *Sim) theirCorners(w *game.World, r *game.RivalState) []*game.Corner {
	var out []*game.Corner
	for _, cid := range w.CityOrder {
		cs := w.Cities[cid].Corners
		for i := range cs {
			if cs[i].Owner == game.OwnerRival && cs[i].FactionID() == r.Faction() {
				out = append(out, &cs[i])
			}
		}
	}
	return out
}
