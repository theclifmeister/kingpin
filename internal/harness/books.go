package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
)

// Saboteur plays like Crewed and works the rival's books (#70) instead
// of striking: it scouts whenever the snapshot is stale or was never
// read, boosts the rival's biggest corner every night the odds at the
// cheapest force clear SaboteurOdds and the war is under the open line
// (warn only while a deal holds, since a boost at push or hit under one
// is a betrayal; a boost a night whatever the war bar said brought a
// crackdown every twenty days), and on a night the odds fall short at
// every force buys off a head of muscle and boosts at hit behind it,
// whenever dirty cash is over SaboteurMargin times the price and the
// books as last read show muscle to buy (a head bought off leaves
// before the enforcers go in, so the buy-off is the defence-shredder
// priced against a strike; a rival that can pay for another head hires
// it back in the morning, and a saboteur that bought heads to thin the
// rival for good bought 58 on one seed and it kept seven), and tips
// the police on the rival's biggest corner
// on the nights it holds more corners than the player, no deal is live
// and the police are ready to act (a tip a night between raids is pages
// for nothing), starting a run of tips only while its own heat is under
// the line and then keeping it up until the raid lands (tips spread out
// fade before the police notice). It never sends the enforcers for
// ground. It is the baseline for "a player who dismantles the rival".
func Saboteur(cfg *content.Config, lieLowAt float64) Policy {
	return saboteur(cfg, lieLowAt, tipsAhead)
}

// Tipster is Saboteur that tips the police every night the rival holds a
// corner, deal or no deal: the price of talking to police, measured.
func Tipster(cfg *content.Config, lieLowAt float64) Policy {
	return saboteur(cfg, lieLowAt, tipsAlways)
}

// SaboteurOdds is the boost odds the saboteur wants before it sends the
// enforcers for a corner's takings.
const SaboteurOdds = 0.5

// SaboteurMargin is how many times the price of a head the saboteur
// holds in dirty cash before it buys one off.
const SaboteurMargin = 5

// SaboteurThin is the chest, in days of the rival's wage bill as last
// read, under which the saboteur buys heads off: a rival that can pay
// for another head hires it back, and one that cannot pay the ones it
// has saves their wages when they go.
const SaboteurThin = 3

// SaboteurWar is the share of the crackdown line the saboteur keeps the
// war under before it sends the enforcers for a till.
const SaboteurWar = 0.75

// tipping is how a saboteur handles the police: never, when the rival
// is ahead and the police are ready, or every night it holds a corner.
type tipping int

const (
	tipsNever tipping = iota
	tipsAhead
	tipsAlways
)

func saboteur(cfg *content.Config, lieLowAt float64, tips tipping) Policy {
	crewed := Crewed(cfg, lieLowAt)
	rv := rivals.New(cfg)
	hot := TooHot(cfg, lieLowAt)
	tun := cfg.Rivals.Rivals
	forces := []events.Force{events.ForceWarn, events.ForcePush, events.ForceHit}
	return func(w *game.World) {
		crewed(w)
		r := w.Rival
		if r.Arrived == 0 || w.RivalHeld() == 0 {
			return
		}
		if (!r.Known.Read() || rv.Stale(w, w.Day)) && w.Scouting == nil {
			_ = w.Scout(rv.ScoutCost())
		}
		target := pickCorner(w, func(c game.Corner) bool { return c.Owner == game.OwnerRival }, size)
		if target == nil {
			return
		}
		if !hot(w) && w.Crew.Role("enforcer") > 0 && w.Strike == nil && r.War < SaboteurWar*tun.CrackdownThreshold {
			for _, f := range forces {
				if f != events.ForceWarn && len(r.Deals) > 0 {
					break
				}
				if rv.Odds(w, f) > SaboteurOdds {
					_ = w.Boost(target.ID, f)
					break
				}
			}
		}
		if k := r.Known; k.Read() && k.Muscle > 0 && k.Cash < SaboteurThin*k.Wages {
			if price := rv.MusclePrice(w); w.Poach == nil && w.Player.DirtyCash > SaboteurMargin*price {
				_ = w.BuyOff(1, price)
			}
		}
		ahead := w.RivalHeld() > w.HeldIn(w.Home().ID) && len(r.Deals) == 0 && rv.RaidReady(w, w.Day+1)
		if w.Tipoff == nil && (tips == tipsAlways || (tips == tipsAhead && ahead && (r.Heat > 0 || !hot(w)))) {
			_ = w.Tip(target.ID)
		}
	}
}
