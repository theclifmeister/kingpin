package news

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// reportHeat writes the heat sim's events into the morning: the
// headlines and the report's lines. It says whether e was one of them.
func (r *reporter) reportHeat(e events.Event) bool {
	w, rep, here, base := r.w, r.rep, r.here, r.base
	switch ev := e.(type) {
	case events.FallGuyBurned:
		r.add("heat", "FallGuyBurned", base)
		r.book(game.FlowLosses, -(ev.CashLost - ev.Clean), -ev.Clean)
		if ev.CashLost > 0 {
			rep.Money = append(rep.Money, fmt.Sprintf("The fall guy's price -%s", format.Money(ev.CashLost)))
		}
		rep.Heat = append(rep.Heat, fmt.Sprintf("THE FALL GUY TOOK IT. The case is closed and heat is down, but %s went on making it stick. There is no second one.", format.Money(ev.CashLost)))
	case events.HeatChanged:
		if ev.City != here.ID && len(ev.Reasons) == 0 && ev.To < 1 {
			break // a city nothing happened in
		}
		rep.Heat = append(rep.Heat, fmt.Sprintf("%s heat %.0f %s %.0f", w.CityName(ev.City), ev.From, format.Arrow, ev.To))
		for _, r := range ev.Reasons {
			rep.Heat = append(rep.Heat, "  "+r)
		}
		if ev.To >= 25 && ev.From < 25 {
			r.add("heat", "HeatWarning", r.at(ev.City))
		}
	case events.Enforcement:
		d := r.at(ev.City)
		d.Level = ev.Level
		r.add("heat", "Enforcement"+capitalize(ev.Level), d)
		rep.Heat = append(rep.Heat, enforcementLine(w, ev)+r.in(ev.City))
		if ev.Stash && ev.House != "" {
			rep.Heat = append(rep.Heat, fmt.Sprintf("  they went straight to %s and emptied it. Somebody told them where.", ev.HouseName))
		} else if ev.Stash {
			rep.Heat = append(rep.Heat, "  they went straight to the stash. Somebody told them where.")
		}
		if ev.Level == content.Sting || ev.Level == content.Raid || ev.Level == content.TaskForce {
			if ev.Evidence > 0 {
				rep.Heat = append(rep.Heat, fmt.Sprintf("  the DA's file on you grows (%d)", w.Heat.Evidence))
			} else {
				rep.Heat = append(rep.Heat, "  they found nothing to hang on you")
			}
		}
		r.book(game.FlowLosses, -ev.CashLost, 0)
		if ev.CashLost > 0 {
			rep.Money = append(rep.Money, seizedLine(w, ev))
		}
	case events.LaidLow:
		r.add("heat", "LaidLow", base)
	case events.RaidFellThrough:
		// The favour (#228): the response that did not come.
		d := r.at(ev.City)
		d.Level = ev.Level
		r.add("law", "RaidFellThrough", d)
		word := ev.Level
		if word == content.TaskForce {
			word = "task force"
		}
		rep.Heat = append(rep.Heat, fmt.Sprintf("The %s%s fell through: Chief %s's people stood down at the last minute. Nothing taken, nothing cooled, and the file grows by %d: the chief's name is in your ledger now.", word, r.in(ev.City), w.Law.Chief.Name, ev.Evidence))
	case events.HouseRaided:
		d := r.at(ev.City)
		d.Name, d.Level = ev.Name, ev.Level
		r.addHouses("heat", "HouseRaided", d)
	case events.TaskForceFormed:
		d := r.at(ev.City)
		r.addOff(game.StreamAssetsNews, "heat", "TaskForceFormed", d)
		line := fmt.Sprintf("A TASK FORCE has formed%s. It comes tomorrow night", r.in(ev.City))
		if ev.Assets > 0 {
			line += " and it will take an asset with it"
		}
		rep.Heat = append(rep.Heat, line+". Lie low: what they find on a quiet night is not a case.")
	case events.AssetSeized:
		d := r.at(ev.City)
		d.Asset = ev.Name
		r.addOff(game.StreamAssetsNews, "heat", "AssetSeized", d)
		rep.Heat = append(rep.Heat, fmt.Sprintf("  they took %s: %s of yours, gone.", ev.Name, format.Money(ev.Cost)))
	case events.StockMoved:
		rep.Territory = append(rep.Territory, fmt.Sprintf("Moved %d %s from %s to %s%s.", ev.Units, w.ProductName(ev.Product), ev.From, ev.To, r.in(ev.City)))
	default:
		return false
	}
	return true
}
