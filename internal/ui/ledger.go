package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// frontRows lists the fronts on offer that the player does not own yet,
// cheapest first, locked ones included so the ladder is visible.
func (m *Model) frontRows() []game.FrontOffer {
	var rows []game.FrontOffer
	for _, o := range m.set.Laundering.Offers() {
		if m.w.Front(o.ID) == nil {
			rows = append(rows, o)
		}
	}
	return rows
}

// askFront opens the picker for buying a front.
func (m *Model) askFront() {
	if m.w.Over != nil {
		return
	}
	if len(m.frontRows()) == 0 {
		m.status = "You own every front there is."
		return
	}
	m.frontCursor = 0
	m.mode = modeFront
}

func (m *Model) confirmFront() {
	rows := m.frontRows()
	m.mode = modePlay
	if len(rows) == 0 {
		return
	}
	o := rows[max(0, min(m.frontCursor, len(rows)-1))]
	f, err := m.w.BuyFront(o)
	if err != nil {
		m.status = "Can't buy: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("Bought %s for %s. It opens tomorrow, washing up to %s/day.", f.Name, money(o.Cost), money(m.set.Laundering.Throughput(m.w, f)))
}

func (m *Model) cycleLaunder() {
	d := (m.w.Laundering.Dial + 1) % 3
	m.w.SetLaunderDial(d)
	if len(m.w.Fronts) == 0 {
		m.status = fmt.Sprintf("Launder dial %s. %s Buy a front on the ledger (7) to use it.", d, launderBlurb(d))
		return
	}
	m.status = fmt.Sprintf("Launder dial %s: washing up to %s/day, audit risk %.1f%%/day. %s", d, money(m.set.Laundering.Capacity(m.w)), m.set.Laundering.AnyAuditRisk(m.w)*100, launderBlurb(d))
}

func launderBlurb(d events.Launder) string {
	switch d {
	case events.LaunderCareful:
		return "Slow and quiet."
	case events.LaunderGreedy:
		return "Fast, and an audit is evidence."
	default:
		return "Steady."
	}
}

// frontStatus is a front's state for the ledger's status column, in
// the one lowercase vocabulary: open, opens tomorrow, audit, back in
// 14d, shut, back in 2d.
func (m *Model) frontStatus(f game.Front) any {
	switch {
	case f.Frozen(m.w.Day+1) && f.Audited > 0 && f.FrozenUntil == f.Audited+m.set.Laundering.Tuning().AuditFreezeDays:
		return styled{theme.Bad, fmt.Sprintf("audit, back in %dd", f.FrozenUntil-m.w.Day)}
	case f.Frozen(m.w.Day + 1):
		return styled{theme.Warning, fmt.Sprintf("shut, back in %dd", f.FrozenUntil-m.w.Day)}
	case f.Bought == m.w.Day:
		return styled{theme.Subtle, "opens tomorrow"}
	default:
		return styled{theme.Good, "open"}
	}
}

// offerCols and offerRows are the fronts on offer, on the ledger and in
// the picker: what one costs, washes and keeps, its audit risk, and
// whether it is open to you, locked until you have moved enough, or
// short of what is in the till.
var offerCols = []col{{"front", kText, 0}, {"cost", kMoney, 0}, {"washes/day", kMoney, 0}, {"upkeep/day", kMoney, 0}, {"audit", kPct, 0}, {"status", kText, 0}}

func (m *Model) offerRows(rows []game.FrontOffer) [][]any {
	var out [][]any
	for _, o := range rows {
		var status any
		switch {
		case o.Locked(m.w):
			status = styled{theme.Subtle, "locked at " + cash(o.UnlockCash)}
		case o.Cost > m.w.Player.DirtyCash:
			status = styled{theme.Bad, "short " + money(o.Cost-m.w.Player.DirtyCash)}
		default:
			status = "open to you"
		}
		out = append(out, []any{o.Name, o.Cost, o.Throughput, o.Upkeep, o.AuditRisk * 100, status})
	}
	return out
}

func (m *Model) viewFront() string {
	rows := m.frontRows()
	if len(rows) == 0 {
		return m.modal("BUY A FRONT", "Nothing for sale.")
	}
	m.frontCursor = max(0, min(m.frontCursor, len(rows)-1))
	var b strings.Builder
	for _, l := range table(offerCols, m.offerRows(rows), m.frontCursor, max(30, m.width-6)) {
		b.WriteString(l + "\n")
	}
	b.WriteString("\n" + theme.Subtle.Render(fmt.Sprintf("Dirty cash %s. It opens tomorrow.  ", cash(m.w.Player.DirtyCash))) + theme.Key.Render("enter") + " buy  " + theme.Key.Render("esc") + " back")
	return m.modal("BUY A FRONT", strings.TrimRight(b.String(), "\n"))
}

func (m *Model) viewLedger() string {
	w := m.w
	l := m.set.Laundering
	tun := l.Tuning()
	var b strings.Builder

	b.WriteString(truncate(theme.PanelTitle.Render("LEDGER")+"  "+theme.Gold.Render("dirty "+cash(w.Player.DirtyCash))+theme.Subtle.Render(" · ")+theme.Good.Render("clean "+cash(w.Player.CleanCash))+
		theme.Subtle.Render(fmt.Sprintf(" · seized %s lifetime", cash(w.Stats.Seized))), m.width) + "\n")
	var dial []string
	for d := events.LaunderCareful; d <= events.LaunderGreedy; d++ {
		if d == w.Laundering.Dial {
			dial = append(dial, theme.Selected.Render(" "+d.String()+" "))
		} else {
			dial = append(dial, theme.Subtle.Render(" "+d.String()+" "))
		}
	}
	b.WriteString(truncate("  dial "+strings.Join(dial, "")+theme.Subtle.Render(fmt.Sprintf("  audit risk %.1f%%/day · d turns it; a greedy audit is evidence", l.AnyAuditRisk(w)*100)), m.width) + "\n")
	b.WriteString(truncate(theme.Subtle.Render(fmt.Sprintf("  washing up to %s/day · upkeep %s/day · the till keeps %s dirty for the street",
		money(l.Capacity(w)), money(l.Upkeep(w)), cash(tun.Float))), m.width) + "\n")
	if thr := m.cfg.Heat.Heat.DirtyCashThreshold; thr > 0 && w.Player.DirtyCash > thr {
		b.WriteString(truncate(theme.Warning.Render(fmt.Sprintf("  ▲ Dirty cash over %s draws heat every day it sits there.", cash(thr))), m.width) + "\n")
	}
	b.WriteString("\n")

	b.WriteString(theme.Bold.Render("FRONTS") + theme.Subtle.Render(fmt.Sprintf("  %d owned · washed %s lifetime", len(w.Fronts), cash(w.Stats.Laundered))) + "\n")
	if len(w.Fronts) == 0 {
		b.WriteString(truncate(theme.Subtle.Render("  None. A front turns dirty cash into clean cash a little every day; b buys one."), m.width) + "\n")
	} else {
		var rows [][]any
		for _, f := range w.Fronts {
			rows = append(rows, []any{f.Name, l.Throughput(w, f), f.WashedToday, f.Washed, l.AuditRisk(w, f) * 100, m.frontStatus(f)})
		}
		for _, line := range table([]col{{"front", kText, 0}, {"washes/day", kMoney, 0}, {"today", kMoney, 0}, {"lifetime", kMoney, 0}, {"audit", kPct, 0}, {"status", kText, 0}}, rows, -1, m.width) {
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n")

	if n := w.Crew.Role("accountant"); n > 0 {
		b.WriteString(truncate(theme.Subtle.Render(fmt.Sprintf("  %d accountant(s) on the payroll: more through every front, fewer audits.", n)), m.width) + "\n")
	} else if len(w.Fronts) > 0 {
		b.WriteString(truncate(theme.Subtle.Render("  An accountant (crew screen, 4) adds to every front and cuts audit risk. Keep them loyal: they skim the wash."), m.width) + "\n")
	}
	b.WriteString("\n")

	// The routes: one dial each, its books beside it. The columns give
	// way to the width: the route, its dial, what it keeps where and
	// what is on it come first.
	lg := m.set.Logistics
	var routes []content.RouteConfig
	for _, cid := range w.CityOrder {
		for _, r := range lg.Routes(cid) {
			if r.From == cid {
				routes = append(routes, r)
			}
		}
	}
	if len(routes) > 0 {
		lost := 0
		for _, n := range w.Logistics.Lost {
			lost += n
		}
		b.WriteString(truncate(theme.Bold.Render("LOGISTICS")+theme.Subtle.Render(fmt.Sprintf("  %d route(s) · shipped %d units in %d run(s) · seized %d in %d · the road spends what is over %s dirty · r and R on the map", len(routes), w.Stats.Shipped, w.Stats.Shipments, lost, w.Stats.Seizures, cash(lg.Float()))), m.width) + "\n")
		var rows [][]any
		for _, r := range routes {
			rs := w.Route(r.ID)
			target, road := m.targetLine(r.ID), m.roadOn(r.ID)
			if target == "" {
				target = "none"
			}
			if road == "" {
				road = "none"
			}
			lots, fares := w.Logistics.RouteSpend(r.ID, w.Day, 7)
			rows = append(rows, []any{r.Name, r.Mode, styled{dialStyle(rs.Dial), rs.Dial}, target, road, lots, fares, w.Logistics.Lost[r.ID]})
		}
		cols := []col{{"route", kText, 0}, {"mode", kText, 0}, {"dial", kDial, 0}, {"target", kText, 0}, {"on the road", kText, 0}, {"lots/wk", kCash, 0}, {"fares/wk", kCash, 0}, {"lost units", kInt, 0}}
		for _, line := range table(cols, rows, -1, m.width) {
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	rows := m.frontRows()
	b.WriteString(theme.Bold.Render("ON OFFER") + theme.Subtle.Render("  b to buy") + "\n")
	if len(rows) == 0 {
		b.WriteString(theme.Subtle.Render("  You own every front there is.") + "\n")
	} else {
		for _, line := range table(offerCols, m.offerRows(rows), -1, m.width) {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}
