package ui

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// TestHandoffOutlastsTheLieLowNight (#524): a handoff queued before
// lying low stays queued through the lie-low night and goes the first
// night you deal, whichever key ended the lie-low day: n, enter (then
// y) or F.
func TestHandoffOutlastsTheLieLowNight(t *testing.T) {
	for _, tc := range []struct {
		name string
		end  []string
	}{
		{"n", []string{"n"}},
		{"enter", []string{"enter", "enter"}},
		{"F", []string{"F", "1", "enter"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t, 120, 40)
			w := m.w
			home, weed := w.Player.Location, w.Products[0]
			w.SetStock(home, weed, 40)
			c := w.OfferContract(game.Contract{Buyer: "x", Name: "Vera", City: home, Product: weed, Units: 30, Premium: 1.5, HeatMul: 0.4, Expires: w.Day + 2, Due: w.Day + 5})
			if err := w.AcceptContract(c.ID); err != nil {
				t.Fatal(err)
			}
			if err := w.Deliver(c.ID, 25); err != nil {
				t.Fatal(err)
			}
			m.Update(key("1"))
			m.Update(key("l"))
			m.Update(key("y"))
			if !w.Today.LieLow {
				t.Fatal("not lying low")
			}
			day := w.Day
			for _, k := range tc.end {
				m.Update(key(k))
			}
			if w.Day != day+1 {
				t.Fatalf("%s did not end the day: day %d from %d, mode %v", tc.name, w.Day, day, m.mode)
			}
			if got := w.Contract(c.ID); got.Delivered != 0 || w.QueuedDelivery(c.ID) != 25 {
				t.Fatalf("the morning after the lie-low night: delivered %d, queued %d; want none and 25", got.Delivered, w.QueuedDelivery(c.ID))
			}
			for m.mode != modePlay {
				m.Update(key("esc"))
			}
			m.Update(key("n"))
			if got := w.Contract(c.ID); got.Delivered != 25 || w.QueuedDelivery(c.ID) != 0 {
				t.Fatalf("the night after: delivered %d, queued %d; want 25 and none", got.Delivered, w.QueuedDelivery(c.ID))
			}
		})
	}
}
