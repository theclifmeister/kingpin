package game

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

// A card shows what each choice costs before you choose (#358). Every
// effect is deterministic, so nothing is uncertain at the moment of
// choosing: Preview runs the choice through the same effects table on a
// copy of the world and reports what moved, clamps and all, so the
// preview is the outcome. The uncertainty is downstream (a grudge
// becomes a war, heat trips a rung, loyalty crosses the skim line), and
// what the lines are is the sims' to say (engine.ChoiceChips), not the
// world's.

// Change is one gauge a choice moves: what it is, whose, and the value
// before and after. Key is one of the Gauges; Member is the crew id for
// a loyalty and 0 otherwise.
type Change struct {
	Key      string
	Member   int
	From, To float64
}

// Delta is how far the gauge moves.
func (c Change) Delta() float64 { return c.To - c.From }

// Gauges are what a card's effects can move, in the order a preview
// lists them: the two bags (dirty_cash and dirty_amount both land in
// dirty), the heat where you stand, each member's loyalty, the four
// numbers the home rival keeps, the units you hold, the corners you
// hold (a card's corner given up, #342), the favours you owe, and the
// three reputation axes. The heat's high-water mark moves with the heat and
// is a stat, not a gauge.
var Gauges = []string{"dirty_cash", "clean_cash", "heat", "loyalty", "war", "grudge", "rival_muscle", "rival_cash", "stock", "corners", "owes", "fear", "respect", "notoriety"}

// gauges reads every gauge in w, in Gauges' order. It only reads: the
// rival's are read off Rivals, not Rival(), which would make one.
func gauges(w *World) []Change {
	out := []Change{
		{Key: "dirty_cash", From: float64(w.Player.DirtyCash)},
		{Key: "clean_cash", From: float64(w.Player.CleanCash)},
	}
	if c := w.Here(); c != nil {
		out = append(out, Change{Key: "heat", From: c.Heat})
	}
	for _, m := range w.Crew.Members {
		out = append(out, Change{Key: "loyalty", Member: m.ID, From: m.Loyalty})
	}
	if len(w.Rivals) > 0 && w.Rivals[0] != nil {
		r := w.Rivals[0]
		out = append(out,
			Change{Key: "war", From: r.War},
			Change{Key: "grudge", From: float64(r.Grudge)},
			Change{Key: "rival_muscle", From: float64(r.Muscle)},
			Change{Key: "rival_cash", From: float64(r.Cash)},
		)
	}
	out = append(out, Change{Key: "stock", From: float64(w.Stashed())})
	held := 0
	for _, c := range w.Corners() {
		if c.Owner == OwnerPlayer {
			held++
		}
	}
	out = append(out, Change{Key: "corners", From: float64(held)}, Change{Key: "owes", From: float64(w.Dilemmas.Owes)})
	for _, a := range Axes {
		out = append(out, Change{Key: a, From: *w.Player.Reputation.Axis(a)})
	}
	return out
}

// diff is what moved between two readings of the gauges, in the
// after's order: a gauge the before lacks (the rival a choice made)
// moved from zero.
func diff(before, after []Change) []Change {
	was := map[[2]any]float64{}
	for _, c := range before {
		was[[2]any{c.Key, c.Member}] = c.From
	}
	var out []Change
	for _, c := range after {
		from := was[[2]any{c.Key, c.Member}]
		if from != c.From {
			out = append(out, Change{Key: c.Key, Member: c.Member, From: from, To: c.From})
		}
	}
	return out
}

// Preview is what choice i on c would change in w: every gauge that
// moves, before and after, in Gauges' order; nil when nothing does.
// It applies the choice through the one effects table (Card.apply,
// Choose's own path) on a copy of w, so the clamps land as they will
// and w is never written. The reputation sim's cap on the three axes'
// sum is applied that evening, to every source at once, and is not in
// the figures. A card with Hide still previews: hiding it is the
// front end's.
func (c *Card) Preview(w *World, i int) ([]Change, error) {
	if i < 0 || i >= len(c.Choices) {
		return nil, ErrBadChoice
	}
	cp, err := w.clone()
	if err != nil {
		return nil, err
	}
	if err := c.apply(cp, i); err != nil {
		return nil, err
	}
	return diff(gauges(w), gauges(cp)), nil
}

// Moved is what moved in w since a reading of its gauges, for a test
// that holds Preview to what Choose did.
func Moved(before []Change, w *World) []Change { return diff(before, gauges(w)) }

// Reading is w's gauges now, the before half of Moved.
func Reading(w *World) []Change { return gauges(w) }

// clone is a deep copy of w, through the save's own encoding: whatever
// a save keeps, the copy has, and nothing it shares with w.
func (w *World) clone() (*World, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(w); err != nil {
		return nil, fmt.Errorf("copy world: %w", err)
	}
	var cp World
	if err := gob.NewDecoder(&buf).Decode(&cp); err != nil {
		return nil, fmt.Errorf("copy world: %w", err)
	}
	return &cp, nil
}
