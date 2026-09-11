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
	w.UpgradesToday = append(w.UpgradesToday, id)
	after := FoldEffects(w, tree)

	// Deltas to the player's own state land now; multipliers the sims
	// read fold from the owned set every step, so only today's supplier
	// quote needs bringing into line.
	w.Player.CarryLimit += u.Effects.CarryBonus
	if after.SupplierMul != before.SupplierMul {
		for _, c := range w.Cities {
			for _, m := range c.Market {
				m.SupplierPrice *= after.SupplierMul / before.SupplierMul
			}
		}
	}
	return *u, nil
}

// Effects is what the owned upgrades add up to, in the terms each sim
// multiplies its own tuning by (FoldEffects). A fresh world folds to the identity: every
// multiplier 1, every replacement 0 (use the config value), nothing owned.
type Effects struct {
	SupplierMul       float64 // on the supplier's price
	BuyPressureMul    float64 // on how much a buy pushes the supplier price
	FillMul           float64 // on every dial's fill
	SaleHeatMul       float64 // on the heat a sale draws
	CrewHeatMul       float64 // on the heat a unit a runner moves draws, relative to one you move
	PatrolCap         float64 // patrol sell cap, 0 = the response's own
	CooldownBonus     int     // days added before the same response can fire again
	StingStockMul     float64 // on stock a sting takes
	RaidLossMul       float64 // on stock and cash a raid takes
	LieLowMultiplier  float64 // lie-low decay multiplier, 0 = heat.toml's
	Decay             float64 // base heat decay, 0 = heat.toml's
	EvidenceCut       int     // evidence knocked off every sting and raid
	EvidenceDecayDays int     // days without new evidence before a point goes cold, 0 = never
	EvidenceArrest    int     // evidence that is an indictment, 0 = heat.toml's
	FallGuy           bool    // somebody else takes the first fall
}

// FoldEffects folds the upgrades w owns into one Effects, using the combining
// rule upgrades.toml documents for each name.
func FoldEffects(w *World, tree content.UpgradesConfig) Effects {
	fx := Effects{SupplierMul: 1, BuyPressureMul: 1, FillMul: 1, SaleHeatMul: 1, CrewHeatMul: 1, StingStockMul: 1, RaidLossMul: 1}
	for _, n := range tree.Nodes {
		if !w.Owns(n.ID) {
			continue
		}
		e := n.Effects
		if e.SupplierMul > 0 {
			fx.SupplierMul = math.Min(fx.SupplierMul, e.SupplierMul)
		}
		if e.BuyPressureMul > 0 {
			fx.BuyPressureMul *= e.BuyPressureMul
		}
		if e.FillMul > 0 {
			fx.FillMul *= e.FillMul
		}
		if e.SaleHeatMul > 0 {
			fx.SaleHeatMul *= e.SaleHeatMul
		}
		if e.CrewHeatMul > 0 {
			fx.CrewHeatMul *= e.CrewHeatMul
		}
		fx.PatrolCap = math.Max(fx.PatrolCap, e.PatrolCap)
		fx.CooldownBonus += e.CooldownBonus
		if e.StingStockMul > 0 {
			fx.StingStockMul *= e.StingStockMul
		}
		if e.RaidLossMul > 0 {
			fx.RaidLossMul *= e.RaidLossMul
		}
		fx.LieLowMultiplier = math.Max(fx.LieLowMultiplier, e.LieLowMultiplier)
		fx.Decay = math.Max(fx.Decay, e.Decay)
		fx.EvidenceCut += e.EvidenceCut
		if e.EvidenceDecayDays > 0 && (fx.EvidenceDecayDays == 0 || e.EvidenceDecayDays < fx.EvidenceDecayDays) {
			fx.EvidenceDecayDays = e.EvidenceDecayDays
		}
		fx.EvidenceArrest = max(fx.EvidenceArrest, e.EvidenceArrest)
		fx.FallGuy = fx.FallGuy || e.FallGuy
	}
	return fx
}

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
