package game

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
)

var (
	ErrUnknownUpgrade = errors.New("no such upgrade")
	ErrOwned          = errors.New("you already have that")
)

// Owns reports whether the player has bought the upgrade with id.
func (w *World) Owns(id string) bool { return w.Upgrades[id] }

// Missing lists the prerequisites of an upgrade the player does not own
// yet, in tree order.
func (w *World) Missing(u content.UpgradeConfig) []string {
	var out []string
	for _, r := range u.Requires {
		if !w.Owns(r) {
			out = append(out, r)
		}
	}
	return out
}

// BuyUpgrade buys the node with id from the tree: it must exist, not be
// owned, have its prerequisites owned, and be affordable from the right
// cash pool. It takes effect at once and stays for the run; the market sim
// reports it at end of day. The world is untouched when it fails.
func (w *World) BuyUpgrade(tree content.UpgradesConfig, id string) (content.UpgradeConfig, error) {
	if w.Over != nil {
		return content.UpgradeConfig{}, ErrGameOver
	}
	u := tree.Upgrade(id)
	if u == nil {
		return content.UpgradeConfig{}, ErrUnknownUpgrade
	}
	if w.Owns(id) {
		return *u, ErrOwned
	}
	if missing := w.Missing(*u); len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for _, m := range missing {
			names = append(names, tree.Upgrade(m).Name)
		}
		return *u, fmt.Errorf("%s needs %s first", u.Name, strings.Join(names, " and "))
	}
	pool, have := "dirty", w.Player.DirtyCash
	if u.Clean {
		pool, have = "clean", w.Player.CleanCash
	}
	if u.Cost > have {
		return *u, fmt.Errorf("%s costs $%d %s, only have $%d %s", u.Name, u.Cost, pool, have, pool)
	}

	before := FoldEffects(w, tree)
	if u.Clean {
		w.Player.CleanCash -= u.Cost
	} else {
		w.Player.DirtyCash -= u.Cost
	}
	if w.Upgrades == nil {
		w.Upgrades = map[string]bool{}
	}
	w.Upgrades[id] = true
	w.Today.UpgradesToday = append(w.Today.UpgradesToday, id)
	after := FoldEffects(w, tree)

	// Deltas to the player's own state land now; multipliers the sims
	// read fold from the owned set every step, so only today's supplier
	// quote needs bringing into line.
	w.Player.CarryLimit += u.Effects.CarryBonus
	if after.SupplierMul != before.SupplierMul {
		for i := range w.Suppliers {
			for id := range w.Suppliers[i].Price {
				w.Suppliers[i].Price[id] *= after.SupplierMul / before.SupplierMul
			}
		}
		for _, c := range w.Cities {
			for _, m := range c.Market {
				m.SupplierPrice *= after.SupplierMul / before.SupplierMul
			}
		}
	}
	return *u, nil
}

// Effects is what the owned upgrades add up to, in the terms each sim
// multiplies its own tuning by (FoldEffects), grouped by the sim that
// reads each (#117). A fresh world folds to the identity: every
// multiplier 1, every bonus and cut 0, every replacement 0 (use the
// config value), nothing owned. Four rules combine two owned nodes that
// carry the same name, fixed per name in the upgrades.toml header:
// a product stacks, a sum adds, the lowest owned wins (a discount is
// replaced, never stacked, so a multiplier over 1 there is ignored) and
// the highest owned wins (a replacement, or a multiplier a node only
// ever raises).
type Effects struct {
	// The market sim.
	SupplierMul          float64 // on the supplier's price (lowest)
	BuyPressureMul       float64 // on how much a buy pushes the supplier price (product)
	FillMul              float64 // on every dial's fill (product)
	SaleImpactMul        float64 // on the price impact of a sale (product)
	DemandMul            float64 // on what your worked corners serve (product)
	GlutDecayMul         float64 // on how fast a glut clears (highest)
	BuyerGapMul          float64 // on the buyers' min and max gap between offers (lowest)
	ContractPremiumBonus float64 // added to every contract's premium (sum)

	// The heat sim.
	SaleHeatMul           float64 // on the heat a sale draws (product)
	CrewHeatMul           float64 // on the heat a unit a runner moves draws, relative to one you move (product)
	PatrolCap             float64 // patrol sell cap, 0 = the response's own (highest)
	CooldownBonus         int     // days added before the same response can fire again (sum)
	StingStockMul         float64 // on stock a sting takes (product)
	RaidLossMul           float64 // on stock and cash a raid takes (product)
	LieLowMultiplier      float64 // lie-low decay multiplier, 0 = heat.toml's (highest)
	Decay                 float64 // base heat decay, 0 = heat.toml's (highest)
	DirtyCashThresholdMul float64 // on the dirty cash the police read nothing into (highest)
	EvidenceCut           int     // evidence knocked off every sting and raid (sum)
	AuditEvidenceCut      int     // evidence knocked off a greedy front's audit (sum)
	EvidenceDecayDays     int     // days without new evidence before a point goes cold, 0 = never (lowest)
	EvidenceArrest        int     // evidence that is an indictment, 0 = heat.toml's (highest)
	FallGuys              int     // how many indictments somebody else takes, one each (sum)

	// The crew sim (#118).
	WageMul            float64 // on the wage bill (lowest)
	LoyaltyLossMul     float64 // on the daily loyalty loss: greed drift, danger, unpaid (product)
	DangerLoyaltyMul   float64 // on what a robbery or a strike costs in loyalty (product)
	SkimChanceMul      float64 // on the chance a member under the line skims (product)
	InformantChanceMul float64 // on the chance a member under the line turns (product)
	CrewSlots          int     // added to the roster's size (sum)
	CandidatesBonus    int     // added to the hiring pool (sum)
	PoolDaysCut        int     // days off the pool's rotation, never under 1 (sum)
	SkillBonus         int     // added to a generated candidate's skill (sum)
	HireFeeMul         float64 // on a candidate's signing fee (lowest)
	StartLoyaltyBonus  int     // added to a generated candidate's loyalty (sum)

	// The laundering sim (#118).
	WashMul        float64 // on every front's throughput (product)
	AuditRiskMul   float64 // on every front's audit risk (product)
	AuditSeizeMul  float64 // on what an audit seizes (lowest)
	UpkeepMul      float64 // on every front's upkeep (lowest)
	AuditFreezeCut int     // days off an audit's freeze, never under 1 (sum)
	FloatMul       float64 // on the float the wash, the road and a supply contract leave in the till (lowest)

	// The logistics sim (#119).
	RouteRiskMul     float64 // on every route's risk of interception (product)
	RouteCapacityMul float64 // on every route's capacity per shipment (product)
	RouteDaysMul     float64 // on every route's days in transit, never under one day (lowest)
	FareMul          float64 // on every route's fare per unit (lowest)
	WholesaleMul     float64 // on the wholesaler's price over street supply (lowest)

	// The street: the territory and rivals sims (#119).
	DriftDaysBonus int     // days added before a held corner nobody works drifts (sum)
	RobberyMul     float64 // on every corner's robbery chance (product)
	GuardBonus     int     // an extra body on every contested corner (sum)
	RivalPushMul   float64 // on the rival's push chance (product)
}

// identity is the fold of nothing owned: every multiplier 1, everything
// else 0.
var identity = Effects{
	SupplierMul: 1, BuyPressureMul: 1, FillMul: 1, SaleImpactMul: 1, DemandMul: 1, GlutDecayMul: 1, BuyerGapMul: 1,
	SaleHeatMul: 1, CrewHeatMul: 1, StingStockMul: 1, RaidLossMul: 1, DirtyCashThresholdMul: 1,
	WageMul: 1, LoyaltyLossMul: 1, DangerLoyaltyMul: 1, SkimChanceMul: 1, InformantChanceMul: 1, HireFeeMul: 1,
	WashMul: 1, AuditRiskMul: 1, AuditSeizeMul: 1, UpkeepMul: 1, FloatMul: 1,
	RouteRiskMul: 1, RouteCapacityMul: 1, RouteDaysMul: 1, FareMul: 1, WholesaleMul: 1,
	RobberyMul: 1, RivalPushMul: 1,
}

// FoldEffects folds the upgrades w owns into one Effects, using the
// combining rule upgrades.toml documents for each name. Every node is
// folded in file order, so a product or a sum is the same whatever the
// order the nodes were bought in.
func FoldEffects(w *World, tree content.UpgradesConfig) Effects {
	fx := identity
	// product: a multiplier stacks with every other owned node's.
	product := func(acc *float64, v float64) {
		if v > 0 {
			*acc *= v
		}
	}
	// lowest: the best discount owned is the one that counts; a value
	// over the identity is ignored.
	lowest := func(acc *float64, v float64) {
		if v > 0 {
			*acc = math.Min(*acc, v)
		}
	}
	// highest: a replacement or a raise; the biggest owned wins.
	highest := func(acc *float64, v float64) { *acc = math.Max(*acc, v) }
	for _, n := range tree.Nodes {
		if !w.Owns(n.ID) {
			continue
		}
		e := n.Effects

		// The market.
		lowest(&fx.SupplierMul, e.SupplierMul)
		product(&fx.BuyPressureMul, e.BuyPressureMul)
		product(&fx.FillMul, e.FillMul)
		product(&fx.SaleImpactMul, e.SaleImpactMul)
		product(&fx.DemandMul, e.DemandMul)
		highest(&fx.GlutDecayMul, e.GlutDecayMul)
		lowest(&fx.BuyerGapMul, e.BuyerGapMul)
		fx.ContractPremiumBonus += e.ContractPremiumBonus

		// Heat.
		product(&fx.SaleHeatMul, e.SaleHeatMul)
		product(&fx.CrewHeatMul, e.CrewHeatMul)
		highest(&fx.PatrolCap, e.PatrolCap)
		fx.CooldownBonus += e.CooldownBonus
		product(&fx.StingStockMul, e.StingStockMul)
		product(&fx.RaidLossMul, e.RaidLossMul)
		highest(&fx.LieLowMultiplier, e.LieLowMultiplier)
		highest(&fx.Decay, e.Decay)
		highest(&fx.DirtyCashThresholdMul, e.DirtyCashThresholdMul)
		fx.EvidenceCut += e.EvidenceCut
		fx.AuditEvidenceCut += e.AuditEvidenceCut
		if e.EvidenceDecayDays > 0 && (fx.EvidenceDecayDays == 0 || e.EvidenceDecayDays < fx.EvidenceDecayDays) {
			fx.EvidenceDecayDays = e.EvidenceDecayDays
		}
		fx.EvidenceArrest = max(fx.EvidenceArrest, e.EvidenceArrest)
		fx.FallGuys += e.FallGuys

		// The crew.
		lowest(&fx.WageMul, e.WageMul)
		product(&fx.LoyaltyLossMul, e.LoyaltyLossMul)
		product(&fx.DangerLoyaltyMul, e.DangerLoyaltyMul)
		product(&fx.SkimChanceMul, e.SkimChanceMul)
		product(&fx.InformantChanceMul, e.InformantChanceMul)
		fx.CrewSlots += e.CrewSlots
		fx.CandidatesBonus += e.CandidatesBonus
		fx.PoolDaysCut += e.PoolDaysCut
		fx.SkillBonus += e.SkillBonus
		lowest(&fx.HireFeeMul, e.HireFeeMul)
		fx.StartLoyaltyBonus += e.StartLoyaltyBonus

		// Laundering.
		product(&fx.WashMul, e.WashMul)
		product(&fx.AuditRiskMul, e.AuditRiskMul)
		lowest(&fx.AuditSeizeMul, e.AuditSeizeMul)
		lowest(&fx.UpkeepMul, e.UpkeepMul)
		fx.AuditFreezeCut += e.AuditFreezeCut
		lowest(&fx.FloatMul, e.FloatMul)

		// Logistics.
		product(&fx.RouteRiskMul, e.RouteRiskMul)
		product(&fx.RouteCapacityMul, e.RouteCapacityMul)
		lowest(&fx.RouteDaysMul, e.RouteDaysMul)
		lowest(&fx.FareMul, e.FareMul)
		lowest(&fx.WholesaleMul, e.WholesaleMul)

		// The street.
		fx.DriftDaysBonus += e.DriftDaysBonus
		product(&fx.RobberyMul, e.RobberyMul)
		fx.GuardBonus += e.GuardBonus
		product(&fx.RivalPushMul, e.RivalPushMul)
	}
	return fx
}

// Float is the dirty cash the wash, the road and a supply contract
// (#113) leave in the till: base, laundering.toml's float, folded by the
// tree (float_mul, the lowest owned wins). It is the one place the
// folded float is computed, so the three readers cannot disagree about
// where the street's restock money starts; each passes its own copy of
// the tree and the base, the way every sim folds for itself, and a
// world owning no float node reads the base exactly (#118).
func (w *World) Float(tree content.UpgradesConfig, base int) int {
	return int(math.Round(float64(base) * FoldEffects(w, tree).FloatMul))
}

// FallGuyLeft reports whether one of the fall guys the tree gives has
// not taken his fall yet: the owned count over what FallsTaken records.
func (w *World) FallGuyLeft(fx Effects) bool { return w.FallsTaken < fx.FallGuys }

// MigrateUpgrades brings a save from before the upgrade tree up to date:
// nothing owned, and the DA's file dated today so a retained lawyer's
// clock starts from here rather than from day 0.
func MigrateUpgrades(w *World) {
	if w.Upgrades == nil {
		w.Upgrades = map[string]bool{}
	}
	if w.Heat.Evidence > 0 && w.Heat.EvidenceDay == 0 {
		w.Heat.EvidenceDay = w.Day
	}
}
