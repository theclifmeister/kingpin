package logistics_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/logistics"
)

// The road and the file (#45). A seizure files the route's risk a day
// at full confidence (the one thing that shows you what a road is);
// RiskFrom folds a risk the file holds the way Risk folds the truth; and
// a road a faction fed you as quiet takes the first shipment sent on
// it, whatever the dice, on every seed: the same ShipmentSeized a real
// seizure emits (the stats, the record, the driver all read it), an
// IntelFalse naming the faction, and the lie's source becomes the
// faction. The second shipment rolls the road as it always did.
func TestLureAndTheRiskFact(t *testing.T) {
	cfg := content.MustLoad()
	tun := cfg.Intel.Intel
	// A seizure files the risk.
	w, s, r := world(t, cfg, 1)
	if err := w.SetRoute(r.ID, events.RouteNormal); err != nil {
		t.Fatal(err)
	}
	if err := w.SetRouteTarget(r.ID, w.Products[0], 100); err != nil {
		t.Fatal(err)
	}
	var evs []events.Event
	for i := 0; i < 3 && w.Stats.Seizures == 0; i++ {
		evs = step(w, s)
	}
	if w.Stats.Seizures == 0 {
		t.Fatal("no seizure on a route with risk 1")
	}
	f, ok := game.Known(w).Fact(r.ID, game.FactRisk)
	if !ok || f.Number != 1 || f.Confidence != 1 || f.Source != game.SourceSeen || f.Stale != tun.StaleRate {
		t.Fatalf("the risk fact %+v", f)
	}
	if kinds(evs)["IntelGained"] == 0 {
		t.Fatalf("no IntelGained on the seizure: %v", kinds(evs))
	}
	// RiskFrom folds a given risk as Risk folds the truth.
	real := logistics.New(cfg)
	w2 := game.NewWorld(7, logistics.StartingCities(cfg.City, cfg.Market), 100_000, 100)
	for _, rc := range cfg.Routes.Routes {
		for _, d := range []events.Ship{events.ShipSlow, events.ShipNormal, events.ShipFast} {
			if got, want := real.RiskFrom(w2, rc, d, rc.Risk), real.Risk(w2, rc, d); !near(got, want) {
				t.Fatalf("%s at %v: RiskFrom %.4f Risk %.4f", rc.ID, d, got, want)
			}
			if rc.Mode != "plane" && real.RiskFrom(w2, rc, d, tun.FeedRisk) >= real.RiskFrom(w2, rc, d, 0.5) { // the plane's risk is the feds' watch (#48), not the road's
				t.Fatalf("%s: a quieter road does not read quieter", rc.ID)
			}
		}
	}
	// The lure: on twenty seeds a road with risk 0 takes the first
	// shipment, the fact names the faction, the second gets through.
	for seed := uint64(1); seed <= 20; seed++ {
		w, s, r := world(t, cfg, 0)
		w.Seed = seed
		w.Rivals = []*game.RivalState{{ID: game.FactionRival, Leader: "Sal", Personality: "defensive", Arrived: 1}}
		w.Learn(game.Fact{Subject: r.ID, Kind: game.FactRisk, Value: "~1%/day", Number: tun.FeedRisk, Confidence: tun.FeedConfidence, Day: w.Day, Source: game.SourceContact, Stale: tun.StaleRate, Forget: tun.Forget, Planted: game.FactionRival})
		if err := w.SetRoute(r.ID, events.RouteNormal); err != nil {
			t.Fatal(err)
		}
		if err := w.SetRouteTarget(r.ID, w.Products[0], 100); err != nil {
			t.Fatal(err)
		}
		step(w, s) // sent
		if len(w.Shipments) != 1 {
			t.Fatalf("seed %d: %d shipments sent", seed, len(w.Shipments))
		}
		evs := step(w, s) // its first night on the road
		k := kinds(evs)
		if k["ShipmentSeized"] != 1 || k["IntelFalse"] != 1 || w.Stats.Seizures != 1 || w.Stats.Bitten != 1 || w.Logistics.Lost[r.ID] == 0 || len(w.Logistics.Seizures) != 1 {
			t.Fatalf("seed %d: the lure did not bite: %v stats %+v lost %v", seed, k, w.Stats, w.Logistics.Lost)
		}
		for _, e := range evs {
			if ev, ok := e.(events.IntelFalse); ok && (ev.Faction != game.FactionRival || ev.Rival != "Sal" || ev.Subject != r.ID || ev.FactKind != game.FactRisk || ev.Name != r.Name) {
				t.Fatalf("seed %d: the bite %+v", seed, ev)
			}
		}
		if f, _ := game.Known(w).Fact(r.ID, game.FactRisk); f.Source != game.FactionRival || w.Lure(r.ID, game.FactRisk) != nil {
			t.Fatalf("seed %d: after the bite %+v", seed, f)
		}
		// The road sends again and, at risk 0, nothing is taken.
		for i := 0; i < 6; i++ {
			step(w, s)
		}
		if w.Stats.Seizures != 1 || w.Stats.Shipments < 2 {
			t.Fatalf("seed %d: after the bite %d seizures over %d shipments", seed, w.Stats.Seizures, w.Stats.Shipments)
		}
	}
	// A lure on a road with the dial off bites nothing and rolls
	// nothing: the road with and without it is the same road.
	a, sa, ra := world(t, cfg, 0.2)
	b, sb, _ := world(t, cfg, 0.2)
	other := cfg.Routes.Routes[1]
	a.Learn(game.Fact{Subject: other.ID, Kind: game.FactRisk, Value: "~1%/day", Number: tun.FeedRisk, Confidence: 0.9, Day: 0, Source: game.SourceContact, Planted: game.FactionRival})
	for _, w := range []*game.World{a, b} {
		if err := w.SetRoute(ra.ID, events.RouteNormal); err != nil {
			t.Fatal(err)
		}
		if err := w.SetRouteTarget(ra.ID, w.Products[0], 100); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 15; i++ {
		step(a, sa)
		step(b, sb)
	}
	if a.Stats.Seizures != b.Stats.Seizures || a.Stats.Shipments != b.Stats.Shipments || a.Stats.Bitten != 0 {
		t.Fatalf("a lure on a road you are not on moved the road: %d/%d seizures, %d/%d shipments", a.Stats.Seizures, b.Stats.Seizures, a.Stats.Shipments, b.Stats.Shipments)
	}
}

// kinds counts a night's events by kind.
func kinds(evs []events.Event) map[string]int {
	out := map[string]int{}
	for _, e := range evs {
		out[e.Kind()]++
	}
	return out
}
