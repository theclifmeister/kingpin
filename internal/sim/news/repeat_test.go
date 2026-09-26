package news_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// A card is not dealt again within repeat_gap days of the last time (#501:
// a gala six days after the last, a petition, a wedding five days apart),
// over a long seeded run of a rich player: the gap holds for every card,
// the run still deals, and without the gap the same run deals a card
// again inside it, so the gap is what holds it. Peace has a price says
// "the shooting stops", so it is dealt only in a war.
func TestCardsDoNotRepeatWithinTheGap(t *testing.T) {
	cfg := content.MustLoad()
	gap := cfg.Dilemmas.Dilemmas.RepeatGap
	if gap < 14 {
		t.Fatalf("repeat_gap is %d days; a card comes back inside two weeks", gap)
	}
	deal := func(gap int) (repeats, dealt, peace int) {
		c := *cfg
		c.Dilemmas.Dilemmas.RepeatGap = gap
		s, err := news.New(&c)
		if err != nil {
			t.Fatal(err)
		}
		w := sim.NewWorld(cfg, 11)
		w.Player.DirtyCash, w.Player.CleanCash, w.Stats.PeakCash = 5_000_000, 1_000_000, 5_000_000
		w.Reach(cfg.Dilemmas.Dilemmas.RichTier, 1)
		w.Home().Heat = 30
		w.Fronts = []game.Front{{ID: "laundromat", Name: "Suds"}}
		w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Dre", Role: "runner", Loyalty: 60}, {ID: 2, Name: "Tank", Role: "enforcer", Loyalty: 70}, {ID: 3, Name: "Boo", Role: "runner", Loyalty: 40}}
		mine := w.PostOf(game.You)
		for i := range w.Home().Corners {
			if k := &w.Home().Corners[i]; k.Owner == game.OwnerNone && k.Borders(*mine) {
				k.Owner = game.OwnerRival
				break
			}
		}
		w.Rival().Arrived = 1
		last := map[string]int{}
		for d := 1; d <= 720; d++ {
			if p := w.Dilemmas.Pending; p != nil {
				if _, err := w.Choose(len(p.Choices) - 1); err != nil {
					t.Fatal(err)
				}
			}
			// A war for the second year, so the peace card has shooting to stop.
			w.Rival().War = 0
			if d > 360 {
				w.Rival().War = 50
			}
			s.Step(w, &game.Tick{Day: d, RNG: game.RNGFor(w.Seed, d)})
			w.Day = d
			p := w.Dilemmas.Pending
			if p == nil || p.Day != d {
				continue
			}
			dealt++
			if l, ok := last[p.ID]; ok && d-l < cfg.Dilemmas.Dilemmas.RepeatGap {
				repeats++
				if gap > 0 {
					t.Errorf("%s dealt on day %d and again on day %d", p.ID, l, d)
				}
			}
			last[p.ID] = d
			if p.ID == "peace_for_a_corner" {
				peace++
				if w.Rival().War < 20 {
					t.Errorf("peace has a price dealt on day %d with the war at %.0f", d, w.Rival().War)
				}
			}
		}
		return repeats, dealt, peace
	}
	_, dealt, peace := deal(gap)
	if dealt < 60 {
		t.Fatalf("only %d cards in two years", dealt)
	}
	if peace == 0 {
		t.Error("peace has a price was never dealt in a year of war")
	}
	if repeats, _, _ := deal(0); repeats == 0 {
		t.Error("with no gap the run never repeats a card inside it: the test pins nothing")
	}
}
