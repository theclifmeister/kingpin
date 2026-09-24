package laundering

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The filthy rich (#392): the trophies, clean money this sim owns as it
// owns the assets, and the rot on a dirty pile nobody can keep dry.

// TrophyOffers lists every trophy in the file, cheapest first, priced
// for BuyTrophy; locked ones are included so the ledger can show what
// is coming.
func (s *Sim) TrophyOffers() []game.TrophyOffer {
	out := make([]game.TrophyOffer, 0, len(s.trophies.Offers))
	for _, t := range s.trophies.Offers {
		out = append(out, game.TrophyOffer{ID: t.ID, Name: t.Name, Cost: t.Cost, UnlockCash: t.UnlockCash})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Cost < out[j].Cost })
	return out
}

// TrophyOffer is the priced offer for a trophy id.
func (s *Sim) TrophyOffer(id string) (game.TrophyOffer, bool) {
	for _, o := range s.TrophyOffers() {
		if o.ID == id {
			return o, true
		}
	}
	return game.TrophyOffer{}, false
}

// BuyTrophy buys the trophy under id at the file's price, or
// ErrNoTrophy for an id the file does not know.
func (s *Sim) BuyTrophy(w *game.World, id string) (game.Trophy, error) {
	o, ok := s.TrophyOffer(id)
	if !ok {
		return game.Trophy{}, game.ErrNoTrophy
	}
	return w.BuyTrophy(o)
}

// trophiesStep is the trophies' books: what the task force took
// tonight (the heat sim's TrophySeized) comes off them, and yesterday's
// purchases are reported. No dice.
func (s *Sim) trophiesStep(w *game.World, t *game.Tick) {
	for _, e := range t.Events() {
		if ev, ok := e.(events.TrophySeized); ok {
			w.LoseTrophy(ev.Trophy, t.Day, "seized")
		}
	}
	for _, tr := range w.Trophies {
		if tr.Bought == t.Day-1 {
			t.Emit(events.TrophyBought{Day: t.Day, Trophy: tr.ID, Name: tr.Name, Cost: tr.Cost})
		}
	}
}

// Rot is what tonight's rot would take of a dirty pile: rot of what is
// over rot_line, nothing at or under it or with no line in the file.
func (s *Sim) Rot(pile int) int {
	tun := s.cfg.Laundering
	if tun.RotLine <= 0 || tun.Rot <= 0 || pile <= tun.RotLine {
		return 0
	}
	return int(math.Round(float64(pile-tun.RotLine) * tun.Rot))
}

// RotLine is the pile the rot starts over, for the ledger.
func (s *Sim) RotLine() int { return s.cfg.Laundering.RotLine }

// rot takes the night's rot off the dirty pile, after the wash, so what
// the fronts washed tonight is out of the damp. No dice.
func (s *Sim) rot(w *game.World, t *game.Tick) {
	amt := s.Rot(w.Player.DirtyCash)
	if amt <= 0 {
		return
	}
	w.Player.DirtyCash -= amt
	w.Stats.Rotted += amt
	t.Emit(events.CashRotted{Day: t.Day, Amount: amt, Pile: w.Player.DirtyCash})
}
