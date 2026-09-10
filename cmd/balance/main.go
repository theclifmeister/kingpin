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
)

func main() {
	runs := flag.Int("runs", 20, "number of seeded runs")
	days := flag.Int("days", 200, "max days per run")
	policy := flag.String("policy", "normal", "idle | quiet | normal | aggressive | careful | managed | crewed")
	trace := flag.Bool("trace", false, "print a per-day trace of the run with -seed")
	seed0 := flag.Uint64("seed", 1, "first seed; also the traced run")
	lieLow := flag.Float64("lielow", 0, "heat at which careful/managed/crewed lie low (0 = policy default)")
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
	default:
		p = harness.Trader(cfg, events.DialNormal)
	}

	var survived, peaks []int
	worth := map[int][]int{}
	endings := map[string]int{}
	for seed := *seed0; seed < *seed0+uint64(*runs); seed++ {
		pol := p
		if *trace && seed == *seed0 {
			pol = func(w *game.World) {
				p(w)
				fmt.Printf("day %3d cash %6d heat %5.1f stock %3d/%3d orders %d crew %d", w.Day, w.Player.DirtyCash, w.Heat.Value, w.Player.TotalStock(), w.Capacity(), len(w.Orders), len(w.Crew.Members))
				for _, id := range w.Products {
					fmt.Printf("  %s $%.1f", id, w.Market[id].Price)
				}
				fmt.Println()
			}
		}
		res, err := harness.Run(cfg, seed, *days, pol)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		survived = append(survived, res.Days)
		peaks = append(peaks, res.PeakCash)
		for _, d := range harness.TierDays {
			if d <= *days {
				worth[d] = append(worth[d], res.NetWorthAt(d))
			}
		}
		if res.Over != nil {
			endings[res.Over.Cause]++
		} else {
			endings["survived"]++
		}
	}
	sort.Ints(survived)
	sort.Ints(peaks)
	fmt.Printf("policy=%s runs=%d days=%d\n", *policy, *runs, *days)
	fmt.Printf("survival days: min %d median %d max %d\n", survived[0], survived[len(survived)/2], survived[len(survived)-1])
	fmt.Printf("peak cash:     min %d median %d max %d\n", peaks[0], peaks[len(peaks)/2], peaks[len(peaks)-1])
	fmt.Printf("net worth:    ")
	for _, d := range harness.TierDays {
		if ws := worth[d]; len(ws) > 0 {
			sort.Ints(ws)
			fmt.Printf(" day %d median %d", d, ws[len(ws)/2])
		}
	}
	fmt.Println()
	fmt.Printf("endings: %v\n", endings)
}
