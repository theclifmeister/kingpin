package logistics_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/logistics"
)

// exportsWorld is the logistics fixture with every lane's risk set,
// and the lane that opens with the book alone.
func exportsWorld(t *testing.T, risk float64) (*game.World, *logistics.Sim, content.LaneConfig, *content.Config) {
	t.Helper()
	cfg := *content.MustLoad()
	cfg.Exports.Lanes = append([]content.LaneConfig(nil), cfg.Exports.Lanes...)
	for i := range cfg.Exports.Lanes {
		cfg.Exports.Lanes[i].Risk = risk
	}
	w, _, _ := world(t, &cfg, 0)
	s := logistics.New(&cfg)
	var lane content.LaneConfig
	for _, l := range cfg.Exports.Lanes {
		if l.Asset == "" {
			lane = l
		}
	}
	if lane.ID == "" {
		t.Fatal("no lane opens with the book alone")
	}
	return w, s, lane, &cfg
}

// shippable is a product the lane takes that the book sells today.
func shippable(t *testing.T, w *game.World, s *logistics.Sim, l content.LaneConfig) string {
	t.Helper()
	for _, id := range l.Products {
		if s.ExportCost(w, id) > 0 && s.ExportPrice(w, l, id) > 0 {
			return id
		}
	}
	t.Fatalf("the %s ships nothing listed here", l.ID)
	return ""
}

func book(cfg *content.Config) game.Asset {
	b := cfg.Assets.ByEffect(content.AssetSupplier)
	return game.Asset{ID: b.ID, Name: b.Name, Effect: b.Effect, City: b.City}
}

// The lanes abroad (#391): shut and silent without the book, whatever
// is ordered; with it a load leaves the night it is ordered, bought off
// the book at own_ratio of street, and lands paid at the locked rate
// after the lane's days; the glut takes its share off the next load's
// price and eases by recover a day; the till over the float caps the
// load; the port doubles a boat out of its city.
func TestExportLanes(t *testing.T) {
	w, s, lane, cfg := exportsWorld(t, 0)
	w.Player.DirtyCash = 1_000_000_000
	id := shippable(t, w, s, lane)
	if err := w.SetExport(lane.ID, id, lane.Capacity); err != nil {
		t.Fatal(err)
	}
	if s.LaneOpen(w, lane) || len(s.LanesOpen(w)) != 0 {
		t.Fatal("a lane is open without the book")
	}
	before := w.Player.DirtyCash
	for _, e := range step(w, s) {
		switch e.(type) {
		case events.ExportShipped, events.ExportLanded, events.ExportSeized:
			t.Fatalf("%s with no book", e.Kind())
		}
	}
	if w.Player.DirtyCash != before || len(w.Exports.Loads) != 0 {
		t.Fatal("the lane moved money or a load without the book")
	}

	w.Assets = []game.Asset{book(cfg)}
	if !s.LaneOpen(w, lane) {
		t.Fatal("the book's lane is shut with the book standing")
	}
	unit := s.ExportCost(w, id)
	b := cfg.Assets.ByEffect(content.AssetSupplier)
	if want := w.Product(b.City, id).Price * b.OwnRatio; !near(unit, want) {
		t.Fatalf("a unit off the book costs %.2f, want %.2f", unit, want)
	}
	price := s.ExportPrice(w, lane, id)
	before = w.Player.DirtyCash
	var shipped events.ExportShipped
	for _, e := range step(w, s) {
		if ev, ok := e.(events.ExportShipped); ok {
			shipped = ev
		}
	}
	if shipped.Units != lane.Capacity || shipped.Cost != int(math.Round(unit*float64(lane.Capacity))) || shipped.Lands != w.Day+lane.Days {
		t.Fatalf("shipped %+v; want %d units for %.0f landing day %d", shipped, lane.Capacity, unit*float64(lane.Capacity), w.Day-1+lane.Days)
	}
	if before-w.Player.DirtyCash != shipped.Cost || w.NetWorth() < before-1 {
		t.Fatalf("the till fell %d for a %d load (net worth %d, was %d)", before-w.Player.DirtyCash, shipped.Cost, w.NetWorth(), before)
	}
	if g := w.Glut(lane.ID, id); !near(g, lane.Glut) {
		t.Fatalf("a full load gluts %.3f, want %.3f", g, lane.Glut)
	}
	if next := s.ExportPrice(w, lane, id); !near(next, price*(1-lane.Glut)) {
		t.Fatalf("the next load's rate %.2f, want %.2f", next, price*(1-lane.Glut))
	}

	// The order off: what is out still lands, on the day, at the rate.
	_ = w.SetExport(lane.ID, "", 0)
	var landed events.ExportLanded
	for d := 1; d < lane.Days; d++ {
		for _, e := range step(w, s) {
			if _, ok := e.(events.ExportLanded); ok {
				t.Fatalf("landed on day %d of %d", d, lane.Days)
			}
		}
	}
	cash := w.Player.DirtyCash
	for _, e := range step(w, s) {
		if ev, ok := e.(events.ExportLanded); ok {
			landed = ev
		}
	}
	if want := int(float64(lane.Capacity)*shipped.Price + 0.5); landed.Revenue != want || w.Player.DirtyCash-cash != want {
		t.Fatalf("landed %+v, the till rose %d; want %d", landed, w.Player.DirtyCash-cash, want)
	}
	if w.Stats.ExportCash != landed.Revenue || w.Stats.ExportUnits != lane.Capacity || w.Stats.ExportLoads != 1 || len(w.Exports.Loads) != 0 {
		t.Fatalf("stats %+v, loads %v", w.Stats, w.Exports.Loads)
	}
	if g := w.Glut(lane.ID, id); !near(g, math.Max(0, lane.Glut-float64(lane.Days)*lane.Recover)) {
		t.Fatalf("the glut after %d days is %.3f", lane.Days, g)
	}

	// The till caps the load: what is over the float, and no more.
	_ = w.SetExport(lane.ID, id, lane.Capacity)
	w.Player.DirtyCash = s.Float() + int(unit*10) + 1
	units, cost := s.LoadTonight(w, lane, s.Budget(w))
	if units != 10 || cost > s.Budget(w) {
		t.Fatalf("a till with ten units over the float loads %d for %d", units, cost)
	}

	// The port doubles a boat out of its city.
	port := cfg.Assets.ByEffect(content.AssetPort)
	if lane.Mode != "boat" || port.City != lane.City {
		t.Fatalf("the book's lane is not a boat out of the port's city: %+v", lane)
	}
	w.Assets = append(w.Assets, game.Asset{ID: port.ID, Name: port.Name, Effect: port.Effect, City: port.City})
	if got := s.LaneCapacity(w, lane); got != int(float64(lane.Capacity)*port.CapacityMul) {
		t.Fatalf("the port leaves the lane at %d units", got)
	}
}

// A load is seized on one roll off the exports stream: every load at
// risk 1 is taken, its cost gone and nothing paid; the watch multiplies
// the risk; and the same seed seizes the same loads.
func TestExportSeizures(t *testing.T) {
	w, s, lane, cfg := exportsWorld(t, 1)
	w.Assets = []game.Asset{book(cfg)}
	w.Player.DirtyCash = 1_000_000_000
	id := shippable(t, w, s, lane)
	_ = w.SetExport(lane.ID, id, 100)
	_ = step(w, s)
	_ = w.SetExport(lane.ID, "", 0)
	cash := w.Player.DirtyCash
	seized := 0
	for d := 0; d < lane.Days; d++ {
		for _, e := range step(w, s) {
			switch e.(type) {
			case events.ExportSeized:
				seized++
			case events.ExportLanded:
				t.Fatal("a load landed at risk 1")
			}
		}
	}
	if seized != 1 || w.Stats.ExportsSeized != 1 || w.Player.DirtyCash != cash || w.Stats.ExportCash != 0 {
		t.Fatalf("seized %d, stats %d, till moved %d", seized, w.Stats.ExportsSeized, w.Player.DirtyCash-cash)
	}
	if len(w.Exports.Record) != 1 || w.Exports.Record[0].Seized == 0 {
		t.Fatalf("the record: %+v", w.Exports.Record)
	}

	low := lane
	low.Risk = 0.1
	if got := s.LaneRisk(w, low, w.Day); !near(got, 0.1) {
		t.Fatalf("risk %.3f unwatched", got)
	}
	w.Heat.WatchUntil = w.Day + 5
	if got := s.LaneRisk(w, low, w.Day); !near(got, 0.1*cfg.Exports.Exports.WatchedMul) {
		t.Fatalf("risk %.3f watched", got)
	}

	// The same seed, the same seizures.
	run := func() []int {
		w, s, lane, cfg := exportsWorld(t, 0.3)
		w.Assets = []game.Asset{book(cfg)}
		w.Player.DirtyCash = 1_000_000_000
		_ = w.SetExport(lane.ID, shippable(t, w, s, lane), 50)
		var days []int
		for d := 0; d < 60; d++ {
			for _, e := range step(w, s) {
				if ev, ok := e.(events.ExportSeized); ok {
					days = append(days, ev.Day)
				}
			}
		}
		return days
	}
	a, b := run(), run()
	if len(a) == 0 || len(a) != len(b) {
		t.Fatalf("seizures %v then %v", a, b)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("seizures %v then %v", a, b)
		}
	}
}
