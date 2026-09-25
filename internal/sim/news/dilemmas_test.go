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
	bad := *cfg
	bad.Dilemmas.Cards = []content.CardConfig{{ID: "x", Title: "x", Text: "x", Trigger: content.CardTrigger{CrewMin: 1}, Choices: []content.ChoiceConfig{
		{Label: "a", Outcome: "a", Effects: map[string]float64{"heat": 1}},
		{Label: "b", Outcome: "b", Effects: map[string]float64{"evidnce": 1}},
	}}}
	if _, err := news.New(&bad); err == nil || !strings.Contains(err.Error(), `unknown effect "evidnce"`) {
		t.Fatalf("err = %v", err)
	}
	// And a loyalty effect on a card that names nobody.
	bad.Dilemmas.Cards[0].Choices[1].Effects = map[string]float64{"loyalty": 1}
	if _, err := news.New(&bad); err == nil || !strings.Contains(err.Error(), "names a member") {
		t.Fatalf("err = %v", err)
	}
}

// Every slot a card's templates use is one its trigger fills (#349): the
// shipped deck constructs, and a card naming a slot nobody fills is a
// construction error wherever the slot sits, not a hole in the text.
func TestEverySlotIsFilledByItsTrigger(t *testing.T) {
	cfg := content.MustLoad()
	if _, err := news.New(cfg); err != nil {
		t.Fatalf("shipped deck: %v", err)
	}
	card := func(tr content.CardTrigger) content.CardConfig {
		return content.CardConfig{ID: "x", Title: "x", Text: "x", Trigger: tr, Choices: []content.ChoiceConfig{
			{Label: "a", Outcome: "a"}, {Label: "b", Outcome: "b"},
		}}
	}
	rows := []struct {
		name   string
		slot   string
		place  func(c *content.CardConfig, src string)
		refuse content.CardTrigger // fills nothing the slot needs
		allow  content.CardTrigger // fills it
	}{
		{"name in text", "Name", func(c *content.CardConfig, s string) { c.Text = s }, content.CardTrigger{CrewMin: 2}, content.CardTrigger{CrewMin: 2, LoyaltyAbove: 1}},
		{"role in title", "Role", func(c *content.CardConfig, s string) { c.Title = s }, content.CardTrigger{CrewMin: 1}, content.CardTrigger{Role: "enforcer"}},
		{"corner in label", "Corner", func(c *content.CardConfig, s string) { c.Choices[0].Label = s }, content.CardTrigger{Rival: true}, content.CardTrigger{Corners: 1}},
		{"theirs in outcome", "Theirs", func(c *content.CardConfig, s string) { c.Choices[1].Outcome = s }, content.CardTrigger{Corners: 1}, content.CardTrigger{Contested: true}},
		{"front in follow-up", "Front", func(c *content.CardConfig, s string) { c.Choices[0].Headline = s }, content.CardTrigger{CashMin: 1}, content.CardTrigger{Fronts: true}},
		{"name inside an if", "Name", func(c *content.CardConfig, s string) { c.Text = "{{if .Amount}}" + s + "{{end}}" }, content.CardTrigger{CrewMin: 2}, content.CardTrigger{LoyaltyBelow: 50}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			bad := *cfg
			c := card(r.refuse)
			r.place(&c, "{{."+r.slot+"}} again")
			bad.Dilemmas.Cards = []content.CardConfig{c}
			if _, err := news.New(&bad); err == nil || !strings.Contains(err.Error(), "{{."+r.slot+"}}") {
				t.Fatalf("refused trigger: err = %v", err)
			}
			bad.Dilemmas.Cards[0].Trigger = r.allow
			if _, err := news.New(&bad); err != nil {
				t.Fatalf("filling trigger: %v", err)
			}
		})
	}
	// The slots every card fills need no trigger.
	ok := *cfg
	c := card(content.CardTrigger{DayMin: 1})
	c.Text = "{{.City}} {{.Rival}} {{.Product}} {{.Amount}}"
	ok.Dilemmas.Cards = []content.CardConfig{c}
	if _, err := news.New(&ok); err != nil {
		t.Fatalf("always-filled slots: %v", err)
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
		slot    func(s game.CardSlots) bool
	}{
		{"day_min", content.CardTrigger{DayMin: 30}, func(w *game.World) { w.Day = 29 }, func(w *game.World) { w.Day = 30 }, nil},
		{"day_max", content.CardTrigger{DayMax: 30}, func(w *game.World) { w.Day = 31 }, func(w *game.World) { w.Day = 30 }, nil},
		{"heat_min", content.CardTrigger{HeatMin: 40}, func(w *game.World) { w.Home().Heat = 39 }, func(w *game.World) { w.Home().Heat = 40 }, nil},
		{"heat_max", content.CardTrigger{HeatMax: 40}, func(w *game.World) { w.Home().Heat = 41 }, func(w *game.World) { w.Home().Heat = 40 }, nil},
		{"cash_min", content.CardTrigger{CashMin: 5000}, func(w *game.World) { w.Player.DirtyCash, w.Player.CleanCash = 4000, 999 }, func(w *game.World) { w.Player.CleanCash = 1000 },
			func(s game.CardSlots) bool { return s.Amount != "" }},
		{"stock_min", content.CardTrigger{StockMin: 30}, func(w *game.World) { w.SetStock(w.Home().ID, w.Products[0], 29) }, func(w *game.World) { w.SetStock(w.Home().ID, w.Products[1], 1) },
			func(s game.CardSlots) bool { return s.Product != "" }},
		{"crew_min", content.CardTrigger{CrewMin: 2}, func(w *game.World) { w.Crew.Members = w.Crew.Members[:1] }, func(w *game.World) {
			w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 9, Name: "Boo", Role: "runner", Loyalty: 50})
		}, nil},
		{"role", content.CardTrigger{Role: "enforcer"}, func(w *game.World) { w.Crew.Members[0].Role = "runner" }, func(w *game.World) { w.Crew.Members[0].Role = "enforcer" },
			func(s game.CardSlots) bool { return s.Name == "Dre" && s.Role == "enforcer" }},
		{"loyalty_below", content.CardTrigger{LoyaltyBelow: 40}, func(w *game.World) { w.Crew.Members[0].Loyalty = 40 },
			func(w *game.World) {
				w.Crew.Members[0].Loyalty = 39
				w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 9, Name: "Boo", Role: "runner", Loyalty: 20})
			},
			func(s game.CardSlots) bool { return s.Name == "Boo" }}, // the least loyal
		{"loyalty_above", content.CardTrigger{LoyaltyAbove: 60}, func(w *game.World) { w.Crew.Members[0].Loyalty = 60 }, func(w *game.World) { w.Crew.Members[0].Loyalty = 61 },
			func(s game.CardSlots) bool { return s.Name == "Dre" }},
		{"corners", content.CardTrigger{Corners: 2}, func(w *game.World) {}, func(w *game.World) { _ = w.Post(w.Home().Corners[1].ID, 1) },
			func(s game.CardSlots) bool { return s.Corner != "" }},
		{"contested", content.CardTrigger{Contested: true}, func(w *game.World) { w.Home().Corners[1].Owner = game.OwnerNone },
			func(w *game.World) {
				// The rival takes the corner next to yours.
				mine := w.PostOf(game.You)
				for i := range w.Home().Corners {
					if w.Home().Corners[i].Borders(*mine) {
						w.Home().Corners[i].Owner = game.OwnerRival
						w.Rival().Arrived = 1
						return
					}
				}
				t.Fatal("no corner borders yours")
			},
			func(s game.CardSlots) bool { return s.Corner != "" && s.Theirs != "" && s.Corner != s.Theirs }},
		{"rival", content.CardTrigger{Rival: true}, func(w *game.World) {}, func(w *game.World) { w.Home().Corners[1].Owner = game.OwnerRival; w.Rival().Arrived = 1 },
			func(s game.CardSlots) bool { return s.Rival != "" }},
		{"personality", content.CardTrigger{Personality: "chaotic"}, func(w *game.World) { w.Home().Corners[1].Owner = game.OwnerRival; w.Rival().Personality = "defensive" },
			func(w *game.World) { w.Rival().Personality = "chaotic" }, nil},
		{"war_min", content.CardTrigger{WarMin: 30}, func(w *game.World) { w.Home().Corners[1].Owner = game.OwnerRival; w.Rival().War = 29 }, func(w *game.World) { w.Rival().War = 30 }, nil},
		{"fronts", content.CardTrigger{Fronts: true}, func(w *game.World) {}, func(w *game.World) { w.Fronts = []game.Front{{ID: "laundromat", Name: "Suds"}} },
			func(s game.CardSlots) bool { return s.Front == "Suds" }},
		// The progression's two (#147): the peak is the high-water mark,
		// not today's pile, and the cities are those with a held corner.
		{"peak_cash_min", content.CardTrigger{PeakCashMin: 25_000}, func(w *game.World) { w.Player.DirtyCash, w.Stats.PeakCash = 30_000, 24_999 },
			func(w *game.World) { w.Player.DirtyCash, w.Stats.PeakCash = 100, 25_000 }, nil},
		{"cities_held", content.CardTrigger{CitiesHeld: 2}, func(w *game.World) {},
			func(w *game.World) { w.Cities[w.CityOrder[1]].Corners[0].Owner = game.OwnerPlayer }, nil},
		// A favour owed (#342), what a card's owes effect runs up.
		{"owes_min", content.CardTrigger{OwesMin: 1}, func(w *game.World) {}, func(w *game.World) { w.Dilemmas.Owes = 1 }, nil},
	}
	for _, row := range rows {
		w := base()
		// One runner on the payroll and no rival, the default a trigger
		// has to be false against or true after verify.
		w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Dre", Role: "runner", Loyalty: 50}}
		w.Player.DirtyCash = 1000
		row.falsify(w)
		card := content.CardConfig{ID: row.name, Trigger: row.trigger}
		if _, ok := game.Eligible(w, card); ok {
			t.Errorf("%s: eligible while the trigger is false", row.name)
		}
		row.verify(w)
		s, ok := game.Eligible(w, card)
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
	s, err := news.New(cfg)
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
	once := *cfg
	once.Dilemmas.Cards = []content.CardConfig{{ID: "one", Title: "One", Text: "Once.", Once: true, Trigger: content.CardTrigger{CrewMin: 1},
		Choices: []content.ChoiceConfig{{Label: "a", Outcome: "a"}, {Label: "b", Outcome: "b"}}}}
	s1, err := news.New(&once)
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

	// A once_per card comes up once a subject (#466): the funeral buries
	// each member's brother once, then the deck is empty.
	per := *cfg
	per.Dilemmas.Cards = []content.CardConfig{{ID: "funeral", Title: "Funeral", Text: "{{.Name}}'s brother.", OncePer: content.OncePerMember, Trigger: content.CardTrigger{LoyaltyAbove: 1},
		Choices: []content.ChoiceConfig{{Label: "a", Outcome: "a"}, {Label: "b", Outcome: "b"}}}}
	s2, err := news.New(&per)
	if err != nil {
		t.Fatal(err)
	}
	w = sim.NewWorld(cfg, 5)
	w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Dre", Role: "runner", Loyalty: 50}, {ID: 2, Name: "Yaya", Role: "runner", Loyalty: 70}}
	var named []string
	for d := 1; d <= 100; d++ {
		s2.Step(w, &game.Tick{Day: d, RNG: game.RNGFor(w.Seed, d)})
		w.Day = d
		if p := w.Dilemmas.Pending; p != nil {
			named = append(named, p.Text)
			_, _ = w.Choose(0)
		}
	}
	if len(named) != 2 || named[0] != "Yaya's brother." || named[1] != "Dre's brother." {
		t.Fatalf("the funerals: %q", named)
	}
}

// A personal card is the size of the thing it is about (#342): every one
// in the deck, rendered against a world with $100M dirty, names a sum no
// bigger than its amount_max, and a business card still grows with the
// bag.
func TestPersonalStakesAreCapped(t *testing.T) {
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, 1)
	w.Player.DirtyCash, w.Player.CleanCash = 100_000_000, 100_000_000 // clean too: a card paid in clean needs it (#384)
	personal := 0
	for _, c := range cfg.Dilemmas.Cards {
		c.Trigger = content.CardTrigger{} // the sum, not the trigger, is under test
		s, ok := game.Eligible(w, c)
		if !ok {
			t.Fatalf("card %s: an empty trigger does not hold", c.ID)
		}
		if c.AmountMax > 0 && s.Sum > c.AmountMax {
			t.Errorf("card %s names %s at $100M dirty; its cap is $%d", c.ID, s.Amount, c.AmountMax)
		}
		if c.Stakes == content.StakesPersonal {
			personal++
		}
		if c.ID == "stash_hit" && s.Sum < 10_000_000 {
			t.Errorf("stash_hit, a business card, names %s at $100M dirty", s.Amount)
		}
	}
	if personal < 10 {
		t.Fatalf("only %d personal cards", personal)
	}
}

// cashKeys are the keys that only move money.
var cashKeys = map[string]bool{"dirty_cash": true, "clean_cash": true, "dirty_amount": true, "clean_amount": true}

// The rich band (#342), the cards gated on the peak or weighted up past
// the rich tier, costs more than money: every card in it has a choice
// that moves something other than cash, and there are enough of them to
// be most of a rich player's deck.
func TestRichCardsMoveMoreThanCash(t *testing.T) {
	cfg := content.MustLoad()
	if cfg.Dilemmas.Dilemmas.RichTier < 2 || cfg.Dilemmas.Dilemmas.RichTier > len(cfg.Progression.Tiers) {
		t.Fatalf("rich_tier %d is not a tier past the first", cfg.Dilemmas.Dilemmas.RichTier)
	}
	band := 0
	for _, c := range cfg.Dilemmas.Cards {
		if c.Trigger.PeakCashMin == 0 && c.WeightRich <= max(c.Weight, 1) {
			continue
		}
		band++
		moves := false
		for _, ch := range c.Choices {
			for k := range ch.Effects {
				moves = moves || !cashKeys[k]
			}
		}
		if !moves {
			t.Errorf("card %s is in the rich band and only moves money", c.ID)
		}
	}
	if band < 6 {
		t.Fatalf("only %d cards in the rich band", band)
	}
}

// Past the rich tier the deck leans to the band (#342): the same rich
// world, dealt a year of cards, draws mostly the band once the tier is
// reached, and far fewer of them with the lean switched off.
func TestRichDeckLeansToTheBand(t *testing.T) {
	cfg := content.MustLoad()
	band := map[string]bool{}
	for _, c := range cfg.Dilemmas.Cards {
		if c.WeightRich > max(c.Weight, 1) {
			band[c.ID] = true
		}
	}
	share := func(richTier int) float64 {
		deal := *cfg
		deal.Dilemmas.Dilemmas.RichTier = richTier
		s, err := news.New(&deal)
		if err != nil {
			t.Fatal(err)
		}
		w := sim.NewWorld(cfg, 5)
		w.Player.DirtyCash, w.Player.CleanCash, w.Stats.PeakCash = 5_000_000, 1_000_000, 5_000_000 // a rich player launders (#384)
		w.Reach(cfg.Dilemmas.Dilemmas.RichTier, 1)
		w.Home().Heat = 30
		w.Fronts = []game.Front{{ID: "laundromat", Name: "Suds"}}
		w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Dre", Role: "runner", Loyalty: 60}, {ID: 2, Name: "Tank", Role: "enforcer", Loyalty: 70}, {ID: 3, Name: "Boo", Role: "runner", Loyalty: 40}}
		// The rival on the corner next to yours, so the rival cards are in.
		mine := w.PostOf(game.You)
		for i := range w.Home().Corners {
			if c := &w.Home().Corners[i]; c.Owner == game.OwnerNone && c.Borders(*mine) {
				c.Owner = game.OwnerRival
				break
			}
		}
		w.Rival().Arrived = 1
		dealt, rich := 0, 0
		for d := 1; d <= 365; d++ {
			if p := w.Dilemmas.Pending; p != nil {
				if _, err := w.Choose(len(p.Choices) - 1); err != nil {
					t.Fatal(err)
				}
			}
			s.Step(w, &game.Tick{Day: d, RNG: game.RNGFor(w.Seed, d)})
			w.Day = d
			if p := w.Dilemmas.Pending; p != nil && p.Day == d {
				dealt++
				if band[p.ID] {
					rich++
				}
			}
		}
		if dealt < 30 {
			t.Fatalf("only %d cards in a year", dealt)
		}
		return float64(rich) / float64(dealt)
	}
	lean, flat := share(cfg.Dilemmas.Dilemmas.RichTier), share(0)
	t.Logf("the band's share of a rich year: %.2f leaning, %.2f flat", lean, flat)
	if lean <= 0.5 || lean <= flat {
		t.Fatalf("the band is %.2f of a rich player's cards (%.2f without the lean); want most", lean, flat)
	}
}

// corner gives up the corner a trigger names, and only gives it up, and
// never on the first choice, the one a stray 1 or ↓ picks: a card that names no
// corner, would gain one or puts it first fails at start-up.
func TestDeckRefusesAStrayCorner(t *testing.T) {
	cfg := content.MustLoad()
	card := func(tr content.CardTrigger, first, second map[string]float64) *content.Config {
		bad := *cfg
		bad.Dilemmas.Cards = []content.CardConfig{{ID: "x", Title: "x", Text: "x", Trigger: tr, Choices: []content.ChoiceConfig{
			{Label: "a", Outcome: "a", Effects: first}, {Label: "b", Outcome: "b", Effects: second},
		}}}
		return &bad
	}
	give := map[string]float64{"corner": -1}
	for name, bad := range map[string]*content.Config{
		"no corner named":  card(content.CardTrigger{CrewMin: 1}, nil, give),
		"a corner gained":  card(content.CardTrigger{Corners: 1}, nil, map[string]float64{"corner": 1}),
		"the first choice": card(content.CardTrigger{Corners: 1}, give, nil),
	} {
		if _, err := news.New(bad); err == nil || !strings.Contains(err.Error(), "corner") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := news.New(card(content.CardTrigger{Contested: true}, nil, give)); err != nil {
		t.Fatalf("a named corner, given up second: %v", err)
	}
}
