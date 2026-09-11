package game

import (
	"fmt"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
)

// snapshot is everything BuyUpgrade may touch, for "the world is untouched
// when it fails".
func snapshot(w *World) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%+v %v %v %v", w.Player, w.Upgrades, w.UpgradesToday, w.FallGuyUsed)
	for id, m := range w.Home().Market {
		fmt.Fprintf(&b, " %s=%.4f", id, m.SupplierPrice)
	}
	return b.String()
}

// Every node refuses a buy without its prerequisites, without the cash,
// or a second time, with a readable error and nothing changed; and buys
// cleanly when everything is in place, from the right pool.
func TestBuyUpgradeTable(t *testing.T) {
	tree := content.MustLoad().Upgrades
	if len(tree.Nodes) == 0 {
		t.Fatal("no upgrades")
	}
	for _, u := range tree.Nodes {
		t.Run(u.ID, func(t *testing.T) {
			w := testWorld()
			pool := func() *int {
				if u.Clean {
					return &w.Player.CleanCash
				}
				return &w.Player.DirtyCash
			}
			// Without prerequisites, with plenty of cash.
			*pool() = u.Cost * 2
			if len(u.Requires) > 0 {
				before := snapshot(w)
				_, err := w.BuyUpgrade(tree, u.ID)
				if err == nil {
					t.Fatal("bought without prerequisites")
				}
				if !strings.Contains(err.Error(), "needs") || !strings.Contains(err.Error(), tree.Upgrade(u.Requires[0]).Name) {
					t.Fatalf("unreadable error: %v", err)
				}
				if snapshot(w) != before {
					t.Fatalf("failed buy changed the world:\n%s\n%s", before, snapshot(w))
				}
			}
			for _, r := range u.Requires {
				w.Upgrades[r] = true
			}
			// Without the cash, or with it in the wrong pool.
			*pool() = u.Cost - 1
			if u.Clean {
				w.Player.DirtyCash = u.Cost * 2
			} else {
				w.Player.CleanCash = u.Cost * 2
			}
			before := snapshot(w)
			if _, err := w.BuyUpgrade(tree, u.ID); err == nil {
				t.Fatal("bought without the cash")
			} else if !strings.Contains(err.Error(), "only have") {
				t.Fatalf("unreadable error: %v", err)
			}
			if snapshot(w) != before {
				t.Fatalf("failed buy changed the world:\n%s\n%s", before, snapshot(w))
			}
			// With everything in place.
			*pool() = u.Cost
			carry, quote := w.Player.CarryLimit, w.Home().Market["a"].SupplierPrice
			got, err := w.BuyUpgrade(tree, u.ID)
			if err != nil || got.ID != u.ID {
				t.Fatalf("buy: %v %+v", err, got)
			}
			if *pool() != 0 || !w.Owns(u.ID) || len(w.UpgradesToday) != 1 || w.UpgradesToday[0] != u.ID {
				t.Fatalf("after buy: cash %d/%d owns %v today %v", w.Player.DirtyCash, w.Player.CleanCash, w.Owns(u.ID), w.UpgradesToday)
			}
			if w.Player.CarryLimit != carry+u.Effects.CarryBonus {
				t.Fatalf("carry %d -> %d, node adds %d", carry, w.Player.CarryLimit, u.Effects.CarryBonus)
			}
			if u.Effects.SupplierMul > 0 && w.Home().Market["a"].SupplierPrice >= quote {
				t.Fatalf("supplier quote %.2f -> %.2f did not drop", quote, w.Home().Market["a"].SupplierPrice)
			}
			// Twice.
			*pool() = u.Cost * 2
			before = snapshot(w)
			if _, err := w.BuyUpgrade(tree, u.ID); err != ErrOwned {
				t.Fatalf("bought twice: %v", err)
			}
			if snapshot(w) != before {
				t.Fatalf("failed buy changed the world:\n%s\n%s", before, snapshot(w))
			}
		})
	}
	w := testWorld()
	if _, err := w.BuyUpgrade(tree, "jetpack"); err != ErrUnknownUpgrade {
		t.Fatalf("bought a node that does not exist: %v", err)
	}
	w.Over = &Ending{Day: 1, Cause: "indicted"}
	w.Player.DirtyCash = 1_000_000
	if _, err := w.BuyUpgrade(tree, "stash"); err != ErrGameOver {
		t.Fatalf("bought after the run ended: %v", err)
	}
}

// The fold uses each effect's own rule: the supplier's discount is
// replaced, not stacked; multipliers multiply; deltas add; an empty world
// is the identity.
func TestFoldEffects(t *testing.T) {
	tree := content.MustLoad().Upgrades
	w := testWorld()
	fx := FoldEffects(w, tree)
	if fx != (Effects{SupplierMul: 1, BuyPressureMul: 1, FillMul: 1, SaleHeatMul: 1, CrewHeatMul: 1, StingStockMul: 1, RaidLossMul: 1}) {
		t.Fatalf("fresh world folds to %+v", fx)
	}
	w.Upgrades["ghosts"], w.Upgrades["cutouts"] = true, true
	if fx := FoldEffects(w, tree); fx.CrewHeatMul < 0.15 || fx.CrewHeatMul > 0.17 {
		t.Fatalf("ghosts and cut-outs: %+v", fx)
	}
	delete(w.Upgrades, "ghosts")
	delete(w.Upgrades, "cutouts")
	w.Upgrades["supplier"] = true
	if fx := FoldEffects(w, tree); fx.SupplierMul != 0.92 || fx.BuyPressureMul != 0.7 {
		t.Fatalf("supplier: %+v", fx)
	}
	w.Upgrades["supplier2"] = true
	if fx := FoldEffects(w, tree); fx.SupplierMul != 0.85 || fx.BuyPressureMul != 0.7 {
		t.Fatalf("supplier2 should replace the discount: %+v", fx)
	}
	for _, id := range []string{"burners", "lookouts", "laylow", "lawyer", "retainer", "fallguy"} {
		w.Upgrades[id] = true
	}
	fx = FoldEffects(w, tree)
	if fx.SaleHeatMul != 0.85 || fx.PatrolCap != 0.8 || fx.CooldownBonus != 2 || fx.StingStockMul != 0.5 ||
		fx.LieLowMultiplier != 3.0 || fx.Decay != 0.13 || fx.EvidenceCut != 1 || fx.EvidenceDecayDays != 30 ||
		fx.EvidenceArrest != 8 || !fx.FallGuy || fx.RaidLossMul != 1 {
		t.Fatalf("security and legal: %+v", fx)
	}
}

func TestSaveKeepsUpgrades(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := testWorld()
	w.Upgrades["stash"] = true
	w.Upgrades["burners"] = true
	w.FallGuyUsed = true
	w.Heat.Evidence = 2
	w.Heat.EvidenceDay = 7
	if err := Save(w); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Owns("stash") || !got.Owns("burners") || got.Owns("lawyer") || !got.FallGuyUsed || got.Heat.Evidence != 2 || got.Heat.EvidenceDay != 7 {
		t.Fatalf("upgrades did not round-trip: %v fallguy %v heat %+v", got.Upgrades, got.FallGuyUsed, got.Heat)
	}
	// A save from before the tree loads with nothing owned and a dated
	// file, and can buy.
	w = testWorld()
	w.Upgrades = nil
	w.Heat.Evidence = 1
	w.Day = 5
	MigrateUpgrades(w)
	if w.Upgrades == nil || len(w.Upgrades) != 0 || w.Heat.EvidenceDay != 5 {
		t.Fatalf("migrated: %v heat %+v", w.Upgrades, w.Heat)
	}
	w.Player.DirtyCash = 1_000_000
	if _, err := w.BuyUpgrade(content.MustLoad().Upgrades, "stash"); err != nil {
		t.Fatal(err)
	}
}
