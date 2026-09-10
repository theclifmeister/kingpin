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
	policy := flag.String("policy", "normal", "idle | quiet | normal | aggressive | careful | managed")
	trace := flag.Bool("trace", false, "print a per-day trace of the run with -seed")
	seed0 := flag.Uint64("seed", 1, "first seed; also the traced run")
	flag.Parse()

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
		p = harness.Careful(cfg, 35)
	case "managed":
		p = harness.Managed(cfg, 50)
	default:
		p = harness.Trader(cfg, events.DialNormal)
	}

	var survived, peaks []int
	endings := map[string]int{}
	for seed := *seed0; seed < *seed0+uint64(*runs); seed++ {
		pol := p
		if *trace && seed == *seed0 {
			pol = func(w *game.World) {
				p(w)
				fmt.Printf("day %3d cash %6d heat %5.1f stock %3d orders %d", w.Day, w.Player.DirtyCash, w.Heat.Value, w.Player.TotalStock(), len(w.Orders))
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
	fmt.Printf("endings: %v\n", endings)
}
