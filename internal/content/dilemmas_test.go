package content

import (
	"strings"
	"testing"
)

// Every card says what kind of sum it names (#342): the shipped deck
// validates, and a card with no stakes, stakes of another kind, a
// personal sum with no cap or a cap under its floor is refused by name.
func TestEveryCardDeclaresItsStakes(t *testing.T) {
	cfg := MustLoad()
	if err := cfg.Dilemmas.validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range cfg.Dilemmas.Cards {
		if c.Stakes != StakesPersonal && c.Stakes != StakesBusiness {
			t.Errorf("card %s: stakes %q", c.ID, c.Stakes)
		}
	}
	base := cfg.Dilemmas.Cards[0]
	rows := []struct {
		name string
		edit func(c *CardConfig)
		want string
	}{
		{"no stakes", func(c *CardConfig) { c.Stakes = "" }, `stakes ""`},
		{"another kind", func(c *CardConfig) { c.Stakes = "family" }, `stakes "family"`},
		{"a personal sum with no cap", func(c *CardConfig) { c.Stakes, c.Amount, c.AmountMax = StakesPersonal, 250, 0 }, "needs an amount_max"},
		{"a cap under the floor", func(c *CardConfig) { c.Amount, c.AmountMax = 500, 400 }, "under amount"},
		{"a negative rich weight", func(c *CardConfig) { c.WeightRich = -1 }, "negative"},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			d := cfg.Dilemmas
			c := base
			r.edit(&c)
			d.Cards = []CardConfig{c}
			if err := d.validate(); err == nil || !strings.Contains(err.Error(), r.want) {
				t.Fatalf("err = %v, want %q", err, r.want)
			}
		})
	}
	// The lean needs a rest weight to lean with.
	d := cfg.Dilemmas
	d.Dilemmas.RichRest = 0
	if err := d.validate(); err == nil || !strings.Contains(err.Error(), "rich_rest") {
		t.Fatalf("rich_tier with no rich_rest: err = %v", err)
	}
	// A personal card that names no sum needs no cap.
	d = cfg.Dilemmas
	c := base
	c.Stakes, c.Amount, c.AmountShare, c.AmountMax = StakesPersonal, 0, 0, 0
	d.Cards = []CardConfig{c}
	if err := d.validate(); err != nil {
		t.Fatalf("a personal card with no sum: %v", err)
	}
}

// A once_per card is once per a subject its trigger names (#466): the
// shipped one-off cards say so, and a once_per of another kind or one
// its trigger never fills is refused by name.
func TestOncePerNamesASubject(t *testing.T) {
	cfg := MustLoad()
	for id, per := range map[string]string{"funeral": OncePerMember, "permit": OncePerFront, "church_roof": OncePerCorner, "a_say": OncePerMember} {
		if c := cfg.Dilemmas.Card(id); c == nil || c.OncePer != per {
			t.Errorf("card %s: once_per %v, want %q", id, c, per)
		}
	}
	// The card game's outcome is the effect it applies (#466: it paid
	// half the stake and said "walked out up {{.Amount}}").
	if c := cfg.Dilemmas.Card("card_game"); c == nil || c.Choices[0].Effects["dirty_amount"] != 0.5 || !strings.Contains(c.Choices[0].Outcome, "up half of it") {
		t.Errorf("the card game's sit-in: %+v", c)
	}
	// Its stake and its net on the one line (#501: the text said "costs
	// $10,000" over a chip that said dirty +$5,000).
	if c := cfg.Dilemmas.Card("card_game"); c == nil || !strings.Contains(c.Choices[0].Label, "{{.Amount}}") || !strings.Contains(c.Choices[0].Label, "{{.Share 0.5}}") {
		t.Errorf("the card game's sit-in label: %+v", c)
	}
	base := cfg.Dilemmas.Cards[0]
	base.Trigger = CardTrigger{CashMin: 1}
	for _, r := range []struct{ per, want string }{
		{"brother", `once_per "brother"`},
		{OncePerMember, "names no member"},
		{OncePerCorner, "names no corner"},
		{OncePerFront, "names no front"},
	} {
		d := cfg.Dilemmas
		c := base
		c.OncePer = r.per
		d.Cards = []CardConfig{c}
		if err := d.validate(); err == nil || !strings.Contains(err.Error(), r.want) {
			t.Errorf("once_per %q: err = %v, want %q", r.per, err, r.want)
		}
	}
}
