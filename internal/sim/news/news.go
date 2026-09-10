// Package news turns the day's events into headlines and the morning
// report. It runs last so it sees everything the other sims emitted.
package news

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the news simulation.
type Sim struct {
	cfg  content.HeadlinesConfig
	tmpl map[string][]*template.Template
	flav []*template.Template
}

// New parses the headline templates once.
func New(cfg content.HeadlinesConfig) (*Sim, error) {
	s := &Sim{cfg: cfg, tmpl: map[string][]*template.Template{}}
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
	var soldRevenue, lostCash, wages, skimmed int
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.PriceMove:
			rep.Prices = append(rep.Prices, priceLine(w, ev))
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
			m := w.Market[ev.Product]
			switch {
			case ev.Sold == 0:
				add("market", "PlayerSoldZero", d)
			case ev.Dial == events.DialAggressive || (m != nil && float64(ev.Sold) >= m.Demand*1.2):
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
			if ev.Level == "sting" || ev.Level == "raid" {
				rep.Heat = append(rep.Heat, fmt.Sprintf("  the DA's file on you grows (%d)", w.Heat.Evidence))
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
			add("crew", "CrewFired", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("You let %s go. The others noticed.", ev.Name))
		case events.CrewQuit:
			d := base
			d.Name, d.Role = ev.Name, ev.Role
			add("crew", "CrewQuit", d)
			rep.Crew = append(rep.Crew, fmt.Sprintf("%s walked. Nobody was surprised.", ev.Name))
		case events.CrewSkimmed:
			add("crew", "CrewSkimmed", base)
			skimmed += ev.Amount
			rep.Crew = append(rep.Crew, fmt.Sprintf("$%d of the takings never made it back. Somebody is skimming.", ev.Amount))
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
	// player woke up with: undo today's buys, fees, sales, wages, skims and
	// seizures from the current total.
	spent := 0
	for _, b := range w.Buys {
		spent += b.Cost
		rep.Money = append(rep.Money, fmt.Sprintf("Bought %d %s at $%.0f = -$%d", b.Qty, w.ProductName(b.Product), b.UnitPrice, b.Cost))
	}
	for _, m := range w.Crew.HiredToday {
		spent += m.Fee
		rep.Money = append(rep.Money, fmt.Sprintf("Signing fee for %s -$%d", m.Name, m.Fee))
	}
	rep.CashBefore = w.Cash() - soldRevenue + lostCash + spent + wages + skimmed
	if soldRevenue > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Street sales +$%d", soldRevenue))
	}
	if skimmed > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Missing from the count -$%d", skimmed))
	}
	if lostCash > 0 {
		rep.Money = append(rep.Money, fmt.Sprintf("Seized by police -$%d", lostCash))
	}

	// Flavour keeps the ticker alive on quiet days.
	if len(s.flav) > 0 && t.RNG.Float64() < s.cfg.FlavourChance {
		txt := render(s.flav[t.RNG.IntN(len(s.flav))], base)
		lines = append(lines, game.Headline{Day: t.Day, Source: "news", Text: txt})
	}

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
	return fmt.Sprintf("%-6s $%7.2f %s $%7.2f (%+.0f%%)", w.ProductName(ev.Product), ev.From, arrow, ev.To, pct)
}

func saleLine(w *game.World, ev events.PlayerSold) string {
	if ev.Sold == 0 {
		return fmt.Sprintf("%-6s wanted %d, sold none (%s)", w.ProductName(ev.Product), ev.Wanted, ev.Dial)
	}
	return fmt.Sprintf("%-6s sold %d/%d at $%.2f avg = +$%d (%s)", w.ProductName(ev.Product), ev.Sold, ev.Wanted, ev.AvgPrice, ev.Revenue, ev.Dial)
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
