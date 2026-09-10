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
	screenCrew
	screenMap
	screenUpgrades
	screenLedger
	screenCount
)

var (
	screenNames = []string{"Dashboard", "Market", "Journal", "Crew", "Map", "Upgrades", "Ledger"}
	screenShort = []string{"Dash", "Market", "News", "Crew", "Map", "Upgr", "Ledger"} // when the title bar is tight
)

type mode int

const (
	modeStart mode = iota // continue or new run
	modePlay
	modeReport
	modeBuy
	modeSell
	modeOver
	modeConfirmNew
	modeConfirmFire
	modeConfirmEnd
	modeHelp
	modePost           // pick who to post on the selected corner
	modeStrike         // pick how hard to send the enforcers at the selected corner
	modeConfirmUpgrade // buy the selected upgrade?
	modeFront          // pick a front to buy
	modeConfirmInvestigate
	modeConfirmPayOff
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
	cursor        int    // product cursor shared by market screen and dialogs
	crewCursor    int    // row on the crew screen: roster first, then candidates
	fireID        int    // member awaiting the fire confirmation
	mapCursor     int    // corner selected on the map
	postRole      string // runner or enforcer, while the post picker is open
	postCursor    int
	strikeCursor  int    // row in the strike picker
	upgradeCursor int    // node selected on the upgrades screen
	upgradeID     string // node awaiting the buy confirmation
	frontCursor   int    // offer selected in the buy-a-front picker
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
	m.crewCursor = 0
	m.upgradeCursor = 0
	m.mapCursor = m.yourCorner()
	m.flash = nil
	m.status = fmt.Sprintf("New run. %s, %s in your pocket. Seed %d.", m.w.City, money(m.w.Player.DirtyCash), m.w.Seed)
	_ = game.Save(m.w)
	m.refreshJournal()
}

func (m *Model) continueRun() error {
	w, err := game.Load(m.set.Migrations()...)
	if err != nil {
		return err
	}
	m.w = w
	m.mode = modePlay
	if w.Over != nil {
		m.mode = modeOver
	}
	m.mapCursor = m.yourCorner()
	m.status = fmt.Sprintf("Continued day %d.", w.Day)
	m.refreshJournal()
	return nil
}

// yourCorner is the map index of the corner you stand on, or 0.
func (m *Model) yourCorner() int {
	for i, c := range m.w.Territory.Corners {
		if c.Runner == game.You {
			return i
		}
	}
	return 0
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
	case modeConfirmFire:
		switch key {
		case "y", "Y":
			m.confirmFire()
		default:
			m.mode = modePlay
		}
		return m, nil
	case modeConfirmEnd:
		switch key {
		case "y", "Y", "enter":
			m.endDay()
		default:
			m.mode = modePlay
		}
		return m, nil
	case modeConfirmUpgrade:
		switch key {
		case "y", "Y":
			m.confirmUpgrade()
		default:
			m.mode = modePlay
		}
		return m, nil
	case modeConfirmInvestigate:
		switch key {
		case "y", "Y":
			m.confirmInvestigate()
		default:
			m.mode = modePlay
		}
		return m, nil
	case modeConfirmPayOff:
		switch key {
		case "y", "Y":
			m.confirmPayOff()
		default:
			m.mode = modePlay
		}
		return m, nil
	case modeHelp:
		m.mode = modePlay
		return m, nil
	case modePost:
		switch key {
		case "esc", "q":
			m.mode = modePlay
		case "up", "k":
			if m.postCursor > 0 {
				m.postCursor--
			}
		case "down", "j":
			if m.postCursor < len(m.postRows(m.postRole))-1 {
				m.postCursor++
			}
		case "enter":
			m.confirmPost()
		default:
			if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
				if i := int(key[0] - '1'); i < len(m.postRows(m.postRole)) {
					m.postCursor = i
					m.confirmPost()
				}
			}
		}
		return m, nil
	case modeStrike:
		switch key {
		case "esc", "q":
			m.mode = modePlay
		case "up", "k":
			if m.strikeCursor > 0 {
				m.strikeCursor--
			}
		case "down", "j":
			if m.strikeCursor < len(m.strikeRows())-1 {
				m.strikeCursor++
			}
		case "enter":
			m.confirmStrike()
		default:
			if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
				if i := int(key[0] - '1'); i < len(m.strikeRows()) {
					m.strikeCursor = i
					m.confirmStrike()
				}
			}
		}
		return m, nil
	case modeFront:
		switch key {
		case "esc", "q":
			m.mode = modePlay
		case "up", "k":
			if m.frontCursor > 0 {
				m.frontCursor--
			}
		case "down", "j":
			if m.frontCursor < len(m.frontRows())-1 {
				m.frontCursor++
			}
		case "enter":
			m.confirmFront()
		default:
			if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
				if i := int(key[0] - '1'); i < len(m.frontRows()) {
					m.frontCursor = i
					m.confirmFront()
				}
			}
		}
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
	case "?":
		m.mode = modeHelp
	case "1":
		m.screen = screenDashboard
	case "2":
		m.screen = screenMarket
	case "3":
		m.screen = screenJournal
		m.refreshJournal()
	case "4":
		m.screen = screenCrew
	case "5":
		m.screen = screenMap
	case "6":
		m.screen = screenUpgrades
	case "7":
		m.screen = screenLedger
	case "tab", "right":
		m.screen = (m.screen + 1) % screenCount
	case "shift+tab", "left":
		m.screen = (m.screen + screenCount - 1) % screenCount
	case "n":
		m.endDay()
	case "enter":
		if m.screen == screenUpgrades {
			m.askUpgrade()
		} else {
			m.mode = modeConfirmEnd
		}
	case "u":
		if m.screen == screenUpgrades {
			m.askUpgrade()
		} else {
			m.status = "Upgrades are bought on the tree (6)."
		}
	case "r":
		if m.w.Report != nil {
			m.mode = modeReport
		}
	case "b":
		if m.screen == screenLedger {
			m.askFront()
		} else {
			m.openDialog(modeBuy)
		}
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
	case "h":
		if m.screen == screenCrew {
			m.hireSelected()
		} else {
			m.status = "Hiring happens on the crew screen (4)."
		}
	case "f":
		if m.screen == screenCrew {
			m.askFire()
		}
	case "p":
		m.cyclePay()
	case "d":
		m.cycleLaunder()
	case "i":
		if m.screen == screenCrew {
			m.askInvestigate()
		} else {
			m.status = "Questions are asked on the crew screen (4)."
		}
	case "$":
		if m.screen == screenCrew {
			m.askPayOff()
		} else {
			m.status = "People are paid off on the crew screen (4)."
		}
	case "c":
		if m.screen == screenMap {
			m.askPost("runner")
		} else {
			m.status = "Corners are claimed on the map (5)."
		}
	case "e":
		if m.screen == screenMap {
			m.askPost("enforcer")
		}
	case "a":
		if m.screen == screenMap {
			m.abandonSelected()
		}
	case "w":
		if m.screen == screenMap {
			m.askStrike()
		} else {
			m.status = "Enforcers are sent from the map (5)."
		}
	case "up", "k":
		switch {
		case m.screen == screenJournal:
			m.journal.ScrollUp(1)
		case m.screen == screenCrew:
			if m.crewCursor > 0 {
				m.crewCursor--
			}
		case m.screen == screenMap:
			if m.mapCursor > 0 {
				m.mapCursor--
			}
		case m.screen == screenUpgrades:
			if m.upgradeCursor > 0 {
				m.upgradeCursor--
			}
		case m.cursor > 0:
			m.cursor--
		}
	case "down", "j":
		switch {
		case m.screen == screenJournal:
			m.journal.ScrollDown(1)
		case m.screen == screenCrew:
			if m.crewCursor < len(m.crewRows())-1 {
				m.crewCursor++
			}
		case m.screen == screenMap:
			if m.mapCursor < len(m.w.Territory.Corners)-1 {
				m.mapCursor++
			}
		case m.screen == screenUpgrades:
			if m.upgradeCursor < len(m.upgradeRows())-1 {
				m.upgradeCursor++
			}
		case m.cursor < len(m.w.Products)-1:
			m.cursor++
		}
	case "pgup":
		m.journal.HalfPageUp()
	case "pgdown":
		m.journal.HalfPageDown()
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
	case modeConfirmFire:
		name := "them"
		if c := m.w.Crew.Member(m.fireID); c != nil {
			name = c.Name
		}
		body = m.modal("FIRE "+strings.ToUpper(name)+"?", "No severance in this business. The rest of the crew\nwill take it personally.\n\n"+theme.Key.Render("y")+" yes   "+theme.Key.Render("any other key")+" no")
	case modeConfirmEnd:
		what := "No sales queued."
		switch {
		case m.w.LieLow:
			what = "Lying low today."
		case len(m.w.Orders) > 0:
			what = fmt.Sprintf("%d order(s) queued.", len(m.w.Orders))
		}
		body = m.modal("END THE DAY?", what+" The sims step and the run autosaves.\n\n"+theme.Key.Render("enter")+" / "+theme.Key.Render("y")+" end the day   "+theme.Key.Render("any other key")+" back")
	case modeHelp:
		body = m.viewHelp()
	case modePost:
		body = m.viewPost()
	case modeStrike:
		body = m.viewStrike()
	case modeConfirmUpgrade:
		body = m.upgradeConfirm()
	case modeFront:
		body = m.viewFront()
	case modeConfirmInvestigate:
		body = m.investigateConfirm()
	case modeConfirmPayOff:
		body = m.payOffConfirm()
	default:
		switch m.screen {
		case screenMarket:
			body = m.viewMarket()
		case screenJournal:
			body = m.viewJournal()
		case screenCrew:
			body = m.viewCrew()
		case screenMap:
			body = m.viewMap()
		case screenUpgrades:
			body = m.viewUpgrades()
		case screenLedger:
			body = m.viewLedger()
		default:
			body = m.viewDashboard()
		}
	}
	body = lipgloss.NewStyle().Width(m.width).Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).Render(body)
	return lines(m.viewTitle(), body, m.viewTicker(), m.viewFooter())
}

func (m *Model) viewTitle() string {
	w := m.w
	tabsFor := func(short int) string {
		var tabs []string
		for i, n := range screenNames {
			var label string
			switch short {
			case 0:
				label = fmt.Sprintf("%d %s", i+1, n)
			case 1:
				label = fmt.Sprintf("%d %s", i+1, screenShort[i])
			default:
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
		s := fmt.Sprintf("Day %d  ", w.Day) + theme.Gold.Render("dirty "+cash(w.Player.DirtyCash)) + "  "
		if clean {
			s += theme.Subtle.Render("clean "+cash(w.Player.CleanCash)) + "  "
		}
		return s + heatStyle(w.Heat.Value).Render(fmt.Sprintf("heat %.0f", w.Heat.Value)) + " "
	}
	// Try the roomy layout first, then progressively shorter ones.
	for _, try := range []struct {
		short int
		clean bool
	}{{0, true}, {0, false}, {1, false}, {2, false}} {
		left, right := tabsFor(try.short), rightFor(try.clean)
		if gap := m.width - lipgloss.Width(left) - lipgloss.Width(right); gap >= 1 {
			return left + strings.Repeat(" ", gap) + right
		}
	}
	return fit(tabsFor(2)+" "+rightFor(false), m.width)
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
	case modeConfirmEnd:
		keys = k("enter", "end day") + k("esc", "back")
	case modeHelp, modeConfirmNew, modeConfirmFire:
		keys = k("any key", "close")
	case modeConfirmUpgrade:
		keys = k("y", "buy") + k("any other key", "back")
	case modeConfirmInvestigate:
		keys = k("y", "ask") + k("any other key", "back")
	case modeConfirmPayOff:
		keys = k("y", "pay") + k("any other key", "back")
	case modePost:
		keys = k("↑↓", "pick") + k("enter", "post") + k("esc", "back")
	case modeStrike:
		keys = k("↑↓", "pick") + k("enter", "send") + k("esc", "back")
	case modeFront:
		keys = k("↑↓", "pick") + k("enter", "buy") + k("esc", "back")
	default:
		switch m.screen {
		case screenCrew:
			keys = k("n", "end day") + k("↑↓", "pick") + k("h", "hire") + k("f", "fire") + k("i", "ask") + k("$", "pay off") + k("p", "pay") + k("?", "help")
		case screenMap:
			keys = k("n", "end day") + k("↑↓", "pick") + k("c", "runner") + k("e", "enforcer") + k("a", "abandon") + k("w", "war") + k("?", "help") + k("q", "quit")
		case screenUpgrades:
			keys = k("n", "end day") + k("↑↓", "pick") + k("enter", "buy") + k("?", "help") + k("q", "quit")
		case screenLedger:
			keys = k("n", "end day") + k("b", "buy a front") + k("d", "launder dial") + k("l", "lie low") + k("?", "help") + k("q", "quit")
		default:
			keys = k("n", "end day") + k("b", "buy") + k("s", "sell") + k("l", "lie low") + k("x", "cancel order") + k("r", "report") + k("?", "help") + k("q", "quit")
		}
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
		// Load errors can be long; wrap inside the box instead of past it.
		b.WriteString("\n" + theme.Warning.Width(max(20, m.width-12)).Render(m.status))
	}
	box := theme.Modal.Render(theme.Title.Render("KINGPIN") + "\n" + theme.Subtle.Render("a drug empire, one day at a time") + "\n\n" + b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) viewHelp() string {
	rows := [][2]string{
		{"1-7 / ← →", "switch screen (tab / shift+tab too)"},
		{"n", "end the day (sims step, autosave)"},
		{"enter", "end the day, after a confirmation"},
		{"b", "buy from the supplier (on the ledger: buy a front)"},
		{"s", "queue a street sale with the dial"},
		{"x", "cancel the order on the selected product"},
		{"l", "lie low today (no sales, heat fades faster)"},
		{"r", "reopen the morning report"},
		{"h / f", "hire / fire the selected person (crew screen)"},
		{"i / $", "investigate who is talking / pay off the selected person (crew)"},
		{"p", "cycle crew pay: stingy / fair / generous"},
		{"c / e / a", "post a runner / an enforcer / abandon the corner (map)"},
		{"w", "send the enforcers at a rival corner: warn / push / hit (map)"},
		{"u / enter", "buy the selected upgrade, after a confirmation (upgrades)"},
		{"d", "cycle the launder dial: careful / normal / greedy"},
		{"↑ ↓ / j k", "move the cursor / scroll journal"},
		{"ctrl+s", "save now"},
		{"N", "abandon run and start over"},
		{"q", "save and quit"},
	}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("%s  %s\n", theme.Key.Render(fit(r[0], 14)), r[1]))
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
	b.WriteString(fmt.Sprintf("Peak cash       %s\n", cash(w.Stats.PeakCash)))
	b.WriteString(fmt.Sprintf("Total revenue   %s\n", cash(w.Stats.TotalRevenue)))
	b.WriteString(fmt.Sprintf("Units moved     %d\n", w.Stats.UnitsSold))
	b.WriteString(fmt.Sprintf("Stings / raids  %d / %d\n", w.Stats.Stings, w.Stats.Raids))
	b.WriteString(fmt.Sprintf("Wages / skimmed %s / %s\n", cash(w.Stats.Wages), cash(w.Stats.Skimmed)))
	b.WriteString(fmt.Sprintf("Corners / robbed %d / %s\n", w.Held(), cash(w.Stats.Robbed)))
	if w.Rival.Arrived > 0 {
		b.WriteString(fmt.Sprintf("Won / lost to %s %d / %d\n", truncate(w.Rival.Leader, 12), w.Stats.CornersWon, w.Stats.CornersLost))
	}
	b.WriteString(fmt.Sprintf("Washed / seized %s / %s\n", cash(w.Stats.Laundered), cash(w.Stats.Seized)))
	b.WriteString(fmt.Sprintf("Clean cash      %s\n", cash(w.Player.CleanCash)))
	if w.Stats.Informants+w.Stats.Defections > 0 {
		b.WriteString(fmt.Sprintf("Snitches / defectors %d / %d\n", w.Stats.Informants, w.Stats.Defections))
	}
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
	section("CREW", r.Crew, lipgloss.NewStyle().Foreground(theme.Crew))
	section("TERRITORY", r.Territory, lipgloss.NewStyle().Foreground(theme.Rivals))
	section("MONEY", append(r.Money, fmt.Sprintf("Cash %s -> %s", cash(r.CashBefore), cash(r.CashAfter))), theme.Gold)
	section("UPGRADES", r.Upgrades, theme.Gold)
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
