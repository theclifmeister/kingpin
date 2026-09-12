package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The key table (#80, the epic's "Key legend" convention): every key the
// game accepts in play mode, once, with the label the pane draws it
// with, the sentence help and the README explain it with, and where it
// applies. keyPlay dispatches through it and nothing else, the details
// pane's KEYS section, the modal footers, the help modal and the
// README's key table are rendered from it, and a key pressed on a screen
// that does not list it is refused with a pointer to the one that does,
// in the one spelling: `Hire on the crew screen (4).` The modals' footers
// are the second table, modeBindings, in the same shape.
//
// The table's order is the pane's: `n end day`, the cursor keys, the
// screen's own actions, the keys that work everywhere, `? help`, and
// last the frame's keys, which help and the README list and the pane
// leaves out (quiet). A binding is listed where it names the screen and
// nowhere else (#109: a key is listed where it is used and works
// wherever it makes sense), so a global one names the screens it
// belongs on (`s sell` the dashboard and the market, `p pay dial` the
// crew) and works on the rest unlisted. A screen's own binding for a key
// wins over the global one there (`b` buys a front on the ledger, `r`
// turns a route's dial on the map), and the global one is not listed
// there.

// binding is one key and what it does.
type binding struct {
	key     string               // as the pane draws it: `↑↓`, `[ ]`, `␣`, `pgup pgdn`, `r`
	label   string               // one or two lowercase words, the same everywhere it appears; `<city>` is the other city
	help    string               // one sentence for help and the README
	keys    []string             // the tea.KeyMsg strings it answers to; the key alone when nil
	screens []screen             // the screens whose pane lists it; nil lists it nowhere
	modes   []mode               // the modals whose footer lists it (modeBindings)
	global  bool                 // works on every screen, listed or not; a screen's own binding for the same key wins there
	quiet   bool                 // in help and the README only: the frame's keys, which the title bar and help carry
	when    func(*Model) bool    // live only while this holds; nil is always
	do      func(*Model, string) // what it does in play mode, given the key pressed
}

// accepts reports whether the binding answers to the key pressed.
func (b binding) accepts(key string) bool {
	if b.keys == nil {
		return key == b.key
	}
	for _, k := range b.keys {
		if k == key {
			return true
		}
	}
	return false
}

// names reports whether the binding names the screen as one of its own.
func (b binding) names(s screen) bool {
	for _, x := range b.screens {
		if x == s {
			return true
		}
	}
	return false
}

// live reports whether the binding is in force now.
func (b binding) live(m *Model) bool { return b.when == nil || b.when(m) }

// The raw keys the cursor bindings answer to: j and k move like the
// arrows everywhere.
var (
	upDown    = []string{"up", "down", "k", "j"}
	leftRight = []string{"left", "right"}
	arrows    = []string{"up", "down", "left", "right", "k", "j"}
)

var screenOf = map[screen]string{
	screenDashboard: "dashboard", screenMarket: "market", screenJournal: "journal", screenCrew: "crew",
	screenMap: "map", screenUpgrades: "upgrades", screenLedger: "ledger", screenRivals: "rivals",
}

func on(ss ...screen) []screen { return ss }
func in(ms ...mode) []mode     { return ms }

// everywhere lists a binding on every screen: `n end day`, `? help`
// and, under paneMinWidth, `␣ more`.
var everywhere = on(screenDashboard, screenMarket, screenJournal, screenCrew, screenMap, screenUpgrades, screenLedger, screenRivals)

// listScreens are the screens whose cursor is the plain up-and-down
// one: the map walks two dimensions and lists its own; the tree's
// branch is a list, and its arrows turn the branch (#120).
var listScreens = on(screenDashboard, screenMarket, screenJournal, screenCrew, screenUpgrades, screenLedger, screenRivals)

// onBuyers is the market's cursor being on the buyers under the table.
func onBuyers(m *Model) bool { return m.screen == screenMarket && m.onBuyers }

// step is the dialog open being on its nth page.
func step(n int) func(*Model) bool { return func(m *Model) bool { return m.modalStep() == n } }

// pastFirstStep is the dialog open being on a page past its first: where
// shift+tab has a page to go back to.
func pastFirstStep(m *Model) bool { return m.modalStep() > 0 }

// numberStep is the open modal being on a number field (#112): the buy,
// sell and cart dialogs' quantity page, the target dialog's number (its
// third page, after units or days, #115), the fund dialog's amount, the
// move dialog's quantity (#73) and the invest dialog's levels (#192).
// The field's shortcuts are listed there and nowhere else.
func numberStep(m *Model) bool {
	switch m.mode {
	case modeFund, modeConfirmFast, modeConfirmBuyOff, modeInvest:
		return true
	case modeBuy:
		return buyAt(1)(m)
	case modeSell, modeCart:
		return m.modalStep() == 1
	case modeTarget:
		return m.modalStep() == 2
	case modeMove:
		return m.mv.step == 3
	}
	return false
}

// moveList is the move dialog (#73) being on a list page: from, to or
// the product; its fourth page is the quantity.
func moveList(m *Model) bool { return m.mv.step < 3 }

// fundLast and fundNext are the fund dialog's last page and the page
// before it: the goodwill page is the last unless a campaign is open
// (#193), when the campaign page is.
func fundLast(m *Model) bool { return !m.campaignOpen() || m.fnd.step == 1 }
func fundNext(m *Model) bool { return !fundLast(m) }

// frontBuy and houseRent are the buy picker's second page, on the
// fronts and on the houses (#73).
func frontBuy(m *Model) bool { return m.frontStep == 1 && m.frontKind == pickFront }

func houseRent(m *Model) bool { return m.frontStep == 1 && m.frontKind == pickHouse }

// buyAt is the buy dialog being on its nth step past the connect step
// (#72): 0 the product, 1 the quantity, 2 the repeat and the pay. The
// connect step, where there is one, is a page before them; buyList is
// a list step, the connect's or the product's, and buyNext any step
// enter moves on from.
func buyAt(n int) func(*Model) bool {
	return func(m *Model) bool { return !m.dlg.pick && m.dlg.step == n }
}

func buyList(m *Model) bool { return m.dlg.pick || m.dlg.step == 0 }

func buyNext(m *Model) bool { return m.dlg.pick || m.dlg.step < 2 }

// buyOnce and buyKeep are the buy dialog's last step at once and at keep
// at (#113): enter buys, or sets the supply contract.
func buyOnce(m *Model) bool { return buyAt(2)(m) && m.dlg.repeat == repeatOnce }

func buyKeep(m *Model) bool { return buyAt(2)(m) && m.dlg.repeat == repeatKeep }

// buyPay is the buy dialog's last step at once with a connect who gives
// credit (#72): c turns the pay notch.
func buyPay(m *Model) bool { return buyOnce(m) && m.creditOffered() }

// sellOnce and sellStanding are the sell dialog's last step at once and
// at standing (#114): enter queues the order, or sets it standing.
func sellOnce(m *Model) bool { return m.modalStep() == 3 && m.dlg.repeat == repeatOnce }

func sellStanding(m *Model) bool { return m.modalStep() == 3 && m.dlg.repeat == repeatStanding }

// stripShown is the terminal being too narrow for the pane beside MAIN,
// so the strip stands in for it and ␣ opens it whole (#111).
func stripShown(m *Model) bool { return m.width < paneMinWidth }

var bindings = []binding{
	{key: "n", label: "end day", help: "end the day: the sims step and the run saves", screens: everywhere, global: true,
		do: func(m *Model, _ string) { m.endDay() }},
	{key: "F", label: "fast-forward", help: "run days until something needs you", screens: on(screenDashboard), global: true,
		do: func(m *Model, _ string) { m.askFast() }},
	// The cursor keys. The map is walked in two dimensions, the market's
	// arrows turn it to the other city, the tree's turn it to the next
	// branch, the journal pages.
	{key: "↑↓", label: "pick", help: "move the cursor (j and k move it too)", keys: upDown, screens: listScreens, global: true,
		do: func(m *Model, key string) { m.moveCursor(0, dir(key)) }},
	{key: "↑↓←→", label: "pick", help: "walk the map's grid", keys: arrows, screens: on(screenMap),
		do: func(m *Model, key string) {
			if key == "left" || key == "right" {
				m.moveCursor(dir(key), 0)
			} else {
				m.moveCursor(0, dir(key))
			}
		}},
	{key: "←→", label: "city", help: "turn the market to the other city", keys: leftRight, screens: on(screenMarket),
		do: func(m *Model, key string) { m.moveCursor(dir(key), 0) }},
	{key: "←→", label: "branch", help: "turn the tree to the next branch", keys: leftRight, screens: on(screenUpgrades),
		do: func(m *Model, key string) { m.moveCursor(dir(key), 0) }},
	{key: "pgup pgdn", label: "page", help: "page through the journal", keys: []string{"pgup", "pgdown"}, screens: on(screenJournal),
		do: func(m *Model, key string) {
			if key == "pgup" {
				m.journalPage(-1)
			} else {
				m.journalPage(1)
			}
		}},
	{key: "[ ]", label: "city", help: "turn the market or the map to the other city", keys: []string{"[", "]"}, screens: on(screenMarket, screenMap), global: true,
		do: func(m *Model, key string) { m.cycleCity(dir(key)) }},
	// The dashboard and the market: the day's cart.
	{key: "c", label: "cart", help: "the day's cart: edit its buys and orders", screens: on(screenDashboard, screenMarket),
		do: func(m *Model, _ string) { m.openCart() }},
	// The market, with the cursor on the buyers under the table.
	{key: "a", label: "accept", help: "take the buyer's offer", screens: on(screenMarket), when: onBuyers,
		do: func(m *Model, _ string) { m.answerContract(true) }},
	{key: "x", label: "decline", help: "turn the buyer's offer down", screens: on(screenMarket), when: onBuyers,
		do: func(m *Model, _ string) { m.answerContract(false) }},
	{key: "d", label: "deliver", help: "hand the buyer what the stash here holds", screens: on(screenMarket), when: onBuyers,
		do: func(m *Model, _ string) { m.deliverSelected() }},
	// The journal: one source at a time.
	{key: "f", label: "filter", help: "show one source's headlines, then all again", screens: on(screenJournal),
		do: func(m *Model, _ string) { m.cycleFilter() }},
	// The crew.
	{key: "h", label: "hire", help: "hire the selected candidate", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.hireSelected() }},
	{key: "f", label: "fire", help: "fire the selected member, after asking", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.askFire() }},
	{key: "t", label: "assign", help: "give the selected lieutenant a city to run", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.askAssign() }},
	{key: "i", label: "investigate", help: "ask who is talking to the police, for a fee", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.askInvestigate() }},
	{key: "$", label: "pay off", help: "buy the selected member's loyalty", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.askPayOff() }},
	// The map.
	{key: "c", label: "post runner", help: "post a runner on the selected corner", screens: on(screenMap),
		do: func(m *Model, _ string) { m.askPost("runner") }},
	{key: "e", label: "post enforcer", help: "post an enforcer on the selected corner", screens: on(screenMap),
		do: func(m *Model, _ string) { m.askPost("enforcer") }},
	{key: "a", label: "abandon", help: "give the selected corner up", screens: on(screenMap),
		do: func(m *Model, _ string) { m.abandonSelected() }},
	{key: "w", label: "send enforcers", help: "send the enforcers at the selected corner", screens: on(screenMap),
		do: func(m *Model, _ string) { m.askStrike() }},
	{key: "u", label: "undercut", help: "sell cheap on the rival's corner next door", screens: on(screenMap),
		do: func(m *Model, _ string) { m.askUndercut() }},
	{key: "t", label: "tip police", help: "tip the police on the selected rival corner", screens: on(screenMap),
		do: func(m *Model, _ string) { m.askTip() }},
	{key: "r", label: "route dial", help: "the selected route: off, slow, normal, fast", screens: on(screenMap),
		do: func(m *Model, _ string) { m.cycleRoute() }},
	{key: "R", label: "route target", help: "what the selected route keeps the far end at", screens: on(screenMap),
		do: func(m *Model, _ string) { m.openTarget() }},
	// The tree.
	{key: "u", label: "buy upgrade", help: "buy the node under the cursor (enter too)", keys: []string{"u", "enter"}, screens: on(screenUpgrades),
		do: func(m *Model, _ string) { m.askUpgrade() }},
	// The ledger.
	{key: "b", label: "buy front", help: "buy a front or rent a house", screens: on(screenLedger),
		do: func(m *Model, _ string) { m.askFront() }},
	{key: "m", label: "move stock", help: "move stock between the street and the houses", screens: on(screenLedger),
		do: func(m *Model, _ string) { m.askMove() }},
	{key: "e", label: "guard house", help: "post an enforcer inside the selected house", screens: on(screenLedger), when: ledgerOnHouse,
		do: func(m *Model, _ string) { m.askGuard() }},
	{key: "x", label: "drop house", help: "drop the selected house, after asking", screens: on(screenLedger), when: ledgerOnHouse,
		do: func(m *Model, _ string) { m.askDrop() }},
	{key: "f", label: "fund city", help: "give a city clean cash for goodwill", screens: on(screenLedger),
		do: func(m *Model, _ string) { m.askFund() }},
	{key: "i", label: "invest", help: "clean cash into the selected front's levels", screens: on(screenLedger), when: ledgerOnFront,
		do: func(m *Model, _ string) { m.askInvest() }},
	{key: "enter", label: "buy / dial", help: "buy the offer or turn the route selected", screens: on(screenLedger), when: ledgerActable,
		do: func(m *Model, _ string) { m.ledgerEnter() }},
	// The rivals.
	{key: "d", label: "propose", help: "offer the rival a truce, tribute or a split", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.askPropose() }},
	{key: "y", label: "accept", help: "take the selected offer", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.answerOffer(true) }},
	{key: "x", label: "decline", help: "turn the selected offer down", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.answerOffer(false) }},
	{key: "i", label: "scout", help: "buy a look at the rival's books", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.askScout() }},
	{key: "$", label: "buy off", help: "pay the rival's muscle to go home", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.askBuyOff() }},
	// Everywhere, listed where it is used.
	{key: "b", label: "buy", help: "buy where you stand or a lieutenant runs", screens: on(screenDashboard, screenMarket), global: true,
		do: func(m *Model, _ string) { m.openDialog(modeBuy) }},
	{key: "s", label: "sell", help: "queue a street sale in the city shown", screens: on(screenDashboard, screenMarket), global: true,
		do: func(m *Model, _ string) { m.openDialog(modeSell) }},
	{key: "x", label: "cancel order", help: "cancel order, else standing, else contract", screens: on(screenDashboard, screenMarket), global: true,
		do: func(m *Model, _ string) { m.cancelSelected() }},
	{key: "l", label: "lie low", help: "lie low today: no sales, heat fades faster", screens: on(screenDashboard), global: true,
		do: func(m *Model, _ string) { m.toggleLieLow() }},
	{key: "p", label: "pay dial", help: "the pay dial: stingy, fair, generous", screens: on(screenCrew), global: true,
		do: func(m *Model, _ string) { m.cyclePay() }},
	{key: "d", label: "launder dial", help: "the launder dial: careful, normal, greedy", screens: on(screenLedger), global: true,
		do: func(m *Model, _ string) { m.cycleLaunder() }},
	{key: "g", label: "go to <city>", help: "go to the other city; the stock stays put", screens: on(screenDashboard, screenMap), global: true,
		do: func(m *Model, _ string) { m.askTravel() }},
	{key: "r", label: "report", help: "reopen the morning report", screens: on(screenDashboard, screenJournal), global: true,
		do: func(m *Model, _ string) {
			if m.w.Report != nil {
				m.mode = modeReport
			}
		}},
	{key: "␣", label: "more", help: "open the details whole (under 100 columns)", keys: []string{" "}, screens: everywhere, global: true, when: stripShown,
		do: func(m *Model, _ string) { m.openDetails() }},
	{key: "?", label: "help", help: "this list", screens: everywhere, global: true,
		do: func(m *Model, _ string) { m.mode = modeHelp }},
	// The frame's keys: the title bar carries the screens, help the rest.
	{key: "enter", label: "end day", help: "end the day, after a confirmation", global: true, quiet: true,
		do: func(m *Model, _ string) { m.mode = modeConfirmEnd }},
	{key: "1-8", label: "switch screen", help: "the screens in the title bar's order", keys: []string{"1", "2", "3", "4", "5", "6", "7", "8"}, global: true, quiet: true,
		do: func(m *Model, key string) { m.switchScreen(screen(key[0] - '1')) }},
	{key: "tab", label: "next screen", help: "next screen; shift+tab back, in dialogs too", keys: []string{"tab", "shift+tab"}, global: true, quiet: true,
		do: func(m *Model, key string) {
			if key == "tab" {
				m.switchScreen((m.screen + 1) % screenCount)
			} else {
				m.switchScreen((m.screen + screenCount - 1) % screenCount)
			}
		}},
	{key: "ctrl+s", label: "save", help: "save now; the end of the day saves too", global: true, quiet: true,
		do: func(m *Model, _ string) { m.save() }},
	{key: "N", label: "new run", help: "start over, after a confirmation", global: true, quiet: true,
		do: func(m *Model, _ string) { m.mode = modeConfirmNew }},
	{key: "q", label: "quit", help: "save and quit", global: true, quiet: true,
		do: func(m *Model, _ string) { m.quit() }}, // saves and sets quitting; keyPlay returns tea.Quit
}

// modeBindings are the modals' footers: what each mode lists, in its
// order. Back is one key and close is one key (#110): every modal
// closes on esc and lists `esc close`, nothing lists `esc back`, and a
// dialog with pages lists `⇧tab back` on every page past the first
// (shift+tab goes back a page keeping the earlier pages' input, tab
// forward once the page is complete; the key column draws `⇧tab`,
// shift+tab being too wide for the footer at 80 columns). Confirmations
// are `y <verb>` / `esc close` (any other key still declines, tab and
// shift+tab aside); pickers and dialogs `enter <verb>` / `esc close` (q
// still closes, silently); the end of the day is the one that also takes
// enter and says so; the report, the card's outcome and help close on
// enter or esc. Every number field (#112, numberField) lists `m max  h
// half  ↑↓ ±1  pgup pgdn ±10` before its enter (a is max's unlisted
// alias). The keys themselves are handled by handleKey; the table is
// what the footer and the status bar say.
var modeBindings = []binding{
	{key: "↑↓", label: "pick", modes: in(modeStart, modePost, modeStrike, modeUndercut, modeFront, modeAssign, modePropose, modeGuard)},
	{key: "↑↓", label: "pick", modes: in(modeMove), when: moveList},
	{key: "↑↓", label: "pick", modes: in(modeSell, modeTarget), when: step(0)},
	{key: "↑↓", label: "pick", modes: in(modeBuy), when: buyList},
	{key: "↑↓", label: "pick", modes: in(modeCart), when: cartHasLines},
	{key: "↑↓", label: "pick", modes: in(modeCard), when: step(0)},
	{key: "←→", label: "dial", modes: in(modeSell), when: step(2)},
	{key: "←→", label: "repeat", modes: in(modeBuy), when: buyAt(2)},
	{key: "←→", label: "repeat", modes: in(modeSell), when: step(3)},
	{key: "←→", label: "dial", modes: in(modeCart), when: cartOnSell},
	{key: "←→", label: "city", modes: in(modeFund), when: step(0)},
	{key: "←→", label: "ticket", modes: in(modeFund), when: step(1)},
	{key: "←→", label: "units/days", modes: in(modeTarget), when: step(1)},
	{key: "1-3", label: "dial", modes: in(modeSell), when: step(2)},
	{key: "1-2", label: "repeat", modes: in(modeBuy), when: buyAt(2)},
	{key: "c", label: "pay", modes: in(modeBuy), when: buyPay},
	{key: "1-2", label: "repeat", modes: in(modeSell), when: step(3)},
	{key: "1-3", label: "dial", modes: in(modeCart), when: cartOnSell},
	{key: "1-3", label: "choose", modes: in(modeCard), when: step(0)},
	{key: "m", label: "max", modes: in(modeBuy, modeSell, modeTarget, modeCart, modeFund, modeConfirmFast, modeMove, modeConfirmBuyOff, modeInvest), when: numberStep},
	{key: "h", label: "half", modes: in(modeBuy, modeSell, modeTarget, modeCart, modeFund, modeConfirmFast, modeMove, modeConfirmBuyOff, modeInvest), when: numberStep},
	{key: "↑↓", label: "±1", modes: in(modeBuy, modeSell, modeTarget, modeCart, modeFund, modeConfirmFast, modeMove, modeConfirmBuyOff, modeInvest), when: numberStep},
	{key: "pgup pgdn", label: "±10", modes: in(modeBuy, modeSell, modeTarget, modeCart, modeFund, modeConfirmFast, modeMove, modeConfirmBuyOff, modeInvest), when: numberStep},
	{key: "enter", label: "next", modes: in(modeSell, modeTarget, modePropose, modeFront), when: step(0)},
	{key: "enter", label: "next", modes: in(modeMove), when: moveList},
	{key: "enter", label: "next", modes: in(modeSell, modeTarget), when: step(1)},
	{key: "enter", label: "next", modes: in(modeBuy), when: buyNext},
	{key: "enter", label: "next", modes: in(modeSell), when: step(2)},
	{key: "enter", label: "select", modes: in(modeStart)},
	{key: "enter", label: "buy", modes: in(modeBuy), when: buyOnce},
	{key: "enter", label: "keep at", modes: in(modeBuy), when: buyKeep},
	{key: "enter", label: "buy", modes: in(modeFront), when: frontBuy},
	{key: "enter", label: "rent", modes: in(modeFront), when: houseRent},
	{key: "enter", label: "move", modes: in(modeMove), when: step(3)},
	{key: "enter", label: "post", modes: in(modeGuard)},
	{key: "y", label: "drop", modes: in(modeConfirmDrop)},
	{key: "enter", label: "sell", modes: in(modeSell), when: sellOnce},
	{key: "enter", label: "sell nightly", modes: in(modeSell), when: sellStanding},
	{key: "enter", label: "set", modes: in(modeTarget), when: step(2)},
	{key: "enter", label: "quantity", modes: in(modeCart), when: cartHasLines},
	{key: "x", label: "remove", modes: in(modeCart), when: cartHasLines},
	{key: "enter", label: "set", modes: in(modeCart), when: step(1)},
	{key: "enter", label: "propose", modes: in(modePropose), when: step(1)},
	{key: "enter", label: "post", modes: in(modePost)},
	{key: "enter", label: "send", modes: in(modeStrike)},
	{key: "enter", label: "undercut", modes: in(modeUndercut)},
	{key: "enter", label: "assign", modes: in(modeAssign)},
	{key: "enter", label: "next", modes: in(modeFund), when: fundNext},
	{key: "enter", label: "give", modes: in(modeFund), when: fundLast},
	{key: "enter", label: "decide", modes: in(modeCard), when: step(0)},
	{key: "enter", label: "new run", modes: in(modeOver)},
	{key: "D", label: "delete", modes: in(modeStart)},
	{key: "y", label: "new run", modes: in(modeConfirmNew)},
	{key: "y", label: "delete", modes: in(modeConfirmDelete)},
	{key: "y", label: "fire", modes: in(modeConfirmFire)},
	{key: "y enter", label: "end day", modes: in(modeConfirmEnd)},
	{key: "y enter", label: "run", modes: in(modeConfirmFast)},
	{key: "y", label: "buy", modes: in(modeConfirmUpgrade)},
	{key: "y", label: "ask", modes: in(modeConfirmInvestigate)},
	{key: "y", label: "pay", modes: in(modeConfirmPayOff)},
	{key: "y", label: "go", modes: in(modeConfirmTravel)},
	{key: "y", label: "scout", modes: in(modeConfirmScout)},
	{key: "y", label: "boost", modes: in(modeConfirmBoost)},
	{key: "y", label: "tip", modes: in(modeConfirmTip)},
	{key: "y enter", label: "pay", modes: in(modeConfirmBuyOff)},
	{key: "enter", label: "invest", modes: in(modeInvest)},
	{key: "q", label: "quit", modes: in(modeStart, modeOver)},
	// The trade's other side (#168): listed on the product step (and a
	// buy's connect step) alone, where the toggle is live.
	{key: "s", label: "sell", modes: in(modeBuy), when: buyList},
	{key: "b", label: "buy", modes: in(modeSell), when: step(0)},
	{key: "⇧tab", label: "back", keys: []string{"shift+tab"}, modes: in(modeBuy, modeSell, modeTarget, modeCart, modePropose, modeFront, modeMove, modeFund), when: pastFirstStep},
	{key: "esc", label: "close", modes: in(modeBuy, modeSell, modeTarget, modePropose, modePost, modeStrike, modeUndercut, modeFront, modeAssign, modeFund, modeCart, modeMove, modeGuard,
		modeConfirmNew, modeConfirmDelete, modeConfirmFire, modeConfirmEnd, modeConfirmUpgrade, modeConfirmInvestigate, modeConfirmPayOff, modeConfirmTravel, modeConfirmFast, modeConfirmDrop,
		modeConfirmScout, modeConfirmBoost, modeConfirmTip, modeConfirmBuyOff, modeInvest)},
	{key: "enter esc", label: "close", modes: in(modeReport, modeHelp, modeStage)},
	{key: "enter esc", label: "close", modes: in(modeCard), when: step(1)},
	{key: "␣ esc", label: "close", modes: in(modeDetails)},
}

// modalStep is the page the open dialog is on: the buy, sell and target
// dialogs and the propose dialog have pages, and the card's outcome is
// its second.
func (m *Model) modalStep() int {
	switch m.mode {
	case modeBuy:
		// The connect step (#72) is a page before the product where
		// the dialog has one, so back has it to go to.
		if m.dlg.pick {
			return 0
		}
		if m.dlg.paged {
			return m.dlg.step + 1
		}
		return m.dlg.step
	case modeSell:
		return m.dlg.step
	case modeTarget:
		return m.tgt.step
	case modeFront:
		return m.frontStep
	case modeMove:
		return m.mv.step
	case modeCart:
		return m.crt.step
	case modePropose:
		return m.proposeStep
	case modeFund:
		return m.fnd.step
	case modeCard:
		if m.cardDone {
			return 1
		}
	}
	return 0
}

// dir is the direction a cursor key moves: -1 up, left or [, +1 down,
// right or ].
func dir(key string) int {
	switch key {
	case "up", "k", "left", "[":
		return -1
	}
	return 1
}

// keysFor is what the pane's KEYS lists for a screen, in table order:
// every live binding that names the screen, the quiet ones left out, and
// none that an earlier one for the same key shadows there. That is the
// rule lookup dispatches by (the first live binding naming the screen
// that takes the key), so a key is listed exactly where pressing it does
// what the label says: the market's `x decline` while the cursor is on
// the buyers, `x cancel order` while it is on the table.
func (m *Model) keysFor(s screen) []binding {
	var out []binding
	for _, b := range bindings {
		if b.quiet || !b.names(s) || !b.live(m) {
			continue
		}
		shadowed := false
		for _, o := range out {
			for _, k := range rawKeys(b) {
				if o.accepts(k) {
					shadowed = true
				}
			}
		}
		if !shadowed {
			out = append(out, b)
		}
	}
	return out
}

// modeKeys is a modal's footer: the bindings listed for the mode that
// are live on its page.
func (m *Model) modeKeys(md mode) []binding {
	var out []binding
	for _, b := range modeBindings {
		for _, x := range b.modes {
			if x == md && b.live(m) {
				out = append(out, b)
			}
		}
	}
	return out
}

// rawKeys is every key string a binding answers to.
func rawKeys(b binding) []string {
	if b.keys == nil {
		return []string{b.key}
	}
	return b.keys
}

// labelOf is the label as the pane draws it, the other city named.
func (m *Model) labelOf(b binding) string {
	if strings.Contains(b.label, "<city>") && m.w != nil {
		return strings.ReplaceAll(b.label, "<city>", m.w.CityName(m.travelTo()))
	}
	return b.label
}

// lookup is the binding a key pressed on the current screen goes to: the
// screen's own first, then a global one. It also reports whether the
// screen has a binding for the key at all: one of its own that is not
// live now (the market's buyer keys with the cursor on the table) takes
// the key and does nothing, so it is neither pointed elsewhere nor let
// through to a global that means something else.
func (m *Model) lookup(key string) (b binding, found, own bool) {
	for _, x := range bindings {
		if x.accepts(key) && x.names(m.screen) {
			own = true
			if x.live(m) {
				return x, true, true
			}
		}
	}
	for _, x := range bindings {
		if x.accepts(key) && x.global && x.live(m) {
			return x, true, own
		}
	}
	return binding{}, false, own
}

// pointer is the refusal for a key pressed where it does nothing: where
// it does, in the one spelling, `Hire on the crew screen (4).`, one
// sentence per binding that takes it; empty for a key nothing takes.
func pointer(key string) string {
	var ps []string
	for _, b := range bindings {
		if !b.accepts(key) || b.global {
			continue
		}
		var where []string
		for i, s := range b.screens {
			p := screenPointer(s)
			if i > 0 {
				p = strings.TrimPrefix(p, "on ") // `on the dashboard screen (1) or the market screen (2)`
			}
			where = append(where, p)
		}
		ps = append(ps, strings.ToUpper(b.label[:1])+b.label[1:]+" "+strings.Join(where, " or ")+".")
	}
	return strings.Join(ps, " ")
}

// screenPointer is the spelling every pointer to a screen uses in prose:
// `on the crew screen (4)`.
func screenPointer(s screen) string {
	return fmt.Sprintf("on the %s screen (%d)", screenOf[s], int(s)+1)
}

// helpGroups are the help modal's and the README's groups: GLOBAL, the
// keys that work everywhere (once, whatever screens list them), then
// each screen's own, every binding once under the first screen it
// names.
type helpGroup struct {
	title string
	keys  []binding
}

func helpGroups() []helpGroup {
	groups := []helpGroup{{title: "GLOBAL"}}
	for s := screen(0); s < screenCount; s++ {
		groups = append(groups, helpGroup{title: strings.ToUpper(screenOf[s])})
	}
	for _, b := range bindings {
		i := 0 // a global one is everyone's, wherever the pane lists it
		if !b.global {
			i = int(b.screens[0]) + 1
		}
		groups[i].keys = append(groups[i].keys, b)
	}
	var out []helpGroup
	for _, g := range groups {
		if len(g.keys) > 0 {
			out = append(out, g)
		}
	}
	return out
}

// The help modal's columns: the key, the label, a dash, and the sentence
// in what is left of the modal at 80 columns, so no row is ever cut.
// helpSentenceW is the most a binding's help may run to; a word's line
// in WORDS has the label's room too.
const (
	helpKeyW      = 9
	helpLabelW    = 14
	helpSentenceW = modalMax - 4 - helpKeyW - 2 - helpLabelW - 3
)

// helpRow is one line of the help modal: `key  label — help`.
func helpRow(key, label, help string) string {
	return theme.Key.Render(fit(key, helpKeyW)) + "  " + theme.Subtle.Render(fit(label, helpLabelW)) + " — " + help
}

// words are the help modal's last group, WORDS: the terms the screens
// use without explaining, one line each. Each fits beside the key
// column at 80 columns; the pane's spelling for a key (`␣`).
var words = [][2]string{
	{"dial", "quiet, normal or aggressive: a sale's volume against its heat"},
	{"float", "the dirty cash the wash and the road leave for the street"},
	{"target", "what a route keeps the far end at: units or days of demand"},
	{"file", "the DA's evidence: stings and raids add pages, enough indicts"},
	{"drift", "a held corner nobody works goes back to the street in days"},
	{"undercut", "sell cheap on a rival corner next door: they lose, no heat"},
	{"keep at", "a supply contract: the stash bought back to a level daily"},
	{"through", "a buy into a city a lieutenant runs for you, at a markup"},
	{"standing", "a sell order that stands nightly until cancelled, at a cut"},
	{"connect", "who sells you product: a price, a lot, a temper, a rel"},
	{"credit", "a connect's book: take now, pay in days, or they answer"},
	{"unlock", "a line crossed: a product, front, connect or role opens"},
	{"house", "a rented stash off the street: rent in clean; a raid hits one"},
	{"pane", "the details beside MAIN from 100 columns, always open"},
	{"strip", "the pane's one line under 100 columns; ␣ opens it over MAIN"},
	{"tier", "the stage a run is in, shown once: Corner to Distribution"},
	{"scout", "a paid look at the rival's books: a snapshot that goes stale"},
	{"boost", "the enforcers rob a rival corner's till, not the corner"},
	{"scene", "a short animation on a morning that matters; any key skips it"},
}

// helpLines is the help modal's body: every binding, grouped, one a
// line as `key  label — help`, then WORDS.
func (m *Model) helpLines() []string {
	var body []string
	for i, g := range helpGroups() {
		if i > 0 {
			body = append(body, "")
		}
		body = append(body, theme.PanelTitle.Render(g.title))
		for _, b := range g.keys {
			body = append(body, helpRow(b.key, b.label, b.help))
		}
	}
	body = append(body, "", theme.PanelTitle.Render("WORDS"))
	for _, w := range words {
		body = append(body, theme.Key.Render(fit(w[0], helpKeyW))+"  "+w[1])
	}
	return body
}

// ReadmeKeys is the README's key table, rendered from the same bindings
// help is: one row a binding, grouped as help groups them. cmd/keys
// prints it and TestReadmeMatchesKeys holds the README to it.
func ReadmeKeys() string {
	var b strings.Builder
	b.WriteString("| Key | Legend | What it does | Where |\n|---|---|---|---|\n")
	for _, g := range helpGroups() {
		for _, k := range g.keys {
			where := "everywhere"
			if !k.global {
				var names []string
				for _, s := range k.screens {
					names = append(names, screenOf[s])
				}
				where = strings.Join(names, ", ")
			}
			label := strings.ReplaceAll(k.label, "<city>", `\<city\>`)
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", k.key, label, k.help, where)
		}
	}
	return b.String()
}

// tutorialLine is the one line of prose that names keys: the
// dashboard's `No sales queued. Press s to sell, n to end the day.`, the
// first thing a new player reads, its two keys drawn the pane's way.
// Every other key the player is told about is in the pane's KEYS or a
// modal's footer.
func tutorialLine() string {
	sub := theme.Subtle.Render
	return sub("No sales queued. Press ") + theme.Key.Render("s") + sub(" to sell, ") + theme.Key.Render("n") + sub(" to end the day.")
}

// The README's key table sits between these markers; cmd/keys writes it
// there and TestReadmeMatchesKeys reads it back. The captures sit between
// ReadmeCaptureBegin(name) and ReadmeCaptureEnd the same way, written by
// `go test ./internal/ui -run TestReadmeCaptures -update` from the rich
// fixture on a fixed seed and read back by the same test.
const (
	ReadmeKeysBegin  = "<!-- keys:begin -->"
	ReadmeKeysEnd    = "<!-- keys:end -->"
	ReadmeCaptureEnd = "<!-- capture:end -->"
)

// ReadmeCaptureBegin is the marker a named capture starts at:
// `<!-- capture:dashboard-80x24 -->`.
func ReadmeCaptureBegin(name string) string { return "<!-- capture:" + name + " -->" }

// SpliceReadme replaces what sits between the begin and end markers with
// body; it reports false when the markers are missing.
func SpliceReadme(readme, begin, end, body string) (string, bool) {
	i := strings.Index(readme, begin)
	if i < 0 {
		return readme, false
	}
	j := strings.Index(readme[i:], end)
	if j < 0 {
		return readme, false
	}
	return readme[:i+len(begin)] + "\n" + body + readme[i+j:], true
}

// ReadmeSection is what sits between the begin and end markers, the
// newline after the begin marker dropped: what the README holds.
func ReadmeSection(readme, begin, end string) (string, bool) {
	i := strings.Index(readme, begin)
	if i < 0 {
		return "", false
	}
	j := strings.Index(readme[i:], end)
	if j < 0 {
		return "", false
	}
	return readme[i+len(begin)+1 : i+j], true
}

// SpliceReadmeKeys replaces the table between the README's key markers.
func SpliceReadmeKeys(readme, table string) (string, bool) {
	return SpliceReadme(readme, ReadmeKeysBegin, ReadmeKeysEnd, table)
}

// ReadmeKeysSection is the table as the README carries it: what cmd/keys
// writes and what the README must hold.
func ReadmeKeysSection(readme string) (string, bool) {
	return ReadmeSection(readme, ReadmeKeysBegin, ReadmeKeysEnd)
}
