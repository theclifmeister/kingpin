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
)

var (
	ErrNoRival     = errors.New("nobody to deal with yet")
	ErrDealLive    = errors.New("you already have that deal")
	ErrOfferLive   = errors.New("they have already offered that; answer it first")
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
}

// Offer is a deal the rival has put on the table, good until Expires.
type Offer struct {
	ID      int
	Deal    Deal
	Expires int
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

// Deal returns the deal of a kind that holds tonight, or nil.
func (w *World) Deal(kind string) *Deal {
	for i := range w.Rival.Deals {
		if d := &w.Rival.Deals[i]; d.Kind == kind && d.Live(w.Day+1) {
			return d
		}
	}
	return nil
}

// AtPeace reports whether a live deal keeps the rival off the player's
// corners today: a truce or a tribute.
func (w *World) AtPeace() bool { return w.Deal(DealTruce) != nil || w.Deal(DealTribute) != nil }

// Offer returns the pending offer with id, or nil.
func (w *World) Offer(id int) *Offer {
	for i := range w.Offers {
		if w.Offers[i].ID == id {
			return &w.Offers[i]
		}
	}
	return nil
}

// Propose puts a deal to the rival; it answers in the morning. One a
// day: proposing again replaces it. Terms are checked for shape here and
// for taste by the rival.
func (w *World) Propose(kind string, terms Terms) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Rival.Arrived == 0 {
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
	if w.Deal(kind) != nil {
		return ErrDealLive
	}
	for _, o := range w.Offers {
		if o.Deal.Kind == kind {
			return ErrOfferLive
		}
	}
	w.Today.Proposal = &Deal{Kind: kind, Terms: terms}
	return nil
}

// Withdraw takes back today's proposal.
func (w *World) Withdraw() { w.Today.Proposal = nil }

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
	if w.Deal(o.Deal.Kind) != nil {
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
func (w *World) SplitLines() [][]string {
	var held, near, all []string
	corners := w.HomeCorners()
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
