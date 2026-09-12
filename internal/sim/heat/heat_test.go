package heat_test

import (
	"math"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
)

// The heat sim at unit level (#144): one world, one step at a time, no
// harness. Every number is read off the config the sim was built from,
// so a retune moves the test and not its meaning; what these pin is the
// shape: the ladder under the law, #27's quiet-day rule, the cooldown,
// decay and the floor, the informant's clock, the cash pile's line and
// the raid's place (#73).

// world is a fresh run on a seed, standing on the home city's first
// corner so a sale there has somewhere to draw heat from, with a fixed
// chief and DA so the ladder reads the same on every seed.
func world(t *testing.T, cfg *content.Config) *game.World {
	t.Helper()
	w := sim.NewWorld(cfg, 3)
	w.Law.Chief.Personality, w.Law.DA.Stance = "corrupt", "moderate"
	if err := w.Post(w.Home().Corners[0].ID, game.You); err != nil {
		t.Fatal(err)
	}
	return w
}

// step runs one day of heat over w with the given events already
// emitted (the earlier sims' sales, the war), and returns the tick.
func step(w *game.World, s *heat.Sim, evs ...events.Event) *game.Tick {
	tk := &game.Tick{Day: w.Day + 1, RNG: game.RNGFor(w.Seed, w.Day+1), Seed: w.Seed}
	for _, e := range evs {
		tk.Emit(e)
	}
	s.Step(w, tk)
	w.Day++
	w.ClearToday(w.Day)
	return tk
}

// enforcement is the tick's one Enforcement, or nil.
func enforcement(tk *game.Tick) *events.Enforcement {
	for _, e := range tk.Events() {
		if ev, ok := e.(events.Enforcement); ok {
			return &ev
		}
	}
	return nil
}

// sale is a PlayerSold in a city today: the player dealt there.
func sale(w *game.World, city string, units int) events.PlayerSold {
	return events.PlayerSold{Day: w.Day + 1, City: city, Product: w.Products[0], Wanted: units, Sold: units, Dial: events.DialNormal}
}

// rung is the ladder's rung for a level.
func rung(cfg *content.Config, level string) content.ResponseConfig {
	for _, r := range cfg.Heat.Responses {
		if r.Level == level {
			return r
		}
	}
	panic("no rung " + level)
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// over is the heat a city carries into the night for its police to
// answer at a rung: the line plus one after the day's decay, since the
// sim decays before it checks the ladder. It can read over 100: the
// day's sales add before the decay and the cap.
func over(w *game.World, s *heat.Sim, r content.ResponseConfig, city *game.City) float64 {
	return (s.Threshold(w, r, city) + 1) / (1 - s.Decay(w))
}

// The ladder is heat.toml's four rungs, in the ladder's order, each
// level once; the constants are the file's names, ranked in that order.
func TestLadderIsTheFile(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	got := s.Thresholds()
	if len(got) != len(content.Levels) {
		t.Fatalf("%d rungs, want %d", len(got), len(content.Levels))
	}
	for i, r := range got {
		if r.Level != content.Levels[i] || content.Rank(r.Level) != i+1 {
			t.Fatalf("rung %d is %s (rank %d), want %s", i, r.Level, content.Rank(r.Level), content.Levels[i])
		}
		if i > 0 && r.Threshold <= got[i-1].Threshold {
			t.Fatalf("%s fires at %.0f, not above %s at %.0f", r.Level, r.Threshold, got[i-1].Level, got[i-1].Threshold)
		}
	}
	if content.Rank("") != 0 || content.Rank("bust") != 0 {
		t.Fatal("a name that is not a level ranks")
	}
}

// The law moves the ladder and nothing else (#41): under every chief
// and DA the sting line is the DA's, the other three lines are the
// police's and drop with the city's pressure, the cooldown of a sting
// or a raid is the chief's while the patrol keeps its cadence, the
// decay and what a patrol lets through are the chief's, and the pages
// an indictment needs are the DA's, never under one. A name the file
// does not know is neutral. ThresholdsIn is the same lines.
func TestLadderUnderTheLaw(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	tun := cfg.Heat.Heat
	chiefs := append(append([]string(nil), content.ChiefPersonalities...), "unknown")
	das := append(append([]string(nil), content.DAStances...), "unknown")
	for _, chief := range chiefs {
		for _, da := range das {
			w := world(t, cfg)
			w.Law.Chief.Personality, w.Law.DA.Stance = chief, da
			cc, dc := cfg.Law.ChiefFor(chief), cfg.Law.DAFor(da)
			if s.Chief(w) != cc || s.DA(w) != dc {
				t.Fatalf("%s/%s: the sim reads %+v %+v", chief, da, s.Chief(w), s.DA(w))
			}
			for _, pressure := range []float64{0, 50, 100} {
				here := w.Here()
				here.Pressure = pressure
				cut := content.Cut(pressure, cfg.Law.Effects.PressureThresholdCut)
				lines := s.ThresholdsIn(w, here)
				for i, r := range s.Thresholds() {
					want := r.Threshold * cut
					if r.Level == content.Sting {
						want = r.Threshold * dc.Sting
					}
					if got := s.Threshold(w, r, here); !near(got, want) || !near(lines[i].Threshold, want) {
						t.Fatalf("%s/%s pressure %.0f: %s fires at %.2f (ThresholdsIn %.2f), want %.2f", chief, da, pressure, r.Level, got, lines[i].Threshold, want)
					}
					if got := s.Threshold(w, r, nil); r.Level != content.Sting && !near(got, r.Threshold) {
						t.Fatalf("%s/%s: %s with no city fires at %.2f, want the file's %.2f", chief, da, r.Level, got, r.Threshold)
					}
				}
				patrol := rung(cfg, content.Patrol)
				wantCap := math.Min(1, patrol.Cap*cc.Cap*content.Cut(pressure, cfg.Law.Effects.PressureCapCut))
				if got := s.PatrolCap(w, patrol, here); !near(got, wantCap) {
					t.Fatalf("%s/%s pressure %.0f: patrol cap %.3f, want %.3f", chief, da, pressure, got, wantCap)
				}
			}
			for _, level := range content.Levels {
				want := float64(tun.CooldownDays)
				if level != content.Patrol {
					want *= cc.Cooldown
				}
				if got := s.CooldownDays(w, level); got != max(1, int(math.Round(want))) {
					t.Fatalf("%s/%s: %s cooldown %d days, want %d", chief, da, level, got, max(1, int(math.Round(want))))
				}
			}
			if got := s.Decay(w); !near(got, math.Min(1, tun.Decay*cc.Decay)) {
				t.Fatalf("%s/%s: decay %.4f, want %.4f", chief, da, got, tun.Decay*cc.Decay)
			}
			// A DA whose ticket ran on your money (#193) lifts the sting
			// line by backed_sting, and nothing else on the ladder.
			w.Law.DA.Backed = true
			for _, r := range s.Thresholds() {
				want := r.Threshold * content.Cut(w.Here().Pressure, cfg.Law.Effects.PressureThresholdCut)
				if r.Level == content.Sting {
					want = r.Threshold * dc.Sting * cfg.Law.Effects.BackedSting
				}
				if got := s.Threshold(w, r, w.Here()); !near(got, want) {
					t.Fatalf("%s/%s backed: %s fires at %.2f, want %.2f", chief, da, r.Level, got, want)
				}
			}
			w.Law.DA.Backed = false
			if got, want := s.EvidenceArrest(w), max(1, int(math.Round(float64(tun.EvidenceArrest)*dc.EvidenceArrest))); got != want {
				t.Fatalf("%s/%s: indictment at %d pages, want %d", chief, da, got, want)
			}
		}
	}
	// The zealous chief is the quick one and the lazy chief the slow
	// one, so the table has the sign the doc gives it.
	w := world(t, cfg)
	w.Law.Chief.Personality = "zealous"
	quick := s.CooldownDays(w, content.Sting)
	w.Law.Chief.Personality = "lazy"
	if slow := s.CooldownDays(w, content.Sting); !(quick < s.CooldownDays(world(t, cfg), content.Sting) && slow > quick) {
		t.Fatalf("sting cooldown: zealous %d, lazy %d", quick, slow)
	}
	w.Law.DA.Stance = "law_and_order"
	fewer := s.EvidenceArrest(w)
	w.Law.DA.Stance = "reform"
	if more := s.EvidenceArrest(w); !(fewer <= s.EvidenceArrest(world(t, cfg)) && more > fewer) {
		t.Fatalf("indictment: law-and-order %d pages, reform %d", fewer, more)
	}
}

// Sale heat follows volume: a sale draws sale_heat per street_units of
// heat-weighted units, times the product, the city, the dial and the
// corner; the quiet dial draws less than normal and aggressive more; a
// city that looks harder draws more; nothing sells from no corner. A
// contract's handoff is the same weight on no corner, scaled by the
// buyer; a move between places is that at move_heat.
func TestSaleHeatFollowsVolume(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	w := world(t, cfg)
	home, product := w.Home().ID, w.Products[0]
	tun := cfg.Heat.Heat
	pc := cfg.Market.Product(product)
	corner := w.Home().Corners[0]
	wanted := int(math.Round(w.Demand(home, product)))
	if wanted <= 0 {
		t.Fatalf("no demand on %s", corner.Name)
	}
	normal := s.SaleHeat(w, home, product, wanted, events.DialNormal)
	want := tun.SaleHeat * float64(wanted) * corner.Heat * w.Home().HeatMul * pc.Heat / tun.StreetUnits * cfg.Market.Dial.Normal.Heat
	if !near(normal, want) {
		t.Fatalf("normal sale of %d: %.4f, want %.4f", wanted, normal, want)
	}
	quiet := s.SaleHeat(w, home, product, wanted, events.DialQuiet)
	loud := s.SaleHeat(w, home, product, wanted, events.DialAggressive)
	if !(quiet < normal && normal < loud) {
		t.Fatalf("dials: quiet %.3f normal %.3f aggressive %.3f", quiet, normal, loud)
	}
	if half := s.SaleHeat(w, home, product, wanted/2, events.DialNormal); !(half < normal) {
		t.Fatalf("half the units draws %.3f, the lot %.3f", half, normal)
	}
	if over := s.SaleHeat(w, home, product, wanted*10, events.DialNormal); over > loud {
		t.Fatalf("wanting ten times the demand draws %.3f: heat follows what could be attempted, not the ask", over)
	}
	w.Home().HeatMul = 2
	if hard := s.SaleHeat(w, home, product, wanted, events.DialNormal); !near(hard, 2*normal) {
		t.Fatalf("a city at heat 2 draws %.3f, want %.3f", hard, 2*normal)
	}
	w.Home().HeatMul = 1
	w.Recall(game.You)
	if off := s.SaleHeat(w, home, product, wanted, events.DialNormal); off != 0 {
		t.Fatalf("off every corner a sale still draws %.3f", off)
	}
	if got := s.ContractHeat(w, home, product, 50, 1); !near(got, tun.SaleHeat*50*w.Home().HeatMul*pc.Heat/tun.StreetUnits) {
		t.Fatalf("a handoff of 50 draws %.4f", got)
	}
	if got := s.MoveHeat(w, home, product, 50); !near(got, s.ContractHeat(w, home, product, 50, cfg.Houses.Houses.MoveHeat)) {
		t.Fatalf("a move of 50 draws %.4f", got)
	}
	if s.SaleHeat(w, "nowhere", product, wanted, events.DialNormal) != 0 || s.ContractHeat(w, home, "nothing", 50, 1) != 0 {
		t.Fatal("a city or a product that is not there draws heat")
	}
}

// The quiet-day rule (#27), at the rung: a sting or a raid on a day the
// player dealt in the city files its pages; the same response on a day
// nothing moved there costs the same stock and cash and cools the same
// heat but files nothing, so the file stays a record of what you did.
// A handoff to a buyer counts as dealing; a sale in the other city does
// not count here.
func TestQuietDayRule(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	for _, level := range []string{content.Sting, content.Raid} {
		r := rung(cfg, level)
		for _, dealt := range []string{"quiet", "sold here", "sold elsewhere", "handoff"} {
			w := world(t, cfg)
			home := w.Home().ID
			w.SetStock(home, w.Products[0], 100)
			w.Player.DirtyCash = 10_000
			w.Home().Heat = over(w, s, r, w.Home())
			var evs []events.Event
			switch dealt {
			case "sold here":
				evs = append(evs, sale(w, home, 10))
			case "sold elsewhere":
				evs = append(evs, sale(w, w.CityOrder[1], 10))
			case "handoff":
				evs = append(evs, events.ContractDelivered{Day: w.Day + 1, City: home, Product: w.Products[0], Units: 10, HeatMul: 1})
			}
			tk := step(w, s, evs...)
			ev := enforcement(tk)
			if ev == nil || ev.Level != level || ev.City != home {
				t.Fatalf("%s %s: enforcement %+v", level, dealt, ev)
			}
			wantStock := int(math.Round(100 * r.StockLoss))
			wantCash := int(math.Round(10_000 * r.CashLoss))
			if ev.StockLost[w.Products[0]] != wantStock || ev.CashLost != wantCash || w.Stock(home, w.Products[0]) != 100-wantStock || w.Player.DirtyCash != 10_000-wantCash {
				t.Fatalf("%s %s: took %v and $%d, stock %d cash %d; want %d and $%d", level, dealt, ev.StockLost, ev.CashLost, w.Stock(home, w.Products[0]), w.Player.DirtyCash, wantStock, wantCash)
			}
			pages := 0
			if dealt == "sold here" || dealt == "handoff" {
				pages = r.Evidence
			}
			if ev.Evidence != pages || w.Heat.Evidence != pages {
				t.Fatalf("%s %s: filed %d pages, the file reads %d; want %d", level, dealt, ev.Evidence, w.Heat.Evidence, pages)
			}
			if w.Heat.Responses[level] != 1 || w.Heat.LastResponse[level] != w.Day {
				t.Fatalf("%s %s: responses %v last %v", level, dealt, w.Heat.Responses, w.Heat.LastResponse)
			}
			if len(w.Heat.Busts) != 1 || w.Heat.Busts[0] != (game.Bust{Day: w.Day, City: home, Level: level, Units: wantStock}) {
				t.Fatalf("%s %s: the bust on record is %+v", level, dealt, w.Heat.Busts)
			}
		}
	}
}

// Every rung: the hottest city's police answer, highest rung first and
// one a day; a patrol caps the street for cap_days at the cap; a rung
// that fired waits its cooldown while the rung above can still fire;
// each repeat of a rung cools less; and the arrest ends the run, or the
// fall guy takes it, once.
func TestTheLadderFires(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	w := world(t, cfg)
	home, hub := w.Home(), w.Cities[w.CityOrder[1]]
	patrol, sting := rung(cfg, content.Patrol), rung(cfg, content.Sting)

	// The patrol, from the hotter city: the hub, though you stand at
	// home. The cap is the patrol's for cap_days, then lifts.
	hub.Heat = over(w, s, patrol, hub)
	home.Heat = 0
	tk := step(w, s)
	if ev := enforcement(tk); ev == nil || ev.Level != content.Patrol || ev.City != hub.ID {
		t.Fatalf("the patrol: %+v", ev)
	}
	if w.Heat.SellCapDays != patrol.CapDays || !near(w.Heat.SellCap, s.PatrolCap(w, patrol, hub)) {
		t.Fatalf("the cap: %d days at %.2f", w.Heat.SellCapDays, w.Heat.SellCap)
	}
	hub.Heat = 0
	for i := 0; i < patrol.CapDays; i++ {
		step(w, s)
	}
	if w.Heat.SellCapDays != 0 || w.Heat.SellCap != 0 {
		t.Fatalf("the cap did not lift: %d days at %.2f", w.Heat.SellCapDays, w.Heat.SellCap)
	}

	// Over the sting line, the sting and not the patrol; on the line
	// again the next day, nothing until the cooldown is up, and then
	// the second sting cools half as much as the first.
	w.SetStock(home.ID, w.Products[0], 1000)
	home.Heat = over(w, s, sting, home)
	tk = step(w, s, sale(w, home.ID, 10))
	if ev := enforcement(tk); ev == nil || ev.Level != content.Sting {
		t.Fatalf("over the sting line: %+v", ev)
	}
	first := over(w, s, sting, home)*(1-s.Decay(w)) - home.Heat
	cool := s.CooldownDays(w, content.Sting)
	for i := 1; i < cool; i++ {
		home.Heat = over(w, s, sting, home)
		if ev := enforcement(step(w, s, sale(w, home.ID, 10))); ev != nil && ev.Level == content.Sting {
			t.Fatalf("day %d after a sting, %d days' cooldown: it fired again", i, cool)
		}
	}
	home.Heat = over(w, s, sting, home)
	tk = step(w, s, sale(w, home.ID, 10))
	if ev := enforcement(tk); ev == nil || ev.Level != content.Sting || w.Heat.Responses[content.Sting] != 2 {
		t.Fatalf("after the cooldown: %+v, responses %v", ev, w.Heat.Responses)
	}
	if second := over(w, s, sting, home)*(1-s.Decay(w)) - home.Heat; !(second < first) {
		t.Fatalf("the second sting cooled %.2f, the first %.2f", second, first)
	}

	// The rung above fires inside the sting's cooldown: it has its own.
	home.Heat = over(w, s, rung(cfg, content.Raid), home)
	if ev := enforcement(step(w, s, sale(w, home.ID, 10))); ev == nil || ev.Level != content.Raid {
		t.Fatalf("over the raid line inside the sting's cooldown: %+v", ev)
	}

	// The arrest, with a fall guy: he takes it, the file is wiped, heat
	// drops to 50 and half the cash goes; without one, the run ends.
	w.Upgrades["fallguy"] = true
	w.Player.DirtyCash, w.Player.CleanCash = 1000, 500
	w.Heat.Evidence = 3
	home.Heat = over(w, s, rung(cfg, content.Arrest), home)
	tk = step(w, s)
	if ev := enforcement(tk); ev != nil || w.Over != nil || w.FallsTaken != 1 || w.Heat.Evidence != 0 || home.Heat > 50 || w.Player.DirtyCash != 500 || w.Player.CleanCash != 250 {
		t.Fatalf("the fall guy: enforcement %+v over %+v falls %d file %d heat %.0f cash %d/%d", ev, w.Over, w.FallsTaken, w.Heat.Evidence, home.Heat, w.Player.DirtyCash, w.Player.CleanCash)
	}
	burned := false
	for _, e := range tk.Events() {
		_, burned = e.(events.FallGuyBurned)
		if burned {
			break
		}
	}
	if !burned {
		t.Fatal("the fall guy's fall was not reported")
	}
	home.Heat = over(w, s, rung(cfg, content.Arrest), home)
	tk = step(w, s)
	if ev := enforcement(tk); ev == nil || ev.Level != content.Arrest || w.Over == nil || w.Over.Cause != "arrested" {
		t.Fatalf("the arrest: %+v over %+v", ev, w.Over)
	}
}

// The file: enough pages is an indictment on any day, whatever the
// heat; a retained lawyer lets a page go cold after evidence_decay_days
// without a new one; the lawyer's evidence_cut thins what a sting
// files, to a floor of nothing.
func TestTheFile(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	w := world(t, cfg)
	w.Heat.Evidence = s.EvidenceArrest(w)
	tk := step(w, s)
	if ev := enforcement(tk); ev == nil || ev.Level != content.Arrest || w.Over == nil || w.Over.Cause != "indicted" {
		t.Fatalf("a full file: %+v over %+v", ev, w.Over)
	}

	w = world(t, cfg)
	w.Upgrades["retainer"] = true
	days := game.FoldEffects(w, cfg.Upgrades).EvidenceDecayDays
	if days <= 0 {
		t.Fatal("the retainer has no decay")
	}
	w.Heat.Evidence, w.Heat.EvidenceDay = 2, w.Day
	for i := 1; i < days; i++ {
		step(w, s)
	}
	if w.Heat.Evidence != 2 {
		t.Fatalf("a page went cold after %d days, the retainer says %d", days-1, days)
	}
	step(w, s)
	if w.Heat.Evidence != 1 || w.Heat.EvidenceDay != w.Day {
		t.Fatalf("after %d days the file reads %d, last grew day %d", days, w.Heat.Evidence, w.Heat.EvidenceDay)
	}

	w = world(t, cfg)
	w.Upgrades["lawyer"], w.Upgrades["paper"] = true, true
	sting := rung(cfg, content.Sting)
	w.Home().Heat = over(w, s, sting, w.Home())
	tk = step(w, s, sale(w, w.Home().ID, 10))
	if ev := enforcement(tk); ev == nil || ev.Level != content.Sting || ev.Evidence != max(0, sting.Evidence-2) || w.Heat.Evidence != ev.Evidence {
		t.Fatalf("a sting with two lawyers: %+v, file %d", ev, w.Heat.Evidence)
	}
}

// Decay, the floor and the cap: a fraction of what is above the floor
// fades every day in every city; lying low fades it faster and is
// reported; a feared name never cools under the floor; nothing goes
// over 100; the peak is the hottest any city has been.
func TestDecayAndTheFloor(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	tun := cfg.Heat.Heat
	w := world(t, cfg)
	home, hub := w.Home(), w.Cities[w.CityOrder[1]]
	home.Heat, hub.Heat = 30, 20
	step(w, s)
	if !near(home.Heat, 30*(1-tun.Decay)) || !near(hub.Heat, 20*(1-tun.Decay)) {
		t.Fatalf("after a day: home %.3f hub %.3f, decay %.2f", home.Heat, hub.Heat, tun.Decay)
	}
	home.Heat = 30
	w.SetLieLow(true)
	tk := step(w, s)
	laid := false
	for _, e := range tk.Events() {
		_, ok := e.(events.LaidLow)
		laid = laid || ok
	}
	if !laid || !near(home.Heat, 30*(1-tun.Decay*tun.LieLowMultiplier)) {
		t.Fatalf("lying low: reported %v, home %.3f, want %.3f", laid, home.Heat, 30*(1-tun.Decay*tun.LieLowMultiplier))
	}
	if w.Today.LieLow {
		t.Fatal("the clock did not clear the lie-low")
	}

	w.Player.Reputation.Fear = 100
	floor := s.Floor(w)
	if floor <= 0 || !near(floor, cfg.Reputation.Effects.FearHeatFloor) {
		t.Fatalf("the floor at full fear is %.2f", floor)
	}
	home.Heat = floor + 10
	step(w, s)
	if !near(home.Heat, floor+10*(1-tun.Decay)) {
		t.Fatalf("over the floor: %.3f, want %.3f", home.Heat, floor+10*(1-tun.Decay))
	}
	home.Heat = floor / 2
	step(w, s)
	if !near(home.Heat, floor) {
		t.Fatalf("under the floor: %.3f, the floor is %.3f", home.Heat, floor)
	}
	w.Player.Reputation.Fear = 0
	home.Heat = 40
	for _, cid := range w.CityOrder {
		w.Cities[cid].Heat = 0
	}
	home.Heat = 40
	w.Player.DirtyCash = 100_000_000 // a pile: heat where you are, past 100
	tk = step(w, s)
	if home.Heat > 100 || w.Heat.Peak != 100 {
		t.Fatalf("the cap: home %.1f peak %.1f", home.Heat, w.Heat.Peak)
	}
	for _, e := range tk.Events() {
		if hc, ok := e.(events.HeatChanged); ok && hc.City == home.ID && (hc.From != 40 || hc.To != home.Heat) {
			t.Fatalf("the report: %+v", hc)
		}
	}
}

// The informant's clock (#13): a turn starts it; every informant_days
// the DA gets informant_evidence pages whatever was sold and whatever
// the lawyer says, with informant_heat where you are that the reasons
// leave out; the count of leaks shows the tell; once nobody is talking
// it resets and the clock is stamped every day, so an accountant the
// audit flips without an event is on the clock from that night. A
// flipped lieutenant feeds the thicker page.
func TestInformantClock(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	tun := cfg.Heat.Heat
	w := world(t, cfg)
	w.Upgrades["lawyer"] = true
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 1, Name: "Runner", Role: "runner", Skill: 50})
	home := w.Home()
	// Nobody talking: the stamp moves with the day, no leak.
	step(w, s)
	if w.Heat.LeakDay != w.Day || w.Heat.Leaks != 0 {
		t.Fatalf("nobody talking: leak day %d (day %d), leaks %d", w.Heat.LeakDay, w.Day, w.Heat.Leaks)
	}
	// A runner turns: the crew sim's event starts the clock.
	w.Crew.Members[0].Informant = true
	turned := w.Day + 1
	step(w, s, events.CrewTurnedInformant{Day: turned, ID: w.Crew.Members[0].ID, Name: w.Crew.Members[0].Name})
	if w.Heat.LeakDay != turned || w.Heat.Evidence != 0 {
		t.Fatalf("the turn: leak day %d, file %d", w.Heat.LeakDay, w.Heat.Evidence)
	}
	for i := 1; i < tun.InformantDays; i++ {
		step(w, s)
		if w.Heat.Evidence != 0 || w.Heat.Leaks != 0 {
			t.Fatalf("day %d of %d: the DA already has %d pages, %d leaks", i, tun.InformantDays, w.Heat.Evidence, w.Heat.Leaks)
		}
	}
	home.Heat = 10
	before := home.Heat * (1 - s.Decay(w))
	tk := step(w, s)
	if w.Heat.Evidence != tun.InformantEvidence || w.Heat.Leaks != 1 || w.Heat.LeakDay != w.Day || w.Heat.EvidenceDay != w.Day {
		t.Fatalf("the leak: file %d leaks %d leak day %d evidence day %d (day %d)", w.Heat.Evidence, w.Heat.Leaks, w.Heat.LeakDay, w.Heat.EvidenceDay, w.Day)
	}
	if !near(home.Heat, before+tun.InformantHeat*(1-s.Decay(w))) {
		t.Fatalf("the leak's heat: %.3f, want %.3f", home.Heat, before+tun.InformantHeat*(1-s.Decay(w)))
	}
	for _, e := range tk.Events() {
		hc, ok := e.(events.HeatChanged)
		if !ok || hc.City != home.ID {
			continue
		}
		if len(hc.Reasons) != 1 {
			t.Fatalf("the report on the leak: %v (want the file's line alone: the heat is the tell)", hc.Reasons)
		}
	}
	for i := 0; i < tun.InformantDays; i++ {
		step(w, s)
	}
	if w.Heat.Leaks != 2 || w.Heat.Evidence != 2*tun.InformantEvidence {
		t.Fatalf("the second leak: leaks %d file %d", w.Heat.Leaks, w.Heat.Evidence)
	}
	// Fired: nobody is talking, the count resets, the file stays.
	w.Crew.Members[0].Informant = false
	step(w, s)
	if w.Heat.Leaks != 0 || w.Heat.LeakDay != w.Day || w.Heat.Evidence != 2*tun.InformantEvidence {
		t.Fatalf("nobody talking again: leaks %d leak day %d file %d", w.Heat.Leaks, w.Heat.LeakDay, w.Heat.Evidence)
	}
	// A lieutenant flips: their page is the lieutenant tuning's.
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 99, Name: "Lt", Role: game.RoleLieutenant, Informant: true})
	step(w, s, events.LieutenantFlipped{Day: w.Day + 1, ID: 99, Name: "Lt"})
	file := w.Heat.Evidence
	for i := 0; i < tun.InformantDays; i++ {
		step(w, s)
	}
	if want := max(tun.InformantEvidence, cfg.Crew.Lieutenant.Evidence); w.Heat.Evidence != file+want {
		t.Fatalf("the lieutenant's leak: %d pages, want %d", w.Heat.Evidence-file, want)
	}
}

// The cash pile's line: dirty cash up to dirty_cash_threshold, raised
// by the Security branch and covered by dirty_cash_cover times what the
// fronts cost, draws nothing; past it, dirty_cash_heat per threshold
// multiple over, where you stand. It is heat and never a page (#27).
func TestCashPileLine(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	tun := cfg.Heat.Heat
	w := world(t, cfg)
	home := w.Home()
	thr := s.DirtyCashThreshold(w)
	if thr != tun.DirtyCashThreshold {
		t.Fatalf("the line is %d, the file says %d", thr, tun.DirtyCashThreshold)
	}
	heatAfter := func(dirty int, cover int) float64 {
		w.Player.DirtyCash = dirty
		w.Fronts = nil
		if cover > 0 {
			w.Fronts = []game.Front{{ID: "f", Name: "F", Cost: cover}}
		}
		home.Heat = 0
		step(w, s)
		return home.Heat
	}
	if got := heatAfter(thr, 0); got != 0 {
		t.Fatalf("on the line: %.3f heat", got)
	}
	over := thr * 3
	want := tun.DirtyCashHeat * float64(over-thr) / float64(thr) * (1 - s.Decay(w))
	if got := heatAfter(over, 0); !near(got, want) {
		t.Fatalf("at %d: %.3f heat, want %.3f", over, got, want)
	}
	front := int(math.Ceil(float64(over-thr) / tun.DirtyCashCover))
	if got := heatAfter(over, front); got != 0 || s.Cover(w) < over-thr {
		t.Fatalf("with a front at $%d covering $%d: %.3f heat", front, s.Cover(w), got)
	}
	w.Upgrades["quietmoney"] = true
	if raised := s.DirtyCashThreshold(w); raised <= thr || heatAfter(raised, 0) != 0 {
		t.Fatalf("quiet money: the line is %d (was %d)", raised, thr)
	}
	if w.Heat.Evidence != 0 {
		t.Fatalf("the pile filed %d pages", w.Heat.Evidence)
	}
}

// The raid's place (#73): a sting or a raid hits the street when there
// is no house holding anything, no roll; a house the police know; with
// an informant on the payroll the fullest house, which becomes known,
// whole, whatever the safehouse would save; else one roll off the
// houses' side stream over the places holding stock, a house empty of
// stock never a candidate, so it never shields the street.
func TestRaidPlace(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	raid := rung(cfg, content.Raid)
	product := "weed"
	offers := func(w *game.World) []game.HouseOffer {
		var out []game.HouseOffer
		for _, o := range cfg.Houses.Offers {
			if o.City == w.Home().ID {
				out = append(out, game.HouseOffer{ID: o.ID, Name: o.Name, City: o.City, Corner: o.Corner, Capacity: o.Capacity, Price: o.Price, Rent: o.Rent})
			}
		}
		return out
	}
	fresh := func() *game.World {
		w := world(t, cfg)
		w.Player.DirtyCash = 10_000_000
		w.Stats.PeakCash = w.Player.DirtyCash
		return w
	}
	var last *game.Tick
	hit := func(w *game.World) events.Enforcement {
		t.Helper()
		w.Home().Heat = over(w, s, raid, w.Home())
		w.Heat.LastResponse = map[string]int{}
		w.Player.DirtyCash = 1000 // the pile that bought the houses would be heat of its own
		last = step(w, s)
		ev := enforcement(last)
		if ev == nil || ev.Level != content.Raid {
			t.Fatalf("no raid: %+v", ev)
		}
		return *ev
	}

	// The street, with no house at all and with an empty one.
	w := fresh()
	w.SetStock(w.Home().ID, product, 100)
	if ev := hit(w); ev.House != "" || ev.StockLost[product] != int(math.Round(100*raid.StockLoss)) {
		t.Fatalf("no house: %+v", ev)
	}
	w.Player.DirtyCash = 10_000_000
	if _, err := w.BuyHouse(offers(w)[0]); err != nil {
		t.Fatal(err)
	}
	w.SetStock(w.Home().ID, product, 100)
	if ev := hit(w); ev.House != "" || w.Houses[0].Raided != 0 {
		t.Fatalf("an empty house shielded the street: %+v", ev)
	}

	// A house the police know, over the street and an unknown house.
	w = fresh()
	for _, o := range offers(w)[:2] {
		if _, err := w.BuyHouse(o); err != nil {
			t.Fatal(err)
		}
	}
	w.SetStock(w.Home().ID, product, 100)
	w.Houses[0].Stock = map[string]int{product: 50}
	w.Houses[1].Stock = map[string]int{product: 200}
	w.Houses[0].Known = true
	ev := hit(w)
	if ev.House != w.Houses[0].ID || ev.HouseName != w.Houses[0].Name || ev.StockLost[product] != int(math.Round(50*raid.StockLoss)) || w.Houses[0].Raided != 1 || w.Stock(w.Home().ID, product) != 350-ev.StockLost[product] {
		t.Fatalf("the known house: %+v, houses %+v", ev, w.Houses)
	}

	// An informant: the fullest house, made known, emptied whole.
	w = fresh()
	for _, o := range offers(w)[:2] {
		if _, err := w.BuyHouse(o); err != nil {
			t.Fatal(err)
		}
	}
	w.Upgrades["safehouse"] = true
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 1, Name: "Runner", Role: "runner", Skill: 50, Informant: true})
	w.SetStock(w.Home().ID, product, 100)
	w.Houses[0].Stock = map[string]int{product: 50}
	w.Houses[1].Stock = map[string]int{product: 200}
	ev = hit(w)
	if ev.House != w.Houses[1].ID || !ev.Stash || ev.StockLost[product] != 200 || w.Houses[1].Units() != 0 || !w.Houses[1].Known || w.Houses[0].Units() != 50 || w.Street(w.Home().ID, product) != 100 {
		t.Fatalf("told: %+v, houses %+v street %d", ev, w.Houses, w.Street(w.Home().ID, product))
	}
	compromised, whole := false, false
	for _, e := range last.Events() {
		switch x := e.(type) {
		case events.HouseCompromised:
			compromised = compromised || (x.Why == "informant" && x.House == w.Houses[1].ID)
		case events.HouseRaided:
			whole = whole || (x.Whole && x.House == w.Houses[1].ID)
		}
	}
	if !compromised || !whole {
		t.Fatalf("told: compromised %v whole %v", compromised, whole)
	}

	// The roll: unknown houses holding stock and the street; every
	// place holding something can be hit, and over many seeds each is.
	seen := map[string]int{}
	for seed := uint64(1); seed <= 40; seed++ {
		w := fresh()
		w.Seed = seed
		for _, o := range offers(w)[:2] {
			if _, err := w.BuyHouse(o); err != nil {
				t.Fatal(err)
			}
		}
		w.SetStock(w.Home().ID, product, 100)
		w.Houses[0].Stock = map[string]int{product: 100}
		w.Houses[1].Stock = map[string]int{product: 100}
		seen[hit(w).House]++
	}
	if len(seen) != 3 {
		t.Fatalf("over 40 seeds the raid hit %v; the roll should reach the street and both houses", seen)
	}
}

// The bought law (#42): a bought chief speeds the decay by
// bribe_decay_mul and adds bribe_cooldown to the sting and raid
// cooldowns at their share (a lazy chief's half), never the patrol's; a
// bought DA needs bribed_da_evidence_mul the pages; the cold ends both;
// and the morning after an envelope came back the file gains
// backfire_evidence and the city backfire_heat whatever was sold, as
// the morning after the DA files the leads gains lead_evidence: the
// second bend in #27 besides the informant's.
func TestBoughtLaw(t *testing.T) {
	cfg := content.MustLoad()
	s := heat.New(cfg)
	fx := cfg.Law.Effects
	b := cfg.Law.Bribes
	w := world(t, cfg)
	w.Day = 5 // the cold is a day, and day 0 is none
	w.Law.Chief.Personality = "lazy"
	decay, cool, patrol, arrest := s.Decay(w), s.CooldownDays(w, content.Sting), s.CooldownDays(w, content.Patrol), s.EvidenceArrest(w)
	w.Law.ChiefBought, w.Law.ChiefShare = w.Day+10, 1
	if got := s.Decay(w); !near(got, math.Min(1, decay*fx.BribeDecayMul)) {
		t.Fatalf("decay under a bought chief %.4f, want %.4f", got, decay*fx.BribeDecayMul)
	}
	if got := s.CooldownDays(w, content.Sting); got != cool+fx.BribeCooldown {
		t.Fatalf("sting cooldown under a bought chief %d, want %d", got, cool+fx.BribeCooldown)
	}
	if got := s.CooldownDays(w, content.Patrol); got != patrol {
		t.Fatalf("the patrol's cadence moved under a bought chief: %d, want %d", got, patrol)
	}
	w.Law.ChiefShare = b.LazyEffect
	if got := s.Decay(w); !near(got, math.Min(1, decay*(1+(fx.BribeDecayMul-1)*b.LazyEffect))) {
		t.Fatalf("decay under a lazy bought chief %.4f", got)
	}
	if got := s.CooldownDays(w, content.Sting); got != cool+int(math.Round(float64(fx.BribeCooldown)*b.LazyEffect)) {
		t.Fatalf("sting cooldown under a lazy bought chief %d", got)
	}
	w.Law.DABought = w.Day + 10
	if got := s.EvidenceArrest(w); got != max(1, int(math.Round(float64(arrest)*fx.BribedDAEvidenceMul))) {
		t.Fatalf("pages under a bought DA %d, want %d x %.2f", got, arrest, fx.BribedDAEvidenceMul)
	}
	w.Law.Cold = w.Day
	if s.Decay(w) != decay || s.CooldownDays(w, content.Sting) != cool || s.EvidenceArrest(w) != arrest {
		t.Fatalf("the cold did not end the deals: decay %.4f cooldown %d pages %d", s.Decay(w), s.CooldownDays(w, content.Sting), s.EvidenceArrest(w))
	}

	// The morning after: no sale anywhere, and the file grows.
	w = world(t, cfg)
	w.Law.Backfired = w.Day + 1 // the law's night is our morning: the tick that follows reads Backfired == t.Day-1
	heat := w.Here().Heat
	step(w, s)
	if w.Heat.Evidence != 0 || w.Here().Heat > heat {
		t.Fatalf("the morning of the backfire itself: evidence %d heat %.2f (was %.2f)", w.Heat.Evidence, w.Here().Heat, heat)
	}
	heat = w.Here().Heat
	tk := step(w, s)
	if w.Heat.Evidence != b.BackfireEvidence || w.Heat.EvidenceDay != w.Day || w.Here().Heat <= heat {
		t.Fatalf("the morning after a backfire: evidence %d (want %d) heat %.2f (was %.2f)", w.Heat.Evidence, b.BackfireEvidence, w.Here().Heat, heat)
	}
	for _, e := range tk.Events() {
		if ev, ok := e.(events.HeatChanged); ok && ev.City == w.Player.Location {
			found := false
			for _, r := range ev.Reasons {
				if strings.Contains(r, "backfired") {
					found = true
				}
			}
			if !found {
				t.Fatalf("the reasons do not name the backfire: %v", ev.Reasons)
			}
		}
	}
	step(w, s)
	if w.Heat.Evidence != b.BackfireEvidence {
		t.Fatalf("a backfire filed twice: %d", w.Heat.Evidence)
	}
	w.Law.Filed = w.Day + 1
	step(w, s)
	step(w, s)
	if w.Heat.Evidence != b.BackfireEvidence+b.LeadEvidence {
		t.Fatalf("the morning after the file opened: evidence %d, want %d", w.Heat.Evidence, b.BackfireEvidence+b.LeadEvidence)
	}
}
