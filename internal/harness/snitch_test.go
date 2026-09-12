package harness

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// SnitchDays is how long an informant on the payroll takes to end a run
// nobody does anything about (#13): the case file fills at the informant
// cadence in heat.toml whatever is sold, so an always-quiet player, who
// is otherwise never indicted, is done within this many days. It is a
// ruler for the tests below, not a run length.
const SnitchDays = 40

// leakDays are the days the report's tell showed: the DA's file grew
// under the heat delta with no sting or raid that day.
func leakDays(res Result) []int {
	busts := map[int]bool{}
	for _, e := range res.Events {
		if ev, ok := e.(events.Enforcement); ok && ev.Evidence > 0 {
			busts[ev.Day] = true
		}
	}
	var days []int
	for _, e := range res.Events {
		hc, ok := e.(events.HeatChanged)
		if !ok || busts[hc.Day] {
			continue
		}
		for _, r := range hc.Reasons {
			if strings.HasPrefix(r, "the DA's file") {
				days = append(days, hc.Day)
			}
		}
	}
	return days
}

// With an informant planted on the payroll, the always-quiet trader is
// indicted within SnitchDays on every seed: the file grows on days with
// only quiet sales and no bust, which is the tell the report shows.
// Without one the same policy survives (TestAlwaysQuietStaysFreeAndEarnsLess).
func TestPlantedInformantIndictsQuietPlayer(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 10; seed++ {
		w := sim.NewWorld(cfg, seed)
		Plant(cfg, w)
		res, err := RunFrom(cfg, w, Horizon, Trader(cfg, events.DialQuiet))
		if err != nil {
			t.Fatal(err)
		}
		if res.Over == nil || res.Over.Cause != "indicted" {
			t.Fatalf("seed %d: quiet trader with an informant still free after %d days (over=%v)", seed, res.Days, res.Over)
		}
		if res.Days > SnitchDays {
			t.Fatalf("seed %d: quiet trader with an informant lasted %d days, expected <= %d", seed, res.Days, SnitchDays)
		}
		leaks := leakDays(res)
		if len(leaks) < 2 {
			t.Fatalf("seed %d: the file grew without a bust on only %d day(s): %v", seed, len(leaks), leaks)
		}
		for i := 1; i < len(leaks); i++ {
			if gap := leaks[i] - leaks[i-1]; gap != cfg.Heat.Heat.InformantDays {
				t.Fatalf("seed %d: leaks %d days apart (%v), want %d", seed, gap, leaks, cfg.Heat.Heat.InformantDays)
			}
		}
		t.Logf("seed %d: indicted on day %d after %d leaks", seed, res.Days, len(leaks))
	}
}

// The informant is beatable by reading the report: a crewed player who
// investigates once the tell has shown twice and fires whoever it names
// survives the horizon on every seed, while the same player who ignores
// the report is indicted on most of them. The vigilant player reads the
// heat gauge too: a full crew's day at the normal dial spikes heat from
// under 40 to the sting line, and the notoriety a big operation earns
// (#14) makes a line of 40 one sting too many on seed 1.
func TestVigilantSurvivesInformant(t *testing.T) {
	cfg := content.MustLoad()
	const seeds = 10
	indicted := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		w := sim.NewWorld(cfg, seed)
		Plant(cfg, w)
		vigilant, err := RunFrom(cfg, w, Horizon, Vigilant(cfg, 35))
		if err != nil {
			t.Fatal(err)
		}
		if vigilant.Over != nil {
			t.Fatalf("seed %d: vigilant player ended on day %d: %s", seed, vigilant.Days, vigilant.Over.Cause)
		}
		named := 0
		for _, e := range vigilant.Events {
			if ev, ok := e.(events.InvestigationRun); ok && ev.Found {
				named++
			}
		}
		if named == 0 && vigilant.World.Crew.Informants() > 0 {
			t.Fatalf("seed %d: vigilant player never found the informant and still has one", seed)
		}
		if vigilant.World.Heat.Evidence >= cfg.Heat.Heat.EvidenceArrest-1 {
			t.Fatalf("seed %d: vigilant player ends with a file of %d", seed, vigilant.World.Heat.Evidence)
		}
		w = sim.NewWorld(cfg, seed)
		Plant(cfg, w)
		crewed, _ := RunFrom(cfg, w, Horizon, Crewed(cfg, 40))
		if crewed.Over != nil && crewed.Over.Cause == "indicted" {
			indicted++
		}
	}
	if indicted < seeds/2 {
		t.Fatalf("crewed player who ignores the report was indicted on only %d of %d seeds; the informant does not bite", indicted, seeds)
	}
}

// investigateUntilNamed plants an informant next to one enforcer of the
// given skill and asks questions every night until somebody is named,
// returning how many nights it took. Nothing is sold and nothing is
// dangerous, so loyalty holds and only the dice and the skill decide.
func investigateUntilNamed(t *testing.T, cfg *content.Config, seed uint64, skill int) int {
	t.Helper()
	w := sim.NewWorld(cfg, seed)
	w.Player.DirtyCash = 1_000_000
	Plant(cfg, w)
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 900, Name: "Tank", Role: "enforcer", Skill: skill, Loyalty: 90, Nerve: 90, Wage: 55})
	w.Crew.NextID = 900
	tries := 0
	res, err := RunFrom(cfg, w, 30, func(w *game.World) {
		if w.Crew.Member(w.Crew.Exposed) != nil {
			return
		}
		if err := w.Investigate(cfg.Crew.Informant.InvestigateCost); err != nil {
			t.Fatalf("seed %d day %d: %v", seed, w.Day, err)
		}
		tries++
	})
	if err != nil {
		t.Fatal(err)
	}
	if m := res.World.Crew.Member(res.World.Crew.Exposed); m == nil || !m.Informant {
		t.Fatalf("seed %d skill %d: nobody named in 30 nights", seed, skill)
	}
	return tries
}

// Investigating with a skill-90 enforcer names the informant within three
// nights on every seed; with a skill-10 one it takes materially longer.
func TestInvestigationSkillTable(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Heat.Heat.DirtyCashHeat = 0 // the cash pile that pays for the questions is not the subject
	const seeds = 20
	rows := []struct {
		skill   int
		maxTry  int     // no seed may take more nights than this
		meanMin float64 // and on average it takes at least this many
	}{
		{90, 3, 0},
		{10, 6, 1.6},
	}
	means := map[int]float64{}
	for _, row := range rows {
		total, worst := 0, 0
		for seed := uint64(1); seed <= seeds; seed++ {
			n := investigateUntilNamed(t, cfg, seed, row.skill)
			total += n
			worst = max(worst, n)
		}
		mean := float64(total) / seeds
		means[row.skill] = mean
		t.Logf("skill %d: named in %.2f nights on average, %d at worst", row.skill, mean, worst)
		if worst > row.maxTry {
			t.Errorf("skill %d: took %d nights on some seed, want <= %d", row.skill, worst, row.maxTry)
		}
		if mean < row.meanMin {
			t.Errorf("skill %d: named in %.2f nights on average, want >= %.1f", row.skill, mean, row.meanMin)
		}
	}
	if means[10] < 1.5*means[90] {
		t.Errorf("skill 10 averages %.2f nights against skill 90's %.2f; skill should matter", means[10], means[90])
	}
}

// A member whose loyalty bottoms out while the rival holds ground goes
// over to it, and walks it onto the corner they ran: the day after the
// defection that corner is the rival's, and the roster is one shorter.
func TestDefectionHandsTheRivalACorner(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 5; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Player.DirtyCash = 100_000
		w.Rival.Arrived = 1
		w.Corner("docks").Owner, w.Corner("docks").Since = game.OwnerRival, 1
		// A runner at the floor on a corner of their own, and a loyal one
		// elsewhere who stays.
		w.Crew.Members = []game.CrewMember{
			{ID: 901, Name: "Vee", Role: "runner", Skill: 50, Loyalty: cfg.Crew.Crew.QuitThreshold, Greed: 90, Nerve: 50, Units: 100, Wage: 50},
			{ID: 902, Name: "Dre", Role: "runner", Skill: 50, Loyalty: 95, Greed: 5, Nerve: 90, Units: 100, Wage: 50},
		}
		w.Crew.NextID = 902
		var walked, kept *game.Corner
		for i := range w.Home().Corners {
			c := &w.Home().Corners[i]
			if c.Owner == game.OwnerNone && c.Runner == 0 {
				if walked == nil {
					walked = c
				} else if kept == nil {
					kept = c
				}
			}
		}
		if err := w.Post(walked.ID, 901); err != nil {
			t.Fatal(err)
		}
		if err := w.Post(kept.ID, 902); err != nil {
			t.Fatal(err)
		}
		res, err := RunFrom(cfg, w, 2, Idle)
		if err != nil {
			t.Fatal(err)
		}
		var defected *events.CrewDefected
		var taken *events.CornerTaken
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.CrewDefected:
				defected = &ev
			case events.CornerTaken:
				if ev.Handed != "" {
					taken = &ev
				}
			case events.CrewQuit:
				t.Fatalf("seed %d: %s quit instead of defecting", seed, ev.Name)
			}
		}
		if defected == nil || defected.Day != 1 || defected.Name != "Vee" || defected.Corner != walked.ID || defected.Rival != w.Rival.Leader {
			t.Fatalf("seed %d: no defection on day 1: %+v", seed, defected)
		}
		if taken == nil || taken.Day != 2 || taken.Corner != walked.ID || taken.Handed != "Vee" {
			t.Fatalf("seed %d: the rival did not gain %s the day after: %+v", seed, walked.Name, taken)
		}
		if got := res.World.Corner(walked.ID); got.Owner != game.OwnerRival {
			t.Fatalf("seed %d: %s is %s's, want the rival's", seed, walked.Name, got.Owner)
		}
		if res.World.Corner(kept.ID).Runner != 902 || len(res.World.Crew.Members) != 1 || res.World.Rival.Muscle < cfg.Rivals.Rivals.StartMuscle+1 {
			t.Fatalf("seed %d: after the defection roster %+v, %s runner %d, rival muscle %d", seed, res.World.Crew.Members, kept.Name, res.World.Corner(kept.ID).Runner, res.World.Rival.Muscle)
		}
		if res.World.Stats.Defections != 1 || res.World.Stats.CornersLost != 1 {
			t.Fatalf("seed %d: stats %+v", seed, res.World.Stats)
		}
	}
}

// A raid while an informant is on the payroll goes straight to the stash:
// every unit goes, whatever the safehouse would have saved. Firing the
// informant is what stops the file, and the crew do not hold that firing
// against you.
func TestInformantRaidAndFiring(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Market.Market.ShockChance, cfg.Market.Market.SlumpChance = 0, 0
	for seed := uint64(1); seed <= 3; seed++ {
		w := sim.NewWorld(cfg, seed)
		Own(cfg, w, "stash", "burners", "lookouts", "safehouse")
		w.Player.DirtyCash = 50_000
		snitch := Plant(cfg, w)
		w.Home().Heat = 90 // a raid tonight, after the day's decay
		for _, id := range w.Products {
			w.SetStock(w.Home().ID, id, 20)
		}
		res, err := RunFrom(cfg, w, 1, Idle)
		if err != nil {
			t.Fatal(err)
		}
		var raid *events.Enforcement
		for _, e := range res.Events {
			if ev, ok := e.(events.Enforcement); ok && ev.Level == "raid" {
				raid = &ev
			}
		}
		if raid == nil || !raid.Stash || res.World.Player.TotalStock() != 0 {
			t.Fatalf("seed %d: raid %+v left %d units with an informant on the payroll", seed, raid, res.World.Player.TotalStock())
		}

		// Now fire them: the file stops, the tell resets, and the rest of
		// the crew shrug.
		w = res.World
		w.Home().Heat = 0
		loyal := map[int]float64{}
		for _, m := range w.Crew.Members {
			loyal[m.ID] = m.Loyalty
		}
		if _, err := w.Fire(snitch.ID); err != nil {
			t.Fatal(err)
		}
		before := w.Heat.Evidence
		res, err = RunFrom(cfg, w, 3*cfg.Heat.Heat.InformantDays, Hide)
		if err != nil {
			t.Fatal(err)
		}
		_ = res
		if res.World.Heat.Evidence != before || res.World.Heat.Leaks != 0 || len(leakDays(res)) != 0 {
			t.Fatalf("seed %d: file %d -> %d, leaks %d after firing the informant", seed, before, res.World.Heat.Evidence, res.World.Heat.Leaks)
		}
		for _, e := range res.Events {
			if ev, ok := e.(events.CrewFired); ok && !ev.Informant {
				t.Fatalf("seed %d: firing the informant reported as %+v", seed, ev)
			}
		}
		for _, m := range res.World.Crew.Members {
			// Fair pay drifts loyalty a little either way; the firing
			// itself must not have cost fire_loyalty.
			if m.Loyalty < loyal[m.ID]-cfg.Crew.Crew.FireLoyalty {
				t.Fatalf("seed %d: %s lost %.1f loyalty over firing the informant", seed, m.Name, loyal[m.ID]-m.Loyalty)
			}
		}
	}
}

// An always-aggressive player with a lawyer on call sees every sting's
// page argued away, and it makes no difference to an informant: the
// witness's pages land in full and date the file, so a retained lawyer's
// clock never starts while one is on the payroll.
func TestInformantPagesIgnoreTheLawyer(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Heat.Heat.DirtyCashHeat = 0
	for seed := uint64(1); seed <= 3; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Player.DirtyCash = 1_000_000
		Own(cfg, w, "lawyer", "retainer")
		Plant(cfg, w)
		days := 4 * cfg.Heat.Heat.InformantDays
		res, err := RunFrom(cfg, w, days, Hide)
		if err != nil {
			t.Fatal(err)
		}
		want := (days / cfg.Heat.Heat.InformantDays) * cfg.Heat.Heat.InformantEvidence
		if res.World.Heat.Evidence != want || res.World.Heat.EvidenceDay != days {
			t.Fatalf("seed %d: file %d dated day %d after %d days with an informant and a lawyer, want %d dated %d", seed, res.World.Heat.Evidence, res.World.Heat.EvidenceDay, days, want, days)
		}
	}
}

// The rival lives at home: a runner who defects from a corner in the
// other city takes nothing there with them. The defection is reported
// without a corner, the rival gains muscle but no ground away from home,
// and the corner is merely left unworked.
func TestDefectionElsewhereHandsNoCorner(t *testing.T) {
	cfg := content.MustLoad()
	// The rival hires nobody of its own (a fee it can never meet) and
	// is dug in on the top row, a take that keeps the two heads the
	// leads add (#139: on one corner it would let them go within days),
	// so the count at the end is the defectors' and nothing else.
	cfg.Rivals.Rivals.MuscleFee = 1e6
	_, hub, _ := twoCities(t, cfg)
	for seed := uint64(1); seed <= 5; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Player.DirtyCash = 100_000
		w.Rival.Arrived = 1
		for _, id := range []string{"docks", "railyard", "oldmill"} {
			w.Corner(id).Owner, w.Corner(id).Since = game.OwnerRival, 1
		}
		w.Crew.Members = []game.CrewMember{
			{ID: 901, Name: "Vee", Role: "runner", Skill: 50, Loyalty: cfg.Crew.Crew.QuitThreshold, Greed: 90, Nerve: 50, Units: 100, Wage: 50},
		}
		w.Crew.NextID = 901
		corner := cfg.City.City(hub).Corners[0].ID
		if err := w.Post(corner, 901); err != nil {
			t.Fatal(err)
		}
		// A lead the rival should not act on even if it were handed one.
		w.Rival.Leads = append(w.Rival.Leads, game.Lead{Name: "Ghost", Corner: corner})
		muscle := w.Rival.Muscle
		res, err := RunFrom(cfg, w, 3, Idle)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.CrewDefected:
				if ev.Day != 1 || ev.Name != "Vee" || ev.Corner != "" || ev.CornerName != "" {
					t.Fatalf("seed %d: defection %+v, want one naming no corner", seed, ev)
				}
			case events.CornerTaken:
				if ev.Handed != "" {
					t.Fatalf("seed %d: the rival was handed %s in %s", seed, ev.Name, hub)
				}
			case events.RivalPushed:
				if c := res.World.Corner(ev.Corner); c != nil && c.City == hub {
					t.Fatalf("seed %d: the rival pushed on %s in %s", seed, ev.Name, hub)
				}
			}
		}
		for _, c := range res.World.City(hub).Corners {
			if c.Owner == game.OwnerRival {
				t.Fatalf("seed %d: the rival holds %s in %s", seed, c.Name, hub)
			}
		}
		if res.World.Rival.Muscle != muscle+2 || res.World.Stats.Defections != 1 || res.World.Stats.CornersLost != 0 {
			t.Fatalf("seed %d: muscle %d (was %d), stats %+v", seed, res.World.Rival.Muscle, muscle, res.World.Stats)
		}
	}
}
