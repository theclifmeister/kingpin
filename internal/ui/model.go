// Package ui is the Bubble Tea front end. The root Model owns the world and
// the clock, switches between screens, and routes key presses to modal
// dialogs. Simulations never import this package.
package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Model is the root Bubble Tea model.
type Model struct {
	cfg   *content.Config
	sess  *engine.Session // the run's engine (#296): the sims, the clock and the bus, one assembly
	rules engine.Rules    // sess.Rules(): the sims' costs, prices and previews (#297)
	w     *game.World     // sess.World(), the run on screen
	opts  Options

	scene       *anim.Player // the scene on screen (#152), nil while none is: the tick chain runs on it
	titleEffect string       // the effect the title loop's current pass plays (#153), the next pass avoids it
	sceneGen    int          // which scene the outstanding tick was issued for
	ticking     bool         // a tick is on its way
	mapScene    []mapFlip    // the corners that changed hands this morning, for the map's scene (#158): until their map is shown or the next morning
	mapPlaying  []string     // the corners the map's scene up now burns, in its frame's order

	width, height  int
	screen         screen
	mode           mode
	cfm            confirm       // the open confirmation's payload (#242): what y does, what the modal shows, where no goes
	city           string        // city the market and map screens show; follows you when you travel
	cursor         int           // product cursor shared by market screen and dialogs
	crewCursor     int           // row on the crew screen: roster first, then candidates
	subjectID      int           // the member a fire, pay-off, bail or assignment is about (#243)
	pick           picker        // the one-page picker open: post, strike, undercut, assign, guard or driver (#243)
	front          frontPicker   // the buy picker (#73): the kind, then the offers
	prop           proposeDialog // the propose dialog (#43): the kind, then the terms
	mapCursor      int           // corner selected on the map
	mapTop         int           // the first row of the map's grid drawn, scrolled to keep the cursor in view
	routeCursor    int           // route selected under the map's grid
	onRoutes       bool          // the map's arrows are on the routes, past the bottom row
	onPolice       bool          // the dashboard's arrows are on HEAT, CASH and LAW, past the product table (#355)
	buyerCursor    int           // contract selected under the market's product table
	onBuyers       bool          // the market's arrows are on the buyers, past the bottom row
	supplierCursor int           // connect selected under the market's buyers (#72)
	onSuppliers    bool          // the market's arrows are on the connects, past the buyers
	branch         int           // branch shown on the upgrades screen, an index into content.Branches: a view cursor like city
	upgradeCursor  []int         // node selected in each branch, one an entry of content.Branches, so a branch left and returned to is where it was
	upgradeID      string        // node awaiting the buy confirmation
	mv             moveDialog
	lab            labDialog // the cut and the cook dialogs (#47)
	ledgerCursor   int       // row on the ledger: fronts, then routes, then offers
	ledgerScroll   int       // first line of the ledger MAIN shows, following the cursor
	stage          int       // the tier whose stage is showing (#149)
	cardCursor     int       // choice highlighted on the dilemma card
	cardDone       bool      // the card is answered; the outcome is showing
	dealCursor     int       // offer selected on the rivals screen
	factionCursor  int       // faction the rivals screen is turned to (#43): an index into World.Rivals
	modalScroll    int       // first body line the open modal shows
	outcome        string    // what the last answer did, while it shows
	journalCursor  int       // headline selected on the journal screen, newest first
	journalTop     int       // first headline the journal screen shows
	journalSeen    int       // the journal's length when the journal screen was last shown; not saved, a view cursor like city
	journalFilter  string    // the source the journal screen shows, or every one when empty (#122); a view cursor like journalSeen
	dlg            dialog
	tgt            targetDialog
	crt            cartDialog
	fnd            fundDialog
	br             bribeDialog
	amt            amountDialog     // the one-field dialog open (#275): invest, reserve, pay a cop, buy off, fast-forward
	spy            spyDialog        // the spy dialog (#45)
	exit           exitDialog       // the walk-away dialog (#49)
	nr             newRunDialog     // the new-run dialog (#50)
	intelCursor    int              // row on the intel screen (#45)
	fastStop       string           // the report's first line after a fast-forward (`Stopped after 3 days: …`), until the next day ends
	slot           int              // the save slot this run lives in: where ctrl+s, the end of the day and quitting save
	profile        *game.Profile    // the game around the runs (#50): loaded with the model, written when a run ends and when a daily starts
	profileErr     string           // what loading it said, for the start menu: a corrupt one set aside, a newer one left alone
	unlocked       []string         // what the run that just ended unlocked, for the summary
	now            func() time.Time // the wall clock, read here alone (#50): the daily's date, the profile's; a test sets it
	startChoice    int              // row on the start menu: the slots, then Quit
	status         string
	statusKind     statusKind           // how the status bar colours the message; set where the status is
	flash          []events.Enforcement // the enforcements of the last tick, via the bus: the bust's scene reads the level (#155)
	reportScene    reportSceneKind      // which scene the report opened on (#159), while m.scene is up in modeReport
	quitting       bool
	quitErr        error // the save on the way out failed: main says so once the screen is back
}

// New wires config, simulations, clock and bus together. With a run in
// any slot the start menu offers the slots; on a fresh install a run
// starts in slot 1.
func New(cfg *content.Config, opts Options) (*Model, error) {
	m, err := wire(cfg, opts)
	if err != nil {
		return nil, err
	}
	for _, s := range game.Slots() {
		if !s.Empty {
			m.mode = modeStart
			return m, nil
		}
	}
	m.newRun(1)
	return m, nil
}

// NewSlot is New opening one slot directly (`kingpin -slot N`): the run
// in it continues, or a new one starts in it if it is empty. A run that
// does not load leaves the player on the start menu with the error, the
// cursor on the slot.
func NewSlot(cfg *content.Config, slot int, opts Options) (*Model, error) {
	if slot < 1 || slot > game.SlotCount {
		return nil, fmt.Errorf("%w: %d (1 to %d)", game.ErrBadSlot, slot, game.SlotCount)
	}
	m, err := wire(cfg, opts)
	if err != nil {
		return nil, err
	}
	m.startChoice = slot - 1
	m.pickStart()
	return m, nil
}

func wire(cfg *content.Config, opts Options) (*Model, error) {
	sess, err := engine.New(cfg)
	if err != nil {
		return nil, err
	}
	m := &Model{
		cfg:   cfg,
		sess:  sess,
		rules: sess.Rules(),
		opts:  opts,
	}
	m.now = time.Now
	m.loadProfile()
	return m, nil
}

// World exposes the current world for tests.
func (m *Model) World() *game.World { return m.w }

// newRun starts a fresh run in the slot, which is where it saves from
// now on.
func (m *Model) newRun(slot int) {
	m.slot = slot
	m.startRun(m.freshSeed())
}

// newSeed is a fresh random seed off the wall clock: the UI's alone
// (#50: time.Now is read here and never in game, sim or the harness,
// TestNoWallClockInTheSims).
func newSeed() uint64 { return uint64(time.Now().UnixNano()) }

// freshSeed is the seed a new run takes: the wall clock's, or the one
// Options.Seeds hands out (#292).
func (m *Model) freshSeed() uint64 {
	if m.opts.Seeds != nil {
		return m.opts.Seeds()
	}
	return newSeed()
}

// restart begins a new run in the current slot as the run that is up
// began (#50: the same character and the hard DA; a daily's is a run
// as its character, not the daily again), on a fresh seed: N's
// confirmation and n on the summary.
func (m *Model) restart() {
	start := game.Start{}
	if m.w != nil {
		start = game.Start{Character: m.w.Start.Character, HardDA: m.w.Start.HardDA}
	}
	m.startRunWith(m.freshSeed(), start)
}

// startRun begins a run from a seed in the current slot: a new world,
// the dashboard and the cursors at their start. A test that wants a run
// it can replay (the README's captures) passes the seed.
func (m *Model) startRun(seed uint64) { m.startRunWith(seed, game.Start{}) }

// startRunWith is startRun as a start (#50): the character, the hard
// DA and the daily the run begins as, sim.NewWorldWith's.
func (m *Model) startRunWith(seed uint64, start game.Start) {
	m.stop() // the title's loop ends with the menu
	m.w = m.sess.NewRun(seed, start)
	m.unlocked = nil
	m.mode = modePlay
	m.screen = screenDashboard
	m.cursor = 0
	m.crewCursor = 0
	m.branch = 0
	m.upgradeCursor = nil
	m.city = m.w.Player.Location
	m.mapCursor = m.yourCorner()
	m.flash = nil
	m.fastStop = ""
	m.mapScene = nil
	who := ""
	if ch := m.cfg.Characters.Character(m.w.Start.Character); ch != nil && m.w.Start.Character != "" {
		who = " " + ch.Name + "."
	}
	m.say(fmt.Sprintf("New run.%s %s, %s in your pocket. Seed %d.", who, m.w.Here().Name, money(m.w.Player.DirtyCash), m.w.Seed))
	if err := m.sess.Save(m.slot); err != nil {
		m.alarm("Save failed: " + err.Error())
	}
	m.journalFilter = "" // a new run's journal is read whole
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

// cycleFaction turns the rivals screen to the next faction (#43), round
// the table.
func (m *Model) cycleFaction(d int) {
	n := len(m.w.Rivals)
	if n == 0 {
		return
	}
	m.factionCursor = ((m.factionCursor+d)%n + n) % n
}

// faction is the faction the rivals screen is turned to (#43): the one
// under the cursor, the rival at home by default.
func (m *Model) faction() *game.RivalState {
	rs := m.w.Rivals
	if len(rs) == 0 {
		return m.w.Rival()
	}
	return rs[clamp(&m.factionCursor, len(rs))]
}

// factionOf is the faction holding a corner, or the rival at home for
// a corner nobody's.
func (m *Model) factionOf(c *game.Corner) *game.RivalState {
	if c != nil && c.Owner == game.OwnerRival {
		if r := m.w.Faction(c.Faction); r != nil {
			return r
		}
	}
	return m.w.Rival()
}

// factionStyle is a faction's colour (#43), by its seat at the table.
func (m *Model) factionStyle(id string) lipgloss.Style {
	return theme.FactionText(m.w.FactionIndex(id))
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
	m.supplierCursor, m.onSuppliers = 0, false
}

// continueRun picks up the run saved in the slot, which is where it
// saves from now on.
func (m *Model) continueRun(slot int) error {
	w, err := m.sess.Load(slot)
	if err != nil {
		return err
	}
	m.stop() // the title's loop ends with the menu
	m.slot = slot
	m.w = w
	m.mode = modePlay
	m.unlocked = nil
	if w.Over != nil {
		m.finish(false) // the scene has been seen
	} else if w.StagePending() > 0 || w.Dilemmas.Pending != nil {
		m.showStage() // saved on a stage or a card: it is still waiting
	}
	m.city = w.Player.Location
	m.mapCursor = m.yourCorner()
	m.fastStop = ""
	m.mapScene = nil
	m.say(fmt.Sprintf("Continued day %d.", w.Day))
	m.journalFilter = ""
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

// endDay ends one day, n's and the confirmation's: the day steps and
// the morning opens. A fast-forward (#116) ends several the same way.
func (m *Model) endDay() {
	if m.w.Over != nil {
		m.finish(false)
		return
	}
	m.morning(m.stepDay())
}

// stepDay ends one day through the session (n's) and does what the TUI
// does with every day (dayEnded); it returns the tick's events. The
// last fast-forward's stop line goes with the day it was for.
func (m *Model) stepDay() []events.Event {
	evs := m.sess.EndDay()
	m.dayEnded(evs)
	return evs
}

// dayEnded is what the TUI does with every day that ends, n's or a
// fast-forward's (Session.FastForward calls it): the day's enforcement
// kept for the bust scene, the last stop line gone, the save, the
// journal.
func (m *Model) dayEnded(evs []events.Event) {
	m.flash = nil
	for _, e := range evs {
		if ev, ok := e.(events.Enforcement); ok {
			m.flash = append(m.flash, ev)
		}
	}
	m.fastStop = ""
	m.save()
	m.refreshJournal()
}

// morning opens the day that has just begun: the run over, else the
// stage (#149), the card or the report, with the danger winning the
// status bar: the tell that somebody on the payroll is talking, in red,
// over the save. evs are the tick's events, the day's that just ended
// (a fast-forward's the stopping day's): the corners that changed
// hands in them are kept for the map's scene (#158).
func (m *Model) morning(evs []events.Event) {
	m.mapScene = mapFlips(evs)
	if m.w.Over != nil {
		m.finish(true) // the ending's scene (#156), then the summary
		return
	}
	if m.talking() {
		m.alarm("Somebody is talking. Investigate " + screenPointer(screenCrew) + ".")
	}
	m.showStage()
}

// QuitErr is why the save on the way out failed, or nil: the program
// has quit, so the screen cannot say it and main does.
func (m *Model) QuitErr() error { return m.quitErr }

func (m *Model) save() {
	if err := m.sess.Save(m.slot); err != nil {
		m.alarm("Save failed: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Day %d saved.", m.w.Day))
}

// Init starts nothing: the UI redraws only on a key or a resize, and on
// a tick while a scene is up (#152), which the first WindowSizeMsg
// starts for the start menu's loop, inside Update.
func (m *Model) Init() tea.Cmd { return nil }

// Update is the Bubble Tea update loop. Every branch ends in tick(): the
// next frame's tick while a scene is up, nil otherwise, so no command
// leaves here in play mode.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		m.titleLoop()
		return m, m.tick()
	case frameMsg:
		return m, m.onFrame(msg)
	case tea.KeyMsg:
		before := m.mode
		r, cmd := m.handleKey(msg)
		if m.mode != before {
			m.modalScroll = 0 // a new modal opens at its top
		}
		m.holdEnds() // #203: a held report's loop ends with the report
		if cmd == nil {
			m.mapSceneStart() // #158: the map's scene, on the key that shows it
			cmd = m.tick()
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
	if m.skip() {
		return m, nil // any key ends an interstitial and is consumed (#152)
	}
	// Tab and shift+tab are the screens' keys and a dialog's pages
	// (#110): a modal with no pages, a confirmation included, leaves them
	// alone rather than closing on them. The start menu takes neither.
	if m.mode != modePlay && (key == "tab" || key == "shift+tab") && !m.hasPages(m.mode) {
		return m, nil
	}
	return modes[m.mode].key(m, k)
}

// keyConfirm is the one yes-or-no confirmation (#242): y does what its
// payload says, any other key goes back where it came from.
func (m *Model) keyConfirm(key string) {
	if key == "y" || key == "Y" {
		m.cfm.act(m)
	} else {
		m.mode = m.cfm.back
	}
}

// keyConfirmEnd ends the day on y or enter; any other key goes back.
func (m *Model) keyConfirmEnd(key string) {
	switch key {
	case "y", "Y", "enter":
		m.endDay()
	default:
		m.mode = modePlay
	}
}

// keyHelp scrolls the help modal; any other key closes it.
func (m *Model) keyHelp(key string) {
	if !m.scrollModal(key) {
		m.mode = modePlay
	}
}

// keyDetails scrolls the details overlay; esc, space, enter and q close
// it.
func (m *Model) keyDetails(key string) {
	switch key {
	case "esc", " ", "enter", "q":
		m.mode = modePlay
	default:
		m.scrollModal(key)
	}
}

// keyReport scrolls the morning report; enter, esc, space, r and q
// close it.
func (m *Model) keyReport(key string) {
	switch key {
	case "enter", "esc", " ", "r", "q":
		m.mode = modePlay
	default:
		m.scrollModal(key)
	}
}

// keyOver is the summary of a run that ended: n starts a new one in the
// slot, esc goes to the start menu, q quits.
func (m *Model) keyOver(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "n":
		_ = game.DeleteSave(m.slot)
		m.restart()
	case "esc":
		m.mode = modeStart
		m.startChoice = m.slot - 1
		m.status = ""
	case "q":
		return m.quit()
	default:
		m.scrollModal(key)
	}
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

// cancelSelected cancels the order on the selected product in the city
// acted on, or, with none, its standing order (#114), or, with neither,
// clears its supply contract (#113).
func (m *Model) cancelSelected() {
	if m.cursor >= len(m.w.Products) {
		return
	}
	id := m.w.Products[m.cursor]
	if city := m.actionCity(); m.w.Cities[city] != nil {
		if _, ok := m.w.Order(city, id); ok {
			m.sess.CancelSell(city, id)
			m.say("Order cancelled.")
		} else if o, ok := m.w.YourStanding(city, id); ok {
			m.sess.CancelStanding(city, id)
			m.say(fmt.Sprintf("Standing order cancelled: %d %s in %s no longer sells nightly.", o.Qty, m.w.ProductName(id), m.w.CityName(city)))
		} else if c, ok := m.w.Supplied(city, id); ok {
			// With no order to cancel, x clears the supply contract
			// (#113): the stash is no longer kept there.
			m.sess.ClearSupply(city, id)
			m.say(fmt.Sprintf("Contract cleared: %s in %s is no longer kept at %d.", m.w.ProductName(id), m.w.CityName(city), c.Units))
		}
	}
}

// toggleLieLow turns lying low on and off for today.
func (m *Model) toggleLieLow() {
	m.sess.SetLieLow(!m.w.Today.LieLow)
	if m.w.Today.LieLow {
		m.say("Lying low today: no sales, heat fades faster.")
	} else {
		m.say("Back on the corner.")
	}
}

// openDetails is space under paneMinWidth: the pane's sections open
// whole as an overlay where the strip is, the one place they can be read
// at 80 columns (#87). Beside MAIN the pane is always open and space
// does nothing (#111).
func (m *Model) openDetails() {
	if m.width < paneMinWidth {
		m.mode = modeDetails
	}
}

func (m *Model) quit() (tea.Model, tea.Cmd) {
	if m.w != nil {
		if err := m.sess.Save(m.slot); err != nil {
			m.quitErr = fmt.Errorf("the run was not saved: %w", err)
		}
	}
	m.stop()
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
	if m.onStart() {
		return m.viewStart()
	}
	view := modes[m.mode].view
	if view == nil {
		return m.frame(m.viewScreen(), m.details(), m.paneKeys(), m.accent())
	}
	body := view(m)
	body = theme.Plain.Width(m.width).Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).Render(body)
	return lines(m.viewTitle(), body, m.viewFooter())
}

// viewConfirm is the open confirmation, as its payload draws it (#242).
func (m *Model) viewConfirm() string { return m.cfm.view(m) }

// viewConfirmEnd is the cart in a sentence, then what the night does.
func (m *Model) viewConfirmEnd() string {
	return m.modal("END THE DAY?", []string{m.endDayLine(), "The sims step and the run autosaves."}, m.modalFooter())
}

// viewDetails is the details pane as an overlay, under paneMinWidth.
func (m *Model) viewDetails() string { return m.overlay(m.details(), m.paneKeys(), m.accent()) }

func (m *Model) viewTitle() string {
	w := m.w
	// The Journal tab carries the count of headlines you have not read
	// (`Journal 3`, the count in the news accent) wherever its name
	// fits; the digits-only bar drops it with the name.
	unread := m.journalUnread()
	tabsFor := func(short int, badge bool) string {
		var tabs []string
		for i, sc := range screens {
			var label string
			switch short {
			case 0:
				label = fmt.Sprintf("%d %s", i+1, sc.name)
			case 1:
				label = fmt.Sprintf("%d %s", i+1, sc.short)
			default:
				label = fmt.Sprintf("%d", i+1)
			}
			if screen(i) == screenJournal && badge && unread > 0 {
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
		parts = append(parts, m.dayLabel(), theme.Gold.Render("dirty "+cash(w.Player.DirtyCash)))
		if clean {
			parts = append(parts, theme.Subtle.Render("clean "+cash(w.Player.CleanCash)))
		}
		parts = append(parts, heatStyle(here.Heat).Render(fmt.Sprintf("heat %.0f", here.Heat)))
		return strings.Join(parts, sep) + " "
	}
	// Try the roomy layout first, then progressively shorter ones: the
	// tabs shorten before the right side loses anything, and the short
	// names drop the unread badge before they give way to digits (#235:
	// nine short names fit 120 columns with nothing to spare, so the
	// badge alone pushed every screen to digits exactly when there was
	// news to point at).
	for _, try := range []struct {
		short       int
		badge       bool
		city, clean bool
	}{
		{0, true, true, true},
		{1, true, true, true}, {1, true, true, false}, {1, true, false, false},
		{1, false, true, true}, {1, false, true, false}, {1, false, false, false},
		{2, false, false, false},
	} {
		left, right := tabsFor(try.short, try.badge), rightFor(try.city, try.clean)
		if gap := m.width - lipgloss.Width(left) - lipgloss.Width(right); gap >= 1 {
			return left + strings.Repeat(" ", gap) + right
		}
	}
	return fit(tabsFor(2, false)+" "+rightFor(false, false), m.width)
}

// paneKeys is the pane's KEYS section in play mode: the key table's
// list for the screen shown (keysFor: `n end day` first, the cursor
// keys, the screen's actions, `? help` last). The status bar lists none
// of it (#109): it is the message and `? help`, and inside a modal the
// modal's footer (modalFooter).
func (m *Model) paneKeys() []binding {
	return m.keysFor(m.screen)
}

// helpPair is the `? help` pair the status bar pins, the key table's.
func (m *Model) helpPair() string {
	for _, b := range bindings {
		if b.key == "?" {
			return k(b.key, m.labelOf(b))
		}
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

// viewHelp is the help modal (#89): the key table, GLOBAL first and then
// each screen's own keys, WORDS, the game's terms in a line each, and
// the one line of voice, in the scrolling modal.
func (m *Model) viewHelp() string {
	body := append(m.helpLines(), "", theme.Subtle.Render("Heat is the antagonist. Greed is always available."))
	return m.modal("HELP", body, m.modalFooter())
}
