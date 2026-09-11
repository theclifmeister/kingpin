// Package ui is the Bubble Tea front end. The root Model owns the world and
// the clock, switches between screens, and routes key presses to modal
// dialogs. Simulations never import this package.
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
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
	screenRivals
	screenCount
)

var (
	screenNames = []string{"Dashboard", "Market", "Journal", "Crew", "Map", "Upgrades", "Ledger", "Rivals"}
	screenShort = []string{"Dash", "Market", "Journal", "Crew", "Map", "Upgr", "Ledger", "Rivals"} // when the title bar is tight
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
	modeCard          // a dilemma card, before the morning report
	modeTarget        // the route target dialog: product -> units
	modeConfirmTravel // move to the other city?
	modePropose       // pick a deal to put to the rival: kind, then terms
	modeAssign        // pick the city a lieutenant runs
	modeFund          // give a city clean cash for goodwill
	modeDetails       // the details pane as an overlay, where the terminal is too narrow to hold it beside MAIN
	modeCart          // the day's cart: its buys and orders, editable until the day ends
	modeCount
)

// statusKind is what a status message is, and so how the status bar
// colours it: neutral for a confirmation of what you did, a warning for
// a refusal, bad for a danger.
type statusKind int

const (
	statusBody statusKind = iota
	statusWarning
	statusBad
)

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
	paneHidden    bool   // space hid the details pane, where it sits beside MAIN
	city          string // city the market and map screens show; follows you when you travel
	cursor        int    // product cursor shared by market screen and dialogs
	crewCursor    int    // row on the crew screen: roster first, then candidates
	fireID        int    // member awaiting the fire confirmation
	mapCursor     int    // corner selected on the map
	mapTop        int    // the first row of the map's grid drawn, scrolled to keep the cursor in view
	routeCursor   int    // route selected under the map's grid
	onRoutes      bool   // the map's arrows are on the routes, past the bottom row
	buyerCursor   int    // contract selected under the market's product table
	onBuyers      bool   // the market's arrows are on the buyers, past the bottom row
	postRole      string // runner or enforcer, while the post picker is open
	postCursor    int
	strikeCursor  int    // row in the strike picker
	upgradeCursor int    // node selected on the upgrades screen
	upgradeID     string // node awaiting the buy confirmation
	frontCursor   int    // offer selected in the buy-a-front picker
	ledgerCursor  int    // row on the ledger: fronts, then routes, then offers
	ledgerScroll  int    // first line of the ledger MAIN shows, following the cursor
	cardCursor    int    // choice highlighted on the dilemma card
	cardDone      bool   // the card is answered; the outcome is showing
	dealCursor    int    // offer selected on the rivals screen
	proposeStep   int    // 0: pick the kind, 1: pick the terms
	proposeKind   int    // index into proposeKinds while on the terms page
	proposeCursor int
	assignCursor  int    // row in the assign picker
	modalScroll   int    // first body line the open modal shows
	outcome       string // what the last answer did, while it shows
	journalCursor int    // headline selected on the journal screen, newest first
	journalTop    int    // first headline the journal screen shows
	journalSeen   int    // the journal's length when the journal screen was last shown; not saved, a view cursor like city
	dlg           dialog
	tgt           targetDialog
	crt           cartDialog
	fnd           fundDialog
	startChoice   int
	status        string
	statusKind    statusKind // how the status bar colours the message; set where the status is
	flash         []string   // enforcement lines from the last tick, via the bus
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

func (m *Model) newRun() { m.startRun(game.NewSeed()) }

// startRun begins a run from a seed: a new world, the dashboard and the
// cursors at their start. A test that wants a run it can replay (the
// README's captures) passes the seed.
func (m *Model) startRun(seed uint64) {
	m.w = sim.NewWorld(m.cfg, seed)
	m.mode = modePlay
	m.screen = screenDashboard
	m.cursor = 0
	m.crewCursor = 0
	m.upgradeCursor = 0
	m.city = m.w.Player.Location
	m.mapCursor = m.yourCorner()
	m.flash = nil
	m.say(fmt.Sprintf("New run. %s, %s in your pocket. Seed %d.", m.w.Here().Name, money(m.w.Player.DirtyCash), m.w.Seed))
	_ = game.Save(m.w)
	m.refreshJournal()
}

// shown is the city the market and map screens are looking at.
func (m *Model) shown() *game.City {
	if c := m.w.City(m.city); c != nil {
		return c
	}
	m.city = m.w.Player.Location
	return m.w.Here()
}

// cycleCity turns the market and map screens to the next city.
func (m *Model) cycleCity(d int) {
	order := m.w.CityOrder
	if len(order) < 2 {
		return
	}
	i := 0
	for j, id := range order {
		if id == m.city {
			i = j
		}
	}
	m.city = order[(i+d+len(order))%len(order)]
	m.mapCursor = m.yourCorner()
	m.routeCursor, m.onRoutes = 0, false
	m.buyerCursor, m.onBuyers = 0, false
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
	} else if w.Dilemmas.Pending != nil {
		m.showCard() // saved on a card: it is still waiting
	}
	m.city = w.Player.Location
	m.mapCursor = m.yourCorner()
	m.say(fmt.Sprintf("Continued day %d.", w.Day))
	m.refreshJournal()
	m.journalSeen = len(w.Journal) // the news before this morning was yesterday's
	return nil
}

// yourCorner is the map index, in the city shown, of the corner you stand
// on, or 0.
func (m *Model) yourCorner() int {
	for i, c := range m.shown().Corners {
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
	// The danger wins the morning's status bar: the tell that somebody
	// on the payroll is talking, in red, over the save.
	if m.talking() {
		m.alarm("Somebody is talking. Investigate " + screenPointer(screenCrew) + ".")
	}
	m.showCard()
}

func (m *Model) save() {
	if err := game.Save(m.w); err != nil {
		m.alarm("Save failed: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Day %d saved.", m.w.Day))
}

// Init starts nothing: the UI redraws only on a key or a resize.
func (m *Model) Init() tea.Cmd { return nil }

// Update is the Bubble Tea update loop.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil
	case tea.KeyMsg:
		before := m.mode
		r, cmd := m.handleKey(msg)
		if m.mode != before {
			m.modalScroll = 0 // a new modal opens at its top
		}
		return r, cmd
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
	case modeConfirmTravel:
		switch key {
		case "y", "Y":
			m.confirmTravel()
		default:
			m.mode = modePlay
		}
		return m, nil
	case modeTarget:
		return m.keyTarget(k)
	case modeFund:
		return m.keyFund(k)
	case modeCart:
		return m.keyCart(k)
	case modeHelp:
		if !m.scrollModal(key) {
			m.mode = modePlay
		}
		return m, nil
	case modeDetails:
		switch key {
		case "esc", " ", "enter", "q":
			m.mode = modePlay
		default:
			m.scrollModal(key)
		}
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
	case modeAssign:
		switch key {
		case "esc", "q":
			m.mode = modePlay
		case "up", "k":
			if m.assignCursor > 0 {
				m.assignCursor--
			}
		case "down", "j":
			if m.assignCursor < len(m.assignRows())-1 {
				m.assignCursor++
			}
		case "enter":
			m.confirmAssign()
		default:
			if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
				if i := int(key[0] - '1'); i < len(m.assignRows()) {
					m.assignCursor = i
					m.confirmAssign()
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
		default:
			m.scrollModal(key)
		}
		return m, nil
	case modeCard:
		return m.keyCard(key)
	case modePropose:
		switch key {
		case "esc", "q":
			if m.proposeStep == 1 {
				m.proposeStep, m.proposeCursor = 0, m.proposeKind
			} else {
				m.mode = modePlay
			}
		case "up", "k":
			if m.proposeCursor > 0 {
				m.proposeCursor--
			}
		case "down", "j":
			if m.proposeCursor < m.proposeRows()-1 {
				m.proposeCursor++
			}
		case "enter":
			m.pickPropose()
		default:
			if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
				if i := int(key[0] - '1'); i < m.proposeRows() {
					m.proposeCursor = i
					m.pickPropose()
				}
			}
		}
		return m, nil
	case modeOver:
		switch key {
		case "enter", "n":
			_ = game.DeleteSave()
			m.newRun()
		case "q":
			return m.quit()
		default:
			m.scrollModal(key)
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
			m.alarm("Could not load save: " + err.Error())
			m.startChoice = 1
		}
		return m, nil
	}
	m.newRun()
	return m, nil
}

// keyPlay is the main screen's key handler: the key table (keys.go) and
// nothing else. A key no binding on this screen takes is refused with a
// pointer to the screen where one does, if it is a letter that means
// something there; a key the screen lists but that is not live now, the
// arrows and the paging keys are silent where they do nothing.
func (m *Model) keyPlay(key string) (tea.Model, tea.Cmd) {
	m.status, m.statusKind = "", statusBody
	b, found, own := m.lookup(key)
	switch {
	case found:
		b.do(m, key)
		if m.quitting {
			return m, tea.Quit
		}
	case !own && len(key) == 1:
		if p := pointer(key); p != "" {
			m.refuse(p)
		}
	}
	return m, nil
}

// switchScreen shows a screen.
func (m *Model) switchScreen(s screen) {
	m.screen = s
	if s == screenJournal {
		m.refreshJournal()
		m.journalSeen = len(m.w.Journal)
	}
}

// moveCursor moves the screen's cursor: down a list, along the map's
// grid, across the upgrade columns, between the cities on the market;
// the journal scrolls. Arrows never move between tabs (the digits and
// tab do that), and a screen with no horizontal structure ignores dx.
func (m *Model) moveCursor(dx, dy int) {
	switch m.screen {
	case screenJournal:
		m.journalMove(dy)
	case screenCrew:
		if dy < 0 && m.crewCursor > 0 {
			m.crewCursor--
		} else if dy > 0 && m.crewCursor < len(m.crewRows())-1 {
			m.crewCursor++
		}
	case screenMap:
		m.mapMove(dx, dy)
	case screenUpgrades:
		m.upgradeMove(dx, dy)
	case screenRivals:
		if dy < 0 && m.dealCursor > 0 {
			m.dealCursor--
		} else if dy > 0 && m.dealCursor < len(m.w.Offers)-1 {
			m.dealCursor++
		}
	case screenLedger:
		m.ledgerMove(dy)
	case screenMarket:
		switch {
		case dx != 0:
			m.cycleCity(dx)
		case m.onBuyers:
			m.buyersMove(dy)
		case dy < 0 && m.cursor > 0:
			m.cursor--
		case dy > 0 && m.cursor < len(m.w.Products)-1:
			m.cursor++
		case dy > 0 && len(m.buyerRows()) > 0:
			// Off the bottom of the table the arrows reach the buyers,
			// the way the map's reach the routes.
			m.onBuyers, m.buyerCursor = true, 0
		}
	default:
		if dy < 0 && m.cursor > 0 {
			m.cursor--
		} else if dy > 0 && m.cursor < len(m.w.Products)-1 {
			m.cursor++
		}
	}
}

// cancelSelected cancels the order on the selected product in the city
// acted on.
func (m *Model) cancelSelected() {
	if m.cursor >= len(m.w.Products) {
		return
	}
	id := m.w.Products[m.cursor]
	if city := m.actionCity(); m.w.Cities[city] != nil {
		if _, ok := m.w.Order(city, id); ok {
			m.w.CancelSell(city, id)
			m.say("Order cancelled.")
		}
	}
}

// toggleLieLow turns lying low on and off for today.
func (m *Model) toggleLieLow() {
	m.w.SetLieLow(!m.w.LieLow)
	if m.w.LieLow {
		m.say("Lying low today: no sales, heat fades faster.")
	} else {
		m.say("Back on the corner.")
	}
}

// toggleDetails is space: beside MAIN the details pane hides and shows;
// where the terminal is too narrow for that it opens as an overlay.
func (m *Model) toggleDetails() {
	if m.width >= paneMinWidth {
		m.paneHidden = !m.paneHidden
		m.resize()
	} else {
		m.mode = modeDetails
	}
}

func (m *Model) quit() (tea.Model, tea.Cmd) {
	if m.w != nil {
		_ = game.Save(m.w)
	}
	m.quitting = true
	return m, tea.Quit
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
		body = m.modal("NEW RUN?", []string{"Abandon the current run and start over?"}, m.modalFooter())
	case modeConfirmFire:
		name := "them"
		if c := m.w.Crew.Member(m.fireID); c != nil {
			name = c.Name
		}
		body = m.modal("FIRE "+name+"?", []string{"No severance in this business. The rest of the crew", "will take it personally."}, m.modalFooter())
	case modeConfirmEnd:
		// The cart in a sentence, then what the night does.
		body = m.modal("END THE DAY?", []string{m.endDayLine(), "The sims step and the run autosaves."}, m.modalFooter())
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
	case modeAssign:
		body = m.viewAssign()
	case modeConfirmInvestigate:
		body = m.investigateConfirm()
	case modeConfirmPayOff:
		body = m.payOffConfirm()
	case modeConfirmTravel:
		body = m.travelConfirm()
	case modeTarget:
		body = m.viewTarget()
	case modeFund:
		body = m.viewFund()
	case modeCart:
		body = m.viewCart()
	case modeCard:
		body = m.viewCard()
	case modePropose:
		body = m.viewPropose()
	case modeDetails:
		body = m.overlay(m.details(), m.legendKeys(), m.accent())
	default:
		return m.frame(m.viewScreen(), m.details(), m.legendKeys(), m.accent())
	}
	body = theme.Plain.Width(m.width).Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).Render(body)
	return lines(m.viewTitle(), body, m.viewFooter())
}

// viewScreen is the MAIN of the screen shown.
func (m *Model) viewScreen() string {
	switch m.screen {
	case screenMarket:
		return m.viewMarket()
	case screenJournal:
		return m.viewJournal()
	case screenCrew:
		return m.viewCrew()
	case screenMap:
		return m.viewMap()
	case screenUpgrades:
		return m.viewUpgrades()
	case screenLedger:
		return m.viewLedger()
	case screenRivals:
		return m.viewRivals()
	default:
		return m.viewDashboard()
	}
}

// details is the pane's content for the screen shown: its sections, the
// selection first. The keys it accepts are the key table's (keysFor).
func (m *Model) details() []section {
	switch m.screen {
	case screenMarket:
		return m.marketDetails()
	case screenJournal:
		return m.journalDetails()
	case screenCrew:
		return m.crewDetails()
	case screenMap:
		return m.mapDetails()
	case screenUpgrades:
		return m.upgradesDetails()
	case screenLedger:
		return m.ledgerDetails()
	case screenRivals:
		return m.rivalsDetails()
	default:
		return m.dashboardDetails()
	}
}

// accent is the colour the screen shown draws its pane and titles in:
// one per sim.
func (m *Model) accent() lipgloss.Color {
	switch m.screen {
	case screenMarket:
		return theme.Market
	case screenJournal:
		return theme.News
	case screenCrew:
		return theme.Crew
	case screenMap, screenRivals:
		return theme.Rivals
	default:
		return theme.Money
	}
}

// say sets the status to a confirmation of what you did, in the body
// colour; refuse to a refusal, in the warning colour, ended with a full
// stop where the site left it off (`Can't hire: the crew is as big as
// you can manage.`); alarm to a danger, in red. Every site sets the
// kind through one of the three (#88, TestStatusKinds).
func (m *Model) say(s string) {
	m.status, m.statusKind = s, statusBody
}

func (m *Model) refuse(s string) {
	m.status, m.statusKind = sentence(s), statusWarning
}

func (m *Model) alarm(s string) {
	m.status, m.statusKind = sentence(s), statusBad
}

// sentence ends s with a full stop where it has no end punctuation, so
// a refusal built on a game error (`Can't sell: only 3 Weed in
// Eastside`) reads as one; a dialog error is capitalized too
// (dialogError).
func sentence(s string) string {
	if s == "" || strings.ContainsRune(".!?", rune(s[len(s)-1])) {
		return s
	}
	return s + "."
}

func (m *Model) viewTitle() string {
	w := m.w
	// The Journal tab carries the count of headlines you have not read
	// (`Journal 3`, the count in the news accent) wherever its name
	// fits; the digits-only bar drops it with the name.
	unread := m.journalUnread()
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
			if screen(i) == screenJournal && short < 2 && unread > 0 {
				label += " " + theme.NewsText.Render(fmt.Sprintf("%d", unread))
			}
			if screen(i) == m.screen && m.mode == modePlay {
				tabs = append(tabs, theme.TabOn.Render(label))
			} else {
				tabs = append(tabs, theme.Tab.Render(label))
			}
		}
		return theme.Title.Render(" KINGPIN ") + strings.Join(tabs, "")
	}
	// The right side is city · Day N · dirty X · clean Y · heat Z. As
	// width runs out it loses the clean cash, then the city (the epic's
	// mockups: the bar is what says where you stand; clean cash is on
	// the CASH panel and the ledger); the day, the dirty cash and the
	// heat never move.
	here := w.Here()
	sep := theme.Subtle.Render(" · ")
	rightFor := func(city, clean bool) string {
		var parts []string
		if city {
			parts = append(parts, theme.Subtle.Render(here.Name))
		}
		parts = append(parts, fmt.Sprintf("Day %d", w.Day), theme.Gold.Render("dirty "+cash(w.Player.DirtyCash)))
		if clean {
			parts = append(parts, theme.Subtle.Render("clean "+cash(w.Player.CleanCash)))
		}
		parts = append(parts, heatStyle(here.Heat).Render(fmt.Sprintf("heat %.0f", here.Heat)))
		return strings.Join(parts, sep) + " "
	}
	// Try the roomy layout first, then progressively shorter ones: the
	// tabs shorten before the right side loses anything.
	for _, try := range []struct {
		short       int
		city, clean bool
	}{{0, true, true}, {1, true, true}, {1, true, false}, {1, false, false}, {2, false, false}} {
		left, right := tabsFor(try.short), rightFor(try.city, try.clean)
		if gap := m.width - lipgloss.Width(left) - lipgloss.Width(right); gap >= 1 {
			return left + strings.Repeat(" ", gap) + right
		}
	}
	return fit(tabsFor(2)+" "+rightFor(false, false), m.width)
}

// legendKeys is the status bar's legend in play mode: the key table's
// list for the screen shown (keysFor: `n end day` first, the cursor
// keys, the screen's actions, `? help` last). The pane's KEYS section
// lists the same. Inside a modal the bar repeats the modal's footer
// instead (modalFooter).
func (m *Model) legendKeys() []binding {
	return m.keysFor(m.screen)
}

// statusStyle is the colour of the status message by its kind: a
// confirmation in the body colour, a refusal in the warning colour, a
// danger in red. Every site sets the kind (say, refuse, alarm).
func (m *Model) statusStyle() lipgloss.Style {
	switch m.statusKind {
	case statusWarning:
		return theme.Warning
	case statusBad:
		return theme.Bad
	}
	return theme.Body
}

// viewFooter is the status bar: the legend at the left, its last pair
// (`? help`) pinned and the pairs before it dropped whole from the
// right until the rest fits beside it, and the status message at the
// right. The message wins: when the two cannot share the row it shows
// alone, never the legend alone.
func (m *Model) viewFooter() string {
	// During a modal the bar repeats the modal's footer and nothing else.
	if f := m.modalFooter(); f != nil {
		return fit(legend(f), m.width)
	}
	pairs := m.legendKeys()
	last := ""
	if n := len(pairs); n > 0 {
		last = k(pairs[n-1].key, m.labelOf(pairs[n-1]))
		pairs = pairs[:n-1]
	}
	msg := ""
	if m.status != "" {
		msg = m.statusStyle().Render(m.status)
	}
	msgW := lipgloss.Width(msg)
	for n := len(pairs); n >= 0; n-- {
		var keys string
		for _, b := range pairs[:n] {
			keys += k(b.key, m.labelOf(b))
		}
		keys += last
		kw := lipgloss.Width(keys)
		switch {
		case msg == "" && kw <= m.width:
			return keys
		case msg != "" && kw+msgW+1 <= m.width:
			return keys + strings.Repeat(" ", m.width-kw-msgW) + msg
		}
	}
	if msg != "" {
		return truncate(msg, m.width)
	}
	return ""
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

func (m *Model) viewStart() string {
	body := []string{theme.Subtle.Render("a drug empire, one day at a time"), ""}
	for i, o := range []string{"Continue saved run", "New run"} {
		if i == m.startChoice {
			body = append(body, theme.Gold.Render("▸ ")+theme.Selected.Render(" "+o+" "))
		} else {
			body = append(body, "   "+o)
		}
	}
	if m.status != "" {
		// Load errors can be long; wrap inside the box instead of past it.
		body = append(body, "")
		for _, l := range m.wrapLines(m.status) {
			body = append(body, m.statusStyle().Render(l))
		}
	}
	// No title bar yet: the box sits where it does on every other screen.
	box := "\n" + m.modal("KINGPIN", body, m.modalFooter())
	return theme.Plain.Width(m.width).Height(m.height).MaxHeight(m.height).Render(box)
}

// viewHelp is the help modal (#89): the key table, GLOBAL first and then
// each screen's own keys, WORDS, the game's terms in a line each, and
// the one line of voice, in the scrolling modal.
func (m *Model) viewHelp() string {
	body := append(m.helpLines(), "", theme.Subtle.Render("Heat is the antagonist. Greed is always available."))
	return m.modal("HELP", body, m.modalFooter())
}

func (m *Model) viewOver() string {
	w := m.w
	e := w.Over
	var b strings.Builder
	b.WriteString(theme.Bad.Bold(true).Render(strings.ToUpper(e.Cause)) + fmt.Sprintf(" on day %d\n\n", e.Day))
	// One fact a row: the label and its value, values written the way
	// the screens write them.
	facts := [][]any{
		{"days survived", fmt.Sprint(e.Day)},
		{"peak cash", cash(w.Stats.PeakCash)},
		{"total revenue", cash(w.Stats.TotalRevenue)},
		{"units moved", fmt.Sprint(w.Stats.UnitsSold)},
		{"stings", fmt.Sprint(w.Stats.Stings)},
		{"raids", fmt.Sprint(w.Stats.Raids)},
		{"wages", cash(w.Stats.Wages)},
		{"skimmed", cash(w.Stats.Skimmed)},
		{"corners held", fmt.Sprint(w.Held())},
		{"robbed", cash(w.Stats.Robbed)},
	}
	if w.Rival.Arrived > 0 {
		facts = append(facts,
			[]any{"corners won", fmt.Sprint(w.Stats.CornersWon)},
			[]any{"lost to " + truncate(w.Rival.Leader, 12), fmt.Sprint(w.Stats.CornersLost)})
	}
	if s := w.Stats; s.Deals+s.Betrayals+s.BetrayedBy > 0 {
		facts = append(facts,
			[]any{"deals", fmt.Sprint(s.Deals)},
			[]any{"broken by you", fmt.Sprint(s.Betrayals)},
			[]any{"broken by them", fmt.Sprint(s.BetrayedBy)})
	}
	facts = append(facts, []any{"washed", cash(w.Stats.Laundered)}, []any{"seized", cash(w.Stats.Seized)})
	if w.Stats.Shipments > 0 {
		facts = append(facts,
			[]any{"shipped", fmt.Sprintf("%s in %s", plural(w.Stats.Shipped, "unit"), plural(w.Stats.Shipments, "run"))},
			[]any{"lost on the road", fmt.Sprintf("%s in %s", plural(w.Stats.SeizedOnRoad, "unit"), plural(w.Stats.Seizures, "run"))})
	}
	facts = append(facts, []any{"clean cash", cash(w.Player.CleanCash)})
	if w.Stats.Informants+w.Stats.Defections > 0 {
		facts = append(facts, []any{"snitches", fmt.Sprint(w.Stats.Informants)}, []any{"defectors", fmt.Sprint(w.Stats.Defections)})
	}
	rep := w.Player.Reputation
	facts = append(facts,
		[]any{"peak heat", fmt.Sprintf("%.0f", w.Heat.Peak)},
		[]any{"the law", fmt.Sprintf("Chief %s (%s) · DA %s (%s)", truncate(w.Law.Chief.Name, 10), w.Law.Chief.Personality, truncate(w.Law.DA.Name, 10), stanceWord(w.Law.DA.Stance))},
		[]any{"pressure", fmt.Sprintf("%.0f", w.Here().Pressure)},
		[]any{"elections", fmt.Sprint(w.Stats.Elections)},
		[]any{"given to the cities", cash(w.Stats.Funded)},
		[]any{"reputation", fmt.Sprintf("fear %.0f · respect %.0f · notoriety %.0f", rep.Fear, rep.Respect, rep.Notoriety)})
	for _, l := range table([]col{{"stat", kText, 0}, {"value", kText, 0}}, facts, -1, m.modalInner()) {
		b.WriteString(l + "\n")
	}
	if n := len(w.Journal); n > 0 {
		b.WriteString("\nLast headline:\n  " + theme.Subtle.Render(w.Journal[n-1].Text) + "\n")
	}
	return m.modal("GAME OVER", strings.Split(strings.TrimRight(b.String(), "\n"), "\n"), m.modalFooter())
}

func (m *Model) viewReport() string {
	r := m.w.Report
	if r == nil {
		return m.modal("MORNING REPORT", []string{"Nothing happened yet."}, m.modalFooter())
	}
	var body []string // the modal cuts a long line to its width, never wraps it
	section := func(title string, ls []string, style lipgloss.Style) {
		if len(ls) == 0 {
			return
		}
		body = append(body, style.Bold(true).Render(title))
		for _, l := range ls {
			body = append(body, "  "+l)
		}
		body = append(body, "")
	}
	section("PRICES", r.Prices, theme.Good)
	section("SALES", r.Sales, theme.Gold)
	section("SHIPMENTS", r.Shipments, theme.RoadText)
	section("HEAT", r.Heat, theme.Bad)
	section("LAW", r.Law, lawReportStyle)
	section("CREW", r.Crew, theme.CrewText)
	section("TERRITORY", r.Territory, theme.RivalText)
	section("MONEY", append(r.Money, fmt.Sprintf("Cash %s %s %s", cash(r.CashBefore), format.Arrow, cash(r.CashAfter))), theme.Gold)
	section("UPGRADES", r.Upgrades, theme.Gold)
	section("NEWS", r.News, theme.Subtle)
	for len(body) > 0 && body[len(body)-1] == "" {
		body = body[:len(body)-1]
	}
	return m.modal(fmt.Sprintf("MORNING REPORT · DAY %d", r.Day), body, m.modalFooter())
}
