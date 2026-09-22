package news

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// reportLaw writes the law sim's events into the morning: the
// headlines and the report's lines. It says whether e was one of them.
func (r *reporter) reportLaw(e events.Event) bool {
	w, t, rep, here := r.w, r.t, r.rep, r.here
	switch ev := e.(type) {
	case events.DAElected:
		d := r.at(w.Home().ID)
		d.Name, d.Stance = ev.Name, stanceWords(ev.Stance)
		key := "DAElected"
		switch {
		case ev.Backed:
			key = "DABought" // #193: the paper knows whose money it was
		case ev.Incumbent:
			key = "DAReElected"
		}
		r.add("law", key, d)
		rep.Law = append([]string{electionLine(ev)}, rep.Law...) // the courthouse before the small print
	case events.ChiefReplaced:
		d := r.at(w.Home().ID)
		d.Name, d.Rival = ev.Name, ev.Old
		key := "ChiefReplaced"
		switch ev.Why {
		case "da":
			key = "ChiefReplacedDA"
		case "campaign":
			key = "ChiefReplacedCampaign"
		}
		r.add("law", key, d)
		rep.Law = append([]string{chiefLine(ev)}, rep.Law...)
	// Campaigns (#193): the money in, and what it bought at the count.
	case events.CampaignBacked:
		r.backed += ev.Amount
		rep.Law = append(rep.Law, fmt.Sprintf("Put %s clean behind the %s ticket in %s: the campaign holds %s, %s of the city's vote", format.Money(ev.Amount), stanceWords(ev.Ticket), w.CityName(ev.City), format.Money(ev.Total), swingWords(ev.Swing)))
		rep.Money = append(rep.Money, fmt.Sprintf("Campaign in %s -%s clean", w.CityName(ev.City), format.Money(ev.Amount)))
	case events.CampaignLost:
		d := r.at(ev.City)
		d.Stance = stanceWords(ev.Winner)
		r.add("law", "CampaignLost", d)
		line := fmt.Sprintf("The %s you paid for in %s lost: DA %s knows who backed the other side. Pressure +%.0f there", format.Money(ev.Cash), w.CityName(ev.City), w.Law.DA.Name, ev.Pressure)
		if ev.Chief {
			line += ", and the mayor named a zealous chief before the count was cold"
		}
		rep.Law = append(rep.Law, line+".")
	case events.CampaignHedged:
		r.add("law", "CampaignHedged", r.at(ev.City))
		rep.Law = append(rep.Law, fmt.Sprintf("%s in %s went to both tickets: nobody owes you, and everybody knows it.", format.Money(ev.Cash), w.CityName(ev.City)))
	// The bought law (#42): the envelopes, the leads, the deals on
	// the road, and the day it all stops.
	case events.BribeAccepted:
		r.bribed += ev.Amount
		leadsCase := 0
		for _, e2 := range t.Events() {
			if lf, ok := e2.(events.LeadFound); ok {
				leadsCase = lf.Case
			}
		}
		who := "Chief " + w.Law.Chief.Name
		effect := fmt.Sprintf("heat fades faster and the stings and raids come slower until day %d", ev.Until)
		if ev.Target == game.BribeDA {
			who = "DA " + w.Law.DA.Name
			effect = fmt.Sprintf("it takes a thicker file to indict until day %d", ev.Until)
		} else if ev.Share < 1 {
			effect = fmt.Sprintf("a lazy chief, half the good: %s", effect)
		}
		line := fmt.Sprintf("%s took the %s: %s. Somebody at the DA's office heard (lead %d of %d).", who, format.Money(ev.Amount), effect, ev.Leads, leadsCase)
		if ev.Favour {
			line += " The chief owes you one: call it in on a morning a raid is due and it will not come."
		}
		rep.Law = append(rep.Law, line)
		rep.Money = append(rep.Money, fmt.Sprintf("Envelope for %s -%s", who, format.Money(ev.Amount)))
	case events.BribeRefused:
		r.bribed += ev.Amount
		who := "Chief " + w.Law.Chief.Name
		if ev.Target == game.BribeDA {
			who = "DA " + w.Law.DA.Name
		}
		why := "pocketed it and did nothing: it was under the price."
		switch ev.Why {
		case "quiet":
			why = "sent it back with no note. A reformer; nothing came of it."
		case "odds":
			why = fmt.Sprintf("kept it and did nothing this time (~%.0f%% it would land).", ev.Odds*100)
		}
		rep.Law = append(rep.Law, fmt.Sprintf("%s %s", who, why))
		rep.Money = append(rep.Money, fmt.Sprintf("Envelope for %s -%s", who, format.Money(ev.Amount)))
	case events.BribeBackfired:
		r.bribed += ev.Amount
		who := "Chief " + w.Law.Chief.Name
		if ev.Target == game.BribeDA {
			who = "DA " + w.Law.DA.Name
		}
		d := r.at(here.ID)
		d.Name = who
		r.add("heat", "BribeBackfired", d) // the blotter's, about you: notoriety and pressure count it
		rep.Law = append(rep.Law, fmt.Sprintf("%s does not take envelopes: the %s is in an evidence bag. Tomorrow the file grows by %d and the heat by %.0f.", who, format.Money(ev.Amount), ev.Evidence, ev.Heat))
		rep.Money = append(rep.Money, fmt.Sprintf("Envelope for %s, backfired -%s", who, format.Money(ev.Amount)))
	case events.LeadsFiled:
		r.add("heat", "LeadsFiled", r.at(here.ID))
		rep.Law = append(rep.Law, fmt.Sprintf("The DA's office has heard about enough envelopes to open a file: tomorrow it grows by %d.", ev.Evidence))
	case events.OfficialsCold:
		r.add("law", "OfficialsCold", r.at(w.Home().ID))
		var ended []string
		if ev.Chief {
			ended = append(ended, "the chief")
		}
		if ev.Bought {
			ended = append(ended, "the DA")
		}
		if n := len(ev.Routes); n > 0 {
			ended = append(ended, format.Plural(n, "deal")+" on the road")
		}
		rep.Law = append([]string{fmt.Sprintf("Under DA %s nobody takes calls any more: %s stopped being yours today, and nothing is for sale while they sit.", ev.DA, strings.Join(ended, ", "))}, rep.Law...)
	case events.PressureShifted:
		key := "PressureShiftedDown"
		if ev.Up() {
			key = "PressureShiftedUp"
		}
		r.add("law", key, r.at(ev.City))
		rep.Law = append(rep.Law, fmt.Sprintf("%s pressure %.0f %s %.0f", w.CityName(ev.City), ev.From, format.Arrow, ev.To))
	case events.CityFunded:
		r.funded += ev.Amount
		rep.Law = append(rep.Law, fmt.Sprintf("Gave %s %s clean: goodwill +%.0f (now %.0f)", w.CityName(ev.City), format.Money(ev.Amount), ev.Goodwill, w.Cities[ev.City].Goodwill))
		rep.Money = append(rep.Money, fmt.Sprintf("Funded %s -%s clean", w.CityName(ev.City), format.Money(ev.Amount)))
	case events.DeedSeized:
		d := r.at(ev.City)
		d.Corner = ev.Name
		r.addOff(game.StreamDeedsNews, "laundering", "DeedSeized", d)
		rep.Law = append(rep.Law, fmt.Sprintf("FORFEITURE: the DA seized the block %s is on%s (%s). %s in deeds against %s washed; the money has no story, and the file will grow in the morning.", ev.Name, r.in(ev.City), format.Money(ev.Price), format.Money(ev.Spent), format.Money(ev.Washed)))
	default:
		return false
	}
	return true
}
