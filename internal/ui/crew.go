package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// crewRows is the list the crew cursor moves over: the roster first, then
// the people looking for work.
func (m *Model) crewRows() []game.CrewMember {
	c := m.w.Crew
	rows := append([]game.CrewMember(nil), c.Members...)
	return append(rows, c.Candidates...)
}

// crewSelected returns the member under the cursor and whether they are on
// the payroll (as opposed to a candidate).
func (m *Model) crewSelected() (game.CrewMember, bool, bool) {
	rows := m.crewRows()
	if len(rows) == 0 {
		return game.CrewMember{}, false, false
	}
	m.crewCursor = max(0, min(m.crewCursor, len(rows)-1))
	return rows[m.crewCursor], m.crewCursor < len(m.w.Crew.Members), true
}

func (m *Model) hireSelected() {
	c, onPayroll, ok := m.crewSelected()
	if !ok || onPayroll {
		m.status = "Move the cursor to someone looking for work, then press h."
		return
	}
	got, err := m.w.Hire(c.ID, m.set.Crew.MaxCrew())
	if err != nil {
		m.status = "Can't hire: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("%s hired for %s.", got.Name, money(got.Fee))
	m.crewCursor = len(m.w.Crew.Members) - 1
}

func (m *Model) askFire() {
	c, onPayroll, ok := m.crewSelected()
	if !ok || !onPayroll {
		m.status = "Move the cursor to someone on the payroll, then press f."
		return
	}
	m.fireID = c.ID
	m.mode = modeConfirmFire
}

func (m *Model) confirmFire() {
	m.mode = modePlay
	got, err := m.w.Fire(m.fireID)
	if err != nil {
		m.status = "Can't fire: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("%s is gone. The rest noticed.", got.Name)
}

func (m *Model) cyclePay() {
	p := (m.w.Crew.Pay + 1) % 3
	m.w.SetPay(p)
	m.status = fmt.Sprintf("Pay %s, %s/day. %s", p, money(m.set.Crew.Wages(m.w, p)), payBlurb(p))
}

func payBlurb(p events.Pay) string {
	switch p {
	case events.PayStingy:
		return "Loyalty bleeds."
	case events.PayGenerous:
		return "Loyalty climbs."
	default:
		return "The greedy drift."
	}
}

func loyaltyStyle(l, threshold float64) lipgloss.Style {
	switch {
	case l < threshold:
		return theme.Bad
	case l < threshold+15:
		return theme.Warning
	default:
		return theme.Good
	}
}

func (m *Model) viewCrew() string {
	w := m.w
	tun := m.set.Crew.Tuning()
	pay := w.Crew.Pay
	var b strings.Builder

	title := theme.PanelTitle.Render("CREW") + theme.Subtle.Render(fmt.Sprintf("  %d of %d on the payroll", len(w.Crew.Members), tun.MaxCrew))
	var dial []string
	for p := events.PayStingy; p <= events.PayGenerous; p++ {
		if p == pay {
			dial = append(dial, theme.Selected.Render(" "+p.String()+" "))
		} else {
			dial = append(dial, theme.Subtle.Render(" "+p.String()+" "))
		}
	}
	b.WriteString(title + "   pay " + strings.Join(dial, "") + theme.Gold.Render(fmt.Sprintf("  %s/day", money(m.set.Crew.Wages(w, pay)))) + "\n")
	if w.Crew.LastSkim > 0 && w.Day-w.Crew.LastSkim < tun.SuspectDays {
		b.WriteString(theme.Bad.Bold(true).Render("  ▲ Skimming suspected.") + theme.Bad.Render(fmt.Sprintf(" Money went missing on day %d. Somebody's loyalty is under %.0f.", w.Crew.LastSkim, tun.SkimThreshold)) + "\n")
	} else {
		b.WriteString(theme.Subtle.Render(fmt.Sprintf("  Under %.0f loyalty they skim. At %.0f they walk. Firing costs everyone else %.0f.", tun.SkimThreshold, tun.QuitThreshold, tun.FireLoyalty)) + "\n")
	}
	b.WriteString("\n")

	barW := 10
	marks := []float64{tun.SkimThreshold / 100}
	header := theme.Subtle.Render(fmt.Sprintf("  %-8s %-9s %5s  %-*s  %6s %6s  ", "", "role", "skill", barW+4, "loyalty", "wage", "units"))
	row := func(i int, c game.CrewMember, last string) string {
		cur := "  "
		name := fit(c.Name, 8)
		if i == m.crewCursor {
			cur = theme.Gold.Render("▸ ")
			name = theme.Selected.Render(name)
		}
		ls := loyaltyStyle(c.Loyalty, tun.SkimThreshold)
		units := theme.Subtle.Render(fmt.Sprintf("%6s", "-"))
		if c.Units > 0 {
			units = fmt.Sprintf("%+6d", c.Units)
		}
		return fmt.Sprintf("%s%s %-9s %5d  %s %s  %6s %s  %s",
			cur, name, c.Role, c.Skill,
			ls.Render(sparkline.Bar(c.Loyalty/100, barW, marks)), ls.Render(fmt.Sprintf("%3.0f", c.Loyalty)),
			money(m.set.Crew.WageAt(c, pay)), units, theme.Subtle.Render(last))
	}

	b.WriteString(theme.Bold.Render("ON THE PAYROLL") + "\n")
	if len(w.Crew.Members) == 0 {
		b.WriteString(theme.Subtle.Render("  Nobody. Runners move product you can't carry; pick one below and press h.") + "\n")
	} else {
		b.WriteString(header + theme.Subtle.Render("since") + "\n")
		for i, c := range w.Crew.Members {
			b.WriteString(row(i, c, fmt.Sprintf("day %d", c.Hired)) + "\n")
		}
	}
	b.WriteString("\n")

	next := tun.PoolDays - (w.Day - w.Crew.PoolDay)
	b.WriteString(theme.Bold.Render("LOOKING FOR WORK") + theme.Subtle.Render(fmt.Sprintf("  new faces in %d day(s)", max(1, next))) + "\n")
	if len(w.Crew.Candidates) == 0 {
		b.WriteString(theme.Subtle.Render("  Nobody right now.") + "\n")
	} else {
		b.WriteString(header + theme.Subtle.Render("fee") + "\n")
		for i, c := range w.Crew.Candidates {
			fee := money(c.Fee)
			if c.Fee > w.Player.DirtyCash {
				fee = theme.Bad.Render(fee)
			}
			b.WriteString(row(len(w.Crew.Members)+i, c, "") + fee + "\n")
		}
	}
	b.WriteString("\n")

	// What the crew adds, in the same terms the dashboard uses.
	b.WriteString(theme.Subtle.Render(fmt.Sprintf("  Capacity %d units (%d yours + %d crew) · reach x%.1f of the street",
		w.Capacity(), w.Player.CarryLimit, w.Capacity()-w.Player.CarryLimit, w.Reach())) + "\n")
	if sl := m.set.Heat.Sloppiness(w); sl > 0 {
		per := sl * m.cfg.Heat.Heat.SloppyHeat * 100
		b.WriteString(theme.Warning.Render(fmt.Sprintf("  Sloppy runners (skill under %d) add +%.1f heat per 100 units moved.", m.cfg.Heat.Heat.SloppySkill, per)) + "\n")
	}
	return b.String()
}
