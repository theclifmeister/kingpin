package game

import "github.com/theclifmeister/kingpin/internal/content"

// Progression is the tier the run is in (#147): Reached maps a tier's
// number (1 the first) to the day it was entered. A tier once reached
// stays reached, whatever the pile does after. The news sim stamps it,
// the first morning the tier's trigger holds, one tier a morning; no
// sim reads it to change what it does (a tier describes the gates, it
// is not one). The zero value is the pre-#147 world, so a save from
// before it loads with nothing reached and catches up on its first
// mornings; no schema bump.
type Progression struct {
	Reached map[int]int
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
