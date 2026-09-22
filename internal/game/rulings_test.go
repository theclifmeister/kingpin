package game

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// A hand-over starts the holder's fields again and leaves your
// stick-ups on the corner to the territory sim's clock (#281).
func TestHandResetsTheHolderNotTheStickUps(t *testing.T) {
	c := Corner{Owner: OwnerPlayer, Runner: 3, Enforcer: 4, Idle: 2, Squeeze: 0.4, Since: 5, Robbed: 2, Starved: 3, StarvedDay: 9, Yours: true}
	c.Hand(OwnerRival, "sal", 12)
	if c.Owner != OwnerRival || c.Faction != "sal" || c.Runner != 0 || c.Enforcer != 0 || c.Idle != 0 || c.Squeeze != 0 ||
		c.Since != 12 || c.Starved != 0 || c.StarvedDay != 0 || c.Robbed != 2 || !c.Yours {
		t.Fatalf("after the hand-over: %+v", c)
	}
	c.Hand(OwnerNone, "", 20)
	if c.Owner != OwnerNone || c.Faction != "" || c.Since != 20 {
		t.Fatalf("back to the street: %+v", c)
	}
}

// The pay and launder dials refuse a position off the dial and a run
// that is over, as the route dial does (#281); the dial stays where it was.
func TestDialSettersRefuseWhatSetRouteRefuses(t *testing.T) {
	w := testWorld()
	if err := w.SetPay(events.PayGenerous); err != nil || w.Crew.Pay != events.PayGenerous {
		t.Fatalf("generous: %v, pay %v", err, w.Crew.Pay)
	}
	if err := w.SetLaunderDial(events.LaunderGreedy); err != nil || w.Laundering.Dial != events.LaunderGreedy {
		t.Fatalf("greedy: %v, dial %v", err, w.Laundering.Dial)
	}
	if err := w.SetPay(events.PayGenerous + 1); err != ErrBadDial || w.Crew.Pay != events.PayGenerous {
		t.Fatalf("off the pay dial: %v, pay %v", err, w.Crew.Pay)
	}
	if err := w.SetPay(-1); err != ErrBadDial {
		t.Fatalf("under the pay dial: %v", err)
	}
	if err := w.SetLaunderDial(events.LaunderGreedy + 1); err != ErrBadDial || w.Laundering.Dial != events.LaunderGreedy {
		t.Fatalf("off the launder dial: %v, dial %v", err, w.Laundering.Dial)
	}
	w.Over = &Ending{Cause: "broke"}
	if err := w.SetPay(events.PayFair); err != ErrGameOver || w.Crew.Pay != events.PayGenerous {
		t.Fatalf("pay after the end: %v", err)
	}
	if err := w.SetLaunderDial(events.LaunderNormal); err != ErrGameOver || w.Laundering.Dial != events.LaunderGreedy {
		t.Fatalf("launder after the end: %v", err)
	}
}
