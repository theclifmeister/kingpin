// Command balance runs headless games under scripted policies and prints
// the distributions the balance pass cares about.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
	"github.com/theclifmeister/kingpin/internal/sim"
)

func main() {
	runs := flag.Int("runs", 20, "number of seeded runs")
	days := flag.Int("days", harness.Horizon, "days to play each run for; a measuring horizon, the game itself has no cap")
	policy := flag.String("policy", "normal", "idle | hide | quiet | normal | aggressive | careful | managed | crewed | territory")
	corners := flag.Int("corners", 3, "corners the territory policy works, counting yours")
	trace := flag.Bool("trace", false, "print a per-day trace of the run with -seed")
	seed0 := flag.Uint64("seed", 1, "first seed; also the traced run")
	lieLow := flag.Float64("lielow", 0, "heat at which careful/managed/crewed lie low (0 = policy default)")
	cash := flag.Int("cash", 0, "start every run with this much dirty cash instead of the default")
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
	case "crewed":
		p = harness.Crewed(cfg, at(40))
	case "territory":
		p = harness.Territory(cfg, at(40), *corners)
	default:
		p = harness.Trader(cfg, events.DialNormal)
	}

	var played, peaks []int
	worth := map[int][]int{}
	endings := map[string]int{}
	robberies, robbed := 0, 0
	for seed := *seed0; seed < *seed0+uint64(*runs); seed++ {
		pol := p
		if *trace && seed == *seed0 {
			pol = func(w *game.World) {
				p(w)
				fmt.Printf("day %3d cash %8d heat %5.1f stock %3d/%3d orders %d crew %d corners %d/%d", w.Day, w.Player.DirtyCash, w.Heat.Value, w.Player.TotalStock(), w.Capacity(), len(w.Orders), len(w.Crew.Members), w.Worked(), w.Held())
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
			if _, ok := e.(events.CornerRobbed); ok {
				robberies++
			}
		}
		robbed += res.World.Stats.Robbed
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
	fmt.Printf("endings: %v\n", endings)
}
