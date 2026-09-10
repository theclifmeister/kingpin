// Package content loads the tuning data (markets, cities, routes, heat,
// crew, rivals, upgrades, laundering, reputation, headlines, dilemmas) that
// lives in TOML files embedded in the binary. Balance changes never need
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
	Routes     RoutesConfig
	Heat       HeatConfig
	Crew       CrewConfig
	Rivals     RivalsConfig
	Laundering LaunderingConfig
	Names      NamesConfig
	Upgrades   UpgradesConfig
	Reputation ReputationConfig
	Headlines  HeadlinesConfig
	Dilemmas   DilemmasConfig
}

// MarketConfig mirrors market.toml.
type MarketConfig struct {
	Market   MarketTuning    `toml:"market"`
	Products []ProductConfig `toml:"product"`
	Dial     DialTable       `toml:"dial"`
}

type MarketTuning struct {
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

// CityConfig mirrors city.toml: the cities, each with its own corners and
// its own take on every product, and how corners are held. The first city
// is home: where a run starts and where the rival sets up.
type CityConfig struct {
	Territory TerritoryTuning `toml:"territory"`
	Cities    []CityEntry     `toml:"city"`
}

// CityEntry is one city. Heat multiplies the sale heat of every unit
// moved there (a port town's police have other things to look at);
// Wholesale says its supplier sells by the lot (routes.toml [wholesale]);
// Market is how its street differs from the product ladder, per product.
type CityEntry struct {
	ID        string                 `toml:"id"`
	Name      string                 `toml:"name"`
	Heat      float64                `toml:"heat"`
	Wholesale bool                   `toml:"wholesale"`
	Market    map[string]CityProduct `toml:"market"`
	Corners   []CornerConfig         `toml:"corner"`
}

// CityProduct is a city's multipliers on a product's base price and
// per-corner demand; a product the city does not list is at 1 and 1.
type CityProduct struct {
	Price  float64 `toml:"price"`
	Demand float64 `toml:"demand"`
}

// Product returns the city's multipliers for a product, 1 and 1 when it
// has no view of it.
func (c CityEntry) Product(id string) CityProduct {
	if p, ok := c.Market[id]; ok {
		if p.Price <= 0 {
			p.Price = 1
		}
		if p.Demand <= 0 {
			p.Demand = 1
		}
		return p
	}
	return CityProduct{Price: 1, Demand: 1}
}

// HeatMul is the city's sale-heat multiplier, 1 when unset.
func (c CityEntry) HeatMul() float64 {
	if c.Heat <= 0 {
		return 1
	}
	return c.Heat
}

// Corner returns the city's corner with id, or nil.
func (c CityEntry) Corner(id string) *CornerConfig {
	for i := range c.Corners {
		if c.Corners[i].ID == id {
			return &c.Corners[i]
		}
	}
	return nil
}

// RoutesConfig mirrors routes.toml: the edges between cities, the ship
// dial, what a seizure does and how the wholesale supplier sells.
type RoutesConfig struct {
	Shipping  ShippingTuning  `toml:"shipping"`
	Wholesale WholesaleTuning `toml:"wholesale"`
	Dial      ShipDialTable   `toml:"dial"`
	Routes    []RouteConfig   `toml:"route"`
}

// ShippingTuning is what a seizure does to the world: heat in both cities
// on the route, a page in the DA's file if the shipment was sent fast, and
// a supply shock on the product in the city it was bound for.
type ShippingTuning struct {
	SeizureHeat     float64 `toml:"seizure_heat"`
	SeizureEvidence int     `toml:"seizure_evidence"`
	ShockFactor     float64 `toml:"shock_factor"`
	ShockDays       int     `toml:"shock_days"`
	RecordDays      int     `toml:"record_days"` // how long a seizure stays on the ledger
}

// WholesaleTuning is the lot the wholesale supplier sells by, at what
// fraction of the street supplier's price, and the peak cash that opens
// the door.
type WholesaleTuning struct {
	Lot        int     `toml:"lot"`
	Mul        float64 `toml:"mul"`
	UnlockCash int     `toml:"unlock_cash"`
}

type ShipDialTable struct {
	Slow   ShipDialConfig `toml:"slow"`
	Normal ShipDialConfig `toml:"normal"`
	Fast   ShipDialConfig `toml:"fast"`
}

// ShipDialConfig scales a route: Days multiplies the days in transit,
// Risk the chance of interception on each of them.
type ShipDialConfig struct {
	Days float64 `toml:"days"`
	Risk float64 `toml:"risk"`
}

// RouteConfig is one edge of the route graph, usable both ways. Days is
// the time in transit at the normal dial, Capacity the most one shipment
// carries, Cost what every unit costs to send (dirty cash), Risk the
// chance per day in transit that the shipment is intercepted.
type RouteConfig struct {
	ID       string  `toml:"id"`
	Name     string  `toml:"name"`
	Mode     string  `toml:"mode"` // car, truck, boat
	From     string  `toml:"from"`
	To       string  `toml:"to"`
	Days     int     `toml:"days"`
	Capacity int     `toml:"capacity"`
	Cost     int     `toml:"cost"`
	Risk     float64 `toml:"risk"`
}

// Connects reports whether the route joins the two cities, either way.
func (r RouteConfig) Connects(a, b string) bool {
	return (r.From == a && r.To == b) || (r.From == b && r.To == a)
}

// Other is the city at the far end of the route from city, or "".
func (r RouteConfig) Other(city string) string {
	switch city {
	case r.From:
		return r.To
	case r.To:
		return r.From
	}
	return ""
}

// Route returns the route with id, or nil.
func (r RoutesConfig) Route(id string) *RouteConfig {
	for i := range r.Routes {
		if r.Routes[i].ID == id {
			return &r.Routes[i]
		}
	}
	return nil
}

// DialFor returns the tuning for a ship dial position.
func (r RoutesConfig) DialFor(d events.Ship) ShipDialConfig {
	switch d {
	case events.ShipSlow:
		return r.Dial.Slow
	case events.ShipFast:
		return r.Dial.Fast
	default:
		return r.Dial.Normal
	}
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
	Diplomacy   DiplomacyTuning              `toml:"diplomacy"`
	Deal        map[string]DealConfig        `toml:"deal"`
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
	Trust           float64 `toml:"trust"`        // trust in the player at the start of a run
	DealBias        float64 `toml:"deal_bias"`    // added to the chance it accepts any proposal
	Betrayal        float64 `toml:"betrayal"`     // chance per live deal per day it breaks one itself
	OfferChance     float64 `toml:"offer_chance"` // chance per day it puts a deal on the table when its situation calls for one
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
	Trust   float64 `toml:"trust"` // what a strike at this force costs the rival's trust in you
}

// DiplomacyTuning is the table: how deals are offered, judged, kept and
// broken. The rival's answer is a chance built from a base per deal kind,
// the terms asked, its trust, its personality and the war, plus what the
// player's reputation adds (reputation.toml [effects]).
type DiplomacyTuning struct {
	OfferDays      int       `toml:"offer_days"`      // days a rival offer stays on the table
	TrustKept      float64   `toml:"trust_kept"`      // trust per day of a live deal
	AcceptTrust    float64   `toml:"accept_trust"`    // added to the chance at trust 100
	AcceptWar      float64   `toml:"accept_war"`      // added to the chance at war 100: a loud war makes peace attractive
	BetrayalFloor  float64   `toml:"betrayal_floor"`  // trust after the player breaks a deal
	DistrustDays   int       `toml:"distrust_days"`   // days after a betrayal the rival takes no deal and offers none
	BetrayalSpread float64   `toml:"betrayal_spread"` // trust every other faction loses (Phase 4)
	JointTrust     float64   `toml:"joint_trust"`     // trust a joint shipment needs (#30)
	TruceDays      []int     `toml:"truce_days"`      // the three lengths a truce can be proposed at
	TributeCuts    []float64 `toml:"tribute_cuts"`    // the three cuts of the player's daily street value a tribute can be
	TributeMin     int       `toml:"tribute_min"`     // a tribute is never under this a day
	LowCashDays    int       `toml:"low_cash_days"`   // an expansionist that cannot pay its muscle this long offers a truce
	UpperHand      float64   `toml:"upper_hand"`      // an opportunist with this many times the muscle on the front line demands tribute
	SplitFair      float64   `toml:"split_fair"`      // share of the city's demand the rival lets the player's side of a split have at trust 0 ...
	SplitTrust     float64   `toml:"split_trust"`     // ... plus this much at trust 100
}

// DealConfig is what the rival thinks of one deal kind: the base chance
// it accepts, and how much the terms move it (the easy option adds terms,
// the hard one takes it away).
type DealConfig struct {
	Base  float64 `toml:"base"`
	Terms float64 `toml:"terms"`
}

// Personalities are the rival personalities in a fixed order, so a pick
// by seed is reproducible.
var Personalities = []string{"expansionist", "defensive", "opportunist", "chaotic"}

// ForceFor returns the tuning for a force dial position.
func (r RivalsConfig) ForceFor(f events.Force) ForceConfig { return r.Force[f.String()] }

// DealKinds are the deals that can be proposed, in the order the UI
// lists them. The joint shipment waits on routes (#30).
var DealKinds = []string{"truce", "tribute", "split"}

// validate checks the diplomacy table reads as one: a [deal.X] table per
// kind, three options for the truce and the tribute, and sane days.
func (r RivalsConfig) validate() error {
	d := r.Diplomacy
	for _, k := range DealKinds {
		if _, ok := r.Deal[k]; !ok {
			return fmt.Errorf("no [deal.%s] table", k)
		}
	}
	if len(d.TruceDays) != 3 || len(d.TributeCuts) != 3 {
		return fmt.Errorf("truce_days and tribute_cuts need three options each, got %d and %d", len(d.TruceDays), len(d.TributeCuts))
	}
	if d.OfferDays < 1 || d.DistrustDays < 1 {
		return fmt.Errorf("offer_days %d and distrust_days %d must be positive", d.OfferDays, d.DistrustDays)
	}
	return nil
}

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
	DealKept    float64 `toml:"deal_kept"` // per day of a truce or split kept
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
	FearDeal           float64 `toml:"fear_deal"`     // rivals: added to the chance a proposal is accepted
	RespectTrust       float64 `toml:"respect_trust"` // rivals: extra on the trust a kept deal-day earns
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
	for i := range d.Cards {
		if d.Cards[i].ID == id {
			return &d.Cards[i]
		}
	}
	return nil
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

// Load parses the embedded TOML files.
func Load() (*Config, error) {
	var c Config
	if err := decode("market.toml", &c.Market); err != nil {
		return nil, err
	}
	if err := decode("city.toml", &c.City); err != nil {
		return nil, err
	}
	if err := decode("routes.toml", &c.Routes); err != nil {
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
	if err := decode("dilemmas.toml", &c.Dilemmas); err != nil {
		return nil, err
	}
	if len(c.Market.Products) == 0 {
		return nil, fmt.Errorf("market.toml: no products defined")
	}
	if err := c.City.validate(); err != nil {
		return nil, fmt.Errorf("city.toml: %w", err)
	}
	if err := c.Routes.validate(c.City); err != nil {
		return nil, fmt.Errorf("routes.toml: %w", err)
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
	if err := c.Rivals.validate(); err != nil {
		return nil, fmt.Errorf("rivals.toml: %w", err)
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
	if err := c.Dilemmas.validate(); err != nil {
		return nil, fmt.Errorf("dilemmas.toml: %w", err)
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
	// An effect name nobody reads, or a trigger field nobody checks, would
	// silently do nothing.
	if name == "upgrades.toml" || name == "reputation.toml" || name == "dilemmas.toml" || name == "routes.toml" {
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

// Corner returns the config for a corner id in any city, or nil.
func (c CityConfig) Corner(id string) *CornerConfig {
	for i := range c.Cities {
		if k := c.Cities[i].Corner(id); k != nil {
			return k
		}
	}
	return nil
}

// City returns the city with id, or nil.
func (c CityConfig) City(id string) *CityEntry {
	for i := range c.Cities {
		if c.Cities[i].ID == id {
			return &c.Cities[i]
		}
	}
	return nil
}

// Home is the first city: where a run starts.
func (c CityConfig) Home() CityEntry { return c.Cities[0] }

// Corners lists every corner of every city, in file order.
func (c CityConfig) Corners() []CornerConfig {
	var out []CornerConfig
	for _, city := range c.Cities {
		out = append(out, city.Corners...)
	}
	return out
}

// validate checks the cities read as a map: at least one, ids unique,
// every one with corners, corner ids unique across the lot, and the start
// corner in the home city.
func (c CityConfig) validate() error {
	if len(c.Cities) == 0 {
		return fmt.Errorf("no cities defined")
	}
	cities := map[string]bool{}
	corners := map[string]bool{}
	for _, city := range c.Cities {
		if city.ID == "" || city.Name == "" {
			return fmt.Errorf("city %q needs an id and a name", city.ID)
		}
		if cities[city.ID] {
			return fmt.Errorf("city %q is defined twice", city.ID)
		}
		cities[city.ID] = true
		if len(city.Corners) == 0 {
			return fmt.Errorf("city %q has no corners", city.ID)
		}
		for _, k := range city.Corners {
			if k.ID == "" || corners[k.ID] {
				return fmt.Errorf("city %q: corner %q missing or defined twice", city.ID, k.ID)
			}
			corners[k.ID] = true
		}
	}
	if c.Home().Corner(c.Territory.Start) == nil {
		return fmt.Errorf("start corner %q is not in %s", c.Territory.Start, c.Home().ID)
	}
	return nil
}

// validate checks the routes join cities that exist, the numbers make
// sense, and the wholesale lot and the dials are usable.
func (r RoutesConfig) validate(cities CityConfig) error {
	seen := map[string]bool{}
	for _, rt := range r.Routes {
		if rt.ID == "" || seen[rt.ID] {
			return fmt.Errorf("route %q missing or defined twice", rt.ID)
		}
		seen[rt.ID] = true
		if cities.City(rt.From) == nil || cities.City(rt.To) == nil || rt.From == rt.To {
			return fmt.Errorf("route %q joins %q and %q", rt.ID, rt.From, rt.To)
		}
		if rt.Days < 1 || rt.Capacity < 1 || rt.Cost < 0 || rt.Risk < 0 || rt.Risk > 1 || rt.Mode == "" {
			return fmt.Errorf("bad route %+v", rt)
		}
	}
	if r.Wholesale.Lot < 1 || r.Wholesale.Mul <= 0 {
		return fmt.Errorf("bad [wholesale] table %+v", r.Wholesale)
	}
	for _, d := range []ShipDialConfig{r.Dial.Slow, r.Dial.Normal, r.Dial.Fast} {
		if d.Days <= 0 || d.Risk < 0 {
			return fmt.Errorf("bad ship dial %+v", d)
		}
	}
	if r.Shipping.ShockDays < 0 || r.Shipping.ShockFactor < 0 {
		return fmt.Errorf("bad [shipping] table %+v", r.Shipping)
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
