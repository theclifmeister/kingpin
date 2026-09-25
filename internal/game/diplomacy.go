package game

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/theclifmeister/kingpin/internal/format"
)

// Deal kinds. The joint shipment (#30) is proposed but not yet struck:
// it needs routes.
const (
	DealTruce    = "truce"    // neither side contests corners; no undercutting, no tips
	DealTribute  = "tribute"  // you pay a day; it leaves your corners alone
	DealSplit    = "split"    // a line through the city; each side keeps to its own
	DealShipment = "shipment" // half the cost and half the loss of one shipment (#30)
	DealHomage   = "homage"   // they pay you a day (#43, #32's tribute in reverse); a faction that has lost enough ground to you offers it
)

var (
	ErrNoRival     = errors.New("nobody to deal with yet")
	ErrDealLive    = errors.New("you already have that deal")
	ErrOfferLive   = errors.New("they have already offered that; answer it first")
	ErrNoProposal  = errors.New("no proposal on the table today") // #474: Withdraw with nothing to take back
	ErrDistrusted  = errors.New("they are not taking your calls")
	ErrNoOffer     = errors.New("no such offer")
	ErrOfferLapsed = errors.New("that offer has lapsed")
	ErrNoRoutes    = errors.New("joint shipments need routes (#30)")
	ErrBadTerms    = errors.New("those are not terms")
)

// Terms are what a deal says. Which fields matter depends on the kind:
// Days for a truce, PerDay (dirty cash) for a tribute, Corners for a
// split (the ids on the player's side of the line), Route and Units for
// a joint shipment.
type Terms struct {
	Days    int
	PerDay  int
	Corners []string
	Route   string
	Units   int
}

// Deal is an agreement with the rival: its kind and terms, the day it
// was struck and the first day it no longer holds (0 for a deal with no
// end): a deal holds on every tick before Until, and the rival sim ends
// it at the close of the last one. Offered says the rival proposed it.
type Deal struct {
	Kind    string
	Terms   Terms
	Since   int
	Until   int
	Offered bool
	Faction string // the faction it is with (#43); "" is the rival at home
}

// Offer is a deal a faction has put on the table, good until Expires.
type Offer struct {
	ID      int
	Deal    Deal
	Expires int
	Faction string // who offered it (#43); "" is the rival at home
}

// With is the faction a deal or an offer is with, resolved: its Faction,
// or the rival at home's.
func (d Deal) With() string {
	if d.Faction == "" {
		return FactionRival
	}
	return d.Faction
}

// With is the faction that made the offer, resolved.
func (o Offer) With() string {
	if o.Faction == "" {
		return FactionRival
	}
	return o.Faction
}

// Live reports whether the deal holds on the tick of day.
func (d Deal) Live(day int) bool { return d.Until == 0 || day < d.Until }

// Left is how many more ticks the deal holds after the tick of day: what
// the morning screen calls days left. A deal with no end reports 0.
func (d Deal) Left(day int) int {
	if d.Until == 0 {
		return 0
	}
	return max(0, d.Until-day-1)
}

// Covers reports whether a split puts the corner on the player's side of
// the line. A deal of any other kind covers nothing.
func (d Deal) Covers(corner string) bool {
	return d.Kind == DealSplit && slices.Contains(d.Terms.Corners, corner)
}

// String is the deal in words, for the report and the screens.
func (d Deal) String() string {
	switch d.Kind {
	case DealTruce:
		return fmt.Sprintf("a %d-day truce", d.Terms.Days)
	case DealTribute:
		return "tribute of " + format.Money(d.Terms.PerDay) + " a day"
	case DealSplit:
		return fmt.Sprintf("a split: %s your side of the line", format.Plural(len(d.Terms.Corners), "corner"))
	case DealHomage:
		return "homage of " + format.Money(d.Terms.PerDay) + " a day to you"
	case DealShipment:
		return fmt.Sprintf("a joint shipment of %d units", d.Terms.Units)
	}
	return d.Kind
}

// Describe is the deal in words with the corners named, for the screens
// and the report: `a split: yours Rail Yard, Docks`.
func (w *World) Describe(d Deal) string {
	if d.Kind != DealSplit {
		return d.String()
	}
	return "a split: yours " + w.Side(d)
}

// Side names the corners a split leaves on the player's side of the
// line, `Rail Yard, Docks`, for a screen that has already said it is a
// split.
func (w *World) Side(d Deal) string {
	var names []string
	for _, id := range d.Terms.Corners {
		if c := w.Corner(id); c != nil {
			names = append(names, c.Name)
		}
	}
	return strings.Join(names, ", ")
}

// Deal returns the deal of a kind that holds tonight with the rival at
// home, or nil.
func (w *World) Deal(kind string) *Deal { return w.DealWith("", kind) }

// DealWith returns the deal of a kind that holds tonight with a faction
// (#43), or nil.
func (w *World) DealWith(faction, kind string) *Deal {
	r := w.Faction(faction)
	if r == nil {
		return nil
	}
	for i := range r.Deals {
		if d := &r.Deals[i]; d.Kind == kind && d.Live(w.Day+1) {
			return d
		}
	}
	return nil
}

// AtPeace reports whether a live deal keeps the rival at home off the
// player's corners today: a truce or a tribute.
func (w *World) AtPeace() bool { return w.AtPeaceWith("") }

// AtPeaceWith reports whether a truce, a tribute or a homage holds with
// a faction tonight (#43): the peace that keeps it off your corners and
// you off its.
func (w *World) AtPeaceWith(faction string) bool {
	return w.DealWith(faction, DealTruce) != nil || w.DealWith(faction, DealTribute) != nil || w.DealWith(faction, DealHomage) != nil
}

// CornerPeace reports whether a deal keeps you off a faction's corner
// tonight: a peace with the faction that holds it, or its side of a
// split's line. A corner nobody's is nobody's business.
func (w *World) CornerPeace(c Corner) bool {
	if c.Owner != OwnerRival {
		return false
	}
	if w.AtPeaceWith(c.Faction) {
		return true
	}
	if d := w.DealWith(c.Faction, DealSplit); d != nil && !d.Covers(c.ID) {
		return true
	}
	return false
}

// Offer returns the pending offer with id, or nil.
func (w *World) Offer(id int) *Offer {
	return find(w.Offers, func(e *Offer) bool { return e.ID == id })
}

// Propose puts a deal to the rival at home; it answers in the morning.
// One a day: proposing again replaces it. Terms are checked for shape
// here and for taste by the rival.
func (w *World) Propose(kind string, terms Terms) error { return w.ProposeTo("", kind, terms) }

// ProposeTo puts a deal to a faction (#43): Propose, with the faction
// named. A homage is theirs to offer, never yours to ask.
func (w *World) ProposeTo(faction, kind string, terms Terms) error {
	if w.Over != nil {
		return ErrGameOver
	}
	r := w.Faction(faction)
	if r == nil {
		return ErrNoFaction
	}
	if !r.Alive() {
		return ErrNoRival
	}
	switch kind {
	case DealTruce:
		if terms.Days <= 0 {
			return ErrBadTerms
		}
	case DealTribute:
		if terms.PerDay <= 0 {
			return ErrBadTerms
		}
	case DealSplit:
		if len(terms.Corners) == 0 {
			return ErrBadTerms
		}
		for _, id := range terms.Corners {
			if w.Corner(id) == nil {
				return ErrNoCorner
			}
		}
	case DealShipment:
		return ErrNoRoutes
	default:
		return ErrBadTerms
	}
	if w.DealWith(r.Faction(), kind) != nil {
		return ErrDealLive
	}
	for _, o := range w.Offers {
		if o.Deal.Kind == kind && o.With() == r.Faction() {
			return ErrOfferLive
		}
	}
	w.Today.Proposal = &Deal{Kind: kind, Terms: terms, Faction: faction}
	return nil
}

// Withdraw takes back today's proposal; with none made it refuses
// (ErrNoProposal, #474) rather than doing nothing.
func (w *World) Withdraw() error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Today.Proposal == nil {
		return ErrNoProposal
	}
	w.Today.Proposal = nil
	return nil
}

// Accept takes the rival's offer with id. The deal is sealed at end of
// day by the rival sim, on exactly the terms offered, and runs from
// tomorrow. An offer past its day cannot be taken.
func (w *World) Accept(id int) (Offer, error) {
	if w.Over != nil {
		return Offer{}, ErrGameOver
	}
	o := w.Offer(id)
	if o == nil {
		return Offer{}, ErrNoOffer
	}
	if w.Day > o.Expires {
		return Offer{}, ErrOfferLapsed
	}
	if w.DealWith(o.With(), o.Deal.Kind) != nil {
		return Offer{}, ErrDealLive
	}
	taken := *o
	w.dropOffer(id)
	w.Today.Accepted = append(w.Today.Accepted, taken)
	return taken, nil
}

// Decline turns the rival's offer with id down. Nothing is said about it.
func (w *World) Decline(id int) (Offer, error) {
	if w.Over != nil {
		return Offer{}, ErrGameOver
	}
	o := w.Offer(id)
	if o == nil {
		return Offer{}, ErrNoOffer
	}
	taken := *o
	w.dropOffer(id)
	return taken, nil
}

func (w *World) dropOffer(id int) {
	w.Offers = slices.DeleteFunc(w.Offers, func(o Offer) bool { return o.ID == id })
}

// HomeCorners are the corners of the home city, the ground the rival and
// its deals are about.
func (w *World) HomeCorners() []Corner {
	if h := w.Home(); h != nil {
		return h.Corners
	}
	return nil
}

// SplitLines are the three lines a split can be proposed along at home,
// the player's side growing with each: what you hold; that plus the free
// corners next to yours and not next to theirs; that plus every free
// corner. A rival corner is never on your side.
func (w *World) SplitLines() [][]string { return w.SplitLinesWith("") }

// SplitLinesWith are the split lines in a faction's city (#43), theirs
// being the corners of any faction there.
func (w *World) SplitLinesWith(faction string) [][]string {
	var held, near, all []string
	corners := w.HomeCorners()
	if r := w.Faction(faction); r != nil && r.Home != "" {
		if city := w.Cities[r.Home]; city != nil {
			corners = city.Corners
		}
	}
	for _, c := range corners {
		switch c.Owner {
		case OwnerPlayer:
			held = append(held, c.ID)
		case OwnerNone:
			all = append(all, c.ID)
			next, theirs := false, false
			for _, o := range corners {
				if c.Borders(o) {
					next = next || o.Held()
					theirs = theirs || o.Owner == OwnerRival
				}
			}
			if next && !theirs {
				near = append(near, c.ID)
			}
		}
	}
	lines := make([][]string, 3)
	lines[0] = append([]string(nil), held...)
	lines[1] = append(append([]string(nil), held...), near...)
	lines[2] = append(append([]string(nil), held...), all...)
	return lines
}
