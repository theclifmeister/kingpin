// Package law simulates the law's actors (#41): a police chief with a
// personality, a district attorney with an election cycle, and the public
// pressure in every city that decides who gets elected. It owns
// World.Law, City.Pressure and City.Goodwill; the heat sim and the rival
// sim read them off the world through their own tuning, never through
// this package. It steps after heat, so what heat read this morning was
// yesterday's pressure, and before laundering. Nothing here touches the
// DA's file: #27 holds, the law changes thresholds, cooldowns and decay.
// Its dice (an election, a new chief) roll on their own side stream,
// Tick.Sub("law"), so the home stream, and every seed-pinned number that
// reads it, is what it was before the law had actors.
package law

import (
	"math"
	"slices"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the law simulation.
type Sim struct {
	cfg    content.LawConfig
	chiefs []string
	das    []string
}

// New builds a law sim from the config, copying what it reads (#144):
// its own law.toml and the chiefs' and DAs' name pools.
func New(cfg *content.Config) *Sim {
	return &Sim{cfg: cfg.Law, chiefs: cfg.Names.Chiefs, das: cfg.Names.DAs}
}

func (s *Sim) Name() string { return "law" }

// Tuning exposes the constants the UI needs to explain itself.
func (s *Sim) Tuning() content.LawTuning { return s.cfg.Law }

// Campaign exposes the campaign tuning (#193): what a city's vote costs
// to move, and how far money moves it.
func (s *Sim) Campaign() content.CampaignTuning { return s.cfg.Campaign }

// rand is the subset of *math/rand/v2.Rand the sim uses.
type rand interface {
	IntN(int) int
	Float64() float64
}

// Seed picks the run's chief and DA from rng so they are part of the
// seed like the rival: a name and a personality, a name and a ticket.
// Both take office on the current day.
func (s *Sim) Seed(w *game.World, rng rand) {
	w.Law.Chief = game.Chief{Name: s.pick(s.chiefs, rng, "Nobody"), Personality: content.ChiefPersonalities[rng.IntN(len(content.ChiefPersonalities))], Since: w.Day}
	w.Law.DA = game.DA{Name: s.pick(s.das, rng, "Nobody"), Stance: content.DAStances[rng.IntN(len(content.DAStances))], ElectedDay: w.Day}
}

// Migrate is the 8 -> 9 step: a save from before the law had actors gets
// a chief and a DA drawn from the current day's RNG, in office from
// today. Pressure and goodwill start at zero, the pre-#41 state.
func (s *Sim) Migrate(w *game.World) {
	if w.Law.Chief.Name == "" || w.Law.DA.Name == "" {
		s.Seed(w, game.RNGFor(w.Seed, w.Day))
	}
}

func (s *Sim) pick(pool []string, rng rand, fallback string) string {
	if len(pool) == 0 {
		return fallback
	}
	return pool[rng.IntN(len(pool))]
}

// Goodwill is what amount in clean cash buys a city, in points.
func (s *Sim) Goodwill(amount int) float64 {
	if s.cfg.Law.GoodwillCash <= 0 {
		return 0
	}
	return float64(amount) / float64(s.cfg.Law.GoodwillCash)
}

// NextElection is the day the DA next faces the voters, or 0 if never.
func (s *Sim) NextElection(w *game.World) int {
	if s.cfg.Law.TermDays <= 0 {
		return 0
	}
	return w.Law.DA.ElectedDay + s.cfg.Law.TermDays
}

// ChiefTermEnds is the day the chief's term is up, or 0 if they serve
// for life.
func (s *Sim) ChiefTermEnds(w *game.World) int {
	if s.cfg.Law.ChiefTerm <= 0 {
		return 0
	}
	return w.Law.Chief.Since + s.cfg.Law.ChiefTerm
}

// LawAndOrderShare is the share of the vote a law-and-order candidate
// takes at a mean pressure, before the moderate takes theirs: even at 50,
// everything at 100 by the swing, nothing at 0. The dashboard shows it
// and the dice use it.
func (s *Sim) LawAndOrderShare(pressure float64) float64 {
	return clamp01(0.5 + (pressure-50)/100*s.cfg.Law.ElectionSwing)
}

// Swing is what the cities' campaigns move the law-and-order share by
// at the next election (#193): each city's money buys a point of its
// vote per campaign.cash up to swing_max, toward its ticket, a hedged
// campaign nothing, and the election reads the mean over the cities as
// it reads the mean pressure. Positive is toward law and order.
func (s *Sim) Swing(w *game.World) float64 {
	if len(w.CityOrder) == 0 {
		return 0
	}
	sum := 0.0
	for _, cid := range w.CityOrder {
		camp := w.Cities[cid].Campaign
		if camp.Hedged || camp.Cash <= 0 {
			continue
		}
		v := s.cfg.Campaign.Swing(camp.Cash)
		if camp.Ticket == "law_and_order" {
			sum += v
		} else {
			sum -= v
		}
	}
	return sum / float64(len(w.CityOrder))
}

// CampaignOpen says whether the next election is within open_days of
// day, so the tickets take money (#193). It is what the sim stamps on
// w.Law.CampaignOpen for the day to come: open while the dashboard's
// countdown reads open_days or less, shut once the vote is counted.
func (s *Sim) CampaignOpen(w *game.World, day int) bool {
	next := s.NextElection(w)
	return next > 0 && day < next && next-day <= s.cfg.Campaign.OpenDays
}

// Step turns today's funding into goodwill, moves every city's pressure
// from the day's violence, hard product and headlines, lets the chief's
// term run out, and holds the election when it is due.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Law
	src := s.cfg.Pressure
	home := w.Home().ID
	here := w.Player.Location

	// Funding: clean cash given today buys goodwill where it was given.
	for _, f := range w.Today.Funded {
		c := w.Cities[f.City]
		if c == nil {
			continue
		}
		g := s.Goodwill(f.Amount)
		c.Goodwill = math.Min(100, c.Goodwill+g)
		t.Emit(events.CityFunded{Day: t.Day, City: f.City, Amount: f.Amount, Goodwill: g})
	}

	// Backing (#193): clean cash put behind a ticket today joins the
	// city's campaign. Money on the other ticket hedges it: the campaign
	// keeps counting the cash and buys nothing at the count.
	for _, b := range w.Today.Backed {
		c := w.Cities[b.City]
		if c == nil {
			continue
		}
		camp := &c.Campaign
		if camp.Cash == 0 {
			w.Stats.Campaigns++
		}
		if camp.Ticket != "" && camp.Ticket != b.Ticket {
			camp.Hedged = true
		}
		if camp.Ticket == "" {
			camp.Ticket = b.Ticket
		}
		camp.Cash += b.Amount
		t.Emit(events.CampaignBacked{Day: t.Day, City: b.City, Ticket: b.Ticket, Amount: b.Amount, Total: camp.Cash, Swing: s.cfg.Campaign.Swing(camp.Cash)})
	}

	// Sources. Violence is the home city's; hard product counts where it
	// sold, on a corner or handed to a buyer; a headline about you counts
	// where you are, as notoriety does.
	gain := map[string]float64{}
	units := map[string]int{}
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CornerStruck:
			gain[home] += src.Strike
		case events.RivalBoosted:
			gain[home] += src.Boost
		case events.RivalRaided:
			gain[home] += src.RivalRaid
		case events.RivalPushed:
			gain[home] += src.Push
		case events.CornerTaken:
			if ev.From == game.OwnerPlayer {
				gain[home] += src.Push
			}
		case events.WarEscalated:
			if ev.Stage == "crackdown" {
				gain[home] += src.Crackdown
			}
		case events.PlayerSold:
			if slices.Contains(src.Hard, ev.Product) {
				units[ev.City] += ev.Sold
			}
		case events.ContractDelivered:
			// A buyer's handoff is product sold in that city (#71): it
			// counts against hard_units the way a corner sale does.
			if slices.Contains(src.Hard, ev.Product) {
				units[ev.City] += ev.Units
			}
		}
	}
	if src.HardUnits > 0 {
		for cid, n := range units {
			gain[cid] += float64(n) / src.HardUnits
		}
	}
	for i := len(w.Journal) - 1; i >= 0 && w.Journal[i].Day == t.Day-1; i-- {
		if slices.Contains(src.Sources, w.Journal[i].Source) {
			gain[here] += src.Headline
		}
	}
	// A campaign is public money (#193): the city it runs in talks.
	for _, cid := range w.CityOrder {
		if w.Cities[cid].Campaign.Cash > 0 {
			gain[cid] += s.cfg.Campaign.Pressure
		}
	}

	// Fade toward the baseline, goodwill takes its cut and fades itself,
	// and a band crossed is news.
	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		from := c.Pressure
		p := c.Pressure + gain[cid]
		p -= (p - tun.Baseline) * tun.Decay
		p -= tun.GoodwillCut * clamp01(c.Goodwill/100)
		c.Pressure = math.Max(0, math.Min(100, p))
		c.Goodwill = math.Max(0, math.Min(100, c.Goodwill-c.Goodwill*tun.GoodwillDecay))
		if band(from, tun.Band) != band(c.Pressure, tun.Band) {
			t.Emit(events.PressureShifted{Day: t.Day, City: cid, From: from, To: c.Pressure})
		}
	}

	// The chief: you learn what they are like after a while in office, or
	// the first time their people come through the door.
	chief := &w.Law.Chief
	if !chief.Observed {
		if t.Day-chief.Since >= tun.ObserveDays {
			chief.Observed = true
		}
		for _, e := range t.Events() {
			if ev, ok := e.(events.Enforcement); ok && ev.Level != content.Arrest {
				chief.Observed = true
			}
		}
	}
	replaced := false
	if end := s.ChiefTermEnds(w); end > 0 && t.Day >= end {
		s.replaceChief(w, t, "term", "")
		replaced = true
	}

	// The election: the cities' mean pressure swings the vote, the
	// campaigns move it by what they bought (#193, before the one draw,
	// so a run with no money on the table rolls the same dice), a
	// moderate takes their share whatever the mood, and one draw
	// decides it. A winner on the sitting DA's ticket is the sitting DA
	// re-elected. A law-and-order DA elected on a loud enough city wants
	// a new chief.
	if next := s.NextElection(w); next > 0 && t.Day >= next {
		mean := w.MeanPressure()
		swing := s.Swing(w)
		share := clamp01(s.LawAndOrderShare(mean) + swing)
		law := share * (1 - tun.Moderate)
		reform := (1 - share) * (1 - tun.Moderate)
		rng := t.Sub(s.Name())
		r := rng.Float64()
		stance := "moderate"
		switch {
		case r < law:
			stance = "law_and_order"
		case r < law+reform:
			stance = "reform"
		}
		da := &w.Law.DA
		incumbent := stance == da.Stance
		if !incumbent {
			da.Name = s.pick(without(s.das, da.Name), rng, da.Name)
			da.Stance = stance
		}
		da.ElectedDay = t.Day
		// The campaigns are spent (#193): a city whose ticket won has a
		// DA who owes you; one whose ticket lost has a DA who knows who
		// paid for the other side, and under a law-and-order winner a
		// zealous chief at once; one that paid both sides has a headline.
		backed := false
		for _, cid := range w.CityOrder {
			c := w.Cities[cid]
			camp := c.Campaign
			c.Campaign = game.Campaign{}
			switch {
			case camp.Cash <= 0:
			case camp.Hedged:
				t.Emit(events.CampaignHedged{Day: t.Day, City: cid, Cash: camp.Cash})
			case camp.Ticket == stance:
				backed = true
				w.Stats.CampaignsWon++
			default:
				c.Pressure = math.Min(100, c.Pressure+s.cfg.Campaign.LoserPressure)
				chief := stance == "law_and_order" && s.cfg.Campaign.LoserChief && !replaced
				if chief {
					s.replaceChief(w, t, "campaign", "zealous")
					replaced = true
				}
				t.Emit(events.CampaignLost{Day: t.Day, City: cid, Ticket: camp.Ticket, Cash: camp.Cash, Winner: stance, Pressure: s.cfg.Campaign.LoserPressure, Chief: chief})
			}
		}
		da.Backed = backed
		w.Stats.Elections++
		t.Emit(events.DAElected{Day: t.Day, Name: da.Name, Stance: stance, Incumbent: incumbent, Pressure: mean, Swing: swing, Backed: backed})
		if stance == "law_and_order" && mean > tun.ReplacePressure && !replaced {
			s.replaceChief(w, t, "da", "")
		}
	}

	// Tomorrow's campaign window (#193): after this step the world's day
	// is the tick's, and the tickets take money from open_days out to
	// the day before the vote (the election's tick resolves that day).
	w.Law.CampaignOpen = s.CampaignOpen(w, t.Day)
}

// replaceChief puts a new chief in office: a new name and a personality
// drawn from the law's side stream, hidden until observed; personality,
// if given, is who the mayor was told to name (#193's zealous chief).
func (s *Sim) replaceChief(w *game.World, t *game.Tick, why, personality string) {
	rng := t.Sub(s.Name())
	old := w.Law.Chief.Name
	name := s.pick(without(s.chiefs, old), rng, old)
	if personality == "" {
		personality = content.ChiefPersonalities[rng.IntN(len(content.ChiefPersonalities))]
	}
	w.Law.Chief = game.Chief{Name: name, Personality: personality, Since: t.Day}
	w.Stats.Chiefs++
	t.Emit(events.ChiefReplaced{Day: t.Day, Name: name, Old: old, Why: why})
}

// without is the pool less one name: nobody succeeds themselves.
func without(pool []string, name string) []string {
	var out []string
	for _, n := range pool {
		if n != name {
			out = append(out, n)
		}
	}
	return out
}

// band is which band of width w a value sits in; 100 sits in the top one.
func band(v, w float64) int {
	if w <= 0 {
		return 0
	}
	return int(math.Min(v, 99.999) / w)
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }
