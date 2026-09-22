package news

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
)

// reportLaundering writes the laundering sim's events into the morning: the
// headlines and the report's lines. It says whether e was one of them.
func (r *reporter) reportLaundering(e events.Event) bool {
	w, rep, base := r.w, r.rep, r.base
	switch ev := e.(type) {
	case events.FrontBought:
		d := base
		d.Front = ev.Name
		r.add("laundering", "FrontBought", d)
		r.spent += ev.Cost
		rep.Money = append(rep.Money, fmt.Sprintf("Bought %s -%s. It opens today.", ev.Name, format.Money(ev.Cost)))
	case events.CashLaundered:
		line := fmt.Sprintf("Washed %s clean through %s", format.Money(ev.Amount), format.Plural(ev.Fronts, "front"))
		if ev.Upkeep > 0 {
			line += fmt.Sprintf(", upkeep -%s", format.Money(ev.Upkeep))
		}
		if ev.Earned > 0 {
			line += fmt.Sprintf(", the businesses earned +%s clean", format.Money(ev.Earned))
		}
		r.upkeep += ev.Upkeep
		r.earned += ev.Earned
		rep.Money = append(rep.Money, line)
	case events.FrontInvested:
		// Levels bought at a front (#192): bookkeeping, and the
		// growth is news only when it crosses the line (FrontGrew).
		r.invested += ev.Cost
		rep.Money = append(rep.Money, fmt.Sprintf("Invested %s clean in %s: level %d, earning %s/day clean", format.Money(ev.Cost), ev.Name, ev.Level, format.Money(ev.Income)))
	case events.Reserved:
		// Clean cash into the offshore account (#195): bookkeeping,
		// and a warning where the move was over the lot.
		r.reserved += ev.Amount + ev.Fee
		line := fmt.Sprintf("Moved %s clean offshore, fee -%s; the account holds %s", format.Money(ev.Amount), format.Money(ev.Fee), format.Money(w.Offshore))
		if ev.Lots > 0 {
			line += fmt.Sprintf(". Over the lot by %s: the DA will read it.", format.Plural(ev.Lots, "lot"))
		}
		rep.Money = append(rep.Money, line)
	case events.FrontGrew:
		d := base
		d.Front = ev.Name
		r.add("laundering", "FrontGrew", d)
		rep.Money = append(rep.Money, fmt.Sprintf("%s has grown enough to make the paper. The town wonders where the money came from.", ev.Name))
	case events.FrontAudited:
		d := base
		d.Front = ev.Name
		r.add("laundering", "FrontAudited", d)
		r.seized += ev.Seized
		line := fmt.Sprintf("AUDIT at %s: shut for %s", ev.Name, format.Plural(ev.Days, "day"))
		if ev.Seized > 0 {
			line += fmt.Sprintf(", %s seized", format.Money(ev.Seized))
		}
		if ev.Dial == events.LaunderGreedy {
			line += ". Run greedy, the books will interest the DA."
		} else {
			line += ". The books were clean enough."
		}
		rep.Money = append(rep.Money, line)
	case events.FrontFrozen:
		d := base
		d.Front = ev.Name
		r.add("laundering", "FrontFrozen", d)
		rep.Money = append(rep.Money, fmt.Sprintf("%s shut for %s: %s upkeep unpaid. Wash something.", ev.Name, format.Plural(ev.Days, "day"), format.Money(ev.Upkeep)))
	case events.AssetBought:
		// The assets (#48): a clean-cash purchase the paper notices,
		// off the assets' own stream, so no pinned run moves.
		d := r.at(ev.City)
		d.Asset = ev.Name
		r.addOff("assets:news", "laundering", "AssetBought", d)
		r.spent += ev.Cost
		rep.Money = append(rep.Money, fmt.Sprintf("Bought %s -%s clean. It stands from today, %s/day clean to keep.", ev.Name, format.Money(ev.Cost), format.Money(ev.Upkeep)))
	case events.AssetFrozen:
		rep.Money = append(rep.Money, fmt.Sprintf("%s stands idle for %s: %s upkeep unpaid. Wash something.", ev.Name, format.Plural(ev.Days, "day"), format.Money(ev.Upkeep)))
	default:
		return false
	}
	return true
}
