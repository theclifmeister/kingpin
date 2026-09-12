package harness

import (
	"fmt"
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// runCity is a world with a lieutenant of the given temper running the
// hub from day 0 with three runners to post, and the player at home
// placing no orders. The policy keeps the hub stash topped up and lies
// low at 50, so what the hub sells, and what it costs, is the
// lieutenant's doing: the same seed under two tempers differs only by
// the temper.
func runCity(t *testing.T, cfg *content.Config, seed uint64, personality string) (*game.World, Policy) {
	t.Helper()
	_, hub, _ := twoCities(t, cfg)
	w := sim.NewWorld(cfg, seed)
	w.Player.DirtyCash = 300_000
	// Every rung on offer from day one: a product that unlocks mid-run
	// would unlock on a different day for the temper that skims, and the
	// tempers are meant to differ in nothing but what they do.
	for _, p := range cfg.Market.Products {
		w.Stats.PeakCash = max(w.Stats.PeakCash, p.UnlockCash)
	}
	for i, name := range []string{"Dre", "Tank", "Sly"} {
		w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 100 + i, Name: name, Role: "runner", Skill: 60, Units: 100, Loyalty: 90, Nerve: 60, Wage: 50})
	}
	w.Crew.NextID = 200
	lt := Delegate(cfg, w, hub, personality)
	w.Crew.Member(lt.ID).Loyalty = 90
	w.Crew.Member(lt.ID).Nerve = 90
	return w, func(w *game.World) {
		for _, id := range cfg.Market.Products[:3] {
			w.SetStock(hub, id.ID, 300)
		}
		if w.MaxHeat() >= 50 {
			w.SetLieLow(true)
		}
	}
}

// A lieutenant sells in their city on days the player does nothing
// there: the delegated player never places an order at home once the
// lieutenant has it, and home sells anyway, on their standing orders;
// unassigned every day, they place none and home sells nothing.
func TestLieutenantSellsWhileYouAreAway(t *testing.T) {
	cfg := content.MustLoad()
	home := cfg.City.Home().ID
	pol := Delegated(cfg, 40, "steady")
	for seed := uint64(1); seed <= 3; seed++ {
		hired := 0
		res, err := Run(cfg, seed, Horizon, func(w *game.World) {
			pol(w)
			if hired == 0 && w.Crew.Lieutenant(home) != nil {
				hired = w.Day
			}
			if hired > 0 {
				for _, o := range w.Today.Orders {
					if o.City == home {
						t.Fatalf("seed %d day %d: the delegated player placed an order at home", seed, w.Day)
					}
				}
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if hired == 0 || hired > 150 {
			t.Fatalf("seed %d: a lieutenant was hired on day %d", seed, hired)
		}
		standing, own := 0, 0
		for _, e := range res.Events {
			ps, ok := e.(events.PlayerSold)
			if !ok || ps.City != home || ps.Day <= hired+1 || ps.Sold == 0 {
				continue
			}
			if ps.Standing {
				standing++
			} else {
				own++
			}
			if ps.Lieutenant == 0 || ps.LieutenantName == "" {
				t.Fatalf("seed %d day %d: a sale at home under a lieutenant names nobody: %+v", seed, ps.Day, ps)
			}
		}
		if standing < 20 || own != 0 {
			t.Fatalf("seed %d: %d sales on standing orders and %d on the player's after the lieutenant took home on day %d", seed, standing, own, hired)
		}

		// Unassigned every evening, nothing sells at home on any day a
		// lieutenant was on the payroll (the policy sells there itself
		// on the days between one it fired and the next it hired).
		w := sim.NewWorld(cfg, seed)
		payroll := map[int]bool{}
		res, _ = RunFrom(cfg, w, Horizon, func(w *game.World) {
			pol(w)
			if lt := w.Crew.Lieutenant(home); lt != nil {
				_ = w.Unassign(lt.ID)
				payroll[w.Day+1] = true
			}
		})
		for _, e := range res.Events {
			if ps, ok := e.(events.PlayerSold); ok && ps.City == home && payroll[ps.Day] && ps.Sold > 0 {
				t.Fatalf("seed %d day %d: home sold %d with the lieutenant unassigned: %+v", seed, ps.Day, ps.Sold, ps)
			}
		}
		if len(payroll) < 20 {
			t.Fatalf("seed %d: a lieutenant was on the payroll on only %d days", seed, len(payroll))
		}
	}
}

// The temper shapes the city, on the same seed and policy: a violent
// lieutenant's city is hotter than a careful one's at day 60, a greedy
// one's takings are no more than a steady one's (the skim), and a careful
// one earns less than a steady one (the quiet dial).
func TestLieutenantPersonalities(t *testing.T) {
	cfg := content.MustLoad()
	_, hub, _ := twoCities(t, cfg)
	type outcome struct {
		heat    float64
		revenue int
		cash    int
		skimmed int
	}
	play := func(seed uint64, personality string) outcome {
		w, pol := runCity(t, cfg, seed, personality)
		res, err := RunFrom(cfg, w, 60, pol)
		if err != nil {
			t.Fatal(err)
		}
		if res.Over != nil {
			t.Fatalf("seed %d %s: the run ended on day %d: %s", seed, personality, res.Days, res.Over.Cause)
		}
		o := outcome{heat: res.World.City(hub).Heat, cash: res.World.Cash()}
		for _, e := range res.Events {
			if ps, ok := e.(events.PlayerSold); ok && ps.City == hub {
				if !ps.Standing || ps.Lieutenant == 0 {
					t.Fatalf("seed %d %s day %d: a hub sale that was not the lieutenant's: %+v", seed, personality, ps.Day, ps)
				}
				o.revenue += ps.Revenue
			}
			if la, ok := e.(events.LieutenantActed); ok {
				o.skimmed += la.Skimmed
			}
		}
		if o.revenue == 0 {
			t.Fatalf("seed %d %s: the lieutenant sold nothing in 60 days", seed, personality)
		}
		return o
	}
	for seed := uint64(1); seed <= 5; seed++ {
		violent, greedy, careful, steady := play(seed, "violent"), play(seed, "greedy"), play(seed, "careful"), play(seed, "steady")
		t.Logf("seed %d: heat at day 60 violent %.0f careful %.0f; revenue greedy %d steady %d careful %d; cash greedy %d steady %d",
			seed, violent.heat, careful.heat, greedy.revenue, steady.revenue, careful.revenue, greedy.cash, steady.cash)
		if violent.heat < careful.heat {
			t.Errorf("seed %d: violent city heat %.1f under careful %.1f", seed, violent.heat, careful.heat)
		}
		if greedy.revenue != steady.revenue || greedy.cash >= steady.cash {
			t.Errorf("seed %d: greedy took %d and kept %d, steady took %d and kept %d; greedy should skim, not sell differently", seed, greedy.revenue, greedy.cash, steady.revenue, steady.cash)
		}
		if greedy.skimmed == 0 || steady.cash-greedy.cash != greedy.skimmed || steady.skimmed != 0 {
			t.Errorf("seed %d: greedy skimmed %d and is %d short of steady (which skimmed %d); the skim should be taken exactly once", seed, greedy.skimmed, steady.cash-greedy.cash, steady.skimmed)
		}
		if careful.revenue >= steady.revenue {
			t.Errorf("seed %d: careful took %d, steady %d", seed, careful.revenue, steady.revenue)
		}
	}
}

// A lieutenant at loyalty 0 walks the next night, within drift_days, and
// every corner they ran is unheld the next morning with the stash gone;
// the runners they posted are still on the payroll, idle.
func TestLieutenantWalks(t *testing.T) {
	cfg := content.MustLoad()
	_, hub, _ := twoCities(t, cfg)
	w, pol := runCity(t, cfg, 2, "steady")
	lt := w.Crew.Lieutenant(hub)
	res, err := RunFrom(cfg, w, 3, pol)
	if err != nil {
		t.Fatal(err)
	}
	if n := res.World.WorkedIn(hub); n != 3 {
		t.Fatalf("after three nights the lieutenant works %d corners, want 3", n)
	}
	res.World.Crew.Member(lt.ID).Loyalty = 0
	// A house in the hub (#73): the walk empties it as it empties the
	// street.
	res.World.Player.CleanCash += 10_000
	if _, err := res.World.BuyHouse(game.HouseOffer{ID: "hubhouse", Name: "Hub house", City: hub, Corner: cfg.City.City(hub).Corners[0].ID, Capacity: 200, Price: 1, Rent: 1}); err != nil {
		t.Fatal(err)
	}
	res.World.Today.HousesBought = nil
	res.World.AddStock(hub, cfg.Market.Products[0].ID, 150)
	if res.World.House("hubhouse").Units() != 150 {
		t.Fatal("the stock did not go into the house")
	}
	res, err = RunFrom(cfg, res.World, cfg.City.Territory.DriftDays, func(w *game.World) {
		pol(w)
		if w.Day == 3 {
			for _, c := range w.City(hub).Corners {
				if c.Held() && c.Robbed < 2 {
					return
				}
			}
			t.Fatal("no corner held in the hub on the day the lieutenant walks")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	walked := 0
	for _, e := range res.Events {
		if ev, ok := e.(events.LieutenantWalked); ok {
			walked++
			if ev.Day != 4 || ev.Name != lt.Name || ev.City != hub || len(ev.Corners) == 0 || ev.Units == 0 || ev.Rival != "" {
				t.Fatalf("walk: %+v", ev)
			}
		}
	}
	if walked != 1 {
		t.Fatalf("%d walks", walked)
	}
	w = res.World
	if w.Crew.Member(lt.ID) != nil || w.Crew.Lieutenant(hub) != nil || w.Stats.Walked != 1 {
		t.Fatal("the lieutenant is still on the payroll")
	}
	for _, c := range w.City(hub).Corners {
		if c.Held() || c.Runner != 0 || c.Enforcer != 0 {
			t.Fatalf("the morning after: %+v", c)
		}
	}
	if len(w.Crew.Members) != 3 {
		t.Fatalf("%d on the payroll after the walk, want the three runners", len(w.Crew.Members))
	}
	if h := w.House("hubhouse"); h == nil || h.Units() != 0 {
		t.Fatalf("the house in the hub after the walk: %+v", h)
	}
	for _, m := range w.Crew.Members {
		if w.PostOf(m.ID) != nil {
			t.Fatalf("%s is still posted", m.Name)
		}
	}
	// The stash was emptied when they walked; the policy has refilled it
	// since, but nothing sells with nobody on a corner.
	for _, e := range res.Events {
		if ps, ok := e.(events.PlayerSold); ok && ps.City == hub && ps.Day > 4 && ps.Sold > 0 {
			t.Fatalf("day %d: the hub sold %d after the lieutenant walked", ps.Day, ps.Sold)
		}
	}
	if _, ok := w.StandingOrder(hub, cfg.Market.Products[0].ID); ok {
		t.Fatal("a standing order survived the walk")
	}
}

// The player's order for a product wins the day over the lieutenant's
// standing one; the other products still sell on theirs.
func TestPlayerOrdersWinOverLieutenant(t *testing.T) {
	cfg := content.MustLoad()
	_, hub, _ := twoCities(t, cfg)
	weed, pills := cfg.Market.Products[0].ID, cfg.Market.Products[1].ID
	w, pol := runCity(t, cfg, 3, "steady")
	res, err := RunFrom(cfg, w, 4, func(w *game.World) {
		pol(w)
		if w.Day == 3 {
			if _, ok := w.StandingOrder(hub, weed); !ok {
				t.Fatal("no standing order for weed on day 3")
			}
			if err := w.PlaceSell(hub, weed, 1, events.DialQuiet); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	var yours, theirs *events.PlayerSold
	for _, e := range res.Events {
		if ps, ok := e.(events.PlayerSold); ok && ps.City == hub && ps.Day == 4 {
			ps := ps
			switch ps.Product {
			case weed:
				yours = &ps
			case pills:
				theirs = &ps
			}
		}
	}
	if yours == nil || yours.Standing || yours.Wanted != 1 || yours.Dial != events.DialQuiet || yours.Lieutenant == 0 {
		t.Fatalf("your order: %+v", yours)
	}
	if theirs == nil || !theirs.Standing || theirs.Wanted == 0 || theirs.Dial != events.DialNormal {
		t.Fatalf("their order: %+v", theirs)
	}
}

// A lieutenant under the flip line turns without dice, on the night it
// happens, and from then on feeds the DA thicker pages than an ordinary
// informant, on the informant clock.
func TestLieutenantFlipsAndFeedsTheFile(t *testing.T) {
	cfg := content.MustLoad()
	_, hub, _ := twoCities(t, cfg)
	w, pol := runCity(t, cfg, 4, "steady")
	lt := w.Crew.Lieutenant(hub)
	lt.Loyalty = cfg.Crew.Lieutenant.Flip - 1
	lt.Nerve = 100 // never the ordinary way
	days := cfg.Heat.Heat.InformantDays + 1
	res, err := RunFrom(cfg, w, days, pol)
	if err != nil {
		t.Fatal(err)
	}
	flipped, turned := 0, 0
	for _, e := range res.Events {
		switch ev := e.(type) {
		case events.LieutenantFlipped:
			flipped++
			if ev.Day != 1 || ev.ID != lt.ID || ev.City != hub {
				t.Fatalf("flip: %+v", ev)
			}
		case events.CrewTurnedInformant:
			turned++
		case events.Enforcement:
			if ev.Evidence > 0 {
				t.Skipf("a %s on day %d added evidence of its own", ev.Level, ev.Day)
			}
		}
	}
	if flipped != 1 || turned != 0 {
		t.Fatalf("%d flips, %d ordinary turns", flipped, turned)
	}
	m := res.World.Crew.Member(lt.ID)
	if m == nil || !m.Informant || res.World.Crew.Informants() != 1 {
		t.Fatalf("after the flip: %+v", m)
	}
	if got, want := res.World.Heat.Evidence, cfg.Crew.Lieutenant.Evidence; got != want {
		t.Fatalf("the file holds %d pages after %d days, want %d: a flipped lieutenant's leak", got, days, want)
	}
}

// Lieutenants only come looking once corners are held in two cities: a
// run that stays home never sees one, the delegated run does.
func TestLieutenantsWantTwoCities(t *testing.T) {
	cfg := content.MustLoad()
	seen := func(res Result) bool {
		for _, e := range res.Events {
			if h, ok := e.(events.CrewHired); ok && h.Role == game.RoleLieutenant {
				return true
			}
		}
		for _, c := range res.World.Crew.Candidates {
			if c.Lieutenant() {
				return true
			}
		}
		return false
	}
	for seed := uint64(1); seed <= 3; seed++ {
		home, _ := Run(cfg, seed, Horizon, Laundered(cfg, 40))
		if seen(home) {
			t.Fatalf("seed %d: a lieutenant came looking for a player who never left home", seed)
		}
		away, _ := Run(cfg, seed, Horizon, Delegated(cfg, 40, ""))
		if !seen(away) {
			t.Fatalf("seed %d: no lieutenant ever came looking for the delegated player", seed)
		}
	}
}

// Assignment and temper survive a save, and the run plays on identically.
func TestLieutenantSurvivesSave(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	home := cfg.City.Home().ID
	pol := Delegated(cfg, 40, "")
	a, err := Run(cfg, 1, 160, pol)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Run(cfg, 1, 120, pol)
	lt := b.World.Crew.Lieutenant(home)
	if lt == nil {
		t.Fatal("no lieutenant running home on day 120 to save")
	}
	if len(b.World.Delegated) == 0 {
		t.Fatal("no standing orders on day 120 to save")
	}
	if err := game.Save(1, b.World); err != nil {
		t.Fatal(err)
	}
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	got := loaded.Crew.Lieutenant(home)
	if got == nil || got.ID != lt.ID || got.Personality != lt.Personality || got.City != lt.City || got.Assigned != lt.Assigned || got.Observed != lt.Observed {
		t.Fatalf("loaded lieutenant %+v, saved %+v", got, lt)
	}
	if fmt.Sprint(loaded.Delegated) != fmt.Sprint(b.World.Delegated) {
		t.Fatalf("standing orders %v, saved %v", loaded.Delegated, b.World.Delegated)
	}
	c, _ := RunFrom(cfg, loaded, 40, pol)
	rest := a.Events[len(b.Events):]
	if len(c.Events) != len(rest) {
		t.Fatalf("after loading, %d events for the last 40 days, want %d", len(c.Events), len(rest))
	}
	for i := range rest {
		if fmt.Sprintf("%#v", rest[i]) != fmt.Sprintf("%#v", c.Events[i]) {
			t.Fatalf("event %d after the save differs:\n%#v\n%#v", i, rest[i], c.Events[i])
		}
	}
}

// Delegation costs the cut, not the city: with a steady lieutenant the
// delegated player is within 20% of the distributor on median net worth
// at the horizon, and is never indicted.
func TestDelegatedNearDistributor(t *testing.T) {
	cfg := content.MustLoad()
	var del, dist []int
	for seed := uint64(1); seed <= 10; seed++ {
		d, err := Run(cfg, seed, Horizon, Delegated(cfg, 40, "steady"))
		if err != nil {
			t.Fatal(err)
		}
		if d.Over != nil {
			t.Fatalf("seed %d: delegated ended on day %d: %s", seed, d.Days, d.Over.Cause)
		}
		x, _ := Run(cfg, seed, Horizon, Distributor(cfg, 40))
		del = append(del, d.NetWorthAt(Horizon))
		dist = append(dist, x.NetWorthAt(Horizon))
	}
	sort.Ints(del)
	sort.Ints(dist)
	medDel, medDist := del[len(del)/2], dist[len(dist)/2]
	t.Logf("day %d median net worth: delegated (steady) %d, distributor %d", Horizon, medDel, medDist)
	if float64(medDel) < 0.8*float64(medDist) {
		t.Fatalf("delegated %d is more than 20%% under the distributor's %d", medDel, medDist)
	}
}

// The lieutenant keeps their city stocked (#174): the delegated player
// with the route off never sees the delegated city dry, nothing stashed
// and nothing the contracts bring in the morning (what the sell dialog
// counts, the stash plus World.SupplyDue), on two mornings running for
// sixty days from the hire, on five seeds, and the lieutenant's
// contracts are what fills it; the player's own contract for a product
// wins over the lieutenant's; a lie-low day buys nothing; unassigned,
// the contract is gone the next morning. (The stash the morning shows
// is what the night left: a stash smaller than what the corners move
// in a night is sold whole, and is not dry. A morning the lieutenant
// works no corner there is nobody's to stock and is not counted.)
func TestLieutenantKeepsTheCityStocked(t *testing.T) {
	cfg := content.MustLoad()
	home := cfg.City.Home().ID
	weed := cfg.Market.Products[0].ID
	for seed := uint64(1); seed <= 5; seed++ {
		pol := Delegated(cfg, 40, "steady")
		hired, dry, worst := 0, 0, 0
		var ltID int
		res, err := Run(cfg, seed, Horizon, func(w *game.World) {
			pol(w)
			for id := range w.Routes {
				_ = w.SetRoute(id, events.RouteOff) // the road never feeds home
			}
			lt := w.Crew.Lieutenant(home)
			if lt != nil && hired == 0 {
				hired, ltID = w.Day, lt.ID
			}
			if hired == 0 {
				return
			}
			if day := w.Day - hired; day > 1 && day <= 61 && w.WorkedIn(home) > 0 {
				tonight := w.StockIn(home)
				for _, id := range w.Products {
					tonight += w.SupplyDue(home, id)
				}
				if tonight == 0 {
					dry++
				} else {
					dry = 0
				}
				worst = max(worst, dry)
			}
			switch w.Day - hired {
			case 10:
				if err := w.SetSupply(home, weed, 1); err != nil {
					t.Fatal(err)
				}
			case 12:
				w.ClearSupply(home, weed)
			case 20:
				w.SetLieLow(true)
			case 30:
				if lt != nil {
					if err := w.Unassign(lt.ID); err != nil {
						t.Fatal(err)
					}
				}
			case 31:
				for _, c := range w.DelegatedSupply {
					if c.City == home {
						t.Fatalf("seed %d: the lieutenant's contract for %s survived being unassigned", seed, c.Product)
					}
				}
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if hired == 0 || hired > Horizon-62 {
			t.Fatalf("seed %d: a lieutenant was hired on day %d", seed, hired)
		}
		if worst > 1 {
			t.Fatalf("seed %d: home was dry on %d mornings running after %s took it on day %d", seed, worst, res.World.Crew.Member(ltID).Name, hired)
		}
		theirs, yours := 0, 0
		for _, e := range res.Events {
			sb, ok := e.(events.SupplyBought)
			if !ok || sb.City != home {
				continue
			}
			switch {
			case sb.Lieutenant != "":
				theirs++
			default:
				yours++
			}
			switch sb.Day - hired {
			case 11:
				if sb.Product == weed && sb.Lieutenant != "" {
					t.Fatalf("seed %d day %d: the lieutenant bought weed over your own contract: %+v", seed, sb.Day, sb)
				}
			case 21:
				if sb.Lieutenant != "" {
					t.Fatalf("seed %d day %d: the lieutenant bought on a lie-low day: %+v", seed, sb.Day, sb)
				}
			case 31:
				if sb.Lieutenant != "" {
					t.Fatalf("seed %d day %d: the lieutenant bought the morning after being unassigned: %+v", seed, sb.Day, sb)
				}
			}
		}
		t.Logf("seed %d: hired day %d; the lieutenant's contracts bought %d times at home, yours %d; longest run of dry mornings %d", seed, hired, theirs, yours, worst)
		if theirs < 30 {
			t.Fatalf("seed %d: the lieutenant's contracts bought only %d times at home", seed, theirs)
		}
	}
}
