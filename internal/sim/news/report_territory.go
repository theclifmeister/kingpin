package news

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// reportTerritory writes the territory sim's events into the morning: the
// headlines and the report's lines. It says whether e was one of them.
func (r *reporter) reportTerritory(e events.Event) bool {
	w, rep := r.w, r.rep
	switch ev := e.(type) {
	case events.CornerClaimed:
		d := r.at(r.cornerCity(ev.Corner))
		d.Corner, d.Name = ev.Name, ev.Worker
		r.add("territory", "CornerClaimed", d)
		switch ev.Worker {
		case "you":
			rep.Territory = append(rep.Territory, fmt.Sprintf("You took %s.", ev.Name))
		case "nobody":
			rep.Territory = append(rep.Territory, fmt.Sprintf("You took %s, but nobody is working it.", ev.Name))
		default:
			rep.Territory = append(rep.Territory, fmt.Sprintf("You took %s; %s is working it.", ev.Name, ev.Worker))
		}
	case events.CornerLost:
		d := r.at(r.cornerCity(ev.Corner))
		d.Corner = ev.Name
		switch ev.Reason {
		case "crackdown":
			r.add("rivals", "CornerCrackdown", d)
			if ev.Owner == game.OwnerRival {
				d = r.crew(d, w.FactionName(ev.Faction))
				rep.Territory = append(rep.Territory, fmt.Sprintf("Police cleared %s: %s lost it.", ev.Name, w.FactionName(ev.Faction)))
			} else {
				rep.Territory = append(rep.Territory, fmt.Sprintf("Police cleared %s: you lost it.", ev.Name))
			}
		default:
			r.add("territory", "CornerLost", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s went back to the street: nobody was working it.", ev.Name))
		}
	case events.CornerRobbed:
		d := r.at(r.cornerCity(ev.Corner))
		d.Corner = ev.Name
		r.add("territory", "CornerRobbed", d)
		rep.Territory = append(rep.Territory, robberyLine(w, ev))
		r.book(game.FlowLosses, -ev.Cash, 0)
		if ev.Cash > 0 {
			rep.Money = append(rep.Money, fmt.Sprintf("Robbed on %s in %s -%s", ev.Name, w.CityName(r.cornerCity(ev.Corner)), format.Money(ev.Cash)))
		}
	// The stash houses (#73).
	case events.HouseBought:
		d := r.at(ev.City)
		d.Name = ev.Name
		r.addHouses("territory", "HouseBought", d)
		r.book(game.FlowInvestments, -ev.Price, 0)
		rep.Money = append(rep.Money, fmt.Sprintf("Took the lease on %s -%s. Rent %s/day clean from tomorrow.", ev.Name, format.Money(ev.Price), format.Money(ev.Rent)))
	case events.HouseRobbed:
		d := r.at(ev.City)
		d.Name, d.Corner = ev.Name, ev.Corner
		r.addHouses("territory", "HouseRobbed", d)
		rep.Territory = append(rep.Territory, houseRobbedLine(w, ev))
	case events.HouseCompromised:
		why := map[string]string{"informant": "somebody on the payroll told them", "robbery": "word got out after the robbery", "bust": "they were inside"}[ev.Why]
		rep.Heat = append(rep.Heat, fmt.Sprintf("The police know about %s%s: %s. It is the one the raid finds; move the stock and drop it.", ev.Name, r.in(ev.City), why))
	case events.HouseLost:
		d := r.at(ev.City)
		d.Name = ev.Name
		r.addHouses("territory", "HouseLost", d)
		rep.Territory = append(rep.Territory, fmt.Sprintf("The landlord threw you out of %s%s: %s gone with it. The rent went unpaid.", ev.Name, r.in(ev.City), format.Plural(ev.Units, "unit")))
	case events.RentPaid:
		r.book(game.FlowRoutes, 0, -ev.Amount)
		if ev.Amount > 0 {
			rep.Money = append(rep.Money, fmt.Sprintf("Rent on %s -%s clean", format.Plural(ev.Houses, "house"), format.Money(ev.Amount)))
		}
		if len(ev.Unpaid) > 0 {
			rep.Territory = append(rep.Territory, fmt.Sprintf("Rent unpaid at %s: no clean cash. The landlord will not wait long.", strings.Join(ev.Unpaid, ", ")))
		}
	// The property (#194). DeedBought and DeedRent are bookkeeping;
	// the purchase past the line and the forfeiture make the paper
	// under the laundering source (a story about your money, as a
	// front's growth is: notoriety, and pressure where you are),
	// off the deeds' own stream: a run with no deed is the run it
	// was.
	case events.DeedBought:
		r.book(game.FlowInvestments, 0, -ev.Price)
		rep.Money = append(rep.Money, fmt.Sprintf("Bought the block %s is on%s -%s clean. It pays %s/day clean.", ev.Name, r.in(ev.City), format.Money(ev.Price), format.Money(ev.Rent)))
	case events.DeedsBought:
		d := r.at(ev.City)
		d.Corner, d.Qty = ev.Name, ev.Count
		r.addOff(game.StreamDeedsNews, "laundering", "DeedsBought", d)
		rep.Law = append(rep.Law, fmt.Sprintf("%s makes the paper: %s in your name now. The town wonders where the money came from.", ev.Name, format.Plural(ev.Count, "block")))
	case events.DeedRent:
		r.book(game.FlowInvestments, 0, ev.Amount)
		rep.Money = append(rep.Money, fmt.Sprintf("Rent from %s +%s clean", format.Plural(ev.Deeds, "block"), format.Money(ev.Amount)))
	case events.Taxed:
		// The tax (#231): the free corners of a city you hold paying
		// for the right to work them.
		r.book(game.FlowTax, ev.Amount, 0)
		rep.Money = append(rep.Money, fmt.Sprintf("The tax: %s%s +%s", format.Plural(ev.Corners, "free corner"), r.in(ev.City), format.Money(ev.Amount)))
	default:
		return false
	}
	return true
}
