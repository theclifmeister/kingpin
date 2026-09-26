package rivals

import "github.com/theclifmeister/kingpin/internal/game"

// Down reads a faction against the crown's count (game.FactionDown,
// #472) off the rule absorb and scatter apply (factions.go), so the
// screens never guess at it: no dice, nothing written. It reads the
// faction this morning; tonight is day w.Day+1, the day absorb reads.
func (s *Sim) Down(w *game.World, r *game.RivalState) game.FactionDown {
	if r == nil {
		return game.FactionDown{}
	}
	if r.Gone() || (r.Arrived > 0 && w.DealWith(r.Faction(), game.DealHomage) != nil) {
		return game.FactionDown{Counts: true}
	}
	f := s.cfg.Factions
	tonight := w.Day + 1
	if r.Arrived == 0 {
		if r.Scouting() {
			return game.FactionDown{Scouting: true}
		}
		d := game.FactionDown{Due: s.ArriveDay(w, r)}
		if f.StrandDays > 0 {
			d.GoneOn = max(tonight, d.Due+f.StrandDays)
		}
		return d
	}
	if n := w.RivalHeldBy(r.Faction()); n > 0 {
		d := game.FactionDown{Corners: n}
		// A run-out faction's claim not yet settled (#495): the spell it
		// paused runs on from its day if the claim is lost before then.
		if sd := f.SettleDays; sd > 0 && r.Spell > 0 && tonight-s.Claimed(w, r) < sd {
			d.Since, d.Settles = r.Spell, s.Claimed(w, r)+sd
		}
		return d
	}
	since := r.RunOut()
	if since == 0 {
		since = tonight // absorb stamps the raid tonight (a save from before RaidedOut)
	}
	d := game.FactionDown{Since: since, GoneOn: max(tonight, since+f.AbsorbDays)}
	raided := r.RaidedOut > r.Routed || (r.Routed == 0 && r.RaidedOut == 0)
	if (raided || r.LastTakenBy == "") && r.Cash >= s.ClaimCost(w, r) {
		d.Rich = true
		d.GoneOn = 0
		if f.StrandDays > 0 {
			d.GoneOn = max(tonight, since+f.StrandDays)
		}
	}
	return d
}
