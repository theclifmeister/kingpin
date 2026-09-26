package laundering_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
)

// A buyer's order breaks the quiet days as WORDS says it (#537: "no
// buyer's order owed"): one taken and still owed at the night's end
// makes the day loud; one delivered in full before the night's end, or
// an offer nobody took, does not. A playtest's orders taken and handed
// over in a day left the count running, where WORDS read "no ... buyer's
// order" as any order at all.
func TestQuietDaysAndBuyerOrders(t *testing.T) {
	cfg := content.MustLoad()
	s := laundering.New(cfg)
	for _, tc := range []struct {
		name   string
		status game.ContractStatus
		loud   bool
	}{
		{"an order taken and owed", game.ContractAccepted, true},
		{"an order delivered in full", game.ContractDelivered, false},
		{"an offer nobody took", game.ContractOffered, false},
	} {
		w := world(0)
		w.QuietDays = 5
		for _, c := range w.Cities {
			c.Heat = 0
		}
		w.Contracts = []game.Contract{{ID: 1, City: w.Player.Location, Status: tc.status, Units: 10, Due: w.Day + 3, Expires: w.Day + 3}}
		step(w, s)
		if got := w.QuietDays == 0; got != tc.loud {
			t.Errorf("%s: quiet days 5 -> %d, loud %v, want loud %v", tc.name, w.QuietDays, got, tc.loud)
		}
	}
}
