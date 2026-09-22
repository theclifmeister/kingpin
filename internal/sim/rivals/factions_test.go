package rivals_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// table is the file with exactly n factions in the run (#43), all at
// home, for the tests that pin the table.
func table(n int) *content.Config {
	cfg := content.MustLoad()
	cfg.Rivals.Factions.Min, cfg.Rivals.Factions.Max = n, n
	cfg.Rivals.Factions.Away = 0
	return cfg
}

// seat puts a faction on a corner, arrived and observed, with muscle
// and a chest, so a test can start from a table already dealt.
func seat(w *game.World, r *game.RivalState, corner string, muscle int) {
	r.Arrived, r.Observed, r.Cash, r.Muscle = 1, true, 200_000, muscle
	c := w.Corner(corner)
	c.Owner, c.Faction, c.Runner, c.Enforcer, c.Since = game.OwnerRival, r.Faction(), 0, 0, 1
}

// The seed deals the table (#43): min to max factions, the rival at
// home first with the draws the duel gave it, the rest with leaders
// nobody else has, at most one chaotic across the table, all at home
// with away = 0, and their chests and muscle the rival's own. A second
// seeding is a no-op.
func TestTableSeeds(t *testing.T) {
	for seed := uint64(1); seed <= 12; seed++ {
		w, s := world(t, table(4), seed)
		d, _ := world(t, duel(), seed)
		if len(w.Rivals) != 4 {
			t.Fatalf("seed %d: %d factions, want 4", seed, len(w.Rivals))
		}
		a, b := w.Rival(), d.Rival()
		if a.Leader != b.Leader || a.Personality != b.Personality || a.Supplier != b.Supplier || a.Cash != b.Cash || a.ID != game.FactionRival {
			t.Fatalf("seed %d: the rival at home is not the duel's: %+v vs %+v", seed, *a, *b)
		}
		seen := map[string]bool{}
		chaotic := 0
		for i, r := range w.Rivals {
			if seen[r.Leader] {
				t.Fatalf("seed %d: two factions led by %s", seed, r.Leader)
			}
			seen[r.Leader] = true
			if r.Personality == "chaotic" {
				chaotic++
			}
			if r.Home != "" || r.Muscle != s.Tuning().StartMuscle || r.Cash <= 0 || r.Trust <= 0 || w.FactionIndex(r.Faction()) != i {
				t.Fatalf("seed %d: faction %d seeded wrong: %+v", seed, i, *r)
			}
			if i > 0 && w.Faction(r.ID) != r {
				t.Fatalf("seed %d: Faction(%q) does not find seat %d", seed, r.ID, i)
			}
		}
		if chaotic > 1 {
			t.Fatalf("seed %d: %d chaotic factions", seed, chaotic)
		}
		s.MigrateFactions(w)
		if len(w.Rivals) != 4 {
			t.Fatalf("seed %d: the migration reseeded a seated table: %d", seed, len(w.Rivals))
		}
	}
	// The count is the file's range by seed.
	counts := map[int]bool{}
	for seed := uint64(1); seed <= 40; seed++ {
		w, _ := world(t, content.MustLoad(), seed)
		counts[len(w.Rivals)] = true
		if n := len(w.Rivals); n < 3 || n > 6 {
			t.Fatalf("seed %d: %d factions, want 3..6", seed, n)
		}
	}
	if len(counts) < 3 {
		t.Fatalf("forty seeds dealt only %v factions", counts)
	}
}

// Arrivals stagger (#43, arrive_gap): the rival at home on arrive_day,
// each seat after it arrive_gap days later; with the whole table at
// home each moves in on a free corner in turn.
func TestArrivalsStagger(t *testing.T) {
	cfg := table(3)
	w, s := world(t, cfg, 3)
	gap, first := cfg.Rivals.Factions.ArriveGap, cfg.Rivals.Rivals.ArriveDay
	for i, r := range w.Rivals {
		if got := s.ArriveDay(w, r); got != first+i*gap {
			t.Fatalf("seat %d arrives day %d, want %d", i, got, first+i*gap)
		}
	}
	for w.Day < first+2*gap {
		step(w, s)
		for i, r := range w.Rivals {
			if r.Arrived > 0 && r.Arrived < first+i*gap {
				t.Fatalf("seat %d arrived on day %d, before its day %d", i, r.Arrived, first+i*gap)
			}
		}
	}
	for i, r := range w.Rivals {
		if r.Arrived == 0 {
			t.Fatalf("seat %d never arrived by day %d: %+v", i, w.Day, *r)
		}
		if n := w.RivalHeldBy(r.Faction()); n == 0 {
			t.Fatalf("seat %d holds nothing", i)
		}
	}
}

// Factions contest each other (#43): a faction pushes on a bordering
// corner of another at the push odds, muscle against muscle, on the
// factions stream; a push that lands hands the corner over and the
// loser holds a grudge and trusts the winner less; every push draws
// the file's heat on the city for the heat sim. Neither writes into
// the player's corners or war.
func TestFactionsContestEachOther(t *testing.T) {
	cfg := table(3)
	cfg.Rivals.Factions.PushHeat = 2
	w, s := world(t, cfg, 2)
	f1, f2 := w.Rivals[0], w.Rivals[1]
	f1.Personality, f2.Personality = "expansionist", "defensive"
	seat(w, f1, "railyard", 12)
	seat(w, f2, "oldmill", 1) // borders railyard
	w.Day = 40
	if odds := s.FactionOdds(w, f1, f2); odds <= 0 || odds > cfg.Rivals.Rivals.PushFlip {
		t.Fatalf("faction odds %.2f", odds)
	}
	var pushed *events.FactionPushed
	for w.Day < 200 && pushed == nil {
		evs := step(w, s)
		pushed = find[events.FactionPushed](evs)
		if p := find[events.RivalPushed](evs); p != nil {
			t.Fatalf("day %d: a push on the player with none of theirs contested: %+v", w.Day, *p)
		}
	}
	if pushed == nil {
		t.Fatal("no faction pushed on another in 160 days")
	}
	if pushed.Faction != f1.Faction() || pushed.Against != f2.Faction() || pushed.AgainstRival != f2.Leader || pushed.Heat != 2 || pushed.City != w.Home().ID {
		t.Fatalf("push %+v", *pushed)
	}
	took := false
	for w.Day < 400 && !took {
		for _, e := range step(w, s) {
			if p, ok := e.(events.FactionPushed); ok && p.Taken {
				took = true
				c := w.Corner(p.Corner)
				if c.Owner != game.OwnerRival || c.Faction != p.Faction {
					t.Fatalf("taken corner %+v", *c)
				}
				loser := w.Faction(p.Against)
				if loser.Grudges[p.Faction] == 0 || loser.TrustIn(p.Faction) >= 50 || loser.LastTakenBy != p.Faction {
					t.Fatalf("the loser holds nothing against the winner: %+v", *loser)
				}
			}
		}
	}
	if !took {
		t.Fatal("a 12-head faction never took a corner off a 1-head one")
	}
	if w.Rival().War != 0 {
		t.Fatalf("a fight between factions moved the rival's war with you: %.1f", w.Rival().War)
	}
}

// A faction with no corners for absorb_days since another took its
// last is absorbed by it (#43): its muscle joins the taker, its deals
// end, its offers lapse and it steps no more. Routed by you it is
// never absorbed: it regroups as the duel's rival did.
func TestAbsorption(t *testing.T) {
	cfg := table(3)
	w, s := world(t, cfg, 4)
	f1, f2 := w.Rivals[0], w.Rivals[1]
	seat(w, f1, "oldmill", cfg.Rivals.Rivals.StartMuscle) // never a head fewer for arrears
	f2.Arrived, f2.Observed, f2.Muscle, f2.Cash = 1, true, 3, 1_000_000
	f2.Routed, f2.LastTakenBy = 30, f1.Faction()
	f2.Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: 100}, Since: 20, Until: 200, Faction: f2.Faction()}}
	w.Offers = []game.Offer{{ID: 1, Deal: game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: 30}, Faction: f2.Faction()}, Expires: 300, Faction: f2.Faction()}}
	w.Day = 30 + cfg.Rivals.Factions.AbsorbDays - 2
	first := step(w, s)
	if find[events.RivalAbsorbed](first) != nil {
		t.Fatal("absorbed a day early")
	}
	if find[events.DealEnded](first) == nil || len(w.Offers) != 0 {
		t.Fatalf("a routed faction's deals and offers stand: %v %d", kinds(first), len(w.Offers))
	}
	evs := step(w, s)
	ab := find[events.RivalAbsorbed](evs)
	if ab == nil || ab.Faction != f2.Faction() || ab.ByFaction != f1.Faction() || ab.Muscle < cfg.Rivals.Rivals.StartMuscle {
		t.Fatalf("day %d: %+v %+v", w.Day, kinds(evs), ab)
	}
	if !f2.Gone() || f2.Absorbed != w.Day || f2.AbsorbedBy != f1.Faction() || f2.Muscle != 0 || len(f2.Deals) != 0 || len(w.Offers) != 0 || f1.Muscle != cfg.Rivals.Rivals.StartMuscle+ab.Muscle {
		t.Fatalf("after: f2 %+v f1 muscle %d offers %d", *f2, f1.Muscle, len(w.Offers))
	}
	if w.Stats.Absorbed != 1 {
		t.Fatalf("stats %+v", w.Stats)
	}
	// Routed by you: never.
	f3 := w.Rivals[2]
	f3.Arrived, f3.Muscle, f3.Routed, f3.LastTakenBy = 1, 2, 1, ""
	for i := 0; i < cfg.Rivals.Factions.AbsorbDays+5; i++ {
		step(w, s)
	}
	if f3.Gone() {
		t.Fatalf("a faction you routed was absorbed: %+v", *f3)
	}
}

// The alliance (#43): when an expansionist pushes on you, a defensive
// faction in the city sides with you (Ally you, Against it), its trust
// in you up by ally_trust a push, and at ally_line its front-line
// muscle joins your strikes on the expansionist at ally_share.
func TestDefensiveAlliesWithTheExpansionistsVictim(t *testing.T) {
	cfg := table(3)
	w, s := world(t, cfg, 5)
	f1, f2 := w.Rivals[0], w.Rivals[1]
	f1.Personality, f2.Personality = "expansionist", "defensive"
	f1.Grudge = 4            // pushes at full pace
	seat(w, f1, "depot", 10) // borders fourth, where you stand
	seat(w, f2, "heights", 6)
	w.Day = 40
	trust := f2.Trust
	var pushed bool
	for w.Day < 200 && !pushed {
		for _, e := range step(w, s) {
			if p, ok := e.(events.RivalPushed); ok && p.Faction == f1.Faction() {
				pushed = true
			}
			if p, ok := e.(events.CornerTaken); ok && p.Faction == f1.Faction() && p.From == game.OwnerPlayer {
				pushed = true
			}
		}
	}
	if !pushed {
		t.Fatal("the expansionist never pushed on you")
	}
	if f2.Ally != game.FactionYou || f2.Against != f1.Faction() || f2.Trust <= trust {
		t.Fatalf("the defensive faction did not side with you: %+v (trust was %.0f)", *f2, trust)
	}
	if v := s.AllyMuscle(w, game.FactionYou, f1.Faction()); v != 0 {
		t.Fatalf("under the line the ally lends %.2f", v)
	}
	f2.Trust = cfg.Rivals.Factions.AllyLine
	w.Corner("riverside").Owner, w.Corner("riverside").Faction = game.OwnerRival, f1.Faction() // a front line for f2
	if v := s.AllyMuscle(w, game.FactionYou, f1.Faction()); v <= 0 {
		t.Fatalf("at the line the ally lends %.2f", v)
	}
	alone := s.AllyMuscle(w, game.FactionYou, f2.Faction())
	if alone != 0 {
		t.Fatalf("the ally lends muscle against itself: %.2f", alone)
	}
	with := s.Odds(w, f1, events.ForcePush)
	f2.Trust = 0
	if without := s.Odds(w, f1, events.ForcePush); with <= without {
		t.Fatalf("an ally's muscle did not lift the odds: %.3f with, %.3f without", with, without)
	}
}

// Poaching (#43): the richest faction with poach_cash corner-days in
// the chest offers your least loyal member poach_mul their wage, off
// the poach stream; under poach_line they go (CrewPoached), over it
// they stay with the dip named. Nobody poaches in a duel, and a
// lieutenant running a city is never the one asked.
func TestPoaching(t *testing.T) {
	cfg := table(3)
	cfg.Rivals.Factions.PoachChance = 1
	w, s := world(t, cfg, 6)
	f1, f2 := w.Rivals[0], w.Rivals[1]
	seat(w, f1, "heights", 3)
	seat(w, f2, "oldmill", 3)
	f2.Cash = 10_000_000 // the richest
	w.Crew.Members[1].Loyalty = cfg.Rivals.Factions.PoachLine - 1
	w.Crew.Members[0].Loyalty = 90
	w.Day = 40
	if p := s.Poacher(w); p != f2 {
		t.Fatalf("poacher %+v", p)
	}
	evs := step(w, s)
	got := find[events.CrewPoached](evs)
	if got == nil || got.ID != 2 || got.Name != "Tank" || got.Faction != f2.Faction() || got.Rival != f2.Leader || got.Stayed || got.Wages != s.PoachOffer(w.Crew.Members[1]) {
		t.Fatalf("poached %+v", got)
	}
	if w.Stats.CrewPoached != 1 {
		t.Fatalf("stats %+v", w.Stats)
	}
	// Over the line they stay, a little less loyal.
	w.Crew.Members[1].Loyalty = cfg.Rivals.Factions.PoachLine + 1
	got = find[events.CrewPoached](step(w, s))
	if got == nil || !got.Stayed || got.Dip != cfg.Rivals.Factions.PoachDip {
		t.Fatalf("stayed %+v", got)
	}
	// The duel poaches nobody.
	d, ds := world(t, duel(), 6)
	seat(d, d.Rival(), "oldmill", 3)
	d.Rival().Cash = 10_000_000
	d.Crew.Members[1].Loyalty = 1
	d.Day = 40
	if find[events.CrewPoached](step(d, ds)) != nil {
		t.Fatal("the duel poached")
	}
}

// The leader's arrest (#43): a faction's heat past leader_arrest_heat
// on your tips takes its leader, and it fragments: RivalLeaderArrested
// with its corners and muscle, its deals ended, its muscle gone to the
// pool, its corners drifting to the street over fragment_days in an
// order the factions stream drew, and it steps no more. The market
// sim reads Fragmented for the price spike the morning after.
func TestTippingFragments(t *testing.T) {
	cfg := table(3)
	w, s := world(t, cfg, 7)
	f1 := w.Rival()
	seat(w, f1, "oldmill", 6)
	for _, id := range []string{"railyard", "projects", "riverside", "heights", "precinct"} {
		c := w.Corner(id)
		c.Owner, c.Faction = game.OwnerRival, f1.Faction()
	}
	f1.Deals = []game.Deal{{Kind: game.DealSplit, Terms: game.Terms{Corners: []string{"fourth"}}, Since: 1}}
	w.Day = 40
	var arrested *events.RivalLeaderArrested
	held, muscle := 0, 0
	for w.Day < 100 && arrested == nil {
		held, muscle = w.RivalHeldBy(f1.Faction()), f1.Muscle
		target := ""
		for _, c := range w.Home().Corners {
			if c.FactionID() == f1.Faction() {
				target = c.ID // a raid on the way up takes a corner: tip on whatever is left
				break
			}
		}
		if err := w.Tip(target); err != nil {
			t.Fatal(err)
		}
		arrested = find[events.RivalLeaderArrested](step(w, s))
		w.Today.Tipoff = nil // the clock's job, in a run
	}
	if arrested == nil {
		t.Fatalf("a tip a night never took the leader: heat %.0f", f1.Heat)
	}
	if arrested.Faction != f1.Faction() || arrested.Corners != held || arrested.Muscle > muscle || arrested.Muscle <= 0 || arrested.Killed || arrested.City != w.Home().ID {
		t.Fatalf("arrest %+v (held %d muscle %d)", *arrested, held, muscle)
	}
	if !f1.Gone() || f1.Fragmented != w.Day || f1.Muscle != 0 || len(f1.Deals) != 0 || len(f1.Fragments) != held || w.Stats.Fragmented != 1 {
		t.Fatalf("after the arrest: %+v", *f1)
	}
	day := f1.Fragmented
	for len(f1.Fragments) > 0 {
		if w.Day >= day+cfg.Rivals.Factions.FragmentDays {
			t.Fatalf("day %d: %d corners still to drift past fragment_days", w.Day, len(f1.Fragments))
		}
		n := w.RivalHeldBy(f1.Faction())
		evs := step(w, s)
		if w.RivalHeldBy(f1.Faction()) >= n {
			t.Fatalf("day %d: no corner drifted (%d held)", w.Day, n)
		}
		for _, e := range evs {
			if ab, ok := e.(events.RivalAbandoned); ok && ab.Reason != "fragmented" {
				t.Fatalf("drift %+v", ab)
			}
		}
	}
	if w.RivalHeldBy(f1.Faction()) != 0 || len(f1.Fragments) != 0 {
		t.Fatalf("after fragment_days %d corners still theirs: %+v", w.RivalHeldBy(f1.Faction()), f1.Fragments)
	}
	for _, c := range w.Home().Corners {
		if c.Owner == game.OwnerNone && c.Faction != "" {
			t.Fatalf("a free corner still names a faction: %+v", c)
		}
	}
	// The world's incident kills the biggest faction in its city the
	// same tick (#44's rival_leader_killed).
	f2 := w.Rivals[1]
	seat(w, f2, "docks", 4)
	tick := &game.Tick{Day: w.Day + 1, RNG: game.RNGFor(w.Seed, w.Day+1)}
	tick.Emit(events.Incident{Day: tick.Day, ID: "rival_leader_killed", City: w.Home().ID, LeaderKilled: true})
	s.Step(w, tick)
	w.Day++
	killed := find[events.RivalLeaderArrested](tick.Events())
	if killed == nil || !killed.Killed || killed.Faction != f2.Faction() || !f2.Gone() {
		t.Fatalf("the incident did not kill the leader: %+v f2 %+v", killed, *f2)
	}
}

// Homage (#43, #32's tribute in reverse): a faction your enforcers have
// taken tribute_corners off offers homage_cut of its take a day, off
// the homage stream; taken, it pays you every night out of its chest
// and stays off your corners, and a night it cannot pay ends it.
// Dominant is every faction on the ground gone or paying.
func TestHomageAndDominant(t *testing.T) {
	cfg := table(3)
	cfg.Rivals.Factions.HomageChance = 1
	w, s := world(t, cfg, 8)
	f1, f2, f3 := w.Rivals[0], w.Rivals[1], w.Rivals[2]
	seat(w, f1, "oldmill", 4)
	seat(w, f2, "heights", 2)
	f1.LostToYou = cfg.Rivals.Factions.TributeCorners
	w.Day = 40
	if w.Dominant() {
		t.Fatal("dominant with two factions on the ground and no deal")
	}
	evs := step(w, s)
	off := find[events.DealOffered](evs)
	if off == nil || off.Deal != game.DealHomage || off.Faction != f1.Faction() {
		t.Fatalf("offer %+v", kinds(evs))
	}
	o := w.Offers[0]
	if o.With() != f1.Faction() || o.Deal.Terms.PerDay <= 0 || o.Deal.Terms.PerDay > s.Income(w, f1) {
		t.Fatalf("the offer: %+v (take %d)", o, s.Income(w, f1))
	}
	if _, err := w.Accept(o.ID); err != nil {
		t.Fatal(err)
	}
	evs = step(w, s)
	w.Today.Accepted = nil // the clock's job, in a run
	if find[events.DealAccepted](evs) == nil || w.DealWith(f1.Faction(), game.DealHomage) == nil || find[events.TributePaid](evs) == nil {
		t.Fatalf("not sealed and paid the night it was taken, as a tribute is: %v", kinds(evs))
	}
	cash := w.Player.DirtyCash
	chest := f1.Cash
	evs = step(w, s)
	paid := find[events.TributePaid](evs)
	if paid == nil || !paid.ToYou || paid.Amount != o.Deal.Terms.PerDay || w.Player.DirtyCash != cash+paid.Amount || f1.Cash != chest+s.Income(w, f1)-s.Wages(w, f1)-paid.Amount || w.Stats.Homage != 2*paid.Amount {
		t.Fatalf("paid %+v; cash %d -> %d, chest %d -> %d (take %d wages %d)", paid, cash, w.Player.DirtyCash, chest, f1.Cash, s.Income(w, f1), s.Wages(w, f1))
	}
	if !w.AtPeaceWith(f1.Faction()) || w.CanUndercut("oldmill") == nil {
		t.Fatal("a homage is not a peace")
	}
	// Dominant once the others are gone.
	f2.Absorbed, f2.Muscle = 1, 0
	f3.Fragmented = 1
	if w.Dominant() {
		t.Fatal("dominant with a faction still in the wings")
	}
	f3.Arrived = 1
	if !w.Dominant() {
		t.Fatal("not dominant with one paying and two gone")
	}
	// A night it cannot pay ends it.
	f1.Cash = 0
	f1.Muscle = 0
	for _, id := range []string{"oldmill", "railyard", "projects", "riverside"} {
		w.Corner(id).Owner, w.Corner(id).Faction = game.OwnerNone, "" // no take
	}
	w.Corner("docks").Owner, w.Corner("docks").Faction = game.OwnerRival, f1.Faction()
	w.Home().Market["weed"].Demand = 0
	evs = step(w, s)
	if find[events.DealEnded](evs) == nil || w.DealWith(f1.Faction(), game.DealHomage) != nil || w.Dominant() {
		t.Fatalf("a homage it could not pay held: %v dominant %v", kinds(evs), w.Dominant())
	}
}

// Trust spreads (#43, betrayal_spread): the step you break a deal with
// one faction, every other faction alive loses the spread of its trust
// in you, once; a deal kept with one moves nobody else's.
func TestBetrayalSpreads(t *testing.T) {
	cfg := table(3)
	w, s := world(t, cfg, 9)
	f1, f2, f3 := w.Rivals[0], w.Rivals[1], w.Rivals[2]
	seat(w, f1, "oldmill", 4)
	seat(w, f2, "heights", 4)
	seat(w, f3, "docks", 4)
	f1.Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: 100}, Since: 1, Until: 200}}
	f2.Trust, f3.Trust = 60, 50
	w.Day = 40
	step(w, s)
	if f2.Trust != 60 || f3.Trust != 50 {
		t.Fatalf("a deal kept with one moved the others: %.0f %.0f", f2.Trust, f3.Trust)
	}
	if err := w.SendEnforcers("oldmill", events.ForceHit); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	if find[events.DealBroken](evs) == nil {
		t.Fatalf("no betrayal: %v", kinds(evs))
	}
	spread := cfg.Rivals.Diplomacy.BetrayalSpread
	if f2.Trust != 60-spread || f3.Trust != 50-spread {
		t.Fatalf("trust after the betrayal: %.0f %.0f, want %.0f %.0f", f2.Trust, f3.Trust, 60-spread, 50-spread)
	}
	sp := find[events.TrustSpread](evs)
	if sp == nil || sp.Others != 2 || sp.Spread != spread || sp.Faction != f1.Faction() {
		t.Fatalf("spread %+v", sp)
	}
}

// The table's share (#43, table_share): once the factions hold their
// share of a city between them, none sets up on a free corner there
// and each pushes on you at its past-cap pace; a run that never fills
// it is untouched.
func TestTableShareCapsTheClaims(t *testing.T) {
	cfg := table(3)
	w, s := world(t, cfg, 10)
	f1, f2 := w.Rivals[0], w.Rivals[1]
	f1.Personality = "expansionist"
	seat(w, f1, "oldmill", 8)
	seat(w, f2, "heights", 4)
	for _, id := range []string{"railyard", "projects", "riverside", "precinct"} {
		c := w.Corner(id)
		c.Owner, c.Faction = game.OwnerRival, f1.Faction()
	}
	if !s.TableFull(w, f1) {
		t.Fatal("six of ten is not full at 0.6")
	}
	w.Day = 40
	for i := 0; i < 60; i++ {
		for _, e := range step(w, s) {
			switch ev := e.(type) {
			case events.RivalEyeing, events.RivalMovedIn:
				t.Fatalf("day %d: a claim with the table full: %+v", w.Day, ev)
			case events.CornerTaken:
				if ev.From == game.OwnerNone {
					t.Fatalf("day %d: a claim with the table full: %+v", w.Day, ev)
				}
			}
		}
	}
	w.Corner("precinct").Owner, w.Corner("precinct").Faction = game.OwnerNone, ""
	if s.TableFull(w, f1) {
		t.Fatal("five of ten is full")
	}
}

// The duel is the duel (#43): with one faction in the run none of the
// table's events fire, none of its state moves and the rival at home
// rolls on the tick's stream alone, over a long run with every kind of
// pressure on it.
func TestDuelHasNoTable(t *testing.T) {
	w, s := world(t, duel(), 11)
	seat(w, w.Rival(), "oldmill", 4)
	w.Day = 20
	for i := 0; i < 200; i++ {
		if i%7 == 0 {
			_ = w.SendEnforcers("oldmill", events.ForcePush)
		}
		for _, e := range step(w, s) {
			switch e.(type) {
			case events.FactionPushed, events.RivalAbsorbed, events.RivalLeaderArrested, events.CrewPoached, events.TrustSpread:
				t.Fatalf("day %d: %s in a duel", w.Day, e.Kind())
			}
			if d, ok := e.(events.DealOffered); ok && d.Deal == game.DealHomage {
				t.Fatalf("day %d: a homage offered in a duel", w.Day)
			}
		}
	}
	r := w.Rival()
	if len(w.Rivals) != 1 || r.Home != "" || len(r.Grudges) != 0 || len(r.Trusts) != 0 || r.Ally != "" || r.Absorbed != 0 || r.Fragmented != 0 || r.LastTakenBy != "" {
		t.Fatalf("the duel's rival carries the table's state: %+v", *r)
	}
}

// Away (#43): with the knob on, a seat after the first may live in the
// other city, at most away_max of them, and its ground, its corner-day
// and its tribute base are that city's.
func TestAwayFactionLivesInTheOtherCity(t *testing.T) {
	cfg := table(4)
	cfg.Rivals.Factions.Away, cfg.Rivals.Factions.AwayMax = 1, 1
	found := false
	for seed := uint64(1); seed <= 6 && !found; seed++ {
		w, s := world(t, cfg, seed)
		w.AddCity(game.StartingCity{ID: "bayport", Name: "Bayport", Products: []game.StartingProduct{{ID: "weed", Name: "Weed", Price: 40, Demand: 30, SupplierRatio: 0.55}}})
		w.Rivals = w.Rivals[:1]
		s.MigrateFactions(w) // reseed with two cities on the map
		away := 0
		for _, r := range w.Rivals {
			if r.Home == "bayport" {
				away++
				found = true
				if w.CityOf(r).ID != "bayport" || s.Standard(w, r) != 40*30 {
					t.Fatalf("seed %d: an away faction's city is wrong: %+v standard %.0f", seed, *r, s.Standard(w, r))
				}
			}
		}
		if away > 1 {
			t.Fatalf("seed %d: %d factions away, away_max 1", seed, away)
		}
	}
	if !found {
		t.Fatal("six seeds at away = 1 seated nobody in Bayport")
	}
}

// The reign (#227): the morning Dominant() has held dominant_days with
// the city held, the sim stamps World.Reign and emits ReignBegan rather
// than ending the run; the crown is open (CanCrown). A faction setting
// up again (its homage ending) zeroes the stamp (ReignBroken), the
// crown closes, and a new homage deal begins it again dominant_days
// on; the run is never ended by the sim.
func TestReignBreaks(t *testing.T) {
	cfg := table(3)
	cfg.Rivals.Factions.HomageChance = 0
	w, s := world(t, cfg, 8)
	f1, f2, f3 := w.Rivals[0], w.Rivals[1], w.Rivals[2]
	days := cfg.Rivals.Endings.DominantDays
	// The city held: more than kingpin_share of home's corners with a
	// runner on each; f2 and f3 gone, f1 on one corner paying homage.
	home := w.Home()
	n := int(cfg.Rivals.Endings.KingpinShare*float64(len(home.Corners))) + 1
	for i := 0; i < n; i++ {
		c := &home.Corners[i]
		c.Owner, c.Faction, c.Runner, c.Since = game.OwnerPlayer, "", 1, 1
	}
	seat(w, f1, home.Corners[len(home.Corners)-1].ID, 2)
	f1.Cash = 10_000_000
	f2.Arrived, f2.Absorbed = 1, 1
	f3.Arrived, f3.Fragmented = 1, 1
	w.Day = 20
	f1.Deals = append(f1.Deals, game.Deal{Kind: game.DealHomage, Terms: game.Terms{PerDay: 100}, Since: w.Day, Faction: f1.Faction()})
	if !w.Dominant() || !s.HoldsTheCity(w) || w.CanCrown() {
		t.Fatalf("dominant %v holds %v crown %v", w.Dominant(), s.HoldsTheCity(w), w.CanCrown())
	}
	var began *events.ReignBegan
	for w.Day-20 <= days+2 && began == nil {
		evs := step(w, s)
		began = find[events.ReignBegan](evs)
		if w.Over != nil {
			t.Fatalf("the sim ended the run: %+v", w.Over)
		}
	}
	if began == nil || w.Reign != w.Day || w.Day-20 != days || !w.CanCrown() || began.Crews != 1 || began.Homage != 100 {
		t.Fatalf("the reign: began %+v, Reign %d on day %d (dominant since %d), crown %v", began, w.Reign, w.Day, s.DominantSince(w), w.CanCrown())
	}
	if w.ReignDay() != 1 {
		t.Fatalf("day %d of the reign on its first morning", w.ReignDay())
	}
	evs := step(w, s)
	if find[events.ReignBegan](evs) != nil || w.Reign != w.Day-1 || w.ReignDay() != 2 {
		t.Fatalf("the second morning: %v, Reign %d, day %d of it", kinds(evs), w.Reign, w.ReignDay())
	}
	// f1's homage ends (it sets up again): the reign breaks.
	f1.Deals = nil
	evs = step(w, s)
	broke := find[events.ReignBroken](evs)
	if broke == nil || w.Reign != 0 || w.CanCrown() || broke.Why != "a crew set up again" {
		t.Fatalf("the reign did not break: %+v Reign %d crown %v", broke, w.Reign, w.CanCrown())
	}
	if err := w.Crown(); err != game.ErrNoReign {
		t.Fatalf("the crown with no reign: %v", err)
	}
	// A new homage deal, and dominant_days on it begins again.
	f1.Deals = append(f1.Deals, game.Deal{Kind: game.DealHomage, Terms: game.Terms{PerDay: 100}, Since: w.Day, Faction: f1.Faction()})
	from := w.Day
	began = nil
	for w.Day-from <= days+2 && began == nil {
		began = find[events.ReignBegan](step(w, s))
	}
	if began == nil || w.Day-from != days || !w.CanCrown() {
		t.Fatalf("the reign did not begin again: %+v on day %d from %d", began, w.Day, from)
	}
	// The share falling breaks it too.
	home.Corners[0].Owner, home.Corners[0].Runner = game.OwnerNone, 0
	broke = find[events.ReignBroken](step(w, s))
	if broke == nil || broke.Why != "the city slipped under the share" || w.Reign != 0 {
		t.Fatalf("the share: %+v Reign %d", broke, w.Reign)
	}
	// And the crown, taken while it holds, is the kingpin ending.
	home.Corners[0].Owner, home.Corners[0].Runner = game.OwnerPlayer, 1
	f1.Deals[0].Since = w.Day - days
	if find[events.ReignBegan](step(w, s)) == nil || !w.CanCrown() {
		t.Fatal("the reign did not begin on the stamp")
	}
	if err := w.Crown(); err != nil || w.Over == nil || w.Over.Cause != content.CauseKingpin || w.Over.Day != w.Day {
		t.Fatalf("the crown: %v %+v", err, w.Over)
	}
}

// The war order (#229): declared on a faction, the sim sends the hand's
// strike every night the hand sends none, at the file's dial on the
// faction's corner nearest your front line (WarTarget: bordering yours
// first, the biggest first), with CornerStruck.War set; a hand's strike
// takes the night instead; the war ends on its own the night the
// faction is gone, pays homage or holds no corner in a city you hold,
// and never writes Over; boxed (NoWar's dial) it sends nothing.
func TestWarEndsWhenTheFactionFolds(t *testing.T) {
	cfg := table(2)
	force, on := cfg.Rivals.War.Force()
	if !on || force != events.ForceHit {
		t.Fatalf("the file's war dial: %v %v", force, on)
	}
	w, s := world(t, cfg, 5)
	f1, f2 := w.Rivals[0], w.Rivals[1]
	home := w.Home()
	// You on the first corner, f1 on two: one bordering yours, one not.
	you := &home.Corners[0]
	you.Owner, you.Runner, you.Since = game.OwnerPlayer, 1, 1
	var next, far *game.Corner
	for i := range home.Corners {
		c := &home.Corners[i]
		if c.ID == you.ID {
			continue
		}
		if c.Borders(*you) && next == nil {
			next = c
		} else if !c.Borders(*you) && far == nil {
			far = c
		}
	}
	seat(w, f1, next.ID, 6)
	seat(w, f1, far.ID, 6)
	f1.Muscle, f1.Cash = 6, 1_000_000
	f2.Arrived, f2.Absorbed = 1, 1
	w.Day = 10
	if err := w.DeclareWar(f1.Faction()); err != nil {
		t.Fatal(err)
	}
	if err := w.DeclareWar(f1.Faction()); err != game.ErrAtWar {
		t.Fatalf("a second war: %v", err)
	}
	if c := s.WarTarget(w, f1); c == nil || c.ID != next.ID {
		t.Fatalf("the target is %+v, want the corner on your front line %s", c, next.ID)
	}
	evs := step(w, s)
	struck := find[events.CornerStruck](evs)
	if struck == nil || !struck.War || struck.Corner != next.ID || struck.Force != events.ForceHit || w.Stats.Strikes != 1 || f1.LastStruck != w.Day {
		t.Fatalf("the war's first night: %+v strikes %d", struck, w.Stats.Strikes)
	}
	if w.Over != nil {
		t.Fatalf("the war ended the run: %+v", w.Over)
	}
	// A hand's strike takes the night: one strike, the hand's.
	if err := w.SendEnforcers(far.ID, events.ForceWarn); err != nil {
		t.Fatal(err)
	}
	evs = step(w, s)
	w.Today.Strike = nil
	n := 0
	for _, e := range evs {
		if ev, ok := e.(events.CornerStruck); ok {
			n++
			if ev.War || ev.Corner != far.ID || ev.Force != events.ForceWarn {
				t.Fatalf("the hand's night: %+v", ev)
			}
		}
	}
	if n != 1 {
		t.Fatalf("%d strikes on the hand's night", n)
	}
	// The faction gone: the war ends on its own, with nothing to end.
	f1.Fragmented = w.Day
	evs = step(w, s)
	ended := find[events.WarEnded](evs)
	if ended == nil || ended.Faction != f1.Faction() || w.War != "" || find[events.CornerStruck](evs) != nil {
		t.Fatalf("the war did not end with the faction gone: %+v war %q %v", ended, w.War, kinds(evs))
	}
	if err := w.CallOffWar(); err != game.ErrNoWar {
		t.Fatalf("calling off no war: %v", err)
	}
	// No corner where you hold ground: refused, and ends a war that
	// loses it. Homage ends it too.
	w, s = world(t, cfg, 5)
	f1, f2 = w.Rivals[0], w.Rivals[1]
	f2.Arrived, f2.Absorbed = 1, 1
	home = w.Home()
	for i := range home.Corners {
		if c := &home.Corners[i]; c.Held() {
			c.Owner, c.Runner, c.Enforcer = game.OwnerNone, 0, 0
		}
	}
	seat(w, f1, home.Corners[1].ID, 6)
	if err := w.DeclareWar(f1.Faction()); err != game.ErrNothingToTake {
		t.Fatalf("a war with no ground held: %v", err)
	}
	home.Corners[0].Owner, home.Corners[0].Runner = game.OwnerPlayer, 1
	w.Day = 10
	if err := w.DeclareWar(f1.Faction()); err != nil {
		t.Fatal(err)
	}
	home.Corners[0].Owner, home.Corners[0].Runner = game.OwnerNone, 0
	if ended := find[events.WarEnded](step(w, s)); ended == nil || w.War != "" {
		t.Fatalf("the war did not end with no ground held: %+v", ended)
	}
	home.Corners[0].Owner, home.Corners[0].Runner = game.OwnerPlayer, 1
	if err := w.DeclareWar(f1.Faction()); err != nil {
		t.Fatal(err)
	}
	f1.Deals = append(f1.Deals, game.Deal{Kind: game.DealHomage, Terms: game.Terms{PerDay: 10}, Since: w.Day, Faction: f1.Faction()})
	f1.Cash = 1_000_000
	if ended := find[events.WarEnded](step(w, s)); ended == nil || ended.Why != "they pay you homage now" || w.War != "" {
		t.Fatalf("the war did not end on homage: %+v", ended)
	}
	// A war on a faction no longer at the table ends naming the faction
	// it was on, not an empty one.
	w.War = "nobody's crew"
	if ended := find[events.WarEnded](step(w, s)); ended == nil || ended.Faction != "nobody's crew" || ended.Why != "they are no more" || w.War != "" {
		t.Fatalf("the war on a missing faction: %+v war %q", ended, w.War)
	}
	// Boxed: the dial off, the war sends nothing.
	boxed := *cfg
	boxed.Rivals.War.Dial = ""
	w, s = world(t, &boxed, 5)
	f1, f2 = w.Rivals[0], w.Rivals[1]
	f2.Arrived, f2.Absorbed = 1, 1
	home = w.Home()
	home.Corners[0].Owner, home.Corners[0].Runner = game.OwnerPlayer, 1
	seat(w, f1, home.Corners[1].ID, 6)
	w.Day = 10
	if err := w.DeclareWar(f1.Faction()); err != nil {
		t.Fatal(err)
	}
	if evs := step(w, s); find[events.CornerStruck](evs) != nil || w.Stats.Strikes != 0 {
		t.Fatalf("a boxed war struck: %v", kinds(evs))
	}
}

// The war is a betrayal (#229): declared under a kept truce, the first
// night's strike breaks every deal with the faction, as a hand's push
// or hit does, and the faction pays it back with a phone call.
func TestWarIsABetrayal(t *testing.T) {
	cfg := table(2)
	w, s := world(t, cfg, 6)
	f1, f2 := w.Rivals[0], w.Rivals[1]
	f2.Arrived, f2.Absorbed = 1, 1
	home := w.Home()
	home.Corners[0].Owner, home.Corners[0].Runner = game.OwnerPlayer, 1
	seat(w, f1, home.Corners[1].ID, 6)
	f1.Trust = 80
	f1.Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: 100}, Since: 5, Until: 200, Faction: f1.Faction()}}
	w.Day = 10
	if !w.AtPeaceWith(f1.Faction()) {
		t.Fatal("no peace to break")
	}
	if err := w.DeclareWar(f1.Faction()); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	broken := find[events.DealBroken](evs)
	if broken == nil || broken.By != "you" || broken.Deal != game.DealTruce || w.AtPeaceWith(f1.Faction()) || f1.Betrayed != w.Day || w.Stats.Betrayals != 1 {
		t.Fatalf("the war under a truce: %+v peace %v betrayed %d stats %d", broken, w.AtPeaceWith(f1.Faction()), f1.Betrayed, w.Stats.Betrayals)
	}
	if find[events.RivalTippedPolice](evs) == nil {
		t.Fatalf("no phone call for the betrayal: %v", kinds(evs))
	}
}
