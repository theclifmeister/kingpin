package engine_test

import (
	"errors"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

func newSession(t *testing.T) (*engine.Session, *game.World) {
	t.Helper()
	s, err := engine.New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	return s, s.NewRun(3, game.Start{})
}

// TestCommandsChargeWhatTheRulesQuote (#297): a command that wraps a
// World method taking a number the sims own reads it off the sims, so
// what a front end shows through Rules is what the command charges.
func TestCommandsChargeWhatTheRulesQuote(t *testing.T) {
	t.Parallel()
	s, w := newSession(t)
	r := s.Rules()
	w.Player.DirtyCash = 1_000_000

	if len(w.Crew.Candidates) == 0 {
		t.Fatal("no candidate to hire")
	}
	if _, err := s.Hire(w.Crew.Candidates[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Investigate(); err != nil {
		t.Fatal(err)
	}
	if got, want := w.Today.Investigation.Cost, r.Crew.InvestigateCost(); got != want {
		t.Errorf("investigation cost %d, the rules quote %d", got, want)
	}

	c := w.Corner(w.PostOf(game.You).ID)
	if c == nil || !c.Held() {
		t.Fatalf("you stand on no corner of yours: %+v", c)
	}
	price := r.Territory.DeedPrice(w, *c)
	if price <= 0 {
		t.Fatalf("no price on %s", c.Name)
	}
	w.Player.CleanCash = price + 1000
	w.Stats.Laundered = 10 * price
	if err := s.BuyDeed(c.ID); err != nil {
		t.Fatal(err)
	}
	if w.Player.CleanCash != 1000 || c.Deed == nil || c.Deed.Price != price {
		t.Errorf("the block: clean cash %d (want 1000), deed %+v at the quoted %d", w.Player.CleanCash, c.Deed, price)
	}

	for d := 0; d < 60 && !w.Rival().Alive() && w.Over == nil; d++ {
		s.EndDay()
	}
	if !w.Rival().Alive() {
		t.Fatal("no rival in sixty days")
	}
	if err := s.ScoutFaction(w.Rival().Faction()); err != nil {
		t.Fatal(err)
	}
	if got, want := w.Today.Scouting.Cost, r.Rivals.ScoutCost(); got != want {
		t.Errorf("scouting cost %d, the rules quote %d", got, want)
	}
}

// TestBuyByIDRefusesWhatIsNotOffered (#297): the buys that take an
// offer take its id, and the engine finds the offer, so a front end can
// buy only what the sims put on offer at their price.
func TestBuyByIDRefusesWhatIsNotOffered(t *testing.T) {
	t.Parallel()
	s, _ := newSession(t)
	if _, err := s.BuyFront("nope"); !errors.Is(err, engine.ErrNoOffer) {
		t.Errorf("BuyFront: %v", err)
	}
	if _, err := s.BuyHouse("nope"); !errors.Is(err, engine.ErrNoOffer) {
		t.Errorf("BuyHouse: %v", err)
	}
	if _, err := s.BuyAsset("nope"); !errors.Is(err, engine.ErrNoOffer) {
		t.Errorf("BuyAsset: %v", err)
	}
	if err := s.Invest("nope", 1); !errors.Is(err, engine.ErrNoOffer) {
		t.Errorf("Invest: %v", err)
	}
	offers := s.HouseOffers()
	if len(offers) == 0 {
		t.Fatal("no house on offer")
	}
	for _, o := range offers {
		if o.ID == "" || o.Price <= 0 {
			t.Errorf("house offer %+v has no id or price", o)
		}
	}
}
