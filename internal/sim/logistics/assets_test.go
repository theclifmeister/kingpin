package logistics_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The routes an asset opens and the port (#48): the plane and the
// tunnel are not open, not listed and not run until their asset is
// owned and standing; the plane's risk is the file's only while the
// feds watch the skies; the port doubles the boats into its city and
// clears customs; and a seizure on the tunnel is TunnelFound, once.
func TestAssetsOnTheRoad(t *testing.T) {
	cfg := content.MustLoad()
	w, s, _ := world(t, cfg, 0)
	var plane, tunnel, boat content.RouteConfig
	for _, r := range cfg.Routes.Routes {
		switch r.Mode {
		case "plane":
			plane = r
		case "tunnel":
			tunnel = r
		case "boat":
			boat = r
		}
	}
	if plane.Asset == "" || tunnel.Asset == "" || boat.Asset != "" {
		t.Fatalf("the file: plane %+v tunnel %+v boat %+v", plane, tunnel, boat)
	}
	home := w.CityOrder[0]
	if s.Open(w, plane) || s.Open(w, tunnel) || !s.Open(w, boat) || len(s.RoutesOpen(w, home)) != len(s.Routes(home))-2 {
		t.Fatal("the plane or the tunnel is open with nothing owned")
	}
	// On, with a target: nothing goes until the asset stands.
	for i, r := range []content.RouteConfig{plane, tunnel} {
		_ = w.SetRoute(r.ID, events.RouteNormal)
		_ = w.SetRouteTarget(r.ID, w.Products[i], 100) // a product each, so one does not fill the other's target
		w.SetStock(r.From, w.Products[i], 500)
	}
	w.Player.DirtyCash = 10_000_000
	for _, e := range step(w, s) {
		if ev, ok := e.(events.ShipmentSent); ok && (ev.Route == plane.ID || ev.Route == tunnel.ID) {
			t.Fatalf("%s sent with nothing owned", ev.Route)
		}
	}
	w.Assets = []game.Asset{{ID: plane.Asset, Name: "plane", City: home}, {ID: tunnel.Asset, Name: "tunnel", City: home}}
	if !s.Open(w, plane) || !s.Open(w, tunnel) || len(s.RoutesOpen(w, home)) != len(s.Routes(home)) {
		t.Fatal("the plane or the tunnel is shut with its asset standing")
	}
	sent := map[string]bool{}
	for _, e := range step(w, s) {
		if ev, ok := e.(events.ShipmentSent); ok {
			sent[ev.Route] = true
		}
	}
	if !sent[plane.ID] || !sent[tunnel.ID] {
		t.Fatalf("sent %v; the plane and the tunnel should run", sent)
	}
	w.Assets[0].FrozenUntil = w.Day + 5
	if s.Open(w, plane) {
		t.Fatal("the plane is open with its asset idle")
	}
	w.Assets[0].FrozenUntil = 0
	// The plane's risk: nothing until the feds watch, then the file's.
	if s.DayRisk(w, plane, events.ShipNormal) != 0 || s.Risk(w, plane, events.ShipFast) != 0 {
		t.Fatalf("the plane's risk with nobody watching: %.3f", s.DayRisk(w, plane, events.ShipNormal))
	}
	w.Heat.WatchUntil = w.Day + 10
	if got := s.DayRisk(w, plane, events.ShipNormal); !near(got, plane.Risk*cfg.Routes.Dial.Normal.Risk) {
		t.Fatalf("the plane's risk under the watch: %.3f, want %.3f", got, plane.Risk*cfg.Routes.Dial.Normal.Risk)
	}
	w.Heat.WatchUntil = 0
	// The port: the boats into its city carry capacity_mul and clear
	// customs; nothing else moves.
	port := cfg.Assets.ByEffect(content.AssetPort)
	if port == nil || boat.Other(port.City) == "" {
		t.Fatalf("no port on %s: %+v", boat.ID, port)
	}
	before := s.Capacity(w, boat)
	risk := s.DayRisk(w, boat, events.ShipNormal)
	w.Assets = append(w.Assets, game.Asset{ID: port.ID, Name: "port", City: port.City})
	if got := s.Capacity(w, boat); got != int(float64(before)*port.CapacityMul+0.5) {
		t.Fatalf("the boat's capacity with the port: %d, want %d", got, int(float64(before)*port.CapacityMul+0.5))
	}
	if s.DayRisk(w, boat, events.ShipNormal) != 0 || risk == 0 {
		t.Fatalf("the boat's risk with the port: %.3f, was %.3f", s.DayRisk(w, boat, events.ShipNormal), risk)
	}
	for _, r := range cfg.Routes.Routes {
		if r.Mode != "boat" && s.Capacity(w, r) != int(float64(r.Capacity)+0.5) && r.Mode != "plane" && r.Mode != "tunnel" {
			t.Fatalf("the port moved %s's capacity", r.ID)
		}
	}
}

// A seizure on the tunnel is TunnelFound: once, naming the asset, and
// the route no longer runs; a seizure on any other route is not.
func TestTunnelFound(t *testing.T) {
	cfg := content.MustLoad()
	w, s, _ := world(t, cfg, 1) // every day seized
	var tunnel content.RouteConfig
	for _, r := range cfg.Routes.Routes {
		if r.Mode == "tunnel" {
			tunnel = r
		}
	}
	w.Assets = []game.Asset{{ID: tunnel.Asset, Name: "tunnel", City: w.CityOrder[0]}}
	_ = w.SetRoute(tunnel.ID, events.RouteNormal)
	_ = w.SetRouteTarget(tunnel.ID, w.Products[0], 100)
	w.Player.DirtyCash = 10_000_000
	step(w, s) // sent
	found := 0
	for _, e := range step(w, s) {
		switch ev := e.(type) {
		case events.TunnelFound:
			found++
			if ev.Asset != tunnel.Asset || ev.Route != tunnel.ID || ev.Units == 0 {
				t.Fatalf("%+v", ev)
			}
		}
	}
	if found != 1 {
		t.Fatalf("found %d times", found)
	}
	// The laundering sim takes the asset off the books; here by hand.
	w.LoseAsset(tunnel.Asset, w.Day, "found")
	if s.Open(w, tunnel) {
		t.Fatal("the tunnel runs after it was found")
	}
	for _, e := range step(w, s) {
		if ev, ok := e.(events.ShipmentSent); ok && ev.Route == tunnel.ID {
			t.Fatal("the tunnel sent after it was found")
		}
	}
}
