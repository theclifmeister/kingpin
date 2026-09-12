package ui

import (
	"fmt"
	"sort"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The gates ahead (#148). Every door the game keeps behind a line of
// peak cash (a product on the ladder, a front's offer, a connect) is
// named before it fires, on the screen that owns the thing, and the
// nearest one is an alert while it is within reach: under unlockNear
// of its line to go. The alert is UI over World and Stats.PeakCash,
// nothing saved, and keyed per gate, so a fast-forward stops once when
// a gate comes into reach and once more, on the Unlocked event, when it
// fires. The rival and the roles have no cash line and no alert: the
// crew screen's pool title names what the lieutenants wait on.

// unlockNear is how close a gate must be before the dashboard names
// it: the distance to go under this share of the line.
const unlockNear = 0.5

// gate is one door ahead: what it opens, the line it opens at and, for
// a connect, who must vouch besides.
type gate struct {
	kind  string // product | front | connect
	id    string
	name  string
	line  int    // peak cash it opens at
	vouch string // a connect's second condition, `Cass at 60`, or ""
}

// gatesAhead is every cash gate above the peak, nearest first (file
// order within a line): the products not yet listed where you stand,
// the fronts on offer still locked and the connects whose door is shut.
func (m *Model) gatesAhead() []gate {
	w := m.w
	peak := w.Stats.PeakCash
	var out []gate
	for _, p := range m.cfg.Market.Products {
		if p.UnlockCash > peak && w.Product(w.Here().ID, p.ID) == nil {
			out = append(out, gate{kind: "product", id: p.ID, name: p.Name, line: p.UnlockCash})
		}
	}
	for _, o := range m.frontRows() {
		if o.UnlockCash > peak {
			out = append(out, gate{kind: "front", id: o.ID, name: o.Name, line: o.UnlockCash})
		}
	}
	for i := range w.Suppliers {
		sup := &w.Suppliers[i]
		if sup.Opened || sup.UnlockCash <= peak {
			continue
		}
		g := gate{kind: "connect", id: sup.ID, name: sup.Name, line: sup.UnlockCash}
		if st := w.StreetSupplier(sup.City); sup.UnlockRel > 0 && st != nil && st.ID != sup.ID {
			g.vouch = fmt.Sprintf("%s at %.0f", st.Name, sup.UnlockRel)
		}
		out = append(out, g)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].line < out[j].line })
	return out
}

// nextGates are the gates at the nearest line ahead.
func (m *Model) nextGates() []gate {
	all := m.gatesAhead()
	var out []gate
	for _, g := range all {
		if g.line != all[0].line {
			break
		}
		out = append(out, g)
	}
	return out
}

// near reports whether the gate is within reach: under unlockNear of
// its line still to go.
func (g gate) near(peak int) bool { return float64(g.line-peak) < float64(g.line)*unlockNear }

// toGo is the distance to the line, `$1.2K to go`.
func (g gate) toGo(peak int) string { return cash(g.line-peak) + " to go" }

// the is the gate's name as a sentence names it: a front takes the
// article, a product and a connect their name.
func (g gate) the() string {
	if g.kind == "front" {
		return "the " + g.name
	}
	return g.name
}

// text is the alert's line: what opens at what line and how far it is
// (`The Laundromat opens at $25K peak: $18K to go.`, `Heroin lists at
// $3K peak: $1.2K to go.`, `The Dutchman deals at $500K peak: $120K to
// go.`).
func (g gate) text(peak int) string {
	name, verb := g.name, "opens"
	switch g.kind {
	case "product":
		verb = "lists"
	case "connect":
		verb = "deals"
	case "front":
		name = "The " + g.name
	}
	line := fmt.Sprintf("%s %s at %s peak", name, verb, cash(g.line))
	if g.vouch != "" {
		line += " and " + g.vouch
	}
	return line + ": " + g.toGo(peak) + "."
}

// unlockAlerts is the alert for each gate at the nearest line while it
// is within reach, keyed per gate.
func (m *Model) unlockAlerts() []alert {
	peak := m.w.Stats.PeakCash
	var out []alert
	for _, g := range m.nextGates() {
		if !g.near(peak) {
			continue
		}
		out = append(out, alert{theme.Gold.Render(g.text(peak)), g.the() + " within reach", "unlock:" + g.kind + ":" + g.id})
	}
	return out
}

// nextProduct is the next product on the ladder not yet listed in the
// city (content's file order), or nil once the ladder is listed.
func (m *Model) nextProduct(city string) *gate {
	w := m.w
	for _, p := range m.cfg.Market.Products {
		if w.Product(city, p.ID) == nil {
			return &gate{kind: "product", id: p.ID, name: p.Name, line: p.UnlockCash}
		}
	}
	return nil
}

// nextProductNote is the market pane's line on the next product on the
// ladder in the shown city: `Heroin lists at $3K peak cash ($1.2K to
// go)`, or the honest form where the supplier there will not sell it.
func (m *Model) nextProductNote(city string) string {
	g := m.nextProduct(city)
	if g == nil {
		return ""
	}
	line := fmt.Sprintf("%s lists at %s peak cash (%s)", g.name, cash(g.line), g.toGo(m.w.Stats.PeakCash))
	if c := m.cfg.City.City(city); c != nil && c.Product(g.id).NoSupply {
		line += "; the supplier here will not sell it, the road brings it"
	}
	return line + "."
}

// lockedStatus is the ledger's status for a front on offer still
// locked: `locked · $18K to go`, or `$18K to go` where the table is
// short of room.
func lockedStatus(w *game.World, o game.FrontOffer, long bool) string {
	toGo := cash(o.UnlockCash-w.Stats.PeakCash) + " to go"
	if long {
		return "locked · " + toGo
	}
	return toGo
}
