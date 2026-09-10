// Package harness runs the game headless under a scripted policy so balance
// can be measured and invariants tested without a terminal.
package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// Policy decides the player's actions for the coming day.
type Policy func(w *game.World)

// Result summarises a headless run.
type Result struct {
	Days     int
	Over     *game.Ending
	PeakCash int
	EndCash  int
	NetWorth []int // net worth at the end of each day played, oldest first
	Events   []events.Event
	World    *game.World
}

// NetWorthAt is the net worth at the end of day d, or at the end of the run
// if it finished sooner.
func (r Result) NetWorthAt(d int) int {
	if len(r.NetWorth) == 0 {
		return 0
	}
	return r.NetWorth[min(d, len(r.NetWorth))-1]
}

// Run plays up to days days from a fresh world with the given seed.
func Run(cfg *content.Config, seed uint64, days int, policy Policy) (Result, error) {
	return RunFrom(cfg, sim.NewWorld(cfg, seed), days, policy)
}

// RunFrom plays up to days days on from w, which the caller may have set
// up (a cash pile, a crew) to test a situation a fresh run takes a while
// to reach.
func RunFrom(cfg *content.Config, w *game.World, days int, policy Policy) (Result, error) {
	_, sims, err := sim.Default(cfg)
	if err != nil {
		return Result{}, err
	}
	clock := game.NewClock(nil, sims...)
	var all []events.Event
	var worth []int
	for d := 0; d < days && w.Over == nil; d++ {
		if policy != nil {
			policy(w)
		}
		all = append(all, clock.EndDay(w)...)
		worth = append(worth, w.NetWorth())
	}
	return Result{Days: w.Day, Over: w.Over, PeakCash: w.Stats.PeakCash, EndCash: w.Cash(), NetWorth: worth, Events: all, World: w}, nil
}

// Idle does nothing; prices drift on their own.
func Idle(*game.World) {}

// Hide lies low every day and never trades: the player who has built
// something and sits on it.
func Hide(w *game.World) { w.SetLieLow(true) }

// Trader restocks every product it can afford and sells everything it holds
// at the given dial, every day. It is deliberately greedy. It buys before it
// queues the sales so stock bought in the morning is on the street the same
// night, the way a player who turns the bag over daily plays.
func Trader(cfg *content.Config, dial events.Dial) Policy {
	pressure := cfg.Market.Market.BuyPricePressure
	return func(w *game.World) {
		// Pushed off a corner, stand on the biggest free one: nothing sells
		// from nowhere.
		if w.PostOf(game.You) == nil {
			if c := pickCorner(w, func(c game.Corner) bool { return c.Held() && c.Runner == 0 }, func(c game.Corner) float64 { return c.Demand }); c != nil {
				_ = w.Post(c.ID, game.You)
			} else if c := pickCorner(w, func(c game.Corner) bool { return c.Owner == game.OwnerNone }, func(c game.Corner) float64 { return c.Demand }); c != nil {
				_ = w.Post(c.ID, game.You)
			}
		}
		// Restock toward a demand-proportional mix that fits what the
		// operation can hold, so a crashed product never hogs the whole bag.
		total := 0.0
		for _, id := range w.Products {
			total += w.Market[id].Demand
		}
		for _, id := range w.Products {
			m := w.Market[id]
			target := int(float64(w.Capacity()) * m.Demand / total)
			room := w.Capacity() - w.Player.TotalStock()
			afford := int(float64(w.Player.DirtyCash) / m.SupplierPrice)
			qty := min(target-w.Player.Stock[id], afford, room)
			if qty > 0 {
				_, _ = w.Buy(id, qty, pressure)
			}
		}
		// Sell what we hold.
		for _, id := range w.Products {
			if q := w.Player.Stock[id]; q > 0 {
				_ = w.PlaceSell(id, q, dial)
			}
		}
	}
}

// Careful trades quietly and lies low whenever heat climbs.
func Careful(cfg *content.Config, lieLowAt float64) Policy {
	trade := Trader(cfg, events.DialQuiet)
	return func(w *game.World) {
		if w.Heat.Value >= lieLowAt {
			w.SetLieLow(true)
			return
		}
		trade(w)
	}
}

// Managed sells at the normal dial and lies low whenever heat reaches
// lieLowAt. It is the baseline for "a player who pays attention".
func Managed(cfg *content.Config, lieLowAt float64) Policy {
	trade := Trader(cfg, events.DialNormal)
	return func(w *game.World) {
		if w.Heat.Value >= lieLowAt {
			w.SetLieLow(true)
			return
		}
		trade(w)
	}
}

// Crewed plays like Managed but builds a crew: it pays fair, signs the most
// skilled runner on offer whenever it can afford the fee with cash to
// spare, posts every runner on the best free corner, and replaces anyone
// whose loyalty has sunk to where they skim.
func Crewed(cfg *content.Config, lieLowAt float64) Policy {
	return Territory(cfg, lieLowAt, 0)
}

// Territory plays like Crewed but works at most corners corners (0 means
// as many as it can staff), counting the one you stand on, and spends the
// crew slots it has left on enforcers for the corners most likely to be
// robbed. Once the rival is in town it keeps a couple of enforcers on the
// corners it borders, whatever else it is doing. It is the baseline for
// "a player who takes ground"; it never sends them in.
func Territory(cfg *content.Config, lieLowAt float64, corners int) Policy {
	managed := Managed(cfg, lieLowAt)
	tun := cfg.Crew.Crew
	if corners <= 0 {
		corners = len(cfg.City.Corners)
	}
	guards := max(1, tun.MaxCrew/3)
	return func(w *game.World) {
		w.SetPay(events.PayFair)
		for _, m := range w.Crew.Members {
			if m.Loyalty < tun.SkimThreshold {
				_, _ = w.Fire(m.ID)
				break // one a day; each firing sours the rest
			}
		}
		// Runners until the corners are staffed, then enforcers for them;
		// with a rival about, enforcers come first once one runner is on.
		want := "runner"
		if w.Crew.Runners()+1 >= corners {
			want = "enforcer"
		}
		if n := w.Crew.Role("enforcer"); want == "enforcer" && n >= min(corners, w.Worked()) {
			want = ""
		}
		if w.Rival.Arrived > 0 && w.Crew.Runners() >= 1 && w.Crew.Role("enforcer") < guards {
			want = "enforcer"
			// A full roster of runners makes room: the least skilled goes.
			if len(w.Crew.Members) >= tun.MaxCrew && len(w.Crew.FiredToday) == 0 {
				worst := -1
				for i, m := range w.Crew.Members {
					if m.Role == "runner" && (worst < 0 || m.Skill < w.Crew.Members[worst].Skill) {
						worst = i
					}
				}
				if worst >= 0 {
					_, _ = w.Fire(w.Crew.Members[worst].ID)
				}
			}
		}
		best := -1
		for i, c := range w.Crew.Candidates {
			if c.Role != want {
				continue
			}
			if best < 0 || c.Skill > w.Crew.Candidates[best].Skill {
				best = i
			}
		}
		if best >= 0 && len(w.Crew.Members) < tun.MaxCrew {
			c := w.Crew.Candidates[best]
			if w.Player.DirtyCash >= c.Fee+cfg.Market.Market.StartCash {
				_, _ = w.Hire(c.ID, tun.MaxCrew)
			}
		}
		// Every idle runner takes back a held corner nobody is working,
		// else the biggest free one, up to the cap; every idle enforcer
		// guards the riskiest unguarded one.
		for _, m := range w.Crew.Members {
			if w.PostOf(m.ID) != nil {
				continue
			}
			switch m.Role {
			case "runner":
				if w.Worked() >= corners {
					continue
				}
				c := pickCorner(w, func(c game.Corner) bool { return c.Held() && c.Runner == 0 }, func(c game.Corner) float64 { return c.Demand })
				if c == nil {
					c = pickCorner(w, func(c game.Corner) bool { return !c.Held() }, func(c game.Corner) float64 { return c.Demand })
				}
				if c != nil {
					_ = w.Post(c.ID, m.ID)
				}
			case "enforcer":
				// The corners the rival borders first, then the riskiest.
				score := func(c game.Corner) float64 {
					if w.Contested(c) {
						return 10 + c.Demand
					}
					return c.Risk
				}
				if c := pickCorner(w, func(c game.Corner) bool { return c.Worked() && c.Enforcer == 0 }, score); c != nil {
					_ = w.Post(c.ID, m.ID)
				}
			}
		}
		managed(w)
	}
}

// Warlike plays like Territory but fights the rival: whenever it has an
// enforcer and heat is under lieLowAt it sends the enforcers against the
// rival's biggest corner at force, every day, and re-posts a runner on
// whatever it wins. It is the baseline for "a player who goes to war".
func Warlike(cfg *content.Config, lieLowAt float64, corners int, force events.Force) Policy {
	territory := Territory(cfg, lieLowAt, corners)
	return func(w *game.World) {
		territory(w)
		if w.Heat.Value >= lieLowAt || w.Crew.Role("enforcer") == 0 {
			return
		}
		if c := pickCorner(w, func(c game.Corner) bool { return c.Owner == game.OwnerRival }, func(c game.Corner) float64 { return c.Demand }); c != nil {
			_ = w.SendEnforcers(c.ID, force)
		}
	}
}

// pickCorner returns the corner passing ok with the highest score, or nil.
func pickCorner(w *game.World, ok func(game.Corner) bool, score func(game.Corner) float64) *game.Corner {
	var best *game.Corner
	for i := range w.Territory.Corners {
		c := &w.Territory.Corners[i]
		if ok(*c) && (best == nil || score(*c) > score(*best)) {
			best = c
		}
	}
	return best
}

// Horizon is how many days the harness plays a run for when it measures
// something. It is a ruler, not a run length: the game has no day cap and a
// run ends only through an ending, so a policy that is "still free at the
// horizon" is one the game never punished for playing on.
const Horizon = 200

// TierDays are the checkpoints the money curve is read at (#24): the day
// each progression tier is expected to have paid off by. Like Horizon they
// are where the harness looks, not where the game stops.
var TierDays = []int{30, 70, 120, Horizon}
