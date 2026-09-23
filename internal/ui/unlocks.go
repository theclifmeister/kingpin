package ui

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The gates ahead (#148). Every door the game keeps behind a line of
// peak cash (a product on the ladder, a front's offer, a connect, an
// asset) is named before it fires, on the screen that owns the thing,
// and the nearest one is an alert while it is within reach. Which gates
// there are and which is near is the engine's (#298, engine.Gate,
// Session.GatesAhead, engine.AlertGate); the words are here. The rival
// and the roles have no cash line and no alert: the crew screen's pool
// title names what the lieutenants wait on.

// gateToGo is the distance to the gate's line, `$1.2K to go`.
func gateToGo(w *game.World, g engine.Gate) string { return cash(g.ToGo(w)) + " to go" }

// gateThe is the gate's name as a sentence names it: a front takes the
// article, a product and a connect their name.
func gateThe(g engine.Gate) string {
	if g.Kind == "front" {
		return "the " + g.Name
	}
	return g.Name
}

// gateText is the alert's line: what opens at what line and how far it
// is (`The Laundromat opens at $25K peak: $18K to go.`, `Heroin lists
// at $3K peak: $1.2K to go.`, `The Dutchman deals at $500K peak: $120K
// to go.`).
func gateText(w *game.World, g engine.Gate) string {
	name, verb := g.Name, "opens"
	switch g.Kind {
	case "product":
		verb = "lists"
	case "connect":
		verb = "deals"
	case "front":
		name = "The " + g.Name
	case "asset":
		verb = "is for sale"
	}
	line := fmt.Sprintf("%s %s at %s peak", name, verb, cash(g.Line))
	if g.Clean {
		line += " clean"
	}
	if g.Vouch != "" {
		line += " and " + g.Vouch
	}
	return line + ": " + gateToGo(w, g) + "."
}

// nextProduct is the next product on the ladder not yet listed in the
// city (content's file order), or nil once the ladder is listed.
func (m *Model) nextProduct(city string) *engine.Gate {
	w := m.w
	for _, p := range m.cfg.Market.Products {
		if w.Product(city, p.ID) == nil {
			return &engine.Gate{Kind: "product", ID: p.ID, Name: p.Name, Line: p.UnlockCash}
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
	line := fmt.Sprintf("%s lists at %s peak cash (%s)", g.Name, cash(g.Line), gateToGo(m.w, *g))
	if c := m.cfg.City.City(city); c != nil && c.Product(g.ID).NoSupply {
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
