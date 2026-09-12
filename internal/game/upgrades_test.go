package game

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
)

// snapshot is everything BuyUpgrade may touch, for "the world is untouched
// when it fails".
func snapshot(w *World) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%+v %v %v %v", w.Player, w.Upgrades, w.UpgradesToday, w.FallsTaken)
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
func TestFoldEffectsOnTheTree(t *testing.T) {
	tree := content.MustLoad().Upgrades
	w := testWorld()
	fx := FoldEffects(w, tree)
	if fx != identity {
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
	for _, id := range []string{"burners", "lookouts", "laylow", "lawyer", "paper", "retainer", "judge", "fallguy", "fallguy2"} {
		w.Upgrades[id] = true
	}
	fx = FoldEffects(w, tree)
	if fx.SaleHeatMul != 0.85 || fx.PatrolCap != 0.8 || fx.CooldownBonus != 2 || fx.StingStockMul != 0.5 ||
		fx.LieLowMultiplier != 3.0 || fx.Decay != 0.13 || fx.EvidenceCut != 2 || fx.EvidenceDecayDays != 20 ||
		fx.EvidenceArrest != 8 || fx.FallGuys != 2 || fx.RaidLossMul != 1 {
		t.Fatalf("security and legal: %+v", fx)
	}
}

// foldRule is how two owned nodes carrying the same effect combine
// (#117): the four rules of the upgrades.toml header.
type foldRule int

const (
	product foldRule = iota // the multipliers stack
	sum                     // the deltas add
	lowest                  // the best discount owned counts; a multiplier over 1 is ignored
	highest                 // the biggest replacement or raise owned counts
)

// Every name of the vocabulary with its fold rule, tabled by the field
// it lands on. TestFoldEffects folds two nodes carrying a and b and
// checks the result against the rule; a field of content.UpgradeEffects
// missing from this table fails the test, so a new name cannot land
// without its rule.
var foldRules = []struct {
	name string
	rule foldRule
}{
	{"CarryBonus", sum},

	{"SupplierMul", lowest},
	{"BuyPressureMul", product},
	{"FillMul", product},
	{"SaleImpactMul", product},
	{"DemandMul", product},
	{"GlutDecayMul", highest},
	{"BuyerGapMul", lowest},
	{"ContractPremiumBonus", sum},

	{"SaleHeatMul", product},
	{"CrewHeatMul", product},
	{"PatrolCap", highest},
	{"CooldownBonus", sum},
	{"StingStockMul", product},
	{"RaidLossMul", product},
	{"LieLowMultiplier", highest},
	{"Decay", highest},
	{"DirtyCashThresholdMul", highest},
	{"EvidenceCut", sum},
	{"AuditEvidenceCut", sum},
	{"EvidenceDecayDays", lowest},
	{"EvidenceArrest", highest},
	{"FallGuys", sum},

	{"WageMul", lowest},
	{"LoyaltyLossMul", product},
	{"DangerLoyaltyMul", product},
	{"SkimChanceMul", product},
	{"InformantChanceMul", product},
	{"CrewSlots", sum},
	{"CandidatesBonus", sum},
	{"PoolDaysCut", sum},
	{"SkillBonus", sum},
	{"HireFeeMul", lowest},
	{"StartLoyaltyBonus", sum},

	{"WashMul", product},
	{"AuditRiskMul", product},
	{"AuditSeizeMul", lowest},
	{"UpkeepMul", lowest},
	{"AuditFreezeCut", sum},
	{"FloatMul", lowest},

	{"RouteRiskMul", product},
	{"RouteCapacityMul", product},
	{"RouteDaysMul", lowest},
	{"FareMul", lowest},
	{"WholesaleMul", lowest},

	{"DriftDaysBonus", sum},
	{"RobberyMul", product},
	{"GuardBonus", sum},
	{"RivalPushMul", product},
}

// Every name in the vocabulary folds by its rule: two owned nodes
// carrying a and b give a*b, a+b, min or max (carry_bonus is applied on
// purchase and is the one name Effects does not carry); the table covers
// every field of content.UpgradeEffects, and every field of Effects has
// its rule in the table.
func TestFoldEffects(t *testing.T) {
	cfgT := reflect.TypeOf(content.UpgradeEffects{})
	fxT := reflect.TypeOf(Effects{})
	tabled := map[string]bool{}
	for _, r := range foldRules {
		tabled[r.name] = true
	}
	for i := 0; i < cfgT.NumField(); i++ {
		if f := cfgT.Field(i); !tabled[f.Name] {
			t.Errorf("content.UpgradeEffects.%s (%s) has no fold rule in the table", f.Name, f.Tag.Get("toml"))
		}
	}
	for i := 0; i < fxT.NumField(); i++ {
		if f := fxT.Field(i); !tabled[f.Name] {
			t.Errorf("Effects.%s has no fold rule in the table", f.Name)
		}
	}
	for _, r := range foldRules {
		if _, ok := fxT.FieldByName(r.name); !ok {
			if r.name == "CarryBonus" {
				continue
			}
			t.Errorf("%s is tabled but Effects has no such field", r.name)
			continue
		}
		cf, ok := cfgT.FieldByName(r.name)
		if !ok {
			t.Errorf("%s is tabled but content.UpgradeEffects has no such field", r.name)
			continue
		}
		// Two nodes, a and b, in one tree: for a multiplier the values
		// sit either side of 1 so lowest and highest are told apart
		// from a product; for a delta they are plain counts.
		a, b := 0.6, 1.5
		if cf.Type.Kind() == reflect.Int {
			a, b = 3, 5
		}
		set := func(v float64) content.UpgradeEffects {
			var e content.UpgradeEffects
			f := reflect.ValueOf(&e).Elem().FieldByName(r.name)
			if f.Kind() == reflect.Int {
				f.SetInt(int64(v))
			} else {
				f.SetFloat(v)
			}
			return e
		}
		tree := content.UpgradesConfig{Nodes: []content.UpgradeConfig{
			{ID: "a", Name: "A", Branch: "operations", Cost: 1, Effects: set(a)},
			{ID: "b", Name: "B", Branch: "operations", Cost: 1, Effects: set(b)},
		}}
		w := testWorld()
		w.Upgrades["a"], w.Upgrades["b"] = true, true
		got := reflect.ValueOf(FoldEffects(w, tree)).FieldByName(r.name)
		var val float64
		if got.Kind() == reflect.Int {
			val = float64(got.Int())
		} else {
			val = got.Float()
		}
		want := map[foldRule]float64{product: a * b, sum: a + b, lowest: math.Min(a, b), highest: math.Max(a, b)}[r.rule]
		if r.name == "EvidenceDecayDays" || r.name == "EvidenceArrest" {
			// Replacements: 0 means the config's own, so lowest and
			// highest are between the nodes that set them.
			want = map[foldRule]float64{lowest: math.Min(a, b), highest: math.Max(a, b)}[r.rule]
		}
		if math.Abs(val-want) > 1e-9 {
			t.Errorf("%s: two nodes at %v and %v fold to %v, want %v (%s)", r.name, a, b, val, want, []string{"product", "sum", "lowest", "highest"}[r.rule])
		}
		// Nothing owned folds to the identity: 1 for a multiplier, 0
		// for the rest.
		w.Upgrades = map[string]bool{}
		none := reflect.ValueOf(FoldEffects(w, tree)).FieldByName(r.name)
		id := reflect.ValueOf(identity).FieldByName(r.name)
		if none.Kind() == reflect.Int && none.Int() != 0 || none.Kind() == reflect.Float64 && none.Float() != id.Float() {
			t.Errorf("%s: nothing owned folds to %v", r.name, none)
		}
	}
}

func TestSaveKeepsUpgrades(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := testWorld()
	w.Upgrades["stash"] = true
	w.Upgrades["burners"] = true
	w.FallsTaken = 1
	w.Heat.Evidence = 2
	w.Heat.EvidenceDay = 7
	if err := Save(1, w); err != nil {
		t.Fatal(err)
	}
	got, err := Load(1)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Owns("stash") || !got.Owns("burners") || got.Owns("lawyer") || got.FallsTaken != 1 || got.Heat.Evidence != 2 || got.Heat.EvidenceDay != 7 {
		t.Fatalf("upgrades did not round-trip: %v falls %d heat %+v", got.Upgrades, got.FallsTaken, got.Heat)
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

// v9World is a schema-9 save as far as the fall guy goes: the flag on
// World that fall_guys as a count (#117) replaced.
type v9World struct {
	SchemaVersion int
	Seed          uint64
	Day           int
	Cities        map[string]*City
	CityOrder     []string
	Player        Player
	Upgrades      map[string]bool
	FallGuyUsed   bool
}

// A save owning the fall guy migrates to the count with the same
// meaning: one who had taken his fall is one fall taken, one who had
// not is none, and the flag is gone from the stream either way.
func TestSaveMigratesTheFallGuy(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	fresh := testWorld()
	for _, used := range []bool{true, false} {
		old := v9World{SchemaVersion: 9, Seed: fresh.Seed, Day: 4, Cities: fresh.Cities, CityOrder: fresh.CityOrder, Player: fresh.Player, Upgrades: map[string]bool{"fallguy": true}, FallGuyUsed: used}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(old); err != nil {
			t.Fatal(err)
		}
		p, _ := SavePath(1)
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(1); err == nil {
			t.Fatal("a schema-9 save loaded without a migration")
		}
		got, err := Load(1, Migration{From: 9, Apply: MigrateFallGuys}, Migration{From: 10, Apply: func(*World) {}}, Migration{From: 11, Apply: MigrateHouses})
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if used {
			want = 1
		}
		if got.SchemaVersion != SchemaVersion || got.FallsTaken != want || !got.Owns("fallguy") || got.Day != 4 {
			t.Fatalf("used %v: schema %d falls %d owns %v day %d", used, got.SchemaVersion, got.FallsTaken, got.Upgrades, got.Day)
		}
		fx := FoldEffects(got, content.MustLoad().Upgrades)
		if got.FallGuyLeft(fx) == used {
			t.Fatalf("used %v: a fall guy is left = %v", used, got.FallGuyLeft(fx))
		}
	}
}
