// Package news turns the day's events into headlines and the morning
// report, and deals the dilemma cards. It runs last so it sees everything
// the other sims emitted.
package news

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the news simulation.
type Sim struct {
	cfg  content.HeadlinesConfig
	dcfg content.DilemmasConfig
	tmpl map[string][]*template.Template
	flav []*template.Template
	deck []card
}

// New parses the headline and card templates once. It refuses a deck
// whose choices use an effect key the world does not apply.
func New(cfg content.HeadlinesConfig, dilemmas content.DilemmasConfig) (*Sim, error) {
	deck, err := parseDeck(dilemmas)
	if err != nil {
		return nil, fmt.Errorf("dilemmas: %w", err)
	}
	s := &Sim{cfg: cfg, dcfg: dilemmas, tmpl: map[string][]*template.Template{}, deck: deck}
	for key, list := range cfg.Templates {
		for i, src := range list {
			t, err := template.New(fmt.Sprintf("%s#%d", key, i)).Parse(src)
			if err != nil {
				return nil, fmt.Errorf("headline %s[%d]: %w", key, i, err)
			}
			s.tmpl[key] = append(s.tmpl[key], t)
		}
	}
	for i, src := range cfg.Flavour {
		t, err := template.New(fmt.Sprintf("flavour#%d", i)).Parse(src)
		if err != nil {
			return nil, fmt.Errorf("flavour[%d]: %w", i, err)
		}
		s.flav = append(s.flav, t)
	}
	return s, nil
}

func (s *Sim) Name() string { return "news" }

// HasTemplate reports whether a template key exists; tests use it to make
// sure every event kind the sims emit can be reported.
func (s *Sim) HasTemplate(key string) bool { return len(s.tmpl[key]) > 0 }

// Keys returns every template key.
func (s *Sim) Keys() []string {
	out := make([]string, 0, len(s.tmpl))
	for k := range s.tmpl {
		out = append(out, k)
	}
	return out
}

type data struct {
	City    string
	Product string
	Qty     int
	Level   string
	Name    string
	Role    string
	Corner  string
	Rival   string
	Front   string
}

// Step writes headlines into the journal and assembles the morning report.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	rep := &game.DayReport{Day: t.Day}
	var lines []game.Headline

	add := func(source, key string, d data) {
		list := s.tmpl[key]
		if len(list) == 0 {
			return
		}
		txt := render(list[t.RNG.IntN(len(list))], d)
		lines = append(lines, game.Headline{Day: t.Day, Source: source, Text: txt})
	}
	base := data{City: w.City}

	// Money before we look at events: sales are already applied by market.
	var soldRevenue, lostCash, spent, wages, skimmed, robbed, upgrades, upkeep, seized, paidOff, investigated int
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.UpgradeBought:
			upgrades += ev.Cost
			d := base
			d.Name = ev.Name
			add("money", "UpgradeBought", d)
			pool := "dirty"
			if ev.Clean {
				pool = "clean"
			}
			rep.Upgrades = append(rep.Upgrades, fmt.Sprintf("%s bought for $%d %s. It is yours for the run.", ev.Name, ev.Cost, pool))
			rep.Money = append(rep.Money, fmt.Sprintf("%s -$%d", ev.Name, ev.Cost))
		case events.FallGuyBurned:
			add("heat", "FallGuyBurned", base)
			lostCash += ev.CashLost
			rep.Heat = append(rep.Heat, fmt.Sprintf("THE FALL GUY TOOK IT. The case is closed and heat is down, but $%d went on making it stick. There is no second one.", ev.CashLost))
		case events.PriceMove:
			rep.Prices = append(rep.Prices, priceLine(w, ev))
		case events.ProductUnlocked:
			d := base
			d.Product = ev.Name
			add("market", "ProductUnlocked", d)
			rep.Prices = append(rep.Prices, fmt.Sprintf("%-8s now on offer from the supplier, around %s a unit", ev.Name, dollars(ev.Price)))
		case events.PriceShock:
			d := base
			d.Product = w.ProductName(ev.Product)
			if ev.Slump {
				add("market", "PriceSlump", d)
			} else {
				add("market", "PriceShock", d)
			}
		case events.PlayerSold:
			soldRevenue += ev.Revenue
			rep.Sales = append(rep.Sales, saleLine(w, ev))
			d := base
			d.Product = w.ProductName(ev.Product)
			d.Qty = ev.Sold
			switch {
			case ev.Sold == 0:
				add("market", "PlayerSoldZero", d)
			case ev.Dial == events.DialAggressive || float64(ev.Sold) >= w.Demand(ev.Product)*1.2:
				add("market", "PlayerSoldBig", d)
			}
		case events.HeatChanged:
			rep.Heat = append(rep.Heat, fmt.Sprintf("Heat %.0f -> %.0f", ev.From, ev.To))
			for _, r := range ev.Reasons {
				rep.Heat = append(rep.Heat, "  "+r)
			}
			if ev.To >= 25 && ev.From < 25 {
				add("heat", "HeatWarning", base)
			}
		case events.Enforcement:
			d := base
			d.Level = ev.Level
			add("heat", "Enforcement"+capitalize(ev.Level), d)
			rep.Heat = append(rep.Heat, enforcementLine(w, ev))
			if ev.Stash {
				rep.Heat = append(rep.Heat, "  they went straight to the stash. Somebody told them where.")
			}
			if ev.Level == "sting" || ev.Level == "raid" {
				if ev.Evidence > 0 {
					rep.Heat = append(rep.Heat, fmt.Sprintf("  the DA's file on you grows (%d)", w.Heat.Evidence))
				} else {
					rep.Heat = append(rep.Heat, "  they found nothing to hang on you")
				}
			}
			lostCash += ev.CashLost
		case events.LaidLow:
			add("heat", "LaidLow", base)
		case events.CrewHired:
			d := base
			d.Name, d.Role = ev.Name, ev.Role
			add("crew", "CrewHired", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s signed on as a %s for $%d", ev.Name, ev.Role, ev.Fee))
		case events.CrewFired:
			d := base
			d.Name, d.Role = ev.Name, ev.Role
			if ev.Informant {
				add("crew", "CrewFiredInformant", d)
				rep.Crew = append(rep.Crew, fmt.Sprintf("You let %s go. Word is they had been talking to the police. Nobody mourned.", ev.Name))
			} else {
				add("crew", "CrewFired", d)
				rep.Crew = append(rep.Crew, fmt.Sprintf("You let %s go. The others noticed.", ev.Name))
			}
		case events.CrewDefected:
			d := base
			d.Name, d.Role, d.Rival, d.Corner = ev.Name, ev.Role, ev.Rival, ev.CornerName
			add("crew", "CrewDefected", d)
			if ev.Corner != "" {
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s went over to %s, and they know %s. Expect trouble there.", ev.Name, ev.Rival, ev.CornerName))
			} else {
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s went over to %s.", ev.Name, ev.Rival))
			}
		case events.InvestigationRun:
			investigated += ev.Cost
			add("crew", "InvestigationRun", base)
			if ev.Found {
				rep.Crew = append(rep.Crew, fmt.Sprintf("The investigation named %s: they have been talking to the police. Fire them (f) and the file stops growing.", ev.Name))
			} else {
				rep.Crew = append(rep.Crew, "The investigation named nobody. The crew resent being asked.")
			}
			rep.Money = append(rep.Money, fmt.Sprintf("Investigation -$%d", ev.Cost))
		case events.CrewPaidOff:
			paidOff += ev.Cost
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s took your money and stays sweet on you, for now.", ev.Name))
			rep.Money = append(rep.Money, fmt.Sprintf("Paid off %s -$%d", ev.Name, ev.Cost))
		case events.CrewQuit:
			d := base
			d.Name, d.Role = ev.Name, ev.Role
			add("crew", "CrewQuit", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s walked. Nobody was surprised.", ev.Name))
		case events.CrewSkimmed:
			add("crew", "CrewSkimmed", base)
			skimmed += ev.Amount
			switch {
			case ev.FromWash == ev.Amount:
				rep.Crew = append(rep.Crew, fmt.Sprintf("$%d of the wash never came out clean. Somebody is cooking the books.", ev.Amount))
			case ev.FromWash > 0:
				rep.Crew = append(rep.Crew, fmt.Sprintf("$%d of the takings never made it back, $%d of it from the wash. Somebody is skimming.", ev.Amount, ev.FromWash))
			default:
				rep.Crew = append(rep.Crew, fmt.Sprintf("$%d of the takings never made it back. Somebody is skimming.", ev.Amount))
			}
		case events.CornerClaimed:
			d := base
			d.Corner, d.Name = ev.Name, ev.Worker
			add("territory", "CornerClaimed", d)
			switch ev.Worker {
			case "you":
				rep.Territory = append(rep.Territory, fmt.Sprintf("You took %s.", ev.Name))
			case "nobody":
				rep.Territory = append(rep.Territory, fmt.Sprintf("You took %s, but nobody is working it.", ev.Name))
			default:
				rep.Territory = append(rep.Territory, fmt.Sprintf("You took %s; %s is working it.", ev.Name, ev.Worker))
			}
		case events.CornerLost:
			d := base
			d.Corner = ev.Name
			switch ev.Reason {
			case "crackdown":
				add("rivals", "CornerCrackdown", d)
				if ev.Owner == game.OwnerRival {
					rep.Territory = append(rep.Territory, fmt.Sprintf("Police cleared %s: %s's crew lost it.", ev.Name, w.Rival.Leader))
				} else {
					rep.Territory = append(rep.Territory, fmt.Sprintf("Police cleared %s: you lost it.", ev.Name))
				}
			default:
				add("territory", "CornerLost", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s went back to the street: nobody was working it.", ev.Name))
			}
		case events.RivalMovedIn:
			d := base
			d.Corner, d.Rival = ev.Name, ev.Rival
			add("rivals", "RivalMovedIn", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew moved in on %s. Somebody new wants the city.", ev.Rival, ev.Name))
		case events.CornerTaken:
			d := base
			d.Corner, d.Rival = ev.Name, ev.Rival
			switch {
			case ev.Handed != "":
				d.Name = ev.Handed
				add("rivals", "CornerHanded", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s walked %s's crew onto %s. It is theirs now.", ev.Handed, ev.Rival, ev.Name))
			case ev.From == game.OwnerPlayer:
				add("rivals", "CornerTaken", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew TOOK %s from you. Your people walked home.", ev.Rival, ev.Name))
			default:
				add("rivals", "RivalClaimed", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew set up on %s.", ev.Rival, ev.Name))
			}
		case events.RivalPushed:
			d := base
			d.Corner, d.Rival = ev.Name, ev.Rival
			add("rivals", "RivalPushed", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew pushed on %s. You held it.", ev.Rival, ev.Name))
		case events.CornerStruck:
			d := base
			d.Corner, d.Rival = ev.Name, ev.Rival
			switch {
			case ev.Routed:
				add("rivals", "RivalRouted", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("Your enforcers %s %s and TOOK it. That was %s's last corner.", pastTense(ev.Force), ev.Name, ev.Rival))
			case ev.Taken:
				add("rivals", "CornerStruckTaken", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("Your enforcers %s %s and TOOK it. Post a runner before it drifts.", pastTense(ev.Force), ev.Name))
			default:
				add("rivals", "CornerStruckHeld", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("Your enforcers %s %s; %s's people held it.", pastTense(ev.Force), ev.Name, ev.Rival))
			}
		case events.RivalTippedPolice:
			d := base
			d.Rival = ev.Rival
			add("rivals", "RivalTippedPolice", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("Somebody tipped the police about you. It was %s. Heat +%.0f.", ev.Rival, ev.Heat))
		case events.RivalUndercut:
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew are undercutting you on %s: -%.0f%% demand there.", ev.Rival, strings.Join(ev.Corners, ", "), ev.Share*100))
		case events.WarEscalated:
			d := base
			if ev.Stage == "crackdown" {
				add("rivals", "WarCrackdown", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("CRACKDOWN. The police cleared %s. Both sides lost ground.", strings.Join(ev.Lost, ", ")))
			} else {
				add("rivals", "WarOpen", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("The war is loud (%.0f/100). Keep it up and the police clear both sides.", ev.War))
			}
		case events.CornerRobbed:
			d := base
			d.Corner = ev.Name
			add("territory", "CornerRobbed", d)
			rep.Territory = append(rep.Territory, robberyLine(w, ev))
			robbed += ev.Cash
		case events.FrontBought:
			d := base
			d.Front = ev.Name
			add("laundering", "FrontBought", d)
			spent += ev.Cost
			rep.Money = append(rep.Money, fmt.Sprintf("Bought %s -$%d. It opens today.", ev.Name, ev.Cost))
		case events.CashLaundered:
			line := fmt.Sprintf("Washed $%d clean through %d front(s)", ev.Amount, ev.Fronts)
			if ev.Upkeep > 0 {
				line += fmt.Sprintf(", upkeep -$%d", ev.Upkeep)
			}
			upkeep += ev.Upkeep
			rep.Money = append(rep.Money, line)
		case events.FrontAudited:
			d := base
			d.Front = ev.Name
			add("laundering", "FrontAudited", d)
			seized += ev.Seized
			line := fmt.Sprintf("AUDIT at %s: shut for %d days", ev.Name, ev.Days)
			if ev.Seized > 0 {
				line += fmt.Sprintf(", $%d seized", ev.Seized)
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
			add("laundering", "FrontFrozen", d)
			rep.Money = append(rep.Money, fmt.Sprintf("%s shut for %d days: $%d upkeep unpaid. Wash something.", ev.Name, ev.Days, ev.Upkeep))
		case events.ReputationShifted:
			key := "Reputation" + capitalize(ev.Axis) + "Down"
			if ev.Up() {
				key = "Reputation" + capitalize(ev.Axis) + "Up"
			}
			add("reputation", key, base)
		case events.CrewPaid:
			wages += ev.Wages
			line := fmt.Sprintf("Wages (%s) -$%d", ev.Pay, ev.Wages)
			if ev.Short > 0 {
				line += fmt.Sprintf(", $%d SHORT", ev.Short)
				rep.Crew = append(rep.Crew, "You could not make payroll. That gets around.")
			}
			rep.Money = append(rep.Money, line)
		}
	}

	// Purchases and signings made during the day. Cash "before" is what the
	// player woke up with: undo today's buys, fees, sales, wages, skims,
	// upkeep and seizures from the current total (the wash itself moves
	// money between pools and changes nothing).
	for _, b := range w.Buys {
		spent += b.Cost
		rep.Money = append(rep.Money, fmt.Sprintf("Bought %d %s at $%.0f = -$%d", b.Qty, w.ProductName(b.Product), b.UnitPrice, b.Cost))
	}
	for _, m := range w.Crew.HiredToday {
		spent += m.Fee
		rep.Money = append(rep.Money, fmt.Sprintf("Signing fee for %s -$%d", m.Name, m.Fee))
	}
	rep.CashBefore = w.Cash() - soldRevenue + lostCash + spent + wages + skimmed + robbed + upgrades + upkeep + seized + paidOff + investigated
	if soldRevenue > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Street sales +$%d", soldRevenue))
	}
	if robbed > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Robbed on the corner -$%d", robbed))
	}
	if skimmed > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Missing from the count -$%d", skimmed))
	}
	if lostCash > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Seized by police -$%d", lostCash))
	}
	if seized > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Seized by the auditors -$%d", seized))
	}

	// Flavour keeps the ticker alive on quiet days.
	if len(s.flav) > 0 && t.RNG.Float64() < s.cfg.FlavourChance {
		txt := render(s.flav[t.RNG.IntN(len(s.flav))], base)
		lines = append(lines, game.Headline{Day: t.Day, Source: "news", Text: txt})
	}

	// Yesterday's card: the choice is already in the journal (Choose put
	// it there); this is the morning after, when the follow-up makes the
	// paper. Then, maybe, tonight's card.
	if a := w.Dilemmas.Answered; a != nil {
		t.Emit(events.DilemmaAnswered{Day: t.Day, Card: a.Card, Choice: a.Choice})
		if a.Headline != "" {
			lines = append(lines, game.Headline{Day: t.Day, Source: "dilemma", Text: a.Headline})
		}
	}
	s.drawCard(w, t)

	for _, h := range lines {
		w.Journal = append(w.Journal, h)
		rep.News = append(rep.News, h.Text)
		t.Emit(events.Headline{Day: h.Day, Source: h.Source, Text: h.Text})
	}
	rep.CashAfter = w.Cash()
	w.Report = rep
}

func render(t *template.Template, d data) string {
	var b bytes.Buffer
	if err := t.Execute(&b, d); err != nil {
		return t.Name()
	}
	return b.String()
}

// pastTense is what the enforcers did, for the report.
func pastTense(f events.Force) string {
	switch f {
	case events.ForceWarn:
		return "warned"
	case events.ForceHit:
		return "hit"
	default:
		return "pushed"
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func priceLine(w *game.World, ev events.PriceMove) string {
	arrow := "→"
	pct := 0.0
	if ev.From > 0 {
		pct = (ev.To - ev.From) / ev.From * 100
	}
	switch {
	case pct > 1:
		arrow = "↑"
	case pct < -1:
		arrow = "↓"
	}
	return fmt.Sprintf("%-8s %8s %s %8s (%+.0f%%)", w.ProductName(ev.Product), dollars(ev.From), arrow, dollars(ev.To), pct)
}

func saleLine(w *game.World, ev events.PlayerSold) string {
	if ev.Sold == 0 {
		return fmt.Sprintf("%-8s wanted %d, sold none (%s)", w.ProductName(ev.Product), ev.Wanted, ev.Dial)
	}
	return fmt.Sprintf("%-8s sold %d/%d at %s avg = +$%d (%s)", w.ProductName(ev.Product), ev.Sold, ev.Wanted, dollars(ev.AvgPrice), ev.Revenue, ev.Dial)
}

// dollars formats a unit price: cents on a cheap bag, whole dollars once
// they stop mattering.
func dollars(v float64) string {
	if v < 1000 {
		return fmt.Sprintf("$%.2f", v)
	}
	return fmt.Sprintf("$%.0f", v)
}

func robberyLine(w *game.World, ev events.CornerRobbed) string {
	parts := []string{}
	for id, q := range ev.StockLost {
		parts = append(parts, fmt.Sprintf("%d %s", q, w.ProductName(id)))
	}
	sort.Strings(parts)
	s := fmt.Sprintf("%s was ROBBED: lost", ev.Name)
	if len(parts) > 0 {
		s += " " + strings.Join(parts, ", ")
	}
	if ev.Cash > 0 {
		if len(parts) > 0 {
			s += " and"
		}
		s += fmt.Sprintf(" $%d", ev.Cash)
	}
	return s + ". An enforcer on the corner would have helped."
}

func enforcementLine(w *game.World, ev events.Enforcement) string {
	switch ev.Level {
	case "patrol":
		return "PATROLS: street sales capped for a few days"
	case "arrest":
		return "ARRESTED."
	}
	parts := []string{}
	for id, q := range ev.StockLost {
		parts = append(parts, fmt.Sprintf("%d %s", q, w.ProductName(id)))
	}
	s := strings.ToUpper(ev.Level) + ": lost"
	if len(parts) > 0 {
		s += " " + strings.Join(parts, ", ")
	}
	if ev.CashLost > 0 {
		s += fmt.Sprintf(" and $%d", ev.CashLost)
	}
	if len(parts) == 0 && ev.CashLost == 0 {
		s += " nothing; they found an empty stash"
	}
	return s
}
