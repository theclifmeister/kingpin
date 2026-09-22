package content

import (
	"fmt"
	"math"
)

// LawConfig mirrors law.toml (#41): the chief's term and the election
// cycle, where public pressure comes from and how it fades, what a chief
// personality and a DA stance do to the heat sim's tuning, and what the
// other sims read off a city's pressure.
type LawConfig struct {
	Law      LawTuning              `toml:"law"`
	Pressure PressureSources        `toml:"pressure"`
	Chief    map[string]ChiefConfig `toml:"chief"`
	DA       map[string]DAConfig    `toml:"da"`
	Campaign CampaignTuning         `toml:"campaign"`
	Bribes   BribeTuning            `toml:"bribes"`
	Effects  LawFX                  `toml:"effects"`
}

// BribeTuning is law.toml's [bribes] (#42): what the chief and the DA
// cost and take, what a backfire and a lead cost, what a checkpoint and
// a customs agent cost and hold, when a law-and-order DA ends it all,
// and what a fixer is worth.
type BribeTuning struct {
	ChiefPrice       int     `toml:"chief_price"`
	DAPrice          int     `toml:"da_price"`
	DAOddsCap        float64 `toml:"da_odds_cap"`
	BribeDays        int     `toml:"bribe_days"`
	LazyEffect       float64 `toml:"lazy_effect"`
	BackfireEvidence int     `toml:"backfire_evidence"`
	BackfireHeat     float64 `toml:"backfire_heat"`
	LeadsCase        int     `toml:"leads_case"`
	LeadEvidence     int     `toml:"lead_evidence"`
	LeadDecayDays    int     `toml:"lead_decay_days"`
	CheckpointPrice  int     `toml:"checkpoint_price"`
	CustomsPrice     int     `toml:"customs_price"`
	CheckpointDays   int     `toml:"checkpoint_days"`
	CallsStopDays    int     `toml:"calls_stop_days"`
	FixerOdds        float64 `toml:"fixer_odds"`
	FixerDiscount    float64 `toml:"fixer_discount"`
	FavoursMax       int     `toml:"favours_max"`     // the favour (#228): favours a bought chief can owe at once; 0 boxes it
	FavourEvidence   int     `toml:"favour_evidence"` // pages the morning a favour is called in: the chief's name is in your ledger
}

// CampaignTuning is law.toml's [campaign] (#193): what clean cash behind
// a DA ticket does to the vote, when a campaign takes money, what it
// costs in pressure while it runs, and what a backed winner and a
// backed loser are worth.
type CampaignTuning struct {
	Cash             int     `toml:"cash"`                // clean cash per point (0.01) of a city's vote share moved
	SwingMax         float64 `toml:"swing_max"`           // the most a city's campaign moves its share
	OpenDays         int     `toml:"open_days"`           // days before the election that tickets take money
	Pressure         float64 `toml:"pressure"`            // pressure a day in a city whose campaign holds money
	BackedDAPriceMul float64 `toml:"backed_da_price_mul"` // #42: the DA's price under one you backed
	LoserPressure    float64 `toml:"loser_pressure"`      // pressure at once in a city that backed the loser
	LoserChief       bool    `toml:"loser_chief"`         // a zealous chief for a city that backed the loser of a law-and-order win
}

type LawTuning struct {
	ChiefTerm       int     `toml:"chief_term"`       // days a chief serves; 0 for life
	TermDays        int     `toml:"term_days"`        // days between DA elections; 0 for none
	ReplacePressure float64 `toml:"replace_pressure"` // a law-and-order DA elected over this mean pressure replaces the chief
	ObserveDays     int     `toml:"observe_days"`     // days in office before the chief's personality shows
	Baseline        float64 `toml:"baseline"`         // pressure fades toward this
	Decay           float64 `toml:"decay"`            // fraction of the gap to baseline closed per day
	Band            float64 `toml:"band"`             // PressureShifted fires on crossing a multiple of this
	GoodwillCash    int     `toml:"goodwill_cash"`    // clean cash per point of goodwill
	GoodwillCut     float64 `toml:"goodwill_cut"`     // pressure a day full goodwill takes off
	GoodwillDecay   float64 `toml:"goodwill_decay"`   // fraction of goodwill that fades per day
	ElectionSwing   float64 `toml:"election_swing"`   // how hard mean pressure swings the vote
	Moderate        float64 `toml:"moderate"`         // the moderate's share of every election
}

// PressureSources is where a city's pressure comes from.
type PressureSources struct {
	Strike    float64  `toml:"strike"`
	Push      float64  `toml:"push"`
	Crackdown float64  `toml:"crackdown"`
	Boost     float64  `toml:"boost"`         // your enforcers robbing a rival corner (#70), landed or not
	RivalRaid float64  `toml:"rival_raid"`    // the police taking a rival corner on your tip (#70)
	FrontGrew float64  `toml:"front_grew"`    // a front whose growth made the paper (#192), where you are, the morning after
	HardUnits float64  `toml:"hard_units"`    // a point per this many units of a hard product sold in a day
	Hard      []string `toml:"hard_products"` // the products that count
	Overdose  float64  `toml:"overdose"`      // an overdose on your corner (#47), in its city
	Body      float64  `toml:"body"`          // a death on your corner, either side (#46), in its city
	Factions  float64  `toml:"factions"`      // two factions fighting over a corner (#43), in its city: their war, the city's story
	Headline  float64  `toml:"headline"`
	Sources   []string `toml:"headline_sources"` // journal sources whose headlines are about you
}

// ChiefConfig is what a chief personality does to the heat sim's tuning:
// multipliers on the response cooldown, what a patrol lets through and
// the daily decay.
type ChiefConfig struct {
	Cooldown float64 `toml:"cooldown"`
	Cap      float64 `toml:"cap"`
	Decay    float64 `toml:"decay"`
}

// DAConfig is what a DA stance does to the heat sim's tuning: multipliers
// on the pages an indictment needs and on the sting threshold.
type DAConfig struct {
	EvidenceArrest float64 `toml:"evidence_arrest"`
	Sting          float64 `toml:"sting"`
}

// LawFX is what a city's pressure does at 100; each sim scales the knob
// it owns and never reads another sim's.
type LawFX struct {
	PressureThresholdCut float64 `toml:"pressure_threshold_cut"` // heat: fraction cut from every response threshold
	PressureCapCut       float64 `toml:"pressure_cap_cut"`       // heat: fraction cut from what a patrol lets through
	PressureTip          float64 `toml:"pressure_tip"`           // rivals: extra on the chance of a police tip
	BackedSting          float64 `toml:"backed_sting"`           // heat: the sting line under a DA you backed (#193); 0 reads as 1
	BribeDecayMul        float64 `toml:"bribe_decay_mul"`        // heat: the decay under a bought chief (#42); 0 reads as 1
	BribeCooldown        int     `toml:"bribe_cooldown"`         // heat: days on the sting and raid cooldown under a bought chief
	BribedDAEvidenceMul  float64 `toml:"bribed_da_evidence_mul"` // heat: the pages an indictment needs under a bought DA; 0 reads as 1
	CheckpointCut        float64 `toml:"checkpoint_cut"`         // logistics: the cut from a bought car or truck edge's day risk
	CustomsCut           float64 `toml:"customs_cut"`            // logistics: the cut from a bought boat edge's day risk
}

// ChiefPersonalities are the personalities a chief can have, in a fixed
// order for generation.
var ChiefPersonalities = []string{"corrupt", "zealous", "lazy"}

// DAStances are the tickets a DA can run on, in a fixed order for
// generation.
var DAStances = []string{"law_and_order", "moderate", "reform"}

// ChiefFor returns the tuning for a chief personality, neutral for one
// the config does not know.
func (l LawConfig) ChiefFor(name string) ChiefConfig {
	if c, ok := l.Chief[name]; ok {
		return c
	}
	return ChiefConfig{Cooldown: 1, Cap: 1, Decay: 1}
}

// DAFor returns the tuning for a DA stance, neutral for one the config
// does not know.
func (l LawConfig) DAFor(name string) DAConfig {
	if d, ok := l.DA[name]; ok {
		return d
	}
	return DAConfig{EvidenceArrest: 1, Sting: 1}
}

// validate checks the tables read as one: a table per personality and
// stance with positive multipliers, and sane pacing.
func (l LawConfig) validate() error {
	for _, p := range ChiefPersonalities {
		c, ok := l.Chief[p]
		if !ok {
			return fmt.Errorf("no [chief.%s] table", p)
		}
		if c.Cooldown <= 0 || c.Cap <= 0 || c.Decay <= 0 {
			return fmt.Errorf("bad [chief.%s] table %+v", p, c)
		}
	}
	for _, st := range DAStances {
		d, ok := l.DA[st]
		if !ok {
			return fmt.Errorf("no [da.%s] table", st)
		}
		if d.EvidenceArrest <= 0 || d.Sting <= 0 {
			return fmt.Errorf("bad [da.%s] table %+v", st, d)
		}
	}
	t := l.Law
	if t.ChiefTerm < 0 || t.TermDays < 0 || t.ObserveDays < 0 || t.Band <= 0 || t.Decay < 0 || t.Decay > 1 || t.GoodwillCash <= 0 || t.GoodwillDecay < 0 || t.GoodwillDecay > 1 || t.Moderate < 0 || t.Moderate >= 1 {
		return fmt.Errorf("bad [law] table %+v", t)
	}
	if fx := l.Effects; fx.PressureThresholdCut < 0 || fx.PressureThresholdCut >= 1 || fx.PressureCapCut < 0 || fx.PressureCapCut > 1 || fx.PressureTip < 0 || fx.BackedSting < 0 ||
		fx.BribeDecayMul < 0 || fx.BribeCooldown < 0 || fx.BribedDAEvidenceMul < 0 || fx.CheckpointCut < 0 || fx.CheckpointCut > 1 || fx.CustomsCut < 0 || fx.CustomsCut > 1 {
		return fmt.Errorf("bad [effects] table %+v", fx)
	}
	if b := l.Bribes; b.ChiefPrice < 0 || b.DAPrice < 0 || b.DAOddsCap < 0 || b.DAOddsCap > 1 || b.BribeDays < 0 || b.LazyEffect < 0 || b.LazyEffect > 1 || b.BackfireEvidence < 0 || b.BackfireHeat < 0 ||
		b.LeadsCase < 0 || b.LeadEvidence < 0 || b.LeadDecayDays < 0 || b.CheckpointPrice < 0 || b.CustomsPrice < 0 || b.CheckpointDays < 0 || b.CallsStopDays < 0 || (t.TermDays > 0 && b.CallsStopDays >= t.TermDays) ||
		b.FixerOdds < 0 || b.FixerDiscount < 0 || b.FixerDiscount >= 1 {
		return fmt.Errorf("bad [bribes] table %+v", b)
	}
	if c := l.Campaign; c.Cash < 0 || c.SwingMax < 0 || c.SwingMax > 0.5 || c.OpenDays < 0 || c.Pressure < 0 || c.BackedDAPriceMul < 0 || c.LoserPressure < 0 || (t.TermDays > 0 && c.OpenDays >= t.TermDays) {
		return fmt.Errorf("bad [campaign] table %+v", c)
	}
	return nil
}

// Swing is the share of a city's vote that cash behind a ticket moves
// (#193): a point per Cash, at most SwingMax. No price, no swing.
func (c CampaignTuning) Swing(cash int) float64 {
	if c.Cash <= 0 || cash <= 0 {
		return 0
	}
	return math.Min(c.SwingMax, float64(cash)/float64(c.Cash)/100)
}

// Fill is the clean cash that buys a city's campaign the whole swing.
func (c CampaignTuning) Fill() int { return int(math.Round(c.SwingMax * 100 * float64(c.Cash))) }
