package content

import (
	"strings"
	"testing"
)

// The fronts' roles (#344): every front in laundering.toml has a role
// with a line, a role names a front the file has, once, and carries
// only the words a front can carry (FrontWords): a word the sims fold
// from the tree alone would do nothing on a front, so it is refused.
func TestFrontRoles(t *testing.T) {
	cfg := MustLoad()
	for _, f := range cfg.Laundering.Fronts {
		if r := cfg.Upgrades.Front(f.ID); r == nil || r.Role == "" {
			t.Errorf("front %s has no role in upgrades.toml", f.ID)
		}
	}
	for name, reach := range FrontWords {
		if reach != "city" && reach != "run" {
			t.Errorf("%s reaches %q", name, reach)
		}
	}
	for _, c := range []struct {
		name string
		role FrontRole
		want string
	}{
		{"a word no front carries", FrontRole{ID: "laundromat", Effects: UpgradeEffects{WashMul: 1.2}}, "wash_mul is not a word"},
		{"carry", FrontRole{ID: "laundromat", Effects: UpgradeEffects{CarryBonus: 5}}, "carry_bonus is not a word"},
		{"no id", FrontRole{Effects: UpgradeEffects{RobberyMul: 0.9}}, "needs an id"},
	} {
		tree := UpgradesConfig{Fronts: []FrontRole{c.role}}
		if err := tree.validate(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
	}
	twice := UpgradesConfig{Fronts: []FrontRole{{ID: "laundromat"}, {ID: "laundromat"}}}
	if err := twice.validate(); err == nil {
		t.Error("a role defined twice is accepted")
	}
	unknown := UpgradesConfig{Fronts: []FrontRole{{ID: "racetrack"}}}
	if err := unknown.validateFronts(cfg.Laundering); err == nil || !strings.Contains(err.Error(), "racetrack") {
		t.Errorf("a role for no front: %v", err)
	}
	e := UpgradeEffects{RouteRiskMul: 0.75, BuyerGapMul: 0.7}
	if city, run := e.Reaching("city"), e.Reaching("run"); city != (UpgradeEffects{RouteRiskMul: 0.75}) || run != (UpgradeEffects{BuyerGapMul: 0.7}) {
		t.Errorf("reaching splits %+v into %+v and %+v", e, city, run)
	}
}
