// Command balance runs headless games under scripted policies and prints
// the distributions the balance pass cares about.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
	"github.com/theclifmeister/kingpin/internal/sim"
)

func main() {
	runs := flag.Int("runs", 20, "number of seeded runs")
	days := flag.Int("days", harness.Horizon, "days to play each run for; a measuring horizon, the game itself has no cap")
	policy := flag.String("policy", "normal", "idle | hide | quiet | normal | aggressive | careful | managed | upgraded | crewed | vigilant | territory | war | laundered")
	corners := flag.Int("corners", 3, "corners the territory and war policies work, counting yours")
	force := flag.String("force", "push", "warn | push | hit: how hard the war policy strikes")
	rival := flag.String("rival", "", "force the rival's personality: expansionist | defensive | opportunist | chaotic (default by seed)")
	trace := flag.Bool("trace", false, "print a per-day trace of the run with -seed")
	seed0 := flag.Uint64("seed", 1, "first seed; also the traced run")
	lieLow := flag.Float64("lielow", 0, "heat at which careful/managed/crewed lie low (0 = policy default)")
	cash := flag.Int("cash", 0, "start every run with this much dirty cash instead of the default")
	own := flag.String("own", "", "comma-separated upgrade ids every run owns from day 0, free (prerequisites first)")
	snitch := flag.Bool("snitch", false, "start every run with an informant on the payroll (harness.Plant)")
	flag.Parse()
	at := func(def float64) float64 {
		if *lieLow > 0 {
			return *lieLow
		}
		return def
	}

	cfg := content.MustLoad()
	var p harness.Policy
	switch *policy {
	case "idle":
		p = harness.Idle
	case "hide":
		p = harness.Hide
	case "quiet":
		p = harness.Trader(cfg, events.DialQuiet)
	case "aggressive":
		p = harness.Trader(cfg, events.DialAggressive)
	case "careful":
		p = harness.Careful(cfg, at(35))
	case "managed":
		p = harness.Managed(cfg, at(50))
	case "upgraded":
		p = harness.Upgraded(cfg, at(50))
	case "crewed":
		p = harness.Crewed(cfg, at(40))
	case "vigilant":
		p = harness.Vigilant(cfg, at(40))
	case "territory":
		p = harness.Territory(cfg, at(40), *corners)
	case "war":
		f := events.ForcePush
		switch *force {
		case "warn":
			f = events.ForceWarn
		case "hit":
			f = events.ForceHit
		}
		p = harness.Warlike(cfg, at(40), *corners, f)
	case "laundered":
		p = harness.Laundered(cfg, at(40))
	default:
		p = harness.Trader(cfg, events.DialNormal)
	}

	var owned []string
	if *own != "" {
		owned = strings.Split(*own, ",")
	}
	var played, peaks []int
	worth := map[int][]int{}
	endings := map[string]int{}
	robberies, robbed := 0, 0
	var rivalHeld, takens []int
	won, strikes, tips, crackdowns := 0, 0, 0, 0
	informants, leaks, investigations, named, defections := 0, 0, 0, 0, 0
	personalities := map[string]int{}
	bought := map[string]int{}
	audits, laundered, clean := 0, 0, 0
	var fear, respect, notoriety []int
	for seed := *seed0; seed < *seed0+uint64(*runs); seed++ {
		pol := p
		if *trace && seed == *seed0 {
			pol = func(w *game.World) {
				p(w)
				fmt.Printf("day %3d dirty %8d clean %9d heat %5.1f file %d stock %3d/%3d orders %d crew %d corners %d/%d rival %d war %3.0f upgrades %d fronts %d %s rep %.0f/%.0f/%.0f", w.Day, w.Player.DirtyCash, w.Player.CleanCash, w.Heat.Value, w.Heat.Evidence, w.Player.TotalStock(), w.Capacity(), len(w.Orders), len(w.Crew.Members), w.Worked(), w.Held(), w.RivalHeld(), w.Rival.War, len(w.Upgrades), len(w.Fronts), w.Laundering.Dial, w.Player.Reputation.Fear, w.Player.Reputation.Respect, w.Player.Reputation.Notoriety)
				for _, id := range w.Products {
					fmt.Printf("  %s $%.1f", id, w.Market[id].Price)
				}
				fmt.Println()
			}
		}
		w := sim.NewWorld(cfg, seed)
		if *cash > 0 {
			w.Player.DirtyCash = *cash
		}
		if *rival != "" {
			w.Rival.Personality = *rival
		}
		harness.Own(cfg, w, owned...)
		if *snitch {
			harness.Plant(cfg, w)
		}
		res, err := harness.RunFrom(cfg, w, *days, pol)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		played = append(played, res.Days)
		peaks = append(peaks, res.PeakCash)
		for _, d := range harness.TierDays {
			if d <= *days {
				worth[d] = append(worth[d], res.NetWorthAt(d))
			}
		}
		if res.Over != nil {
			endings[res.Over.Cause]++
		} else {
			endings["still free"]++
		}
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.CornerRobbed:
				robberies++
			case events.RivalTippedPolice:
				tips++
			case events.WarEscalated:
				if ev.Stage == "crackdown" {
					crackdowns++
				}
			case events.FrontAudited:
				audits++
			case events.CrewTurnedInformant:
				informants++
			case events.InvestigationRun:
				investigations++
				if ev.Found {
					named++
				}
			case events.CrewDefected:
				defections++
			case events.HeatChanged:
				for _, r := range ev.Reasons {
					if strings.HasPrefix(r, "the DA's file") {
						leaks++
					}
				}
			}
		}
		robbed += res.World.Stats.Robbed
		rivalHeld = append(rivalHeld, res.World.RivalHeld())
		takens = append(takens, res.World.Stats.CornersLost)
		won += res.World.Stats.CornersWon
		strikes += res.World.Stats.Strikes
		personalities[res.World.Rival.Personality]++
		for id := range res.World.Upgrades {
			bought[id]++
		}
		laundered += res.World.Stats.Laundered
		clean += res.World.Player.CleanCash
		rep := res.World.Player.Reputation
		fear, respect, notoriety = append(fear, int(rep.Fear)), append(respect, int(rep.Respect)), append(notoriety, int(rep.Notoriety))
	}
	sort.Ints(played)
	sort.Ints(peaks)
	fmt.Printf("policy=%s runs=%d horizon=%d days\n", *policy, *runs, *days)
	fmt.Printf("days played:   min %d median %d max %d\n", played[0], played[len(played)/2], played[len(played)-1])
	fmt.Printf("peak cash:     min %d median %d max %d\n", peaks[0], peaks[len(peaks)/2], peaks[len(peaks)-1])
	fmt.Printf("net worth:    ")
	for _, d := range harness.TierDays {
		if ws := worth[d]; len(ws) > 0 {
			sort.Ints(ws)
			fmt.Printf(" day %d median %d", d, ws[len(ws)/2])
		}
	}
	fmt.Println()
	fmt.Printf("robberies:     %d per run, $%d lost per run\n", robberies / *runs, robbed / *runs)
	sort.Ints(rivalHeld)
	sort.Ints(takens)
	fmt.Printf("rival:         holds %d corners at the end (median), took %d/%d/%d of yours (min/median/max), tipped police %.1f times per run, %d crackdowns; %v\n",
		rivalHeld[len(rivalHeld)/2], takens[0], takens[len(takens)/2], takens[len(takens)-1], float64(tips)/float64(*runs), crackdowns, personalities)
	if strikes > 0 {
		fmt.Printf("war:           %.1f strikes per run, %.1f corners won per run\n", float64(strikes)/float64(*runs), float64(won)/float64(*runs))
	}
	if informants+leaks+investigations+defections > 0 || *snitch {
		fmt.Printf("snitching:     %d turned, %d pages leaked, %d investigations named %d, %d defections (totals over %d runs)\n", informants, leaks, investigations, named, defections, *runs)
	}
	if len(bought) > 0 {
		var ids []string
		for _, n := range cfg.Upgrades.Nodes {
			if bought[n.ID] > 0 {
				ids = append(ids, fmt.Sprintf("%s %d", n.ID, bought[n.ID]))
			}
		}
		fmt.Printf("upgrades:      %s (runs owning each)\n", strings.Join(ids, ", "))
	}
	fmt.Printf("laundering:    $%d washed per run, %d audits per run, $%d clean at the end\n", laundered / *runs, audits / *runs, clean / *runs)
	sort.Ints(fear)
	sort.Ints(respect)
	sort.Ints(notoriety)
	fmt.Printf("reputation:    fear %d respect %d notoriety %d at the end (medians), fear max %d respect max %d notoriety max %d\n",
		fear[len(fear)/2], respect[len(respect)/2], notoriety[len(notoriety)/2], fear[len(fear)-1], respect[len(respect)-1], notoriety[len(notoriety)-1])
	fmt.Printf("endings: %v\n", endings)
}
