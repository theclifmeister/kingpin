// Package content loads the tuning data (markets, city, heat, crew, rivals,
// upgrades, laundering, reputation, headlines) that lives in TOML files embedded in the binary. Balance changes never need
// code changes.
package content

import (
	"embed"
	"fmt"
	"slices"

	"github.com/BurntSushi/toml"

	"github.com/theclifmeister/kingpin/internal/events"
)

//go:embed *.toml
var files embed.FS

// Config is everything the simulations need to be constructed.
type Config struct {
	Market     MarketConfig
	City       CityConfig
	Heat       HeatConfig
	Crew       CrewConfig
	Rivals     RivalsConfig
	Laundering LaunderingConfig
	Names      NamesConfig
	Upgrades   UpgradesConfig
	Reputation ReputationConfig
	Headlines  HeadlinesConfig
}

// MarketConfig mirrors market.toml.
type MarketConfig struct {
	Market   MarketTuning    `toml:"market"`
	Products []ProductConfig `toml:"product"`
	Dial     DialTable       `toml:"dial"`
}

type MarketTuning struct {
	City              string  `toml:"city"`
	StartCash         int     `toml:"start_cash"`
	CarryLimit        int     `toml:"carry_limit"`
	HistoryDays       int     `toml:"history_days"`
	SupplierRatio     float64 `toml:"supplier_ratio"`
	BuyPricePressure  float64 `toml:"buy_price_pressure"`
	PriceFloorRatio   float64 `toml:"price_floor_ratio"`
	PriceCeilingRatio float64 `toml:"price_ceiling_ratio"`
	Reversion         float64 `toml:"reversion"`
	GlutDecay         float64 `toml:"glut_decay"`
	SaleImpact        float64 `toml:"sale_impact"`
	ShockChance       float64 `toml:"shock_chance"`
	SlumpChance       float64 `toml:"slump_chance"`
}

type ProductConfig struct {
	ID          string  `toml:"id"`
	Name        string  `toml:"name"`
	BasePrice   float64 `toml:"base_price"`
	Volatility  float64 `toml:"volatility"`
	Demand      float64 `toml:"demand"`
	DemandNoise float64 `toml:"demand_noise"`
	Heat        float64 `toml:"heat"`
	UnlockCash  int     `toml:"unlock_cash"` // supplier offers it once peak cash reaches this; 0 = from day one
}

type DialTable struct {
	Quiet      DialConfig `toml:"quiet"`
	Normal     DialConfig `toml:"normal"`
	Aggressive DialConfig `toml:"aggressive"`
}

type DialConfig struct {
	Fill   float64 `toml:"fill"`
	Price  float64 `toml:"price"`
	Impact float64 `toml:"impact"`
	Heat   float64 `toml:"heat"`
}

// CityConfig mirrors city.toml: the corners and how they are held.
type CityConfig struct {
	Territory TerritoryTuning `toml:"territory"`
	Corners   []CornerConfig  `toml:"corner"`
}

type TerritoryTuning struct {
	Start         string  `toml:"start"`
	DriftDays     int     `toml:"drift_days"`
	RobberyChance float64 `toml:"robbery_chance"`
	RobberyStock  float64 `toml:"robbery_stock"`
	RobberyCash   float64 `toml:"robbery_cash"`
	EnforcerCut   float64 `toml:"enforcer_cut"`
}

type CornerConfig struct {
	ID     string             `toml:"id"`
	Name   string             `toml:"name"`
	X      int                `toml:"x"` // map cell
	Y      int                `toml:"y"`
	Demand float64            `toml:"demand"` // size relative to one standard corner
	Heat   float64            `toml:"heat"`   // sale-heat multiplier for units moved here
	Risk   float64            `toml:"risk"`   // robbery-chance multiplier
	Taste  map[string]float64 `toml:"taste"`  // per-product demand multiplier; missing = 1
}

// HeatConfig mirrors heat.toml.
type HeatConfig struct {
	Heat      HeatTuning       `toml:"heat"`
	Responses []ResponseConfig `toml:"response"`
}

type HeatTuning struct {
	Decay              float64 `toml:"decay"`
	LieLowMultiplier   float64 `toml:"lie_low_multiplier"`
	SaleHeat           float64 `toml:"sale_heat"`
	StreetUnits        float64 `toml:"street_units"`
	DirtyCashThreshold int     `toml:"dirty_cash_threshold"`
	DirtyCashHeat      float64 `toml:"dirty_cash_heat"`
	CooldownDays       int     `toml:"cooldown_days"`
	EvidenceArrest     int     `toml:"evidence_arrest"`
	CrewHeat           float64 `toml:"crew_heat"`
	InformantDays      int     `toml:"informant_days"`
	InformantEvidence  int     `toml:"informant_evidence"`
	InformantHeat      float64 `toml:"informant_heat"`
	SloppySkill        int     `toml:"sloppy_skill"`
	SloppyHeat         float64 `toml:"sloppy_heat"`
	AuditHeat          float64 `toml:"audit_heat"`     // heat an audited front adds the morning after
	AuditEvidence      int     `toml:"audit_evidence"` // evidence an audit adds when the front was run greedy
}

type ResponseConfig struct {
	Level     string  `toml:"level"`
	Threshold float64 `toml:"threshold"`
	CapDays   int     `toml:"cap_days"`
	Cap       float64 `toml:"cap"`
	StockLoss float64 `toml:"stock_loss"`
	CashLoss  float64 `toml:"cash_loss"`
	HeatDrop  float64 `toml:"heat_drop"`
	Evidence  int     `toml:"evidence"`
}

// CrewConfig mirrors crew.toml.
type CrewConfig struct {
	Crew      CrewTuning            `toml:"crew"`
	Informant InformantTuning       `toml:"informant"`
	Pay       PayTable              `toml:"pay"`
	Role      map[string]RoleConfig `toml:"role"`
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
}

// RivalsConfig mirrors rivals.toml.
type RivalsConfig struct {
	Rivals      RivalsTuning                 `toml:"rivals"`
	Personality map[string]PersonalityConfig `toml:"personality"`
	Force       map[string]ForceConfig       `toml:"force"`
}

type RivalsTuning struct {
	ArriveDay          int     `toml:"arrive_day"`
	StartCash          int     `toml:"start_cash"`
	StartMuscle        int     `toml:"start_muscle"`
	Margin             float64 `toml:"margin"`
	MuscleWage         int     `toml:"muscle_wage"`
	MuscleFee          int     `toml:"muscle_fee"`
	ClaimCost          int     `toml:"claim_cost"`
	SupplierMin        float64 `toml:"supplier_min"`
	SupplierMax        float64 `toml:"supplier_max"`
	PushFlip           float64 `toml:"push_flip"`
	PushWar            float64 `toml:"push_war"`
	UndercutPrice      float64 `toml:"undercut_price"`
	TipHeat            float64 `toml:"tip_heat"`
	ObserveDays        int     `toml:"observe_days"`
	RegroupDays        int     `toml:"regroup_days"`
	WarDecay           float64 `toml:"war_decay"`
	WarThreshold       float64 `toml:"war_threshold"`
	CrackdownThreshold float64 `toml:"crackdown_threshold"`
	CrackdownCorners   int     `toml:"crackdown_corners"`
	CrackdownHeat      float64 `toml:"crackdown_heat"`
	CrackdownMuscle    float64 `toml:"crackdown_muscle"`
}

type PersonalityConfig struct {
	ClaimChance     float64 `toml:"claim_chance"`
	MaxCorners      int     `toml:"max_corners"`
	PushPastCap     float64 `toml:"push_past_cap"` // multiplier on push_chance once it holds max_corners; 0 stops
	Grow            string  `toml:"grow"`          // biggest, adjacent, random
	PushChance      float64 `toml:"push_chance"`
	MusclePerCorner float64 `toml:"muscle_per_corner"`
	Undercut        float64 `toml:"undercut"`
	TipChance       float64 `toml:"tip_chance"`
	Defence         float64 `toml:"defence"`
}

type ForceConfig struct {
	Attack  float64 `toml:"attack"`
	Flip    float64 `toml:"flip"`
	Heat    float64 `toml:"heat"`
	War     float64 `toml:"war"`
	Loyalty float64 `toml:"loyalty"`
}

// Personalities are the rival personalities in a fixed order, so a pick
// by seed is reproducible.
var Personalities = []string{"expansionist", "defensive", "opportunist", "chaotic"}

// ForceFor returns the tuning for a force dial position.
func (r RivalsConfig) ForceFor(f events.Force) ForceConfig { return r.Force[f.String()] }

// LaunderingConfig mirrors laundering.toml.
type LaunderingConfig struct {
	Laundering LaunderingTuning `toml:"laundering"`
	Dial       LaunderTable     `toml:"dial"`
	Fronts     []FrontConfig    `toml:"front"`
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
}

// ReputationConfig mirrors reputation.toml: where the three axes come
// from, how they fade, and what the other sims read off them.
type ReputationConfig struct {
	Reputation ReputationTuning `toml:"reputation"`
	Fear       FearSources      `toml:"fear"`
	Respect    RespectSources   `toml:"respect"`
	Notoriety  NotorietySources `toml:"notoriety"`
	Effects    ReputationFX     `toml:"effects"`
}

type ReputationTuning struct {
	Baseline float64 `toml:"baseline"` // every axis fades toward this
	Decay    float64 `toml:"decay"`    // fraction of the gap to baseline closed per day
	Total    float64 `toml:"total"`    // the three never add up to more than this
	Band     float64 `toml:"band"`     // ReputationShifted fires on crossing a multiple of this
}

type FearSources struct {
	StrikeTaken float64 `toml:"strike_taken"`
	StrikeHeld  float64 `toml:"strike_held"`
	PushHeld    float64 `toml:"push_held"`
}

type RespectSources struct {
	GenerousPay float64 `toml:"generous_pay"`
	Payoff      float64 `toml:"payoff"`
	ShortPay    float64 `toml:"short_pay"`
}

type NotorietySources struct {
	Units       float64  `toml:"units"` // a point per this many units sold in a day
	Headline    float64  `toml:"headline"`
	Sources     []string `toml:"headline_sources"` // journal sources whose headlines are about you
	StrikeTaken float64  `toml:"strike_taken"`
	StrikeHeld  float64  `toml:"strike_held"`
}

// ReputationFX is what each axis does at 100; each sim scales the knob it
// owns by the axis and never reads another sim's.
type ReputationFX struct {
	FearPushCut        float64 `toml:"fear_push_cut"`
	FearHeatFloor      float64 `toml:"fear_heat_floor"`
	RespectLoyaltyCut  float64 `toml:"respect_loyalty_cut"`
	RespectSupplierCut float64 `toml:"respect_supplier_cut"`
	NotorietyHireCut   float64 `toml:"notoriety_hire_cut"`
	NotorietyHeat      float64 `toml:"notoriety_heat"`
}

// Scale is v at axis 0 and v times (1 + full) at axis 100: how an effect
// that adds grows with an axis.
func Scale(axis, full float64) float64 { return 1 + full*clamp01(axis/100) }

// Cut is 1 at axis 0 and 1 - full at axis 100: how an effect that takes
// away grows with an axis.
func Cut(axis, full float64) float64 { return 1 - full*clamp01(axis/100) }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// NamesConfig mirrors names.toml.
type NamesConfig struct {
	Crew   []string `toml:"crew"`
	Rivals []string `toml:"rivals"`
}

// UpgradesConfig mirrors upgrades.toml: the upgrade tree.
type UpgradesConfig struct {
	Nodes []UpgradeConfig `toml:"upgrade"`
}

// Branches an upgrade can belong to, in display order.
var Branches = []string{"operations", "security", "legal"}

// UpgradeConfig is one node of the tree.
type UpgradeConfig struct {
	ID       string         `toml:"id"`
	Name     string         `toml:"name"`
	Branch   string         `toml:"branch"`
	Cost     int            `toml:"cost"`
	Clean    bool           `toml:"clean"`    // paid in clean cash rather than dirty
	Requires []string       `toml:"requires"` // ids that must be owned first
	Desc     string         `toml:"desc"`
	Effects  UpgradeEffects `toml:"effects"`
}

// UpgradeEffects are the named multipliers and deltas a node carries. The
// header of upgrades.toml says how each combines across owned nodes; a
// zero value means the node does not touch that effect.
type UpgradeEffects struct {
	CarryBonus        int     `toml:"carry_bonus"`
	SupplierMul       float64 `toml:"supplier_mul"`
	BuyPressureMul    float64 `toml:"buy_pressure_mul"`
	FillMul           float64 `toml:"fill_mul"`
	SaleHeatMul       float64 `toml:"sale_heat_mul"`
	PatrolCap         float64 `toml:"patrol_cap"`
	CooldownBonus     int     `toml:"cooldown_bonus"`
	StingStockMul     float64 `toml:"sting_stock_mul"`
	RaidLossMul       float64 `toml:"raid_loss_mul"`
	LieLowMultiplier  float64 `toml:"lie_low_multiplier"`
	Decay             float64 `toml:"decay"`
	EvidenceCut       int     `toml:"evidence_cut"`
	EvidenceDecayDays int     `toml:"evidence_decay_days"`
	EvidenceArrest    int     `toml:"evidence_arrest"`
	FallGuy           bool    `toml:"fall_guy"`
}

// HeadlinesConfig mirrors headlines.toml.
type HeadlinesConfig struct {
	FlavourChance float64             `toml:"flavour_chance"`
	Templates     map[string][]string `toml:"templates"`
	Flavour       []string            `toml:"flavour"`
}

// Load parses the embedded TOML files.
func Load() (*Config, error) {
	var c Config
	if err := decode("market.toml", &c.Market); err != nil {
		return nil, err
	}
	if err := decode("city.toml", &c.City); err != nil {
		return nil, err
	}
	if err := decode("heat.toml", &c.Heat); err != nil {
		return nil, err
	}
	if err := decode("crew.toml", &c.Crew); err != nil {
		return nil, err
	}
	if err := decode("rivals.toml", &c.Rivals); err != nil {
		return nil, err
	}
	if err := decode("laundering.toml", &c.Laundering); err != nil {
		return nil, err
	}
	if err := decode("names.toml", &c.Names); err != nil {
		return nil, err
	}
	if err := decode("upgrades.toml", &c.Upgrades); err != nil {
		return nil, err
	}
	if err := decode("reputation.toml", &c.Reputation); err != nil {
		return nil, err
	}
	if err := decode("headlines.toml", &c.Headlines); err != nil {
		return nil, err
	}
	if len(c.Market.Products) == 0 {
		return nil, fmt.Errorf("market.toml: no products defined")
	}
	if len(c.City.Corners) == 0 {
		return nil, fmt.Errorf("city.toml: no corners defined")
	}
	if c.City.Corner(c.City.Territory.Start) == nil {
		return nil, fmt.Errorf("city.toml: start corner %q is not defined", c.City.Territory.Start)
	}
	for _, role := range []string{"runner", "enforcer", "accountant"} {
		if _, ok := c.Crew.Role[role]; !ok {
			return nil, fmt.Errorf("crew.toml: no [role.%s] table", role)
		}
	}
	if len(c.Names.Crew) < c.Crew.Crew.MaxCrew+c.Crew.Crew.Candidates {
		return nil, fmt.Errorf("names.toml: only %d crew names", len(c.Names.Crew))
	}
	for _, p := range Personalities {
		if _, ok := c.Rivals.Personality[p]; !ok {
			return nil, fmt.Errorf("rivals.toml: no [personality.%s] table", p)
		}
	}
	for _, f := range []events.Force{events.ForceWarn, events.ForcePush, events.ForceHit} {
		if _, ok := c.Rivals.Force[f.String()]; !ok {
			return nil, fmt.Errorf("rivals.toml: no [force.%s] table", f)
		}
	}
	if len(c.Names.Rivals) == 0 {
		return nil, fmt.Errorf("names.toml: no rival names")
	}
	if err := c.Upgrades.validate(); err != nil {
		return nil, fmt.Errorf("upgrades.toml: %w", err)
	}
	if len(c.Laundering.Fronts) == 0 {
		return nil, fmt.Errorf("laundering.toml: no fronts defined")
	}
	if r := c.Reputation.Reputation; r.Total <= 0 || r.Band <= 0 || r.Decay < 0 || r.Decay > 1 {
		return nil, fmt.Errorf("reputation.toml: bad [reputation] table %+v", r)
	}
	seen := map[string]bool{}
	for _, f := range c.Laundering.Fronts {
		if f.ID == "" || seen[f.ID] || f.Cost <= 0 || f.Throughput <= 0 {
			return nil, fmt.Errorf("laundering.toml: bad front %+v", f)
		}
		seen[f.ID] = true
	}
	return &c, nil
}

// validate checks the tree hangs together: ids unique, branches known,
// costs positive, and every prerequisite an earlier node (so the tree has
// no cycles and lists in dependency order).
func (u UpgradesConfig) validate() error {
	seen := map[string]bool{}
	for _, n := range u.Nodes {
		if n.ID == "" || n.Name == "" {
			return fmt.Errorf("upgrade %q needs an id and a name", n.ID)
		}
		if seen[n.ID] {
			return fmt.Errorf("upgrade %q is defined twice", n.ID)
		}
		if !slices.Contains(Branches, n.Branch) {
			return fmt.Errorf("upgrade %q: unknown branch %q", n.ID, n.Branch)
		}
		if n.Cost <= 0 {
			return fmt.Errorf("upgrade %q: cost must be positive", n.ID)
		}
		for _, r := range n.Requires {
			if !seen[r] {
				return fmt.Errorf("upgrade %q requires unknown upgrade %q (prerequisites must be defined first)", n.ID, r)
			}
		}
		seen[n.ID] = true
	}
	return nil
}

// MustLoad is Load for tests and main; it panics on error.
func MustLoad() *Config {
	c, err := Load()
	if err != nil {
		panic(err)
	}
	return c
}

func decode(name string, v any) error {
	b, err := files.ReadFile(name)
	if err != nil {
		return err
	}
	md, err := toml.Decode(string(b), v)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	// An effect name nobody reads would silently do nothing.
	if name == "upgrades.toml" || name == "reputation.toml" {
		if keys := md.Undecoded(); len(keys) > 0 {
			return fmt.Errorf("%s: unknown key %s", name, keys[0])
		}
	}
	return nil
}

// Upgrade returns the node with id, or nil.
func (u UpgradesConfig) Upgrade(id string) *UpgradeConfig {
	for i := range u.Nodes {
		if u.Nodes[i].ID == id {
			return &u.Nodes[i]
		}
	}
	return nil
}

// Branch returns the nodes of one branch, in tree order.
func (u UpgradesConfig) Branch(branch string) []UpgradeConfig {
	var out []UpgradeConfig
	for _, n := range u.Nodes {
		if n.Branch == branch {
			out = append(out, n)
		}
	}
	return out
}

// Product returns the config for id, or nil.
func (m MarketConfig) Product(id string) *ProductConfig {
	for i := range m.Products {
		if m.Products[i].ID == id {
			return &m.Products[i]
		}
	}
	return nil
}

// Corner returns the config for id, or nil.
func (c CityConfig) Corner(id string) *CornerConfig {
	for i := range c.Corners {
		if c.Corners[i].ID == id {
			return &c.Corners[i]
		}
	}
	return nil
}

// Front returns the config for id, or nil.
func (l LaunderingConfig) Front(id string) *FrontConfig {
	for i := range l.Fronts {
		if l.Fronts[i].ID == id {
			return &l.Fronts[i]
		}
	}
	return nil
}

// DialFor returns the tuning for a launder dial position.
func (l LaunderingConfig) DialFor(d events.Launder) LaunderConfig {
	switch d {
	case events.LaunderCareful:
		return l.Dial.Careful
	case events.LaunderGreedy:
		return l.Dial.Greedy
	default:
		return l.Dial.Normal
	}
}

// PayFor returns the tuning for a pay dial position.
func (c CrewConfig) PayFor(p events.Pay) PayConfig {
	switch p {
	case events.PayStingy:
		return c.Pay.Stingy
	case events.PayGenerous:
		return c.Pay.Generous
	default:
		return c.Pay.Fair
	}
}
