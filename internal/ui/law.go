package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// stanceWord is a DA's ticket as the screen prints it.
func stanceWord(stance string) string {
	if stance == "law_and_order" {
		return "law-and-order"
	}
	return stance
}

// lawLines is the dashboard's LAW panel: the chief and what they are
// like once you have seen them work, the DA and their ticket with the
// days to the election, and the pressure where you are with what you
// have bought the city.
func (m *Model) lawLines(width int) string {
	w := m.w
	l := w.Law
	here := w.Here()
	var b strings.Builder
	chief := fmt.Sprintf("Chief %s", l.Chief.Name)
	if l.Chief.Observed {
		chief += theme.Subtle.Render(" · " + l.Chief.Personality)
	} else {
		chief += theme.Subtle.Render(" · new in the job")
	}
	if end := m.set.Law.ChiefTermEnds(w); end > 0 {
		chief += theme.Subtle.Render(fmt.Sprintf(" · %dd left", max(0, end-w.Day)))
	}
	b.WriteString(chief + "\n")
	da := fmt.Sprintf("DA %s", l.DA.Name) + theme.Subtle.Render(" · "+stanceWord(l.DA.Stance))
	if next := m.set.Law.NextElection(w); next > 0 {
		da += theme.Subtle.Render(fmt.Sprintf(" · election in %dd", max(0, next-w.Day)))
	}
	b.WriteString(da + "\n")
	// The pressure bar where you are, in heat's red; goodwill once you
	// have bought some; the other city's number when there is room.
	barW := max(6, min(16, width-28))
	line := theme.Bad.Render("pressure " + sparkline.Bar(here.Pressure/100, barW, nil) + fmt.Sprintf(" %.0f", here.Pressure))
	if here.Goodwill > 0 {
		line += theme.Good.Render(fmt.Sprintf(" · goodwill %.0f", here.Goodwill))
	}
	if width >= 60 {
		for _, cid := range w.CityOrder {
			if c := w.Cities[cid]; c != here {
				line += theme.Subtle.Render(fmt.Sprintf(" · %s %.0f", c.Name, c.Pressure))
			}
		}
	}
	b.WriteString(line + "\n")
	return b.String()
}

// fundDialog is the state of the fund-a-city modal: which city and how
// much clean cash.
type fundDialog struct {
	city int // index into CityOrder
	amt  textinput.Model
	err  string
}

// askFund opens the fund dialog on the city you are in.
func (m *Model) askFund() {
	if m.w.Over != nil {
		return
	}
	if m.w.Player.CleanCash <= 0 {
		m.status = "Goodwill is bought with clean cash, and you have none. A front washes it."
		return
	}
	ti := textinput.New()
	ti.Placeholder = "blank = up to 100"
	ti.CharLimit = 9
	ti.Width = 24
	ti.Prompt = "> "
	ti.Focus()
	m.fnd = fundDialog{amt: ti}
	for i, id := range m.w.CityOrder {
		if id == m.w.Player.Location {
			m.fnd.city = i
		}
	}
	m.mode = modeFund
}

// fundCity is the city the fund dialog is turned to.
func (m *Model) fundCity() *game.City {
	if len(m.w.CityOrder) == 0 {
		return m.w.Here()
	}
	m.fnd.city = max(0, min(m.fnd.city, len(m.w.CityOrder)-1))
	return m.w.Cities[m.w.CityOrder[m.fnd.city]]
}

// maxFund is what a blank amount means: enough clean cash to take the
// city's goodwill to 100, or all of it if that is less.
func (m *Model) maxFund(c *game.City) int {
	need := int((100 - c.Goodwill) * float64(m.set.Law.Tuning().GoodwillCash))
	return max(0, min(need, m.w.Player.CleanCash))
}

func (m *Model) keyFund(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.fnd
	d.err = ""
	switch key {
	case "esc":
		m.mode = modePlay
		return m, nil
	case "left", "[", "right", "]":
		if n := len(m.w.CityOrder); n > 1 {
			if key == "left" || key == "[" {
				d.city = (d.city + n - 1) % n
			} else {
				d.city = (d.city + 1) % n
			}
		}
		return m, nil
	case "enter":
		return m.confirmFund()
	}
	var cmd tea.Cmd
	d.amt, cmd = d.amt.Update(k)
	return m, cmd
}

func (m *Model) confirmFund() (tea.Model, tea.Cmd) {
	c := m.fundCity()
	amt, err := parseQtyInput(m.fnd.amt.Value(), m.maxFund(c))
	if err != nil {
		m.fnd.err = err.Error()
		return m, nil
	}
	if err := m.w.Fund(c.ID, amt); err != nil {
		m.fnd.err = err.Error()
		return m, nil
	}
	m.mode = modePlay
	m.status = fmt.Sprintf("Gave %s %s clean. Goodwill +%.0f tonight; it takes the pressure off a little every day.", c.Name, money(amt), m.set.Law.Goodwill(amt))
	return m, nil
}

func (m *Model) viewFund() string {
	w := m.w
	c := m.fundCity()
	tun := m.set.Law.Tuning()
	var b strings.Builder
	var cities []string
	for i, id := range w.CityOrder {
		name := " " + w.CityName(id) + " "
		if i == m.fnd.city {
			cities = append(cities, theme.Selected.Render(name))
		} else {
			cities = append(cities, theme.Subtle.Render(name))
		}
	}
	lineW := max(20, m.width-8) // inside the modal's frame
	b.WriteString("City      " + strings.Join(cities, " ") + theme.Subtle.Render("  ← → to turn") + "\n")
	b.WriteString(fmt.Sprintf("Now       pressure %s  goodwill %s\n", theme.Bad.Render(fmt.Sprintf("%.0f", c.Pressure)), theme.Good.Render(fmt.Sprintf("%.0f", c.Goodwill))))
	b.WriteString(fmt.Sprintf("Amount    %s   %s\n", m.fnd.amt.View(), theme.Subtle.Render("clean "+cash(w.Player.CleanCash))))
	if amt, err := parseQtyInput(m.fnd.amt.Value(), m.maxFund(c)); err == nil {
		g := m.set.Law.Goodwill(amt)
		style := theme.Gold
		if amt > w.Player.CleanCash {
			style = theme.Bad
		}
		b.WriteString(truncate(fmt.Sprintf("Buys      %s goodwill for %s   %s", theme.Good.Render(fmt.Sprintf("+%.0f", g)), style.Render(money(amt)), theme.Subtle.Render(fmt.Sprintf("(%s a point, 100 at most)", money(tun.GoodwillCash)))), lineW) + "\n")
	}
	b.WriteString("\n" + truncate(theme.Subtle.Render(fmt.Sprintf("Full goodwill takes %.1f pressure off the city a day; it fades %.0f%% a day.", tun.GoodwillCut, tun.GoodwillDecay*100)), lineW) + "\n")
	b.WriteString(truncate(theme.Subtle.Render("Community centres, campaigns, benevolent funds: clean money only."), lineW) + "\n")
	if m.fnd.err != "" {
		b.WriteString("\n" + theme.Bad.Render(m.fnd.err) + "\n")
	}
	b.WriteString("\n" + theme.Key.Render("enter") + " give  " + theme.Key.Render("esc") + " back")
	return m.modal("FUND "+strings.ToUpper(c.Name), strings.TrimRight(b.String(), "\n"))
}

// lawReportStyle is the colour the report's LAW section is printed in.
var lawReportStyle = lipgloss.NewStyle().Foreground(theme.Heat)
