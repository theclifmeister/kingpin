package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// stanceWord is a DA's ticket as the screen prints it.
func stanceWord(stance string) string {
	if stance == "law_and_order" {
		return "law-and-order"
	}
	return stance
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
	var cities []string
	for i, id := range w.CityOrder {
		name := " " + w.CityName(id) + " "
		if i == m.fnd.city {
			cities = append(cities, theme.Selected.Render(name))
		} else {
			cities = append(cities, theme.Subtle.Render(name))
		}
	}
	body := []string{
		"City      " + strings.Join(cities, " "),
		fmt.Sprintf("Now       pressure %s  goodwill %s", theme.Bad.Render(fmt.Sprintf("%.0f", c.Pressure)), theme.Good.Render(fmt.Sprintf("%.0f", c.Goodwill))),
		fmt.Sprintf("Amount    %s   %s", m.fnd.amt.View(), theme.Subtle.Render("clean "+cash(w.Player.CleanCash))),
	}
	if amt, err := parseQtyInput(m.fnd.amt.Value(), m.maxFund(c)); err == nil {
		g := m.set.Law.Goodwill(amt)
		style := theme.Gold
		if amt > w.Player.CleanCash {
			style = theme.Bad
		}
		body = append(body, fmt.Sprintf("Buys      %s goodwill for %s   %s", theme.Good.Render(fmt.Sprintf("+%.0f", g)), style.Render(money(amt)), theme.Subtle.Render(fmt.Sprintf("(%s a point, 100 at most)", money(tun.GoodwillCash)))))
	}
	body = append(body, "",
		theme.Subtle.Render(fmt.Sprintf("Full goodwill takes %.1f pressure off the city a day; it fades %.0f%% a day.", tun.GoodwillCut, tun.GoodwillDecay*100)),
		theme.Subtle.Render("Community centres, campaigns, benevolent funds: clean money only."))
	if m.fnd.err != "" {
		body = append(body, "", theme.Bad.Render(m.fnd.err))
	}
	return m.modal("FUND · "+c.Name, body, m.modalFooter())
}

// lawReportStyle is the colour the report's LAW section is printed in.
var lawReportStyle = lipgloss.NewStyle().Foreground(theme.Heat)
