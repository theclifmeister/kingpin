package news

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// reportRivals writes the rivals sim's events into the morning: the
// headlines and the report's lines. It says whether e was one of them.
func (r *reporter) reportRivals(e events.Event) bool {
	w, t, rep, base := r.w, r.t, r.rep, r.base
	switch ev := e.(type) {
	case events.RivalMovedIn:
		d := base
		d.Corner, d.Rival = ev.Name, ev.Rival
		d = r.crew(d, ev.Rival)
		r.add("rivals", "RivalMovedIn", d)
		rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew moved in on %s. Somebody new wants the city.", ev.Rival, ev.Name))
	case events.CornerTaken:
		d := base
		d.Corner, d.Rival = ev.Name, ev.Rival
		d = r.crew(d, ev.Rival)
		switch {
		case ev.Handed != "":
			d.Name = ev.Handed
			r.add("rivals", "CornerHanded", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s walked %s's crew onto %s. It is theirs now.", ev.Handed, ev.Rival, ev.Name))
		case ev.From == game.OwnerPlayer && ev.Pricewar:
			r.add("rivals", "CornerTaken", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew TOOK %s from you: the price war's answer. Your people walked home.", ev.Rival, ev.Name))
		case ev.From == game.OwnerPlayer:
			r.add("rivals", "CornerTaken", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew TOOK %s from you. Your people walked home.", ev.Rival, ev.Name))
		default:
			r.add("rivals", "RivalClaimed", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew set up on %s.", ev.Rival, ev.Name))
		}
	case events.RivalEyeing:
		d := base
		d.Corner, d.Rival = ev.Name, ev.Rival
		d = r.crew(d, ev.Rival)
		r.add("rivals", "RivalEyeing", d)
		rep.Territory = append(rep.Territory, fmt.Sprintf("Word is %s's crew are setting up on %s tomorrow%s. Post somebody on it tonight and it stays off them.", ev.Rival, ev.Name, eyeingWhy(w.Faction(ev.Faction))))
	case events.RivalOutbid:
		d := base
		d.Corner, d.Rival = ev.Name, ev.Rival
		d = r.crew(d, ev.Rival)
		r.add("rivals", "RivalOutbid", d)
		rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew came for %s and found your people on it. They left; they will not forget it.", ev.Rival, ev.Name))
	case events.RivalPushed:
		d := base
		d.Corner, d.Rival = ev.Name, ev.Rival
		d = r.crew(d, ev.Rival)
		r.add("rivals", "RivalPushed", d)
		if ev.Pricewar {
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew pushed on %s over the price war. You held it.", ev.Rival, ev.Name))
		} else {
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew pushed on %s. You held it.", ev.Rival, ev.Name))
		}
	case events.CornerStruck:
		d := base
		d.Corner, d.Rival = ev.Name, ev.Rival
		d = r.crew(d, ev.Rival)
		who := "Your enforcers"
		if ev.War {
			who = fmt.Sprintf("The war on %s's crew: your enforcers", ev.Rival) // the war order (#229)
		}
		switch {
		case ev.Routed:
			r.add("rivals", "RivalRouted", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s %s %s and TOOK it. That was %s's last corner.", who, pastTense(ev.Force), ev.Name, ev.Rival))
		case ev.Taken:
			r.add("rivals", "CornerStruckTaken", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s %s %s and TOOK it. Post a runner before it drifts.", who, pastTense(ev.Force), ev.Name))
		default:
			r.add("rivals", "CornerStruckHeld", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s %s %s; %s's people held it.", who, pastTense(ev.Force), ev.Name, ev.Rival))
		}
	case events.WarEnded:
		d := base
		d.Rival = ev.Rival
		d = r.crew(d, ev.Rival)
		r.add("rivals", "WarEnded", d)
		rep.Territory = append(rep.Territory, fmt.Sprintf("The war on %s's crew is over: %s. The enforcers stand down.", ev.Rival, ev.Why))
	case events.RivalTippedPolice:
		d := base
		d.Rival = ev.Rival
		d = r.crew(d, ev.Rival)
		r.add("rivals", "RivalTippedPolice", d)
		rep.Territory = append(rep.Territory, fmt.Sprintf("Somebody tipped the police about you. It was %s. Heat +%.0f.", ev.Rival, ev.Heat))
	// The books (#70): the scout, the boost, the tip, the raid and
	// the buy-off. The scout, the tip and the buy-off are report-only;
	// the boost and the raid are news.
	case events.RivalScouted:
		r.scouted += ev.Cost
		who := w.Faction(ev.Faction)
		if who == nil {
			who = w.Rival()
		}
		if ev.Read {
			rep.Territory = append(rep.Territory, fmt.Sprintf("Your scout read %s's books: %s in the chest, %s a day coming in, %s on the payroll costing %s a day. It goes stale; the rivals screen (8) says how old it is.", who.Leader, format.Cash(ev.Cash), format.Cash(ev.Income), format.Plural(ev.Muscle, "head"), format.Cash(ev.Wages)))
		} else {
			rep.Territory = append(rep.Territory, fmt.Sprintf("Your scout got nowhere near %s's books. Next time is likelier.", who.Leader))
		}
		rep.Money = append(rep.Money, fmt.Sprintf("Scouting %s's books -%s", who.Leader, format.Money(ev.Cost)))
	case events.RivalBoosted:
		d := base
		d.Corner, d.Rival = ev.Name, ev.Rival
		d = r.crew(d, ev.Rival)
		if ev.Taken {
			r.boosted += ev.Cash
			r.add("rivals", "RivalBoosted", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("Your enforcers robbed %s on %s: %s off their day's take, into your pocket. The corner is still theirs.", ev.Rival, ev.Name, format.Money(ev.Cash)))
			rep.Money = append(rep.Money, fmt.Sprintf("Robbed %s on %s +%s", ev.Rival, ev.Name, format.Money(ev.Cash)))
		} else {
			r.add("rivals", "RivalBoostedHeld", d)
			line := fmt.Sprintf("Your enforcers went for %s's takings on %s and came back with nothing.", ev.Rival, ev.Name)
			if ev.Hurt > 0 {
				line += " One of them got hurt."
			}
			rep.Territory = append(rep.Territory, line)
		}
	case events.PoliceTipped:
		line := fmt.Sprintf("You tipped the police on %s. Their attention on %s is at %.0f.", ev.Name, ev.Rival, ev.RivalHeat)
		if ev.Betrayal {
			line += " That broke the peace."
		}
		rep.Territory = append(rep.Territory, line)
	case events.RivalRaided:
		d := base
		d.Corner, d.Rival = ev.Name, ev.Rival
		d = r.crew(d, ev.Rival)
		r.add("rivals", "RivalRaided", d)
		rep.Territory = append(rep.Territory, fmt.Sprintf("The police RAIDED %s on %s: it is free, and %s of theirs went in the van. Post a runner before somebody else does.", ev.Rival, ev.Name, format.Plural(ev.Muscle, "head")))
	case events.RivalMusclePoached:
		d := base
		d.Rival = ev.Rival
		d = r.crew(d, ev.Rival)
		r.poached += ev.Cost - ev.Refund
		switch {
		case ev.Failed:
			rep.Territory = append(rep.Territory, fmt.Sprintf("Your money never reached %s's people, or they took it and stayed. %s knows you tried.", ev.Rival, ev.Rival))
			rep.Money = append(rep.Money, fmt.Sprintf("Buying off %s's muscle, lost -%s", ev.Rival, format.Money(ev.Cost)))
		case ev.Got == 0:
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s had nobody left to buy off. Your money came back.", ev.Rival))
		default:
			r.add("rivals", "RivalMusclePoached", d)
			line := fmt.Sprintf("%s of %s's muscle took your money and went home. They are nobody's now.", format.Plural(ev.Got, "head"), ev.Rival)
			if ev.Refund > 0 {
				line += fmt.Sprintf(" They only had %d; %s came back.", ev.Got, format.Money(ev.Refund))
			}
			rep.Territory = append(rep.Territory, line)
			rep.Money = append(rep.Money, fmt.Sprintf("Bought off %s of %s's muscle -%s", format.Plural(ev.Got, "head"), ev.Rival, format.Money(ev.Cost-ev.Refund)))
		}
	case events.RivalUndercut:
		rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew are undercutting you on %s: -%.0f%% demand there.", ev.Rival, strings.Join(ev.Corners, ", "), ev.Share*100))
	case events.RivalAbandoned:
		d := base
		d.Corner, d.Rival = ev.Name, ev.Rival
		d = r.crew(d, ev.Rival)
		r.add("rivals", "RivalAbandoned", d)
		rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew GAVE UP %s: the price war made it not worth holding. It is free; post a runner before somebody else does.", ev.Rival, ev.Name))
	case events.DealOffered:
		d := base
		d.Rival, d.Deal = ev.Rival, ev.Deal
		d = r.crew(d, ev.Rival)
		r.add("rivals", "DealOffered", d)
		rep.Territory = append(rep.Territory, fmt.Sprintf("%s offers %s. It stands %s: answer it on the rivals screen (8).", ev.Rival, ev.Terms, format.Plural(ev.Expires-t.Day+1, "day")))
	case events.DealAccepted:
		d := base
		d.Rival, d.Deal = ev.Rival, ev.Deal
		d = r.crew(d, ev.Rival)
		r.add("rivals", "DealAccepted", d)
		if ev.Offered {
			rep.Territory = append(rep.Territory, fmt.Sprintf("You took %s's offer: %s. It holds from tonight.", ev.Rival, ev.Terms))
		} else {
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s ACCEPTED %s. It holds from tonight.", ev.Rival, ev.Terms))
		}
	case events.DealRefused:
		d := base
		d.Rival, d.Deal = ev.Rival, ev.Deal
		d = r.crew(d, ev.Rival)
		r.add("rivals", "DealRefused", d)
		rep.Territory = append(rep.Territory, fmt.Sprintf("%s refused %s.", ev.Rival, ev.Terms))
	case events.DealBroken:
		d := base
		d.Rival, d.Deal = ev.Rival, ev.Deal
		d = r.crew(d, ev.Rival)
		r.add("rivals", "DealBroken", d)
		if ev.By == "rival" {
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s BROKE the %s: %s. So much for that.", ev.Rival, ev.Deal, ev.Why))
		} else {
			rep.Territory = append(rep.Territory, fmt.Sprintf("You BROKE the %s with %s: %s. Trust is gone, and they made a call.", ev.Deal, ev.Rival, ev.Why))
		}
	case events.DealEnded:
		if ev.Deal == game.DealHomage {
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s can no longer pay you homage. The money stops; they are nobody's now.", ev.Rival))
		} else {
			rep.Territory = append(rep.Territory, fmt.Sprintf("The %s with %s has run out. Expect them back on your corners.", ev.Deal, ev.Rival))
		}
	case events.TributePaid:
		if ev.ToYou {
			rep.Money = append(rep.Money, fmt.Sprintf("Homage from %s +%s", ev.Rival, format.Money(ev.Amount)))
			break
		}
		r.tribute += ev.Amount
		rep.Money = append(rep.Money, fmt.Sprintf("Tribute to %s -%s", ev.Rival, format.Money(ev.Amount)))
	// The table (#43): factions fighting each other, one absorbing
	// another, a leader taken, your crew poached, a betrayal
	// remembered by everyone.
	case events.FactionPushed:
		d := r.at(ev.City)
		d.Corner, d.Rival = ev.Name, ev.Rival
		d = r.crew(d, ev.Rival)
		d.Name = ev.AgainstRival
		if ev.Taken {
			r.add("rivals", "FactionTook", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew TOOK %s off %s's%s.", ev.Rival, ev.Name, ev.AgainstRival, r.in(ev.City)))
		} else {
			r.add("rivals", "FactionPushed", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew pushed on %s's people on %s%s and were held off.", ev.Rival, ev.AgainstRival, ev.Name, r.in(ev.City)))
		}
	case events.RivalAbsorbed:
		d := base
		d.Rival = ev.Rival
		d = r.crew(d, ev.Rival)
		d.Name = ev.By
		if ev.By != "" {
			r.add("rivals", "RivalAbsorbed", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew is no more: what was left of it went over to %s. One faction fewer.", ev.Rival, ev.By))
		} else {
			r.add("rivals", "RivalScattered", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew scattered: nobody left to run with. One faction fewer.", ev.Rival))
		}
	case events.RivalLeaderArrested:
		d := r.at(ev.City)
		d.Rival = ev.Rival
		d = r.crew(d, ev.Rival)
		if ev.Killed {
			r.add("rivals", "RivalLeaderKilled", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s is DEAD. Their crew is coming apart: %s go back to the street over the coming days, prices%s spike, and their people are looking for work%s.", ev.Rival, format.Plural(ev.Corners, "corner"), r.in(ev.City), pointer(ev.Muscle)))
		} else {
			r.add("rivals", "RivalLeaderArrested", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("The police took %s. Their crew is coming apart: %s go back to the street over the coming days, prices%s spike, and their people are looking for work%s.", ev.Rival, format.Plural(ev.Corners, "corner"), r.in(ev.City), pointer(ev.Muscle)))
		}
	case events.CrewPoached:
		d := base
		d.Name, d.Role, d.Rival = ev.Name, ev.Role, ev.Rival
		d = r.crew(d, ev.Rival)
		if ev.Stayed {
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s offered %s %s a day to come over. They stayed, and they know they are wanted.", ev.Rival, ev.Name, format.Money(ev.Wages)))
		} else {
			r.add("crew", "CrewPoached", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s offered %s %s a day and they TOOK it. They are %s's now.", ev.Rival, ev.Name, format.Money(ev.Wages), ev.Rival))
		}
	case events.TrustSpread:
		rep.Territory = append(rep.Territory, fmt.Sprintf("Word of the broken deal got round: %s trust you %.0f less.", format.Plural(ev.Others, "other faction"), ev.Spread))
	case events.WarEscalated:
		d := base
		if ev.Stage == "crackdown" {
			r.add("rivals", "WarCrackdown", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("CRACKDOWN. The police cleared %s. Both sides lost ground.", strings.Join(ev.Lost, ", ")))
		} else {
			r.add("rivals", "WarOpen", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("The war is loud (%.0f/100). Keep it up and the police clear both sides.", ev.War))
		}
	default:
		return false
	}
	return true
}
