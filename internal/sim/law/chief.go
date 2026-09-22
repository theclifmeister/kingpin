package law

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// chief is the chief's day: observed, filed, and replaced at the end of
// a term or on an incident; it reports whether they were replaced.
func (s *Sim) chief(w *game.World, t *game.Tick) bool {
	tun := s.cfg.Law
	// The chief: you learn what they are like after a while in office, or
	// the first time their people come through the door.
	chief := &w.Law.Chief
	if !chief.Observed {
		if t.Day-chief.Since >= tun.ObserveDays {
			chief.Observed = true
		}
		for _, e := range t.Events() {
			if ev, ok := e.(events.Enforcement); ok && ev.Level != content.Arrest {
				chief.Observed = true
			}
		}
	}
	s.intelChief(w, t)
	replaced := false
	if end := s.ChiefTermEnds(w); end > 0 && t.Day >= end {
		s.replaceChief(w, t, "term", "")
		replaced = true
	}
	// The world's incidents (#44), dealt first thing this tick: a chief
	// who resigned is replaced this morning, on the law's own dice, and
	// a snap election is held that many days out (the term resets when
	// it is), on the mood the city is in then. An actor whose clock is
	// stopped (a term of 0, harness.Appoint) is held through both.
	for _, e := range t.Events() {
		if ev, ok := e.(events.Incident); ok {
			if ev.NewChief && !replaced && tun.ChiefTerm > 0 {
				s.replaceChief(w, t, "resigned", "")
				replaced = true
			}
			if ev.Election > 0 && tun.TermDays > 0 {
				w.Law.SnapElection = t.Day + ev.Election
			}
		}
	}
	return replaced
}

// replaceChief puts a new chief in office: a new name and a personality
// drawn from the law's side stream, hidden until observed; why is term,
// da, resigned (#44) or campaign, and personality, if given, is who the
// mayor was told to name (#193's zealous chief).
func (s *Sim) replaceChief(w *game.World, t *game.Tick, why, personality string) {
	rng := t.Sub(game.StreamLaw)
	old := w.Law.Chief.Name
	name := s.pick(without(s.chiefs, old), rng, old)
	if personality == "" {
		personality = content.ChiefPersonalities[rng.IntN(len(content.ChiefPersonalities))]
	}
	w.Law.Chief = game.Chief{Name: name, Personality: personality, Since: t.Day}
	w.Stats.Chiefs++
	w.Unlearn(game.SubjectChief, game.FactPersonality) // a new chief is one you know nothing about (#45)
	t.Emit(events.ChiefReplaced{Day: t.Day, Name: name, Old: old, Why: why})
}

// without is the pool less one name: nobody succeeds themselves.
func without(pool []string, name string) []string {
	var out []string
	for _, n := range pool {
		if n != name {
			out = append(out, n)
		}
	}
	return out
}

// intelChief files what you know of the chief (#45): their temper at
// full confidence once Observed (the days in office, or their people
// through the door; no dice), and off a cop paid today (World.Today.Cop)
// at the cop's accuracy for the price, a wrong word naming one of the
// other tempers, rolled on Tick.Sub("intel") after the heat sim's roll
// on the same envelope. The observed fact never fades, the cop's does
// (you see for yourself soon enough); a chief replaced takes either
// with them (replaceChief).
func (s *Sim) intelChief(w *game.World, t *game.Tick) {
	chief := w.Law.Chief
	known, _ := game.Known(w).Fact(game.SubjectChief, game.FactPersonality)
	if chief.Observed && chief.Personality != "" && known.Confidence < 1 {
		f := game.Fact{Subject: game.SubjectChief, Kind: game.FactPersonality, Value: chief.Personality, Confidence: 1, Day: t.Day, Source: game.SourceSeen}
		w.Learn(f)
		t.Emit(events.IntelGained{Day: t.Day, Subject: f.Subject, FactKind: f.Kind, Value: f.Value, Confidence: 1, Source: f.Source, Name: chief.Name})
		return
	}
	o := w.Today.Cop
	if o == nil || chief.Personality == "" || known.Confidence >= 1 {
		return
	}
	p := s.intel.Accuracy(o.Amount)
	rng := t.Sub(game.StreamIntel)
	word := chief.Personality
	if rng.Float64() >= p {
		others := without(content.ChiefPersonalities, chief.Personality)
		word = others[rng.IntN(len(others))]
	}
	f := game.Fact{Subject: game.SubjectChief, Kind: game.FactPersonality, Value: word, Confidence: p, Day: t.Day, Source: game.SourceCop, Stale: s.intel.StaleRate, Forget: s.intel.Forget}
	w.Learn(f)
	t.Emit(events.IntelGained{Day: t.Day, Subject: f.Subject, FactKind: f.Kind, Value: word, Confidence: p, Source: f.Source, Name: chief.Name})
}
