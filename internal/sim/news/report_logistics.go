package news

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// reportLogistics writes the logistics sim's events into the morning: the
// headlines and the report's lines. It says whether e was one of them.
func (r *reporter) reportLogistics(e events.Event) bool {
	w, rep, base := r.w, r.rep, r.base
	switch ev := e.(type) {
	case events.WholesaleBought:
		// The route's lots, bought this morning for what it sends.
		r.book(game.FlowRoutes, -ev.Cost, 0)
		r.charge(ev.Name, ev.Cost)
		rep.Shipments = append(rep.Shipments, fmt.Sprintf("Bought %d %s (%s) in %s for %s -%s", ev.Units, w.ProductName(ev.Product), format.Plural(ev.Lots, "lot"), w.CityName(ev.City), format.The(ev.Name), format.Money(ev.Cost)))
	case events.ShipmentSent:
		// Paid this morning, when the route put it on the road.
		r.book(game.FlowRoutes, -ev.Cost, 0)
		r.charge(ev.Name, ev.Cost)
		rep.Shipments = append(rep.Shipments, fmt.Sprintf("%d %s left %s for %s by %s, %s: %s, fare -%s", ev.Units, w.ProductName(ev.Product), w.CityName(ev.From), w.CityName(ev.To), ev.Mode, ev.Dial, format.Plural(ev.Days, "day"), format.Money(ev.Cost)))
	case events.RouteIdle:
		// Short and nothing sent (#459): a playtest's routes read
		// "shipped 0" for days and the report said nothing. Each line
		// fits the report at 80 columns: the modal cuts, never wraps.
		switch ev.Why {
		case events.IdleTill:
			if over := ev.Dirty - ev.Till; over > 0 {
				// Over the till, but not by a lot and its fare.
				rep.Shipments = append(rep.Shipments, fmt.Sprintf("%s idle: only %s over the %s till, short of a lot", ev.Name, format.Money(over), format.Money(ev.Till)))
			} else {
				rep.Shipments = append(rep.Shipments, fmt.Sprintf("%s idle: %s dirty, none over the %s till", ev.Name, format.Money(ev.Dirty), format.Money(ev.Till)))
			}
		case events.IdleStock:
			what := "nothing it is short of"
			if len(ev.Products) <= 2 {
				var names []string
				for _, p := range ev.Products {
					names = append(names, w.ProductName(p))
				}
				what = "no " + strings.Join(names, " or ")
			}
			rep.Shipments = append(rep.Shipments, fmt.Sprintf("%s idle: %s in the %s stash", ev.Name, what, w.CityName(ev.From)))
		}
	case events.ShipmentArrived:
		rep.Shipments = append(rep.Shipments, fmt.Sprintf("%d %s landed in %s from %s by %s", ev.Units, w.ProductName(ev.Product), w.CityName(ev.To), w.CityName(ev.From), ev.Mode))
	case events.ShipmentSeized:
		d := r.at(ev.To)
		d.Product, d.Qty, d.Mode, d.From, d.To = w.ProductName(ev.Product), ev.Units, ev.Mode, w.CityName(ev.From), w.CityName(ev.To)
		r.add("logistics", "ShipmentSeized", d)
		line := fmt.Sprintf("SEIZED on the road: %d %s bound for %s by %s, sent %s, every unit gone. Heat in both cities.", ev.Units, w.ProductName(ev.Product), w.CityName(ev.To), ev.Mode, ev.Dial)
		if ev.Dial == events.ShipFast {
			line += " Sent fast, it was asking to be looked at: the DA's file grows."
		}
		if ev.DriverName != "" {
			line += " " + ev.DriverName + " was driving."
		}
		rep.Shipments = append(rep.Shipments, line)
		if w.Stats.Seizures == 1 {
			rep.Shipments = append(rep.Shipments, "The first one is the cue: a hot road wants the dial turned down (map, r), and the route sends what it lost again tomorrow.")
		}
	case events.CheckpointBought:
		r.book(game.FlowRoutes, -ev.Cost, 0)
		what := "checkpoint"
		if ev.Mode == "boat" || ev.Mode == "plane" {
			what = "customs agent"
		}
		rep.Shipments = append(rep.Shipments, fmt.Sprintf("The %s on %s is yours until day %d: the risk on that edge is cut while it holds.", what, format.The(ev.Name), ev.Until))
		rep.Money = append(rep.Money, fmt.Sprintf("The %s on %s -%s", what, format.The(ev.Name), format.Money(ev.Cost)))
	case events.TunnelFound:
		d := base
		d.Asset, d.Route, d.Product = ev.Name, ev.Name, w.ProductName(ev.Product)
		r.addOff(game.StreamAssetsNews, "heat", "TunnelFound", d)
		rep.Shipments = append(rep.Shipments, fmt.Sprintf("THE TUNNEL IS FOUND: %d %s taken in it, and it is shut for good.", ev.Units, w.ProductName(ev.Product)))
	// The export lanes (#391): the load is paid for off the book the
	// night it leaves and pays on landing; a seizure loses what it cost.
	case events.ExportShipped:
		r.book(game.FlowPurchases, -ev.Cost, 0)
		rep.Shipments = append(rep.Shipments, fmt.Sprintf("%d %s left %s on %s, bought off the book for -%s: %s a unit abroad, landing day %d", ev.Units, w.ProductName(ev.Product), w.CityName(ev.City), format.The(ev.Name), format.Money(ev.Cost), format.Price(ev.Price), ev.Lands))
		rep.Money = append(rep.Money, fmt.Sprintf("The %s's load off the book -%s", ev.Name, format.Money(ev.Cost)))
	case events.ExportLanded:
		r.book(game.FlowSales, ev.Revenue, 0)
		if w.Stats.ExportUnits == ev.Units {
			// The first load paid is news: the city learns its size.
			d := r.at(ev.City)
			d.Product, d.Qty, d.Mode, d.Route = w.ProductName(ev.Product), ev.Units, ev.Mode, ev.Name
			r.addOff(game.StreamExportsNews, "logistics", "ExportLanded", d)
		}
		rep.Shipments = append(rep.Shipments, fmt.Sprintf("The %s landed %d %s abroad: +%s dirty on %s it cost.", ev.Name, ev.Units, w.ProductName(ev.Product), format.Money(ev.Revenue), format.Money(ev.Cost)))
		rep.Money = append(rep.Money, fmt.Sprintf("The %s landed abroad %s", ev.Name, format.Signed(ev.Revenue)))
	case events.ExportSeized:
		d := r.at(ev.City)
		d.Product, d.Qty, d.Mode, d.Route = w.ProductName(ev.Product), ev.Units, ev.Mode, ev.Name
		r.addOff(game.StreamExportsNews, "logistics", "ExportSeized", d)
		line := fmt.Sprintf("SEIZED abroad: the %s's %d %s, and the %s it cost, gone.", ev.Name, ev.Units, w.ProductName(ev.Product), format.Money(ev.Cost))
		if ev.Watched {
			line += " The feds were watching: every lane runs hot while they do."
		}
		rep.Shipments = append(rep.Shipments, line)
	// Intel (#45): the facts filed tonight are the report's INTEL
	// section; a spy going under, one found and a lie that bit are
	// news, their templates picked off the intel side stream.
	case events.IntelGained:
		rep.Intel = append(rep.Intel, intelLine(w, ev))
	case events.IntelFalse:
		d := base
		d = r.crew(d, ev.Rival)
		switch ev.FactKind {
		case game.FactRisk:
			d.Route = ev.Name
			r.addIntel("rivals", "IntelFalseRoute", d)
			rep.Intel = append(rep.Intel, fmt.Sprintf("The word on %s was %s's: they had customs waiting. The intel screen (9) names them now.", ev.Name, ev.Rival))
		default:
			d.Corner = ev.Name
			r.addIntel("rivals", "IntelFalseStash", d)
			rep.Intel = append(rep.Intel, fmt.Sprintf("The till on %s was empty: the word was %s's. The intel screen (9) names them now.", ev.Name, ev.Rival))
		}
	default:
		return false
	}
	return true
}
