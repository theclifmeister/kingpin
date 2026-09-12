package news_test

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// The deck as shipped: 20+ cards, every one with a trigger and two or
// three choices whose effect keys the world applies, covering crew, rival,
// heat and money, and reading at 80 columns.
func TestDeckIsValid(t *testing.T) {
	cfg := content.MustLoad()
	deck := cfg.Dilemmas.Cards
	if len(deck) < 20 {
		t.Fatalf("only %d cards; the first hour will repeat", len(deck))
	}
	kinds := map[string]int{}
	for _, c := range deck {
		tr := c.Trigger
		if !tr.Set() {
			t.Errorf("card %s has no trigger", c.ID)
		}
		if n := len(c.Choices); n < 2 || n > 3 {
			t.Errorf("card %s has %d choices", c.ID, n)
		}
		if len(c.Text) > 300 {
			t.Errorf("card %s: text is %d characters; it will not fit an 80x24 card", c.ID, len(c.Text))
		}
		for i, ch := range c.Choices {
			for key := range ch.Effects {
				if !game.KnownEffect(key) {
					t.Errorf("card %s choice %d: unknown effect %q", c.ID, i, key)
				}
			}
			if len(ch.Label) > 48 {
				t.Errorf("card %s choice %d: label is %d characters", c.ID, i, len(ch.Label))
			}
			if len(ch.Outcome) > 90 {
				t.Errorf("card %s choice %d: outcome is %d characters; the journal line will be cut", c.ID, i, len(ch.Outcome))
			}
		}
		switch {
		case tr.Rival || tr.Personality != "" || tr.WarMin > 0 || tr.Contested:
			kinds["rival"]++
		case tr.Role != "" || tr.LoyaltyBelow > 0 || tr.LoyaltyAbove > 0 || tr.CrewMin > 0:
			kinds["crew"]++
		case tr.HeatMin > 0:
			kinds["heat"]++
		case tr.CashMin > 0 || tr.Fronts || tr.StockMin > 0:
			kinds["money"]++
		}
	}
	for _, k := range []string{"crew", "rival", "heat", "money"} {
		if kinds[k] < 3 {
			t.Errorf("only %d %s cards", kinds[k], k)
		}
	}
	t.Logf("%d cards: %v", len(deck), kinds)
}

// A typo in an effect key is a construction error, not a silent no-op.
func TestDeckRefusesUnknownEffectKey(t *testing.T) {
	cfg := content.MustLoad()
	bad := cfg.Dilemmas
	bad.Cards = []content.CardConfig{{ID: "x", Title: "x", Text: "x", Trigger: content.CardTrigger{CrewMin: 1}, Choices: []content.ChoiceConfig{
		{Label: "a", Outcome: "a", Effects: map[string]float64{"heat": 1}},
		{Label: "b", Outcome: "b", Effects: map[string]float64{"evidnce": 1}},
	}}}
	if _, err := news.New(cfg.Headlines, bad, cfg.Progression); err == nil || !strings.Contains(err.Error(), `unknown effect "evidnce"`) {
		t.Fatalf("err = %v", err)
	}
	// And a loyalty effect on a card that names nobody.
	bad.Cards[0].Choices[1].Effects = map[string]float64{"loyalty": 1}
	if _, err := news.New(cfg.Headlines, bad, cfg.Progression); err == nil || !strings.Contains(err.Error(), "names a member") {
		t.Fatalf("err = %v", err)
	}
}

// A card is never eligible while its trigger is false, and fills its
// slots from the world when it is true: one row per trigger kind.
func TestTriggersHold(t *testing.T) {
	cfg := content.MustLoad()
	base := func() *game.World {
		w := sim.NewWorld(cfg, 1)
		w.Day = 20
		return w
	}
	rows := []struct {
		name    string
		trigger content.CardTrigger
		falsify func(w *game.World)
		verify  func(w *game.World)
		slot    func(s news.Slots) bool
	}{
		{"day_min", content.CardTrigger{DayMin: 30}, func(w *game.World) { w.Day = 29 }, func(w *game.World) { w.Day = 30 }, nil},
		{"day_max", content.CardTrigger{DayMax: 30}, func(w *game.World) { w.Day = 31 }, func(w *game.World) { w.Day = 30 }, nil},
		{"heat_min", content.CardTrigger{HeatMin: 40}, func(w *game.World) { w.Home().Heat = 39 }, func(w *game.World) { w.Home().Heat = 40 }, nil},
		{"heat_max", content.CardTrigger{HeatMax: 40}, func(w *game.World) { w.Home().Heat = 41 }, func(w *game.World) { w.Home().Heat = 40 }, nil},
		{"cash_min", content.CardTrigger{CashMin: 5000}, func(w *game.World) { w.Player.DirtyCash, w.Player.CleanCash = 4000, 999 }, func(w *game.World) { w.Player.CleanCash = 1000 },
			func(s news.Slots) bool { return s.Amount != "" }},
		{"stock_min", content.CardTrigger{StockMin: 30}, func(w *game.World) { w.SetStock(w.Home().ID, w.Products[0], 29) }, func(w *game.World) { w.SetStock(w.Home().ID, w.Products[1], 1) },
			func(s news.Slots) bool { return s.Product != "" }},
		{"crew_min", content.CardTrigger{CrewMin: 2}, func(w *game.World) { w.Crew.Members = w.Crew.Members[:1] }, func(w *game.World) {
			w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 9, Name: "Boo", Role: "runner", Loyalty: 50})
		}, nil},
		{"role", content.CardTrigger{Role: "enforcer"}, func(w *game.World) { w.Crew.Members[0].Role = "runner" }, func(w *game.World) { w.Crew.Members[0].Role = "enforcer" },
			func(s news.Slots) bool { return s.Name == "Dre" && s.Role == "enforcer" }},
		{"loyalty_below", content.CardTrigger{LoyaltyBelow: 40}, func(w *game.World) { w.Crew.Members[0].Loyalty = 40 },
			func(w *game.World) {
				w.Crew.Members[0].Loyalty = 39
				w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 9, Name: "Boo", Role: "runner", Loyalty: 20})
			},
			func(s news.Slots) bool { return s.Name == "Boo" }}, // the least loyal
		{"loyalty_above", content.CardTrigger{LoyaltyAbove: 60}, func(w *game.World) { w.Crew.Members[0].Loyalty = 60 }, func(w *game.World) { w.Crew.Members[0].Loyalty = 61 },
			func(s news.Slots) bool { return s.Name == "Dre" }},
		{"corners", content.CardTrigger{Corners: 2}, func(w *game.World) {}, func(w *game.World) { _ = w.Post(w.Home().Corners[1].ID, 1) },
			func(s news.Slots) bool { return s.Corner != "" }},
		{"contested", content.CardTrigger{Contested: true}, func(w *game.World) { w.Home().Corners[1].Owner = game.OwnerNone },
			func(w *game.World) {
				// The rival takes the corner next to yours.
				mine := w.PostOf(game.You)
				for i := range w.Home().Corners {
					if w.Home().Corners[i].Borders(*mine) {
						w.Home().Corners[i].Owner = game.OwnerRival
						w.Rival.Arrived = 1
						return
					}
				}
				t.Fatal("no corner borders yours")
			},
			func(s news.Slots) bool { return s.Corner != "" && s.Theirs != "" && s.Corner != s.Theirs }},
		{"rival", content.CardTrigger{Rival: true}, func(w *game.World) {}, func(w *game.World) { w.Home().Corners[1].Owner = game.OwnerRival; w.Rival.Arrived = 1 },
			func(s news.Slots) bool { return s.Rival != "" }},
		{"personality", content.CardTrigger{Personality: "chaotic"}, func(w *game.World) { w.Home().Corners[1].Owner = game.OwnerRival; w.Rival.Personality = "defensive" },
			func(w *game.World) { w.Rival.Personality = "chaotic" }, nil},
		{"war_min", content.CardTrigger{WarMin: 30}, func(w *game.World) { w.Home().Corners[1].Owner = game.OwnerRival; w.Rival.War = 29 }, func(w *game.World) { w.Rival.War = 30 }, nil},
		{"fronts", content.CardTrigger{Fronts: true}, func(w *game.World) {}, func(w *game.World) { w.Fronts = []game.Front{{ID: "laundromat", Name: "Suds"}} },
			func(s news.Slots) bool { return s.Front == "Suds" }},
		// The progression's two (#147): the peak is the high-water mark,
		// not today's pile, and the cities are those with a held corner.
		{"peak_cash_min", content.CardTrigger{PeakCashMin: 25_000}, func(w *game.World) { w.Player.DirtyCash, w.Stats.PeakCash = 30_000, 24_999 },
			func(w *game.World) { w.Player.DirtyCash, w.Stats.PeakCash = 100, 25_000 }, nil},
		{"cities_held", content.CardTrigger{CitiesHeld: 2}, func(w *game.World) {},
			func(w *game.World) { w.Cities[w.CityOrder[1]].Corners[0].Owner = game.OwnerPlayer }, nil},
	}
	for _, row := range rows {
		w := base()
		// One runner on the payroll and no rival, the default a trigger
		// has to be false against or true after verify.
		w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Dre", Role: "runner", Loyalty: 50}}
		w.Player.DirtyCash = 1000
		row.falsify(w)
		card := content.CardConfig{ID: row.name, Trigger: row.trigger}
		if _, ok := news.Eligible(w, card); ok {
			t.Errorf("%s: eligible while the trigger is false", row.name)
		}
		row.verify(w)
		s, ok := news.Eligible(w, card)
		if !ok {
			t.Errorf("%s: not eligible while the trigger is true", row.name)
			continue
		}
		if row.slot != nil && !row.slot(s) {
			t.Errorf("%s: slots not filled: %+v", row.name, s)
		}
	}
}

// Pacing: with the whole deck eligible, cards come no closer than min_gap
// and no further apart than max_gap, one a day at most, never while one
// waits for an answer, and a once card once.
func TestDrawPacing(t *testing.T) {
	cfg := content.MustLoad()
	pace := cfg.Dilemmas.Dilemmas
	s, err := news.New(cfg.Headlines, cfg.Dilemmas, cfg.Progression)
	if err != nil {
		t.Fatal(err)
	}
	w := sim.NewWorld(cfg, 5)
	w.Player.DirtyCash = 100_000
	w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Dre", Role: "runner", Loyalty: 50}, {ID: 2, Name: "Tank", Role: "enforcer", Loyalty: 30}}
	var days []int
	for d := 1; d <= 200; d++ {
		// Morning: answer yesterday's card, except every third one, which
		// the player sits on for a day; a pending card blocks the deck.
		if p := w.Dilemmas.Pending; p != nil && (len(days)%3 != 0 || p.Day < d-1) {
			if _, err := w.Choose(len(p.Choices) - 1); err != nil {
				t.Fatal(err)
			}
		}
		before := w.Dilemmas.Pending
		s.Step(w, &game.Tick{Day: d, RNG: game.RNGFor(w.Seed, d)})
		w.Day = d
		if c := w.Dilemmas.Pending; c != nil && c != before {
			if before != nil {
				t.Fatalf("day %d: a second card dealt over one waiting", d)
			}
			if c.Day != d {
				t.Fatalf("card dated %d drawn on day %d", c.Day, d)
			}
			days = append(days, d)
		}
	}
	if len(days) < 200/(pace.MaxGap+1) {
		t.Fatalf("only %d cards in 200 days", len(days))
	}
	for i := 1; i < len(days); i++ {
		if gap := days[i] - days[i-1]; gap < pace.MinGap || gap > pace.MaxGap {
			t.Errorf("cards on days %d and %d: gap %d outside %d..%d", days[i-1], days[i], gap, pace.MinGap, pace.MaxGap)
		}
	}
	t.Logf("%d cards in 200 days: %v", len(days), days)

	// A once card comes up once, then the deck is empty.
	once := cfg.Dilemmas
	once.Cards = []content.CardConfig{{ID: "one", Title: "One", Text: "Once.", Once: true, Trigger: content.CardTrigger{CrewMin: 1},
		Choices: []content.ChoiceConfig{{Label: "a", Outcome: "a"}, {Label: "b", Outcome: "b"}}}}
	s1, err := news.New(cfg.Headlines, once, cfg.Progression)
	if err != nil {
		t.Fatal(err)
	}
	w = sim.NewWorld(cfg, 5)
	w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Dre", Role: "runner", Loyalty: 50}}
	drawn := 0
	for d := 1; d <= 100; d++ {
		s1.Step(w, &game.Tick{Day: d, RNG: game.RNGFor(w.Seed, d)})
		w.Day = d
		if w.Dilemmas.Pending != nil {
			drawn++
			_, _ = w.Choose(0)
		}
	}
	if drawn != 1 {
		t.Fatalf("once card drawn %d times", drawn)
	}
}
