package harness

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
)

// Unlocks are announced (#148): every gate the game keeps behind a line
// fires one Unlocked the morning it opens, and a run that crosses no
// line is the run it was.

// gates is which doors stand open on a morning, as the sims read them:
// the products listed at home, the connects with a line who have
// Opened, the offers not Locked and the roles whose condition holds.
type gates map[string]bool

func gatesOpen(cfg *content.Config, set *sim.Set, w *game.World) gates {
	g := gates{}
	home := w.Home().ID
	for _, p := range cfg.Market.Products {
		if w.Product(home, p.ID) != nil {
			g["product:"+p.ID] = true
		}
	}
	for _, sup := range w.Suppliers {
		if sup.Opened && (sup.UnlockCash > 0 || sup.UnlockRel > 0) {
			g["connect:"+sup.ID] = true
		}
	}
	for _, o := range set.Laundering.Offers() {
		if o.UnlockCash > 0 && !o.Locked(w) {
			g["front:"+o.ID] = true
		}
	}
	if len(w.Fronts) > 0 {
		g["role:accountant"] = true
	}
	if crew.LieutenantsWanted(w) {
		g["role:lieutenant"] = true
	}
	if set.Crew.ChemistsWanted(w) {
		g["role:chemist"] = true // #47: meth on the ladder
	}
	if crew.FixersWanted(w) {
		g["role:fixer"] = true // #42: an envelope paid
	}
	if crew.DriversWanted(w) {
		g["role:driver"] = true // #46: a route run
	}
	return g
}

// TestEveryGateIsAnnounced runs the boss over five seeds and pins that
// every morning a product lists, a connect opens, a front's Locked
// flips, or the accountant, the lieutenant or the chemist (#47) joins
// the pool, the tick's
// events carry exactly one Unlocked for it, the report's UNLOCKED
// section names it and the journal has one headline for it under the
// unlock source; and that no other morning carries one.
func TestEveryGateIsAnnounced(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 5; seed++ {
		w := sim.NewWorld(cfg, seed)
		was := gatesOpen(cfg, set, w)
		fired := map[int][]string{}  // day -> the gates that first read open that morning
		report := map[int][]string{} // day -> the report's UNLOCKED lines
		policy := Boss(cfg, 40, "")
		res, err := RunFrom(cfg, w, 150, func(w *game.World) {
			now := gatesOpen(cfg, set, w)
			for k := range now {
				if !was[k] {
					fired[w.Day] = append(fired[w.Day], k)
					was[k] = true
				}
			}
			sort.Strings(fired[w.Day])
			if w.Report != nil {
				report[w.Day] = w.Report.Unlocked
			}
			policy(w)
		})
		if err != nil {
			t.Fatal(err)
		}
		got := map[int][]string{}
		names := map[int][]string{}
		for _, e := range res.Events {
			if ev, ok := e.(events.Unlocked); ok {
				got[ev.Day] = append(got[ev.Day], ev.Gate+":"+ev.ID)
				names[ev.Day] = append(names[ev.Day], ev.Name)
				if ev.Why == "" {
					t.Errorf("seed %d day %d: %s:%s with no why", seed, ev.Day, ev.Gate, ev.ID)
				}
			}
		}
		headlines := map[int]int{}
		for _, h := range res.World.Journal {
			if h.Source == "unlock" {
				headlines[h.Day]++
			}
		}
		total := 0
		for day := 1; day <= res.Days; day++ {
			sort.Strings(got[day])
			if strings.Join(fired[day], " ") != strings.Join(got[day], " ") {
				t.Fatalf("seed %d day %d: gates opened %v, announced %v", seed, day, fired[day], got[day])
			}
			if len(report[day]) != len(got[day]) || headlines[day] != len(got[day]) {
				t.Fatalf("seed %d day %d: %d announced, %d report lines %v, %d headlines", seed, day, len(got[day]), len(report[day]), report[day], headlines[day])
			}
			for i, n := range names[day] {
				if !strings.Contains(report[day][i], n) {
					t.Errorf("seed %d day %d: the line %q does not name %s", seed, day, report[day][i], n)
				}
				if len(report[day][i]) > 70 {
					t.Errorf("seed %d day %d: the line %q is %d wide, over the modal at 80 columns", seed, day, report[day][i], len(report[day][i]))
				}
			}
			total += len(got[day])
		}
		if total < 8 {
			t.Errorf("seed %d: only %d gates announced in %d days", seed, total, res.Days)
		}
		var days []string
		for day := 1; day <= res.Days; day++ {
			for _, k := range got[day] {
				days = append(days, fmt.Sprintf("d%d %s", day, k))
			}
		}
		t.Logf("seed %d: %d gates announced over %d days (over: %v): %s", seed, total, res.Days, res.Over, strings.Join(days, ", "))
	}
}

// oldRunPrint is a run's fingerprint: every day's cash and net worth,
// every event's kind and day, and every headline's day, source and
// words, hashed. Two runs that print the same made the same draws and
// took the same turns. Nothing a float writes goes in (the heat, an
// event's numbers, a headline's figures): arm64 fuses a multiply-add
// where amd64 does not, so a float's last digit is the machine's, and
// the print has to be the same on a laptop and on CI. The one copy
// change #148 made to a headline, the article agreeing with the value
// (`an Eastside crew`), is folded: the print reads every `an` as `a`.
// IntelGained (#45) is left out the same way: it is report-only
// bookkeeping the night writes off its own events with no dice (a push
// on you files the muscle you met), so a run that never used the file
// carries it and made the same draws and took the same turns;
// QuietBroken (#465) likewise, the laundering sim's note of what ended
// a quiet streak it always counted, no dice. The
// morning's lead (#354) is left out of the journal for the same reason:
// the news sim's reading of the night, no dice, and nothing reads it.
func oldRunPrint(t *testing.T, cfg *content.Config, seed uint64, days int, policy Policy) (string, []events.Event) {
	t.Helper()
	w := sim.NewWorld(cfg, seed)
	h := sha256.New()
	res, err := RunWith(cfg, w, days, func(w *game.World) {
		policy(w)
		fmt.Fprintf(h, "d%d %d %d %d %d\n", w.Day, w.Player.DirtyCash, w.Player.CleanCash, w.Stats.PeakCash, w.NetWorth())
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fold := strings.NewReplacer(" an ", " a ", "An ", "A ", "0", "", "1", "", "2", "", "3", "", "4", "", "5", "", "6", "", "7", "", "8", "", "9", "")
	for _, e := range res.Events {
		if _, ok := e.(events.IntelGained); ok {
			continue
		}
		if _, ok := e.(events.QuietBroken); ok {
			continue
		}
		day := 0
		if f := reflect.ValueOf(e).FieldByName("Day"); f.IsValid() && f.Kind() == reflect.Int {
			day = int(f.Int())
		}
		fmt.Fprintf(h, "%d %s\n", day, e.Kind())
	}
	for _, l := range res.World.Journal {
		if l.Source == "digest" {
			continue
		}
		fmt.Fprintf(h, "%d %s %s\n", l.Day, l.Source, fold.Replace(l.Text))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16], res.Events
}

// TestNoUnlockIsTheOldRun pins that a run that crosses no line (idle,
// hide: a $500 bag never reaches heroin's $3K) is byte for byte the run
// main played before #148, with the deck dealt: the prints are main's
// (4df71be) over three seeds and 120 days, and no Unlocked fires.
// Seeds 2 and 3 moved with #501 (a card is not dealt again within
// repeat_gap days): each run dealt a card again inside three weeks, and
// the gap deals another; with repeat_gap = 0 the prints are main's.
func TestNoUnlockIsTheOldRun(t *testing.T) {
	t.Parallel()
	// The duel (#43, harness.OneFaction): this pins a mechanism on a seed, and the table moves the seed's dice.
	cfg := OneFaction(content.MustLoad())
	want := map[string]string{
		"idle 1": "29f15946d5ae8dc7", "hide 1": "e485af09c510d56d",
		"idle 2": "cc4f521c8aaa00ca", "hide 2": "245df41e7fafad0c",
		"idle 3": "c2058ba1af5086e7", "hide 3": "e26be7adf66d9b54",
	}
	for _, seed := range []uint64{1, 2, 3} {
		for _, pol := range []struct {
			name string
			p    Policy
		}{{"idle", Idle}, {"hide", Hide}} {
			key := fmt.Sprintf("%s %d", pol.name, seed)
			got, evs := oldRunPrint(t, cfg, seed, 120, pol.p)
			if got != want[key] {
				t.Errorf("%s: prints %s, main printed %s", key, got, want[key])
			}
			for _, e := range evs {
				if ev, ok := e.(events.Unlocked); ok {
					t.Errorf("%s: %+v in a run that crossed no line", key, ev)
				}
			}
		}
	}
}
