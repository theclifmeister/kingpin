package content

import (
	"fmt"
	"sort"

	"github.com/theclifmeister/kingpin/internal/events"
)

// CrewConfig mirrors crew.toml.
type CrewConfig struct {
	Crew       CrewTuning            `toml:"crew"`
	Informant  InformantTuning       `toml:"informant"`
	Lieutenant LieutenantTuning      `toml:"lieutenant"`
	Life       LifeTuning            `toml:"life"`
	Pay        PayTable              `toml:"pay"`
	Role       map[string]RoleConfig `toml:"role"`
	Traits     TraitsTuning          `toml:"traits"`  // #346: when a veteran shows what they are
	Trait      map[string]Trait      `toml:"trait"`   // #346: what each trait is, by name
	Captain    CaptainTuning         `toml:"captain"` // #346: who can run crew care for a city, and what it costs
}

// What a member lived through (#346), the words a trait's lived table
// weighs: stood on a corner the police hit, shot and got up, did time.
const (
	LivedRaid = "raid"
	LivedShot = "shot"
	LivedJail = "jail"
)

// TraitsTuning is when a member shows a trait (#346): at Days of
// service, counted from the day they signed. Zero is the table boxed:
// nobody shows one, and a run is the run before the feature.
type TraitsTuning struct {
	Days int `toml:"days"` // days on the payroll before a member reveals a trait; 0 never
}

// On reports whether traits are revealed at all.
func (t TraitsTuning) On() bool { return t.Days > 0 }

// Trait is one trait a veteran can show (#346): how likely it is, by
// role and by what they lived through, and the crew effect words it
// folds for that member alone. A zero multiplier reads as 1, so a trait
// names only the words it moves.
type Trait struct {
	Weight float64            `toml:"weight"` // the base weight in the draw
	Role   map[string]float64 `toml:"role"`   // times this for a member of the role; with a table, a role it does not name never draws it
	Lived  map[string]float64 `toml:"lived"`  // times this for each of raid, shot, jail they lived through
	Good   bool               `toml:"good"`   // a trait the crew screen reads as a strength
	Says   string             `toml:"says"`   // the pane's line for it

	LoyaltyLossMul float64 `toml:"loyalty_loss_mul"` // on a night's loyalty loss, theirs alone
	SkimChanceMul  float64 `toml:"skim_chance_mul"`  // on their roll to skim under the line
	Sharp          bool    `toml:"sharp"`            // never counted as sloppy on a corner (heat.toml sloppy_skill)
	BuyerGapMul    float64 `toml:"buyer_gap_mul"`    // on the buyers' gaps while they are at work (buyers.toml min_gap, max_gap)
	Deterrence     float64 `toml:"deterrence"`       // enforcers' worth of deterrence against a skimmer they add at work
	Heat           float64 `toml:"heat"`             // their corner counts this sloppy (heat.toml sloppy_heat), on top of any skill gap
}

// TraitNames are the trait names in the config, in a fixed order: the order
// the draw walks them.
func (c CrewConfig) TraitNames() []string {
	names := make([]string, 0, len(c.Trait))
	for n := range c.Trait {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// TraitOf is the trait a member has, the zero Trait for none or one the
// file does not know: every fold of the zero Trait is the old number.
func (c CrewConfig) TraitOf(name string) Trait {
	if name == "" {
		return Trait{}
	}
	return c.Trait[name]
}

// WeightFor is the trait's weight in the draw for a member of role who
// lived through lived: the weight, times the role's entry (none, with a
// role table, is 0), times the lived entry for each word there is one.
func (t Trait) WeightFor(role string, lived []string) float64 {
	w := t.Weight
	if len(t.Role) > 0 {
		w *= t.Role[role]
	}
	for _, l := range lived {
		if m, ok := t.Lived[l]; ok {
			w *= m
		}
	}
	return w
}

// one reads a zero multiplier as 1.
func one(v float64) float64 {
	if v == 0 {
		return 1
	}
	return v
}

// LossMul is the trait's fold on a night's loyalty loss.
func (t Trait) LossMul() float64 { return one(t.LoyaltyLossMul) }

// SkimMul is the trait's fold on the roll to skim.
func (t Trait) SkimMul() float64 { return one(t.SkimChanceMul) }

// GapMul is the trait's fold on the buyers' gaps.
func (t Trait) GapMul() float64 { return one(t.BuyerGapMul) }

// CaptainTuning is the captain (#346): a trusted member who runs crew
// care for a city through the player's own actions. Loyalty and Days
// are what naming one needs; Care is the loyalty under which they stop
// caring (they do not flip: that is the lieutenant's drama); Cut is
// their share of the city's takings; Margin is how close over the quit
// line a member is before the captain pays them off; Budgets are the
// nightly pay-off budgets the player picks from, the first the default.
type CaptainTuning struct {
	Loyalty float64 `toml:"loyalty"` // naming one needs this loyalty ...
	Days    int     `toml:"days"`    // ... and this many days on the payroll
	Care    float64 `toml:"care"`    // under this loyalty they stop caring
	Cut     float64 `toml:"cut"`     // share of their city's takings they keep
	Margin  float64 `toml:"margin"`  // a member this close over the quit line gets a pay-off
	Budgets []int   `toml:"budgets"` // the pay-off budgets a night the player picks from
}

// LifeTuning is crew life (#46): kin, ageing, arrests and getting
// shot. The zero value is the table boxed: nothing ages, nobody is
// arrested, wounded or killed and no kin come looking, so a run under
// it is the run before the feature (harness.NoLife).
type LifeTuning struct {
	YearDays       int                `toml:"year_days"` // a crew year; 0 is nobody ageing
	AgeMin         int                `toml:"age_min"`   // a candidate's age at generation
	AgeMax         int                `toml:"age_max"`
	SkillGrowth    float64            `toml:"skill_growth"` // skill a day on the payroll at fair pay, times the pay dial's wage multiplier
	SkillCap       int                `toml:"skill_cap"`    // ... up to this
	NerveAge       int                `toml:"nerve_age"`    // past this age every birthday takes NerveLoss
	NerveLoss      int                `toml:"nerve_loss"`
	RetireAge      int                `toml:"retire_age"`    // the birthday they retire on
	RetireLoyal    float64            `toml:"retire_loyal"`  // a retiree at or over this loyalty recommends a kin
	ArrestChance   float64            `toml:"arrest_chance"` // per runner or enforcer on a corner the police hit, and the chemist on a raid that took stock
	JailDays       int                `toml:"jail_days"`
	BailLoyalty    float64            `toml:"bail_loyalty"` // what a bail buys the one bailed
	JailLoyalty    float64            `toml:"jail_loyalty"` // an unbailed release comes back this far under the informant line
	ReleaseTurn    float64            `toml:"release_turn"` // ... and turns informant at once with this chance
	WoundChance    float64            `toml:"wound_chance"` // the guard, per strike or failed push, before the multipliers
	KillChance     float64            `toml:"kill_chance"`
	WoundDays      int                `toml:"wound_days"`
	KinChance      float64            `toml:"kin_chance"`       // a hire's kin come looking
	KinDiscount    float64            `toml:"kin_discount"`     // ... at this much off the fee
	KinLoyalty     float64            `toml:"kin_loyalty"`      // what a firing, a walk-out or a pay-off moves the kin's loyalty by
	KinBodyLoyalty float64            `toml:"kin_body_loyalty"` // what a kin shot dead on your corner costs
	KinBodyNerve   int                `toml:"kin_body_nerve"`   // ... and the nerve it gives them
	Force          map[string]float64 `toml:"force"`            // the shot multiplier by a strike's force: warn, push, hit
	Personality    map[string]float64 `toml:"personality"`      // ... and by the rival's personality on a push
}

// On reports whether the table is live: a year_days of zero boxes it.
func (l LifeTuning) On() bool { return l.YearDays > 0 }

// ForceMul is the shot multiplier at a force, 1 for one the table does
// not name.
func (l LifeTuning) ForceMul(force string) float64 {
	if m, ok := l.Force[force]; ok {
		return m
	}
	return 1
}

// PersonalityMul is the shot multiplier for a rival personality, 1 for
// one the table does not name.
func (l LifeTuning) PersonalityMul(p string) float64 {
	if m, ok := l.Personality[p]; ok {
		return m
	}
	return 1
}

func (l LifeTuning) validate() error {
	if !l.On() {
		return nil
	}
	if l.AgeMin <= 0 || l.AgeMax < l.AgeMin || l.RetireAge <= l.AgeMax || l.SkillCap <= 0 || l.JailDays <= 0 || l.WoundDays <= 0 {
		return fmt.Errorf("[life]: ages %d..%d, retire %d, skill cap %d, jail %d, wound %d days do not make a life", l.AgeMin, l.AgeMax, l.RetireAge, l.SkillCap, l.JailDays, l.WoundDays)
	}
	for _, p := range []float64{l.ArrestChance, l.ReleaseTurn, l.WoundChance, l.KillChance, l.KinChance, l.KinDiscount} {
		if p < 0 || p > 1 {
			return fmt.Errorf("[life]: a chance %v is not in 0..1", p)
		}
	}
	return nil
}

// LieutenantTuning is who runs a city for you: how often one is looking
// for work, when they turn and what they feed the DA, how long before
// you know what they are like, and the four temperaments.
type LieutenantTuning struct {
	Chance        float64                          `toml:"chance"`         // chance a candidate is a lieutenant, once corners are held in two cities
	Flip          float64                          `toml:"flip"`           // under this loyalty a lieutenant turns informant, no dice
	Evidence      int                              `toml:"evidence"`       // pages a flipped lieutenant feeds the DA per leak
	RevealDays    int                              `toml:"reveal_days"`    // days on the job before the report names their personality
	BetrayShare   float64                          `toml:"betray_share"`   // a lieutenant who flips running a city that holds this share of your corners ends the run betrayed (#49); 0 never
	BetrayCorners int                              `toml:"betray_corners"` // ... and at least this many of them: a flip over one corner is a leak, not a betrayal
	RobbedOff     int                              `toml:"robbed_off"`     // a corner robbed this many times is not worth the stock: they take the crew off it and never post or guard it (#275)
	Personality   map[string]LieutenantPersonality `toml:"personality"`
}

// LieutenantPersonality is what a temperament does to the city it runs.
type LieutenantPersonality struct {
	Dial      string  `toml:"dial"`       // the sell dial they favour: quiet, normal, aggressive
	Heat      float64 `toml:"heat"`       // multiplier on the sale heat of their city
	Skim      float64 `toml:"skim"`       // share of their city's takings they take on top of the cut, at any loyalty
	Guard     bool    `toml:"guard"`      // they post idle enforcers on their corners
	StockDays float64 `toml:"stock_days"` // days of the worked corners' demand they keep the stash at by supply contract (#174); 0 buys nothing
}

// LieutenantPersonalities are the temperaments a lieutenant can have, in
// a fixed order for generation.
var LieutenantPersonalities = []string{"violent", "greedy", "careful", "steady"}

// Temper returns the tuning for a lieutenant temperament, or a steady
// default for one the config does not know.
func (t LieutenantTuning) Temper(name string) LieutenantPersonality {
	if p, ok := t.Personality[name]; ok {
		return p
	}
	return LieutenantPersonality{Dial: events.DialNormal.String(), Heat: 1, Guard: true}
}

// SellDial is the dial a temperament sells at; normal for a name the
// sell dial does not know, which validate refuses at load.
func (p LieutenantPersonality) SellDial() events.Dial {
	if d, ok := events.ParseDial(p.Dial); ok {
		return d
	}
	return events.DialNormal
}

// InformantTuning is who turns, and what finding and keeping them costs.
type InformantTuning struct {
	Loyalty            float64 `toml:"loyalty"`
	Nerve              int     `toml:"nerve"`
	Chance             float64 `toml:"chance"`
	InvestigateCost    int     `toml:"investigate_cost"`
	InvestigateBase    float64 `toml:"investigate_base"`
	InvestigateSkill   float64 `toml:"investigate_skill"`
	InvestigateLearn   float64 `toml:"investigate_learn"`
	InvestigateLoyalty float64 `toml:"investigate_loyalty"`
	PayoffWages        int     `toml:"payoff_wages"`
	PayoffLoyalty      float64 `toml:"payoff_loyalty"`
}

type CrewTuning struct {
	MaxCrew         int     `toml:"max_crew"`
	Candidates      int     `toml:"candidates"`
	PoolDays        int     `toml:"pool_days"`
	UnitsPerSkill   float64 `toml:"units_per_skill"`
	HireFeeBase     int     `toml:"hire_fee_base"`
	HireFeePerSkill float64 `toml:"hire_fee_per_skill"`
	StartLoyaltyMin int     `toml:"start_loyalty_min"`
	StartLoyaltyMax int     `toml:"start_loyalty_max"`
	SkillMin        int     `toml:"skill_min"` // a candidate's skill is drawn in skill_min..skill_max (#275), before the tree's skill_bonus
	SkillMax        int     `toml:"skill_max"`
	GreedMin        int     `toml:"greed_min"` // ... greed in greed_min..greed_max
	GreedMax        int     `toml:"greed_max"`
	NerveMin        int     `toml:"nerve_min"` // ... and nerve in nerve_min..nerve_max
	NerveMax        int     `toml:"nerve_max"`
	GreedDrift      float64 `toml:"greed_drift"`
	DangerDays      int     `toml:"danger_days"`
	DangerLoyalty   float64 `toml:"danger_loyalty"`
	FireLoyalty     float64 `toml:"fire_loyalty"`
	UnpaidLoyalty   float64 `toml:"unpaid_loyalty"`
	SkimThreshold   float64 `toml:"skim_threshold"`
	SkimChance      float64 `toml:"skim_chance"`
	SkimShare       float64 `toml:"skim_share"`
	SkimCap         float64 `toml:"skim_cap"`
	SuspectDays     int     `toml:"suspect_days"`
	QuitThreshold   float64 `toml:"quit_threshold"`
	AlertMargin     float64 `toml:"alert_margin"` // a member this close over their next line is an alert (#345); 0 is none
}

type PayTable struct {
	Stingy   PayConfig `toml:"stingy"`
	Fair     PayConfig `toml:"fair"`
	Generous PayConfig `toml:"generous"`
}

type PayConfig struct {
	Wage    float64 `toml:"wage"`    // multiplier on each member's fair wage
	Loyalty float64 `toml:"loyalty"` // loyalty drift per day
}

type RoleConfig struct {
	WageBase     float64 `toml:"wage_base"`
	WagePerSkill float64 `toml:"wage_per_skill"`
	Protection   float64 `toml:"protection"` // fraction of danger loyalty loss each one absorbs
	Deterrence   float64 `toml:"deterrence"` // fraction of skim chance each one removes
	Cut          float64 `toml:"cut"`        // share of their city's takings a lieutenant keeps
	Crew         int     `toml:"crew"`       // roster slots an assigned lieutenant adds

	// The chemist (#47): their quality is QualityBase + QualityPerSkill
	// x skill, what a cook lands at; a cut keeps CutBonus x skill points
	// of what it would have lost; a cook takes CookDays and is for at
	// most BatchPerSkill x skill units.
	QualityBase     float64 `toml:"quality_base"`
	QualityPerSkill float64 `toml:"quality_per_skill"`
	CutBonus        float64 `toml:"cut_bonus"`
	CookDays        int     `toml:"cook_days"`
	BatchPerSkill   float64 `toml:"batch_per_skill"`
	UnlockProduct   string  `toml:"unlock_product"` // the product whose listing brings a chemist looking for work

	// The fixer (#42): the chance a candidate is one once fixers are
	// wanted, and what a backfire costs their loyalty.
	Chance          float64 `toml:"chance"`
	BackfireLoyalty float64 `toml:"backfire_loyalty"`

	// Crew life (#46): the clean cash that bails a member of the role
	// out, and for the driver what a skill-100 one takes off a
	// shipment's risk per day on the road.
	Bail      int     `toml:"bail"`
	DriverCut float64 `toml:"driver_cut"`
}

// PayFor returns the tuning for a pay dial position.
func (c CrewConfig) PayFor(p events.Pay) PayConfig {
	return threeWay(int(p), c.Pay.Stingy, c.Pay.Fair, c.Pay.Generous)
}

// validate checks the crew file has what the sims index directly: a
// [role.X] table for every role the game hires (game.Role*, the driver's
// and the fixer's included), a table per lieutenant temperament with a
// dial the market knows, a sane [lieutenant] and [role.lieutenant], a
// [life] that makes a life, and a chemist whose unlock product is on the
// ladder (market).
func (c CrewConfig) validate(market MarketConfig) error {
	for _, role := range []string{"runner", "enforcer", "accountant", "lieutenant", "chemist", "driver", "fixer"} {
		if _, ok := c.Role[role]; !ok {
			return fmt.Errorf("no [role.%s] table", role)
		}
	}
	for _, p := range LieutenantPersonalities {
		lp, ok := c.Lieutenant.Personality[p]
		if !ok {
			return fmt.Errorf("no [lieutenant.personality.%s] table", p)
		}
		if _, ok := events.ParseDial(lp.Dial); !ok {
			return fmt.Errorf("[lieutenant.personality.%s] dial %q: not one of %v", p, lp.Dial, events.DialNames())
		}
		if lp.Heat <= 0 || lp.Skim < 0 || lp.Skim >= 1 || lp.StockDays < 0 {
			return fmt.Errorf("bad [lieutenant.personality.%s] table %+v", p, lp)
		}
	}
	t := c.Crew
	for _, r := range []struct {
		name     string
		min, max int
	}{{"skill", t.SkillMin, t.SkillMax}, {"greed", t.GreedMin, t.GreedMax}, {"nerve", t.NerveMin, t.NerveMax}} {
		if r.min < 0 || r.max < r.min || r.max > 100 {
			return fmt.Errorf("[crew] %s_min %d and %s_max %d are not a range in 0..100", r.name, r.min, r.name, r.max)
		}
	}
	if t.AlertMargin < 0 {
		return fmt.Errorf("[crew] alert_margin %v is under zero", t.AlertMargin)
	}
	if lt := c.Lieutenant; lt.Chance < 0 || lt.Chance > 1 || lt.Flip < 0 || lt.RevealDays < 0 || lt.RobbedOff < 1 {
		return fmt.Errorf("bad [lieutenant] table %+v", lt)
	}
	if r := c.Role["lieutenant"]; r.Cut < 0 || r.Cut >= 1 || r.Crew < 0 {
		return fmt.Errorf("bad [role.lieutenant] table %+v", r)
	}
	if err := c.Life.validate(); err != nil {
		return err
	}
	if err := c.validateVeterans(); err != nil {
		return err
	}
	if r := c.Role["chemist"]; r.QualityBase < 0 || r.QualityPerSkill < 0 || r.CutBonus < 0 || r.CookDays < 1 || r.BatchPerSkill <= 0 || market.Product(r.UnlockProduct) == nil {
		return fmt.Errorf("bad [role.chemist] table %+v", r)
	}
	return nil
}

// validateVeterans checks the traits and the captain (#346): every
// trait drawable and saying what it is, its tables naming only what
// exists, and a captain with a budget to pick.
func (c CrewConfig) validateVeterans() error {
	if c.Traits.Days < 0 {
		return fmt.Errorf("[traits] days %d is under zero", c.Traits.Days)
	}
	if c.Traits.On() && len(c.Trait) == 0 {
		return fmt.Errorf("[traits] days %d with no [trait.*] table", c.Traits.Days)
	}
	for _, n := range c.TraitNames() {
		tr := c.Trait[n]
		if tr.Weight <= 0 || tr.LoyaltyLossMul < 0 || tr.SkimChanceMul < 0 || tr.BuyerGapMul < 0 || tr.Deterrence < 0 || tr.Heat < 0 || tr.Says == "" {
			return fmt.Errorf("bad [trait.%s] table %+v", n, tr)
		}
		for k, v := range tr.Lived {
			if (k != LivedRaid && k != LivedShot && k != LivedJail) || v < 0 {
				return fmt.Errorf("[trait.%s] lived %s = %v: lived is raid, shot or jail, at zero or over", n, k, v)
			}
		}
		for k, v := range tr.Role {
			if _, ok := c.Role[k]; !ok || v < 0 {
				return fmt.Errorf("[trait.%s] role %s = %v: no such role, or under zero", n, k, v)
			}
		}
	}
	cp := c.Captain
	if cp.Days < 0 || cp.Loyalty < 0 || cp.Loyalty > 100 || cp.Care < 0 || cp.Cut < 0 || cp.Cut >= 1 || cp.Margin < 0 || len(cp.Budgets) == 0 {
		return fmt.Errorf("bad [captain] table %+v", cp)
	}
	for _, b := range cp.Budgets {
		if b < 0 {
			return fmt.Errorf("[captain] budget %d is under zero", b)
		}
	}
	return nil
}
