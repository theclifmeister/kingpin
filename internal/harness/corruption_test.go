package harness

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// The bought law (#42): under a corrupt chief the corrupt player, who
// pays the chief whenever it runs hot, lasts longer than the
// distributor it is built on; under a zealous chief the envelope comes
// back and it is indicted sooner. The same sixteen seeds, the law
// fixed, both starting with CorruptTableCash so the envelope is
// affordable from day one and both lying low at CorruptTableHeat, hot
// enough to be indicted. The measure is the mean days survived (capped
// at 2*Horizon) with the survivors counted beside it: at this heat a
// median saturates at the cap on one side or the other.
func TestCorruptChiefTable(t *testing.T) {
	cfg := content.MustLoad()
	days := func(chief string, policy func(*content.Config, float64) Policy) (mean, alive int) {
		for seed := uint64(1); seed <= CorruptTableSeeds; seed++ {
			w := sim.NewWorld(cfg, seed)
			w.Player.DirtyCash = CorruptTableCash
			run := Appoint(cfg, w, chief, "moderate")
			res, err := RunFrom(run, w, 2*Horizon, policy(run, CorruptTableHeat))
			if err != nil {
				t.Fatal(err)
			}
			mean += res.Days
			if res.Over == nil {
				alive++
			}
		}
		return mean / CorruptTableSeeds, alive
	}
	paid, paidAlive := days("corrupt", Corrupt)
	straight, straightAlive := days("corrupt", Distributor)
	t.Logf("under a corrupt chief: corrupt lasts %d days (%d of %d survive), distributor %d (%d survive)", paid, paidAlive, CorruptTableSeeds, straight, straightAlive)
	if paid <= straight {
		t.Errorf("paying a corrupt chief should buy days: corrupt %d vs distributor %d", paid, straight)
	}
	paid, paidAlive = days("zealous", Corrupt)
	straight, straightAlive = days("zealous", Distributor)
	t.Logf("under a zealous chief: corrupt lasts %d days (%d survive), distributor %d (%d survive)", paid, paidAlive, straight, straightAlive)
	if paid >= straight {
		t.Errorf("paying a zealous chief should cost days: corrupt %d vs distributor %d", paid, straight)
	}
}

// CorruptTableSeeds, CorruptTableCash and CorruptTableHeat are the
// corrupt chief table's settings: the seeds, the dirty cash in hand on
// day one and the lie-low line.
const (
	CorruptTableSeeds = 16
	CorruptTableCash  = 300_000
	CorruptTableHeat  = 55
)

// A run that never bribes is the run before (#42): no lead, no page from
// a lead or a backfire, nothing bought on the law or the road, no fixer
// in the pool and no bribe event, under the policies that pay nobody.
// TestSeedDigest pins the boss's sixty days byte for byte.
func TestNoBribeIsTheOldRun(t *testing.T) {
	cfg := content.MustLoad()
	for name, policy := range map[string]Policy{"laundered": Laundered(cfg, 40), "distributor": Distributor(cfg, 40), "boss": Boss(cfg, 40, "")} {
		res, err := Run(cfg, 2, 2*Horizon, func(w *game.World) {
			l := w.Law
			if l.Leads != 0 || l.LeadDay != 0 || l.ChiefBought != 0 || l.DABought != 0 || l.Backfired != 0 || l.Filed != 0 || w.Stats.Bribes != 0 || w.Stats.Leads != 0 || w.Stats.Checkpoints != 0 {
				t.Fatalf("%s day %d: the bought law moved with nobody paying: %+v %+v", name, w.Day, l, w.Stats)
			}
			for id, rs := range w.Routes {
				if rs.Bought != 0 {
					t.Fatalf("%s day %d: route %s bought with nobody paying", name, w.Day, id)
				}
			}
			for _, c := range append(append([]game.CrewMember(nil), w.Crew.Candidates...), w.Crew.Members...) {
				if c.Role == game.RoleFixer {
					t.Fatalf("%s day %d: a fixer (%s) with nobody paying", name, w.Day, c.Name)
				}
			}
			policy(w)
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.BribeAccepted, events.BribeRefused, events.BribeBackfired, events.LeadFound, events.LeadsFiled, events.CheckpointBought:
				t.Fatalf("%s: %+v in a run that never bribed", name, ev)
			case events.OfficialsCold:
				t.Fatalf("%s: %+v in a run with nothing bought", name, ev)
			}
		}
	}
}

// The quiet-day rule's second exception (#27, #42): a bribe that
// backfires files a page the next morning whatever was sold, because
// bribing is something you did. The rich hider who hands a zealous
// chief an envelope gains exactly backfire_evidence pages and
// backfire_heat, and nothing else on their quiet days, under every DA.
func TestBackfireOnAQuietDayIsEvidence(t *testing.T) {
	cfg := content.MustLoad()
	b := cfg.Law.Bribes
	for _, da := range content.DAStances {
		w := sim.NewWorld(cfg, 1)
		w.Player.DirtyCash = 5_000_000
		run := Appoint(cfg, w, "zealous", da)
		var heatBefore float64
		res, _ := RunFrom(run, w, 40, func(w *game.World) {
			if w.Day == 10 {
				heatBefore = w.Here().Heat
				// The zealous chief files it; under a law-and-order DA
				// the chief takes no calls, and the DA files it instead.
				target, amount := game.BribeChief, b.ChiefPrice
				if w.Cold() {
					target, amount = game.BribeDA, b.DAPrice
				}
				if err := w.Bribe(target, amount); err != nil {
					t.Fatal(err)
				}
			}
		})
		if res.Over != nil {
			t.Fatalf("%s: the hider ended on day %d: %s", da, res.Days, res.Over.Cause)
		}
		if res.World.Heat.Evidence != b.BackfireEvidence || res.World.Stats.Backfires != 1 {
			t.Fatalf("%s: evidence %d after one backfire, want %d; stats %+v", da, res.World.Heat.Evidence, b.BackfireEvidence, res.World.Stats)
		}
		backfired, filed := 0, false
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.BribeBackfired:
				backfired++
			case events.HeatChanged:
				for _, r := range ev.Reasons {
					if ev.Day == 12 && ev.City == res.World.Player.Location && strings.Contains(r, "backfired") {
						filed = true
					}
				}
			case events.Enforcement:
				if ev.Evidence != 0 {
					t.Fatalf("%s day %d: %s on a quiet day added %d evidence", da, ev.Day, ev.Level, ev.Evidence)
				}
			}
		}
		if backfired != 1 || !filed {
			t.Fatalf("%s: %d backfires, filed %v (heat before %.1f)", da, backfired, filed, heatBefore)
		}
	}
}

// Bought officials, routes and leads survive a save (#42), and a run
// that bribes replays from its seed.
func TestBribesSurviveSave(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	b := cfg.Law.Bribes
	play := func(seed uint64, days int) (Result, error) {
		w := sim.NewWorld(cfg, seed)
		w.Player.DirtyCash = 2_000_000
		w.Law.Chief.Personality, w.Law.DA.Stance = "corrupt", "moderate"
		return RunFrom(cfg, w, days, func(w *game.World) {
			Corrupt(cfg, 40)(w)
			if w.Day == 5 || w.Day == 6 {
				_ = w.Bribe(game.BribeChief, b.ChiefPrice)
				_ = w.BuyCheckpoint("coast", b.CheckpointPrice, b.CheckpointDays)
			}
		})
	}
	a, err := play(4, 8)
	if err != nil {
		t.Fatal(err)
	}
	l := a.World.Law
	if l.ChiefBought <= a.World.Day || l.Leads == 0 || !a.World.CheckpointLive("coast", a.World.Day) {
		t.Fatalf("nothing to save: %+v route %+v", l, a.World.Route("coast"))
	}
	if err := game.Save(1, a.World); err != nil {
		t.Fatal(err)
	}
	got, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if got.Law != a.World.Law {
		t.Fatalf("law after load %+v, want %+v", got.Law, a.World.Law)
	}
	if got.Route("coast").Bought != a.World.Route("coast").Bought {
		t.Fatalf("route after load %+v, want %+v", got.Route("coast"), a.World.Route("coast"))
	}
	if got.Stats.Bribes != a.World.Stats.Bribes || got.Stats.Leads != a.World.Stats.Leads || got.Stats.Checkpoints != a.World.Stats.Checkpoints {
		t.Fatalf("stats after load %+v", got.Stats)
	}
	x, _ := play(4, 60)
	y, _ := play(4, 60)
	if x.World.Law != y.World.Law || len(x.Events) != len(y.Events) || x.World.Stats != y.World.Stats {
		t.Fatalf("replay differs: %+v vs %+v", x.World.Law, y.World.Law)
	}
}

// The day a law-and-order DA is elected the deals have calls_stop_days
// to live (#42): played through, none is live the day after that,
// OfficialsCold fires once, and nothing is for sale while they sit.
func TestOfficialsColdEndsTheDeals(t *testing.T) {
	cfg := content.MustLoad()
	b := cfg.Law.Bribes
	term := cfg.Law.Law.TermDays
	for seed := uint64(1); seed <= 40; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Player.DirtyCash = 2_000_000
		w.Law.Chief.Personality, w.Law.DA.Stance = "corrupt", "moderate"
		elected := 0
		cold := 0
		res, err := RunFrom(cfg, w, term+b.CallsStopDays+3, func(w *game.World) {
			for _, c := range w.Cities {
				c.Pressure = 90 // a law-and-order winner, eventually
			}
			// Kept bought, and renewed three days before the vote so
			// both are live well past it.
			if w.Day < term-2 && !w.Cold() {
				if !w.Law.ChiefBoughtOn(w.Day) || w.Day == term-3 {
					_ = w.Bribe(game.BribeChief, b.ChiefPrice)
				}
				if _, live := w.Checkpoint("coast"); !live || w.Day == term-3 {
					_ = w.BuyCheckpoint("coast", b.CheckpointPrice, b.CheckpointDays)
				}
			}
			if elected > 0 && w.Day > elected+b.CallsStopDays {
				if w.Law.ChiefBoughtOn(w.Day) || w.Law.DABoughtOn(w.Day) || w.CheckpointLive("coast", w.Day) {
					t.Fatalf("seed %d day %d: a deal is live %d days after a law-and-order DA took office: %+v %+v", seed, w.Day, w.Day-elected, w.Law, w.Route("coast"))
				}
				if err := w.Bribe(game.BribeChief, b.ChiefPrice); err != game.ErrOfficialsCold {
					t.Fatalf("seed %d day %d: the chief took a call under a law-and-order DA: %v", seed, w.Day, err)
				}
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.Over != nil {
			t.Fatalf("seed %d: the run ended on day %d: %s", seed, res.Days, res.Over.Cause)
		}
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.DAElected:
				if ev.Stance == "law_and_order" && !ev.Incumbent {
					elected = ev.Day
				}
			case events.OfficialsCold:
				cold++
				if ev.Day != elected+b.CallsStopDays || len(ev.Routes) == 0 {
					t.Fatalf("seed %d: %+v (elected day %d)", seed, ev, elected)
				}
			}
		}
		if elected == 0 {
			continue
		}
		if cold != 1 {
			t.Fatalf("seed %d: OfficialsCold fired %d times", seed, cold)
		}
		// Replay the end with the check above live: the policy saw
		// `elected` only after the run, so play it once more knowing it.
		w2 := sim.NewWorld(cfg, seed)
		w2.Player.DirtyCash = 2_000_000
		w2.Law.Chief.Personality, w2.Law.DA.Stance = "corrupt", "moderate"
		_, _ = RunFrom(cfg, w2, term+b.CallsStopDays+3, func(w *game.World) {
			for _, c := range w.Cities {
				c.Pressure = 90
			}
			// Kept bought, and renewed three days before the vote so
			// both are live well past it.
			if w.Day < term-2 && !w.Cold() {
				if !w.Law.ChiefBoughtOn(w.Day) || w.Day == term-3 {
					_ = w.Bribe(game.BribeChief, b.ChiefPrice)
				}
				if _, live := w.Checkpoint("coast"); !live || w.Day == term-3 {
					_ = w.BuyCheckpoint("coast", b.CheckpointPrice, b.CheckpointDays)
				}
			}
			if w.Day > elected+b.CallsStopDays && (w.Law.ChiefBoughtOn(w.Day) || w.CheckpointLive("coast", w.Day) || w.Route("coast").Bought != 0) {
				t.Fatalf("seed %d day %d: live or on the books past the cold: %+v %+v", seed, w.Day, w.Law, w.Route("coast"))
			}
		})
		return
	}
	t.Fatal("40 seeds at pressure 100 and no law-and-order winner")
}
