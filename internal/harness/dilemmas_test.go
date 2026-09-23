package harness

import (
	"reflect"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// drawn lists the cards a run dealt, as "day:card".
func drawn(res Result) []string {
	var out []string
	for _, e := range res.Events {
		if ev, ok := e.(events.DilemmaDrawn); ok {
			out = append(out, string(rune('0'+ev.Day/100))+string(rune('0'+ev.Day/10%10))+string(rune('0'+ev.Day%10))+":"+ev.Card)
		}
	}
	return out
}

// Two runs with the same seed and the same choices see the same cards on
// the same days, whatever the choices are; different choices may not.
func TestCardsAreDeterministic(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	for _, pick := range []struct {
		name string
		c    Chooser
	}{{"decline", Decline}, {"first", First}} {
		for seed := uint64(1); seed <= 3; seed++ {
			a, err := RunWith(cfg, sim.NewWorld(cfg, seed), 120, Crewed(cfg, 40), pick.c)
			if err != nil {
				t.Fatal(err)
			}
			b, err := RunWith(cfg, sim.NewWorld(cfg, seed), 120, Crewed(cfg, 40), pick.c)
			if err != nil {
				t.Fatal(err)
			}
			if da, db := drawn(a), drawn(b); !reflect.DeepEqual(da, db) {
				t.Fatalf("seed %d %s: cards differ between two identical runs:\n%v\n%v", seed, pick.name, da, db)
			}
			if len(a.Events) != len(b.Events) || a.EndCash != b.EndCash {
				t.Fatalf("seed %d %s: runs differ: %d vs %d events, $%d vs $%d", seed, pick.name, len(a.Events), len(b.Events), a.EndCash, b.EndCash)
			}
			if len(drawn(a)) < 120/(cfg.Dilemmas.Dilemmas.MaxGap+2) {
				t.Fatalf("seed %d %s: only %d cards in 120 days", seed, pick.name, len(drawn(a)))
			}
		}
	}
}

// Cards are part of saved state: a run stopped on a card and continued
// plays out exactly like one that never stopped.
func TestCardSurvivesSave(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	_, sims, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	policy := Crewed(cfg, 40)
	play := func(w *game.World, days int, log *[]string) {
		clock := game.NewClock(nil, sims...)
		for d := 0; d < days; d++ {
			if c := w.Dilemmas.Pending; c != nil {
				*log = append(*log, c.ID+"/"+c.Choices[0].Label)
				_, _ = w.Choose(0)
			}
			policy(w)
			clock.EndDay(w)
		}
	}
	var straight, resumed []string
	a := sim.NewWorld(cfg, 11)
	play(a, 60, &straight)

	b := sim.NewWorld(cfg, 11)
	// Stop on the first morning a card is waiting, save, load, go on.
	clock := game.NewClock(nil, sims...)
	for b.Dilemmas.Pending == nil {
		policy(b)
		clock.EndDay(b)
	}
	if err := game.Save(1, b); err != nil {
		t.Fatal(err)
	}
	b2, err := game.Load(1)
	if err != nil {
		t.Fatal(err)
	}
	if b2.Dilemmas.Pending == nil || b2.Dilemmas.Pending.ID != b.Dilemmas.Pending.ID || b2.Dilemmas.Pending.Text != b.Dilemmas.Pending.Text {
		t.Fatalf("the card did not come back: %+v", b2.Dilemmas.Pending)
	}
	play(b2, 60-b2.Day, &resumed)
	if !reflect.DeepEqual(straight, resumed) {
		t.Fatalf("cards differ after a save on a card:\n%v\n%v", straight, resumed)
	}
	if a.Cash() != b2.Cash() || a.Day != b2.Day {
		t.Fatalf("worlds differ after a save on a card: day %d $%d vs day %d $%d", a.Day, a.Cash(), b2.Day, b2.Cash())
	}
}

// Every card in the deck comes up somewhere across the personalities and a
// few seeds, and none of them renders with a hole in it.
func TestEveryCardIsDealtAndReadsClean(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	seen := map[string]int{}
	check := func(t *testing.T, w *game.World) {
		c := w.Dilemmas.Pending
		if c == nil {
			return
		}
		seen[c.ID]++
		texts := []string{c.Title, c.Text}
		for _, ch := range c.Choices {
			texts = append(texts, ch.Label, ch.Outcome, ch.Headline)
		}
		for _, s := range texts {
			if strings.Contains(s, "<no value>") || strings.Contains(s, "{{") || strings.Contains(s, "$0") {
				t.Errorf("card %s on day %d renders %q", c.ID, c.Day, s)
			}
		}
	}
	// The rich laundered player for the money cards, the warlike one from
	// the usual stake for the cards that need a war on (rich, it routs the
	// rival before one starts).
	for _, p := range content.Personalities {
		for seed := uint64(1); seed <= 4; seed++ {
			for i, policy := range []Policy{Laundered(cfg, 40), Warlike(cfg, 40, 3, events.ForceHit)} {
				w := sim.NewWorld(cfg, seed)
				w.Rival().Personality = p
				if i == 0 {
					w.Player.DirtyCash = 30_000
				}
				pick := func(w *game.World, c *game.Card) int {
					check(t, w)
					return int(seed) % len(c.Choices)
				}
				if _, err := RunWith(cfg, w, Horizon, policy, pick); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	var missing []string
	for _, c := range cfg.Dilemmas.Cards {
		if seen[c.ID] == 0 {
			missing = append(missing, c.ID)
		}
	}
	if len(missing) > 0 {
		t.Errorf("cards never dealt across 32 runs: %v (seen %v)", missing, seen)
	}
}

// TestPreviewIsTheOutcome (#358): for every card in the deck, on worlds
// a few runs reached and on the same worlds broke and hot (the clamps),
// Preview(i) is what Choose(i) moves, gauge for gauge, and asking never
// writes the world (TestPreviewNeverWritesTheWorld's half, here where
// the worlds are).
func TestPreviewIsTheOutcome(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	var worlds []*game.World
	for seed := uint64(1); seed <= 2; seed++ {
		for i, policy := range []Policy{Laundered(cfg, 40), Warlike(cfg, 40, 3, events.ForceHit)} {
			w := sim.NewWorld(cfg, seed)
			if i == 0 {
				w.Player.DirtyCash = 30_000
			}
			res, err := RunFrom(cfg, w, 40, policy)
			if err != nil {
				t.Fatal(err)
			}
			worlds = append(worlds, res.World)
		}
	}
	for _, w := range worlds[:2] {
		broke, _ := game.Decode(mustEncode(t, w))
		broke.Player.DirtyCash, broke.Player.CleanCash = 0, 0
		hot, _ := game.Decode(mustEncode(t, w))
		hot.Here().Heat = 97
		for i := range hot.Crew.Members {
			hot.Crew.Members[i].Loyalty = 98
		}
		worlds = append(worlds, broke, hot)
	}
	// The rich band's (#342): a bag the capped sums stop short of.
	rich, _ := game.Decode(mustEncode(t, worlds[1]))
	rich.Player.DirtyCash, rich.Stats.PeakCash = 5_000_000, 5_000_000
	worlds = append(worlds, rich)
	moved := 0
	keys := map[string]bool{}
	for n, w := range worlds {
		member := 0
		if len(w.Crew.Members) > 0 {
			member = w.Crew.Members[0].ID
		}
		corner := ""
		for _, c := range w.Corners() {
			if c.Owner == game.OwnerPlayer {
				corner = c.ID
				break
			}
		}
		for _, cc := range cfg.Dilemmas.Cards {
			card := func() *game.Card {
				c := &game.Card{ID: cc.ID, Member: member, Corner: corner, Amount: game.CardSum(cc, w.Player.DirtyCash)}
				for _, ch := range cc.Choices {
					c.Choices = append(c.Choices, game.Choice{Label: ch.Label, Outcome: ch.Outcome, Effects: ch.Effects})
				}
				return c
			}
			for i := range cc.Choices {
				cp, err := game.Decode(mustEncode(t, w))
				if err != nil {
					t.Fatal(err)
				}
				cp.Over = nil
				before := digest(cp)
				got, err := card().Preview(cp, i)
				if err != nil {
					t.Fatalf("world %d, %s choice %d: %v", n, cc.ID, i, err)
				}
				if digest(cp) != before {
					t.Fatalf("world %d, %s choice %d: Preview wrote the world", n, cc.ID, i)
				}
				cp.Dilemmas.Pending = card()
				reading := game.Reading(cp)
				if _, err := cp.Choose(i); err != nil {
					t.Fatalf("world %d, %s choice %d: %v", n, cc.ID, i, err)
				}
				if want := game.Moved(reading, cp); !reflect.DeepEqual(got, want) {
					t.Errorf("world %d, %s choice %d: preview %+v, Choose moved %+v", n, cc.ID, i, got, want)
				}
				if len(got) > 0 {
					moved++
				}
				for _, ch := range got {
					keys[ch.Key] = true
				}
			}
		}
	}
	if moved == 0 {
		t.Error("no preview moved anything")
	}
	// The rich band's keys (#342) are in the preview too: a corner given
	// up and a favour owed.
	for _, k := range []string{"corners", "owes"} {
		if !keys[k] {
			t.Errorf("no preview moved %s", k)
		}
	}
}

// TestPreviewNeverWritesTheWorld (#358): asking what a choice does
// leaves the world as it was, every choice of a card that moves every
// gauge, on a world with a crew, a rival and stock.
func TestPreviewNeverWritesTheWorld(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, 3)
	w.Player.DirtyCash = 30_000
	res, err := RunFrom(cfg, w, 30, Laundered(cfg, 40))
	if err != nil {
		t.Fatal(err)
	}
	w = res.World
	all := map[string]float64{}
	for _, k := range game.EffectKeys {
		all[k] = -3
	}
	c := &game.Card{ID: "all", Amount: 500, Choices: []game.Choice{{Effects: all}, {Effects: map[string]float64{"stock_share": 0.5, "heat": 40, "crew_loyalty": 10}}, {}}}
	if len(w.Crew.Members) > 0 {
		c.Member = w.Crew.Members[0].ID
	}
	before := digest(w)
	for i := range c.Choices {
		if _, err := c.Preview(w, i); err != nil {
			t.Fatal(err)
		}
	}
	if digest(w) != before {
		t.Fatal("Preview wrote the world")
	}
	if _, err := c.Preview(w, 3); err != game.ErrBadChoice {
		t.Errorf("choice 3 of 3: %v", err)
	}
}

func mustEncode(t *testing.T, w *game.World) []byte {
	t.Helper()
	b, err := game.Encode(w)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
