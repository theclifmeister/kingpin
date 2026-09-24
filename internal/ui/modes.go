package ui

// The mode state machine is one table (#274): modes holds, for each
// mode, what a switch per question used to spread over the package (its
// view, its key handler, whether tab walks its pages, the dialog state
// its pages live in). The switches drifted apart once: hasPages lacked
// the spy dialog while its view and its keys had it (#272). A mode is
// added here, in the constants and in the table's row, and
// TestModeTableIsComplete fails on a row left empty.

import (
	tea "github.com/charmbracelet/bubbletea"
)

type mode int

const (
	modeStart mode = iota // the start menu: a save slot to continue or start in, or quit
	modePlay
	modeConfirm // one yes-or-no confirmation, its payload in Model.cfm (#242)
	modeReport
	modeBuy
	modeSell
	modeOver
	modeConfirmEnd
	modeHelp
	modePost          // pick who to post on the selected corner
	modeStrike        // pick how hard to send the enforcers at the selected corner
	modeFront         // pick a front to buy
	modeStage         // the stage entered this morning (#149), before the card and the report
	modeCard          // a dilemma card, before the morning report
	modeTarget        // the route target dialog: product -> units or days -> the number
	modePropose       // pick a deal to put to the rival: kind, then terms
	modeAssign        // pick the city a lieutenant runs
	modeFund          // give a city clean cash for goodwill
	modeDetails       // the details pane as an overlay, where the terminal is too narrow to hold it beside MAIN
	modeCart          // the day's cart: its buys and orders, editable until the day ends
	modeConfirmFast   // run days until something needs you (#116): the cap, then enter
	modeUndercut      // pick the dial to undercut the selected rival corner at (#68)
	modeMove          // move stock between the street and the houses in a city (#73): from, to, product, quantity
	modeGuard         // pick the enforcer who guards the selected house (#73)
	modeConfirmBuyOff // pay the rival's muscle to go home: the heads, then enter (#70)
	modeCut           // cut a product where you stand (#47): the product, then the percent added
	modeCook          // a chemist's cook order (#47): the product, then the units
	modeInvest        // clean cash into the selected front's levels (#192): the levels, then enter
	modeBribe         // an envelope for the chief or the DA (#42): the target, then the amount
	modeReserve       // clean cash into the offshore account (#195): the amount, then enter
	modeDriver        // pick the driver who rides the selected route (#46)
	modePayCop        // pay a cop for a word on the police (#45): the amount, then enter
	modeSpy           // plant a spy (#45): the faction, then who goes under
	modeExit          // walk away (#49): retire on the account or vanish on a new identity, then the confirmation
	modeNewRun        // a new run from the start menu (#50): the character, the seed, the hard DA
	modeRestock       // top the stash up to days of demand (#356): the days, the plan under them, then enter
	modeCaptain       // name a captain (#346): the city, the budget ←→, then enter
	modePresets       // the operation presets (#357): the list, then the review of what one changes
	modeCount
)

// modeSpec is one mode's row.
type modeSpec struct {
	name string // what a test's message calls the mode

	// view draws the modal over the screen: the body between the title
	// and the footer. nil is the play screen's frame, modePlay's alone.
	// The start menu's and the new-run dialog's are viewStart, which
	// View reaches through onStart before the table.
	view func(*Model) string

	// key takes a key pressed in the mode, after ctrl+c, an interstitial
	// and the tab rule (handleKey).
	key keyFunc

	// pages is the modal's keys walking pages, so tab and shift+tab are
	// its (#110); nil is a modal with none, which leaves them alone
	// rather than closing on them. The fund dialog has a second page,
	// the campaign, only while one is open (#193).
	pages func(*Model) bool

	// paged is the state of the dialog open (#243): its page and its
	// number field. nil is a modal with no pages of its own: a
	// confirmation, a reader, the card.
	paged func(*Model) paged
}

// keyFunc is a mode's key handler.
type keyFunc func(*Model, tea.KeyMsg) (tea.Model, tea.Cmd)

// byKey and byKeyCmd adapt a handler that reads the key's string: one
// that leaves the model where it is, and one that may return a command.
func byKey(f func(*Model, string)) keyFunc {
	return func(m *Model, k tea.KeyMsg) (tea.Model, tea.Cmd) {
		f(m, k.String())
		return m, nil
	}
}

func byKeyCmd(f func(*Model, string) (tea.Model, tea.Cmd)) keyFunc {
	return func(m *Model, k tea.KeyMsg) (tea.Model, tea.Cmd) { return f(m, k.String()) }
}

func always(*Model) bool { return true }

// The dialog states, one reader each, for the paged column.
func dlgState(m *Model) paged   { return &m.dlg }
func tgtState(m *Model) paged   { return &m.tgt }
func crtState(m *Model) paged   { return &m.crt }
func mvState(m *Model) paged    { return &m.mv }
func labState(m *Model) paged   { return &m.lab }
func brState(m *Model) paged    { return &m.br }
func fndState(m *Model) paged   { return &m.fnd }
func spyState(m *Model) paged   { return &m.spy }
func exitState(m *Model) paged  { return &m.exit }
func nrState(m *Model) paged    { return &m.nr }
func frontState(m *Model) paged { return &m.front }
func propState(m *Model) paged  { return &m.prop }
func amtState(m *Model) paged   { return &m.amt }
func pickState(m *Model) paged  { return &m.pick }
func preState(m *Model) paged   { return &m.pre }

// modes is the table, one row a mode. It is filled in init: its
// functions read it back (a view's footer asks the open dialog's page),
// which a package-level initializer would make a cycle.
var modes [modeCount]modeSpec

func init() {
	modes = [modeCount]modeSpec{
		modeStart:         {name: "start", view: (*Model).viewStart, key: byKeyCmd((*Model).keyStart)},
		modePlay:          {name: "play", key: byKeyCmd((*Model).keyPlay)},
		modeConfirm:       {name: "confirm", view: (*Model).viewConfirm, key: byKey((*Model).keyConfirm)},
		modeReport:        {name: "report", view: (*Model).viewReport, key: byKey((*Model).keyReport)},
		modeBuy:           {name: "buy", view: (*Model).viewDialog, key: (*Model).keyDialog, pages: always, paged: dlgState},
		modeSell:          {name: "sell", view: (*Model).viewDialog, key: (*Model).keyDialog, pages: always, paged: dlgState},
		modeOver:          {name: "over", view: (*Model).viewOver, key: byKeyCmd((*Model).keyOver)},
		modeConfirmEnd:    {name: "confirm end", view: (*Model).viewConfirmEnd, key: byKey((*Model).keyConfirmEnd)},
		modeHelp:          {name: "help", view: (*Model).viewHelp, key: byKey((*Model).keyHelp)},
		modePost:          {name: "post", view: (*Model).viewPost, key: byKey((*Model).keyPost), paged: pickState},
		modeStrike:        {name: "strike", view: (*Model).viewStrike, key: byKey((*Model).keyStrike), paged: pickState},
		modeFront:         {name: "front", view: (*Model).viewFront, key: byKey((*Model).keyFront), pages: always, paged: frontState},
		modeStage:         {name: "stage", view: (*Model).viewStage, key: byKeyCmd((*Model).keyStage)},
		modeCard:          {name: "card", view: (*Model).viewCard, key: byKeyCmd((*Model).keyCard)},
		modeTarget:        {name: "target", view: (*Model).viewTarget, key: (*Model).keyTarget, pages: always, paged: tgtState},
		modePropose:       {name: "propose", view: (*Model).viewPropose, key: byKey((*Model).keyPropose), pages: always, paged: propState},
		modeAssign:        {name: "assign", view: (*Model).viewAssign, key: byKey((*Model).keyAssign), paged: pickState},
		modeFund:          {name: "fund", view: (*Model).viewFund, key: (*Model).keyFund, pages: (*Model).campaignOpen, paged: fndState},
		modeDetails:       {name: "details", view: (*Model).viewDetails, key: byKey((*Model).keyDetails)},
		modeCart:          {name: "cart", view: (*Model).viewCart, key: (*Model).keyCart, pages: always, paged: crtState},
		modeConfirmFast:   {name: "confirm fast", view: (*Model).viewFast, key: (*Model).keyFast, paged: amtState},
		modeUndercut:      {name: "undercut", view: (*Model).viewUndercut, key: byKey((*Model).keyUndercut), paged: pickState},
		modeMove:          {name: "move", view: (*Model).viewMove, key: (*Model).keyMove, pages: always, paged: mvState},
		modeGuard:         {name: "guard", view: (*Model).viewGuard, key: byKey((*Model).keyGuard), paged: pickState},
		modeConfirmBuyOff: {name: "confirm buy-off", view: (*Model).viewBuyOff, key: (*Model).keyBuyOff, paged: amtState},
		modeCut:           {name: "cut", view: (*Model).viewLab, key: (*Model).keyLab, pages: always, paged: labState},
		modeCook:          {name: "cook", view: (*Model).viewLab, key: (*Model).keyLab, pages: always, paged: labState},
		modeInvest:        {name: "invest", view: (*Model).viewInvest, key: (*Model).keyInvest, paged: amtState},
		modeBribe:         {name: "bribe", view: (*Model).viewBribe, key: (*Model).keyBribe, pages: always, paged: brState},
		modeReserve:       {name: "reserve", view: (*Model).viewReserve, key: (*Model).keyReserve, paged: amtState},
		modeDriver:        {name: "driver", view: (*Model).viewDriver, key: byKey((*Model).keyDriver), paged: pickState},
		modePayCop:        {name: "pay cop", view: (*Model).viewPayCop, key: (*Model).keyPayCop, paged: amtState},
		modeSpy:           {name: "spy", view: (*Model).viewSpy, key: byKey((*Model).keySpy), pages: func(m *Model) bool { return !m.spy.single }, paged: spyState},
		modeExit:          {name: "exit", view: (*Model).viewExit, key: byKey((*Model).keyExit), pages: always, paged: exitState},
		modeNewRun:        {name: "new run", view: (*Model).viewStart, key: (*Model).keyNewRun, pages: always, paged: nrState},
		modeRestock:       {name: "restock", view: (*Model).viewRestock, key: (*Model).keyRestock, paged: amtState},
		modeCaptain:       {name: "captain", view: (*Model).viewCaptain, key: byKey((*Model).keyCaptain), paged: pickState},
		modePresets:       {name: "presets", view: (*Model).viewPresets, key: byKey((*Model).keyPresets), pages: always, paged: preState},
	}
}

// hasPages is a modal whose keys walk pages: the dialogs tab and
// shift+tab move through (the pages column).
func (m *Model) hasPages(md mode) bool {
	p := modes[md].pages
	return p != nil && p(m)
}

// openPaged is the state of the dialog open (#243), or nil for a
// modal with no pages of its own (the paged column).
func (m *Model) openPaged() paged {
	if p := modes[m.mode].paged; p != nil {
		return p(m)
	}
	return nil
}
