package content

import (
	"fmt"
	"reflect"
	"slices"
)

// UpgradesConfig mirrors upgrades.toml: the upgrade tree, and the
// fronts' roles (#344) in the same vocabulary, so every sim that folds
// the tree folds an owned front's effects from the copy it already holds.
type UpgradesConfig struct {
	Nodes  []UpgradeConfig `toml:"upgrade"`
	Fronts []FrontRole     `toml:"front"`
}

// FrontRole is what a kind of front does beyond the wash (#344): a line
// for the ledger and the buy picker, and effects in the tree's
// vocabulary that fold beside the owned nodes (game.FoldEffectsIn). Only
// the words in FrontWords may appear: the ones read where a front can
// stand (a city) or run-wide; a word the sims fold from the tree alone
// would do nothing on a front, so the check refuses it.
type FrontRole struct {
	ID      string         `toml:"id"` // a laundering.toml front
	Role    string         `toml:"role"`
	Effects UpgradeEffects `toml:"effects"`
}

// FrontWords are the effect names a front may carry (#344), each with
// where it reaches: "city" for the city the front stands in (a route
// with an end there, its corners and houses, its sales, its law, its
// blocks), "run" for the whole run (the buyers' book and the offshore
// account have no city).
var FrontWords = map[string]string{
	"route_risk_mul":   "city",
	"robbery_mul":      "city",
	"sale_heat_mul":    "city",
	"goodwill_day":     "city",
	"deed_cost_mul":    "city",
	"rent_mul":         "city",
	"buyer_gap_mul":    "run",
	"offshore_fee_mul": "run",
}

// Front returns the role of the front with id, or nil for a front with
// none (its effects are then the identity).
func (u UpgradesConfig) Front(id string) *FrontRole {
	return find(u.Fronts, func(e *FrontRole) bool { return e.ID == id })
}

// Branches an upgrade can belong to, in display order.
var Branches = []string{"operations", "security", "legal", "crew", "laundering", "logistics", "street"}

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

// UpgradeEffects are the named multipliers and deltas a node carries,
// grouped by the sim that reads each (#117): the header of upgrades.toml
// says how each combines across owned nodes (game.FoldEffects); a zero
// value means the node does not touch that effect. Decode refuses a name
// that is not here, so a typo in the file fails at start-up rather than
// doing nothing.
type UpgradeEffects struct {
	// Applied on purchase (game.World.BuyUpgrade).
	CarryBonus int `toml:"carry_bonus"`

	// The market sim.
	SupplierMul          float64 `toml:"supplier_mul"`
	BuyPressureMul       float64 `toml:"buy_pressure_mul"`
	FillMul              float64 `toml:"fill_mul"`
	SaleImpactMul        float64 `toml:"sale_impact_mul"`
	DemandMul            float64 `toml:"demand_mul"`
	GlutDecayMul         float64 `toml:"glut_decay_mul"`
	BuyerGapMul          float64 `toml:"buyer_gap_mul"`
	ContractPremiumBonus float64 `toml:"contract_premium_bonus"`

	// The heat sim.
	SaleHeatMul           float64 `toml:"sale_heat_mul"`
	CrewHeatMul           float64 `toml:"crew_heat_mul"`
	PatrolCap             float64 `toml:"patrol_cap"`
	CooldownBonus         int     `toml:"cooldown_bonus"`
	StingStockMul         float64 `toml:"sting_stock_mul"`
	RaidLossMul           float64 `toml:"raid_loss_mul"`
	LieLowMultiplier      float64 `toml:"lie_low_multiplier"`
	Decay                 float64 `toml:"decay"`
	DirtyCashThresholdMul float64 `toml:"dirty_cash_threshold_mul"`
	EvidenceCut           int     `toml:"evidence_cut"`
	AuditEvidenceCut      int     `toml:"audit_evidence_cut"`
	EvidenceDecayDays     int     `toml:"evidence_decay_days"`
	EvidenceArrest        int     `toml:"evidence_arrest"`
	FallGuys              int     `toml:"fall_guys"`
	Identities            int     `toml:"identities"` // new identities (#49): with one, an indictment past the fall guys is the vanished ending, and Vanish is open

	// The crew sim (#118).
	WageMul            float64 `toml:"wage_mul"`
	LoyaltyLossMul     float64 `toml:"loyalty_loss_mul"`
	DangerLoyaltyMul   float64 `toml:"danger_loyalty_mul"`
	SkimChanceMul      float64 `toml:"skim_chance_mul"`
	InformantChanceMul float64 `toml:"informant_chance_mul"`
	CrewSlots          int     `toml:"crew_slots"`
	CandidatesBonus    int     `toml:"candidates_bonus"`
	PoolDaysCut        int     `toml:"pool_days_cut"`
	SkillBonus         int     `toml:"skill_bonus"`
	HireFeeMul         float64 `toml:"hire_fee_mul"`
	StartLoyaltyBonus  int     `toml:"start_loyalty_bonus"`
	AutoBail           bool    `toml:"auto_bail"` // the bondsman (#230): an arrest is bailed from clean cash the night it lands, when the cash covers it

	// The laundering sim (#118).
	WashMul        float64 `toml:"wash_mul"`
	AuditRiskMul   float64 `toml:"audit_risk_mul"`
	AuditSeizeMul  float64 `toml:"audit_seize_mul"`
	UpkeepMul      float64 `toml:"upkeep_mul"`
	AuditFreezeCut int     `toml:"audit_freeze_cut"`
	FloatMul       float64 `toml:"float_mul"`
	OffshoreFeeMul float64 `toml:"offshore_fee_mul"` // on [offshore] fee (#344)

	// The logistics sim (#119).
	RouteRiskMul     float64 `toml:"route_risk_mul"`
	RouteCapacityMul float64 `toml:"route_capacity_mul"`
	RouteDaysMul     float64 `toml:"route_days_mul"`
	FareMul          float64 `toml:"fare_mul"`
	WholesaleMul     float64 `toml:"wholesale_mul"`

	// The street: the territory and rivals sims (#119).
	DriftDaysBonus int     `toml:"drift_days_bonus"`
	RobberyMul     float64 `toml:"robbery_mul"`
	GuardBonus     int     `toml:"guard_bonus"`
	RivalPushMul   float64 `toml:"rival_push_mul"`

	// The property: the territory sim (#344).
	DeedCostMul float64 `toml:"deed_cost_mul"`
	RentMul     float64 `toml:"rent_mul"`

	// The law sim (#344).
	GoodwillDay float64 `toml:"goodwill_day"` // goodwill points added a day
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
	fronts := map[string]bool{}
	for _, f := range u.Fronts {
		if f.ID == "" || fronts[f.ID] {
			return fmt.Errorf("front role %q needs an id, once", f.ID)
		}
		fronts[f.ID] = true
		if name := f.Effects.outside(FrontWords); name != "" {
			return fmt.Errorf("front role %q: %s is not a word a front carries (FrontWords)", f.ID, name)
		}
	}
	return nil
}

// validateFronts checks every front role names a laundering.toml front:
// the join between the two files, checked after both.
func (u UpgradesConfig) validateFronts(l LaunderingConfig) error {
	for _, f := range u.Fronts {
		if l.Front(f.ID) == nil {
			return fmt.Errorf("front role %q: no such front in laundering.toml", f.ID)
		}
	}
	return nil
}

// Reaching is e with only the words whose FrontWords reach is reach
// ("city" or "run") kept (#344): how the ledger says which of a front's
// effects land in its city and which on the whole run.
func (e UpgradeEffects) Reaching(reach string) UpgradeEffects {
	v := reflect.ValueOf(&e).Elem()
	for i := range v.NumField() {
		if FrontWords[v.Type().Field(i).Tag.Get("toml")] != reach {
			v.Field(i).SetZero()
		}
	}
	return e
}

// outside returns the toml name of the first effect set on e that is
// not in words, or "".
func (e UpgradeEffects) outside(words map[string]string) string {
	v := reflect.ValueOf(e)
	for i := range v.NumField() {
		name := v.Type().Field(i).Tag.Get("toml")
		if !v.Field(i).IsZero() && words[name] == "" {
			return name
		}
	}
	return ""
}

// Upgrade returns the node with id, or nil.
func (u UpgradesConfig) Upgrade(id string) *UpgradeConfig {
	return find(u.Nodes, func(e *UpgradeConfig) bool { return e.ID == id })
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
