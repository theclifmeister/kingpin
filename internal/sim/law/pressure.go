package law

import (
	"math"
	"slices"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// fund turns the clean cash given today into goodwill.
func (s *Sim) fund(w *game.World, t *game.Tick) {
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
}

// sources is each city's pressure gained today, from the day's events,
// yesterday's paper and what stands in the city.
func (s *Sim) sources(w *game.World, t *game.Tick) map[string]float64 {
	src := s.cfg.Pressure
	home := w.Home().ID
	here := w.Player.Location
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
		case events.FactionPushed:
			gain[ev.City] += src.Factions // two factions fighting (#43): violence in its city
		case events.CornerTaken:
			if ev.From == game.OwnerPlayer {
				gain[home] += src.Push
			}
		case events.WarEscalated:
			if ev.Stage == events.StageCrackdown {
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
		case events.Overdose:
			// Bad product on your corner (#47) is the city's story: it
			// is pressure where it happened and never a page (#27).
			gain[ev.City] += src.Overdose
		case events.CrewShot:
			// A body on a corner, either side (#46), is the city's
			// story the same way, where it fell (home for a strike or
			// a push with no corner named).
			if ev.Dead {
				city := ev.City
				if city == "" {
					city = home
				}
				gain[city] += src.Body
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
	// A front whose growth made the paper yesterday (#192): the
	// laundering sim steps after this one, so its Grew stamp is how the
	// story reaches today's pressure, where you are, as a headline does.
	for _, f := range w.Fronts {
		if f.Grew != 0 && f.Grew == t.Day-1 {
			gain[here] += src.FrontGrew
		}
	}
	// A campaign is public money (#193): the city it runs in talks.
	for _, cid := range w.CityOrder {
		if w.Cities[cid].Campaign.Cash > 0 {
			gain[cid] += s.cfg.Campaign.Pressure
		}
	}
	// A deed is public record (#194): every block you hold in a city is
	// pressure there a day, over what goodwill covers.
	if s.deed.On() && s.deed.Pressure > 0 {
		for _, cid := range w.CityOrder {
			gain[cid] += s.deed.Pressure * float64(w.DeedsIn(cid))
		}
	}
	// An asset (#48) is a thing the whole city can see: its pressure
	// lands in its city every day it stands.
	for _, a := range s.assets.Offers {
		if a.Pressure > 0 && w.AssetLive(a.ID) {
			gain[a.City] += a.Pressure
		}
	}
	return gain
}

// fade moves every city's pressure by what it gained and toward the
// baseline, and fades its goodwill.
func (s *Sim) fade(w *game.World, t *game.Tick, gain map[string]float64) {
	tun := s.cfg.Law
	// Fade toward the baseline, goodwill takes its cut and fades itself,
	// and a band crossed is news.
	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		from := c.Pressure
		p := c.Pressure + gain[cid]
		p -= (p - tun.Baseline) * tun.Decay
		p -= tun.GoodwillCut * max(0, min(1, c.Goodwill/100))
		c.Pressure = max(0, min(100, p))
		c.Goodwill = max(0, min(100, c.Goodwill-c.Goodwill*tun.GoodwillDecay))
		if band(from, tun.Band) != band(c.Pressure, tun.Band) {
			t.Emit(events.PressureShifted{Day: t.Day, City: cid, From: from, To: c.Pressure})
		}
	}
}

// band is which band of width w a value sits in; 100 sits in the top one.
func band(v, w float64) int {
	if w <= 0 {
		return 0
	}
	return int(math.Min(v, 99.999) / w)
}
