package engine_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestChoiceChipsSayTheLines (#358): a choice's chips give its figures,
// and where a move crosses a line the world knows they say so in words:
// the heat over the sting line, a member over or under the skim line, a
// grudge. A card with hide shows "costs you something" and nothing
// else; a choice that moves nothing says so; the view carries the same
// chips.
func TestChoiceChipsSayTheLines(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	s, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := s.NewRun(7, game.Start{})
	var sting float64
	for _, r := range s.Rules().Heat.ThresholdsIn(w, w.Here()) {
		if r.Level == content.Sting {
			sting = r.Threshold
		}
	}
	skim := s.Rules().Crew.Tuning().SkimThreshold
	w.Here().Heat = sting - 2
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 901, Name: "Vee", Role: "runner", Loyalty: skim + 2})
	w.Player.DirtyCash = 20_000
	c := &game.Card{ID: "test", Member: 901, Amount: 12_000, Choices: []game.Choice{
		{Label: "Pay", Effects: map[string]float64{"dirty_amount": -1, "loyalty": 8, "respect": 5}},
		{Label: "Push", Effects: map[string]float64{"heat": 6, "loyalty": -5, "grudge": 20}},
		{Label: "Walk"},
	}}
	texts := func(cs []engine.Chip) string {
		var out []string
		for _, c := range cs {
			out = append(out, c.Text)
		}
		return strings.Join(out, " · ")
	}
	n := func(v float64) string { return fmt.Sprintf("%.0f", v) }
	rival := w.FactionName("")
	got := engine.ChoiceChips(cfg, s.Rules(), w, c)
	for i, want := range []string{
		"dirty −$12K · Vee's loyalty +8 · respect +5",
		"heat " + n(sting-2) + " → " + n(sting+4) + ": over the sting line · Vee's loyalty " + n(skim+2) + " → " + n(skim-3) + ": under the skim line · grudge +20: " + rival + " remember",
		"changes nothing",
	} {
		if texts(got[i]) != want {
			t.Errorf("choice %d: %q, want %q", i, texts(got[i]), want)
		}
	}
	for i, want := range []string{engine.ToneCost, engine.ToneLine, engine.ToneNote} {
		if got[i][0].Tone != want {
			t.Errorf("choice %d's first chip is %q, want %q", i, got[i][0].Tone, want)
		}
	}

	w.Dilemmas.Pending = c
	v := s.View()
	if len(v.Card.Choices) != 3 || v.Card.Choices[1].Label != "Push" || texts(v.Card.Choices[1].Preview) != texts(got[1]) {
		t.Errorf("the view's card is %+v", v.Card)
	}

	// The rich band's keys (#342): the card's corner given up, a favour.
	var held *game.Corner
	for _, k := range w.Corners() {
		if k.Owner == game.OwnerPlayer {
			held = &k
			break
		}
	}
	if held == nil {
		t.Fatal("no corner held on day 0")
	}
	deal := &game.Card{ID: "deal", Corner: held.ID, Choices: []game.Choice{{Label: "Sell", Effects: map[string]float64{"corner": -1, "owes": 1}}}}
	if got := texts(engine.ChoiceChips(cfg, s.Rules(), w, deal)[0]); got != "give up "+held.Name+" · you'll owe a favour" {
		t.Errorf("the corner and the favour read %q", got)
	}

	// A move too small to print is no chip (#465: a war at 0.03 clamped
	// to zero read `war with Slick Eddie's crew −0.0`).
	w.Rival().War = 0.03
	calm := &game.Card{ID: "calm", Choices: []game.Choice{{Label: "Calm", Effects: map[string]float64{"war": -10, "respect": 2}}}}
	if got := texts(engine.ChoiceChips(cfg, s.Rules(), w, calm)[0]); got != "respect +2" {
		t.Errorf("a war already at zero reads %q", got)
	}

	c.Hide = true
	for i, cs := range engine.ChoiceChips(cfg, s.Rules(), w, c) {
		if len(cs) != 1 || cs[0].Text != engine.Hidden {
			t.Errorf("hidden choice %d shows %+v", i, cs)
		}
	}
}

// A crew loyalty chip reads as its text does (#384): one member down 25
// while the rest go up 2 says "down to −23" and is a cost, not a gain.
func TestMixedCrewLoyaltyIsACost(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	s, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := s.NewRun(7, game.Start{})
	w.Crew.Members = []game.CrewMember{{ID: 901, Name: "Vee", Role: "runner", Loyalty: 90}, {ID: 902, Name: "Dre", Role: "runner", Loyalty: 90}}
	c := &game.Card{ID: "a_say", Member: 901, Choices: []game.Choice{
		{Label: "Remind", Effects: map[string]float64{"loyalty": -25, "crew_loyalty": 2}},
		{Label: "Give", Effects: map[string]float64{"loyalty": 5, "crew_loyalty": 2}},
	}}
	got := engine.ChoiceChips(cfg, s.Rules(), w, c)
	if ch := got[0][0]; ch.Text != "crew loyalty down as much as 23" || ch.Tone != engine.ToneCost {
		t.Errorf("the mixed move reads %+v", ch)
	}
	if ch := got[1][0]; !strings.HasPrefix(ch.Text, "crew loyalty up as much as ") || ch.Tone != engine.ToneGain {
		t.Errorf("the rise reads %+v", ch)
	}
}
