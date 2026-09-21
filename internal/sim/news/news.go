// Package news turns the day's events into headlines and the morning
// report, and deals the dilemma cards. It runs last so it sees everything
// the other sims emitted.
package news

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
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
	pcfg content.ProgressionConfig
	tmpl map[string][]*template.Template
	flav []*template.Template
	deck []card
	inc  map[string]*template.Template // the incidents' report lines by id (#44)
}

// New parses the headline and card templates once, copying what it
// reads of the config (#144): the headlines, the dilemma deck, the
// progression and the incidents' report lines (#44). It refuses a deck
// whose choices use an effect key the world does not apply. The
// progression (#147) is the tiers the sim stamps every morning.
func New(cfg *content.Config) (*Sim, error) {
	deck, err := parseDeck(cfg.Dilemmas)
	if err != nil {
		return nil, fmt.Errorf("dilemmas: %w", err)
	}
	s := &Sim{cfg: cfg.Headlines, dcfg: cfg.Dilemmas, pcfg: cfg.Progression, tmpl: map[string][]*template.Template{}, deck: deck, inc: map[string]*template.Template{}}
	for _, inc := range cfg.Incidents.Table {
		t, err := template.New(inc.ID + ".report").Funcs(articles).Parse(article(inc.Report))
		if err != nil {
			return nil, fmt.Errorf("incident %s report: %w", inc.ID, err)
		}
		s.inc[inc.ID] = t
	}
	for key, list := range s.cfg.Templates {
		for i, src := range list {
			t, err := template.New(fmt.Sprintf("%s#%d", key, i)).Funcs(articles).Parse(article(src))
			if err != nil {
				return nil, fmt.Errorf("headline %s[%d]: %w", key, i, err)
			}
			s.tmpl[key] = append(s.tmpl[key], t)
		}
	}
	for i, src := range s.cfg.Flavour {
		t, err := template.New(fmt.Sprintf("flavour#%d", i)).Parse(src)
		if err != nil {
			return nil, fmt.Errorf("flavour[%d]: %w", i, err)
		}
		s.flav = append(s.flav, t)
	}
	return s, nil
}

func (s *Sim) Name() string { return "news" }

// articles are the template functions the article rewrite calls: `a`
// is format.A, `A` the same at the head of a sentence.
var articles = template.FuncMap{
	"a": format.A,
	"A": func(noun string) string {
		s := format.A(noun)
		return strings.ToUpper(s[:1]) + s[1:]
	},
}

// articleRE is an indefinite article written before a field in a
// template source: `a {{.City}}`, `An {{.Product}}`.
var articleRE = regexp.MustCompile(`\b([Aa])n? \{\{(\.\w+)\}\}`)

// article rewrites every `a {{.X}}` in a template source to `{{a .X}}`,
// so the article agrees with the value (`an Eastside outfit`, `a
// Bayport outfit`; #148: the file wrote `a {{.City}}` and Eastside
// read `a Eastside`). The value alone is what decides it, so a
// template's own words are left as written.
func article(src string) string {
	return articleRE.ReplaceAllString(src, "{{$1 $2}}")
}

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

// data is what a template can name. The paper's regulars (#44: the DA,
// the chief, the rival's leader and faction) are filled on every line
// from the world, so any headline can read like a paper (`as DA Ramirez
// promises a crackdown`); the rest come from the event.
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
	Route   string // a route by name (#44)
	Days    int    // how long an incident's effect runs (#44)
	DA      string // the sitting DA's surname (#44)
	Chief   string // the chief's surname (#44)
	Leader  string // the rival's leader (#44)
	Faction string // the rival's faction, `Big Sal's crew` (#44)
	Asset   string // an asset by name (#48)
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
	// with the deck in and out of the box. The connects' lines (#72)
	// pick theirs off the connects' stream for the same reason: a run
	// that never takes credit is the run it was.
	addOff := func(stream, source, key string, d data) {
		list := s.tmpl[key]
		if len(list) == 0 {
			return
		}
		txt := render(list[t.Sub(stream).IntN(len(list))], d)
		lines = append(lines, game.Headline{Day: t.Day, Source: source, Text: txt})
	}
	addBuyers := func(key string, d data) { addOff("buyers", "buyers", key, d) }
	addSuppliers := func(key string, d data) { addOff("suppliers", "market", key, d) }
	// The houses' lines (#73) pick theirs off a side stream too: a run
	// with no house is the run it was.
	addHouses := func(source, key string, d data) { addOff("houses:news", source, key, d) }
	// The intel lines (#45) pick theirs off the intel side stream: a run
	// with no spy under and nobody feeding it is the run it was.
	addIntel := func(source, key string, d data) { addOff("intel:news", source, key, d) }
	here := w.Here()
	base := data{City: here.Name, DA: w.Law.DA.Name, Chief: w.Law.Chief.Name, Leader: w.Rival().Leader}
	if w.Rival().Leader != "" {
		base.Faction = w.Rival().Leader + "'s crew"
	}
	// crew names the faction a rival event is about (#43): the event's
	// leader and their crew in the Leader and Faction slots, so a line
	// about the second faction names the second faction.
	crew := func(d data, leader string) data {
		if leader != "" {
			d.Leader, d.Faction = leader, leader+"'s crew"
		}
		return d
	}
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

	// The tier (#147), first: the run enters the first tier past the
	// highest reached whose trigger holds this morning, one a morning,
	// so a night that crosses two lines is two mornings and each has
	// one thing to say. The headline picks its template off the
	// progression's own stream: the home city's dice never move for
	// it, and a run before #147 replays as it did.
	if n := s.tier(w, t); n > 0 {
		tier := s.pcfg.Tier(n)
		rep.Tier = tierLines(n, len(s.pcfg.Tiers), *tier)
		addOff("progression", "news", "TierReached", base)
	}
	// The reign (#227): while the city is yours the TIER section opens
	// with where the reign stands, its homage counted off tonight's
	// TributePaid; the morning it began carries the headline, and the
	// morning it broke says how.
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.ReignBegan:
			d := at(ev.City)
			add("rivals", "ReignBegan", d)
			rep.Tier = append([]string{fmt.Sprintf("The city is yours: every crew in %s is gone or paying. Take the crown when you are ready (walk away on the dashboard), or reign.", d.City)}, rep.Tier...)
		case events.ReignBroken:
			rep.Tier = append([]string{fmt.Sprintf("The reign is over for now: %s. Hold the city and the table and it begins again.", ev.Why)}, rep.Tier...)
		}
	}
	if w.Reign > 0 {
		rep.Tier = append([]string{reignLine(w, t)}, rep.Tier...)
	}

	// The world's incident (#44), dealt first thing this tick: a
	// headline under the world source, its template picked off the
	// incidents' own side stream (the home city's dice never move for
	// it), and the report opens with the row's line, before the tier.
	for _, e := range t.Events() {
		ev, ok := e.(events.Incident)
		if !ok {
			continue
		}
		d := at(ev.City)
		d.Name, d.Route, d.Days = ev.Person, ev.Route, ev.Days
		if ev.Chief != "" {
			d.Chief = ev.Chief // the chief it named, not the one the law sim seated since
		}
		if ev.DA != "" {
			d.DA = ev.DA
		}
		if ev.Product != "" {
			d.Product = w.ProductName(ev.Product)
		}
		if ev.LeaderKilled {
			// The leader the rivals sim took this tick (#43), not the
			// rival at home's.
			for _, e := range t.Events() {
				if k, ok := e.(events.RivalLeaderArrested); ok && k.Killed {
					d = crew(d, k.Rival)
				}
			}
		}
		key := content.IncidentConfig{ID: ev.ID}.Key()
		if !s.HasTemplate(key) {
			key = "Incident"
			d.Name = ev.Name
		}
		addOff("incidents:news", "world", key, d)
		if tm := s.inc[ev.ID]; tm != nil {
			rep.Incident = append(rep.Incident, render(tm, d))
		} else {
			rep.Incident = append(rep.Incident, ev.Name+".")
		}
	}

	// Money before we look at events: sales are already applied by market.
	var soldRevenue, lostCash, spent, wages, skimmed, robbed, upgrades, upkeep, seized, paidOff, investigated, shipping, tribute, cuts, standingCut, funded, backed, contracts, forfeits, repaid, rent, earned, invested, cutting, cooking, reserved, deeds, deedRent int
	var scouted, poached, boosted int // the books (#70): what a scout and a buy-off cost, less the refund, and what a boost took
	var bribed, checkpoints int       // the bought law (#42): the envelopes and the deals on the road, paid up front
	routeCost := map[string]int{}     // what each route cost today, lots and fares, by name in the order first seen
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
		case events.Unlocked:
			// A gate crossed (#148): one UNLOCKED line, first in the
			// report, and a headline under the unlock source, which
			// neither notoriety nor the law counts. A product's pick
			// stays on the home stream and a connect's on the
			// connects', where they were, so no pinned run moves; a
			// front's and a role's come off their own side stream.
			rep.Unlocked = append(rep.Unlocked, unlockLine(w, ev))
			d := at(ev.City)
			d.Name = ev.Name
			switch ev.Gate {
			case "product":
				d.Product = ev.Name
				add("unlock", "UnlockedProduct", d)
			case "connect":
				addOff("suppliers", "unlock", "UnlockedConnect", d)
			case "front":
				d.Front = ev.Name
				addOff("unlocks", "unlock", "UnlockedFront", d)
			case "role":
				d.Role = ev.ID
				addOff("unlocks", "unlock", "UnlockedRole", d)
			case "asset":
				d.Asset = ev.Name
				addOff("assets:news", "unlock", "UnlockedAsset", d)
			}
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
			cuts += ev.Cut
			standingCut += ev.Cut
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
		case events.SupplyBought:
			// The contract's buy this morning (#113): the money line is
			// the receipt's, below, and this is the sales section's. A
			// lieutenant's contract (#174) is the CREW line's instead.
			if ev.Lieutenant == "" {
				rep.Sales = append(rep.Sales, fmt.Sprintf("Supply contract bought %d %s at %s to keep %d%s = -%s", ev.Units, w.ProductName(ev.Product), format.Price(ev.Price), ev.Level, in(ev.City), format.Money(ev.Cost)))
			}
		case events.StandingShort:
			if ev.Stock == 0 {
				rep.Sales = append(rep.Sales, fmt.Sprintf("Standing order for %d %s%s: nothing stashed, nothing sold. Restock, or cancel it.", ev.Units, w.ProductName(ev.Product), in(ev.City)))
			} else {
				rep.Sales = append(rep.Sales, fmt.Sprintf("Standing order for %d %s%s: only %d stashed.", ev.Units, w.ProductName(ev.Product), in(ev.City), ev.Stock))
			}
		case events.SupplyShort:
			why := "there was no cash over the float for the rest"
			if ev.Why == "room" {
				why = "the stash there has no room for the rest"
			}
			if ev.Why == "supplier" {
				why = "nobody there sells it today"
			}
			if ev.Lieutenant != "" {
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s could not keep %s stocked%s: %d under the level, %s.", ev.Lieutenant, w.ProductName(ev.Product), in(ev.City), ev.Short, why))
			} else {
				rep.Sales = append(rep.Sales, fmt.Sprintf("Supply contract short: %d %s under the level%s, %s.", ev.Short, w.ProductName(ev.Product), in(ev.City), why))
			}
		case events.CreditTaken:
			rep.Money = append(rep.Money, fmt.Sprintf("%s put %s on your book%s: you owe them %s, due day %d.", ev.Name, format.Money(ev.Amount), in(ev.City), format.Money(ev.Debt), ev.Due))
		case events.DebtPaid:
			repaid += ev.Amount
			rep.Money = append(rep.Money, fmt.Sprintf("Paid %s the %s you owed, on the day. -%s", ev.Name, format.Money(ev.Amount), format.Money(ev.Amount)))
		case events.DebtLate:
			repaid += ev.Paid
			d := at(ev.City)
			d.Name = ev.Name
			addSuppliers("DebtLate", d)
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
			d := at(ev.City)
			d.Name = ev.Name
			addSuppliers("SupplierFrozen", d)
			if ev.Why == "floor" {
				rep.Sales = append(rep.Sales, fmt.Sprintf("%s has stopped taking your calls%s: %s, and nothing sells from them until then. Buy from somebody else.", ev.Name, in(ev.City), format.Plural(ev.Days, "day")))
			}
		case events.SupplierWarned:
			d := at(ev.City)
			d.Name, d.Product = ev.Name, w.ProductName(ev.Product)
			addSuppliers("SupplierWarned", d)
			if ev.Slump {
				rep.Sales = append(rep.Sales, fmt.Sprintf("%s says the street%s is going quiet on %s tomorrow. Sell tonight.", ev.Name, in(ev.City), w.ProductName(ev.Product)))
			} else {
				rep.Sales = append(rep.Sales, fmt.Sprintf("%s says %s is about to jump%s tomorrow. Stock up.", ev.Name, w.ProductName(ev.Product), in(ev.City)))
			}
		case events.SupplierCollected:
			d := at(ev.City)
			d.Name = ev.Name
			addSuppliers("SupplierCollected", d)
			switch {
			case ev.Member != 0 && ev.Hurt:
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s's people found %s over the %s you owe: a beating, loyalty -%.0f, off the corner.", ev.Name, ev.MemberName, format.Money(ev.Owed), ev.Loyalty))
			case ev.Member != 0:
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s's people had a word with %s over the %s you owe: loyalty -%.0f. They stood their ground.", ev.Name, ev.MemberName, format.Money(ev.Owed), ev.Loyalty))
			case ev.Units > 0:
				rep.Money = append(rep.Money, fmt.Sprintf("%s's people took %d %s from the stash%s against the %s you owe.", ev.Name, ev.Units, w.ProductName(ev.Product), in(ev.City), format.Money(ev.Owed)))
			default:
				rep.Money = append(rep.Money, fmt.Sprintf("%s's people came for the %s you owe and found nothing to take. They will be back.", ev.Name, format.Money(ev.Owed)))
			}
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
			d = crew(d, ev.Rival)
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
		case events.CrewBailed:
			// Crew life (#46): the cells, the cots and the funerals. The
			// bondsman's bail (#230, Who) is on the arrest's own line;
			// here it is the money.
			paidOff += ev.Cost
			if ev.Who != "" {
				rep.Money = append(rep.Money, fmt.Sprintf("Bail for %s -%s clean (%s)", ev.Name, format.Money(ev.Cost), ev.Who))
				break
			}
			rep.Crew = append(rep.Crew, fmt.Sprintf("Bail is down for %s: they walk tomorrow.", ev.Name))
			rep.Money = append(rep.Money, fmt.Sprintf("Bail for %s -%s clean", ev.Name, format.Money(ev.Cost)))
		case events.CrewArrested:
			d := at(ev.City)
			d.Name, d.Role, d.Corner = ev.Name, ev.Role, ev.CornerName
			add("crew", "CrewArrested", d)
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
				add("crew", "CrewReleased", d)
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s is out. Nobody came for them, and the DA had them for a while.", ev.Name))
			}
		case events.CrewShot:
			d := at(ev.City)
			d.Name, d.Role, d.Corner, d.Rival = ev.Name, ev.Role, ev.CornerName, ev.Rival
			where := ""
			if ev.CornerName != "" {
				where = " on " + ev.CornerName
			}
			switch {
			case ev.Theirs:
				add("rivals", "MuscleKilled", d)
				rep.Crew = append(rep.Crew, fmt.Sprintf("One of %s's people was shot dead%s. The paper has your name next to it.", ev.Rival, where))
			case ev.Dead:
				add("crew", "CrewKilled", d)
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s was shot dead%s. The paper has your name next to it.", ev.Name, where))
			default:
				add("crew", "CrewShot", d)
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s was shot%s: laid up for %s, off the corner.", ev.Name, where, format.Plural(ev.Days, "day")))
			}
		case events.CrewRecovered:
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s is back on their feet.", ev.Name))
		case events.CrewRetired:
			d := base
			d.Name, d.Role = ev.Name, ev.Role
			add("crew", "CrewRetired", d)
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
			add("crew", "CrewQuit", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s walked. Nobody was surprised.", ev.Name))
		case events.LieutenantWalked:
			d := at(ev.City)
			d.Name, d.Rival = ev.Name, ev.Rival
			d = crew(d, ev.Rival)
			if ev.Rival != "" {
				add("crew", "LieutenantWalkedRival", d)
			} else {
				add("crew", "LieutenantWalked", d)
			}
			rep.Crew = append(rep.Crew, lieutenantWalkedLine(ev))
		case events.LieutenantActed:
			cuts += ev.Cut
			skimmed += ev.Skimmed
			rep.Crew = append(rep.Crew, lieutenantLines(w, ev)...)
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
					d = crew(d, w.FactionName(ev.Faction))
					rep.Territory = append(rep.Territory, fmt.Sprintf("Police cleared %s: %s lost it.", ev.Name, w.FactionName(ev.Faction)))
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
			d = crew(d, ev.Rival)
			add("rivals", "RivalMovedIn", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew moved in on %s. Somebody new wants the city.", ev.Rival, ev.Name))
		case events.CornerTaken:
			d := base
			d.Corner, d.Rival = ev.Name, ev.Rival
			d = crew(d, ev.Rival)
			switch {
			case ev.Handed != "":
				d.Name = ev.Handed
				add("rivals", "CornerHanded", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s walked %s's crew onto %s. It is theirs now.", ev.Handed, ev.Rival, ev.Name))
			case ev.From == game.OwnerPlayer && ev.Pricewar:
				add("rivals", "CornerTaken", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew TOOK %s from you: the price war's answer. Your people walked home.", ev.Rival, ev.Name))
			case ev.From == game.OwnerPlayer:
				add("rivals", "CornerTaken", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew TOOK %s from you. Your people walked home.", ev.Rival, ev.Name))
			default:
				add("rivals", "RivalClaimed", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew set up on %s.", ev.Rival, ev.Name))
			}
		case events.RivalEyeing:
			d := base
			d.Corner, d.Rival = ev.Name, ev.Rival
			d = crew(d, ev.Rival)
			add("rivals", "RivalEyeing", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("Word is %s's crew are setting up on %s tomorrow%s. Post somebody on it tonight and it stays off them.", ev.Rival, ev.Name, eyeingWhy(w.Faction(ev.Faction))))
		case events.RivalOutbid:
			d := base
			d.Corner, d.Rival = ev.Name, ev.Rival
			d = crew(d, ev.Rival)
			add("rivals", "RivalOutbid", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew came for %s and found your people on it. They left; they will not forget it.", ev.Rival, ev.Name))
		case events.RivalPushed:
			d := base
			d.Corner, d.Rival = ev.Name, ev.Rival
			d = crew(d, ev.Rival)
			add("rivals", "RivalPushed", d)
			if ev.Pricewar {
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew pushed on %s over the price war. You held it.", ev.Rival, ev.Name))
			} else {
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew pushed on %s. You held it.", ev.Rival, ev.Name))
			}
		case events.CornerStruck:
			d := base
			d.Corner, d.Rival = ev.Name, ev.Rival
			d = crew(d, ev.Rival)
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
			d = crew(d, ev.Rival)
			add("rivals", "RivalTippedPolice", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("Somebody tipped the police about you. It was %s. Heat +%.0f.", ev.Rival, ev.Heat))
		// The books (#70): the scout, the boost, the tip, the raid and
		// the buy-off. The scout, the tip and the buy-off are report-only;
		// the boost and the raid are news.
		case events.RivalScouted:
			scouted += ev.Cost
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
			d = crew(d, ev.Rival)
			if ev.Taken {
				boosted += ev.Cash
				add("rivals", "RivalBoosted", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("Your enforcers robbed %s on %s: %s off their day's take, into your pocket. The corner is still theirs.", ev.Rival, ev.Name, format.Money(ev.Cash)))
				rep.Money = append(rep.Money, fmt.Sprintf("Robbed %s on %s +%s", ev.Rival, ev.Name, format.Money(ev.Cash)))
			} else {
				add("rivals", "RivalBoostedHeld", d)
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
			d = crew(d, ev.Rival)
			add("rivals", "RivalRaided", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("The police RAIDED %s on %s: it is free, and %s of theirs went in the van. Post a runner before somebody else does.", ev.Rival, ev.Name, format.Plural(ev.Muscle, "head")))
		case events.RivalMusclePoached:
			d := base
			d.Rival = ev.Rival
			d = crew(d, ev.Rival)
			poached += ev.Cost - ev.Refund
			switch {
			case ev.Failed:
				rep.Territory = append(rep.Territory, fmt.Sprintf("Your money never reached %s's people, or they took it and stayed. %s knows you tried.", ev.Rival, ev.Rival))
				rep.Money = append(rep.Money, fmt.Sprintf("Buying off %s's muscle, lost -%s", ev.Rival, format.Money(ev.Cost)))
			case ev.Got == 0:
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s had nobody left to buy off. Your money came back.", ev.Rival))
			default:
				add("rivals", "RivalMusclePoached", d)
				line := fmt.Sprintf("%s of %s's muscle took your money and went home. They are nobody's now.", format.Plural(ev.Got, "head"), ev.Rival)
				if ev.Refund > 0 {
					line += fmt.Sprintf(" They only had %d; %s came back.", ev.Got, format.Money(ev.Refund))
				}
				rep.Territory = append(rep.Territory, line)
				rep.Money = append(rep.Money, fmt.Sprintf("Bought off %s of %s's muscle -%s", format.Plural(ev.Got, "head"), ev.Rival, format.Money(ev.Cost-ev.Refund)))
			}
		case events.RivalUndercut:
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew are undercutting you on %s: -%.0f%% demand there.", ev.Rival, strings.Join(ev.Corners, ", "), ev.Share*100))
		case events.PlayerUndercut:
			// The price war (#68): one line per corner and product, in
			// SALES, since the units are part of the night's sale.
			rep.Sales = append(rep.Sales, fmt.Sprintf("Undercut %s on %s: %d %s cheap = +%s, %.0f%% of their trade there", cornerHolder(w, ev.Corner), ev.Name, ev.Units, w.ProductName(ev.Product), format.Money(ev.Revenue), ev.Share*100))
		case events.RivalAbandoned:
			d := base
			d.Corner, d.Rival = ev.Name, ev.Rival
			d = crew(d, ev.Rival)
			add("rivals", "RivalAbandoned", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew GAVE UP %s: the price war made it not worth holding. It is free; post a runner before somebody else does.", ev.Rival, ev.Name))
		case events.DealOffered:
			d := base
			d.Rival, d.Deal = ev.Rival, ev.Deal
			d = crew(d, ev.Rival)
			add("rivals", "DealOffered", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s offers %s. It stands %s: answer it on the rivals screen (8).", ev.Rival, ev.Terms, format.Plural(ev.Expires-t.Day+1, "day")))
		case events.DealAccepted:
			d := base
			d.Rival, d.Deal = ev.Rival, ev.Deal
			d = crew(d, ev.Rival)
			add("rivals", "DealAccepted", d)
			if ev.Offered {
				rep.Territory = append(rep.Territory, fmt.Sprintf("You took %s's offer: %s. It holds from tonight.", ev.Rival, ev.Terms))
			} else {
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s ACCEPTED %s. It holds from tonight.", ev.Rival, ev.Terms))
			}
		case events.DealRefused:
			d := base
			d.Rival, d.Deal = ev.Rival, ev.Deal
			d = crew(d, ev.Rival)
			add("rivals", "DealRefused", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("%s refused %s.", ev.Rival, ev.Terms))
		case events.DealBroken:
			d := base
			d.Rival, d.Deal = ev.Rival, ev.Deal
			d = crew(d, ev.Rival)
			add("rivals", "DealBroken", d)
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
			tribute += ev.Amount
			rep.Money = append(rep.Money, fmt.Sprintf("Tribute to %s -%s", ev.Rival, format.Money(ev.Amount)))
		// The table (#43): factions fighting each other, one absorbing
		// another, a leader taken, your crew poached, a betrayal
		// remembered by everyone.
		case events.FactionPushed:
			d := at(ev.City)
			d.Corner, d.Rival = ev.Name, ev.Rival
			d = crew(d, ev.Rival)
			d.Name = ev.AgainstRival
			if ev.Taken {
				add("rivals", "FactionTook", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew TOOK %s off %s's%s.", ev.Rival, ev.Name, ev.AgainstRival, in(ev.City)))
			} else {
				add("rivals", "FactionPushed", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew pushed on %s's people on %s%s and were held off.", ev.Rival, ev.AgainstRival, ev.Name, in(ev.City)))
			}
		case events.RivalAbsorbed:
			d := base
			d.Rival = ev.Rival
			d = crew(d, ev.Rival)
			d.Name = ev.By
			if ev.By != "" {
				add("rivals", "RivalAbsorbed", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew is no more: what was left of it went over to %s. One faction fewer.", ev.Rival, ev.By))
			} else {
				add("rivals", "RivalScattered", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s's crew scattered: nobody left to run with. One faction fewer.", ev.Rival))
			}
		case events.RivalLeaderArrested:
			d := at(ev.City)
			d.Rival = ev.Rival
			d = crew(d, ev.Rival)
			if ev.Killed {
				add("rivals", "RivalLeaderKilled", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("%s is DEAD. Their crew is coming apart: %s go back to the street over the coming days, prices%s spike, and their people are looking for work%s.", ev.Rival, format.Plural(ev.Corners, "corner"), in(ev.City), pointer(ev.Muscle)))
			} else {
				add("rivals", "RivalLeaderArrested", d)
				rep.Territory = append(rep.Territory, fmt.Sprintf("The police took %s. Their crew is coming apart: %s go back to the street over the coming days, prices%s spike, and their people are looking for work%s.", ev.Rival, format.Plural(ev.Corners, "corner"), in(ev.City), pointer(ev.Muscle)))
			}
		case events.CrewPoached:
			d := base
			d.Name, d.Role, d.Rival = ev.Name, ev.Role, ev.Rival
			d = crew(d, ev.Rival)
			if ev.Stayed {
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s offered %s %s a day to come over. They stayed, and they know they are wanted.", ev.Rival, ev.Name, format.Money(ev.Wages)))
			} else {
				add("crew", "CrewPoached", d)
				rep.Crew = append(rep.Crew, fmt.Sprintf("%s offered %s %s a day and they TOOK it. They are %s's now.", ev.Rival, ev.Name, format.Money(ev.Wages), ev.Rival))
			}
		case events.TrustSpread:
			rep.Territory = append(rep.Territory, fmt.Sprintf("Word of the broken deal got round: %s trust you %.0f less.", format.Plural(ev.Others, "other faction"), ev.Spread))
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
			if ev.Earned > 0 {
				line += fmt.Sprintf(", the businesses earned +%s clean", format.Money(ev.Earned))
			}
			upkeep += ev.Upkeep
			earned += ev.Earned
			rep.Money = append(rep.Money, line)
		case events.FrontInvested:
			// Levels bought at a front (#192): bookkeeping, and the
			// growth is news only when it crosses the line (FrontGrew).
			invested += ev.Cost
			rep.Money = append(rep.Money, fmt.Sprintf("Invested %s clean in %s: level %d, earning %s/day clean", format.Money(ev.Cost), ev.Name, ev.Level, format.Money(ev.Income)))
		case events.Reserved:
			// Clean cash into the offshore account (#195): bookkeeping,
			// and a warning where the move was over the lot.
			reserved += ev.Amount + ev.Fee
			line := fmt.Sprintf("Moved %s clean offshore, fee -%s; the account holds %s", format.Money(ev.Amount), format.Money(ev.Fee), format.Money(w.Offshore))
			if ev.Lots > 0 {
				line += fmt.Sprintf(". Over the lot by %s: the DA will read it.", format.Plural(ev.Lots, "lot"))
			}
			rep.Money = append(rep.Money, line)
		case events.FrontGrew:
			d := base
			d.Front = ev.Name
			add("laundering", "FrontGrew", d)
			rep.Money = append(rep.Money, fmt.Sprintf("%s has grown enough to make the paper. The town wonders where the money came from.", ev.Name))
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
			if ev.DriverName != "" {
				line += " " + ev.DriverName + " was driving."
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
			switch {
			case ev.Backed:
				key = "DABought" // #193: the paper knows whose money it was
			case ev.Incumbent:
				key = "DAReElected"
			}
			add("law", key, d)
			rep.Law = append([]string{electionLine(ev)}, rep.Law...) // the courthouse before the small print
		case events.ChiefReplaced:
			d := at(w.Home().ID)
			d.Name, d.Rival = ev.Name, ev.Old
			key := "ChiefReplaced"
			switch ev.Why {
			case "da":
				key = "ChiefReplacedDA"
			case "campaign":
				key = "ChiefReplacedCampaign"
			}
			add("law", key, d)
			rep.Law = append([]string{chiefLine(ev)}, rep.Law...)
		// Campaigns (#193): the money in, and what it bought at the count.
		case events.CampaignBacked:
			backed += ev.Amount
			rep.Law = append(rep.Law, fmt.Sprintf("Put %s clean behind the %s ticket in %s: the campaign holds %s, %s of the city's vote", format.Money(ev.Amount), stanceWords(ev.Ticket), w.CityName(ev.City), format.Money(ev.Total), swingWords(ev.Swing)))
			rep.Money = append(rep.Money, fmt.Sprintf("Campaign in %s -%s clean", w.CityName(ev.City), format.Money(ev.Amount)))
		case events.CampaignLost:
			d := at(ev.City)
			d.Stance = stanceWords(ev.Winner)
			add("law", "CampaignLost", d)
			line := fmt.Sprintf("The %s you paid for in %s lost: DA %s knows who backed the other side. Pressure +%.0f there", format.Money(ev.Cash), w.CityName(ev.City), w.Law.DA.Name, ev.Pressure)
			if ev.Chief {
				line += ", and the mayor named a zealous chief before the count was cold"
			}
			rep.Law = append(rep.Law, line+".")
		case events.CampaignHedged:
			add("law", "CampaignHedged", at(ev.City))
			rep.Law = append(rep.Law, fmt.Sprintf("%s in %s went to both tickets: nobody owes you, and everybody knows it.", format.Money(ev.Cash), w.CityName(ev.City)))
		// The bought law (#42): the envelopes, the leads, the deals on
		// the road, and the day it all stops.
		case events.BribeAccepted:
			bribed += ev.Amount
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
		case events.RaidFellThrough:
			// The favour (#228): the response that did not come.
			d := at(ev.City)
			d.Level = ev.Level
			add("law", "RaidFellThrough", d)
			word := ev.Level
			if word == content.TaskForce {
				word = "task force"
			}
			rep.Heat = append(rep.Heat, fmt.Sprintf("The %s%s fell through: Chief %s's people stood down at the last minute. Nothing taken, nothing cooled, and the file grows by %d: the chief's name is in your ledger now.", word, in(ev.City), w.Law.Chief.Name, ev.Evidence))
		case events.BribeRefused:
			bribed += ev.Amount
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
			bribed += ev.Amount
			who := "Chief " + w.Law.Chief.Name
			if ev.Target == game.BribeDA {
				who = "DA " + w.Law.DA.Name
			}
			d := at(here.ID)
			d.Name = who
			add("heat", "BribeBackfired", d) // the blotter's, about you: notoriety and pressure count it
			rep.Law = append(rep.Law, fmt.Sprintf("%s does not take envelopes: the %s is in an evidence bag. Tomorrow the file grows by %d and the heat by %.0f.", who, format.Money(ev.Amount), ev.Evidence, ev.Heat))
			rep.Money = append(rep.Money, fmt.Sprintf("Envelope for %s, backfired -%s", who, format.Money(ev.Amount)))
		case events.LeadsFiled:
			add("heat", "LeadsFiled", at(here.ID))
			rep.Law = append(rep.Law, fmt.Sprintf("The DA's office has heard about enough envelopes to open a file: tomorrow it grows by %d.", ev.Evidence))
		case events.OfficialsCold:
			add("law", "OfficialsCold", at(w.Home().ID))
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
		case events.CheckpointBought:
			checkpoints += ev.Cost
			what := "checkpoint"
			if ev.Mode == "boat" || ev.Mode == "plane" {
				what = "customs agent"
			}
			rep.Shipments = append(rep.Shipments, fmt.Sprintf("The %s on the %s is yours until day %d: the risk on that edge is cut while it holds.", what, ev.Name, ev.Until))
			rep.Money = append(rep.Money, fmt.Sprintf("The %s on the %s -%s", what, ev.Name, format.Money(ev.Cost)))
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
		// The stash houses (#73).
		case events.HouseBought:
			d := at(ev.City)
			d.Name = ev.Name
			addHouses("territory", "HouseBought", d)
			spent += ev.Price
			rep.Money = append(rep.Money, fmt.Sprintf("Took the lease on %s -%s. Rent %s/day clean from tomorrow.", ev.Name, format.Money(ev.Price), format.Money(ev.Rent)))
		case events.HouseRobbed:
			d := at(ev.City)
			d.Name, d.Corner = ev.Name, ev.Corner
			addHouses("territory", "HouseRobbed", d)
			rep.Territory = append(rep.Territory, houseRobbedLine(w, ev))
		case events.HouseRaided:
			d := at(ev.City)
			d.Name, d.Level = ev.Name, ev.Level
			addHouses("heat", "HouseRaided", d)
		case events.HouseCompromised:
			why := map[string]string{"informant": "somebody on the payroll told them", "robbery": "word got out after the robbery", "bust": "they were inside"}[ev.Why]
			rep.Heat = append(rep.Heat, fmt.Sprintf("The police know about %s%s: %s. It is the one the raid finds; move the stock and drop it.", ev.Name, in(ev.City), why))
		case events.HouseLost:
			d := at(ev.City)
			d.Name = ev.Name
			addHouses("territory", "HouseLost", d)
			rep.Territory = append(rep.Territory, fmt.Sprintf("The landlord threw you out of %s%s: %s gone with it. The rent went unpaid.", ev.Name, in(ev.City), format.Plural(ev.Units, "unit")))
		case events.Overdose:
			// The city's story (#47): a headline naming the corner, off
			// the overdoses' own stream, and a LAW line, since the
			// pressure is what it costs you; never a page.
			d := at(ev.City)
			d.Product = w.ProductName(ev.Product)
			d.Corner = ev.CornerName
			if d.Corner == "" {
				d.Corner = "a " + d.City + " corner"
			}
			addOff("overdose:news", "overdose", "Overdose", d)
			where := ev.CornerName
			if where == "" {
				where = "your corners"
			}
			rep.Law = append(rep.Law, fmt.Sprintf("OVERDOSE on %s%s: somebody went down on your %s (quality %.0f). The city is talking, the DA is listening.", where, in(ev.City), w.ProductName(ev.Product), ev.Quality))
		case events.StockCut:
			cutting += ev.Cost
			hand := ""
			if ev.Chemist != "" {
				hand = ", " + ev.Chemist + "'s hand on it"
			}
			rep.Sales = append(rep.Sales, fmt.Sprintf("Cut %d %s into %d%s: quality %.0f → %.0f%s = -%s", ev.Units, w.ProductName(ev.Product), ev.Units+ev.Added, in(ev.City), ev.From, ev.To, hand, format.Money(ev.Cost)))
			rep.Money = append(rep.Money, fmt.Sprintf("Cutting %s -%s", w.ProductName(ev.Product), format.Money(ev.Cost)))
		case events.CookOrdered:
			cooking += ev.Cost
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s is cooking %d %s%s: quality %.0f, ready in %s = -%s", ev.Chemist, ev.Units, w.ProductName(ev.Product), in(ev.City), ev.Quality, format.Plural(ev.Days, "day"), format.Money(ev.Cost)))
			rep.Money = append(rep.Money, fmt.Sprintf("Precursors for %s -%s", w.ProductName(ev.Product), format.Money(ev.Cost)))
		case events.Cooked:
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s's batch landed%s: %d %s at quality %.0f, paid %s on the order.", ev.Chemist, in(ev.City), ev.Units, w.ProductName(ev.Product), ev.Quality, format.Money(ev.Cost)))
		case events.AssetBought:
			// The assets (#48): a clean-cash purchase the paper notices,
			// off the assets' own stream, so no pinned run moves.
			d := at(ev.City)
			d.Asset = ev.Name
			addOff("assets:news", "laundering", "AssetBought", d)
			spent += ev.Cost
			rep.Money = append(rep.Money, fmt.Sprintf("Bought %s -%s clean. It stands from today, %s/day clean to keep.", ev.Name, format.Money(ev.Cost), format.Money(ev.Upkeep)))
		case events.AssetFrozen:
			rep.Money = append(rep.Money, fmt.Sprintf("%s stands idle for %s: %s upkeep unpaid. Wash something.", ev.Name, format.Plural(ev.Days, "day"), format.Money(ev.Upkeep)))
		case events.TaskForceFormed:
			d := at(ev.City)
			addOff("assets:news", "heat", "TaskForceFormed", d)
			line := fmt.Sprintf("A TASK FORCE has formed%s. It comes tomorrow night", in(ev.City))
			if ev.Assets > 0 {
				line += " and it will take an asset with it"
			}
			rep.Heat = append(rep.Heat, line+". Lie low: what they find on a quiet night is not a case.")
		case events.AssetSeized:
			d := at(ev.City)
			d.Asset = ev.Name
			addOff("assets:news", "heat", "AssetSeized", d)
			rep.Heat = append(rep.Heat, fmt.Sprintf("  they took %s: %s of yours, gone.", ev.Name, format.Money(ev.Cost)))
		case events.TunnelFound:
			d := base
			d.Asset, d.Route, d.Product = ev.Name, ev.Name, w.ProductName(ev.Product)
			addOff("assets:news", "heat", "TunnelFound", d)
			rep.Shipments = append(rep.Shipments, fmt.Sprintf("THE TUNNEL IS FOUND: %d %s taken in it, and it is shut for good.", ev.Units, w.ProductName(ev.Product)))
		case events.RentPaid:
			rent += ev.Amount
			if ev.Amount > 0 {
				rep.Money = append(rep.Money, fmt.Sprintf("Rent on %s -%s clean", format.Plural(ev.Houses, "house"), format.Money(ev.Amount)))
			}
			if len(ev.Unpaid) > 0 {
				rep.Territory = append(rep.Territory, fmt.Sprintf("Rent unpaid at %s: no clean cash. The landlord will not wait long.", strings.Join(ev.Unpaid, ", ")))
			}
		case events.StockMoved:
			rep.Territory = append(rep.Territory, fmt.Sprintf("Moved %d %s from %s to %s%s.", ev.Units, w.ProductName(ev.Product), ev.From, ev.To, in(ev.City)))
		// The property (#194). DeedBought and DeedRent are bookkeeping;
		// the purchase past the line and the forfeiture make the paper
		// under the laundering source (a story about your money, as a
		// front's growth is: notoriety, and pressure where you are),
		// off the deeds' own stream: a run with no deed is the run it
		// was.
		case events.DeedBought:
			deeds += ev.Price
			rep.Money = append(rep.Money, fmt.Sprintf("Bought the block %s is on%s -%s clean. It pays %s/day clean.", ev.Name, in(ev.City), format.Money(ev.Price), format.Money(ev.Rent)))
		case events.DeedsBought:
			d := at(ev.City)
			d.Corner, d.Qty = ev.Name, ev.Count
			addOff("deeds:news", "laundering", "DeedsBought", d)
			rep.Law = append(rep.Law, fmt.Sprintf("%s makes the paper: %s in your name now. The town wonders where the money came from.", ev.Name, format.Plural(ev.Count, "block")))
		case events.DeedRent:
			deedRent += ev.Amount
			rep.Money = append(rep.Money, fmt.Sprintf("Rent from %s +%s clean", format.Plural(ev.Deeds, "block"), format.Money(ev.Amount)))
		case events.DeedSeized:
			d := at(ev.City)
			d.Corner = ev.Name
			addOff("deeds:news", "laundering", "DeedSeized", d)
			rep.Law = append(rep.Law, fmt.Sprintf("FORFEITURE: the DA seized the block %s is on%s (%s). %s in deeds against %s washed; the money has no story, and the file will grow in the morning.", ev.Name, in(ev.City), format.Money(ev.Price), format.Money(ev.Spent), format.Money(ev.Washed)))
		// Intel (#45): the facts filed tonight are the report's INTEL
		// section; a spy going under, one found and a lie that bit are
		// news, their templates picked off the intel side stream.
		case events.IntelGained:
			rep.Intel = append(rep.Intel, intelLine(w, ev))
		case events.SpyPlanted:
			d := base
			d.Name, d.Role = ev.Name, ev.Role
			d = crew(d, ev.Rival)
			addIntel("crew", "SpyPlanted", d)
			rep.Intel = append(rep.Intel, fmt.Sprintf("%s went under with %s's crew tonight. They sell nothing for you now and report every few days; the intel screen (9) keeps what they send.", ev.Name, ev.Rival))
		case events.SpyFound:
			d := base
			d.Name, d.Role = ev.Name, ev.Role
			d = crew(d, ev.Rival)
			switch {
			case ev.Dead:
				addIntel("crew", "SpyShot", d)
				rep.Intel = append(rep.Intel, fmt.Sprintf("%s's people made %s. They were found shot. %s.", ev.Rival, ev.Name, format.Plural(ev.Reports, "report")+" came back before it"))
			case ev.Why == "gone":
				rep.Intel = append(rep.Intel, fmt.Sprintf("%s's crew is no more; %s came home with nothing to add.", ev.Rival, ev.Name))
			default:
				addIntel("crew", "SpyFound", d)
				rep.Intel = append(rep.Intel, fmt.Sprintf("%s's people made %s and sent them home. They are back on the payroll, %s to their name.", ev.Rival, ev.Name, format.Plural(ev.Reports, "report")))
			}
		case events.IntelFalse:
			d := base
			d = crew(d, ev.Rival)
			switch ev.FactKind {
			case game.FactRisk:
				d.Route = ev.Name
				addIntel("rivals", "IntelFalseRoute", d)
				rep.Intel = append(rep.Intel, fmt.Sprintf("The word on %s was %s's: they had customs waiting. The intel screen (9) names them now.", ev.Name, ev.Rival))
			default:
				d.Corner = ev.Name
				addIntel("rivals", "IntelFalseStash", d)
				rep.Intel = append(rep.Intel, fmt.Sprintf("The till on %s was empty: the word was %s's. The intel screen (9) names them now.", ev.Name, ev.Rival))
			}
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
	for _, b := range w.Today.Buys {
		if b.Contract && b.Day != t.Day {
			continue // yesterday's contract receipts, kept for the cart, were paid for yesterday
		}
		from := ""
		if sup := w.Supplier(b.Supplier); sup != nil {
			from = " from " + sup.Name
		}
		if b.Lieutenant != "" && !b.Contract {
			from += " through " + b.Lieutenant // a buy into their city from elsewhere (#174)
		}
		line := fmt.Sprintf("Bought %d %s at %s%s = -%s", b.Qty, w.ProductName(b.Product), format.Price(b.UnitPrice), from, format.Money(b.Cost))
		switch {
		case b.Credit:
			// On the book, not out of the till (#72).
			line = fmt.Sprintf("Bought %d %s at %s%s on credit = %s on the book", b.Qty, w.ProductName(b.Product), format.Price(b.UnitPrice), from, format.Money(b.Cost))
		case b.Contract && b.Lieutenant != "":
			spent += b.Cost
			line = fmt.Sprintf("%s's restock: %d %s at %s%s = -%s", b.Lieutenant, b.Qty, w.ProductName(b.Product), format.Price(b.UnitPrice), from, format.Money(b.Cost))
		case b.Contract:
			spent += b.Cost
			line = fmt.Sprintf("Supply contract: %d %s at %s%s = -%s", b.Qty, w.ProductName(b.Product), format.Price(b.UnitPrice), from, format.Money(b.Cost))
		default:
			spent += b.Cost
		}
		rep.Money = append(rep.Money, line)
	}
	for _, m := range w.Crew.HiredToday {
		spent += m.Fee
		rep.Money = append(rep.Money, fmt.Sprintf("Signing fee for %s -%s", m.Name, format.Money(m.Fee)))
	}
	rep.CashBefore = w.Cash() - soldRevenue - contracts + forfeits + lostCash + spent + wages + skimmed + robbed + upgrades + upkeep + seized + paidOff + investigated + shipping + tribute + cuts + funded + backed + repaid + rent + scouted + poached + bribed + checkpoints - boosted - earned + invested + cutting + cooking + reserved + deeds - deedRent
	if soldRevenue > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Street sales +%s", format.Money(soldRevenue)))
	}
	if standingCut > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("The crew's cut on the standing orders -%s", format.Money(standingCut)))
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

// swingWords is a campaign's pull on a city's vote (#193), in points.
func swingWords(swing float64) string {
	return fmt.Sprintf("%.1f points", swing*100)
}

// electionLine is the report's line on a DA election.
func electionLine(ev events.DAElected) string {
	if ev.Backed {
		// #193: the ticket you paid for won.
		if ev.Incumbent {
			return fmt.Sprintf("DA %s re-elected on the %s ticket, on your money: the sting line sits higher while they owe you.", ev.Name, stanceWords(ev.Stance))
		}
		return fmt.Sprintf("DA %s elected on the %s ticket, on your money: the sting line sits higher while they owe you.", ev.Name, stanceWords(ev.Stance))
	}
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
	if ev.Why == "campaign" {
		return fmt.Sprintf("The new DA remembers who paid for the other side: %s is out, %s is in, and the word is zealous.", ev.Old, ev.Name)
	}
	if ev.Why == "da" {
		return fmt.Sprintf("The new DA wanted a new chief: %s is out, %s is in. You will learn what they are like.", ev.Old, ev.Name)
	}
	if ev.Why == "resigned" {
		return fmt.Sprintf("Chief %s resigned; %s takes over. You will learn what they are like.", ev.Old, ev.Name)
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
// eyeingWhy is the reason behind the tell (#69), once the rival's
// temper has shown: until then the tell names the corner and the early
// game reads as rumour.
func eyeingWhy(r *game.RivalState) string {
	if r == nil || !r.Observed {
		return ""
	}
	switch r.Personality {
	case "expansionist":
		return ", an expansionist wanting the city"
	case "defensive":
		return ", a defensive outfit filling in next to its own"
	case "opportunist":
		return ", an opportunist taking the best block going"
	case "chaotic":
		return ", chaotic as ever, on a whim"
	}
	return ""
}

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

// saleLine is a sale in the report: the units and the take at the dial,
// then whose the order was where it was not placed that day (the
// lieutenant's by name; `standing` and what the crew kept off one of
// yours, #114).
func saleLine(w *game.World, ev events.PlayerSold) string {
	who := ""
	switch {
	case ev.Delegated:
		who = ", " + ev.LieutenantName
	case ev.Standing:
		who = ", standing"
		if ev.Cut > 0 {
			who += ", cut " + format.Money(ev.Cut)
		}
	}
	if ev.Sold == 0 {
		return fmt.Sprintf("%-8s wanted %d, sold none (%s%s)", w.ProductName(ev.Product), ev.Wanted, ev.Dial, who)
	}
	// The quality's mark on the price (#47), only where it left one.
	if ev.QualityMul > 0 && math.Abs(ev.QualityMul-1) >= 0.005 {
		who += fmt.Sprintf(", quality %.0f ×%.2f", ev.Quality, ev.QualityMul)
	}
	return fmt.Sprintf("%-8s sold %d/%d at %s avg = +%s (%s%s)", w.ProductName(ev.Product), ev.Sold, ev.Wanted, format.Price(ev.AvgPrice), format.Money(ev.Revenue), ev.Dial, who)
}

// lieutenantLines is what a lieutenant's night reads like in the report:
// what they did with the crew and the corners, what their contracts
// bought this morning (#174, `bought 120 Weed for $2,500`), and, once
// you know them, what they are like. A greedy one's skim is missing
// money like anyone else's; the line never says so.
func lieutenantLines(w *game.World, ev events.LieutenantActed) []string {
	var did []string
	for _, b := range ev.Bought {
		did = append(did, fmt.Sprintf("bought %d %s for %s", b.Units, w.ProductName(b.Product), format.Money(b.Cost)))
	}
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

// houseRobbedLine is a stash house robbery for the report (#73).
func houseRobbedLine(w *game.World, ev events.HouseRobbed) string {
	parts := []string{}
	for id, q := range ev.StockLost {
		parts = append(parts, fmt.Sprintf("%d %s", q, w.ProductName(id)))
	}
	sort.Strings(parts)
	s := fmt.Sprintf("%s was ROBBED: lost %s. Word gets out: the police know the house now.", ev.Name, strings.Join(parts, ", "))
	if !ev.Guarded {
		s += " An enforcer inside would have helped."
	}
	return s
}

func enforcementLine(w *game.World, ev events.Enforcement) string {
	switch ev.Level {
	case content.Patrol:
		return "PATROLS: street sales capped for a few days"
	case content.Arrest:
		return "ARRESTED."
	}
	parts := []string{}
	for id, q := range ev.StockLost {
		parts = append(parts, fmt.Sprintf("%d %s", q, w.ProductName(id)))
	}
	sort.Strings(parts)
	word := strings.ToUpper(ev.Level)
	if ev.Level == content.TaskForce {
		word = "TASK FORCE" // #48: the level's id is one word, the paper's two
	}
	s := word + ": lost"
	if ev.House != "" {
		s = word + " at " + ev.HouseName + ": lost"
	}
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

// unlockLine is the report's UNLOCKED line for a gate crossed (#148), in
// the voice of the sections' lines and short enough for the modal at 80
// columns: what opened, where to find it and the line it opened on.
func unlockLine(w *game.World, ev events.Unlocked) string {
	switch ev.Gate {
	case "product":
		// The line is honest where the supplier does not sell it: the
		// port's product reaches home by the road, never through b.
		if p := w.Product(w.Here().ID, ev.ID); p != nil && p.NoSupply {
			var where []string
			for _, cid := range w.CityOrder {
				if cp := w.Product(cid, ev.ID); cp != nil && !cp.NoSupply {
					where = append(where, w.CityName(cid))
				}
			}
			if len(where) > 0 {
				return fmt.Sprintf("%s is on offer in %s only: the road brings it here.", ev.Name, strings.Join(where, " and "))
			}
			return fmt.Sprintf("%s is on the ladder, but no supplier sells it.", ev.Name)
		}
		return fmt.Sprintf("%s is on offer, around %s a unit: %s.", ev.Name, format.Price(ev.Price), ev.Why)
	case "front":
		return fmt.Sprintf("The %s is open to you on the ledger screen (7): %s.", ev.Name, format.Cash(ev.Cost))
	case "connect":
		where := ""
		if ev.City != w.Here().ID && w.Cities[ev.City] != nil {
			where = " in " + w.CityName(ev.City)
		}
		return fmt.Sprintf("%s will deal with you now%s: %s.", ev.Name, where, ev.Why)
	case "role":
		return fmt.Sprintf("%s want work on the crew screen (4): %s.", ev.Name, ev.Why)
	case "asset":
		return fmt.Sprintf("%s is for sale on the ledger screen (7), clean cash: %s.", ev.Name, format.Cash(ev.Cost))
	}
	return fmt.Sprintf("%s is open to you: %s.", ev.Name, ev.Why)
}

// cornerHolder names the leader whose crew holds a corner, for a line
// about it (#43); "the rival" for a corner nobody's.
func cornerHolder(w *game.World, id string) string {
	if c := w.Corner(id); c != nil && c.Owner == game.OwnerRival {
		if r := w.Faction(c.Faction); r != nil && r.Leader != "" {
			return r.Leader
		}
	}
	return "the rival"
}

// pointer is the crew screen pointer for the muscle a fragmented
// faction left in the hiring pool (#43), or nothing for none.
func pointer(muscle int) string {
	if muscle <= 0 {
		return ""
	}
	return fmt.Sprintf(" (%s in the pool, cheap)", format.Plural(muscle, "enforcer"))
}

// intelLine is a filed fact as the report says it (#45): what was
// learnt, of whom, how and how sure.
func intelLine(w *game.World, ev events.IntelGained) string {
	sure := fmt.Sprintf("%.0f%% sure", ev.Confidence*100)
	how := map[string]string{
		game.SourceSeen: "Seen", game.SourceBooks: "The books say", game.SourceCop: "The cop says",
		game.SourceSpy: "Your spy says", game.SourceContact: "A contact says",
	}[ev.Source]
	if how == "" {
		how = "Word is"
	}
	name := ev.Name
	if name == "" {
		name = ev.Subject
	}
	switch ev.FactKind {
	case game.FactPersonality:
		if ev.Subject == game.SubjectChief {
			return fmt.Sprintf("%s: Chief %s is %s (%s).", how, name, ev.Value, sure)
		}
		return fmt.Sprintf("%s: %s is %s (%s).", how, name, ev.Value, sure)
	case game.FactMuscle:
		return fmt.Sprintf("%s: %s has %s heads (%s).", how, name, ev.Value, sure)
	case game.FactMove:
		if c := w.Corner(ev.Value); c != nil {
			return fmt.Sprintf("%s: %s moves on %s next (%s).", how, name, c.Name, sure)
		}
	case game.FactStash:
		if c := w.Corner(ev.Value); c != nil {
			return fmt.Sprintf("%s: %s's till is fattest on %s (%s).", how, name, c.Name, sure)
		}
	case game.FactResponse:
		return fmt.Sprintf("%s: the next thing coming in %s is a %s (%s).", how, name, ev.Value, sure)
	case game.FactRisk:
		return fmt.Sprintf("%s: %s is seized %s in transit (%s).", how, name, ev.Value, sure)
	}
	return fmt.Sprintf("%s: %s %s %s (%s).", how, name, ev.FactKind, ev.Value, sure)
}
