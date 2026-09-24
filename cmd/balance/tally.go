package main

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
	"github.com/theclifmeister/kingpin/internal/sim/territory"
)

// The counters the runs add up, one struct a subsystem (#274). They
// were some 120 loose locals in main, several declared on one line with
// `:=`, and a shadowed one (the frozen/raids bug #273 fixed) compiled
// silently and printed a zero; as fields of a tally nothing can shadow
// them, and every line of the summary reads the struct its subsystem
// owns.

// rivalTally is the first faction: its corners and books at the pace
// days (#60, #139), what it held and took at the end, its tips, its
// crackdowns and its personality.
type rivalTally struct {
	held, takens                    []int
	at                              map[int][]int // pace day -> corners, a run each
	cashAt, incomeAt, wagesAt       map[int][]int // the books at the pace days (#139)
	muscle, heat                    []int         // at the end
	tips, crackdowns, won, strikes  int
	personalities                   map[string]int
	undercuts, undercutUnits, quits int // the price war (#68)
}

// tableTally is the table (#43): corners per faction at the pace days
// (by seat), the pushes between factions, the absorptions, the leaders
// taken, the crew poached, the homage paid, and the days any run read
// Dominant.
type tableTally struct {
	at                                   map[int][][]int
	pushes, takes, absorbed, arrested    int
	poachedCrew, poachOffers             int
	homagePaid, homageDays, dominantDays int
	counts                               map[int]int // factions in the run -> runs

	// Following the money (#341): the scouts sent, the arrivals and the
	// scouts that went home over every run; the runs an away faction
	// arrived in; and, in those, the share of the city it moved on that
	// you and the factions hold at the end, in percent.
	moves, expanded, withdrew, expandedRuns int
	awayYou, awayThem                       []int
}

// booksTally is the moves against the rival's books (#70).
type booksTally struct {
	scouts, reads, boosts, landed, boosted, tips, raids, poached int
	evidence                                                     []int // the file at the end
}

// crewTally is the snitching, the lieutenants and crew life (#46).
type crewTally struct {
	informants, leaks, investigations, named, defections         int
	lieutenants, cuts, walked, flipped                           int
	tempers                                                      map[string]int
	bodies, fallen, arrests, bails, bailCash, wounded, pensioned int
	driven, drivenSeized                                         int
}

// launderTally is the wash, the fronts' levels (#192), the offshore
// account (#195) and the assets (#48).
type launderTally struct {
	audits, laundered, clean                        int
	earned, invested, levels, legit, frozen         int
	offshore, fees, offshoreRuns, structured        int
	retired                                         []int
	assetsBought, assetCash, assetsLost, taskForces int
	tunnelsFound, assetRuns, assetUpkeep            int
	assetsOwned                                     map[string]int
	exportLoads, exportUnits, exportsSeized         int // the lanes abroad (#391)
	exportCash, exportCost, exportRuns              int
}

// lawTally is the law: pressure and goodwill at the end, the elections
// and chiefs, the campaigns (#193), the bribes (#42), the favours
// (#228) and the tax (#231).
type lawTally struct {
	pressure, goodwill                                     []int
	elections, chiefs, funded                              int
	campaigns, campaignsWon, backed                        int
	bribes, bribed, backfires, checkpoints, checkpointCash int
	leads, favours, taxed                                  int
	stances, chiefTempers                                  map[string]int
}

// tradeTally is the road, the connects, the houses (#73), the deeds
// (#194) and the quality (#47).
type tradeTally struct {
	shipments, shipped, seizures, seizedUnits      int
	rels                                           []int
	debtDays, late, frozen, collected, credit      int
	housesHeld, housesLost, houseUnits, rent       int
	raidUnits, raids                               int
	deedsHeld, deedsBought, deedCash, deedRent     int
	deedsSeized, deedRentDay                       int
	cutUnits, cutCost, cooked, cookCost, overdoses int
	chemists                                       int
	soldUnits, soldWeighed                         float64
	repeats                                        []int
}

// intelTally is intel (#45): the cops paid and what they cost, the
// spies planted, found and shot with the reports they filed, the lies
// fed and the ones that bit, and the facts at the end.
type intelTally struct {
	copsPaid, copCash, spies, spiesFound, spiesShot, reports, lures, bitten, factsHeld int
}

// tally is every counter the summary prints.
type tally struct {
	cfg *content.Config
	ld  *laundering.Sim
	tr  *territory.Sim

	played, peaks, scores []int
	worth, reached        map[int][]int
	endings               map[string]int
	fired                 map[string]int // incident id -> times it fired (#44)
	firedRuns             int
	robberies, robbed     int
	bought, dealt         map[string]int // upgrades owned, cards drawn
	fear, respect, notor  []int
	deals, refused        int
	betrayals, betrayedBy int
	tribute, offers       int
	trust                 []int

	rival rivalTally
	table tableTally
	books booksTally
	crew  crewTally
	wash  launderTally
	law   lawTally
	trade tradeTally
	intel intelTally
}

func newTally(cfg *content.Config) *tally {
	return &tally{
		cfg: cfg, ld: laundering.New(cfg), tr: territory.New(cfg),
		worth: map[int][]int{}, reached: map[int][]int{}, endings: map[string]int{},
		fired: map[string]int{}, bought: map[string]int{}, dealt: map[string]int{},
		rival: rivalTally{at: map[int][]int{}, cashAt: map[int][]int{}, incomeAt: map[int][]int{}, wagesAt: map[int][]int{}, personalities: map[string]int{}},
		table: tableTally{at: map[int][][]int{}, counts: map[int]int{}},
		crew:  crewTally{tempers: map[string]int{}},
		wash:  launderTally{assetsOwned: map[string]int{}},
		law:   lawTally{stances: map[string]int{}, chiefTempers: map[string]int{}},
	}
}

// paceDay reads the rival's corners, its books and the table's corners
// on a pace day.
func (t *tally) paceDay(w *game.World, d int) {
	t.rival.at[d] = append(t.rival.at[d], w.RivalHeld())
	income, wages := harness.RivalBooks(t.cfg, w)
	t.rival.cashAt[d] = append(t.rival.cashAt[d], w.Rival().Cash)
	t.rival.incomeAt[d] = append(t.rival.incomeAt[d], income)
	t.rival.wagesAt[d] = append(t.rival.wagesAt[d], wages)
	for i, r := range w.Rivals {
		for len(t.table.at[d]) <= i {
			t.table.at[d] = append(t.table.at[d], nil)
		}
		t.table.at[d][i] = append(t.table.at[d][i], w.RivalHeldBy(r.Faction()))
	}
}

// morning is what the policy wrapper reads every morning before the
// policy plays: the pace days and whether anyone is dominant.
func (t *tally) morning(w *game.World) {
	for _, d := range harness.PaceDays {
		if w.Day == d {
			t.paceDay(w, d)
		}
	}
	if w.Dominant() {
		t.table.dominantDays++
	}
}

// add folds one finished run into the tally; with trace it prints the
// run's incidents and deeds the way -trace always has.
func (t *tally) add(res harness.Result, days int, trace bool) {
	w := res.World
	if w.Day == days {
		for _, d := range harness.PaceDays {
			if d == days {
				t.paceDay(w, d)
			}
		}
	}
	t.played = append(t.played, res.Days)
	t.peaks = append(t.peaks, res.PeakCash)
	for n := 2; n <= len(t.cfg.Progression.Tiers); n++ {
		t.reached[n] = append(t.reached[n], w.ReachedOn(n))
	}
	for _, d := range harness.TierDays {
		if d <= days {
			t.worth[d] = append(t.worth[d], res.NetWorthAt(d))
		}
	}
	if res.Over != nil {
		t.endings[res.Over.Cause]++
		t.scores = append(t.scores, w.Stats.Score)
	} else {
		t.endings["still free"]++
	}
	for _, e := range res.Events {
		t.event(e, trace)
	}
	st := w.Stats
	t.trade.cutUnits += st.Cut
	t.trade.cutCost += st.CutCost
	t.trade.cooked += st.Cooked
	t.trade.cookCost += st.CookCost
	t.trade.overdoses += st.Overdoses
	if w.Crew.Chemist() != nil {
		t.trade.chemists++
	}
	in := &t.intel
	in.copsPaid += st.CopsPaid
	in.copCash += st.CopCash
	in.spies += st.Spies
	in.spiesFound += st.SpiesFound
	in.spiesShot += st.SpiesShot
	in.reports += st.Reports
	in.lures += st.Lures
	in.bitten += st.Bitten
	in.factsHeld += len(game.Known(w).Facts())
	c := &t.crew
	c.bodies += st.Bodies
	c.fallen += st.Fallen
	c.arrests += st.Arrests
	c.bails += st.Bails
	c.bailCash += st.BailCash
	c.wounded += st.Wounded
	c.pensioned += st.Retired
	for _, corner := range w.Home().Corners {
		if corner.Held() {
			t.trade.repeats = append(t.trade.repeats, int(math.Round(corner.Repeats()*100)))
		}
	}
	t.robbed += st.Robbed
	c.cuts += st.Cuts
	c.walked += st.Walked
	for _, m := range w.Crew.Members {
		if m.Runs() {
			c.lieutenants++
			c.tempers[m.Personality]++
		}
	}
	r := &t.rival
	r.held = append(r.held, w.RivalHeld())
	r.muscle = append(r.muscle, w.Rival().Muscle)
	r.heat = append(r.heat, int(w.Rival().Heat))
	t.books.evidence = append(t.books.evidence, w.Heat.Evidence)
	r.takens = append(r.takens, st.CornersLost)
	r.won += st.CornersWon
	r.strikes += st.Strikes
	r.personalities[w.Rival().Personality]++
	t.table.homagePaid += st.Homage
	t.table.poachedCrew += st.CrewPoached
	t.table.absorbed += st.Absorbed
	t.table.arrested += st.Fragmented
	t.table.moves += st.Moves
	t.table.expanded += st.Expanded
	t.table.withdrew += st.Withdrew
	if st.Expanded > 0 {
		t.table.expandedRuns++
		for _, cid := range w.CityOrder[1:] {
			city := w.Cities[cid]
			if len(city.Corners) == 0 || !slices.ContainsFunc(w.Rivals, func(r *game.RivalState) bool { return r != nil && r.Home == cid && r.Arrived > 0 }) {
				continue
			}
			you, them := 0, 0
			for _, c := range city.Corners {
				switch c.Owner {
				case game.OwnerPlayer:
					you++
				case game.OwnerRival:
					them++
				}
			}
			t.table.awayYou = append(t.table.awayYou, 100*you/len(city.Corners))
			t.table.awayThem = append(t.table.awayThem, 100*them/len(city.Corners))
		}
	}
	for id := range w.Upgrades {
		t.bought[id]++
	}
	l := &t.wash
	l.laundered += st.Laundered
	l.clean += w.Player.CleanCash
	l.offshore += w.Offshore
	if st.ExportLoads > 0 {
		l.exportRuns++
		l.exportLoads += st.ExportLoads
		l.exportUnits += st.ExportUnits
		l.exportsSeized += st.ExportsSeized
		l.exportCash += st.ExportCash
		l.exportCost += st.ExportCost
	}
	if st.Assets > 0 || st.TaskForces > 0 {
		l.assetRuns++
		l.assetsBought += st.Assets
		l.assetCash += st.AssetCash
		l.assetsLost += st.AssetsLost
		l.taskForces += st.TaskForces
		l.assetUpkeep += st.AssetUpkeep
		for _, a := range w.Assets {
			l.assetsOwned[a.ID]++
		}
		for _, a := range w.AssetsLost {
			if a.Why == "found" {
				l.tunnelsFound++
			}
		}
	}
	l.fees += st.Fees
	if w.Offshore > 0 {
		l.offshoreRuns++
	}
	if res.Over != nil && res.Over.Cause == "retired" {
		l.retired = append(l.retired, w.Offshore)
	}
	l.earned += st.Earned
	l.invested += st.Invested
	for _, f := range w.Fronts {
		l.levels += f.Level
	}
	l.legit += t.ld.LegitIncome(w)
	tr := &t.trade
	tr.shipments += st.Shipments
	tr.shipped += st.Shipped
	tr.seizures += st.Seizures
	tr.seizedUnits += st.SeizedOnRoad
	rep := w.Player.Reputation
	t.fear, t.respect, t.notor = append(t.fear, int(rep.Fear)), append(t.respect, int(rep.Respect)), append(t.notor, int(rep.Notoriety))
	t.deals += st.Deals
	t.refused += st.DealsRefused
	t.betrayals += st.Betrayals
	t.betrayedBy += st.BetrayedBy
	t.tribute += st.Tribute
	t.trust = append(t.trust, int(w.Rival().Trust))
	lw := &t.law
	lw.pressure = append(lw.pressure, int(w.Here().Pressure))
	lw.goodwill = append(lw.goodwill, int(w.Here().Goodwill))
	lw.elections += st.Elections
	lw.chiefs += st.Chiefs
	lw.funded += st.Funded
	lw.campaigns += st.Campaigns
	lw.campaignsWon += st.CampaignsWon
	lw.backed += st.Backed
	lw.bribes += st.Bribes
	lw.bribed += st.Bribed
	lw.backfires += st.Backfires
	lw.checkpoints += st.Checkpoints
	lw.checkpointCash += st.CheckpointCash
	lw.leads += st.Leads
	lw.favours += st.Favours
	lw.taxed += st.Taxed
	lw.stances[w.Law.DA.Stance]++
	lw.chiefTempers[w.Law.Chief.Personality]++
	// The street connect where the run ended: the relationship the
	// policy built.
	if sup := w.StreetSupplier(w.Player.Location); sup != nil {
		tr.rels = append(tr.rels, int(sup.Rel))
	}
	tr.debtDays += st.DebtDays
	tr.late += st.LatePayments
	tr.credit += st.Credit
	tr.housesHeld += len(w.Houses)
	tr.housesLost += st.HousesLost
	// The property (#194): the deeds held at the end, what they cost
	// and paid back, the DA's seizures, and the rent a day at the end.
	tr.deedsHeld += len(w.Deeds())
	tr.deedsBought += st.Deeds
	tr.deedCash += st.DeedCash
	tr.deedRent += st.DeedRent
	tr.deedsSeized += st.DeedsSeized
	for _, corner := range w.Deeds() {
		tr.deedRentDay += t.tr.DeedRent(w, corner, corner.Deed.Price)
	}
	tr.houseUnits += st.HouseUnits
	tr.rent += st.Rent
}

// event folds one of a run's events into the tally.
func (t *tally) event(e events.Event, trace bool) {
	switch ev := e.(type) {
	case events.Incident:
		t.fired[ev.ID]++
		t.firedRuns++
		if trace {
			fmt.Printf("day %3d incident %s in %s", ev.Day, ev.ID, ev.City)
			if ev.Route != "" {
				fmt.Printf(" route %s", ev.Route)
			}
			if ev.Product != "" {
				fmt.Printf(" product %s", ev.Product)
			}
			if ev.Days > 0 {
				fmt.Printf(" %dd", ev.Days)
			}
			fmt.Println()
		}
	case events.DeedBought:
		if trace {
			fmt.Printf("day %3d deed %s in %s $%d, rent $%d/day\n", ev.Day, ev.Corner, ev.City, ev.Price, ev.Rent)
		}
	case events.DeedSeized:
		if trace {
			fmt.Printf("day %3d forfeiture %s in %s $%d: $%d in deeds against $%d washed\n", ev.Day, ev.Corner, ev.City, ev.Price, ev.Spent, ev.Washed)
		}
	case events.CornerRobbed:
		t.robberies++
	case events.FactionPushed:
		t.table.pushes++
		if ev.Taken {
			t.table.takes++
		}
	case events.CrewPoached:
		t.table.poachOffers++
	case events.TributePaid:
		if ev.ToYou {
			t.table.homageDays++
		}
	case events.RivalTippedPolice:
		t.rival.tips++
	case events.RivalScouted:
		t.books.scouts++
		if ev.Read {
			t.books.reads++
		}
	case events.RivalBoosted:
		t.books.boosts++
		if ev.Taken {
			t.books.landed++
			t.books.boosted += ev.Cash
		}
	case events.PoliceTipped:
		t.books.tips++
	case events.RivalRaided:
		t.books.raids++
	case events.RivalMusclePoached:
		t.books.poached += ev.Got
	case events.PlayerUndercut:
		t.rival.undercuts++
		t.rival.undercutUnits += ev.Units
	case events.RivalAbandoned:
		t.rival.quits++
	case events.WarEscalated:
		if ev.Stage == events.StageCrackdown {
			t.rival.crackdowns++
		}
	case events.FrontAudited:
		t.wash.audits++
	case events.FrontFrozen:
		t.wash.frozen++
	case events.Reserved:
		t.wash.structured += ev.Lots
	case events.CrewTurnedInformant:
		t.crew.informants++
	case events.InvestigationRun:
		t.crew.investigations++
		if ev.Found {
			t.crew.named++
		}
	case events.CrewDefected:
		t.crew.defections++
	case events.LieutenantFlipped:
		t.crew.flipped++
	case events.DilemmaDrawn:
		t.dealt[ev.Card]++
	case events.DealOffered:
		t.offers++
	case events.SupplierFrozen:
		t.trade.frozen++
	case events.SupplierCollected:
		t.trade.collected++
	case events.HeatChanged:
		for _, r := range ev.Reasons {
			if strings.HasPrefix(r, "the DA's file") {
				t.crew.leaks++
			}
		}
	case events.PlayerSold:
		t.trade.soldUnits += float64(ev.Sold)
		t.trade.soldWeighed += float64(ev.Sold) * ev.Quality
	case events.ShipmentSent:
		if ev.Driver != 0 {
			t.crew.driven++
		}
	case events.ShipmentSeized:
		if ev.Driver != 0 {
			t.crew.drivenSeized++
		}
	case events.Enforcement:
		if ev.Level == content.Raid || ev.Level == content.Sting {
			t.trade.raids++
			for _, n := range ev.StockLost {
				t.trade.raidUnits += n
			}
		}
	}
}

// summary is what the tool prints after the runs: the flags it reads
// are the ones a line names or gates on.
type summary struct {
	policy, pace, credit, cards, character string
	runs, days                             int
	hardDA, snitch, incidents, dealing     bool
	cut                                    float64
}

// print writes the summary, line for line what main printed before the
// counters had structs (#274).
func (t *tally) print(s summary) {
	cfg, runs := t.cfg, s.runs
	sort.Ints(t.played)
	sort.Ints(t.peaks)
	played, peaks := t.played, t.peaks
	fmt.Printf("policy=%s runs=%d horizon=%d days\n", s.policy, runs, s.days)
	if s.character != "" || s.hardDA {
		who := s.character
		if who == "" {
			who = cfg.Characters.Default().ID
		}
		fmt.Printf("character:     %s (%s), hard DA %v\n", who, cfg.Characters.Character(who).Name, s.hardDA)
	}
	fmt.Printf("days played:   min %d median %d max %d\n", played[0], played[len(played)/2], played[len(played)-1])
	fmt.Printf("peak cash:     min %d median %d max %d\n", peaks[0], peaks[len(peaks)/2], peaks[len(peaks)-1])
	fmt.Printf("net worth:    ")
	for _, d := range harness.TierDays {
		if ws := t.worth[d]; len(ws) > 0 {
			sort.Ints(ws)
			fmt.Printf(" day %d median %d", d, ws[len(ws)/2])
		}
	}
	fmt.Println()
	// The median day each tier is entered: a run that never entered it
	// sorts last, so a tier half the runs never reach reads as never.
	fmt.Printf("tiers:        ")
	for n := 2; n <= len(cfg.Progression.Tiers); n++ {
		days := t.reached[n]
		sort.Slice(days, func(i, j int) bool {
			if days[i] < 0 || days[j] < 0 {
				return days[j] < 0 && days[i] >= 0
			}
			return days[i] < days[j]
		})
		med := days[len(days)/2]
		if med < 0 {
			fmt.Printf(" %d %s never", n, cfg.Progression.Tiers[n-1].Name)
		} else {
			fmt.Printf(" %d %s d%d", n, cfg.Progression.Tiers[n-1].Name, med)
		}
		if n < len(cfg.Progression.Tiers) {
			fmt.Printf(" ·")
		}
	}
	fmt.Println(" (median day entered)")
	fmt.Printf("robberies:     %d per run, $%d lost per run\n", t.robberies/runs, t.robbed/runs)
	r := &t.rival
	sort.Ints(r.held)
	sort.Ints(r.takens)
	fmt.Printf("rival:         holds %d corners at the end (median), took %d/%d/%d of yours (min/median/max), tipped police %.1f times per run, %d crackdowns; %v\n",
		r.held[len(r.held)/2], r.takens[0], r.takens[len(r.takens)/2], r.takens[len(r.takens)-1], float64(r.tips)/float64(runs), r.crackdowns, r.personalities)
	fmt.Printf("rival pace:   ")
	for _, d := range harness.PaceDays {
		if ws := r.at[d]; len(ws) > 0 {
			sort.Ints(ws)
			fmt.Printf(" day %d median %d corners (%d..%d)", d, ws[len(ws)/2], ws[0], ws[len(ws)-1])
		}
	}
	fmt.Printf(" (pace %s)\n", s.pace)
	tb := &t.table
	if len(tb.counts) > 1 || tb.counts[1] == 0 {
		fmt.Printf("factions:      %v per run (count: runs);", tb.counts)
		for _, d := range harness.PaceDays {
			if fs := tb.at[d]; len(fs) > 0 {
				fmt.Printf(" day %d corners", d)
				for i, ws := range fs {
					if len(ws) == 0 {
						continue
					}
					sort.Ints(ws)
					fmt.Printf(" f%d %d", i+1, ws[len(ws)/2])
				}
				fmt.Printf(";")
			}
		}
		fmt.Printf(" %d pushes between factions (%d corners changed hands), %d absorbed, %d leaders taken, %d of your crew poached (%d offers), $%d homage over %d days (totals over %d runs); dominant on %d days\n",
			tb.pushes, tb.takes, tb.absorbed, tb.arrested, tb.poachedCrew, tb.poachOffers, tb.homagePaid, tb.homageDays, runs, tb.dominantDays)
	}
	if tb.moves > 0 {
		fmt.Printf("expansion:     an away faction arrived in %d of %d runs (%d scouts sent, %d arrived, %d went home)", tb.expandedRuns, runs, tb.moves, tb.expanded, tb.withdrew)
		if n := len(tb.awayYou); n > 0 {
			sort.Ints(tb.awayYou)
			sort.Ints(tb.awayThem)
			fmt.Printf("; the city it moved on at the end: you %d%% of its corners, the factions %d%% (medians)", tb.awayYou[n/2], tb.awayThem[n/2])
		}
		fmt.Println()
	}
	fmt.Printf("rival books:  ")
	for _, d := range harness.PaceDays {
		if cs := r.cashAt[d]; len(cs) > 0 {
			sort.Ints(cs)
			sort.Ints(r.incomeAt[d])
			sort.Ints(r.wagesAt[d])
			fmt.Printf(" day %d cash %d income %d/day wages %d/day", d, cs[len(cs)/2], r.incomeAt[d][len(cs)/2], r.wagesAt[d][len(cs)/2])
		}
	}
	fmt.Println(" (medians)")
	if r.strikes > 0 {
		fmt.Printf("war:           %.1f strikes per run, %.1f corners won per run\n", float64(r.strikes)/float64(runs), float64(r.won)/float64(runs))
	}
	if r.undercuts+r.quits > 0 {
		sort.Ints(r.muscle)
		fmt.Printf("price war:     %.1f undercuts per run moving %d units, %d corners abandoned (totals over %d runs), rival muscle %d at the end (median)\n",
			float64(r.undercuts)/float64(runs), r.undercutUnits/runs, r.quits, runs, r.muscle[len(r.muscle)/2])
	}
	b := &t.books
	if b.scouts+b.boosts+b.tips+b.poached > 0 {
		sort.Ints(r.muscle)
		sort.Ints(r.heat)
		sort.Ints(b.evidence)
		fmt.Printf("books:         %.1f scouts per run (%d read), %.1f boosts (%d landed, $%d taken) per run, %.1f tips per run bringing %d raids, %d heads bought off (totals over %d runs); rival muscle %d, rival heat %d, file %d at the end (medians)\n",
			float64(b.scouts)/float64(runs), b.reads, float64(b.boosts)/float64(runs), b.landed, b.boosted/runs, float64(b.tips)/float64(runs), b.raids, b.poached, runs, r.muscle[len(r.muscle)/2], r.heat[len(r.heat)/2], b.evidence[len(b.evidence)/2])
	}
	tr := &t.trade
	if tr.cutUnits+tr.cooked+tr.overdoses > 0 || s.cut > 0 || s.policy == "cook" {
		meanQ := 0.0
		if tr.soldUnits > 0 {
			meanQ = tr.soldWeighed / tr.soldUnits
		}
		rep := 100
		if len(tr.repeats) > 0 {
			sort.Ints(tr.repeats)
			rep = tr.repeats[len(tr.repeats)/2]
		}
		fmt.Printf("quality:       %d units cut in for $%d, %d cooked for $%d (per run), %d overdoses over %d runs, %d runs end with a chemist; sold at quality %.0f (mean), held corners keep %d%% of their customers at the end (median)\n",
			tr.cutUnits/runs, tr.cutCost/runs, tr.cooked/runs, tr.cookCost/runs, tr.overdoses, runs, tr.chemists, meanQ, rep)
	}
	c := &t.crew
	if c.bodies+c.arrests+c.wounded+c.pensioned+c.driven > 0 {
		fmt.Printf("crew life:     %.1f bodies per run (%.1f yours), %.1f arrests, %.1f bails for $%d, %.1f wounded, %.1f retired; %d shipments driven, %d of them seized (totals over %d runs)\n",
			float64(c.bodies)/float64(runs), float64(c.fallen)/float64(runs), float64(c.arrests)/float64(runs), float64(c.bails)/float64(runs), c.bailCash/runs, float64(c.wounded)/float64(runs), float64(c.pensioned)/float64(runs), c.driven, c.drivenSeized, runs)
	}
	in := &t.intel
	if in.copsPaid+in.spies+in.lures > 0 || s.policy == "informed" {
		fmt.Printf("intel:         %.1f cops paid for $%d per run, %.1f spies planted (%d found, %d of them shot; %d reports), %.1f lies fed (%d bit) per run; %.1f facts held at the end (over %d runs)\n",
			float64(in.copsPaid)/float64(runs), in.copCash/runs, float64(in.spies)/float64(runs), in.spiesFound, in.spiesShot, in.reports, float64(in.lures)/float64(runs), in.bitten, float64(in.factsHeld)/float64(runs), runs)
	}
	if c.informants+c.leaks+c.investigations+c.defections > 0 || s.snitch {
		fmt.Printf("snitching:     %d turned, %d pages leaked, %d investigations named %d, %d defections (totals over %d runs)\n", c.informants, c.leaks, c.investigations, c.named, c.defections, runs)
	}
	if c.lieutenants+c.walked+c.flipped > 0 {
		fmt.Printf("lieutenants:   %d running a city at the end, $%d cut per run, %d walked, %d flipped (totals over %d runs); %v\n", c.lieutenants, c.cuts/runs, c.walked, c.flipped, runs, c.tempers)
	}
	if len(t.bought) > 0 {
		var ids []string
		for _, n := range cfg.Upgrades.Nodes {
			if t.bought[n.ID] > 0 {
				ids = append(ids, fmt.Sprintf("%s %d", n.ID, t.bought[n.ID]))
			}
		}
		fmt.Printf("upgrades:      %s (runs owning each)\n", strings.Join(ids, ", "))
	}
	l := &t.wash
	fmt.Printf("laundering:    $%d washed per run, %d audits per run, $%d clean at the end\n", l.laundered/runs, l.audits/runs, l.clean/runs)
	if l.invested > 0 {
		fmt.Printf("fronts:        %d levels owned at the end, $%d invested, $%d earned per run, %d shut for upkeep per run; legit income $%d/day at the end (means)\n", l.levels/runs, l.invested/runs, l.earned/runs, l.frozen/runs, l.legit/runs)
	}
	if l.offshoreRuns > 0 {
		sort.Ints(l.retired)
		score := 0
		if len(l.retired) > 0 {
			score = l.retired[len(l.retired)/2]
		}
		fmt.Printf("offshore:      $%d in the account at the end, $%d in fees per run, %d lots over the line (totals over %d runs); %d retired, scoring $%d (median)\n", l.offshore/runs, l.fees/runs, l.structured, runs, len(l.retired), score)
	}
	if l.assetRuns > 0 {
		var ids []string
		for _, a := range cfg.Assets.Offers {
			if l.assetsOwned[a.ID] > 0 {
				ids = append(ids, fmt.Sprintf("%s %d", a.ID, l.assetsOwned[a.ID]))
			}
		}
		fmt.Printf("assets:        %.1f bought per run for $%d, $%d upkeep per run, %.1f seized per run, %.1f task forces per run, %d tunnels found (over %d runs); owned at the end: %s\n",
			float64(l.assetsBought)/float64(runs), l.assetCash/runs, l.assetUpkeep/runs, float64(l.assetsLost)/float64(runs), float64(l.taskForces)/float64(runs), l.tunnelsFound, runs, strings.Join(ids, ", "))
	}
	if l.exportRuns > 0 {
		fmt.Printf("exports:       %.1f loads per run, %d units landed for $%d on $%d off the book per run, %.1f seized per run (%d of %d runs shipped)\n",
			float64(l.exportLoads)/float64(runs), l.exportUnits/runs, l.exportCash/runs, l.exportCost/runs, float64(l.exportsSeized)/float64(runs), l.exportRuns, runs)
	}
	if tr.shipments > 0 {
		fmt.Printf("logistics:     %.1f shipments per run carrying %d units, %.1f seized per run taking %d units (%.0f%% of shipments)\n",
			float64(tr.shipments)/float64(runs), tr.shipped/runs, float64(tr.seizures)/float64(runs), tr.seizedUnits/runs, 100*float64(tr.seizures)/float64(tr.shipments))
	}
	sort.Ints(t.fear)
	sort.Ints(t.respect)
	sort.Ints(t.notor)
	fmt.Printf("reputation:    fear %d respect %d notoriety %d at the end (medians), fear max %d respect max %d notoriety max %d\n",
		t.fear[len(t.fear)/2], t.respect[len(t.respect)/2], t.notor[len(t.notor)/2], t.fear[len(t.fear)-1], t.respect[len(t.respect)-1], t.notor[len(t.notor)-1])
	if t.deals+t.refused+t.offers > 0 {
		sort.Ints(t.trust)
		fmt.Printf("diplomacy:     %d deals struck, %d refused, %d offered by the rival, %d broken by you, %d by them, $%d tribute per run, trust %d at the end (median)\n",
			t.deals, t.refused, t.offers, t.betrayals, t.betrayedBy, t.tribute/runs, t.trust[len(t.trust)/2])
	}
	lw := &t.law
	sort.Ints(lw.pressure)
	sort.Ints(lw.goodwill)
	fmt.Printf("law:           pressure %d goodwill %d at the end (medians), pressure max %d, %d elections, %d chiefs replaced, $%d given per run; DA %v chief %v\n",
		lw.pressure[len(lw.pressure)/2], lw.goodwill[len(lw.goodwill)/2], lw.pressure[len(lw.pressure)-1], lw.elections, lw.chiefs, lw.funded/runs, lw.stances, lw.chiefTempers)
	fmt.Printf("campaigns:     %d backed, %d won, $%d put behind a ticket per run\n", lw.campaigns, lw.campaignsWon, lw.backed/runs)
	if lw.taxed > 0 {
		fmt.Printf("tax:           $%d per run off the free corners of a city held\n", lw.taxed/runs)
	}
	fmt.Printf("bribes:        %d envelopes ($%d per run), %d backfired, %d leads, %d favours called in; %d checkpoints and customs deals ($%d per run)\n", lw.bribes, lw.bribed/runs, lw.backfires, lw.leads, lw.favours, lw.checkpoints, lw.checkpointCash/runs)
	if len(tr.rels) > 0 {
		sort.Ints(tr.rels)
		fmt.Printf("suppliers:     rel %d with the street connect at the end (median), %d days in debt per run, %d late payments, %d freezes, %d collections, $%d taken on credit per run (credit %s)\n",
			tr.rels[len(tr.rels)/2], tr.debtDays/runs, tr.late, tr.frozen, tr.collected, tr.credit/runs, s.credit)
	}
	if tr.housesHeld > 0 || tr.housesLost > 0 {
		fmt.Printf("houses:        %d held at the end per run, %d lost to the landlord, $%d rent per run, %d units lost out of the houses per run; %d stings and raids took %d units per run\n",
			tr.housesHeld/runs, tr.housesLost, tr.rent/runs, tr.houseUnits/runs, tr.raids, tr.raidUnits/runs)
	}
	if tr.deedsBought > 0 {
		fmt.Printf("property:      %d deeds held at the end per run (%d bought, %d seized by the DA), $%d spent and $%d paid back per run, $%d/day rent at the end (means)\n",
			tr.deedsHeld/runs, tr.deedsBought, tr.deedsSeized, tr.deedCash/runs, tr.deedRent/runs, tr.deedRentDay/runs)
	}
	if s.dealing {
		total := 0
		var ids []string
		for _, card := range cfg.Dilemmas.Cards {
			if t.dealt[card.ID] > 0 {
				total += t.dealt[card.ID]
				ids = append(ids, fmt.Sprintf("%s %d", card.ID, t.dealt[card.ID]))
			}
		}
		fmt.Printf("cards:         %.1f per run answered %s, %d of %d in the deck seen: %s\n", float64(total)/float64(runs), s.cards, len(ids), len(cfg.Dilemmas.Cards), strings.Join(ids, ", "))
	}
	if s.incidents {
		var ids []string
		for _, inc := range cfg.Incidents.Table {
			if t.fired[inc.ID] > 0 {
				ids = append(ids, fmt.Sprintf("%s %d", inc.ID, t.fired[inc.ID]))
			}
		}
		fmt.Printf("incidents:     %.1f per run, %d of %d in the table seen: %s\n", float64(t.firedRuns)/float64(runs), len(ids), len(cfg.Incidents.Table), strings.Join(ids, ", "))
	}
	// The endings (#49): how every run ended, and the median score of
	// the ones that did (the offshore account over one plus the bodies,
	// docs/endings.md); a run still going on the horizon has no score.
	sort.Ints(t.scores)
	median := 0
	if len(t.scores) > 0 {
		median = t.scores[len(t.scores)/2]
	}
	fmt.Printf("endings:       %v; %d of %d ended, scoring $%d (median)\n", t.endings, len(t.scores), runs, median)
}
