package news

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// reportCrew writes the crew sim's events into the morning: the
// headlines and the report's lines. It says whether e was one of them.
func (r *reporter) reportCrew(e events.Event) bool {
	w, rep, base := r.w, r.rep, r.base
	switch ev := e.(type) {
	case events.CrewHired:
		d := base
		d.Name, d.Role = ev.Name, ev.Role
		r.add("crew", "CrewHired", d)
		rep.Crew = append(rep.Crew, fmt.Sprintf("%s signed on as %s for %s", ev.Name, format.A(ev.Role), format.Money(ev.Fee)))
	case events.CrewFired:
		d := base
		d.Name, d.Role = ev.Name, ev.Role
		if ev.Informant {
			r.add("crew", "CrewFiredInformant", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("You let %s go. Word is they had been talking to the police. Nobody mourned.", ev.Name))
		} else {
			r.add("crew", "CrewFired", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("You let %s go. The others noticed.", ev.Name))
		}
	case events.CrewDefected:
		d := base
		d.Name, d.Role, d.Rival, d.Corner = ev.Name, ev.Role, ev.Rival, ev.CornerName
		d = r.crew(d, ev.Rival)
		r.add("crew", "CrewDefected", d)
		if ev.Corner != "" {
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s went over to %s, and they know %s. Expect trouble there.", ev.Name, ev.Rival, ev.CornerName))
		} else {
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s went over to %s.", ev.Name, ev.Rival))
		}
	case events.InvestigationRun:
		r.book(game.FlowRoutes, -(ev.Cost - ev.Clean), -ev.Clean)
		r.add("crew", "InvestigationRun", base)
		if ev.Found {
			rep.Crew = append(rep.Crew, fmt.Sprintf("The investigation named %s: they have been talking to the police. Fire them (f) and the file stops growing.", ev.Name))
		} else {
			rep.Crew = append(rep.Crew, "The investigation named nobody. The crew resent being asked.")
		}
		rep.Money = append(rep.Money, fmt.Sprintf("Investigation -%s", format.Money(ev.Cost)))
	case events.CrewPaidOff:
		r.book(game.FlowWages, -(ev.Cost - ev.Clean), -ev.Clean)
		rep.Crew = append(rep.Crew, fmt.Sprintf("%s took your money and stays sweet on you, for now.", ev.Name))
		rep.Money = append(rep.Money, fmt.Sprintf("Paid off %s -%s", ev.Name, format.Money(ev.Cost)))
	case events.CrewBailed:
		// Crew life (#46): the cells, the cots and the funerals. The
		// bondsman's bail (#230, Who) is on the arrest's own line;
		// here it is the money.
		r.book(game.FlowRoutes, 0, -ev.Cost)
		if ev.Who != "" {
			rep.Money = append(rep.Money, fmt.Sprintf("Bail for %s -%s clean (%s)", ev.Name, format.Money(ev.Cost), ev.Who))
			break
		}
		rep.Crew = append(rep.Crew, fmt.Sprintf("Bail is down for %s: they walk tomorrow.", ev.Name))
		rep.Money = append(rep.Money, fmt.Sprintf("Bail for %s -%s clean", ev.Name, format.Money(ev.Cost)))
	case events.CrewArrested:
		d := r.at(ev.City)
		d.Name, d.Role, d.Corner = ev.Name, ev.Role, ev.CornerName
		r.add("crew", "CrewArrested", d)
		// The bail's tail (#230): a hand bail's price, the bondsman's
		// bail already paid, or the bondsman's account too short.
		tail := fmt.Sprintf("%s in a cell, %s clean to walk them out tomorrow.", format.Plural(ev.Days, "day"), format.Money(ev.Bail))
		switch {
		case ev.Sprung:
			tail = fmt.Sprintf("sprung by the lawyer before morning, %s clean out of the account.", format.Money(ev.Bail))
		case ev.Short:
			tail = fmt.Sprintf("%s in a cell; the lawyer could not cover the %s clean bail, and neither could you.", format.Plural(ev.Days, "day"), format.Money(ev.Bail))
		}
		switch {
		case ev.Route != "":
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s was taken with the shipment: %s", ev.Name, tail))
		case ev.Corner != "":
			rep.Crew = append(rep.Crew, fmt.Sprintf("The police took %s off %s: %s", ev.Name, ev.CornerName, tail))
		default:
			rep.Crew = append(rep.Crew, fmt.Sprintf("The raid found the lab: %s taken, %s", ev.Name, tail))
		}
	case events.CrewReleased:
		d := base
		d.Name, d.Role = ev.Name, ev.Role
		if ev.Bailed {
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s is out on your bail and knows who paid it.", ev.Name))
		} else {
			r.add("crew", "CrewReleased", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s is out. Nobody came for them, and the DA had them for a while.", ev.Name))
		}
	case events.CrewShot:
		d := r.at(ev.City)
		d.Name, d.Role, d.Corner, d.Rival = ev.Name, ev.Role, ev.CornerName, ev.Rival
		where := ""
		if ev.CornerName != "" {
			where = " on " + ev.CornerName
		}
		switch {
		case ev.Theirs:
			r.add("rivals", "MuscleKilled", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("One of %s's people was shot dead%s. The paper has your name next to it.", ev.Rival, where))
		case ev.Dead:
			r.add("crew", "CrewKilled", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s was shot dead%s. The paper has your name next to it.", ev.Name, where))
		default:
			r.add("crew", "CrewShot", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s was shot%s: laid up for %s, off the corner.", ev.Name, where, format.Plural(ev.Days, "day")))
		}
	case events.CrewRecovered:
		rep.Crew = append(rep.Crew, fmt.Sprintf("%s is back on their feet.", ev.Name))
	case events.CrewRetired:
		d := base
		d.Name, d.Role = ev.Name, ev.Role
		r.add("crew", "CrewRetired", d)
		switch {
		case ev.Kin != "":
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s retired at %d and put a word in for %s, who is looking for work.", ev.Name, ev.Age, ev.Kin))
		case ev.Sour:
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s retired at %d, and not kindly. They had a long talk with somebody on the way out.", ev.Name, ev.Age))
		default:
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s retired at %d.", ev.Name, ev.Age))
		}
	case events.KinLooking:
		rep.Crew = append(rep.Crew, fmt.Sprintf("%s's %s %s is looking for work, and would sign for %s.", ev.Of, ev.Role, ev.Name, format.Money(ev.Fee)))
	case events.CrewQuit:
		d := base
		d.Name, d.Role = ev.Name, ev.Role
		r.add("crew", "CrewQuit", d)
		rep.Crew = append(rep.Crew, fmt.Sprintf("%s walked. Nobody was surprised.", ev.Name))
	case events.LieutenantWalked:
		d := r.at(ev.City)
		d.Name, d.Rival = ev.Name, ev.Rival
		d = r.crew(d, ev.Rival)
		if ev.Rival != "" {
			r.add("crew", "LieutenantWalkedRival", d)
		} else {
			r.add("crew", "LieutenantWalked", d)
		}
		rep.Crew = append(rep.Crew, lieutenantWalkedLine(ev))
	case events.LieutenantActed:
		// The cut is the sales' net; a greedy one's skim on top is in
		// the night's CrewSkimmed with everybody else's.
		r.book(game.FlowSales, -ev.Cut, 0)
		rep.Crew = append(rep.Crew, lieutenantLines(w, ev)...)
		if ev.Cut > 0 {
			rep.Money = append(rep.Money, fmt.Sprintf("%s's cut of %s -%s", ev.Name, ev.CityName, format.Money(ev.Cut)))
		}
	case events.CrewTrait:
		// Veterans (#346): a known quantity now.
		d := base
		d.Name, d.Role, d.Trait = ev.Name, ev.Role, ev.Trait
		r.add("crew", "CrewTrait", d)
		if ev.Good {
			rep.Crew = append(rep.Crew, fmt.Sprintf("%d days in, %s has shown what they are: %s.", ev.Days, ev.Name, ev.Trait))
		} else {
			rep.Crew = append(rep.Crew, fmt.Sprintf("%d days in, %s has shown what they are: %s. Watch them.", ev.Days, ev.Name, ev.Trait))
		}
	case events.CaptainActed:
		// The cut is the sales' net, as a lieutenant's is; the pay-offs
		// are the loyalty bought on top of the wages, as a hand's are.
		r.book(game.FlowSales, -ev.Cut, 0)
		r.book(game.FlowWages, -(ev.Spent - ev.SpentClean), -ev.SpentClean)
		rep.Crew = append(rep.Crew, captainLines(ev)...)
		if ev.Cut > 0 {
			rep.Money = append(rep.Money, fmt.Sprintf("%s's cut of %s -%s", ev.Name, ev.CityName, format.Money(ev.Cut)))
		}
		if ev.Spent > 0 {
			rep.Money = append(rep.Money, fmt.Sprintf("%s's pay-offs in %s -%s", ev.Name, ev.CityName, format.Money(ev.Spent)))
		}
	case events.CrewSkimmed:
		r.add("crew", "CrewSkimmed", base)
		r.skimmed += ev.Amount
		r.book(game.FlowLosses, -(ev.Amount - ev.FromWash), -ev.FromWash)
		rep.Crew = append(rep.Crew, skimLine(ev))
	case events.CrewPaid:
		r.book(game.FlowWages, -ev.Wages, 0)
		line := fmt.Sprintf("Wages (%s) -%s", ev.Pay, format.Money(ev.Wages))
		if ev.Short > 0 {
			line += fmt.Sprintf(", %s SHORT", format.Money(ev.Short))
			rep.Crew = append(rep.Crew, "You could not make payroll. That gets around.")
		}
		rep.Money = append(rep.Money, line)
	case events.CookOrdered:
		r.book(game.FlowPurchases, -ev.Cost, 0)
		rep.Crew = append(rep.Crew, fmt.Sprintf("%s is cooking %d %s%s: quality %.0f, ready in %s = -%s", ev.Chemist, ev.Units, w.ProductName(ev.Product), r.in(ev.City), ev.Quality, format.Plural(ev.Days, "day"), format.Money(ev.Cost)))
		rep.Money = append(rep.Money, fmt.Sprintf("Precursors for %s -%s", w.ProductName(ev.Product), format.Money(ev.Cost)))
	case events.Cooked:
		rep.Crew = append(rep.Crew, fmt.Sprintf("%s's batch landed%s: %d %s at quality %.0f, paid %s on the order.", ev.Chemist, r.in(ev.City), ev.Units, w.ProductName(ev.Product), ev.Quality, format.Money(ev.Cost)))
	case events.SpyPlanted:
		d := base
		d.Name, d.Role = ev.Name, ev.Role
		d = r.crew(d, ev.Rival)
		r.addIntel("crew", "SpyPlanted", d)
		rep.Intel = append(rep.Intel, fmt.Sprintf("%s went under with %s's crew tonight. They sell nothing for you now and report every few days; the intel screen (9) keeps what they send.", ev.Name, ev.Rival))
	case events.SpyFound:
		d := base
		d.Name, d.Role = ev.Name, ev.Role
		d = r.crew(d, ev.Rival)
		switch {
		case ev.Dead:
			r.addIntel("crew", "SpyShot", d)
			rep.Intel = append(rep.Intel, fmt.Sprintf("%s's people made %s. They were found shot. %s.", ev.Rival, ev.Name, format.Plural(ev.Reports, "report")+" came back before it"))
		case ev.Why == "gone":
			rep.Intel = append(rep.Intel, fmt.Sprintf("%s's crew is no more; %s came home with nothing to add.", ev.Rival, ev.Name))
		default:
			r.addIntel("crew", "SpyFound", d)
			rep.Intel = append(rep.Intel, fmt.Sprintf("%s's people made %s and sent them home. They are back on the payroll, %s to their name.", ev.Rival, ev.Name, format.Plural(ev.Reports, "report")))
		}
	default:
		return false
	}
	return true
}

// skimLine is the night's missing money for the CREW section. A
// lieutenant's cut is not skimming and is never in it: where one was
// kept tonight the line says the skim is on top of it (#502: "$850 of
// the takings never made it back" beside "Gato kept $850 of it" read as
// the cut counted twice).
func skimLine(ev events.CrewSkimmed) string {
	if ev.FromWash == ev.Amount {
		return fmt.Sprintf("%s of the wash never came out clean. Somebody is cooking the books.", format.Money(ev.Amount))
	}
	s := fmt.Sprintf("%s of the takings never made it back", format.Money(ev.Amount))
	if ev.FromWash > 0 {
		s += fmt.Sprintf(", %s of it from the wash", format.Money(ev.FromWash))
	}
	if ev.Cuts > 0 {
		s += fmt.Sprintf(", on top of the %s the lieutenants kept as their cut", format.Money(ev.Cuts))
	}
	return s + ". Somebody is skimming."
}
