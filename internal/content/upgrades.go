package content

import (
	"fmt"
	"slices"
)

// UpgradesConfig mirrors upgrades.toml: the upgrade tree.
type UpgradesConfig struct {
	Nodes []UpgradeConfig `toml:"upgrade"`
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
