package engine_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// TestViewCarriesTheHouses (#581): view 19 carries what Street
// Edition's owned houses read: the price, the rent, the day bought and
// the nights the rent has gone unpaid, as the world has them.
func TestViewCarriesTheHouses(t *testing.T) {
	t.Parallel()
	s, w := freshSession(t)
	w.Houses = append(w.Houses, game.House{ID: "h1", Name: "The Flat", City: w.Player.Location, Capacity: 40, Price: 9000, Rent: 120, Bought: 3, Unpaid: 2, Stock: map[string]int{}})
	v := s.View()
	if len(v.Houses) != 1 {
		t.Fatalf("%d houses in the view", len(v.Houses))
	}
	if h := v.Houses[0]; h.Price != 9000 || h.Rent != 120 || h.Bought != 3 || h.Unpaid != 2 {
		t.Errorf("the house: %+v", h)
	}
}
