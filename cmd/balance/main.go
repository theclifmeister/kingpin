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
	policy := flag.String("policy", "normal", "idle | hide | quiet | normal | aggressive | careful | managed | upgraded | crewed | vigilant | territory | war | diplomat | laundered | funded | distributor | delegated | dealer | stocked | routine | boss | pricewar")
	lt := flag.String("lt", "", "force the delegated policy's lieutenant temper: violent | greedy | careful | steady (default as generated)")
	corners := flag.Int("corners", 3, "corners the territory and war policies work, counting yours")
	force := flag.String("force", "push", "warn | push | hit: how hard the war policy strikes")
	undercut := flag.String("undercut", "normal", "quiet | normal | aggressive: the dial the pricewar policy undercuts at")
	rival := flag.String("rival", "", "force the rival's personality: expansionist | defensive | opportunist | chaotic (default by seed); none keeps the rival out of the run (harness.NoRival)")
	heatFlag := flag.String("heat", "on", "on | off: off switches heat off, nothing adds any and the police never answer (harness.NoHeat)")
	pace := flag.String("pace", "on", "on | off: off has the rival claim at the flat pace it had before #60 (harness.FlatPace)")
	chief := flag.String("chief", "", "force the police chief's personality for the whole run: corrupt | zealous | lazy (default by seed, replaced on schedule)")
	da := flag.String("da", "", "force the DA's stance for the whole run: law_and_order | moderate | reform (default by seed, elections every term)")
	trace := flag.Bool("trace", false, "print a per-day trace of the run with -seed")
	seed0 := flag.Uint64("seed", 1, "first seed; also the traced run")
	lieLow := flag.Float64("lielow", 0, "heat at which careful/managed/crewed lie low (0 = policy default)")
	cash := flag.Int("cash", 0, "start every run with this much dirty cash instead of the default")
	own := flag.String("own", "", "comma-separated upgrade ids every run owns from day 0, free (prerequisites first)")
	snitch := flag.Bool("snitch", false, "start every run with an informant on the payroll (harness.Plant)")
	cards := flag.String("cards", "", "deal the dilemma cards and answer every one with: decline (the last choice) | first (default: no cards)")
	flag.Parse()
	at := func(def float64) float64 {
		if *lieLow > 0 {
			return *lieLow
		}
		return def
	}

	cfg := content.MustLoad()
	if *rival == "none" {
		cfg = harness.NoRival(cfg)
	}
	if *heatFlag == "off" {
		cfg = harness.NoHeat(cfg)
	}
	if *pace == "off" {
		cfg = harness.FlatPace(cfg)
	}
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
		p = harness.Upgraded(cfg, at(40))
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
	case "diplomat":
		p = harness.Diplomat(cfg, at(40), *corners)
	case "pricewar":
		d := events.DialNormal
		switch *undercut {
		case "quiet":
			d = events.DialQuiet
		case "aggressive":
			d = events.DialAggressive
		}
		p = harness.Pricewar(cfg, at(40), *corners, d)
	case "laundered":
		p = harness.Laundered(cfg, at(40))
	case "funded":
		p = harness.Funded(cfg, at(40))
	case "distributor":
		p = harness.Distributor(cfg, at(40))
	case "delegated":
		p = harness.Delegated(cfg, at(40), *lt)
	case "dealer":
		p = harness.Dealer(cfg, at(40))
	case "stocked":
		p = harness.Stocked(cfg, at(40))
	case "routine":
		p = harness.Routine(cfg, at(40))
	case "boss":
		p = harness.Boss(cfg, at(40), *lt)
	default:
		p = harness.Trader(cfg, events.DialNormal)
	}

	var owned []string
	if *own != "" {
		owned = strings.Split(*own, ",")
	}
	var pick harness.Chooser
	switch *cards {
	case "decline":
		pick = harness.Decline
	case "first":
		pick = harness.First
	case "":
	default:
		fmt.Fprintf(os.Stderr, "unknown -cards %q\n", *cards)
		os.Exit(2)
	}
	var played, peaks []int
	worth := map[int][]int{}
	endings := map[string]int{}
	robberies, robbed := 0, 0
	var rivalHeld, takens []int
	rivalAt := map[int][]int{}
	won, strikes, tips, crackdowns := 0, 0, 0, 0
	undercuts, undercutUnits, abandons := 0, 0, 0
	var muscle []int
	informants, leaks, investigations, named, defections := 0, 0, 0, 0, 0
	lieutenants, cuts, walked, flipped := 0, 0, 0, 0
	tempers := map[string]int{}
	personalities := map[string]int{}
	bought := map[string]int{}
	audits, laundered, clean := 0, 0, 0
	shipments, shipped, seizures, seizedUnits := 0, 0, 0, 0
	var fear, respect, notoriety []int
	dealt := map[string]int{}
	deals, refused, betrayals, betrayedBy, tribute, offers := 0, 0, 0, 0, 0, 0
	var trust []int
	var pressure, goodwill []int
	elections, chiefs, funded := 0, 0, 0
	stances := map[string]int{}
	tempersOfChief := map[string]int{}
	for seed := *seed0; seed < *seed0+uint64(*runs); seed++ {
		// The rival's corners at the pace days, read the morning after.
		pol := func(w *game.World) {
			for _, d := range harness.PaceDays {
				if w.Day == d {
					rivalAt[d] = append(rivalAt[d], w.RivalHeld())
				}
			}
			p(w)
		}
		if *trace && seed == *seed0 {
			inner := pol
			pol = func(w *game.World) {
				inner(w)
				fmt.Printf("day %3d %s dirty %8d clean %9d heat", w.Day, w.Player.Location, w.Player.DirtyCash, w.Player.CleanCash)
				for _, cid := range w.CityOrder {
					fmt.Printf(" %.0f", w.Cities[cid].Heat)
				}
				fmt.Printf(" pressure")
				for _, cid := range w.CityOrder {
					fmt.Printf(" %.0f", w.Cities[cid].Pressure)
				}
				fmt.Printf(" file %d stock %3d/%3d +%d road orders %d crew %d corners %d/%d rival %d war %3.0f upgrades %d fronts %d %s rep %.0f/%.0f/%.0f", w.Heat.Evidence, w.Player.TotalStock(), w.Capacity(w.Player.Location), w.TotalStock()-w.Player.TotalStock(), len(w.Orders), len(w.Crew.Members), w.Worked(), w.Held(), w.RivalHeld(), w.Rival.War, len(w.Upgrades), len(w.Fronts), w.Laundering.Dial, w.Player.Reputation.Fear, w.Player.Reputation.Respect, w.Player.Reputation.Notoriety)
				for _, id := range w.Products {
					fmt.Printf("  %s", id)
					for _, cid := range w.CityOrder {
						fmt.Printf(" $%.1f", w.Cities[cid].Market[id].Price)
					}
				}
				fmt.Println()
			}
		}
		w := sim.NewWorld(cfg, seed)
		if *cash > 0 {
			w.Player.DirtyCash = *cash
		}
		if *rival != "" && *rival != "none" {
			w.Rival.Personality = *rival
		}
		harness.Own(cfg, w, owned...)
		if *snitch {
			harness.Plant(cfg, w)
		}
		runCfg := harness.Appoint(cfg, w, *chief, *da)
		var res harness.Result
		var err error
		if pick != nil {
			res, err = harness.RunWith(runCfg, w, *days, pol, pick)
		} else {
			res, err = harness.RunFrom(runCfg, w, *days, pol)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if res.World.Day == *days {
			for _, d := range harness.PaceDays {
				if d == *days {
					rivalAt[d] = append(rivalAt[d], res.World.RivalHeld())
				}
			}
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
			case events.PlayerUndercut:
				undercuts++
				undercutUnits += ev.Units
			case events.RivalAbandoned:
				abandons++
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
			case events.LieutenantFlipped:
				flipped++
			case events.DilemmaDrawn:
				dealt[ev.Card]++
			case events.DealOffered:
				offers++
			case events.HeatChanged:
				for _, r := range ev.Reasons {
					if strings.HasPrefix(r, "the DA's file") {
						leaks++
					}
				}
			}
		}
		robbed += res.World.Stats.Robbed
		cuts += res.World.Stats.Cuts
		walked += res.World.Stats.Walked
		for _, m := range res.World.Crew.Members {
			if m.Runs() {
				lieutenants++
				tempers[m.Personality]++
			}
		}
		rivalHeld = append(rivalHeld, res.World.RivalHeld())
		muscle = append(muscle, res.World.Rival.Muscle)
		takens = append(takens, res.World.Stats.CornersLost)
		won += res.World.Stats.CornersWon
		strikes += res.World.Stats.Strikes
		personalities[res.World.Rival.Personality]++
		for id := range res.World.Upgrades {
			bought[id]++
		}
		laundered += res.World.Stats.Laundered
		clean += res.World.Player.CleanCash
		shipments += res.World.Stats.Shipments
		shipped += res.World.Stats.Shipped
		seizures += res.World.Stats.Seizures
		seizedUnits += res.World.Stats.SeizedOnRoad
		rep := res.World.Player.Reputation
		fear, respect, notoriety = append(fear, int(rep.Fear)), append(respect, int(rep.Respect)), append(notoriety, int(rep.Notoriety))
		st := res.World.Stats
		deals, refused, betrayals, betrayedBy, tribute = deals+st.Deals, refused+st.DealsRefused, betrayals+st.Betrayals, betrayedBy+st.BetrayedBy, tribute+st.Tribute
		trust = append(trust, int(res.World.Rival.Trust))
		pressure = append(pressure, int(res.World.Here().Pressure))
		goodwill = append(goodwill, int(res.World.Here().Goodwill))
		elections, chiefs, funded = elections+st.Elections, chiefs+st.Chiefs, funded+st.Funded
		stances[res.World.Law.DA.Stance]++
		tempersOfChief[res.World.Law.Chief.Personality]++
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
	fmt.Printf("rival pace:   ")
	for _, d := range harness.PaceDays {
		if ws := rivalAt[d]; len(ws) > 0 {
			sort.Ints(ws)
			fmt.Printf(" day %d median %d corners (%d..%d)", d, ws[len(ws)/2], ws[0], ws[len(ws)-1])
		}
	}
	fmt.Printf(" (pace %s)\n", *pace)
	if strikes > 0 {
		fmt.Printf("war:           %.1f strikes per run, %.1f corners won per run\n", float64(strikes)/float64(*runs), float64(won)/float64(*runs))
	}
	if undercuts+abandons > 0 {
		sort.Ints(muscle)
		fmt.Printf("price war:     %.1f undercuts per run moving %d units, %d corners abandoned (totals over %d runs), rival muscle %d at the end (median)\n",
			float64(undercuts)/float64(*runs), undercutUnits / *runs, abandons, *runs, muscle[len(muscle)/2])
	}
	if informants+leaks+investigations+defections > 0 || *snitch {
		fmt.Printf("snitching:     %d turned, %d pages leaked, %d investigations named %d, %d defections (totals over %d runs)\n", informants, leaks, investigations, named, defections, *runs)
	}
	if lieutenants+walked+flipped > 0 {
		fmt.Printf("lieutenants:   %d running a city at the end, $%d cut per run, %d walked, %d flipped (totals over %d runs); %v\n", lieutenants, cuts / *runs, walked, flipped, *runs, tempers)
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
	if shipments > 0 {
		fmt.Printf("logistics:     %.1f shipments per run carrying %d units, %.1f seized per run taking %d units (%.0f%% of shipments)\n",
			float64(shipments)/float64(*runs), shipped / *runs, float64(seizures)/float64(*runs), seizedUnits / *runs, 100*float64(seizures)/float64(shipments))
	}
	sort.Ints(fear)
	sort.Ints(respect)
	sort.Ints(notoriety)
	fmt.Printf("reputation:    fear %d respect %d notoriety %d at the end (medians), fear max %d respect max %d notoriety max %d\n",
		fear[len(fear)/2], respect[len(respect)/2], notoriety[len(notoriety)/2], fear[len(fear)-1], respect[len(respect)-1], notoriety[len(notoriety)-1])
	if deals+refused+offers > 0 {
		sort.Ints(trust)
		fmt.Printf("diplomacy:     %d deals struck, %d refused, %d offered by the rival, %d broken by you, %d by them, $%d tribute per run, trust %d at the end (median)\n",
			deals, refused, offers, betrayals, betrayedBy, tribute / *runs, trust[len(trust)/2])
	}
	sort.Ints(pressure)
	sort.Ints(goodwill)
	fmt.Printf("law:           pressure %d goodwill %d at the end (medians), pressure max %d, %d elections, %d chiefs replaced, $%d given per run; DA %v chief %v\n",
		pressure[len(pressure)/2], goodwill[len(goodwill)/2], pressure[len(pressure)-1], elections, chiefs, funded / *runs, stances, tempersOfChief)
	if pick != nil {
		total := 0
		var ids []string
		for _, c := range cfg.Dilemmas.Cards {
			if dealt[c.ID] > 0 {
				total += dealt[c.ID]
				ids = append(ids, fmt.Sprintf("%s %d", c.ID, dealt[c.ID]))
			}
		}
		fmt.Printf("cards:         %.1f per run answered %s, %d of %d in the deck seen: %s\n", float64(total)/float64(*runs), *cards, len(ids), len(cfg.Dilemmas.Cards), strings.Join(ids, ", "))
	}
	fmt.Printf("endings: %v\n", endings)
}
