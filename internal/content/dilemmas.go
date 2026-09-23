package content

import "fmt"

// DilemmasConfig mirrors dilemmas.toml: the cards and their pacing.
type DilemmasConfig struct {
	Dilemmas DilemmasTuning `toml:"dilemmas"`
	Cards    []CardConfig   `toml:"card"`
}

// DilemmasTuning paces the cards: none for MinGap days after the last,
// then a rising chance each day until one is certain at MaxGap.
type DilemmasTuning struct {
	MinGap int `toml:"min_gap"`
	MaxGap int `toml:"max_gap"`
}

// CardConfig is one dilemma: when it can come up, what it says and what
// each answer does. Text and choices are text/templates over the slots
// the news sim fills from the world (Name, Role, Corner, Theirs, Rival,
// City, Front, Product, Amount). Amount is the sum the card is about:
// AmountShare of the player's dirty cash (the bag it is paid from), but at
// least Amount.
type CardConfig struct {
	ID          string         `toml:"id"`
	Title       string         `toml:"title"`
	Text        string         `toml:"text"`
	Weight      float64        `toml:"weight"` // relative draw weight; 0 means 1
	Once        bool           `toml:"once"`   // at most once per run
	Hide        bool           `toml:"hide"`   // the drama is the unknown (#358): the choices show no preview
	Amount      int            `toml:"amount"`
	AmountShare float64        `toml:"amount_share"`
	Trigger     CardTrigger    `toml:"trigger"`
	Choices     []ChoiceConfig `toml:"choice"`
}

// CardTrigger is when a card is eligible: every field set must hold. A
// zero value means the field is not checked, so a card needs at least one
// set. Fields that name somebody also fill the card's slots: a member
// (Role, LoyaltyBelow, LoyaltyAbove), a corner of yours (Corners,
// Contested) and the rival corner it borders (Contested), a front (Fronts).
type CardTrigger struct {
	DayMin       int     `toml:"day_min"`
	DayMax       int     `toml:"day_max"`
	HeatMin      float64 `toml:"heat_min"`
	HeatMax      float64 `toml:"heat_max"` // 0 = unchecked
	CashMin      int     `toml:"cash_min"` // dirty plus clean
	StockMin     int     `toml:"stock_min"`
	CrewMin      int     `toml:"crew_min"`
	Role         string  `toml:"role"`          // a member of this role is on the payroll
	LoyaltyBelow float64 `toml:"loyalty_below"` // a member (of Role, if set) under this loyalty
	LoyaltyAbove float64 `toml:"loyalty_above"`
	Corners      int     `toml:"corners"` // worked corners
	Contested    bool    `toml:"contested"`
	Rival        bool    `toml:"rival"` // the rival holds ground
	Personality  string  `toml:"personality"`
	WarMin       float64 `toml:"war_min"`
	Fronts       bool    `toml:"fronts"`
	PeakCashMin  int     `toml:"peak_cash_min"`  // Stats.PeakCash, the high-water mark; cash_min is today's pile (#147)
	CitiesHeld   int     `toml:"cities_held"`    // cities with a held corner, the lieutenant gate's count (#147)
	City         string  `toml:"city"`           // the city the card or incident is about (#44): it must exist, and it fills the City slot
	DAStance     string  `toml:"da_stance"`      // the sitting DA's ticket (#44)
	PeakCleanMin int     `toml:"peak_clean_min"` // Stats.PeakClean, the clean high-water mark the assets unlock on (#48)
}

// Set reports whether the trigger checks anything at all.
func (t CardTrigger) Set() bool { return t != CardTrigger{} }

// ChoiceConfig is one answer: its label, what happened (for the journal),
// an optional follow-up headline the next morning, and the effects as
// deltas keyed by name. The world owns the list of legal keys
// (game.EffectKeys); the news sim refuses a card that uses any other.
type ChoiceConfig struct {
	Label    string             `toml:"label"`
	Outcome  string             `toml:"outcome"`
	Headline string             `toml:"headline"`
	Effects  map[string]float64 `toml:"effects"`
}

// Card returns the card with id, or nil.
func (d DilemmasConfig) Card(id string) *CardConfig {
	return find(d.Cards, func(e *CardConfig) bool { return e.ID == id })
}

// validate checks the deck reads as a deck: ids unique, every card with a
// title, a trigger and two or three choices, and the pacing sane.
func (d DilemmasConfig) validate() error {
	if d.Dilemmas.MinGap < 1 || d.Dilemmas.MaxGap < d.Dilemmas.MinGap {
		return fmt.Errorf("min_gap %d and max_gap %d must be 1 <= min <= max", d.Dilemmas.MinGap, d.Dilemmas.MaxGap)
	}
	seen := map[string]bool{}
	for _, c := range d.Cards {
		if c.ID == "" || c.Title == "" || c.Text == "" {
			return fmt.Errorf("card %q needs an id, a title and text", c.ID)
		}
		if seen[c.ID] {
			return fmt.Errorf("card %q is defined twice", c.ID)
		}
		seen[c.ID] = true
		if !c.Trigger.Set() {
			return fmt.Errorf("card %q has no trigger", c.ID)
		}
		if n := len(c.Choices); n < 2 || n > 3 {
			return fmt.Errorf("card %q has %d choices; want 2 or 3", c.ID, n)
		}
		for i, ch := range c.Choices {
			if ch.Label == "" || ch.Outcome == "" {
				return fmt.Errorf("card %q choice %d needs a label and an outcome", c.ID, i)
			}
		}
	}
	return nil
}
