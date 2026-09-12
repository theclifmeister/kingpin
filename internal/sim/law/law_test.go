package law_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/law"
)

func tick(w *game.World, day int, evs ...events.Event) *game.Tick {
	t := &game.Tick{Day: day, RNG: game.RNGFor(w.Seed, day), Seed: w.Seed}
	for _, e := range evs {
		t.Emit(e)
	}
	return t
}

func kinds(t *game.Tick) map[string]int {
	out := map[string]int{}
	for _, e := range t.Events() {
		out[e.Kind()]++
	}
	return out
}

// The seed picks a named chief with a personality and a named DA with a
// ticket, in office from day 0, and a save from before has them seeded
// on load.
func TestSeedAndMigrate(t *testing.T) {
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, 5)
	l := w.Law
	if l.Chief.Name == "" || l.Chief.Personality == "" || l.DA.Name == "" || l.DA.Stance == "" || l.Chief.Since != 0 || l.DA.ElectedDay != 0 {
		t.Fatalf("seeded law: %+v", l)
	}
	if sim.NewWorld(cfg, 5).Law != l {
		t.Fatal("the same seed picked different actors")
	}
	w.Law = game.LawState{}
	w.Day = 40
	law.New(cfg).Migrate(w)
	if w.Law.Chief.Name == "" || w.Law.DA.Name == "" || w.Law.Chief.Since != 40 || w.Law.DA.ElectedDay != 40 {
		t.Fatalf("migrated law: %+v", w.Law)
	}
}

// Violence, hard product and headlines about you raise pressure where
// they happen; it fades toward the baseline; goodwill takes its cut and
// fades too; a band crossed is an event.
func TestPressureSources(t *testing.T) {
	cfg := content.MustLoad()
	s := law.New(cfg)
	src := cfg.Law.Pressure
	tun := cfg.Law.Law
	w := sim.NewWorld(cfg, 1)
	home, hub := w.CityOrder[0], w.CityOrder[1]

	// A quiet day: pressure closes on the baseline from zero.
	s.Step(w, tick(w, 1))
	if p := w.Home().Pressure; p <= 0 || p > tun.Baseline {
		t.Fatalf("quiet day: pressure %.2f, want between 0 and the baseline %.0f", p, tun.Baseline)
	}
	// Violence at home, hard product in the hub, a headline about you
	// where you stand (home); home starts a little under a band line.
	for _, c := range w.Cities {
		c.Pressure = 20
	}
	before := 20.0
	w.Journal = append(w.Journal, game.Headline{Day: 1, Source: "heat", Text: "x"}, game.Headline{Day: 1, Source: "market", Text: "y"})
	tk := tick(w, 2,
		events.CornerStruck{Day: 2, Force: events.ForcePush},
		events.RivalPushed{Day: 2},
		events.WarEscalated{Day: 2, Stage: "crackdown"},
		events.PlayerSold{Day: 2, City: hub, Product: "heroin", Wanted: 100, Sold: 50},
		events.PlayerSold{Day: 2, City: hub, Product: "weed", Wanted: 100, Sold: 500},
	)
	s.Step(w, tk)
	wantHome := before + src.Strike + src.Push + src.Crackdown + src.Headline
	wantHome -= (wantHome - tun.Baseline) * tun.Decay
	if p := w.Home().Pressure; abs(p-wantHome) > 1e-9 {
		t.Fatalf("home pressure %.4f, want %.4f", p, wantHome)
	}
	wantHub := before + 50/src.HardUnits
	wantHub -= (wantHub - tun.Baseline) * tun.Decay
	if p := w.Cities[hub].Pressure; abs(p-wantHub) > 1e-9 {
		t.Fatalf("hub pressure %.4f, want %.4f (weed is not a hard product)", p, wantHub)
	}
	if n := kinds(tk)["PressureShifted"]; n != 1 {
		t.Fatalf("home crossed a band (%.1f -> %.1f) and emitted %d PressureShifted", before, w.Home().Pressure, n)
	}

	// Goodwill: funding buys it, it takes pressure off and fades.
	w.Player.CleanCash = 100_000
	if err := w.Fund(home, 20_000); err != nil {
		t.Fatal(err)
	}
	p0 := w.Home().Pressure
	tk = tick(w, 3)
	s.Step(w, tk)
	g := s.Goodwill(20_000)
	if kinds(tk)["CityFunded"] != 1 || w.Home().Goodwill <= 0 || w.Home().Goodwill > g {
		t.Fatalf("funded: %v goodwill %.2f (bought %.2f)", kinds(tk), w.Home().Goodwill, g)
	}
	want := p0 - (p0-tun.Baseline)*tun.Decay - tun.GoodwillCut*g/100
	if p := w.Home().Pressure; abs(p-want) > 1e-9 {
		t.Fatalf("pressure with goodwill %.4f, want %.4f", p, want)
	}
	if w.Home().Goodwill >= g {
		t.Fatal("goodwill did not fade")
	}
	w.Today.Funded = nil
	if err := w.Fund(hub, 1); err != nil {
		t.Fatal(err)
	}
	w.Player.CleanCash = 0
	if err := w.Fund(hub, 1); err != game.ErrNoCleanCash {
		t.Fatalf("funding with no clean cash: %v", err)
	}
	w.Player.DirtyCash = 1_000_000
	if err := w.Fund(hub, 1); err != game.ErrNoCleanCash {
		t.Fatalf("funding with dirty cash only: %v", err)
	}
}

// The chief's term runs out on schedule and a new one takes office; a
// law-and-order DA elected on a loud city replaces them early; the
// election is on its day and a moderate's win keeps a moderate.
func TestTermsAndElections(t *testing.T) {
	cfg := content.MustLoad()
	s := law.New(cfg)
	tun := cfg.Law.Law
	w := sim.NewWorld(cfg, 2)
	old := w.Law.Chief
	tk := tick(w, tun.ChiefTerm-1)
	s.Step(w, tk)
	if kinds(tk)["ChiefReplaced"] != 0 || w.Law.Chief.Name != old.Name {
		t.Fatalf("chief replaced a day early: %v", kinds(tk))
	}
	tk = tick(w, tun.ChiefTerm)
	s.Step(w, tk)
	if kinds(tk)["ChiefReplaced"] != 1 || w.Law.Chief.Name == old.Name || w.Law.Chief.Since != tun.ChiefTerm || w.Law.Chief.Observed || w.Stats.Chiefs != 1 {
		t.Fatalf("chief on the last day of the term: %v %+v", kinds(tk), w.Law.Chief)
	}
	if s.ChiefTermEnds(w) != 2*tun.ChiefTerm {
		t.Fatalf("next term ends %d", s.ChiefTermEnds(w))
	}

	// A chief is observed after observe_days, or on the first sting.
	w = sim.NewWorld(cfg, 2)
	tk = tick(w, 1, events.Enforcement{Day: 1, Level: content.Sting})
	s.Step(w, tk)
	if !w.Law.Chief.Observed {
		t.Fatal("a sting did not show the chief's hand")
	}
	w = sim.NewWorld(cfg, 2)
	s.Step(w, tick(w, tun.ObserveDays))
	if !w.Law.Chief.Observed {
		t.Fatalf("%d days in office did not show the chief's hand", tun.ObserveDays)
	}

	// The election, with the cities loud: over 30 seeds a law-and-order
	// winner arrives, and the first one with pressure over the line
	// replaces the chief at once.
	found := false
	for seed := uint64(1); seed <= 30 && !found; seed++ {
		w = sim.NewWorld(cfg, seed)
		for _, c := range w.Cities {
			c.Pressure = 90
		}
		w.Law.DA.Stance = "reform"
		chief := w.Law.Chief.Name
		tk = tick(w, tun.TermDays)
		s.Step(w, tk)
		k := kinds(tk)
		if k["DAElected"] != 1 || w.Law.DA.ElectedDay != tun.TermDays || w.Stats.Elections != 1 {
			t.Fatalf("seed %d: election day: %v %+v", seed, k, w.Law.DA)
		}
		if w.Law.DA.Stance == "law_and_order" {
			found = true
			if k["ChiefReplaced"] != 1 || w.Law.Chief.Name == chief {
				t.Fatalf("seed %d: a law-and-order DA on a loud city kept chief %s: %v", seed, chief, k)
			}
			if s.NextElection(w) != 2*tun.TermDays {
				t.Fatalf("next election %d", s.NextElection(w))
			}
		}
	}
	if !found {
		t.Fatal("30 elections at pressure 90 and no law-and-order winner")
	}

	// A quiet city re-elects a reformer by name.
	w = sim.NewWorld(cfg, 3)
	for _, c := range w.Cities {
		c.Pressure = 0
	}
	w.Law.DA.Stance = "reform"
	name := w.Law.DA.Name
	for seed := uint64(1); seed <= 30; seed++ {
		w.Seed = seed
		w.Law.DA = game.DA{Name: name, Stance: "reform"}
		tk = tick(w, tun.TermDays)
		s.Step(w, tk)
		for _, e := range tk.Events() {
			if ev, ok := e.(events.DAElected); ok && ev.Stance == "reform" && (!ev.Incumbent || ev.Name != name || w.Law.DA.Name != name) {
				t.Fatalf("seed %d: a reformer won again and the DA changed: %+v", seed, ev)
			}
		}
	}

	// Terms of zero stop the clock.
	forever := *cfg
	forever.Law.Law.ChiefTerm, forever.Law.Law.TermDays = 0, 0
	f := law.New(&forever)
	w = sim.NewWorld(cfg, 4)
	if f.NextElection(w) != 0 || f.ChiefTermEnds(w) != 0 {
		t.Fatal("a zero term still schedules")
	}
	for d := 1; d <= 400; d++ {
		tk = tick(w, d)
		f.Step(w, tk)
		if k := kinds(tk); k["DAElected"]+k["ChiefReplaced"] > 0 {
			t.Fatalf("day %d: %v with the clock stopped", d, k)
		}
	}
}

// The share of the vote follows pressure: half at 50, everything at the
// top of the swing, nothing at the bottom.
func TestLawAndOrderShare(t *testing.T) {
	cfg := content.MustLoad()
	s := law.New(cfg)
	if v := s.LawAndOrderShare(50); abs(v-0.5) > 1e-9 {
		t.Fatalf("share at 50: %.3f", v)
	}
	if s.LawAndOrderShare(100) <= s.LawAndOrderShare(70) || s.LawAndOrderShare(70) <= s.LawAndOrderShare(30) || s.LawAndOrderShare(30) <= s.LawAndOrderShare(0) {
		t.Fatal("the share does not rise with pressure")
	}
	if s.LawAndOrderShare(100) > 1 || s.LawAndOrderShare(0) < 0 {
		t.Fatal("the share left 0..1")
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// Campaigns (#193): the money joins the city's campaign the night it is
// given and the campaign talks; the window opens open_days before the
// election and shuts after it; at the count a full campaign moves the
// law-and-order share by exactly swing_max and no more, a backed
// winner is Backed, a backed loser costs the city loser_pressure and,
// under a law-and-order winner, a zealous chief, and money on both
// tickets buys nothing but a headline.
func TestCampaigns(t *testing.T) {
	cfg := content.MustLoad()
	s := law.New(cfg)
	tun := cfg.Law.Law
	cmp := cfg.Law.Campaign
	w := sim.NewWorld(cfg, 6)
	home, hub := w.CityOrder[0], w.CityOrder[1]
	w.Player.CleanCash = 100_000_000

	// The window: shut the day before it opens, open from open_days out
	// to the day of the vote, shut the day after.
	opens := tun.TermDays - cmp.OpenDays
	s.Step(w, tick(w, opens-2))
	if w.Law.CampaignOpen || s.CampaignOpen(w, opens-1) {
		t.Fatalf("campaign open %d days out", tun.TermDays-(opens-1))
	}
	if err := w.Back(home, "reform", 1); err != game.ErrCampaignClosed {
		t.Fatalf("backed before the window: %v", err)
	}
	s.Step(w, tick(w, opens-1))
	if !w.Law.CampaignOpen || !s.CampaignOpen(w, opens) || !s.CampaignOpen(w, tun.TermDays) || s.CampaignOpen(w, tun.TermDays+1) {
		t.Fatalf("window: stamped %v, %d %v, %d %v, %d %v", w.Law.CampaignOpen, opens, s.CampaignOpen(w, opens), tun.TermDays, s.CampaignOpen(w, tun.TermDays), tun.TermDays+1, s.CampaignOpen(w, tun.TermDays+1))
	}

	// Money in: the campaign holds it, the report has it, the city talks.
	if err := w.Back(home, "reform", 2*cmp.Cash); err != nil {
		t.Fatal(err)
	}
	for _, c := range w.Cities {
		c.Pressure = 20
	}
	tk := tick(w, opens)
	s.Step(w, tk)
	camp := w.Home().Campaign
	if camp.Ticket != "reform" || camp.Cash != 2*cmp.Cash || camp.Hedged || kinds(tk)["CampaignBacked"] != 1 || w.Stats.Campaigns != 1 {
		t.Fatalf("after backing: %+v %v stats %+v", camp, kinds(tk), w.Stats)
	}
	if abs(cmp.Swing(camp.Cash)-0.02) > 1e-9 {
		t.Fatalf("$%d buys %.4f of the vote, want two points", camp.Cash, cmp.Swing(camp.Cash))
	}
	quiet := 20 - (20-tun.Baseline)*tun.Decay
	loud := 20 + cmp.Pressure - (20+cmp.Pressure-tun.Baseline)*tun.Decay
	if p := w.Home().Pressure; abs(p-loud) > 1e-9 {
		t.Fatalf("home pressure with a campaign %.4f, want %.4f", p, loud)
	}
	if p := w.Cities[hub].Pressure; abs(p-quiet) > 1e-9 {
		t.Fatalf("hub pressure with no campaign %.4f, want %.4f", p, quiet)
	}

	// The swing: a point per cash, capped at swing_max, the mean over
	// the cities, toward the ticket.
	w.Home().Campaign = game.Campaign{Ticket: "reform", Cash: 100 * cmp.Fill()}
	w.Cities[hub].Campaign = game.Campaign{}
	if got := s.Swing(w); abs(got+cmp.SwingMax/2) > 1e-9 {
		t.Fatalf("a full reform campaign in one of two cities swings %.4f, want %.4f", got, -cmp.SwingMax/2)
	}
	w.Cities[hub].Campaign = game.Campaign{Ticket: "law_and_order", Cash: cmp.Fill()}
	if got := s.Swing(w); abs(got) > 1e-9 {
		t.Fatalf("full campaigns both ways swing %.4f", got)
	}
	w.Cities[hub].Campaign.Hedged = true
	if got := s.Swing(w); abs(got+cmp.SwingMax/2) > 1e-9 {
		t.Fatalf("a hedged campaign moved the vote: %.4f", got)
	}

	// The count, over 50 seeds with the cities at 80: full reform
	// campaigns in both cities return law-and-order less often than no
	// money, and still return it (money never decides). The dice are the
	// same draw either way, so the difference is the seeds whose draw
	// fell inside the swing.
	count := func(back bool) (n int) {
		for seed := uint64(1); seed <= 50; seed++ {
			w := sim.NewWorld(cfg, seed)
			for _, c := range w.Cities {
				c.Pressure = 80
				if back {
					c.Campaign = game.Campaign{Ticket: "reform", Cash: cmp.Fill()}
				}
			}
			w.Law.DA.Stance = "moderate"
			tk := tick(w, tun.TermDays)
			s.Step(w, tk)
			for _, e := range tk.Events() {
				if ev, ok := e.(events.DAElected); ok {
					if back && abs(ev.Swing+cmp.SwingMax) > 1e-9 {
						t.Fatalf("seed %d: swing %.4f on full campaigns, want %.4f", seed, ev.Swing, -cmp.SwingMax)
					}
					if ev.Stance == "law_and_order" {
						n++
					}
				}
			}
		}
		return n
	}
	unbacked, backed := count(false), count(true)
	t.Logf("law-and-order won %d of 50 at pressure 80 unbacked, %d with full reform campaigns", unbacked, backed)
	if backed >= unbacked || backed == 0 {
		t.Fatalf("money should move the vote and never decide it: %d unbacked, %d backed", unbacked, backed)
	}
	if lost := unbacked - backed; float64(lost) > 50*cmp.SwingMax*(1-tun.Moderate)*3 {
		t.Fatalf("money moved %d of 50 elections, more than swing_max %.2f allows", lost, cmp.SwingMax)
	}

	// A backed winner: the DA is Backed, the campaign spent, the win
	// counted; a backed loser: pressure at once in that city and, under
	// a law-and-order winner, a zealous chief named for it; a hedge: the
	// headline and nothing else. Seeds are walked until each outcome
	// has been seen.
	seen := map[string]bool{}
	for seed := uint64(1); seed <= 60 && len(seen) < 3; seed++ {
		w := sim.NewWorld(cfg, seed)
		for _, c := range w.Cities {
			c.Pressure = 50
		}
		w.Law.DA.Stance = "moderate"
		w.Home().Campaign = game.Campaign{Ticket: "reform", Cash: cmp.Fill()}
		w.Cities[hub].Campaign = game.Campaign{Ticket: "law_and_order", Cash: cmp.Fill(), Hedged: true}
		chief := w.Law.Chief
		hubP := w.Cities[hub].Pressure
		tk := tick(w, tun.TermDays)
		s.Step(w, tk)
		k := kinds(tk)
		if k["CampaignHedged"] != 1 || w.Cities[hub].Campaign != (game.Campaign{}) || w.Home().Campaign != (game.Campaign{}) {
			t.Fatalf("seed %d: hedged campaign: %v %+v", seed, k, w.Cities[hub].Campaign)
		}
		if got := w.Cities[hub].Pressure; got > hubP {
			t.Fatalf("seed %d: the hedged city's pressure rose %.2f -> %.2f", seed, hubP, got)
		}
		switch w.Law.DA.Stance {
		case "reform":
			seen["won"] = true
			if !w.Law.DA.Backed || k["CampaignLost"] != 0 || w.Stats.CampaignsWon != 1 {
				t.Fatalf("seed %d: backed winner: %+v %v stats %+v", seed, w.Law.DA, k, w.Stats)
			}
		case "law_and_order":
			seen["lost-law"] = true
			if w.Law.DA.Backed || k["CampaignLost"] != 1 || k["ChiefReplaced"] != 1 || w.Law.Chief.Personality != "zealous" || w.Law.Chief.Name == chief.Name {
				t.Fatalf("seed %d: backed loser under law and order: %+v %v chief %+v", seed, w.Law.DA, k, w.Law.Chief)
			}
			for _, e := range tk.Events() {
				if ev, ok := e.(events.ChiefReplaced); ok && ev.Why != "campaign" {
					t.Fatalf("seed %d: chief replaced for %q, want campaign", seed, ev.Why)
				}
				if ev, ok := e.(events.CampaignLost); ok && (!ev.Chief || ev.City != home || ev.Pressure != cmp.LoserPressure) {
					t.Fatalf("seed %d: %+v", seed, ev)
				}
			}
		default:
			seen["lost-moderate"] = true
			if w.Law.DA.Backed || k["CampaignLost"] != 1 || k["ChiefReplaced"] != 0 || w.Law.Chief.Name != chief.Name || w.Law.Chief.Personality != chief.Personality {
				t.Fatalf("seed %d: backed loser under a moderate: %+v %v chief %+v", seed, w.Law.DA, k, w.Law.Chief)
			}
		}
		if w.Law.DA.Stance != "reform" {
			// The loser's pressure lands before the fade of the same step:
			// the city ends above where 50 alone would have left it.
			if p := w.Home().Pressure; p <= 50-(50-tun.Baseline)*tun.Decay {
				t.Fatalf("seed %d: the losing city's pressure %.2f did not rise", seed, p)
			}
		}
	}
	if len(seen) < 3 {
		t.Fatalf("60 seeds at pressure 50 did not show every outcome: %v", seen)
	}
}

// Bribes (#42): the chief's answer by personality (corrupt takes it for
// bribe_days, lazy at half, zealous files it; under the price it is
// pocketed), the DA's by ticket (a reformer refuses quietly, law and
// order files it, a moderate takes it at the odds of the amount against
// the price, a DA you backed likewise at half the price), every taken
// envelope a lead and leads_case of them a file, leads fading, and the
// cold: a law-and-order DA elected ends every live deal calls_stop_days
// on, once, and nothing is for sale while they sit.
func TestBribes(t *testing.T) {
	cfg := content.MustLoad()
	s := law.New(cfg)
	tun := cfg.Law.Bribes
	fresh := func(chief, da string) *game.World {
		w := sim.NewWorld(cfg, 8)
		w.Player.DirtyCash = 10_000_000
		w.Law.Chief.Personality, w.Law.DA.Stance = chief, da
		return w
	}
	pay := func(w *game.World, target string, amount int) *game.Tick {
		if err := w.Bribe(target, amount); err != nil {
			t.Fatal(err)
		}
		tk := tick(w, w.Day+1)
		s.Step(w, tk)
		w.Day++
		w.ClearToday(w.Day)
		return tk
	}

	// The chief.
	w := fresh("corrupt", "moderate")
	tk := pay(w, game.BribeChief, tun.ChiefPrice-1)
	if k := kinds(tk); k["BribeRefused"] != 1 || k["BribeAccepted"] != 0 || w.Law.ChiefBought != 0 || w.Law.Leads != 0 {
		t.Fatalf("under the price: %v %+v", k, w.Law)
	}
	tk = pay(w, game.BribeChief, tun.ChiefPrice)
	if k := kinds(tk); k["BribeAccepted"] != 1 || k["LeadFound"] != 1 || w.Law.ChiefBought != w.Day+tun.BribeDays || w.Law.ChiefShare != 1 || w.Law.Leads != 1 || w.Law.LeadDay != w.Day || w.Stats.Leads != 1 {
		t.Fatalf("a corrupt chief: %v %+v", k, w.Law)
	}
	if !w.Law.ChiefBoughtOn(w.Day) || w.Law.ChiefBoughtOn(w.Law.ChiefBought) {
		t.Fatalf("bought on %d until %d: %v %v", w.Day, w.Law.ChiefBought, w.Law.ChiefBoughtOn(w.Day), w.Law.ChiefBoughtOn(w.Law.ChiefBought))
	}
	until := w.Law.ChiefBought
	pay(w, game.BribeChief, tun.ChiefPrice) // renewing adds to what is left
	if w.Law.ChiefBought != until+tun.BribeDays {
		t.Fatalf("renewed until %d, want %d", w.Law.ChiefBought, until+tun.BribeDays)
	}
	w = fresh("lazy", "moderate")
	pay(w, game.BribeChief, tun.ChiefPrice)
	if w.Law.ChiefShare != tun.LazyEffect || !w.Law.ChiefBoughtOn(w.Day) {
		t.Fatalf("a lazy chief: %+v", w.Law)
	}
	w = fresh("zealous", "moderate")
	tk = pay(w, game.BribeChief, tun.ChiefPrice)
	if k := kinds(tk); k["BribeBackfired"] != 1 || k["BribeAccepted"]+k["LeadFound"] != 0 || w.Law.ChiefBought != 0 || w.Law.Backfired != w.Day || w.Stats.Backfires != 1 || w.Law.Leads != 0 {
		t.Fatalf("a zealous chief: %v %+v", k, w.Law)
	}
	for _, e := range tk.Events() {
		if ev, ok := e.(events.BribeBackfired); ok && (ev.Target != game.BribeChief || ev.Evidence != tun.BackfireEvidence || ev.Heat != tun.BackfireHeat) {
			t.Fatalf("%+v", ev)
		}
	}

	// The DA.
	w = fresh("corrupt", "reform")
	tk = pay(w, game.BribeDA, tun.DAPrice)
	if k := kinds(tk); k["BribeRefused"] != 1 || w.Law.DABought != 0 || w.Law.Backfired != 0 || w.Law.Leads != 0 {
		t.Fatalf("a reformer: %v %+v", k, w.Law)
	}
	w = fresh("corrupt", "law_and_order")
	tk = pay(w, game.BribeDA, tun.DAPrice)
	if k := kinds(tk); k["BribeBackfired"] != 1 || w.Law.DABought != 0 || w.Law.Backfired != w.Day {
		t.Fatalf("law and order: %v %+v", k, w.Law)
	}
	if err := w.Bribe(game.BribeChief, tun.ChiefPrice); err != game.ErrOfficialsCold {
		t.Fatalf("the chief under a law-and-order DA: %v", err)
	}
	if err := w.BuyCheckpoint("coast", 1, 1); err != game.ErrOfficialsCold {
		t.Fatalf("a checkpoint under a law-and-order DA: %v", err)
	}
	w = fresh("corrupt", "moderate")
	if got, want := s.DAOdds(w, tun.DAPrice), 0.5; abs(got-want) > 1e-9 {
		t.Fatalf("odds at the price %.3f, want %.3f", got, want)
	}
	if got := s.DAOdds(w, 10*tun.DAPrice); got != tun.DAOddsCap {
		t.Fatalf("odds at ten times the price %.3f, want the cap %.3f", got, tun.DAOddsCap)
	}
	w.Law.DA.Backed = true
	if got, want := s.DAPrice(w), int(float64(tun.DAPrice)*cfg.Law.Campaign.BackedDAPriceMul); got != want {
		t.Fatalf("a backed DA's price %d, want %d", got, want)
	}
	w.Law.DA.Stance = "reform"
	if s.DAOdds(w, tun.DAPrice) <= 0 {
		t.Fatal("a backed reformer should take an envelope")
	}
	w.Law.DA.Backed = false
	w.Law.DA.Stance = "moderate"
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 900, Name: "Fix", Role: game.RoleFixer, Skill: 100, Loyalty: 80})
	if got, want := s.DAPrice(w), int(math.Round(float64(tun.DAPrice)*(1-tun.FixerDiscount))); got != want {
		t.Fatalf("with a skill-100 fixer the price is %d, want %d", got, want)
	}
	if got, want := s.DAOdds(w, s.DAPrice(w)), math.Min(tun.DAOddsCap, 0.5+tun.FixerOdds); abs(got-want) > 1e-9 {
		t.Fatalf("with a fixer the odds at the price are %.3f, want %.3f", got, want)
	}
	// Over 40 seeds a moderate takes the envelope at the cap sometimes
	// and not always, and every taken one is a lead; three leads are a
	// file.
	taken, refused := 0, 0
	for seed := uint64(1); seed <= 40; seed++ {
		w := fresh("corrupt", "moderate")
		w.Seed = seed
		tk := pay(w, game.BribeDA, 10*tun.DAPrice)
		k := kinds(tk)
		switch {
		case k["BribeAccepted"] == 1:
			taken++
			if !w.Law.DABoughtOn(w.Day) || w.Law.Leads != 1 || k["LeadFound"] != 1 {
				t.Fatalf("seed %d: taken but %+v %v", seed, w.Law, k)
			}
		case k["BribeRefused"] == 1:
			refused++
			if w.Law.DABought != 0 || w.Law.Leads != 0 {
				t.Fatalf("seed %d: refused but %+v", seed, w.Law)
			}
		default:
			t.Fatalf("seed %d: %v", seed, k)
		}
	}
	t.Logf("a moderate at the cap: took %d of 40, refused %d", taken, refused)
	if taken == 0 || refused == 0 {
		t.Fatalf("the moderate's odds are not odds: %d taken, %d refused", taken, refused)
	}

	// Leads: the third opens a file, and they fade.
	w = fresh("corrupt", "moderate")
	var filed *events.LeadsFiled
	for i := 0; i < tun.LeadsCase; i++ {
		w.Law.ChiefBought = 0 // a fresh envelope each day, not a renewal that would not add a lead
		tk = pay(w, game.BribeChief, tun.ChiefPrice)
		for _, e := range tk.Events() {
			if ev, ok := e.(events.LeadsFiled); ok {
				filed = &ev
			}
		}
	}
	if filed == nil || filed.Leads != tun.LeadsCase || filed.Evidence != tun.LeadEvidence || w.Law.Leads != 0 || w.Law.Filed != w.Day || w.Stats.Leads != tun.LeadsCase {
		t.Fatalf("after %d envelopes: filed %+v law %+v stats %+v", tun.LeadsCase, filed, w.Law, w.Stats)
	}
	w.Law.Leads, w.Law.LeadDay = 2, w.Day
	for d := 0; d < tun.LeadDecayDays; d++ {
		s.Step(w, tick(w, w.Day+1))
		w.Day++
	}
	if w.Law.Leads != 1 || w.Law.LeadDay != w.Day {
		t.Fatalf("leads after %d quiet days: %d (last %d, day %d)", tun.LeadDecayDays, w.Law.Leads, w.Law.LeadDay, w.Day)
	}

	// The cold: a law-and-order DA elected with a bought chief, a bought
	// DA and a bought route; calls_stop_days on, every deal ends at once
	// with one OfficialsCold; a re-elected one brings no second; a
	// moderate elected next opens the phones again.
	for seed := uint64(1); seed <= 60; seed++ {
		w := fresh("corrupt", "moderate")
		w.Seed = seed
		for _, c := range w.Cities {
			c.Pressure = 100
		}
		pay(w, game.BribeChief, tun.ChiefPrice)
		if err := w.BuyCheckpoint("coast", tun.CheckpointPrice, tun.CheckpointDays); err != nil {
			t.Fatal(err)
		}
		w.Law.DABought = w.Day + tun.BribeDays
		w.Law.DA.ElectedDay = w.Day - cfg.Law.Law.TermDays + 1
		tk := tick(w, w.Day+1)
		s.Step(w, tk)
		w.Day++
		if w.Law.DA.Stance != "law_and_order" {
			continue
		}
		if w.Law.Cold != w.Day+tun.CallsStopDays {
			t.Fatalf("seed %d: cold on %d, want %d", seed, w.Law.Cold, w.Day+tun.CallsStopDays)
		}
		cold := 0
		for w.Day < w.Law.Cold+2 {
			live := w.Law.ChiefBoughtOn(w.Day) && w.Law.DABoughtOn(w.Day) && w.CheckpointLive("coast", w.Day)
			if before := w.Day < w.Law.Cold; live != before {
				t.Fatalf("seed %d day %d (cold %d): live %v", seed, w.Day, w.Law.Cold, live)
			}
			tk := tick(w, w.Day+1)
			s.Step(w, tk)
			w.Day++
			for _, e := range tk.Events() {
				if ev, ok := e.(events.OfficialsCold); ok {
					cold++
					if !ev.Chief || !ev.Bought || len(ev.Routes) != 1 || ev.Routes[0] != "coast" || ev.Day != w.Law.Cold {
						t.Fatalf("seed %d: %+v", seed, ev)
					}
				}
			}
		}
		if cold != 1 || w.Law.ChiefBought != 0 || w.Law.DABought != 0 {
			t.Fatalf("seed %d: OfficialsCold fired %d times; law %+v", seed, cold, w.Law)
		}
		if err := w.Bribe(game.BribeChief, tun.ChiefPrice); err != game.ErrOfficialsCold {
			t.Fatalf("seed %d: the chief while the cold stands: %v", seed, err)
		}
		// The next election: a moderate winner resets the cold.
		w.Law.DA.Stance = "moderate"
		w.Law.DA.ElectedDay = w.Day - cfg.Law.Law.TermDays
		s.Step(w, tick(w, w.Day+1))
		if w.Law.DA.Stance != "law_and_order" && w.Law.Cold != 0 {
			t.Fatalf("seed %d: the cold outlived the law-and-order DA: %+v", seed, w.Law)
		}
		return
	}
	t.Fatal("60 elections at pressure 100 and no law-and-order winner")
}
