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
	deed   content.DeedTuning   // #194: the pressure a deed adds in its city a day, and the forfeiture's line
	assets content.AssetsConfig // #48: the pressure an owned asset adds in its city every day
	intel  content.IntelTuning  // #45: what a cop's word on the chief is worth
}

// New builds a law sim from the config, copying what it reads (#144):
// its own law.toml, the chiefs' and DAs' name pools, of the deeds
// (#194, city.toml [deed]) two numbers: pressure, a deed's a day in its
// city, and forfeit_ratio, the multiple of what the fronts have washed
// the deeds held may cost before the DA takes one back, the assets
// (#48) for the pressure each adds in its city while owned, and the
// intel tuning (#45) for the chief's fact.
func New(cfg *content.Config) *Sim {
	return &Sim{cfg: cfg.Law, chiefs: cfg.Names.Chiefs, das: cfg.Names.DAs, deed: cfg.City.Deed, assets: cfg.Assets, intel: cfg.Intel.Intel}
}

// DeedLimit is what the deeds held may cost between them before the DA
// seizes one (#194): forfeit_ratio times Stats.Laundered. The ledger's
// line, and what the harness's boss keeps under.
func (s *Sim) DeedLimit(w *game.World) int {
	return int(math.Floor(s.deed.ForfeitRatio * float64(w.Stats.Laundered)))
}

// Forfeits reports whether the deeds held have passed the limit: the
// money has no story.
func (s *Sim) Forfeits(w *game.World) bool {
	return s.deed.On() && w.DeedValue() > s.DeedLimit(w)
}

func (s *Sim) Name() string { return "law" }

// Tuning exposes the constants the UI needs to explain itself.
func (s *Sim) Tuning() content.LawTuning { return s.cfg.Law }

// Campaign exposes the campaign tuning (#193): what a city's vote costs
// to move, and how far money moves it.
func (s *Sim) Campaign() content.CampaignTuning { return s.cfg.Campaign }

// Seed picks the run's chief and DA from rng so they are part of the
// seed like the rival: a name and a personality, a name and a ticket.
// Both take office on the current day.
func (s *Sim) Seed(w *game.World, rng game.Rand) {
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

func (s *Sim) pick(pool []string, rng game.Rand, fallback string) string {
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

// NextElection is the day the DA next faces the voters, or 0 if never:
// the end of the term, or a snap election called by an incident (#44)
// if it comes sooner.
func (s *Sim) NextElection(w *game.World) int {
	if s.cfg.Law.TermDays <= 0 {
		return 0
	}
	next := w.Law.DA.ElectedDay + s.cfg.Law.TermDays
	if snap := w.Law.SnapElection; snap > 0 && snap < next {
		return snap
	}
	return next
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
	return max(0, min(1, 0.5+(pressure-50)/100*s.cfg.Law.ElectionSwing))
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
// w.Law.CampaignOpen for the day to come.
func (s *Sim) CampaignOpen(w *game.World, day int) bool {
	next := s.NextElection(w)
	return next > 0 && day <= next && next-day <= s.cfg.Campaign.OpenDays
}

// Bribes exposes the bribe tuning (#42): the prices and the days.
func (s *Sim) Bribes() content.BribeTuning { return s.cfg.Bribes }

// DAPrice is what the DA's office wants today (#42): da_price, halved
// (backed_da_price_mul) under a DA whose ticket ran on your money
// (#193), less the fixer's cut by their skill. It is the amount at which
// a moderate takes it at even odds.
func (s *Sim) DAPrice(w *game.World) int {
	price := float64(s.cfg.Bribes.DAPrice)
	if w.Law.DA.Backed && s.cfg.Campaign.BackedDAPriceMul > 0 {
		price *= s.cfg.Campaign.BackedDAPriceMul
	}
	if f := w.Crew.Fixer(); f != nil {
		price *= 1 - s.cfg.Bribes.FixerDiscount*float64(f.Skill)/100
	}
	return max(1, int(math.Round(price)))
}

// DAOdds is the chance the DA takes an envelope of amount (#42): the
// amount against DAPrice, plus the fixer's word, capped at da_odds_cap;
// nothing for a reformer, and a law-and-order DA does not take, they
// file. A DA you backed takes it at a moderate's odds whatever their
// ticket: they owe you.
func (s *Sim) DAOdds(w *game.World, amount int) float64 {
	da := w.Law.DA
	if !da.Backed && da.Stance != "moderate" {
		return 0
	}
	odds := float64(amount) / float64(s.DAPrice(w)) / 2
	if f := w.Crew.Fixer(); f != nil {
		odds += s.cfg.Bribes.FixerOdds * float64(f.Skill) / 100
	}
	return max(0, min(s.cfg.Bribes.DAOddsCap, odds))
}

// ChiefTakes is what the chief does with an envelope (#42): a corrupt
// one takes it whole, a lazy one at lazy_effect, a zealous one files it
// (share 0, backfire true). Under the price it is pocketed either way.
func (s *Sim) ChiefTakes(w *game.World, amount int) (share float64, backfire bool) {
	switch w.Law.Chief.Personality {
	case "zealous":
		return 0, true
	case "lazy":
		share = s.cfg.Bribes.LazyEffect
	default:
		share = 1
	}
	if amount < s.cfg.Bribes.ChiefPrice {
		return 0, false
	}
	return share, false
}

// bribes resolves today's envelopes (#42) on the bribes side stream,
// fades the leads, and on the cold day ends every live deal.
func (s *Sim) bribes(w *game.World, t *game.Tick) {
	tun := s.cfg.Bribes
	l := &w.Law
	if l.Leads > 0 && tun.LeadDecayDays > 0 && t.Day-l.LeadDay >= tun.LeadDecayDays {
		l.Leads--
		l.LeadDay = t.Day
	}
	lead := func() {
		l.Leads++
		l.LeadDay = t.Day
		w.Stats.Leads++
		t.Emit(events.LeadFound{Day: t.Day, Leads: l.Leads, Case: tun.LeadsCase})
		if tun.LeadsCase > 0 && l.Leads >= tun.LeadsCase {
			t.Emit(events.LeadsFiled{Day: t.Day, Leads: l.Leads, Evidence: tun.LeadEvidence})
			l.Leads = 0
			l.Filed = t.Day
		}
	}
	backfire := func(target string, amount int) {
		l.Backfired = t.Day
		w.Stats.Backfires++
		t.Emit(events.BribeBackfired{Day: t.Day, Target: target, Amount: amount, Evidence: tun.BackfireEvidence, Heat: tun.BackfireHeat})
	}
	for _, b := range w.Today.Bribes {
		switch b.Target {
		case game.BribeChief:
			share, bad := s.ChiefTakes(w, b.Amount)
			switch {
			case bad:
				backfire(b.Target, b.Amount)
			case share <= 0:
				t.Emit(events.BribeRefused{Day: t.Day, Target: b.Target, Amount: b.Amount, Why: "short"})
			default:
				from := t.Day
				if l.ChiefBoughtOn(t.Day) {
					from = l.ChiefBought
				}
				l.ChiefBought = from + tun.BribeDays
				l.ChiefShare = share
				// The favour (#228): a chief whose envelope takes owes
				// you one, to favours_max; a backfire owes nothing.
				favour := tun.FavoursMax > 0 && l.Favours < tun.FavoursMax
				if favour {
					l.Favours++
				}
				t.Emit(events.BribeAccepted{Day: t.Day, Target: b.Target, Amount: b.Amount, Until: l.ChiefBought, Share: share, Leads: l.Leads + 1, Favour: favour})
				lead()
			}
		case game.BribeDA:
			switch {
			case l.DA.Stance == "law_and_order" && !l.DA.Backed:
				backfire(b.Target, b.Amount)
			case l.DA.Stance == "reform" && !l.DA.Backed:
				t.Emit(events.BribeRefused{Day: t.Day, Target: b.Target, Amount: b.Amount, Why: "quiet"})
			default:
				odds := s.DAOdds(w, b.Amount)
				if t.Sub(game.StreamBribes).Float64() >= odds {
					t.Emit(events.BribeRefused{Day: t.Day, Target: b.Target, Amount: b.Amount, Why: "odds", Odds: odds})
					continue
				}
				from := t.Day
				if l.DABoughtOn(t.Day) {
					from = l.DABought
				}
				l.DABought = from + tun.BribeDays
				t.Emit(events.BribeAccepted{Day: t.Day, Target: b.Target, Amount: b.Amount, Until: l.DABought, Odds: odds, Leads: l.Leads + 1})
				lead()
			}
		}
	}
	// The cold day: a law-and-order DA took office calls_stop_days ago
	// and the word has got round. Whatever was live ends; the routes'
	// deals are dead from today by the same rule (World.CheckpointLive)
	// and the logistics sim clears them tomorrow.
	if l.Cold > 0 && t.Day == l.Cold {
		ev := events.OfficialsCold{Day: t.Day, DA: l.DA.Name, Chief: l.ChiefBought > t.Day, Bought: l.DABought > t.Day}
		for _, id := range sortedRoutes(w) {
			if w.Routes[id].Bought > t.Day {
				ev.Routes = append(ev.Routes, id)
			}
		}
		l.ChiefBought, l.ChiefShare, l.DABought = 0, 0, 0
		l.Favours = 0 // the cold kills the favour with the bribe (#228)
		if ev.Chief || ev.Bought || len(ev.Routes) > 0 {
			t.Emit(ev)
		}
	}
}

// sortedRoutes is the route ids with a setting, in a fixed order.
func sortedRoutes(w *game.World) []string {
	var ids []string
	for id := range w.Routes {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// Step turns today's funding into goodwill, moves every city's pressure
// from the day's violence, hard product and headlines, lets the chief's
// term run out, and holds the election when it is due. It runs as
// phases in a fixed order (#275): the envelopes, the funding
// (pressure.go), the backing (election.go), the pressure's sources and
// its fade (pressure.go), the chief (chief.go), the election
// (election.go), the forfeiture and tomorrow's campaign window. The
// order is the dice's and the report's: a phase moved is a run that
// reads differently.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	// The envelopes (#42), before the vote: a bribe on election day goes
	// to the sitting DA.
	s.bribes(w, t)

	s.fund(w, t)
	s.back(w, t)
	gain := s.sources(w, t)
	s.fade(w, t, gain)
	replaced := s.chief(w, t)
	s.elect(w, t, replaced)
	s.forfeit(w, t)

	// Tomorrow's campaign window (#193): the tickets take money from
	// open_days before the election to the day of it.
	w.Law.CampaignOpen = s.CampaignOpen(w, t.Day+1)
}

// forfeit takes the newest deed back when the deeds outrun the wash.
func (s *Sim) forfeit(w *game.World, t *game.Tick) {
	// The forfeiture (#194): the deeds held cost more than forfeit_ratio
	// times what the fronts have washed, so the money has no story and
	// the DA takes the newest block back: one a night, no refund, no
	// dice (a threshold, not a die). The heat sim reads Forfeited the
	// next morning and files the pages: buying it was something you did.
	if s.Forfeits(w) {
		if c := w.NewestDeed(); c != nil {
			spent, limit := w.DeedValue(), s.DeedLimit(w)
			d := w.SeizeDeed(c.ID)
			w.Law.Forfeited = t.Day
			t.Emit(events.DeedSeized{Day: t.Day, Corner: c.ID, Name: c.Name, City: c.City, Price: d.Price, Spent: spent, Washed: w.Stats.Laundered, Limit: limit})
		}
	}
}
