package heat

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Next is the police's next move in a city as the ladder stands (#45):
// the highest rung whose line the city's heat is at or over (the lowest
// rung, the patrol, under every line) and the first day it can fire,
// tomorrow or the day its cooldown lifts (the arrest has none). With
// investigations on (#343) the sting's day is the night the hit lands:
// an investigation open in the city is the sting on its Due whatever
// the heat, and a sting line met is lead_days past the night it opens.
// It is the truth a cop's word is right about; the file never reads it.
func (s *Sim) Next(w *game.World, city *game.City, day int) (level string, from int) {
	resp := s.Thresholds()
	if len(resp) == 0 {
		return "", day + 1
	}
	level = resp[0].Level
	for _, r := range resp {
		if city.Heat >= s.Threshold(w, r, city) {
			level = r.Level
		}
	}
	from = day + 1
	if last, ok := w.Heat.LastResponse[level]; ok && level != content.Arrest {
		from = max(from, last+s.CooldownDays(w, level))
	}
	if !s.Investigating() || content.Rank(level) > content.Rank(content.Sting) {
		return level, from
	}
	// With investigations on (#343) the sting is a named hit: the day
	// the cop gives is the night it lands, not the night it opens.
	switch inv := w.Heat.Investigation; {
	case inv.Open() && inv.City == city.ID:
		// One open here lands on its night whatever the heat does.
		return content.Sting, max(day+1, inv.Due)
	case level == content.Sting && len(Leads(w, city.ID)) > 0:
		// The line met with a source to name: opened the night the
		// rung is free (one open elsewhere holds it until it lands),
		// the hit lead_days after.
		if inv.Open() {
			from = max(from, inv.Due+1)
		}
		from += s.cfg.Investigation.LeadDays
	}
	return level, from
}

// Due is the response the police would make tonight on the heat as it
// stands this morning (#228), for the favour: the task force announced
// yesterday, else the highest rung the hottest city meets whose
// cooldown has lifted, as Step reads the ladder. It is "" when nothing
// would fire, when the rung met is the task force's (tonight it is
// announced, not made), the patrol's (a cadence, not worth a call) or
// the arrest's (the DA's, and no chief stops it). The night's decay
// runs before the ladder is read, so it is what the morning says, not a
// promise; the favour is spent on the call either way.
func (s *Sim) Due(w *game.World) string {
	if s.TaskForceForming(w) {
		return content.TaskForce
	}
	if inv := w.Heat.Investigation; inv.Open() && w.Day+1 >= inv.Due {
		return content.Sting // an investigation lands tonight (#343)
	}
	hot := s.hottest(w)
	resp := s.Thresholds()
	for i := len(resp) - 1; i >= 0; i-- {
		r := resp[i]
		if hot.Heat < s.Threshold(w, r, hot) {
			continue
		}
		if r.Level == content.TaskForce && !s.TaskForceEligible(w) {
			continue
		}
		if last, ok := w.Heat.LastResponse[r.Level]; ok && w.Day+1-last < s.CooldownDays(w, r.Level) && r.Level != content.Arrest {
			continue
		}
		if r.Level == content.Sting && s.Investigating() && w.Heat.Investigation.Open() {
			continue
		}
		switch r.Level {
		case content.TaskForce, content.Patrol, content.Arrest:
			return ""
		}
		return r.Level
	}
	return ""
}

// cop files the police's next move where the player stands off a cop
// paid today (World.Today.Cop): right at cop_accuracy for the price
// (less money, less often), else off by a rung or up to cop_slip days,
// rolled on Tick.Sub("intel"). The law sim, stepping after, files the
// chief's temper off the same envelope.
func (s *Sim) cop(w *game.World, t *game.Tick) {
	o := w.Today.Cop
	if o == nil {
		return
	}
	tun := s.intel
	city := w.Here()
	level, from := s.Next(w, city, t.Day)
	p := tun.Accuracy(o.Amount)
	rng := t.Sub(game.StreamIntel)
	if rng.Float64() >= p {
		// A wrong word: the rung beside it, or a few days out.
		resp := s.Thresholds()
		if rng.IntN(2) == 0 && len(resp) > 1 {
			i := content.Rank(level) - 1 // its place on the ladder, 0-based
			switch {
			case i <= 0:
				i = 1
			case i >= len(resp)-1:
				i = len(resp) - 2
			case rng.IntN(2) == 0:
				i++
			default:
				i--
			}
			level = resp[i].Level
		} else {
			from += 1 + rng.IntN(max(1, tun.CopSlip))
		}
	}
	f := game.Fact{
		Subject: city.ID, Kind: game.FactResponse, Value: level, Number: float64(from),
		Confidence: p, Day: t.Day, Source: game.SourceCop, Stale: tun.StaleRate, Forget: tun.Forget,
	}
	w.Learn(f)
	t.Emit(events.IntelGained{Day: t.Day, Subject: city.ID, FactKind: game.FactResponse, Value: level, Confidence: p, Source: game.SourceCop, Name: city.Name})
}
