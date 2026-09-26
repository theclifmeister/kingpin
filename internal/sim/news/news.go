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
	rcfg content.RivalEndingsTuning // the kingpin's share, for the paper's title (#233)
	tmpl map[string][]*template.Template
	flav []*template.Template
	swag []*template.Template // the boss's headlines (#233)
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
	s := &Sim{cfg: cfg.Headlines, dcfg: cfg.Dilemmas, pcfg: cfg.Progression, rcfg: cfg.Rivals.Endings, tmpl: map[string][]*template.Template{}, deck: deck, inc: map[string]*template.Template{}}
	for i, src := range cfg.Headlines.Swagger {
		t, err := template.New(fmt.Sprintf("swagger#%d", i)).Funcs(articles).Parse(article(src))
		if err != nil {
			return nil, fmt.Errorf("swagger[%d]: %w", i, err)
		}
		s.swag = append(s.swag, t)
	}
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
	"a":   format.A,
	"the": format.The,
	"The": func(name string) string {
		s := format.The(name)
		return strings.ToUpper(s[:1]) + s[1:]
	},
	"A": func(noun string) string {
		s := format.A(noun)
		return strings.ToUpper(s[:1]) + s[1:]
	},
}

// articleRE is an indefinite article written before a field in a
// template source: `a {{.City}}`, `An {{.Product}}`.
var articleRE = regexp.MustCompile(`\b([Aa])n? \{\{(\.\w+)\}\}`)

// article rewrites every `a {{.X}}` in a template source to `{{a .X}}`
// and every `the {{.X}}` to `{{the .X}}`,
// so the article agrees with the value (`an Eastside outfit`, `a
// Bayport outfit`; #148: the file wrote `a {{.City}}` and Eastside
// read `a Eastside`). The value alone is what decides it, so a
// template's own words are left as written.
func article(src string) string {
	return theRE.ReplaceAllString(articleRE.ReplaceAllString(src, "{{$1 $2}}"), "{{${1}he $2}}")
}

// theRE is a definite article written before a field: `the {{.Route}}`,
// rewritten to `{{the .Route}}` so a name with its own (`The Channel`)
// is not given a second (#502).
var theRE = regexp.MustCompile(`\b([Tt])he \{\{(\.\w+)\}\}`)

// HasTemplate reports whether a template key exists; tests use it to make
// sure every event kind the sims emit can be reported.
func (s *Sim) HasTemplate(key string) bool { return len(s.tmpl[key]) > 0 }

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
	Title   string // what the paper calls you (#233): a dealer, a crew, the boss of Eastside
	Trait   string // a veteran's trait (#346)
}

// Step writes headlines into the journal and assembles the morning report.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	rep := &game.DayReport{Day: t.Day}

	here := w.Here()
	base := data{City: here.Name, DA: w.Law.DA.Name, Chief: w.Law.Chief.Name, Leader: w.Rival().Leader, Title: s.title(w)}
	if w.Rival().Leader != "" {
		base.Faction = w.Rival().Leader + "'s crew"
	}
	r := &reporter{s: s, w: w, t: t, rep: rep, here: here, base: base, routeCost: map[string]int{}, flow: map[string]game.Pools{}}

	// The tier (#147), first: the run enters the first tier past the
	// highest reached whose trigger holds this morning, one a morning,
	// so a night that crosses two lines is two mornings and each has
	// one thing to say. The headline picks its template off the
	// progression's own stream: the home city's dice never move for
	// it, and a run before #147 replays as it did.
	if n := s.tier(w, t); n > 0 {
		tier := s.pcfg.Tier(n)
		rep.Tier = tierLines(n, len(s.pcfg.Tiers), *tier)
		r.addOff(game.StreamProgression, "news", "TierReached", base)
	}
	// The rich list (#392): a line crossed is a headline off its own
	// stream and a line under the tier.
	if ev, ok := s.richList(w, t); ok {
		d := base
		d.Qty, d.Name = ev.Rank, format.Cash(ev.NetWorth)
		r.addOff(game.StreamRichNews, "news", "RichListed", d)
		rep.Tier = append(rep.Tier, fmt.Sprintf("THE RICH LIST puts you at #%d, worth %s. Nobody there can say where it came from.", ev.Rank, format.Cash(ev.NetWorth)))
	}
	// The reign (#227): while the city is yours the TIER section opens
	// with where the reign stands, its homage counted off tonight's
	// TributePaid; the morning it began carries the headline, and the
	// morning it broke says how.
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.ReignBegan:
			d := r.at(ev.City)
			r.add("rivals", "ReignBegan", d)
			line := fmt.Sprintf("The city is yours: every crew in %s is gone or paying. Take the crown when you are ready (walk away on the dashboard), or reign.", d.City)
			if ev.Again {
				line = fmt.Sprintf("The city is yours again: every crew in %s is gone or paying, and the crown is open.", d.City)
			}
			rep.Tier = append([]string{line}, rep.Tier...)
		case events.ReignBroken:
			rep.Tier = append([]string{fmt.Sprintf("The reign is over for now: %s. Hold the city and the table and it begins again.", ev.Why)}, rep.Tier...)
		case events.StraightOpened:
			// Going straight (#398): the businessman's streak is in, and
			// the ending is the player's to take.
			rep.Tier = append([]string{fmt.Sprintf("The fronts earn %s a day against %s off the street: you could go straight (walk away on the dashboard), or play on.", format.Money(ev.Income), format.Money(ev.Street))}, rep.Tier...)
		case events.StraightLapsed:
			rep.Tier = append([]string{"The street out-earned the fronts, or the city turned: going straight is off until the books carry you again."}, rep.Tier...)
		}
	}
	if w.Reign > 0 {
		rep.Tier = append([]string{reignLine(w, t)}, rep.Tier...)
	}
	// The morning opens on you (#233): what your name did last night,
	// when there is something to say, under the reign's line.
	swagger, ground := swaggerLines(w, t)
	rep.Tier = append(swagger, rep.Tier...)
	rep.Territory = append(ground, rep.Territory...)

	// The world's incident (#44), dealt first thing this tick: a
	// headline under the world source, its template picked off the
	// incidents' own side stream (the home city's dice never move for
	// it), and the report opens with the row's line, before the tier.
	for _, e := range t.Events() {
		ev, ok := e.(events.Incident)
		if !ok {
			continue
		}
		d := r.at(ev.City)
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
					d = r.crew(d, k.Rival)
				}
			}
		}
		key := content.IncidentConfig{ID: ev.ID}.Key()
		if !s.HasTemplate(key) {
			key = "Incident"
			d.Name = ev.Name
		}
		r.addOff(game.StreamIncidentsNews, "world", key, d)
		if tm := s.inc[ev.ID]; tm != nil {
			rep.Incident = append(rep.Incident, render(tm, d))
		} else {
			rep.Incident = append(rep.Incident, ev.Name+".")
		}
	}

	for _, e := range t.Events() {
		r.report(e)
	}

	// What each route cost today, lots and fares together: the money
	// section carries the one line, the shipments section the total.
	for _, name := range r.routeOrder {
		rep.Shipments = append(rep.Shipments, fmt.Sprintf("The %s cost %s today, lots and fares.", name, format.Money(r.routeCost[name])))
		rep.Money = append(rep.Money, fmt.Sprintf("The %s: lots and fares -%s", name, format.Money(r.routeCost[name])))
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
			// The unit is the credit price; its markup is said (#502).
			markup := ""
			if sup := w.Supplier(b.Supplier); sup != nil && sup.CreditRatio > 1 {
				markup = " (" + format.Times(sup.CreditRatio, 2) + " on credit)"
			}
			line = fmt.Sprintf("Bought %d %s at %s%s%s = %s on the book", b.Qty, w.ProductName(b.Product), format.Price(b.UnitPrice), markup, from, format.Money(b.Cost))
		case b.Contract && b.Lieutenant != "":
			r.book(game.FlowPurchases, -b.Cost, 0)
			line = fmt.Sprintf("%s's restock: %d %s at %s%s = -%s", b.Lieutenant, b.Qty, w.ProductName(b.Product), format.Price(b.UnitPrice), from, format.Money(b.Cost))
		case b.Contract:
			r.book(game.FlowPurchases, -b.Cost, 0)
			line = fmt.Sprintf("Supply contract: %d %s at %s%s = -%s", b.Qty, w.ProductName(b.Product), format.Price(b.UnitPrice), from, format.Money(b.Cost))
		default:
			r.book(game.FlowPurchases, -b.Cost, 0)
		}
		rep.Money = append(rep.Money, line)
	}
	for _, m := range w.Crew.HiredToday {
		r.book(game.FlowRoutes, -m.Fee, 0)
		rep.Money = append(rep.Money, fmt.Sprintf("Signing fee for %s -%s", m.Name, format.Money(m.Fee)))
	}
	// What the day's errands cost as they were paid (#351): a cop's
	// word and a look at the books leave the pile the moment they are
	// bought, whatever the night makes of them, so the flow reads the
	// orders, not the events.
	if o := w.Today.Cop; o != nil && o.Amount > 0 {
		r.book(game.FlowRoutes, -o.Amount, 0)
		rep.Money = append(rep.Money, fmt.Sprintf("A cop's word -%s", format.Money(o.Amount)))
	}
	// Clean cash drawn back into the dirty pile (#395): at once, so the
	// flow reads the order, the laundering line both ways.
	if o := w.Today.CashedOut; o.Amount > 0 {
		r.book(game.FlowLaundering, o.Amount-o.Fee, -o.Amount)
		rep.Money = append(rep.Money, fmt.Sprintf("Cashed out %s clean: +%s dirty, the banker kept -%s", format.Money(o.Amount), format.Money(o.Amount-o.Fee), format.Money(o.Fee)))
	}
	if o := w.Today.Scouting; o != nil {
		r.book(game.FlowRoutes, -(o.Cost - o.Clean), -o.Clean)
		if !r.scouted {
			rep.Money = append(rep.Money, fmt.Sprintf("Scouting books nobody keeps now -%s", format.Money(o.Cost)))
		}
	}
	if o := w.Today.Poach; o != nil {
		r.book(game.FlowInvestments, -o.Cost, 0)
		if !r.poached {
			rep.Money = append(rep.Money, fmt.Sprintf("Buying off muscle nobody pays now -%s", format.Money(o.Cost)))
		}
	}
	// The card answered this morning, where it moved money: its choice
	// wrote the piles before the night began.
	if a := w.Dilemmas.Answered; a != nil && a.Cash != (game.Pools{}) {
		r.book(game.FlowOther, a.Cash.Dirty, a.Cash.Clean)
		rep.Money = append(rep.Money, fmt.Sprintf("%s: %s", a.Title, format.Signed(a.Cash.Total())))
	}
	if r.soldRevenue > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Street sales +%s", format.Money(r.soldRevenue)))
	}
	if r.standingCut > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("The crew's cut on the standing orders -%s", format.Money(r.standingCut)))
	}
	if r.skimmed > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Missing from the count -%s", format.Money(r.skimmed)))
	}
	if r.seized > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Seized by the auditors -%s", format.Money(r.seized)))
	}

	// Flavour keeps the ticker alive on quiet days.
	if len(s.flav) > 0 && t.RNG.Float64() < s.cfg.FlavourChance {
		txt := render(s.flav[t.RNG.IntN(len(s.flav))], base)
		r.lines = append(r.lines, game.Headline{Day: t.Day, Source: "news", Text: txt})
	}
	// The paper names the boss (#233): while the city is yours in the
	// kingpin's sense, a swagger headline at the flavour's chance off
	// its own stream, so the home stream and every run that never held
	// the city are what they were.
	if len(s.swag) > 0 && s.boss(w) {
		if rng := t.Sub(game.StreamSwagger); rng.Float64() < s.cfg.FlavourChance {
			txt := render(s.swag[rng.IntN(len(s.swag))], base)
			r.lines = append(r.lines, game.Headline{Day: t.Day, Source: "news", Text: txt})
		}
	}

	// Yesterday's card: the choice is already in the journal (Choose put
	// it there); this is the morning after, when the follow-up makes the
	// paper. Then, maybe, tonight's card.
	if a := w.Dilemmas.Answered; a != nil {
		t.Emit(events.DilemmaAnswered{Day: t.Day, Card: a.Card, Choice: a.Choice})
		if a.Headline != "" {
			r.lines = append(r.lines, game.Headline{Day: t.Day, Source: "dilemma", Text: a.Headline})
		}
	}
	s.drawCard(w, t)

	for _, h := range r.lines {
		w.Journal = append(w.Journal, h)
		rep.News = append(rep.News, h.Text)
		t.Emit(events.Headline{Day: h.Day, Source: h.Source, Text: h.Text})
	}
	// The night's cash flow (#351): the lines booked above, closed on
	// the piles as they stand, the opening worked back from them. CASH
	// BEFORE and AFTER are its two ends, and the history keeps the last
	// [flow] days of them.
	rep.Flow = game.NewCashFlow(t.Day, r.flow, game.Pools{Dirty: w.Player.DirtyCash, Clean: w.Player.CleanCash})
	rep.CashBefore = rep.Flow.Opening.Total()
	rep.CashAfter = w.Cash()
	// The lead (#354): the night's biggest changes, read before the
	// history takes tonight's flow, and kept in the journal under the
	// digest's own source so its filter reads them day by day. They are
	// no headline the bus carries: the report is where they are read.
	rep.Lead = s.lead(w, t, rep.Flow)
	for _, l := range rep.Lead {
		w.Journal = append(w.Journal, game.Headline{Day: t.Day, Source: "digest", Text: l.Text})
	}
	w.Report = rep
	w.Flows = append(w.Flows, rep.Flow)
	if n := len(w.Flows) - s.cfg.Flow.Days; n > 0 {
		w.Flows = append([]game.CashFlow(nil), w.Flows[n:]...)
	}
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
		who += fmt.Sprintf(", quality %.0f %s", ev.Quality, format.Times(ev.QualityMul, 2))
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

// captainLines is the report's CREW line for a captain's night (#346):
// what they did for the city (a line for each thing done), or that they
// were in no state to, or have stopped caring; nothing for a quiet
// night. What it cost is the MONEY section's.
func captainLines(ev events.CaptainActed) []string {
	switch {
	case ev.Absent:
		return []string{fmt.Sprintf("%s is in no state to look after %s's crew tonight.", ev.Name, ev.CityName)}
	case ev.Careless:
		return []string{fmt.Sprintf("%s has stopped caring about %s's crew. Nobody is looking after them.", ev.Name, ev.CityName)}
	}
	var did []string
	if n := len(ev.Pulled); n > 0 {
		did = append(did, fmt.Sprintf("pulled %s off the corner, skimming suspected", strings.Join(ev.Pulled, ", ")))
	}
	if n := len(ev.Posted); n > 0 {
		did = append(did, fmt.Sprintf("posted %s on %s", count(n, "runner"), strings.Join(ev.Posted, ", ")))
	}
	if n := len(ev.Paid); n > 0 {
		did = append(did, fmt.Sprintf("paid off %s for %s", strings.Join(ev.Paid, ", "), format.Money(ev.Spent)))
	}
	if len(did) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("%s looked after %s's crew: %s.", ev.Name, ev.CityName, strings.Join(did, "; "))}
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
	if ev.Enforcer != "" {
		// The guard was there (#502): name them and what they bought.
		return s + fmt.Sprintf(". %s was on the corner and cut the odds from %s to %s a night; it happened anyway.", ev.Enforcer, format.Pct(ev.Bare, 1), format.Pct(ev.Odds, 1))
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

// seizedLine is a raid's cash in the MONEY section (#351), naming the
// cause: `Seized by police in Eastside -$42,000 (the raid on the stash)`.
func seizedLine(w *game.World, ev events.Enforcement) string {
	what := strings.ToLower(ev.Level)
	if ev.Level == content.TaskForce {
		what = "task force"
	}
	where := "the stash"
	if ev.House != "" {
		where = ev.HouseName
	}
	return fmt.Sprintf("Seized by police in %s -%s (the %s on %s)", w.CityName(ev.City), format.Money(ev.CashLost), what, where)
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
		if len(parts) > 0 {
			s += " and"
		}
		s += " " + format.Money(ev.CashLost) // #465: `STING: lost $1,486`, never `lost and $1,486`
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
		if w.Crew.OnPayroll(ev.ID) > 0 {
			// The character started with one (the Cook's chemist, #473).
			return fmt.Sprintf("More %s want work on the crew screen (4): %s.", strings.ToLower(ev.Name), ev.Why)
		}
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
	sure := format.Pct(ev.Confidence, 0) + " sure"
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
	case game.FactScout:
		return fmt.Sprintf("%s: %s has scouts in %s (%s).", how, name, w.CityName(ev.Value), sure)
	}
	return fmt.Sprintf("%s: %s %s %s (%s).", how, name, ev.FactKind, ev.Value, sure)
}
