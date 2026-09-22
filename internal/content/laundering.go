package content

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/events"
)

// LaunderingConfig mirrors laundering.toml.
type LaunderingConfig struct {
	Laundering  LaunderingTuning  `toml:"laundering"`
	Dial        LaunderTable      `toml:"dial"`
	Growth      GrowthConfig      `toml:"growth"`
	Offshore    OffshoreConfig    `toml:"offshore"`
	Businessman BusinessmanConfig `toml:"businessman"`
	Fronts      []FrontConfig     `toml:"front"`
}

// BusinessmanConfig is the [businessman] table (#49): the run ends a
// businessman once the fronts' own income net of their upkeep
// (Sim.LegitIncome) has out-earned the street's revenue for LegitDays
// days in a row with home's goodwill over its pressure (World.LegitDays
// counts them, this sim's). Zero boxes it.
type BusinessmanConfig struct {
	LegitDays int `toml:"legit_days"`
}

// OffshoreConfig is the [offshore] table (#195): the account clean cash
// is reserved into, which survives every ending and is the score. Lot
// is the clean cash a day that moves unnoticed (over it, the heat sim
// files structure_evidence a lot the morning after), Fee the share of
// what moves that the account keeps, RetireCash what the account needs
// and RetireDays how many quiet days (every city under RetireHeat, no
// strike, no patrol or worse, no live contract) before Retire is open.
type OffshoreConfig struct {
	Lot        int     `toml:"lot"`
	Fee        float64 `toml:"fee"`
	RetireCash int     `toml:"retire_cash"`
	RetireDays int     `toml:"retire_days"`
	RetireHeat float64 `toml:"retire_heat"`
}

type LaunderingTuning struct {
	Float                int     `toml:"float"` // dirty cash the wash always leaves in the till
	AuditFreezeDays      int     `toml:"audit_freeze_days"`
	AuditSeize           float64 `toml:"audit_seize"`
	UpkeepFreezeDays     int     `toml:"upkeep_freeze_days"`
	AccountantThroughput float64 `toml:"accountant_throughput"`
	AccountantRiskCut    float64 `toml:"accountant_risk_cut"`
}

type LaunderTable struct {
	Careful LaunderConfig `toml:"careful"`
	Normal  LaunderConfig `toml:"normal"`
	Greedy  LaunderConfig `toml:"greedy"`
}

type LaunderConfig struct {
	Mul  float64 `toml:"mul"`  // multiplier on every front's throughput
	Risk float64 `toml:"risk"` // multiplier on every front's audit risk
}

type FrontConfig struct {
	ID         string  `toml:"id"`
	Name       string  `toml:"name"`
	Cost       int     `toml:"cost"`       // dirty cash, once
	Throughput int     `toml:"throughput"` // dirty cash washed per day at the normal dial
	Upkeep     int     `toml:"upkeep"`     // clean cash per day
	AuditRisk  float64 `toml:"audit_risk"` // chance per day of an audit at the normal dial
	UnlockCash int     `toml:"unlock_cash"`

	// The levels (#192): clean cash invested in the front for clean
	// income of its own. Income is what the first level earns a day,
	// LevelCost what it costs, LevelMul what each level's cost, income,
	// throughput and upkeep are over the last, MaxLevel how far it goes.
	// A front with none of these has no levels to buy.
	Income    int     `toml:"income"`
	LevelCost int     `toml:"level_cost"`
	LevelMul  float64 `toml:"level_mul"`
	MaxLevel  int     `toml:"max_level"`
}

// GrowthConfig is the [growth] table (#192): what a levelled front adds
// to its audit risk (AuditLevel per level), the wash a front's own
// income has to explain (a day's wash over LegitRatio times its income
// scales the risk by the excess), and the level whose reaching makes the
// paper (HeadlineLevel; the pressure and notoriety it draws are the law's
// and the reputation sim's, off the headline and the front's Grew stamp).
type GrowthConfig struct {
	AuditLevel    float64 `toml:"audit_level"`
	LegitRatio    float64 `toml:"legit_ratio"`
	HeadlineLevel int     `toml:"headline_level"`
}

// Front returns the config for id, or nil.
func (l LaunderingConfig) Front(id string) *FrontConfig {
	return find(l.Fronts, func(e *FrontConfig) bool { return e.ID == id })
}

// DialFor returns the tuning for a launder dial position.
func (l LaunderingConfig) DialFor(d events.Launder) LaunderConfig {
	return threeWay(int(d), l.Dial.Careful, l.Dial.Normal, l.Dial.Greedy)
}

// validate checks the fronts and the tables that grow them: at least
// one front, ids unique, a price and a wash on each, the levels (#192)
// whole where a front has them, and [growth] and [offshore] (#195) in
// range.
func (l LaunderingConfig) validate() error {
	if len(l.Fronts) == 0 {
		return fmt.Errorf("no fronts defined")
	}
	seen := map[string]bool{}
	for _, f := range l.Fronts {
		if f.ID == "" || seen[f.ID] || f.Cost <= 0 || f.Throughput <= 0 {
			return fmt.Errorf("bad front %+v", f)
		}
		if f.MaxLevel > 0 && (f.Income <= 0 || f.LevelCost <= 0 || f.LevelMul < 1) {
			return fmt.Errorf("front %s has levels but no income, level_cost or level_mul", f.ID)
		}
		seen[f.ID] = true
	}
	if g := l.Growth; g.AuditLevel < 0 || g.LegitRatio < 0 || g.HeadlineLevel < 0 {
		return fmt.Errorf("bad [growth] table %+v", g)
	}
	if o := l.Offshore; o.Lot < 0 || o.Fee < 0 || o.Fee >= 1 || o.RetireCash < 0 || o.RetireDays < 0 || o.RetireHeat < 0 {
		return fmt.Errorf("bad [offshore] table %+v", o)
	}
	return nil
}
