package ui

import (
	"fmt"
	"strings"

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

// frontStatus is one word on a front's state for the ledger.
func (m *Model) frontStatus(f game.Front) string {
	switch {
	case f.Frozen(m.w.Day+1) && f.Audited > 0 && f.FrozenUntil == f.Audited+m.set.Laundering.Tuning().AuditFreezeDays:
		return theme.Bad.Render(fmt.Sprintf("AUDIT, back in %dd", f.FrozenUntil-m.w.Day))
	case f.Frozen(m.w.Day + 1):
		return theme.Warning.Render(fmt.Sprintf("shut, back in %dd", f.FrozenUntil-m.w.Day))
	case f.Bought == m.w.Day:
		return theme.Subtle.Render("opens tomorrow")
	default:
		return theme.Good.Render("open")
	}
}

func (m *Model) viewFront() string {
	rows := m.frontRows()
	if len(rows) == 0 {
		return m.modal("BUY A FRONT", "Nothing for sale.")
	}
	m.frontCursor = max(0, min(m.frontCursor, len(rows)-1))
	var b strings.Builder
	b.WriteString(theme.Subtle.Render(fmt.Sprintf("  %-18s %10s %10s %10s  %s", "", "cost", "washes/day", "upkeep/day", "audit/day")) + "\n")
	for i, o := range rows {
		line := fmt.Sprintf("%-18s %10s %10s %10s  ", fit(o.Name, 18), money(o.Cost), money(o.Throughput), money(o.Upkeep))
		var note string
		switch {
		case o.Locked(m.w):
			note = theme.Subtle.Render(fmt.Sprintf("locked (%s)", cash(o.UnlockCash)))
		case o.Cost > m.w.Player.DirtyCash:
			note = theme.Bad.Render(fmt.Sprintf("%.1f%%  can't afford", o.AuditRisk*100))
		default:
			note = fmt.Sprintf("%.1f%%", o.AuditRisk*100)
		}
		if i == m.frontCursor {
			b.WriteString(theme.Gold.Render("▸ ") + theme.Selected.Render(line) + note + "\n")
		} else {
			b.WriteString("  " + theme.Subtle.Render(line) + note + "\n")
		}
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
		b.WriteString(theme.Subtle.Render(fmt.Sprintf("  %-18s %9s %9s %10s %6s  %s", "", "per day", "today", "lifetime", "audit", "status")) + "\n")
		for _, f := range w.Fronts {
			b.WriteString(truncate(fmt.Sprintf("  %-18s %9s %9s %10s %5.1f%%  %s",
				fit(f.Name, 18), money(l.Throughput(w, f)), money(f.WashedToday), money(f.Washed), l.AuditRisk(w, f)*100, m.frontStatus(f)), m.width) + "\n")
		}
	}
	b.WriteString("\n")

	rows := m.frontRows()
	b.WriteString(theme.Bold.Render("ON OFFER") + theme.Subtle.Render("  b to buy") + "\n")
	if len(rows) == 0 {
		b.WriteString(theme.Subtle.Render("  You own every front there is.") + "\n")
	} else {
		b.WriteString(theme.Subtle.Render(fmt.Sprintf("  %-18s %10s %10s %10s  %s", "", "cost", "washes/day", "upkeep/day", "audit/day")) + "\n")
		for _, o := range rows {
			line := fmt.Sprintf("  %-18s %10s %10s %10s  ", fit(o.Name, 18), money(o.Cost), money(o.Throughput), money(o.Upkeep))
			switch {
			case o.Locked(m.w):
				line = theme.Subtle.Render(line + fmt.Sprintf("locked (%s)", cash(o.UnlockCash)))
			case o.Cost > w.Player.DirtyCash:
				line += theme.Bad.Render(fmt.Sprintf("%.1f%%  need %s dirty", o.AuditRisk*100, money(o.Cost)))
			default:
				line += fmt.Sprintf("%.1f%%", o.AuditRisk*100)
			}
			b.WriteString(truncate(line, m.width) + "\n")
		}
	}
	if n := w.Crew.Role("accountant"); n > 0 {
		b.WriteString("\n" + truncate(theme.Subtle.Render(fmt.Sprintf("  %d accountant(s) on the payroll: more through every front, fewer audits.", n)), m.width) + "\n")
	} else if len(w.Fronts) > 0 {
		b.WriteString("\n" + truncate(theme.Subtle.Render("  An accountant (crew screen, 4) adds to every front and cuts audit risk. Keep them loyal: they skim the wash."), m.width) + "\n")
	}
	return b.String()
}
