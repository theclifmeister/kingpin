package heat_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
)

// The task force (#48) at unit level: the rung between the raid and the
// arrest that forms only against an asset or a pile over
// taskforce_cash, is announced the day the line is crossed, comes the
// next night whatever the heat, takes the file's stock and cash, files
// pages only on a day you dealt (#27), names one asset to seize (the
// one in the city it came to, else the costliest), waits its own flat
// cooldown, and stamps the watch on the skies; the treaty's window
// drops its line; the floor an owned asset puts under every city holds
// against decay; and the ladder the player faces lists the rung only
// once it can form.

func owning(w *game.World, ids ...string) {
	for i, id := range ids {
		w.Assets = append(w.Assets, game.Asset{ID: id, Name: id, City: w.CityOrder[i%len(w.CityOrder)], Cost: 1_000_000 * (i + 1), Bought: w.Day})
	}
}

func TestTaskForceFormsAgainstAnAssetOrThePile(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	tf := rung(cfg, content.TaskForce)
	// Nothing owned, a small pile: the rung is skipped and the raid
	// answers at the same heat, as it always did.
	w := world(t, cfg)
	if s.TaskForceEligible(w) {
		t.Fatal("eligible with nothing owned")
	}
	for _, r := range s.Ladder(w, w.Here()) {
		if r.Level == content.TaskForce {
			t.Fatal("the player's ladder lists the task force with nothing owned")
		}
	}
	w.Here().Heat = over(w, s, tf, w.Here())
	tk := step(w, s, sale(w, w.Here().ID, 100))
	if ev := enforcement(tk); ev == nil || ev.Level != content.Raid {
		t.Fatalf("over the task force's line with nothing owned: %+v, want the raid", ev)
	}
	for _, e := range tk.Events() {
		if _, ok := e.(events.TaskForceFormed); ok {
			t.Fatal("a task force formed with nothing owned")
		}
	}
	// The pile alone over taskforce_cash is enough.
	w = world(t, cfg)
	w.Player.DirtyCash = cfg.Heat.Heat.TaskforceCash + 1
	if !s.TaskForceEligible(w) {
		t.Fatal("not eligible on the pile")
	}
	// An asset owned: the line is crossed, the announcement is the
	// day's one response, nothing fires.
	w = world(t, cfg)
	owning(w, "port")
	if !s.TaskForceEligible(w) || len(s.Ladder(w, w.Here())) != len(content.Levels) {
		t.Fatal("not eligible with an asset, or the ladder hides the rung")
	}
	w.Here().Heat = over(w, s, tf, w.Here())
	tk = step(w, s, sale(w, w.Here().ID, 100))
	formed := false
	for _, e := range tk.Events() {
		if ev, ok := e.(events.TaskForceFormed); ok {
			formed = ev.City == w.Here().ID && ev.Assets == 1
		}
	}
	if !formed || enforcement(tk) != nil || w.Heat.TaskForceDay != w.Day || !s.TaskForceForming(w) {
		t.Fatalf("crossing the line: formed %v, enforcement %+v, TaskForceDay %d (day %d)", formed, enforcement(tk), w.Heat.TaskForceDay, w.Day)
	}
	// It comes the next night, whatever the heat: a quiet day, so no
	// page; the stock and cash go by the file; one asset is named;
	// the watch on the skies is stamped for the cooldown; the rung is
	// on cooldown after and no chief shortens it.
	w.Here().Heat = 10
	w.SetStock(w.Here().ID, w.Products[0], 100)
	w.Player.DirtyCash = 10_000
	tk = step(w, s)
	ev := enforcement(tk)
	if ev == nil || ev.Level != content.TaskForce || ev.Evidence != 0 || w.Heat.Evidence != 0 {
		t.Fatalf("the night after: %+v, evidence %d", ev, w.Heat.Evidence)
	}
	if ev.StockLost[w.Products[0]] != int(100*tf.StockLoss) || ev.CashLost != int(10_000*tf.CashLoss) {
		t.Fatalf("took %v and %d; the file says %.2f and %.2f", ev.StockLost, ev.CashLost, tf.StockLoss, tf.CashLoss)
	}
	seized := 0
	for _, e := range tk.Events() {
		if ev, ok := e.(events.AssetSeized); ok && ev.Asset == "port" {
			seized++
		}
	}
	if seized != 1 || w.Stats.TaskForces != 1 || w.Heat.TaskForceDay != 0 {
		t.Fatalf("seized %d, task forces %d, TaskForceDay %d", seized, w.Stats.TaskForces, w.Heat.TaskForceDay)
	}
	if s.CooldownDays(w, content.TaskForce) != tf.Cooldown || w.Heat.WatchUntil != w.Day+tf.Cooldown {
		t.Fatalf("cooldown %d, watch until %d (day %d); the file says %d flat", s.CooldownDays(w, content.TaskForce), w.Heat.WatchUntil, w.Day, tf.Cooldown)
	}
	w.Law.Chief.Personality = "lazy"
	if s.CooldownDays(w, content.TaskForce) != tf.Cooldown {
		t.Fatal("a chief moved the task force's cooldown")
	}
	// On cooldown the line is crossed and the raid answers instead.
	w.Here().Heat = over(w, s, tf, w.Here())
	tk = step(w, s, sale(w, w.Here().ID, 100))
	if ev := enforcement(tk); ev == nil || ev.Level != content.Raid {
		t.Fatalf("on cooldown: %+v, want the raid", ev)
	}
	// A dealing day when it comes is a page in the file.
	w = world(t, cfg)
	owning(w, "port")
	w.Here().Heat = over(w, s, tf, w.Here())
	step(w, s, sale(w, w.Here().ID, 100))
	tk = step(w, s, sale(w, w.Here().ID, 100))
	if ev := enforcement(tk); ev == nil || ev.Level != content.TaskForce || ev.Evidence != tf.Evidence {
		t.Fatalf("a dealing day: %+v, want %d pages", ev, tf.Evidence)
	}
}

// The asset it names: the one in the city it came to, else the
// costliest owned; nothing with nothing owned (the pile alone).
func TestTaskForceNamesOneAsset(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	tf := rung(cfg, content.TaskForce)
	w := world(t, cfg)
	home, away := w.CityOrder[0], w.CityOrder[1]
	w.Assets = []game.Asset{
		{ID: "lab", Name: "lab", City: home, Cost: 5},
		{ID: "port", Name: "port", City: away, Cost: 9},
		{ID: "tunnel", Name: "tunnel", City: home, Cost: 7},
	}
	w.Here().Heat = over(w, s, tf, w.Here())
	step(w, s, sale(w, home, 100))
	tk := step(w, s)
	named := ""
	for _, e := range tk.Events() {
		if ev, ok := e.(events.AssetSeized); ok {
			named += ev.Asset + " "
		}
	}
	if named != "tunnel " {
		t.Fatalf("named %q: want the costliest at home, the tunnel", named)
	}
	// The pile alone, with fronts enough to cover it (a pile past the
	// cover is heat enough for the arrest).
	w = world(t, cfg)
	w.Player.DirtyCash = cfg.Heat.Heat.TaskforceCash + 1
	w.Fronts = []game.Front{{ID: "laundromat", Name: "Suds", Cost: int(float64(w.Player.DirtyCash)/cfg.Heat.Heat.DirtyCashCover) + 1}}
	w.Here().Heat = over(w, s, tf, w.Here())
	step(w, s, sale(w, home, 100))
	tk = step(w, s)
	for _, e := range tk.Events() {
		if _, ok := e.(events.AssetSeized); ok {
			t.Fatal("seized an asset with none owned")
		}
	}
	if ev := enforcement(tk); ev == nil || ev.Level != content.TaskForce {
		t.Fatalf("on the pile alone: %+v", ev)
	}
}

// The treaty drops the line and the floor holds against decay.
func TestTaskForceLineAndTheFloor(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	tf := rung(cfg, content.TaskForce)
	w := world(t, cfg)
	base := s.Threshold(w, tf, w.Here())
	w.Heat.LineUntil, w.Heat.LineMul = w.Day+10, 0.85
	if got := s.Threshold(w, tf, w.Here()); !near(got, base*0.85) {
		t.Fatalf("under the treaty the line is %.2f, want %.2f", got, base*0.85)
	}
	if got := s.Threshold(w, rung(cfg, content.Raid), w.Here()); near(got, s.Threshold(w, rung(cfg, content.Raid), w.Here())*0.85) {
		t.Fatal("the treaty moved the raid's line")
	}
	w.Heat.LineUntil = 0
	// The floor: the highest owned and standing asset's, and decay
	// never takes a city under it.
	var top *content.AssetConfig
	for i := range cfg.Assets.Offers {
		if a := &cfg.Assets.Offers[i]; top == nil || a.HeatFloor > top.HeatFloor {
			top = a
		}
	}
	for _, a := range cfg.Assets.Offers {
		owning(w, a.ID)
	}
	if got := s.Floor(w); !near(got, top.HeatFloor) {
		t.Fatalf("floor %.2f with everything owned, want %.2f (%s)", got, top.HeatFloor, top.ID)
	}
	for i := range w.Assets {
		if cfg.Assets.Asset(w.Assets[i].ID).HeatFloor == top.HeatFloor {
			w.Assets[i].FrozenUntil = w.Day + 5
		}
	}
	if got := s.Floor(w); near(got, top.HeatFloor) {
		t.Fatal("an idle asset's floor holds")
	}
	for i := range w.Assets {
		w.Assets[i].FrozenUntil = 0
	}
	w.Here().Heat = top.HeatFloor + 1
	for i := 0; i < 30; i++ {
		step(w, s)
	}
	if w.Here().Heat < top.HeatFloor || w.Here().Heat > top.HeatFloor+0.1 {
		t.Fatalf("after a month of nothing the heat is %.2f, the floor %.2f", w.Here().Heat, top.HeatFloor)
	}
}

// The task force takes the costliest thing owned (#392): a trophy that
// cost more than the asset it would take is taken instead of it; a
// cheaper trophy is left and the asset goes; with no asset the trophy
// goes. Exactly one thing a firing.
func TestTaskForceTakesATrophy(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	tf := rung(cfg, content.TaskForce)
	fire := func(assets []game.Asset, trophies []game.Trophy) (asset, trophy string) {
		w := world(t, cfg)
		home := w.CityOrder[0]
		for i := range assets {
			assets[i].City = home
		}
		w.Assets, w.Trophies = assets, trophies
		w.Here().Heat = over(w, s, tf, w.Here())
		step(w, s, sale(w, home, 100))
		for _, e := range step(w, s).Events() {
			switch ev := e.(type) {
			case events.AssetSeized:
				asset += ev.Asset + " "
			case events.TrophySeized:
				trophy += ev.Trophy + " "
			}
		}
		return asset, trophy
	}
	if a, tr := fire([]game.Asset{{ID: "lab", Name: "lab", Cost: 5}}, []game.Trophy{{ID: "penthouse", Name: "P", Cost: 3}, {ID: "yacht", Name: "Y", Cost: 9}}); a != "" || tr != "yacht " {
		t.Fatalf("a yacht over a lab: took asset %q trophy %q", a, tr)
	}
	if a, tr := fire([]game.Asset{{ID: "lab", Name: "lab", Cost: 5}}, []game.Trophy{{ID: "penthouse", Name: "P", Cost: 3}}); a != "lab " || tr != "" {
		t.Fatalf("a lab over a penthouse: took asset %q trophy %q", a, tr)
	}
	// With no asset the task force forms only on the pile; a trophy is
	// what it takes then.
	w := world(t, cfg)
	w.Player.DirtyCash = cfg.Heat.Heat.TaskforceCash + 1
	w.Fronts = []game.Front{{ID: "laundromat", Name: "Suds", Cost: int(float64(w.Player.DirtyCash)/cfg.Heat.Heat.DirtyCashCover) + 1}}
	w.Trophies = []game.Trophy{{ID: "zoo", Name: "Z", Cost: 80}}
	w.Here().Heat = over(w, s, tf, w.Here())
	step(w, s, sale(w, w.CityOrder[0], 100))
	took := ""
	for _, e := range step(w, s).Events() {
		if ev, ok := e.(events.TrophySeized); ok {
			took += ev.Trophy
		}
	}
	if took != "zoo" {
		t.Fatalf("on the pile with a zoo: took %q", took)
	}
}
