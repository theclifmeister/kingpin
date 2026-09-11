// Package news turns the day's events into headlines and the morning
// report, and deals the dilemma cards. It runs last so it sees everything
// the other sims emitted.
package news

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"
	"text/template"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
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
	Mode    string // how a shipment travelled
	From    string // the cities a shipment joined
	To      string
	Deal    string
	Stance  string // a DA's ticket or a chief's personality, in words
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
	// The buyers' lines pick their template off the deck's own side stream
	// (#71): the deck never touches the home city's dice, and its news
	// must not either, or a run would deal its cards on different days
	// with the deck in and out of the box.
	addBuyers := func(key string, d data) {
		list := s.tmpl[key]
		if len(list) == 0 {
			return
		}
		txt := render(list[t.Sub("buyers").IntN(len(list))], d)
		lines = append(lines, game.Headline{Day: t.Day, Source: "buyers", Text: txt})
	}
	here := w.Here()
	base := data{City: here.Name}
	// in names a city for a line about somewhere other than where you are.
	in := func(city string) string {
		if city == here.ID || w.Cities[city] == nil {
			return ""
		}
		return " in " + w.CityName(city)
	}
	// at is base for an event in a city.
	at := func(city string) data {
		d := base
		if c := w.Cities[city]; c != nil {
			d.City = c.Name
		}
		return d
	}
	cornerCity := func(id string) string {
		if c := w.Corner(id); c != nil {
			return c.City
		}
		return here.ID
	}

	// Money before we look at events: sales are already applied by market.
	var soldRevenue, lostCash, spent, wages, skimmed, robbed, upgrades, upkeep, seized, paidOff, investigated, shipping, tribute, cuts, funded, contracts, forfeits int
	routeCost := map[string]int{} // what each route cost today, lots and fares, by name in the order first seen
	var routeOrder []string
	charge := func(route string, cost int) {
		if _, ok := routeCost[route]; !ok {
			routeOrder = append(routeOrder, route)
		}
		routeCost[route] += cost
	}
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
			rep.Upgrades = append(rep.Upgrades, fmt.Sprintf("%s bought for %s %s. It is yours for the run.", ev.Name, format.Money(ev.Cost), pool))
			rep.Money = append(rep.Money, fmt.Sprintf("%s -%s", ev.Name, format.Money(ev.Cost)))
		case events.FallGuyBurned:
			add("heat", "FallGuyBurned", base)
			lostCash += ev.CashLost
			rep.Heat = append(rep.Heat, fmt.Sprintf("THE FALL GUY TOOK IT. The case is closed and heat is down, but %s went on making it stick. There is no second one.", format.Money(ev.CashLost)))
		case events.PriceMove:
			// The street where you are; the market screen has the rest.
			if ev.City == here.ID {
				rep.Prices = append(rep.Prices, priceLine(w, ev))
			}
		case events.ProductUnlocked:
			d := base
			d.Product = ev.Name
			add("market", "ProductUnlocked", d)
			rep.Prices = append(rep.Prices, fmt.Sprintf("%-8s now on offer from the supplier, around %s a unit", ev.Name, format.Price(ev.Price)))
		case events.PriceShock:
			d := at(ev.City)
			d.Product = w.ProductName(ev.Product)
			switch {
			case ev.Seized:
				add("market", "PriceShockSeized", d)
				rep.Prices = append(rep.Prices, fmt.Sprintf("%-8s ×%.1f%s for %s: the street was waiting on the shipment", w.ProductName(ev.Product), ev.Factor, in(ev.City), format.Plural(ev.Days, "day")))
			case ev.Slump:
				add("market", "PriceSlump", d)
			default:
				add("market", "PriceShock", d)
			}
		case events.PlayerSold:
			soldRevenue += ev.Revenue
			rep.Sales = append(rep.Sales, saleLine(w, ev)+in(ev.City))
			d := at(ev.City)
			d.Product = w.ProductName(ev.Product)
			d.Qty = ev.Sold
			switch {
			case ev.Sold == 0:
				add("market", "PlayerSoldZero", d)
			case ev.Dial == events.DialAggressive || float64(ev.Sold) >= w.Demand(ev.City, ev.Product)*1.2:
				add("market", "PlayerSoldBig", d)
			}
		case events.ContractOffered:
			rep.Sales = append(rep.Sales, fmt.Sprintf("%s Answer it on the market (2)%s: it stands %s.", ev.Pitch, in(ev.City), format.Plural(ev.Expires-t.Day+1, "day")))
			d := at(ev.City)
			d.Product = w.ProductName(ev.Product)
			d.Name = ev.Name
			addBuyers("ContractOffered", d)
		case events.ContractAccepted:
			rep.Sales = append(rep.Sales, fmt.Sprintf("You took %s's order: %d %s by day %d%s. Deliver it there (2, d).", ev.Name, ev.Units, w.ProductName(ev.Product), ev.Due, in(ev.City)))
		case events.ContractDelivered:
			contracts += ev.Revenue
			line := fmt.Sprintf("Handed %d %s to %s at %s (×%.3g street) = +%s%s", ev.Units, w.ProductName(ev.Product), ev.Name, format.Price(ev.Price), ev.Price/math.Max(ev.Street, 1e-9), format.Money(ev.Revenue), in(ev.City))
			if ev.Complete {
				line += ". Delivered in full."
			} else {
				line += fmt.Sprintf(". %d still owed.", ev.Owed)
			}
			if ev.Price < ev.Signed {
				line += fmt.Sprintf(" The street was %s the day they asked.", format.Price(ev.Signed))
			}
			rep.Sales = append(rep.Sales, line)
			rep.Money = append(rep.Money, fmt.Sprintf("%s paid +%s", ev.Name, format.Money(ev.Revenue)))
			if ev.Complete {
				d := at(ev.City)
				d.Product = w.ProductName(ev.Product)
				d.Name = ev.Name
				d.Qty = ev.Total
				addBuyers("ContractDelivered", d)
			}
		case events.ContractFailed:
			forfeits += ev.Cash
			line := fmt.Sprintf("You let %s down: %d of %d %s delivered by the day%s.", ev.Name, ev.Delivered, ev.Units, w.ProductName(ev.Product), in(ev.City))
			if ev.Cash > 0 {
				line += fmt.Sprintf(" They took %s for the rest.", format.Money(ev.Cash))
				rep.Money = append(rep.Money, fmt.Sprintf("%s collected for the shortfall -%s", ev.Name, format.Money(ev.Cash)))
			}
			line += " Word gets round."
			rep.Sales = append(rep.Sales, line)
			d := at(ev.City)
			d.Product = w.ProductName(ev.Product)
			d.Name = ev.Name
			addBuyers("ContractFailed", d)
		case events.ContractExpired:
			rep.Sales = append(rep.Sales, fmt.Sprintf("%s's offer lapsed: %d %s nobody answered for%s.", capitalize(ev.Name), ev.Units, w.ProductName(ev.Product), in(ev.City)))
		case events.HeatChanged:
			if ev.City != here.ID && len(ev.Reasons) == 0 && ev.To < 1 {
				break // a city nothing happened in
			}
			rep.Heat = append(rep.Heat, fmt.Sprintf("%s heat %.0f %s %.0f", w.CityName(ev.City), ev.From, format.Arrow, ev.To))
			for _, r := range ev.Reasons {
				rep.Heat = append(rep.Heat, "  "+r)
			}
			if ev.To >= 25 && ev.From < 25 {
				add("heat", "HeatWarning", at(ev.City))
			}
		case events.Enforcement:
			d := at(ev.City)
			d.Level = ev.Level
			add("heat", "Enforcement"+capitalize(ev.Level), d)
			rep.Heat = append(rep.Heat, enforcementLine(w, ev)+in(ev.City))
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
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s signed on as %s for %s", ev.Name, format.A(ev.Role), format.Money(ev.Fee)))
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
			rep.Money = append(rep.Money, fmt.Sprintf("Investigation -%s", format.Money(ev.Cost)))
		case events.CrewPaidOff:
			paidOff += ev.Cost
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s took your money and stays sweet on you, for now.", ev.Name))
			rep.Money = append(rep.Money, fmt.Sprintf("Paid off %s -%s", ev.Name, format.Money(ev.Cost)))
		case events.CrewQuit:
			d := base
			d.Name, d.Role = ev.Name, ev.Role
			add("crew", "CrewQuit", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s walked. Nobody was surprised.", ev.Name))
		case events.LieutenantWalked:
			d := at(ev.City)
			d.Name, d.Rival = ev.Name, ev.Rival
			if ev.Rival != "" {
				add("crew", "LieutenantWalkedRival", d)
			} else {
				add("crew", "LieutenantWalked", d)
			}
			rep.Crew = append(rep.Crew, lieutenantWalkedLine(ev))
		case events.LieutenantActed:
			cuts += ev.Cut
			skimmed += ev.Skimmed
			rep.Crew = append(rep.Crew, lieutenantLines(ev)...)
			if ev.Cut > 0 {
				rep.Money = append(rep.Money, fmt.Sprintf("%s's cut of %s -%s", ev.Name, ev.CityName, format.Money(ev.Cut)))
			}
		case events.CrewSkimmed:
			add("crew", "CrewSkimmed", base)
			skimmed += ev.Amount
			switch {
			case ev.FromWash == ev.Amount:
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s of the wash never came out clean. Somebody is cooking the books.", format.Money(ev.Amount)))
			case ev.FromWash > 0:
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s of the takings never made it back, %s of it from the wash. Somebody is skimming.", format.Money(ev.Amount), format.Money(ev.FromWash)))
			default:
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s of the takings never made it back. Somebody is skimming.", format.Money(ev.Amount)))
			}
		case events.CornerClaimed:
			d := at(cornerCity(ev.Corner))
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
			d := at(cornerCity(ev.Corner))
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
		case events.DealOffered:
			d := base
			d.Rival, d.Deal = ev.Rival, ev.Deal
			add("rivals", "DealOffered", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s offers %s. It stands %s: answer it on the rivals screen (8).", ev.Rival, ev.Terms, format.Plural(ev.Expires-t.Day+1, "day")))
		case events.DealAccepted:
			d := base
			d.Rival, d.Deal = ev.Rival, ev.Deal
			add("rivals", "DealAccepted", d)
			if ev.Offered {
				rep.Territory = append(rep.Territory, fmt.Sprintf("You took %s's offer: %s. It holds from tonight.", ev.Rival, ev.Terms))
			} else {
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s ACCEPTED %s. It holds from tonight.", ev.Rival, ev.Terms))
			}
		case events.DealRefused:
			d := base
			d.Rival, d.Deal = ev.Rival, ev.Deal
			add("rivals", "DealRefused", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s refused %s.", ev.Rival, ev.Terms))
		case events.DealBroken:
			d := base
			d.Rival, d.Deal = ev.Rival, ev.Deal
			add("rivals", "DealBroken", d)
			if ev.By == "rival" {
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s BROKE the %s: %s. So much for that.", ev.Rival, ev.Deal, ev.Why))
			} else {
				rep.Territory = append(rep.Territory, fmt.Sprintf("You BROKE the %s with %s: %s. Trust is gone, and they made a call.", ev.Deal, ev.Rival, ev.Why))
			}
		case events.DealEnded:
			rep.Territory = append(rep.Territory, fmt.Sprintf("The %s with %s has run out. Expect them back on your corners.", ev.Deal, ev.Rival))
		case events.TributePaid:
			tribute += ev.Amount
			rep.Money = append(rep.Money, fmt.Sprintf("Tribute to %s -%s", ev.Rival, format.Money(ev.Amount)))
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
			d := at(cornerCity(ev.Corner))
			d.Corner = ev.Name
			add("territory", "CornerRobbed", d)
			rep.Territory = append(rep.Territory, robberyLine(w, ev))
			robbed += ev.Cash
		case events.FrontBought:
			d := base
			d.Front = ev.Name
			add("laundering", "FrontBought", d)
			spent += ev.Cost
			rep.Money = append(rep.Money, fmt.Sprintf("Bought %s -%s. It opens today.", ev.Name, format.Money(ev.Cost)))
		case events.CashLaundered:
			line := fmt.Sprintf("Washed %s clean through %s", format.Money(ev.Amount), format.Plural(ev.Fronts, "front"))
			if ev.Upkeep > 0 {
				line += fmt.Sprintf(", upkeep -%s", format.Money(ev.Upkeep))
			}
			upkeep += ev.Upkeep
			rep.Money = append(rep.Money, line)
		case events.FrontAudited:
			d := base
			d.Front = ev.Name
			add("laundering", "FrontAudited", d)
			seized += ev.Seized
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
			add("laundering", "FrontFrozen", d)
			rep.Money = append(rep.Money, fmt.Sprintf("%s shut for %s: %s upkeep unpaid. Wash something.", ev.Name, format.Plural(ev.Days, "day"), format.Money(ev.Upkeep)))
		case events.WholesaleBought:
			// The route's lots, bought this morning for what it sends.
			shipping += ev.Cost
			charge(ev.Name, ev.Cost)
			rep.Shipments = append(rep.Shipments, fmt.Sprintf("Bought %d %s (%s) in %s for the %s -%s", ev.Units, w.ProductName(ev.Product), format.Plural(ev.Lots, "lot"), w.CityName(ev.City), ev.Name, format.Money(ev.Cost)))
		case events.ShipmentSent:
			// Paid this morning, when the route put it on the road.
			shipping += ev.Cost
			charge(ev.Name, ev.Cost)
			rep.Shipments = append(rep.Shipments, fmt.Sprintf("%d %s left %s for %s by %s, %s: %s, fare -%s", ev.Units, w.ProductName(ev.Product), w.CityName(ev.From), w.CityName(ev.To), ev.Mode, ev.Dial, format.Plural(ev.Days, "day"), format.Money(ev.Cost)))
		case events.ShipmentArrived:
			rep.Shipments = append(rep.Shipments, fmt.Sprintf("%d %s landed in %s from %s by %s", ev.Units, w.ProductName(ev.Product), w.CityName(ev.To), w.CityName(ev.From), ev.Mode))
		case events.ShipmentSeized:
			d := at(ev.To)
			d.Product, d.Qty, d.Mode, d.From, d.To = w.ProductName(ev.Product), ev.Units, ev.Mode, w.CityName(ev.From), w.CityName(ev.To)
			add("logistics", "ShipmentSeized", d)
			line := fmt.Sprintf("SEIZED on the road: %d %s bound for %s by %s, sent %s, every unit gone. Heat in both cities.", ev.Units, w.ProductName(ev.Product), w.CityName(ev.To), ev.Mode, ev.Dial)
			if ev.Dial == events.ShipFast {
				line += " Sent fast, it was asking to be looked at: the DA's file grows."
			}
			rep.Shipments = append(rep.Shipments, line)
			if w.Stats.Seizures == 1 {
				rep.Shipments = append(rep.Shipments, "The first one is the cue: a hot road wants the dial turned down (map, r), and the route sends what it lost again tomorrow.")
			}
		case events.ReputationShifted:
			key := "Reputation" + capitalize(ev.Axis) + "Down"
			if ev.Up() {
				key = "Reputation" + capitalize(ev.Axis) + "Up"
			}
			add("reputation", key, base)
		case events.DAElected:
			d := at(w.Home().ID)
			d.Name, d.Stance = ev.Name, stanceWords(ev.Stance)
			key := "DAElected"
			if ev.Incumbent {
				key = "DAReElected"
			}
			add("law", key, d)
			rep.Law = append([]string{electionLine(ev)}, rep.Law...) // the courthouse before the small print
		case events.ChiefReplaced:
			d := at(w.Home().ID)
			d.Name, d.Rival = ev.Name, ev.Old
			key := "ChiefReplaced"
			if ev.Why == "da" {
				key = "ChiefReplacedDA"
			}
			add("law", key, d)
			rep.Law = append([]string{chiefLine(ev)}, rep.Law...)
		case events.PressureShifted:
			key := "PressureShiftedDown"
			if ev.Up() {
				key = "PressureShiftedUp"
			}
			add("law", key, at(ev.City))
			rep.Law = append(rep.Law, fmt.Sprintf("%s pressure %.0f %s %.0f", w.CityName(ev.City), ev.From, format.Arrow, ev.To))
		case events.CityFunded:
			funded += ev.Amount
			rep.Law = append(rep.Law, fmt.Sprintf("Gave %s %s clean: goodwill +%.0f (now %.0f)", w.CityName(ev.City), format.Money(ev.Amount), ev.Goodwill, w.Cities[ev.City].Goodwill))
			rep.Money = append(rep.Money, fmt.Sprintf("Funded %s -%s clean", w.CityName(ev.City), format.Money(ev.Amount)))
		case events.CrewPaid:
			wages += ev.Wages
			line := fmt.Sprintf("Wages (%s) -%s", ev.Pay, format.Money(ev.Wages))
			if ev.Short > 0 {
				line += fmt.Sprintf(", %s SHORT", format.Money(ev.Short))
				rep.Crew = append(rep.Crew, "You could not make payroll. That gets around.")
			}
			rep.Money = append(rep.Money, line)
		}
	}

	// What each route cost today, lots and fares together: the money
	// section carries the one line, the shipments section the total.
	for _, name := range routeOrder {
		rep.Shipments = append(rep.Shipments, fmt.Sprintf("The %s cost %s today, lots and fares.", name, format.Money(routeCost[name])))
		rep.Money = append(rep.Money, fmt.Sprintf("The %s: lots and fares -%s", name, format.Money(routeCost[name])))
	}

	// Purchases and signings made during the day. Cash "before" is what the
	// player woke up with: undo today's buys, fees, sales, wages, skims,
	// upkeep and seizures from the current total (the wash itself moves
	// money between pools and changes nothing).
	for _, b := range w.Buys {
		spent += b.Cost
		rep.Money = append(rep.Money, fmt.Sprintf("Bought %d %s at %s = -%s", b.Qty, w.ProductName(b.Product), format.Price(b.UnitPrice), format.Money(b.Cost)))
	}
	for _, m := range w.Crew.HiredToday {
		spent += m.Fee
		rep.Money = append(rep.Money, fmt.Sprintf("Signing fee for %s -%s", m.Name, format.Money(m.Fee)))
	}
	rep.CashBefore = w.Cash() - soldRevenue - contracts + forfeits + lostCash + spent + wages + skimmed + robbed + upgrades + upkeep + seized + paidOff + investigated + shipping + tribute + cuts + funded
	if soldRevenue > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Street sales +%s", format.Money(soldRevenue)))
	}
	if robbed > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Robbed on the corner -%s", format.Money(robbed)))
	}
	if skimmed > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Missing from the count -%s", format.Money(skimmed)))
	}
	if lostCash > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Seized by police -%s", format.Money(lostCash)))
	}
	if seized > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Seized by the auditors -%s", format.Money(seized)))
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

// stanceWords is a DA's ticket as the paper prints it.
func stanceWords(stance string) string {
	switch stance {
	case "law_and_order":
		return "law-and-order"
	default:
		return stance
	}
}

// electionLine is the report's line on a DA election.
func electionLine(ev events.DAElected) string {
	if ev.Incumbent {
		return fmt.Sprintf("DA %s re-elected on the %s ticket; nothing changes at the courthouse.", ev.Name, stanceWords(ev.Stance))
	}
	switch ev.Stance {
	case "law_and_order":
		return fmt.Sprintf("DA %s elected on a law-and-order ticket: the file need not be as thick, and the sting line drops.", ev.Name)
	case "reform":
		return fmt.Sprintf("DA %s elected on a reform ticket: it takes a thicker file to indict, and stings come later.", ev.Name)
	default:
		return fmt.Sprintf("DA %s elected, a moderate: the courthouse runs by the book.", ev.Name)
	}
}

// chiefLine is the report's line on a new police chief.
func chiefLine(ev events.ChiefReplaced) string {
	if ev.Why == "da" {
		return fmt.Sprintf("The new DA wanted a new chief: %s is out, %s is in. You will learn what they are like.", ev.Old, ev.Name)
	}
	return fmt.Sprintf("Chief %s's term is up; %s takes over. You will learn what they are like.", ev.Old, ev.Name)
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
	arrow := format.Arrow
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
	return fmt.Sprintf("%-8s %8s %s %8s (%+.0f%%)", w.ProductName(ev.Product), format.Price(ev.From), arrow, format.Price(ev.To), pct)
}

func saleLine(w *game.World, ev events.PlayerSold) string {
	who := ""
	if ev.Standing {
		who = ", " + ev.LieutenantName
	}
	if ev.Sold == 0 {
		return fmt.Sprintf("%-8s wanted %d, sold none (%s%s)", w.ProductName(ev.Product), ev.Wanted, ev.Dial, who)
	}
	return fmt.Sprintf("%-8s sold %d/%d at %s avg = +%s (%s%s)", w.ProductName(ev.Product), ev.Sold, ev.Wanted, format.Price(ev.AvgPrice), format.Money(ev.Revenue), ev.Dial, who)
}

// lieutenantLines is what a lieutenant's night reads like in the report:
// what they did with the crew and the corners, and, once you know them,
// what they are like. A greedy one's skim is missing money like anyone
// else's; the line never says so.
func lieutenantLines(ev events.LieutenantActed) []string {
	var did []string
	if n := len(ev.Posted); n > 0 {
		did = append(did, fmt.Sprintf("posted %s on %s", count(n, "runner"), strings.Join(ev.Posted, ", ")))
	}
	if n := len(ev.Guarded); n > 0 {
		did = append(did, fmt.Sprintf("put %s on %s", count(n, "enforcer"), strings.Join(ev.Guarded, ", ")))
	}
	if n := len(ev.Dropped); n > 0 {
		did = append(did, fmt.Sprintf("gave up %s", strings.Join(ev.Dropped, ", ")))
	}
	if ev.Orders > 0 {
		did = append(did, fmt.Sprintf("sells %s tomorrow", ev.Dial))
	}
	line := fmt.Sprintf("%s runs %s", ev.Name, ev.CityName)
	if len(did) > 0 {
		line += ": " + strings.Join(did, ", ")
	}
	lines := []string{line + "."}
	if ev.Revenue > 0 {
		lines = append(lines, fmt.Sprintf("  %s took %s; %s kept %s of it.", ev.CityName, format.Money(ev.Revenue), ev.Name, format.Money(ev.Cut)))
	}
	if ev.Revealed {
		lines = append(lines, fmt.Sprintf("  You have seen enough of %s to know: %s.", ev.Name, ev.Personality))
	}
	return lines
}

// lieutenantWalkedLine is the report on a lieutenant who left with the
// city.
func lieutenantWalkedLine(ev events.LieutenantWalked) string {
	s := fmt.Sprintf("%s WALKED, and took %s with them", ev.Name, ev.CityName)
	switch {
	case len(ev.Corners) > 0 && ev.Rival != "":
		s += fmt.Sprintf(": %s now fly %s's colours", strings.Join(ev.Corners, ", "), ev.Rival)
	case len(ev.Corners) > 0:
		s += fmt.Sprintf(": %s went back to the street", strings.Join(ev.Corners, ", "))
	}
	if ev.Units > 0 {
		s += fmt.Sprintf(", and the %s stashed there are gone", format.Plural(ev.Units, "unit"))
	}
	return s + "."
}

// count is n things, pluralised.
func count(n int, what string) string {
	if n == 1 {
		return "a " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
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
		s += " " + format.Money(ev.Cash)
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
		s += " and " + format.Money(ev.CashLost)
	}
	if len(parts) == 0 && ev.CashLost == 0 {
		s += " nothing; they found an empty stash"
	}
	return s
}
