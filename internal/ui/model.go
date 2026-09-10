// Package ui is the Bubble Tea front end. The root Model owns the world and
// the clock, switches between screens, and routes key presses to modal
// dialogs. Simulations never import this package.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

type screen int

const (
	screenDashboard screen = iota
	screenMarket
	screenJournal
	screenCount
)

var screenNames = []string{"Dashboard", "Market", "Journal"}

type mode int

const (
	modeStart mode = iota // continue or new run
	modePlay
	modeReport
	modeBuy
	modeSell
	modeOver
	modeConfirmNew
	modeHelp
)

type tickMsg time.Time

const tickerPeriod = 180 * time.Millisecond

// Model is the root Bubble Tea model.
type Model struct {
	cfg   *content.Config
	bus   *events.Bus
	set   *sim.Set
	clock *game.Clock
	w     *game.World

	width, height int
	screen        screen
	mode          mode
	cursor        int // product cursor shared by market screen and dialogs
	journal       viewport.Model
	dlg           dialog
	startChoice   int
	tick          int
	status        string
	flash         []string // enforcement lines from the last tick, via the bus
	quitting      bool
}

// New wires config, simulations, clock and bus together. If a save exists
// the player is offered Continue / New run; otherwise a run starts.
func New(cfg *content.Config) (*Model, error) {
	set, sims, err := sim.Default(cfg)
	if err != nil {
		return nil, err
	}
	bus := events.NewBus()
	m := &Model{
		cfg:   cfg,
		bus:   bus,
		set:   set,
		clock: game.NewClock(bus, sims...),
	}
	bus.Subscribe(m.onEvent)
	m.journal = viewport.New(80, 20)
	if game.HasSave() {
		m.mode = modeStart
	} else {
		m.newRun()
	}
	return m, nil
}

// World exposes the current world for tests.
func (m *Model) World() *game.World { return m.w }

func (m *Model) onEvent(e events.Event) {
	if ev, ok := e.(events.Enforcement); ok {
		m.flash = append(m.flash, strings.ToUpper(ev.Level))
	}
}

func (m *Model) newRun() {
	m.w = sim.NewWorld(m.cfg, game.NewSeed())
	m.mode = modePlay
	m.screen = screenDashboard
	m.cursor = 0
	m.flash = nil
	m.status = fmt.Sprintf("New run. %s, %s in your pocket. Seed %d.", m.w.City, money(m.w.Player.DirtyCash), m.w.Seed)
	_ = game.Save(m.w)
	m.refreshJournal()
}

func (m *Model) continueRun() error {
	w, err := game.Load()
	if err != nil {
		return err
	}
	m.w = w
	m.mode = modePlay
	if w.Over != nil {
		m.mode = modeOver
	}
	m.status = fmt.Sprintf("Continued day %d.", w.Day)
	m.refreshJournal()
	return nil
}

func (m *Model) endDay() {
	if m.w.Over != nil {
		m.mode = modeOver
		return
	}
	m.flash = nil
	m.clock.EndDay(m.w)
	m.save()
	m.refreshJournal()
	if m.w.Over != nil {
		m.mode = modeOver
		return
	}
	m.mode = modeReport
}

func (m *Model) save() {
	if err := game.Save(m.w); err != nil {
		m.status = "Save failed: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("Day %d saved.", m.w.Day)
}

// Init starts the ticker that scrolls the news crawl.
func (m *Model) Init() tea.Cmd { return tickCmd() }

func tickCmd() tea.Cmd {
	return tea.Tick(tickerPeriod, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Update is the Bubble Tea update loop.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.journal.Width = max(10, m.width-4)
		m.journal.Height = max(3, m.bodyHeight()-2)
		m.refreshJournal()
		return m, nil
	case tickMsg:
		m.tick++
		return m, tickCmd()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	if key == "ctrl+c" {
		return m.quit()
	}
	switch m.mode {
	case modeStart:
		return m.keyStart(key)
	case modeConfirmNew:
		switch key {
		case "y", "Y":
			m.newRun()
		default:
			m.mode = modePlay
		}
		return m, nil
	case modeHelp:
		m.mode = modePlay
		return m, nil
	case modeReport:
		switch key {
		case "enter", "esc", " ", "r", "q":
			m.mode = modePlay
		}
		return m, nil
	case modeOver:
		switch key {
		case "enter", "n":
			_ = game.DeleteSave()
			m.newRun()
		case "q":
			return m.quit()
		}
		return m, nil
	case modeBuy, modeSell:
		return m.keyDialog(k)
	}
	return m.keyPlay(key)
}

func (m *Model) keyStart(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k", "down", "j", "tab":
		m.startChoice = 1 - m.startChoice
	case "c":
		m.startChoice = 0
		return m.pickStart()
	case "n":
		m.startChoice = 1
		return m.pickStart()
	case "enter":
		return m.pickStart()
	case "q":
		return m.quit()
	}
	return m, nil
}

func (m *Model) pickStart() (tea.Model, tea.Cmd) {
	if m.startChoice == 0 {
		if err := m.continueRun(); err != nil {
			m.status = "Could not load save: " + err.Error() + " (press n for a new run)"
			m.startChoice = 1
		}
		return m, nil
	}
	m.newRun()
	return m, nil
}

func (m *Model) keyPlay(key string) (tea.Model, tea.Cmd) {
	m.status = ""
	switch key {
	case "q":
		return m.quit()
	case "ctrl+s":
		m.save()
	case "?", "h":
		m.mode = modeHelp
	case "1":
		m.screen = screenDashboard
	case "2":
		m.screen = screenMarket
	case "3":
		m.screen = screenJournal
		m.refreshJournal()
	case "tab":
		m.screen = (m.screen + 1) % screenCount
	case "shift+tab":
		m.screen = (m.screen + screenCount - 1) % screenCount
	case "n":
		m.endDay()
	case "r":
		if m.w.Report != nil {
			m.mode = modeReport
		}
	case "b":
		m.openDialog(modeBuy)
	case "s":
		m.openDialog(modeSell)
	case "x":
		id := m.w.Products[m.cursor]
		if _, ok := m.w.Orders[id]; ok {
			m.w.CancelSell(id)
			m.status = "Order cancelled."
		}
	case "l":
		m.w.SetLieLow(!m.w.LieLow)
		if m.w.LieLow {
			m.status = "Lying low today: no sales, heat fades faster."
		} else {
			m.status = "Back on the corner."
		}
	case "N":
		m.mode = modeConfirmNew
	case "up", "k":
		if m.screen == screenJournal {
			m.journal.LineUp(1)
		} else if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.screen == screenJournal {
			m.journal.LineDown(1)
		} else if m.cursor < len(m.w.Products)-1 {
			m.cursor++
		}
	case "pgup":
		m.journal.HalfViewUp()
	case "pgdown":
		m.journal.HalfViewDown()
	}
	return m, nil
}

func (m *Model) quit() (tea.Model, tea.Cmd) {
	if m.w != nil {
		_ = game.Save(m.w)
	}
	m.quitting = true
	return m, tea.Quit
}

func (m *Model) bodyHeight() int {
	// title bar + ticker + footer
	return max(5, m.height-3)
}

// View renders the whole screen.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		return "loading…"
	}
	if m.mode == modeStart || m.w == nil {
		return m.viewStart()
	}
	var body string
	switch m.mode {
	case modeReport:
		body = m.viewReport()
	case modeBuy, modeSell:
		body = m.viewDialog()
	case modeOver:
		body = m.viewOver()
	case modeConfirmNew:
		body = m.modal("NEW RUN?", "Abandon the current run and start over?\n\n"+theme.Key.Render("y")+" yes   "+theme.Key.Render("any other key")+" no")
	case modeHelp:
		body = m.viewHelp()
	default:
		switch m.screen {
		case screenMarket:
			body = m.viewMarket()
		case screenJournal:
			body = m.viewJournal()
		default:
			body = m.viewDashboard()
		}
	}
	body = lipgloss.NewStyle().Width(m.width).Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).Render(body)
	return lines(m.viewTitle(), body, m.viewTicker(), m.viewFooter())
}

func (m *Model) viewTitle() string {
	w := m.w
	tabsFor := func(short bool) string {
		var tabs []string
		for i, n := range screenNames {
			label := fmt.Sprintf("%d %s", i+1, n)
			if short {
				label = fmt.Sprintf("%d", i+1)
			}
			if screen(i) == m.screen && m.mode == modePlay {
				tabs = append(tabs, theme.TabOn.Render(label))
			} else {
				tabs = append(tabs, theme.Tab.Render(label))
			}
		}
		return theme.Title.Render(" KINGPIN ") + strings.Join(tabs, "")
	}
	rightFor := func(clean bool) string {
		s := fmt.Sprintf("Day %d  ", w.Day) + theme.Gold.Render("dirty "+money(w.Player.DirtyCash)) + "  "
		if clean {
			s += theme.Subtle.Render("clean "+money(w.Player.CleanCash)) + "  "
		}
		return s + heatStyle(w.Heat.Value).Render(fmt.Sprintf("heat %.0f", w.Heat.Value)) + " "
	}
	// Try the roomy layout first, then progressively shorter ones.
	for _, try := range [][2]bool{{false, true}, {false, false}, {true, false}} {
		left, right := tabsFor(try[0]), rightFor(try[1])
		if gap := m.width - lipgloss.Width(left) - lipgloss.Width(right); gap >= 1 {
			return left + strings.Repeat(" ", gap) + right
		}
	}
	return fit(tabsFor(true)+" "+rightFor(false), m.width)
}

func (m *Model) viewTicker() string {
	items := m.w.Journal
	if len(items) > 12 {
		items = items[len(items)-12:]
	}
	if len(items) == 0 {
		return theme.Subtle.Render(fit(" ◆ No news yet. Make some.", m.width))
	}
	var parts []string
	for _, h := range items {
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.Source(h.Source)).Render(h.Text))
	}
	sep := theme.Subtle.Render("  ◆  ")
	text := strings.Join(parts, sep) + sep
	// Scroll by rotating the plain runes; styling is per item so we rotate
	// the styled string by measuring visible width of a prefix.
	plain := []rune(stripANSI(text))
	if len(plain) == 0 {
		return ""
	}
	off := m.tick % len(plain)
	rot := string(append(append([]rune{}, plain[off:]...), plain[:off]...))
	return lipgloss.NewStyle().Foreground(theme.News).Render(fit(rot, m.width))
}

func (m *Model) viewFooter() string {
	var keys string
	switch m.mode {
	case modeReport:
		keys = k("enter", "close report")
	case modeBuy, modeSell:
		keys = k("↑↓", "pick") + k("enter", "next") + k("esc", "back")
	case modeOver:
		keys = k("enter", "new run") + k("q", "quit")
	case modeHelp, modeConfirmNew:
		keys = k("any key", "close")
	default:
		keys = k("n", "end day") + k("b", "buy") + k("s", "sell") + k("l", "lie low") + k("x", "cancel order") + k("r", "report") + k("?", "help") + k("q", "quit")
	}
	status := theme.Warning.Render(m.status)
	gap := m.width - lipgloss.Width(keys) - lipgloss.Width(status)
	if gap < 1 {
		return fit(keys, m.width)
	}
	return keys + strings.Repeat(" ", gap) + status
}

func k(key, label string) string {
	return " " + theme.Key.Render(key) + " " + theme.Subtle.Render(label) + " "
}

func heatStyle(v float64) lipgloss.Style {
	switch {
	case v >= 75:
		return theme.Bad.Bold(true)
	case v >= 55:
		return theme.Bad
	case v >= 30:
		return theme.Warning
	default:
		return theme.Good
	}
}

// modal centres a bordered box in the body.
func (m *Model) modal(title, content string) string {
	box := theme.Modal.Render(theme.Title.Render(title) + "\n\n" + content)
	return lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) viewStart() string {
	opts := []string{"Continue saved run", "New run"}
	var b strings.Builder
	for i, o := range opts {
		if i == m.startChoice {
			b.WriteString(theme.Selected.Render(" ▸ "+o+" ") + "\n")
		} else {
			b.WriteString("   " + o + "\n")
		}
	}
	b.WriteString("\n" + theme.Subtle.Render("enter select · c continue · n new · q quit"))
	if m.status != "" {
		b.WriteString("\n" + theme.Warning.Render(m.status))
	}
	box := theme.Modal.Render(theme.Title.Render("KINGPIN") + "\n" + theme.Subtle.Render("a drug empire, one day at a time") + "\n\n" + b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) viewHelp() string {
	rows := [][2]string{
		{"1 2 3 / tab", "switch screen"},
		{"n", "end the day (sims step, autosave)"},
		{"b", "buy from the supplier"},
		{"s", "queue a street sale with the dial"},
		{"x", "cancel the order on the selected product"},
		{"l", "lie low today (no sales, heat fades faster)"},
		{"r", "reopen the morning report"},
		{"↑ ↓ / j k", "move the product cursor / scroll journal"},
		{"ctrl+s", "save now"},
		{"N", "abandon run and start over"},
		{"q", "save and quit"},
	}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("%s  %s\n", theme.Key.Render(fit(r[0], 12)), r[1]))
	}
	b.WriteString("\n" + theme.Subtle.Render("Heat is the antagonist. Greed is always available."))
	return m.modal("HELP", b.String())
}

func (m *Model) viewOver() string {
	w := m.w
	e := w.Over
	var b strings.Builder
	b.WriteString(theme.Bad.Bold(true).Render(strings.ToUpper(e.Cause)) + fmt.Sprintf(" on day %d\n\n", e.Day))
	b.WriteString(fmt.Sprintf("Days survived   %d\n", e.Day))
	b.WriteString(fmt.Sprintf("Peak cash       %s\n", money(w.Stats.PeakCash)))
	b.WriteString(fmt.Sprintf("Total revenue   %s\n", money(w.Stats.TotalRevenue)))
	b.WriteString(fmt.Sprintf("Units moved     %d\n", w.Stats.UnitsSold))
	b.WriteString(fmt.Sprintf("Stings / raids  %d / %d\n", w.Stats.Stings, w.Stats.Raids))
	b.WriteString(fmt.Sprintf("Peak heat       %.0f\n", w.Heat.Peak))
	if n := len(w.Journal); n > 0 {
		b.WriteString("\nLast headline:\n  " + theme.Subtle.Render(truncate(w.Journal[n-1].Text, max(20, m.width-20))) + "\n")
	}
	b.WriteString("\n" + theme.Key.Render("enter") + " new run   " + theme.Key.Render("q") + " quit")
	return m.modal("GAME OVER", b.String())
}

func (m *Model) viewReport() string {
	r := m.w.Report
	if r == nil {
		return m.modal("MORNING REPORT", "Nothing happened yet.")
	}
	var b strings.Builder
	section := func(title string, ls []string, style lipgloss.Style) {
		if len(ls) == 0 {
			return
		}
		b.WriteString(style.Bold(true).Render(title) + "\n")
		for _, l := range ls {
			b.WriteString("  " + l + "\n")
		}
		b.WriteString("\n")
	}
	section("PRICES", r.Prices, theme.Good)
	section("SALES", r.Sales, theme.Gold)
	section("HEAT", r.Heat, theme.Bad)
	section("MONEY", append(r.Money, fmt.Sprintf("Cash %s -> %s", money(r.CashBefore), money(r.CashAfter))), theme.Gold)
	section("NEWS", r.News, theme.Subtle)
	content := clampLines(strings.TrimRight(b.String(), "\n"), m.bodyHeight()-6)
	return m.modal(fmt.Sprintf("MORNING REPORT · DAY %d", r.Day), content)
}

func (m *Model) refreshJournal() {
	if m.w == nil {
		return
	}
	var b strings.Builder
	for i := len(m.w.Journal) - 1; i >= 0; i-- {
		h := m.w.Journal[i]
		b.WriteString(theme.Subtle.Render(fmt.Sprintf("day %3d  ", h.Day)))
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Source(h.Source)).Render(h.Text))
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		b.WriteString(theme.Subtle.Render("The paper has nothing to say about you. Yet."))
	}
	m.journal.SetContent(b.String())
	m.journal.GotoTop()
}

func (m *Model) viewJournal() string {
	title := theme.PanelTitle.Render("JOURNAL") + theme.Subtle.Render(fmt.Sprintf("  %d headlines, newest first", len(m.w.Journal)))
	return title + "\n" + m.journal.View()
}

// stripANSI removes escape sequences so the ticker can be rotated by rune.
func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			in = true
		case in && r == 'm':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
