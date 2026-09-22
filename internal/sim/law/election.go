package law

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// back joins today's campaign money to its city's campaign.
func (s *Sim) back(w *game.World, t *game.Tick) {
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
}

// elect holds the election when it is due; replaced is whether the chief
// was already replaced today.
func (s *Sim) elect(w *game.World, t *game.Tick, replaced bool) {
	tun := s.cfg.Law
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
		share := max(0, min(1, s.LawAndOrderShare(mean)+swing))
		law := share * (1 - tun.Moderate)
		reform := (1 - share) * (1 - tun.Moderate)
		rng := t.Sub(game.StreamLaw)
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
		w.Law.SnapElection = 0
		// A law-and-order DA taking office (#42): every live deal ends
		// calls_stop_days on, and nobody takes a call while they sit; any
		// other winner opens the phones again.
		if !incumbent {
			w.Law.Cold = 0
			if stance == "law_and_order" {
				w.Law.Cold = t.Day + s.cfg.Bribes.CallsStopDays
			}
		}
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
}
