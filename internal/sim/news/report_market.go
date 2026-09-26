package news

import (
	"fmt"
	"math"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// reportMarket writes the market sim's events into the morning: the
// headlines and the report's lines. It says whether e was one of them.
func (r *reporter) reportMarket(e events.Event) bool {
	w, t, rep, here, base := r.w, r.t, r.rep, r.here, r.base
	switch ev := e.(type) {
	case events.UpgradeBought:
		if ev.Clean {
			r.book(game.FlowInvestments, 0, -ev.Cost)
		} else {
			r.book(game.FlowInvestments, -ev.Cost, 0)
		}
		d := base
		d.Name = ev.Name
		r.add("money", "UpgradeBought", d)
		pool := "dirty"
		if ev.Clean {
			pool = "clean"
		}
		rep.Upgrades = append(rep.Upgrades, fmt.Sprintf("%s bought for %s %s. It is yours for the run.", ev.Name, format.Money(ev.Cost), pool))
		rep.Money = append(rep.Money, fmt.Sprintf("%s -%s", ev.Name, format.Money(ev.Cost)))
	case events.PriceMove:
		// The street where you are; the market screen has the rest.
		if ev.City == here.ID {
			rep.Prices = append(rep.Prices, priceLine(w, ev))
		}
	case events.Unlocked:
		// A gate crossed (#148): one UNLOCKED line, first in the
		// report, and a headline under the unlock source, which
		// neither notoriety nor the law counts. A product's pick
		// stays on the home stream and a connect's on the
		// connects', where they were, so no pinned run moves; a
		// front's and a role's come off their own side stream.
		rep.Unlocked = append(rep.Unlocked, unlockLine(w, ev))
		d := r.at(ev.City)
		d.Name = ev.Name
		switch ev.Gate {
		case "product":
			d.Product = ev.Name
			r.add("unlock", "UnlockedProduct", d)
		case "connect":
			r.addOff(game.StreamSuppliers, "unlock", "UnlockedConnect", d)
		case "front":
			d.Front = ev.Name
			r.addOff(game.StreamUnlocks, "unlock", "UnlockedFront", d)
		case "role":
			d.Role = ev.ID
			r.addOff(game.StreamUnlocks, "unlock", "UnlockedRole", d)
		case "asset":
			d.Asset = ev.Name
			r.addOff(game.StreamAssetsNews, "unlock", "UnlockedAsset", d)
		}
	case events.PriceShock:
		d := r.at(ev.City)
		d.Product = w.ProductName(ev.Product)
		switch {
		case ev.Seized:
			r.add("market", "PriceShockSeized", d)
			rep.Prices = append(rep.Prices, fmt.Sprintf("%-8s %s%s for %s: the street was waiting on the shipment", w.ProductName(ev.Product), format.Times(ev.Factor, 1), r.in(ev.City), format.Plural(ev.Days, "day")))
		case ev.Slump:
			r.add("market", "PriceSlump", d)
		default:
			r.add("market", "PriceShock", d)
		}
	case events.PlayerSold:
		r.soldRevenue += ev.Revenue
		r.standingCut += ev.Cut
		r.book(game.FlowSales, ev.Revenue-ev.Cut, 0)
		rep.Sales = append(rep.Sales, saleLine(w, ev)+r.in(ev.City))
		// A standing order of yours that sold out with as much again left
		// in the stash is too small (#418): a playtest's 5 Heroin a night
		// stood seventy days beside a stash of 25 and a contract keeping 30.
		if left := w.Stock(ev.City, ev.Product); ev.Standing && !ev.Delegated && ev.Sold > 0 && ev.Sold >= ev.Wanted && left >= ev.Wanted {
			rep.Sales = append(rep.Sales, fmt.Sprintf("  the standing order sold all %d with %d more in the stash: raise it on the sell dialog or the cart", ev.Wanted, left))
		}
		d := r.at(ev.City)
		d.Product = w.ProductName(ev.Product)
		d.Qty = ev.Sold
		switch {
		case ev.Sold == 0:
			r.add("market", "PlayerSoldZero", d)
		case ev.Dial == events.DialAggressive || float64(ev.Sold) >= w.Demand(ev.City, ev.Product)*1.2:
			r.add("market", "PlayerSoldBig", d)
		}
	case events.ContractOffered:
		rep.Sales = append(rep.Sales, fmt.Sprintf("%s Answer it on the market screen (2)%s: it stands %s.", ev.Pitch, r.in(ev.City), format.Plural(ev.Expires-t.Day+1, "day")))
		d := r.at(ev.City)
		d.Product = w.ProductName(ev.Product)
		d.Name = ev.Name
		r.addBuyers("ContractOffered", d)
	case events.ContractAccepted:
		line := fmt.Sprintf("You took the order from %s: %d %s by day %d%s. Deliver it there (2, d).", ev.Name, ev.Units, w.ProductName(ev.Product), ev.Due, r.in(ev.City))
		// The market hands a lot over before it settles yesterday's
		// acceptances, so an order taken and delivered on one day would
		// read handed before it was taken (#465): the acceptance goes in
		// ahead of the order's first handoff.
		i, ok := r.handed[ev.ID]
		if !ok {
			rep.Sales = append(rep.Sales, line)
			break
		}
		rep.Sales = append(rep.Sales[:i], append([]string{line}, rep.Sales[i:]...)...)
		for id, j := range r.handed {
			if j >= i {
				r.handed[id] = j + 1
			}
		}
	case events.SupplyBought:
		// The contract's buy this morning (#113): the money line is
		// the receipt's, below, and this is the sales section's. A
		// lieutenant's contract (#174) is the CREW line's instead.
		if ev.Lieutenant == "" {
			rep.Sales = append(rep.Sales, fmt.Sprintf("Supply contract bought %d %s at %s to keep %d%s = -%s", ev.Units, w.ProductName(ev.Product), format.Price(ev.Price), ev.Level, r.in(ev.City), format.Money(ev.Cost)))
		}
	case events.StandingShort:
		if ev.All {
			rep.Sales = append(rep.Sales, fmt.Sprintf("Standing order for all the %s%s: nothing stashed, nothing sold.", w.ProductName(ev.Product), r.in(ev.City)))
			break
		}
		if ev.Stock == 0 {
			rep.Sales = append(rep.Sales, fmt.Sprintf("Standing order for %d %s%s: nothing stashed, nothing sold. Restock, or cancel it.", ev.Units, w.ProductName(ev.Product), r.in(ev.City)))
		} else {
			rep.Sales = append(rep.Sales, fmt.Sprintf("Standing order for %d %s%s: only %d stashed.", ev.Units, w.ProductName(ev.Product), r.in(ev.City), ev.Stock))
		}
	case events.SupplyShort:
		if ev.Why == events.SupplyRoad {
			// Held by the road (#503): the level counts what is on the
			// way, so the contract waits for it to land.
			rep.Sales = append(rep.Sales, fmt.Sprintf("Supply contract holding: %d %s on the road to %s.", ev.Short, w.ProductName(ev.Product), w.CityName(ev.City)))
			break
		}
		why := "there was no cash over the float for the rest"
		if ev.Why == "room" {
			why = "the stash there has no room for the rest"
		}
		if ev.Why == "supplier" {
			why = "nobody there sells it today"
		}
		// Short of cash, yours reads short enough for the report at 80
		// columns, and a line under it says what took the cash (#459):
		// a playtest's contracts ran short every morning, the why cut
		// off, on what the night before had washed.
		took := tookLine(w, ev)
		switch {
		case ev.Lieutenant != "":
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s could not keep %s stocked%s: %d under the level, %s.", ev.Lieutenant, w.ProductName(ev.Product), r.in(ev.City), ev.Short, why))
			if took != "" {
				rep.Crew = append(rep.Crew, took)
			}
		case ev.Why == "cash":
			rep.Sales = append(rep.Sales, fmt.Sprintf("Supply contract out of cash: %d %s short%s.", ev.Short, w.ProductName(ev.Product), r.in(ev.City)))
			if took != "" {
				rep.Sales = append(rep.Sales, took)
			}
		default:
			rep.Sales = append(rep.Sales, fmt.Sprintf("Supply contract short: %d %s under the level%s, %s.", ev.Short, w.ProductName(ev.Product), r.in(ev.City), why))
		}
	case events.CreditTaken:
		rep.Money = append(rep.Money, fmt.Sprintf("%s put %s on your book%s: you owe them %s, due day %d.", ev.Name, format.Money(ev.Amount), r.in(ev.City), format.Money(ev.Debt), ev.Due))
	case events.DebtPaid:
		r.book(game.FlowPurchases, -(ev.Amount - ev.Clean), -ev.Clean)
		rep.Money = append(rep.Money, fmt.Sprintf("Paid %s the %s you owed, on the day. -%s", ev.Name, format.Money(ev.Amount), format.Money(ev.Amount)))
	case events.DebtLate:
		r.book(game.FlowPurchases, -(ev.Paid - ev.Clean), -ev.Clean)
		d := r.at(ev.City)
		d.Name = ev.Name
		r.addSuppliers("DebtLate", d)
		line := fmt.Sprintf("LATE: you owed %s %s and could pay %s.", ev.Name, format.Money(ev.Owed), format.Money(ev.Paid))
		switch ev.What {
		case "extended":
			line += fmt.Sprintf(" They let it ride once: %s due again day %d. They remember.", format.Money(ev.Left), ev.Due)
		case "frozen":
			line += fmt.Sprintf(" They are not taking your calls; %s due day %d.", format.Money(ev.Left), ev.Due)
			if ev.Fee > 0 {
				line += fmt.Sprintf(" A fee of %s went on the book.", format.Money(ev.Fee))
			}
		case "collected":
			line += fmt.Sprintf(" They sent somebody. %s due day %d.", format.Money(ev.Left), ev.Due)
		}
		rep.Money = append(rep.Money, line)
	case events.SupplierFrozen:
		d := r.at(ev.City)
		d.Name = ev.Name
		r.addSuppliers("SupplierFrozen", d)
		if ev.Why == "floor" {
			rep.Sales = append(rep.Sales, fmt.Sprintf("%s has stopped taking your calls%s: %s, and nothing sells from them until then. Buy from somebody else.", ev.Name, r.in(ev.City), format.Plural(ev.Days, "day")))
		}
	case events.SupplierWarned:
		d := r.at(ev.City)
		d.Name, d.Product = ev.Name, w.ProductName(ev.Product)
		r.addSuppliers("SupplierWarned", d)
		if ev.Slump {
			rep.Sales = append(rep.Sales, fmt.Sprintf("%s says the street%s is going quiet on %s tomorrow. Sell tonight.", ev.Name, r.in(ev.City), w.ProductName(ev.Product)))
		} else {
			rep.Sales = append(rep.Sales, fmt.Sprintf("%s says %s is about to jump%s tomorrow. Stock up.", ev.Name, w.ProductName(ev.Product), r.in(ev.City)))
		}
	case events.SupplierCollected:
		d := r.at(ev.City)
		d.Name = ev.Name
		r.addSuppliers("SupplierCollected", d)
		switch {
		case ev.Member != 0 && ev.Hurt:
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s's people found %s over the %s you owe: a beating, loyalty -%.0f, off the corner.", ev.Name, ev.MemberName, format.Money(ev.Owed), ev.Loyalty))
		case ev.Member != 0:
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s's people had a word with %s over the %s you owe: loyalty -%.0f. They stood their ground.", ev.Name, ev.MemberName, format.Money(ev.Owed), ev.Loyalty))
		case ev.Units > 0:
			rep.Money = append(rep.Money, fmt.Sprintf("%s's people took %d %s from the stash%s against the %s you owe.", ev.Name, ev.Units, w.ProductName(ev.Product), r.in(ev.City), format.Money(ev.Owed)))
		default:
			rep.Money = append(rep.Money, fmt.Sprintf("%s's people came for the %s you owe and found nothing to take. They will be back.", ev.Name, format.Money(ev.Owed)))
		}
	case events.HandoffHeld:
		// Lying low held it (#503): said, never dropped silently.
		rep.Sales = append(rep.Sales, fmt.Sprintf("Handoff held, lying low: %d %s for %s%s.", ev.Units, w.ProductName(ev.Product), ev.Name, r.in(ev.City)),
			fmt.Sprintf("  %d still owed by day %d: queue it again", ev.Owed, ev.Due))
	case events.ContractDelivered:
		r.book(game.FlowSales, ev.Revenue, 0)
		line := fmt.Sprintf("Handed %d %s to %s at %s (%s street) = +%s%s", ev.Units, w.ProductName(ev.Product), ev.Name, format.Price(ev.Price), format.TimesSig(ev.Price/math.Max(ev.Street, 1e-9), 3), format.Money(ev.Revenue), r.in(ev.City))
		if ev.Complete {
			line += ". Delivered in full."
		} else {
			line += fmt.Sprintf(". %d still owed.", ev.Owed)
		}
		if ev.Price < ev.Signed {
			line += fmt.Sprintf(" The street was %s the day they asked.", format.Price(ev.Signed))
		}
		if r.handed == nil {
			r.handed = map[int]int{}
		}
		if _, ok := r.handed[ev.ID]; !ok {
			r.handed[ev.ID] = len(rep.Sales)
		}
		rep.Sales = append(rep.Sales, line)
		rep.Money = append(rep.Money, fmt.Sprintf("%s paid +%s", ev.Name, format.Money(ev.Revenue)))
		if ev.Complete {
			d := r.at(ev.City)
			d.Product = w.ProductName(ev.Product)
			d.Name = ev.Name
			d.Qty = ev.Total
			r.addBuyers("ContractDelivered", d)
		}
	case events.ContractFailed:
		r.book(game.FlowLosses, -(ev.Cash - ev.Clean), -ev.Clean)
		line := fmt.Sprintf("You let %s down: %d of %d %s delivered by the day%s.", ev.Name, ev.Delivered, ev.Units, w.ProductName(ev.Product), r.in(ev.City))
		if ev.Cash > 0 {
			line += fmt.Sprintf(" They took %s for the rest.", format.Money(ev.Cash))
			rep.Money = append(rep.Money, fmt.Sprintf("%s collected for the shortfall -%s", ev.Name, format.Money(ev.Cash)))
		}
		line += " Word gets round."
		rep.Sales = append(rep.Sales, line)
		d := r.at(ev.City)
		d.Product = w.ProductName(ev.Product)
		d.Name = ev.Name
		r.addBuyers("ContractFailed", d)
	case events.ContractExpired:
		rep.Sales = append(rep.Sales, fmt.Sprintf("The offer from %s lapsed: %d %s nobody answered for%s.", ev.Name, ev.Units, w.ProductName(ev.Product), r.in(ev.City)))
	case events.PlayerUndercut:
		// The price war (#68): one line per corner and product, in
		// SALES, since the units are part of the night's sale.
		rep.Sales = append(rep.Sales, fmt.Sprintf("Undercut %s on %s: %d %s cheap = +%s, %s of their trade there", cornerHolder(w, ev.Corner), ev.Name, ev.Units, w.ProductName(ev.Product), format.Money(ev.Revenue), format.Pct(ev.Share, 0)))
	case events.Overdose:
		// The city's story (#47): a headline naming the corner, off
		// the overdoses' own stream, and a LAW line, since the
		// pressure is what it costs you; never a page.
		d := r.at(ev.City)
		d.Product = w.ProductName(ev.Product)
		d.Corner = ev.CornerName
		if d.Corner == "" {
			d.Corner = "a " + d.City + " corner"
		}
		r.addOff(game.StreamOverdoseNews, "overdose", "Overdose", d)
		where := ev.CornerName
		if where == "" {
			where = "your corners"
		}
		rep.Law = append(rep.Law, fmt.Sprintf("OVERDOSE on %s%s: somebody went down on your %s (quality %.0f). The city is talking, the DA is listening.", where, r.in(ev.City), w.ProductName(ev.Product), ev.Quality))
	case events.StockCut:
		r.book(game.FlowPurchases, -ev.Cost, 0)
		hand := ""
		if ev.Chemist != "" {
			hand = ", " + ev.Chemist + "'s hand on it"
		}
		rep.Sales = append(rep.Sales, fmt.Sprintf("Cut %d %s into %d%s: quality %.0f → %.0f%s = -%s", ev.Units, w.ProductName(ev.Product), ev.Units+ev.Added, r.in(ev.City), ev.From, ev.To, hand, format.Money(ev.Cost)))
		rep.Money = append(rep.Money, fmt.Sprintf("Cutting %s -%s", w.ProductName(ev.Product), format.Money(ev.Cost)))
	default:
		return false
	}
	return true
}

// tookLine is what took a contract's cash (#459), the line under its
// short: the wash last night and the till it left, or, where the day
// spent the till down after the wash, the till the contract bought on
// (#502: "the wash took $2,000 last night and left the till $168,479" on
// a morning the player had spent down to $18K). "" when the wash took
// nothing or the short was not cash.
func tookLine(w *game.World, ev events.SupplyShort) string {
	n := len(w.Flows)
	if ev.Why != "cash" || n == 0 || w.Flows[n-1].Line(game.FlowLaundering).Dirty >= 0 {
		return ""
	}
	last := w.Flows[n-1]
	if ev.Till < last.Closing.Dirty {
		return fmt.Sprintf("  the till was down to %s when it bought", format.Money(ev.Till))
	}
	return fmt.Sprintf("  the wash took %s last night and left the till %s", format.Money(-last.Line(game.FlowLaundering).Dirty), format.Money(last.Closing.Dirty))
}
