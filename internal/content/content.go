// Package content loads the tuning data (markets, city, heat, crew, rivals, headlines) that
// lives in TOML files embedded in the binary. Balance changes never need
// code changes.
package content

import (
	"embed"
	"fmt"

	"github.com/BurntSushi/toml"

	"github.com/theclifmeister/kingpin/internal/events"
)

//go:embed *.toml
var files embed.FS

// Config is everything the simulations need to be constructed.
type Config struct {
	Market    MarketConfig
	City      CityConfig
	Heat      HeatConfig
	Crew      CrewConfig
	Rivals    RivalsConfig
	Names     NamesConfig
	Headlines HeadlinesConfig
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
	SloppySkill        int     `toml:"sloppy_skill"`
	SloppyHeat         float64 `toml:"sloppy_heat"`
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
	Crew CrewTuning            `toml:"crew"`
	Pay  PayTable              `toml:"pay"`
	Role map[string]RoleConfig `toml:"role"`
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

// NamesConfig mirrors names.toml.
type NamesConfig struct {
	Crew   []string `toml:"crew"`
	Rivals []string `toml:"rivals"`
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
	if err := decode("names.toml", &c.Names); err != nil {
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
	for _, role := range []string{"runner", "enforcer"} {
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
	return &c, nil
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
	if _, err := toml.Decode(string(b), v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
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
