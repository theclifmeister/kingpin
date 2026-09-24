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
