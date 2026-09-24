// Command balance runs headless games under scripted policies and prints
// the distributions the balance pass cares about.
package main

import (
	"flag"
	"fmt"
	"os"
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
	policy := flag.String("policy", "normal", "the scripted policy every run plays (harness.Policies): "+strings.Join(harness.PolicyNames(), " | "))
	lt := flag.String("lt", "", "force the delegated policy's lieutenant temper: violent | greedy | careful | steady (default as generated)")
	corners := flag.Int("corners", 3, "corners the territory and war policies work, counting yours")
	force := flag.String("force", "push", "warn | push | hit: how hard the war policy strikes")
	undercut := flag.String("undercut", "normal", "quiet | normal | aggressive: the dial the pricewar policy undercuts at")
	rival := flag.String("rival", "", "force the first faction's personality: expansionist | defensive | opportunist | chaotic (default by seed); none keeps the rivals out of the run (harness.NoRival)")
	factions := flag.Int("factions", 0, "factions in the run (#43): 1 is the duel the game was before the table, 0 the file's count by seed (rivals.toml [factions] min..max)")
	heatFlag := flag.String("heat", "on", "on | off: off switches heat off, nothing adds any and the police never answer (harness.NoHeat)")
	pace := flag.String("pace", "on", "on | off: off has the rival claim at the flat pace it had before #60 (harness.FlatPace)")
	credit := flag.String("credit", "on", "on | off: off withdraws every connect's credit (harness.NoCredit)")
	houses := flag.Int("houses", 0, "houses the stashed policy keeps a city (0 = harness.StashHouses); 1 keeps everything in one")
	fronts := flag.String("fronts", "on", "on | off: off has the stashed policy buy no fronts, so nothing pays the rent")
	chief := flag.String("chief", "", "force the police chief's personality for the whole run: corrupt | zealous | lazy (default by seed, replaced on schedule)")
	da := flag.String("da", "", "force the DA's stance for the whole run: law_and_order | moderate | reform (default by seed, elections every term)")
	trace := flag.Bool("trace", false, "print a per-day trace of the run with -seed")
	seed0 := flag.Uint64("seed", 1, "first seed; also the traced run")
	lieLow := flag.Float64("lielow", 0, "heat at which careful/managed/crewed lie low (0 = policy default)")
	cash := flag.Int("cash", 0, "start every run with this much dirty cash instead of the default")
	own := flag.String("own", "", "comma-separated upgrade ids every run owns from day 0, free (prerequisites first)")
	snitch := flag.Bool("snitch", false, "start every run with an informant on the payroll (harness.Plant)")
	cards := flag.String("cards", "", "deal the dilemma cards and answer every one with: decline (the last choice) | first (default: no cards)")
	margin := flag.Float64("margin", harness.BossMargin, "how many times the next level's price the boss holds in clean cash before it invests (harness.BossMargin); 1 invests everything")
	incidents := flag.String("incidents", "on", "on | off: off boxes the world's incident table (#44); the harness tests run with it boxed, so a pinned number reads with off")
	cut := flag.Float64("cut", 0, "cut everything the policy buys by this ratio (#47, harness.Cutter): 0.5 adds half again at nothing")
	life := flag.String("life", "on", "on | off: off boxes crew.toml's [life] table (#46, harness.NoLife): nobody ages, is arrested, wounded or killed; a run with it off is the run before the feature")
	character := flag.String("character", "", "start every run as this character of characters.toml (#50, harness.Character): dealer | cook | bookkeeper | excop | dockhand (default the dealer, the run as it is); a start is on the world on day 0 and no sim reads it")
	hardDA := flag.Bool("hardda", false, "start every run with a law-and-order DA and a zealous chief (#50, the hard DA toggle): set at NewWorld, never pinned; -chief and -da pin")
	deeds := flag.String("deeds", "on", "on | off: off boxes city.toml's [deed] table (#194, harness.NoDeeds): no block is on sale, so boss buys none; a run with it off is the run before the feature")
	investigation := flag.String("investigation", "", "on | off: switch heat.toml's [investigation] (#343, harness.Investigations): on, the police name an operation before the sting; off, the blind sting, the run before the feature (default the file's)")
	flag.Parse()
	if *runs < 1 {
		fmt.Fprintf(os.Stderr, "-runs %d: at least one run\n", *runs)
		os.Exit(2)
	}
	// Every string flag with a fixed vocabulary is checked before
	// anything runs (#274): an unknown policy fell back to normal and an
	// unknown -force, -undercut or on/off to its default, so a typo
	// printed the default's numbers under the typo's name.
	entry, ok := harness.PolicyNamed(*policy)
	if !ok {
		refuse("policy", *policy, harness.PolicyNames())
	}
	oneOf("force", *force, events.ForceNames()...)
	oneOf("undercut", *undercut, events.DialNames()...)
	forceDial, _ := events.ParseForce(*force)
	undercutDial, _ := events.ParseDial(*undercut)
	oneOf("heat", *heatFlag, "on", "off")
	oneOf("pace", *pace, "on", "off")
	oneOf("credit", *credit, "on", "off")
	oneOf("fronts", *fronts, "on", "off")
	oneOf("life", *life, "on", "off")
	oneOf("deeds", *deeds, "on", "off")
	oneOf("investigation", *investigation, "", "on", "off")
	oneOf("incidents", *incidents, "on", "off")
	oneOf("cards", *cards, "", "decline", "first")
	oneOf("rival", *rival, append([]string{"", "none"}, content.Personalities...)...)
	oneOf("lt", *lt, append([]string{""}, content.LieutenantPersonalities...)...)
	oneOf("chief", *chief, append([]string{""}, content.ChiefPersonalities...)...)
	oneOf("da", *da, append([]string{""}, content.DAStances...)...)

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
	if *factions > 0 {
		cfg = harness.Factions(cfg, *factions)
	}
	if *credit == "off" {
		cfg = harness.NoCredit(cfg)
	}
	if *life == "off" {
		cfg = harness.NoLife(cfg)
	}
	if *character != "" && cfg.Characters.Character(*character) == nil {
		fmt.Fprintf(os.Stderr, "unknown -character %q\n", *character)
		os.Exit(2)
	}
	if *deeds == "off" {
		cfg = harness.NoDeeds(cfg)
	}
	if *investigation != "" {
		cfg = harness.Investigations(cfg, *investigation == "on")
	}
	p := entry.Make(cfg, harness.PolicyOpts{
		LieLow:     *lieLow,
		Corners:    *corners,
		Force:      forceDial,
		Undercut:   undercutDial,
		Lieutenant: *lt,
		Margin:     *margin,
		Houses:     *houses,
		Fronts:     *fronts != "off",
	})
	if *cut > 0 {
		p = harness.Cutter(cfg, *cut, p)
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
	}
	t := newTally(cfg)
	for seed := *seed0; seed < *seed0+uint64(*runs); seed++ {
		// The rival's corners at the pace days, read the morning after.
		pol := func(w *game.World) {
			t.morning(w)
			p(w)
		}
		traced := *trace && seed == *seed0
		if traced {
			inner := pol
			pol = func(w *game.World) {
				inner(w)
				traceDay(w)
			}
		}
		start := harness.Character(cfg, *character)
		start.HardDA = *hardDA
		w := sim.NewWorldWith(cfg, seed, start)
		if *cash > 0 {
			w.Player.DirtyCash = *cash
		}
		if *rival != "" && *rival != "none" {
			w.Rival().Personality = *rival
		}
		t.table.counts[len(w.Rivals)]++
		harness.Own(cfg, w, owned...)
		if *snitch {
			harness.Plant(cfg, w)
		}
		runCfg := harness.Appoint(cfg, w, *chief, *da)
		res, err := harness.Play(runCfg, w, *days, pol, harness.Options{Cards: pick, Incidents: *incidents == "on"})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		t.add(res, *days, traced)
	}
	t.print(summary{
		policy: *policy, pace: *pace, credit: *credit, cards: *cards, character: *character,
		runs: *runs, days: *days,
		hardDA: *hardDA, snitch: *snitch, incidents: *incidents == "on", dealing: pick != nil,
		cut: *cut,
	})
}

// oneOf refuses a string flag whose value is not one of valid (#274):
// every string flag with a fixed vocabulary goes through it, so a typo
// exits 2 with the valid values instead of playing the default.
func oneOf(name, value string, valid ...string) {
	for _, v := range valid {
		if v == value {
			return
		}
	}
	refuse(name, value, valid)
}

// refuse prints the unknown value with the valid ones and exits 2.
func refuse(name, value string, valid []string) {
	named := make([]string, len(valid))
	for i, v := range valid {
		if v == "" {
			v = `""`
		}
		named[i] = v
	}
	fmt.Fprintf(os.Stderr, "unknown -%s %q: one of %s\n", name, value, strings.Join(named, " | "))
	os.Exit(2)
}

// traceDay is -trace's line for a day: where the player stands, the
// cash, the heat and the pressure a city, the file, the stock, the
// crew, the corners, the rival, the tree, the fronts, the deeds, the
// reputation and every product's price a city.
func traceDay(w *game.World) {
	fmt.Printf("day %3d %s dirty %8d clean %9d heat", w.Day, w.Player.Location, w.Player.DirtyCash, w.Player.CleanCash)
	for _, cid := range w.CityOrder {
		fmt.Printf(" %.0f", w.Cities[cid].Heat)
	}
	fmt.Printf(" pressure")
	for _, cid := range w.CityOrder {
		fmt.Printf(" %.0f", w.Cities[cid].Pressure)
	}
	fmt.Printf(" file %d stock %3d/%3d +%d road orders %d crew %d corners %d/%d rival %d war %3.0f upgrades %d fronts %d %s deeds %d rep %.0f/%.0f/%.0f", w.Heat.Evidence, w.Stashed(), w.Capacity(w.Player.Location), w.TotalStock()-w.Stashed(), len(w.Today.Orders), len(w.Crew.Members), w.Worked(), w.Held(), w.RivalHeld(), w.Rival().War, len(w.Upgrades), len(w.Fronts), w.Laundering.Dial, len(w.Deeds()), w.Player.Reputation.Fear, w.Player.Reputation.Respect, w.Player.Reputation.Notoriety)
	for _, id := range w.Products {
		fmt.Printf("  %s", id)
		for _, cid := range w.CityOrder {
			fmt.Printf(" $%.1f", w.Cities[cid].Market[id].Price)
		}
	}
	fmt.Println()
}
