package content

import "fmt"

// DilemmasConfig mirrors dilemmas.toml: the cards and their pacing.
type DilemmasConfig struct {
	Dilemmas DilemmasTuning `toml:"dilemmas"`
	Cards    []CardConfig   `toml:"card"`
}

// DilemmasTuning paces the cards: none for MinGap days after the last,
// then a rising chance each day until one is certain at MaxGap. From
// RichTier on (a progression tier's number; 0 never) a card is drawn at
// its WeightRich, or at RichRest times its weight if it sets none, so a
// rich player's deck leans to the cards that cost standing, ground or a
// favour (#342).
type DilemmasTuning struct {
	MinGap   int     `toml:"min_gap"`
	MaxGap   int     `toml:"max_gap"`
	RichTier int     `toml:"rich_tier"`
	RichRest float64 `toml:"rich_rest"`
}

// The two kinds of stake a card declares (#342). A personal sum is a
// fixed range, amount up to amount_max, however big the bag; a business
// sum is a share of the bag, capped only if amount_max says so.
const (
	StakesPersonal = "personal"
	StakesBusiness = "business"
)

// CardConfig is one dilemma: when it can come up, what it says and what
// each answer does. Text and choices are text/templates over the slots
// the news sim fills from the world (Name, Role, Corner, Theirs, Rival,
// City, Front, Product, Amount). Amount is the sum the card is about:
// AmountShare of the player's dirty cash (the bag it is paid from), but at
// least Amount and, when AmountMax is set, at most AmountMax. Stakes says
// which kind of sum it is (StakesPersonal, StakesBusiness): a personal
// card that names a sum must cap it (#342).
type CardConfig struct {
	ID          string         `toml:"id"`
	Title       string         `toml:"title"`
	Text        string         `toml:"text"`
	Stakes      string         `toml:"stakes"`
	Weight      float64        `toml:"weight"`      // relative draw weight; 0 means 1
	WeightRich  float64        `toml:"weight_rich"` // the weight from the deck's rich_tier on; 0 means Weight
	Once        bool           `toml:"once"`        // at most once per run
	OncePer     string         `toml:"once_per"`    // at most once per run for each OncePer* subject the trigger names (#466): a member's brother is buried once
	Hide        bool           `toml:"hide"`        // the drama is the unknown (#358): the choices show no preview
	Amount      int            `toml:"amount"`
	AmountShare float64        `toml:"amount_share"`
	AmountMax   int            `toml:"amount_max"` // the cap on the sum; 0 none (a business card only)
	Trigger     CardTrigger    `toml:"trigger"`
	Choices     []ChoiceConfig `toml:"choice"`
}

// The subjects a card can be once per (CardConfig.OncePer, #466): the
// member, the corner of yours or the front its trigger names. Such a
// card never names the same one twice in a run; the trigger passes over
// one it has named for the next in line, and the card is not drawn when
// none is left.
const (
	OncePerMember = "member"
	OncePerCorner = "corner"
	OncePerFront  = "front"
)

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
	OwesMin      int     `toml:"owes_min"`       // favours owed (World.Dilemmas.Owes), what a card's owes effect runs up (#342)
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
// title, a trigger, its stakes and two or three choices, and the pacing
// sane.
func (d DilemmasConfig) validate() error {
	if d.Dilemmas.MinGap < 1 || d.Dilemmas.MaxGap < d.Dilemmas.MinGap {
		return fmt.Errorf("min_gap %d and max_gap %d must be 1 <= min <= max", d.Dilemmas.MinGap, d.Dilemmas.MaxGap)
	}
	if d.Dilemmas.RichTier < 0 || d.Dilemmas.RichTier > 0 && (d.Dilemmas.RichRest <= 0 || d.Dilemmas.RichRest > 1) {
		return fmt.Errorf("rich_tier %d and rich_rest %v must be 0, or a tier and 0 < rest <= 1", d.Dilemmas.RichTier, d.Dilemmas.RichRest)
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
		if err := c.validateStakes(); err != nil {
			return err
		}
		if err := c.validateOncePer(); err != nil {
			return err
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

// validateOncePer checks a card's once_per (#466) names a subject its
// trigger fills: a member for member, a corner for corner, a front for
// front.
func (c CardConfig) validateOncePer() error {
	t := c.Trigger
	ok := true
	switch c.OncePer {
	case "":
	case OncePerMember:
		ok = t.Role != "" || t.LoyaltyBelow > 0 || t.LoyaltyAbove > 0
	case OncePerCorner:
		ok = t.Corners > 0 || t.Contested
	case OncePerFront:
		ok = t.Fronts
	default:
		return fmt.Errorf("card %q: once_per %q; want %q, %q or %q", c.ID, c.OncePer, OncePerMember, OncePerCorner, OncePerFront)
	}
	if !ok {
		return fmt.Errorf("card %q: once_per %q, but its trigger names no %s", c.ID, c.OncePer, c.OncePer)
	}
	return nil
}

// validateStakes checks a card's stakes and its sum (#342): a declared
// kind, no negative sum or weight, a cap no lower than the floor, and a
// cap on every personal card that names a sum, so a furnace stays a
// furnace whatever the bag.
func (c CardConfig) validateStakes() error {
	if c.Stakes != StakesPersonal && c.Stakes != StakesBusiness {
		return fmt.Errorf("card %q: stakes %q; want %q or %q", c.ID, c.Stakes, StakesPersonal, StakesBusiness)
	}
	if c.Amount < 0 || c.AmountShare < 0 || c.AmountMax < 0 || c.Weight < 0 || c.WeightRich < 0 {
		return fmt.Errorf("card %q: a negative amount, share, cap or weight", c.ID)
	}
	if c.AmountMax > 0 && c.AmountMax < c.Amount {
		return fmt.Errorf("card %q: amount_max %d is under amount %d", c.ID, c.AmountMax, c.Amount)
	}
	if c.Stakes == StakesPersonal && (c.Amount > 0 || c.AmountShare > 0) && c.AmountMax == 0 {
		return fmt.Errorf("card %q: a personal card that names a sum needs an amount_max", c.ID)
	}
	return nil
}
