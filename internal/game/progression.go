package game

import "github.com/theclifmeister/kingpin/internal/content"

// Progression is the tier the run is in (#147): Reached maps a tier's
// number (1 the first) to the day it was entered. A tier once reached
// stays reached, whatever the pile does after. The news sim stamps it,
// the first morning the tier's trigger holds, one tier a morning; no
// sim reads it to change what it does (a tier describes the gates, it
// is not one). The zero value is the pre-#147 world, so a save from
// before it loads with nothing reached and catches up on its first
// mornings; no schema bump. Seen is the tiers whose stage the player
// has been shown (#149, the interstitial): UI state kept on the world
// so a save on the modal reopens it, as a save on a card does; nil is
// nothing seen, so no schema bump either.
type Progression struct {
	Reached map[int]int
	Seen    map[int]bool
}

// Tier is the highest tier reached, 1 while none is stamped: the first
// tier is day 0 and never stamped.
func (w *World) Tier() int {
	n := 1
	for t := range w.Progression.Reached {
		n = max(n, t)
	}
	return n
}

// ReachedOn is the day tier n was entered, or -1 if it never was; tier
// 1 is day 0.
func (w *World) ReachedOn(n int) int {
	if n == 1 {
		return 0
	}
	if d, ok := w.Progression.Reached[n]; ok {
		return d
	}
	return -1
}

// Reach stamps tier n as entered on day.
func (w *World) Reach(n, day int) {
	if w.Progression.Reached == nil {
		w.Progression.Reached = map[int]int{}
	}
	w.Progression.Reached[n] = day
}

// StagePending is the highest tier reached whose stage has not been
// shown, or 0: what the interstitial (#149) opens on. The highest only,
// so a save from before the stage, with three tiers reached and none
// seen, is shown one modal and not walked through them; SeeStage marks
// the lower ones seen with it. Tier 1 is never pending: it is day 0,
// never stamped, and a run does not open on a stage.
func (w *World) StagePending() int {
	n := 0
	for t := range w.Progression.Reached {
		if t > n && !w.Progression.Seen[t] {
			n = t
		}
	}
	return n
}

// SeeStage marks tier n's stage seen, and every tier under it: the
// stage shown is the highest reached, and the ones it passed are not
// shown after it.
func (w *World) SeeStage(n int) {
	if w.Progression.Seen == nil {
		w.Progression.Seen = map[int]bool{}
	}
	for t := range w.Progression.Reached {
		if t <= n {
			w.Progression.Seen[t] = true
		}
	}
}

// TierName is the name of the tier the run is in, as the file spells it.
func (w *World) TierName(cfg content.ProgressionConfig) string {
	if t := cfg.Tier(w.Tier()); t != nil {
		return t.Name
	}
	return ""
}

// CitiesHeld counts the cities with a held corner: the lieutenant gate's
// count (crew.LieutenantsWanted) and a trigger's (cities_held).
func (w *World) CitiesHeld() int {
	n := 0
	for _, cid := range w.CityOrder {
		if w.HeldIn(cid) > 0 {
			n++
		}
	}
	return n
}
