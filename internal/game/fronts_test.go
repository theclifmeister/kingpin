package game

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
)

// The fronts' roles (#344): a front's effects fold beside the owned
// nodes by the same rules, in the city it stands in alone
// (FoldEffectsIn) or wherever it stands (FoldEffectsAll); a front with
// no role, or a tree with no roles, folds nothing; a front stands where
// it was bought, and one from before the roles (no city) stands at home.
func TestFoldEffectsIn(t *testing.T) {
	tree := content.UpgradesConfig{
		Nodes: []content.UpgradeConfig{{ID: "watch", Name: "Watch", Branch: "street", Cost: 1, Effects: content.UpgradeEffects{RobberyMul: 0.5}}},
		Fronts: []content.FrontRole{
			{ID: "diner", Effects: content.UpgradeEffects{RobberyMul: 0.8, GoodwillDay: 0.5}},
			{ID: "club", Effects: content.UpgradeEffects{BuyerGapMul: 0.7}},
		},
	}
	w := testWorld()
	w.AddCity(StartingCity{ID: "far", Name: "Far", HeatMul: 1})
	w.Upgrades["watch"] = true
	if fx := FoldEffectsIn(w, tree, "test"); fx != FoldEffects(w, tree) {
		t.Fatalf("no front folds %+v", fx)
	}
	w.Player.DirtyCash = 1_000_000
	if _, err := w.BuyFront(FrontOffer{ID: "diner", Name: "Diner", Cost: 1}); err != nil {
		t.Fatal(err)
	}
	if f := w.Front("diner"); f.City != "test" || w.FrontCity(*f) != "test" {
		t.Fatalf("the diner stands in %q", f.City)
	}
	w.Fronts = append(w.Fronts, Front{ID: "club"}, Front{ID: "laundromat", City: "far"})
	if w.FrontCity(*w.Front("club")) != "test" {
		t.Fatal("a front from before the roles does not stand at home")
	}
	home := FoldEffectsIn(w, tree, "test")
	if math.Abs(home.RobberyMul-0.4) > 1e-9 || home.GoodwillDay != 0.5 || home.BuyerGapMul != 0.7 {
		t.Fatalf("home folds %+v", home)
	}
	if far := FoldEffectsIn(w, tree, "far"); far != FoldEffects(w, tree) {
		t.Fatalf("the far city folds home's fronts: %+v", far)
	}
	if both := FoldEffectsIn(w, tree, "far", "test"); both != home {
		t.Fatalf("a road with an end at home folds %+v, want %+v", both, home)
	}
	w.Fronts[0].City = "far"
	if all := FoldEffectsAll(w, tree); math.Abs(all.RobberyMul-0.4) > 1e-9 || all.BuyerGapMul != 0.7 {
		t.Fatalf("every front folds %+v", all)
	}
	tree.Fronts = nil
	if all := FoldEffectsAll(w, tree); all != FoldEffects(w, tree) {
		t.Fatalf("no roles fold %+v", all)
	}
}
