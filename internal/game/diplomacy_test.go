package game

import (
	"errors"
	"testing"
)

// dealWorld is testWorld with a rival in town on a third corner.
func dealWorld() *World {
	w := testWorld()
	h := w.Home()
	h.Corners = append(h.Corners, Corner{ID: "strip", City: h.ID, Name: "Strip", X: 2, Demand: 1, Heat: 1, Risk: 1, Owner: OwnerRival})
	w.Rival.Leader, w.Rival.Arrived, w.Rival.Trust = "Rico", 1, 40
	w.Day = 10
	return w
}

// Proposals are checked for shape, one a day, none against a live deal
// or a pending offer, and the joint shipment waits on routes.
func TestPropose(t *testing.T) {
	w := dealWorld()
	if err := w.Propose(DealTruce, Terms{}); !errors.Is(err, ErrBadTerms) {
		t.Fatalf("a truce with no days: %v", err)
	}
	if err := w.Propose(DealShipment, Terms{Units: 10}); !errors.Is(err, ErrNoRoutes) {
		t.Fatalf("a joint shipment: %v", err)
	}
	if err := w.Propose("alliance", Terms{}); !errors.Is(err, ErrBadTerms) {
		t.Fatalf("an unknown deal: %v", err)
	}
	if err := w.Propose(DealSplit, Terms{Corners: []string{"nowhere"}}); !errors.Is(err, ErrNoCorner) {
		t.Fatalf("a split over no corner: %v", err)
	}
	if err := w.Propose(DealTruce, Terms{Days: 15}); err != nil {
		t.Fatal(err)
	}
	if err := w.Propose(DealTribute, Terms{PerDay: 500}); err != nil || w.Today.Proposal.Kind != DealTribute {
		t.Fatalf("proposing again should replace: %v %+v", err, w.Today.Proposal)
	}
	w.Withdraw()
	if w.Today.Proposal != nil {
		t.Fatal("withdraw left the proposal")
	}
	w.Rival.Deals = []Deal{{Kind: DealTruce, Terms: Terms{Days: 15}, Since: 10, Until: 25}}
	if err := w.Propose(DealTruce, Terms{Days: 15}); !errors.Is(err, ErrDealLive) {
		t.Fatalf("a truce over a truce: %v", err)
	}
	w.Offers = []Offer{{ID: 1, Deal: Deal{Kind: DealTribute, Terms: Terms{PerDay: 300}}, Expires: 14}}
	if err := w.Propose(DealTribute, Terms{PerDay: 500}); !errors.Is(err, ErrOfferLive) {
		t.Fatalf("a tribute over a tribute offer: %v", err)
	}
	w.Rival.Arrived = 0
	if err := w.Propose(DealSplit, Terms{Corners: []string{"home"}}); !errors.Is(err, ErrNoRival) {
		t.Fatalf("a proposal before the rival is in town: %v", err)
	}
}

// Accepting queues the offer for the rival sim on exactly its terms and
// takes it off the table; declining just takes it off; a lapsed offer
// cannot be taken.
func TestAcceptAndDecline(t *testing.T) {
	w := dealWorld()
	w.Offers = []Offer{
		{ID: 1, Deal: Deal{Kind: DealTruce, Terms: Terms{Days: 30}, Offered: true}, Expires: 14},
		{ID: 2, Deal: Deal{Kind: DealTribute, Terms: Terms{PerDay: 300}, Offered: true}, Expires: 9},
	}
	if _, err := w.Accept(2); !errors.Is(err, ErrOfferLapsed) {
		t.Fatalf("taking a lapsed offer: %v", err)
	}
	if _, err := w.Accept(9); !errors.Is(err, ErrNoOffer) {
		t.Fatalf("taking no offer: %v", err)
	}
	o, err := w.Accept(1)
	if err != nil {
		t.Fatal(err)
	}
	if o.Deal.Terms.Days != 30 || len(w.Today.Accepted) != 1 || w.Today.Accepted[0].ID != 1 || len(w.Offers) != 1 || w.Offers[0].ID != 2 {
		t.Fatalf("after accepting: %+v accepted %+v offers %+v", o, w.Today.Accepted, w.Offers)
	}
	if _, err := w.Decline(2); err != nil || len(w.Offers) != 0 {
		t.Fatalf("declining: %v offers %+v", err, w.Offers)
	}
}

// A deal holds on the ticks before Until and the morning asks about the
// coming tick; a split covers its corners; the words read right.
func TestDealLifetime(t *testing.T) {
	d := Deal{Kind: DealTruce, Terms: Terms{Days: 15}, Since: 11, Until: 26}
	if !d.Live(11) || !d.Live(25) || d.Live(26) {
		t.Fatalf("a truce from 11 to 26 is live on 11..25: %v %v %v", d.Live(11), d.Live(25), d.Live(26))
	}
	if d.Left(11) != 14 || d.Left(25) != 0 {
		t.Fatalf("days left: %d on 11, %d on 25", d.Left(11), d.Left(25))
	}
	w := dealWorld()
	w.Rival.Deals = []Deal{d}
	w.Day = 24
	if w.Deal(DealTruce) == nil {
		t.Fatal("morning 24: tonight is 25, the truce holds")
	}
	w.Day = 25
	if w.Deal(DealTruce) != nil {
		t.Fatal("morning 25: tonight is 26, the truce is over")
	}
	split := Deal{Kind: DealSplit, Terms: Terms{Corners: []string{"home", "docks"}}}
	if !split.Covers("home") || split.Covers("strip") || d.Covers("home") {
		t.Fatal("covers")
	}
	for _, tc := range []struct {
		d    Deal
		want string
	}{
		{d, "a 15-day truce"},
		{Deal{Kind: DealTribute, Terms: Terms{PerDay: 1200}}, "tribute of $1,200 a day"},
		{split, "a split: 2 corners your side of the line"},
	} {
		if got := tc.d.String(); got != tc.want {
			t.Fatalf("%q, want %q", got, tc.want)
		}
	}
	if got := w.Side(split); got != "Home, Docks" {
		t.Errorf("Side(split) = %q", got)
	}
	if got := w.Describe(split); got != "a split: yours Home, Docks" {
		t.Fatalf("%q", got)
	}
}

// The three lines a split can run along grow from what you hold to the
// whole free city, and never take in a rival corner; while a split
// holds, nobody is posted past it, and walking off a corner is noted.
func TestSplitLinesAndTheLine(t *testing.T) {
	w := dealWorld()
	lines := w.SplitLines()
	if len(lines) != 3 || len(lines[0]) != 1 || lines[0][0] != "home" {
		t.Fatalf("lines %v", lines)
	}
	// docks borders home (yours) and strip (theirs): not on the near line.
	if len(lines[1]) != 1 || len(lines[2]) != 2 {
		t.Fatalf("lines %v", lines)
	}
	for _, l := range lines {
		for _, id := range l {
			if id == "strip" {
				t.Fatalf("a rival corner on your side: %v", lines)
			}
		}
	}
	w.Rival.Deals = []Deal{{Kind: DealSplit, Terms: Terms{Corners: []string{"home"}}, Since: 10}}
	w.Crew.Members = []CrewMember{{ID: 1, Name: "Dre", Role: "runner"}}
	if err := w.Post("docks", 1); err == nil {
		t.Fatal("posted past the line")
	}
	if err := w.Post("home", 1); err != nil {
		t.Fatalf("posting on your own side: %v", err)
	}
	if err := w.Abandon("home"); err != nil || len(w.Today.Abandoned) != 1 || w.Today.Abandoned[0] != "home" {
		t.Fatalf("abandon: %v %v", err, w.Today.Abandoned)
	}
}
