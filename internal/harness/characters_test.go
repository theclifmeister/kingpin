package harness

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// The characters (#50, docs/profile.md) are starts, not cheats: every
// one passes the four difficulty tests under the harness's policies
// as the default does. On ten seeds each, the aggressive trader is
// arrested or indicted within forty days and out-earns the quiet one
// over its span, the quiet one stays free and earns, and on the
// medians managed beats quiet and crewed beats managed (the per-seed
// reading of the two orderings flips where a start takes a roster
// slot: the cook's seed 5 and the bookkeeper's seed 7 hold five
// runners where the default holds six, the rival takes the last
// corner and the crewed policy never re-posts). The medians of the
// managed, quiet and crewed peaks are logged against the default's,
// every one that moves more than twenty percent marked. The dockhand
// starts in Bayport, where no faction lives, so its take there draws
// one (#341); the harness's players hit the scouts of a faction moving
// on the city they work (HitScoutsIn, #379), and without that answer
// the dockhand's crewed run on seed 6 was indicted on day 113, under
// the tier-3 line.
func TestCharactersAreStartsNotCheats(t *testing.T) {
	t.Parallel()
	// Investigations boxed (#343, the blind sting): this measures the starts against each other, and the police answer every start alike; with them on, a miss cools nothing, so the crewed player who lies low at 40 through two named hits on the bookkeeper's seed 1 peaks at $475k against the blind sting's $593k, under managed's $497k, a second per-seed flip beside seed 7's roster slot (the medians hold).
	cfg := Investigations(content.MustLoad(), false)
	base := measure(t, cfg, "")
	for _, ch := range cfg.Characters.Characters {
		ch := ch
		t.Run(ch.ID, func(t *testing.T) {
			t.Parallel()
			m := measure(t, cfg, ch.ID)
			t.Logf("%s: the aggressive trader lasts %d days (median) against the default's %d", ch.ID, m.aggDays, base.aggDays)
			for _, row := range []struct {
				name      string
				got, want int
			}{{"managed", m.managed, base.managed}, {"quiet", m.quiet, base.quiet}, {"crewed", m.crewed, base.crewed}} {
				delta := float64(row.got-row.want) / float64(row.want) * 100
				mark := ""
				if delta > 20 || delta < -20 {
					mark = "  <- moved more than 20%"
				}
				t.Logf("%s: %s median peak %d against the default's %d (%+.0f%%)%s", ch.ID, row.name, row.got, row.want, delta, mark)
			}
		})
	}
}

// medians are the managed, quiet and crewed peaks' medians over the
// seeds measured.
type medians struct{ managed, quiet, crewed, aggDays int }

// measure runs the four difficulty tests as a character on ten seeds
// and returns the medians of the managed, quiet and crewed peaks.
func measure(t *testing.T, cfg *content.Config, id string) medians {
	t.Helper()
	var mp, qp, cp, ad []int
	flips := 0
	for seed := uint64(1); seed <= 10; seed++ {
		agg, _ := RunAs(cfg, id, seed, Horizon, Trader(cfg, events.DialAggressive))
		if agg.Over == nil || (agg.Over.Cause != content.CauseArrested && agg.Over.Cause != content.CauseIndicted) || agg.Days > 40 {
			t.Fatalf("%q seed %d: the aggressive trader lasted %d days (over %v); a start is not a cheat", id, seed, agg.Days, agg.Over)
		}
		quiet, _ := RunAs(cfg, id, seed, Horizon, Trader(cfg, events.DialQuiet))
		if quiet.Over != nil || quiet.EndCash <= cfg.Market.Market.StartCash {
			t.Fatalf("%q seed %d: the quiet trader ended (%v) or lost money (%d)", id, seed, quiet.Over, quiet.EndCash)
		}
		short, _ := RunAs(cfg, id, seed, max(1, agg.Days-1), Trader(cfg, events.DialQuiet))
		if agg.PeakCash <= short.PeakCash {
			t.Fatalf("%q seed %d: over %d days aggressive peaked at %d, quiet at %d; greed should pay short term", id, seed, agg.Days-1, agg.PeakCash, short.PeakCash)
		}
		managed, _ := RunAs(cfg, id, seed, Horizon, Managed(cfg, 50))
		if managed.Over != nil {
			t.Fatalf("%q seed %d: the managed trader ended on day %d: %s", id, seed, managed.Days, managed.Over.Cause)
		}
		crewed, _ := RunAs(cfg, id, seed, Horizon, Crewed(cfg, 40))
		if (crewed.Over != nil && crewed.Days <= TierDays[2]) || len(crewed.World.Crew.Members) == 0 {
			t.Fatalf("%q seed %d: crewed ended on day %d (%v) with %d on the payroll", id, seed, crewed.Days, crewed.Over, len(crewed.World.Crew.Members))
		}
		if managed.PeakCash <= quiet.PeakCash || crewed.PeakCash <= managed.PeakCash {
			flips++
			t.Logf("%q seed %d: quiet %d managed %d crewed %d; the ordering flips on this seed", id, seed, quiet.PeakCash, managed.PeakCash, crewed.PeakCash)
		}
		mp = append(mp, managed.PeakCash)
		qp = append(qp, quiet.PeakCash)
		cp = append(cp, crewed.PeakCash)
		ad = append(ad, agg.Days)
	}
	sort.Ints(mp)
	sort.Ints(qp)
	sort.Ints(cp)
	sort.Ints(ad)
	m := medians{mp[len(mp)/2], qp[len(qp)/2], cp[len(cp)/2], ad[len(ad)/2]}
	if m.managed <= m.quiet || m.crewed <= m.managed {
		t.Fatalf("%q: medians quiet %d managed %d crewed %d; managing heat and runners should pay", id, m.quiet, m.managed, m.crewed)
	}
	if flips > 1 {
		t.Fatalf("%q: the ordering flipped on %d of 10 seeds", id, flips)
	}
	return m
}

// TestProfileNeverTouchesTheRun (#50): the default character on a seed
// is the run the seed always was, byte for byte, with no profile, an
// empty one and a full one under KINGPIN_HOME (the profile is the UI's
// file: nothing in game, sim or the harness reads it), and
// sim.NewWorldWith as the default character is sim.NewWorld.
func TestProfileNeverTouchesTheRun(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	const seed, days = 11, 60
	play := func(w *game.World) string {
		res, err := RunFrom(cfg, w, days, Crewed(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		return digest(res.World)
	}
	none := play(sim.NewWorld(cfg, seed))
	if err := game.SaveProfile(game.NewProfile()); err != nil {
		t.Fatal(err)
	}
	empty := play(sim.NewWorldWith(cfg, seed, game.Start{Character: cfg.Characters.Default().ID}))
	full := game.NewProfile()
	for _, ch := range cfg.Characters.Characters {
		full.Unlocks[ch.ID] = true
	}
	full.Unlocks[game.HardDAID] = true
	full.Record(cfg.Characters, cfg.Progression, game.RunRecord{Seed: 3, Ending: content.CauseKingpin, Score: 1_000_000, Days: 300, Stage: "cartel", Date: "20260913", Daily: "20260913"})
	full.SavePreset(game.Preset{Name: "Day 40", Day: 40, Launder: events.LaunderGreedy, Pay: events.PayGenerous}) // a saved preset (#357) is the profile's too
	if err := game.SaveProfile(full); err != nil {
		t.Fatal(err)
	}
	again := play(Play0(cfg, seed))
	if none != empty || none != again {
		t.Fatal("the default character's run moved under a profile")
	}
}

// Play0 is a fresh default world: the harness's runs read no profile.
func Play0(cfg *content.Config, seed uint64) *game.World { return sim.NewWorld(cfg, seed) }

// TestCharacterIsDayZero (#50): a character changes the world on day 0
// and nothing after reads it: a character's world on day 0 differs
// from the default's (the dealer's is the default's), and the run from
// it with the character's id wiped off the world is byte for byte the
// run with it kept, on every day to sixty, under a policy that hires,
// holds ground and washes.
func TestCharacterIsDayZero(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	const seed, days = 5, 60
	encode := digest
	base := encode(sim.NewWorld(cfg, seed))
	for _, ch := range cfg.Characters.Characters {
		kept := sim.NewWorldWith(cfg, seed, Character(cfg, ch.ID))
		wiped := sim.NewWorldWith(cfg, seed, Character(cfg, ch.ID))
		wiped.Start = game.Start{}
		if ch.Start.Empty() {
			if encode(kept) != base {
				t.Errorf("%s: an empty start moved the world on day 0", ch.ID)
			}
		} else if encode(wiped) == base {
			t.Errorf("%s: the start left the world as the default's on day 0", ch.ID)
		}
		a, _ := RunFrom(cfg, kept, days, Laundered(cfg, 40))
		b, _ := RunFrom(cfg, wiped, days, Laundered(cfg, 40))
		wiped.Start = kept.Start
		if encode(a.World) != encode(b.World) {
			t.Errorf("%s: something after day 0 read the character", ch.ID)
		}
	}
}

// TestCharacterStartsAreDeterministic (#50): every character's day-0
// world is the same twice on a seed, its start is what the file says
// (the crew by role, the nodes, the products, the corner, the
// reputation, the chief's temper in the file), and it differs between
// seeds where the start rolls dice (the crew's stats).
func TestCharacterStartsAreDeterministic(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	for _, ch := range cfg.Characters.Characters {
		a := sim.NewWorldWith(cfg, 3, Character(cfg, ch.ID))
		b := sim.NewWorldWith(cfg, 3, Character(cfg, ch.ID))
		if fmt.Sprintf("%#v", a.Crew.Members) != fmt.Sprintf("%#v", b.Crew.Members) || a.Player.Location != b.Player.Location {
			t.Errorf("%s: two worlds on one seed differ", ch.ID)
		}
		if got := a.Start.Character; (ch.ID == cfg.Characters.Default().ID) != (got == "") {
			t.Errorf("%s: stamped as %q", ch.ID, got)
		}
		s := ch.Start
		if len(a.Crew.Members) != len(s.Crew) {
			t.Errorf("%s: %d on the payroll, want %d", ch.ID, len(a.Crew.Members), len(s.Crew))
		}
		for i, role := range s.Crew {
			if m := a.Crew.Members[i]; m.Role != role || m.Hired != 0 || m.Name == "" || m.Name == "Nobody" {
				t.Errorf("%s: member %d is %+v, want a %s hired on day 0", ch.ID, i, m, role)
			}
		}
		for _, id := range s.Upgrades {
			if !a.Owns(id) {
				t.Errorf("%s: does not own %s", ch.ID, id)
			}
		}
		for _, id := range s.Products {
			for _, cid := range a.CityOrder {
				if a.Product(cid, id) == nil {
					t.Errorf("%s: %s is not listed in %s", ch.ID, id, cid)
				}
			}
		}
		if s.City != "" {
			c := a.PostOf(game.You)
			if a.Player.Location != s.City || c == nil || c.ID != s.Corner || a.Held() != 1 {
				t.Errorf("%s: stands in %s on %v holding %d", ch.ID, a.Player.Location, c, a.Held())
			}
		}
		if r := a.Player.Reputation; r.Fear != s.Reputation.Fear || r.Respect != s.Reputation.Respect || r.Notoriety != s.Reputation.Notoriety {
			t.Errorf("%s: reputation %+v, want %+v", ch.ID, r, s.Reputation)
		}
		if known := game.Known(a).Chief(); s.KnowChief && known != a.Law.Chief.Personality || !s.KnowChief && known != game.Unknown {
			t.Errorf("%s: the chief reads %q, the file says know_chief %v", ch.ID, known, s.KnowChief)
		}
		if len(s.Crew) > 0 {
			c := sim.NewWorldWith(cfg, 4, Character(cfg, ch.ID))
			if fmt.Sprintf("%#v", a.Crew.Members) == fmt.Sprintf("%#v", c.Crew.Members) {
				t.Errorf("%s: the same crew on two seeds", ch.ID)
			}
		}
		// The pool and the law are the seed's whatever the character.
		d := sim.NewWorld(cfg, 3)
		if fmt.Sprintf("%#v", a.Crew.Candidates) != fmt.Sprintf("%#v", d.Crew.Candidates) || a.Law.Chief != d.Law.Chief || a.Law.DA != d.Law.DA || a.Rival().Leader != d.Rival().Leader {
			t.Errorf("%s: the pool, the law or the rival moved off the seed's", ch.ID)
		}
	}
}

// TestHardDASeatsTheLaw (#50): the toggle starts the run with a
// law-and-order DA and a zealous chief with the seed's names, and
// nothing else moves on day 0; the elections still run (the stance is
// set, never pinned as harness.Appoint pins it).
func TestHardDASeatsTheLaw(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 5; seed++ {
		w := sim.NewWorldWith(cfg, seed, game.Start{HardDA: true})
		d := sim.NewWorld(cfg, seed)
		if w.Law.DA.Stance != "law_and_order" || w.Law.Chief.Personality != "zealous" || !w.Start.HardDA {
			t.Fatalf("seed %d: the law is %+v / %+v", seed, w.Law.DA, w.Law.Chief)
		}
		if w.Law.DA.Name != d.Law.DA.Name || w.Law.Chief.Name != d.Law.Chief.Name {
			t.Fatalf("seed %d: the names moved", seed)
		}
		w.Law, w.Start = d.Law, d.Start
		if digest(w) != digest(d) {
			t.Fatalf("seed %d: the hard DA moved more than the law", seed)
		}
	}
	w := sim.NewWorldWith(cfg, 1, game.Start{HardDA: true})
	res, _ := RunFrom(cfg, w, cfg.Law.Law.TermDays+2, Managed(cfg, 50))
	elected := false
	for _, e := range res.Events {
		if _, ok := e.(events.DAElected); ok {
			elected = true
		}
	}
	if !elected {
		t.Fatal("no election under the hard DA: the stance was pinned")
	}
}

// TestDailySeedIsTheDate (#50): the daily's seed is a function of the
// UTC date alone, pinned here so two processes on two machines agree
// on it, the same at any hour of the date and different the next day;
// its key is the date as YYYYMMDD.
func TestDailySeedIsTheDate(t *testing.T) {
	t.Parallel()
	morning := time.Date(2026, 9, 13, 0, 0, 1, 0, time.UTC)
	night := time.Date(2026, 9, 13, 23, 59, 59, 0, time.UTC)
	late := time.Date(2026, 9, 13, 20, 0, 0, 0, time.FixedZone("west", -8*3600)) // 04:00 on the 14th UTC
	if game.DailyKey(morning) != "20260913" || game.DailyKey(night) != "20260913" || game.DailyKey(late) != "20260914" {
		t.Fatalf("keys %s %s %s", game.DailyKey(morning), game.DailyKey(night), game.DailyKey(late))
	}
	const pinned = 0x8a76da7003db4145
	if got := game.DailySeed(morning); got != game.DailySeed(night) || got == game.DailySeed(late) {
		t.Fatalf("seeds %d %d %d", got, game.DailySeed(night), game.DailySeed(late))
	}
	if got := game.DailySeed(morning); got != pinned {
		t.Fatalf("2026-09-13 seeds %#x, pinned %#x: the fold moved, and every daily with it", got, uint64(pinned))
	}
	a, _ := Run(content.MustLoad(), game.DailySeed(morning), 10, Idle)
	b, _ := Run(content.MustLoad(), game.DailySeed(night), 10, Idle)
	if fmt.Sprintf("%#v", a.Events) != fmt.Sprintf("%#v", b.Events) {
		t.Fatal("two dailies on one date diverged")
	}
}

// TestHeirStartsPosted (#232): the heir's day 0 has the old man's
// corner held with the old man's enforcer guarding it, beside the
// corner you stand on; the stash owned; the reputation as the file
// says; and nothing else of the start on the world
// (TestCharacterIsDayZero has the rest). The corners key posts the
// start crew in order, so a two-corner start with a runner would work
// the first and guard the second; the heir's shrank to one corner and
// no runner (the file says why).
func TestHeirStartsPosted(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	ch := cfg.Characters.Character("heir")
	if ch == nil {
		t.Fatal("no heir in the file")
	}
	w := sim.NewWorldWith(cfg, 2, Character(cfg, "heir"))
	if w.Day != 0 || len(ch.Start.Corners) != 1 || len(w.Crew.Members) != 1 {
		t.Fatalf("day %d, %d corners, %d on the payroll", w.Day, len(ch.Start.Corners), len(w.Crew.Members))
	}
	enforcer := w.Crew.Members[0]
	if enforcer.Role != "enforcer" {
		t.Fatalf("the crew: %s", enforcer.Role)
	}
	corner := w.Corner(ch.Start.Corners[0])
	if !corner.Held() || corner.Enforcer != enforcer.ID || corner.Runner != 0 || corner.Since != 0 {
		t.Fatalf("the old man's corner: %+v", corner)
	}
	if post := w.PostOf(enforcer.ID); post == nil || post.ID != corner.ID {
		t.Fatalf("the post: %v", post)
	}
	if you := w.PostOf(game.You); you == nil || you.ID != cfg.City.Territory.Start || w.HeldIn(w.Home().ID) != 2 {
		t.Fatalf("you stand on %v with %d held at home", you, w.HeldIn(w.Home().ID))
	}
	if !w.Owns("stash") || w.Player.Reputation.Fear != 25 || w.Player.Reputation.Notoriety != 15 || w.Player.Reputation.Respect != 0 {
		t.Fatalf("the rest of the start: owns %v, reputation %+v", w.Upgrades, w.Player.Reputation)
	}
	// The unlock is the kingpin ending, the crown's (#227).
	if ch.Unlock.Ending != content.CauseKingpin || !game.Met(ch.Unlock, cfg.Progression, content.CauseKingpin, "") {
		t.Fatalf("the unlock: %+v", ch.Unlock)
	}
}
