// The key tables themselves (#80): bindings, every key play mode takes,
// and modeBindings, the modals' footers. keys.go says what a binding is
// and dispatches, lists and refuses through them (#275: out of keys.go).

package ui

import (
	"github.com/theclifmeister/kingpin/internal/game"
)

var bindings = []binding{
	{key: "n", label: "end day", help: "end the day: the sims step and the run saves", screens: everywhere, global: true,
		do: func(m *Model, _ string) { m.endDay() }},
	{key: "F", label: "fast-forward", help: "run days until something needs you", screens: on(screenDashboard), global: true,
		do: func(m *Model, _ string) { m.askFast() }},
	// The cursor keys. The map is walked in two dimensions, the market's
	// arrows turn it to the other city, the tree's turn it to the next
	// branch, the journal pages.
	{key: "↑↓", label: "pick", help: "move the cursor (j and k move it too)", keys: upDown, screens: listScreens, global: true, listed: arrowsListed,
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
	{key: "[ ]", label: "city", help: "the next city on the market and the map", keys: []string{"[", "]"}, screens: on(screenMarket, screenMap), global: true,
		do: func(m *Model, key string) { m.cycleCity(dir(key)) }},
	// The rivals screen's own [ ] turns the faction, and says so (#239).
	{key: "[ ]", label: "faction", help: "the next faction at the table", keys: []string{"[", "]"}, screens: on(screenRivals),
		do: func(m *Model, key string) { m.cycleFaction(dir(key)) }},
	// The dashboard's own [ ] picks an alert in ALERTS (#352), and o
	// opens what answers it.
	{key: "[ ]", label: "alert", help: "pick an alert in ALERTS", keys: []string{"[", "]"}, screens: on(screenDashboard), when: hasAlerts, off: offAlerts,
		do: func(m *Model, key string) { m.cycleAlert(dir(key)) }},
	// The dashboard and the market: the day's cart.
	{key: "c", label: "cart", help: "the day's cart: edit its buys and orders", screens: on(screenDashboard, screenMarket),
		do: func(m *Model, _ string) { m.openCart() }},
	// The market, with the cursor on the buyers under the table.
	{key: "a", label: "accept", help: "take the buyer's offer", screens: on(screenMarket), when: onBuyers, off: offBuyers("accept"),
		do: func(m *Model, _ string) { m.answerContract(true) }},
	{key: "x", label: "decline", help: "turn the buyer's offer down", screens: on(screenMarket), when: onBuyers,
		do: func(m *Model, _ string) { m.answerContract(false) }},
	{key: "%", label: "cut", help: "cut a product in the stash where you stand", screens: on(screenMarket), when: onProducts, off: offProducts("cut"),
		do: func(m *Model, _ string) { m.askCut() }},
	{key: "o", label: "cook", help: "the chemist cooks a batch where you stand", screens: on(screenMarket), when: onProducts, listed: hasChemist, off: offProducts("cook"),
		do: func(m *Model, _ string) { m.askCook() }},
	{key: "d", label: "deliver", help: "hand the buyer what the stash here holds", screens: on(screenMarket), when: onBuyers, off: offBuyers("deliver to"),
		do: func(m *Model, _ string) { m.deliverSelected() }},
	{key: "R", label: "restock", help: "buy days of your corners' demand, reviewed", screens: on(screenMarket),
		do: func(m *Model, _ string) { m.askRestock() }},
	{key: "P", label: "presets", help: "the routine's dials in one bundle, reviewed", screens: on(screenMarket),
		do: func(m *Model, _ string) { m.askPresets() }},
	// The journal: one source at a time.
	{key: "f", label: "filter", help: "show one source's headlines, then all again", screens: on(screenJournal),
		do: func(m *Model, _ string) { m.cycleFilter() }},
	// The crew.
	{key: "h", label: "hire", help: "hire the selected candidate", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.hireSelected() }},
	{key: "f", label: "fire", help: "fire the selected member, after asking", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.askFire() }},
	{key: "c", label: "captain", help: "make the selected veteran captain of a city", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.askCaptain() }},
	{key: "l", label: "assign", help: "give the selected lieutenant a city to run", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.askAssign() }},
	{key: "i", label: "investigate", help: "ask who is talking to the police, for a fee", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.askInvestigate() }},
	{key: "$", label: "pay off", help: "buy the selected member's loyalty", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.askPayOff() }},
	{key: "b", label: "bail", help: "clean cash to walk the selected member out", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.askBail() }},
	// The map.
	{key: "c", label: "post runner", help: "post a runner on the selected corner", screens: on(screenMap),
		do: func(m *Model, _ string) { m.askPost(game.RoleRunner) }},
	{key: "e", label: "post enforcer", help: "post an enforcer on the selected corner", screens: on(screenMap),
		do: func(m *Model, _ string) { m.askPost(game.RoleEnforcer) }},
	{key: "a", label: "abandon", help: "give the selected corner up", screens: on(screenMap),
		do: func(m *Model, _ string) { m.abandonSelected() }},
	{key: "w", label: "send enforcers", help: "send the enforcers at the selected corner", screens: on(screenMap),
		do: func(m *Model, _ string) { m.askStrike() }},
	{key: "u", label: "undercut", help: "sell cheap on the rival's corner next door", screens: on(screenMap),
		do: func(m *Model, _ string) { m.askUndercut() }},
	{key: "t", label: "tip police", help: "tip the police on the selected rival corner", screens: on(screenMap),
		do: func(m *Model, _ string) { m.askTip() }},
	{key: "d", label: "buy block", help: "buy the block the selected corner is on", screens: on(screenMap),
		do: func(m *Model, _ string) { m.askDeed() }},
	// The route keys act on the shown cursor (#239): on a corner they
	// refuse and point at the routes, so the hidden routes cursor is
	// never turned by accident (the global r report is theirs to shadow
	// on the map, so they stay live there).
	{key: "r", label: "route dial", help: "the selected route: off, slow, normal, fast", screens: on(screenMap),
		do: func(m *Model, _ string) {
			if !m.onRoutes {
				m.refuse("Pick a route: the dial is on the routes under the grid, not a corner.")
				return
			}
			m.cycleRoute()
		}},
	{key: "R", label: "route target", help: "what the selected route keeps the far end at", screens: on(screenMap),
		do: func(m *Model, _ string) {
			if !m.onRoutes {
				m.refuse("Pick a route: the target is on the routes under the grid, not a corner.")
				return
			}
			m.openTarget()
		}},
	{key: "$", label: "buy checkpoint", help: "buy the checkpoint or customs on the route", screens: on(screenMap), when: mapOnRoutes,
		do: func(m *Model, _ string) { m.askCheckpoint() }},
	{key: "v", label: "driver", help: "put a driver on the selected route", screens: on(screenMap), when: mapOnRoutes,
		do: func(m *Model, _ string) { m.askDriver() }},
	{key: "i", label: "intel", help: "the file on the faction holding the corner", screens: on(screenMap), when: mapOnRivalCorner,
		do: func(m *Model, _ string) {
			c := m.mapSelected()
			r := m.factionOf(c)
			m.jumpIntel(r.Faction(), m.rivalName(r))
		}},
	// The tree.
	{key: "u", label: "buy upgrade", help: "buy the node under the cursor (enter too)", screens: on(screenUpgrades),
		do: func(m *Model, _ string) { m.askUpgrade() }},
	// The ledger.
	{key: "b", label: "buy", help: "buy a front, a house or an asset", screens: on(screenLedger),
		do: func(m *Model, _ string) { m.askFront() }},
	{key: "m", label: "move stock", help: "move stock between the street and the houses", screens: on(screenLedger),
		do: func(m *Model, _ string) { m.askMove() }},
	{key: "e", label: "guard house", help: "post an enforcer inside the selected house", screens: on(screenLedger), when: ledgerOnHouse,
		do: func(m *Model, _ string) { m.askGuard() }},
	{key: "x", label: "drop house", help: "drop the selected house, after asking", screens: on(screenLedger), when: ledgerOnHouse, off: offHouse,
		do: func(m *Model, _ string) { m.askDrop() }},
	{key: "$", label: "bribe", help: "an envelope for the chief or the DA", screens: on(screenLedger),
		do: func(m *Model, _ string) { m.askBribe() }},
	{key: "v", label: "call favour", help: "the bought chief owes you: no raid tonight", screens: on(screenLedger),
		do: func(m *Model, _ string) { m.askFavour() }},
	{key: "f", label: "fund city", help: "give a city clean cash for goodwill", screens: on(screenLedger),
		do: func(m *Model, _ string) { m.askFund() }},
	{key: "u", label: "invest", help: "clean cash into the selected front's levels", screens: on(screenLedger), when: ledgerOnFront,
		do: func(m *Model, _ string) { m.askInvest() }},
	{key: "o", label: "reserve", help: "clean cash into the offshore account", screens: on(screenLedger),
		do: func(m *Model, _ string) { m.askReserve() }},
	{key: "c", label: "cash out", help: "clean cash into the dirty pile, at a fee", screens: on(screenLedger),
		do: func(m *Model, _ string) { m.askCashOut() }},
	{key: "t", label: "export order", help: "the selected export lane's nightly load", screens: on(screenLedger), when: ledgerOnLane,
		do: func(m *Model, _ string) { m.askExport() }},
	{key: "t", label: "buy trophy", help: "buy the selected trophy, after asking", screens: on(screenLedger), when: ledgerOnTrophyOffer,
		do: func(m *Model, _ string) { m.askTrophy() }},
	{key: "w", label: "walk away", help: "retire, vanish, or take the crown", screens: on(screenDashboard),
		do: func(m *Model, _ string) { m.askExit() }},
	{key: "o", label: "open alert", help: "go where the selected alert is answered", screens: on(screenDashboard), when: hasAlerts,
		do: func(m *Model, _ string) { m.openSelectedAlert() }},
	// The rivals.
	{key: "d", label: "propose", help: "offer the rival a truce, tribute or a split", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.askPropose() }},
	{key: "a", label: "accept", help: "take the selected offer", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.answerOffer(true) }},
	{key: "x", label: "decline", help: "turn the selected offer down", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.answerOffer(false) }},
	{key: "i", label: "scout", help: "buy a look at the rival's books", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.askScout() }},
	{key: "w", label: "declare war", help: "enforcers on the faction every night, hit", screens: on(screenRivals), when: notAtWar,
		do: func(m *Model, _ string) { m.askWar() }},
	{key: "w", label: "call off war", help: "stand the enforcers down", screens: on(screenRivals), when: atWar,
		do: func(m *Model, _ string) { m.askCallOffWar() }},
	{key: "$", label: "buy off", help: "pay the rival's muscle to go home", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.askBuyOff() }},
	{key: "h", label: "hit scouts", help: "run a faction's scouts out of town, once", screens: on(screenRivals), when: onScouts,
		do: func(m *Model, _ string) { m.askHitScouts() }},
	// Intel (#45).
	{key: "$", label: "pay cop", help: "a cop's word on the chief and the police", screens: on(screenIntel),
		do: func(m *Model, _ string) { m.askPayCop() }},
	{key: "p", label: "plant spy", help: "send a crew member under with a faction", screens: on(screenIntel),
		do: func(m *Model, _ string) { m.askSpy() }},
	{key: "i", label: "intel", help: "the file on the chief and the police here", screens: on(screenDashboard),
		do: func(m *Model, _ string) { m.jumpIntel(game.SubjectChief, "the chief") }},
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
	{key: "␣", label: "more", help: "open the details whole, uncut", keys: []string{" "}, screens: everywhere, global: true, listed: stripShown,
		do: func(m *Model, _ string) { m.openDetails() }},
	{key: "?", label: "help", help: "this list", screens: everywhere, global: true,
		do: func(m *Model, _ string) { m.mode = modeHelp }},
	// The frame's keys: the title bar carries the screens, help the rest.
	{key: "enter", label: "end day", help: "end the day, after a confirmation", global: true, quiet: true,
		do: func(m *Model, _ string) { m.mode = modeConfirmEnd }},
	{key: "1-9", label: "switch screen", help: "the screens in the title bar's order", keys: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}, global: true, quiet: true,
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
		do: func(m *Model, _ string) { m.ask("new run", (*Model).newConfirm, (*Model).restart) }},
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
	{key: "↑↓", label: "pick", modes: in(modeStart, modePost, modeStrike, modeFront, modeAssign, modePropose, modeGuard, modeDriver, modeSpy, modeCaptain, modeAmbitions)},
	// Every picker takes the digits as select-and-commit and says so (#241).
	{key: "1-9", label: "choose", modes: in(modePost, modeStrike, modeFront, modeAssign, modePropose, modeGuard, modeDriver, modeSpy, modeCaptain, modeAmbitions)},
	{key: "←→", label: "budget", modes: in(modeCaptain)},
	{key: "←→", label: "product", modes: in(modeExport)},
	{key: "1-9", label: "choose", modes: in(modeExit), when: step(0)},
	// The undercut is a dial like the sale's (#241): ←→ turns it, 1-3 pick a notch.
	{key: "←→", label: "dial", modes: in(modeUndercut)},
	{key: "1-3", label: "dial", modes: in(modeUndercut)},
	{key: "↑↓", label: "pick", modes: in(modeExit, modeNewRun), when: step(0)},
	{key: "1-6", label: "choose", modes: in(modeNewRun), when: step(0)},
	{key: "←→", label: "toggle", modes: in(modeNewRun), when: step(2)},
	{key: "↑↓", label: "pick", modes: in(modeMove), when: moveList},
	{key: "↑↓", label: "pick", modes: in(modeCut, modeCook), when: step(0)},
	{key: "↑↓", label: "pick", modes: in(modeSell, modeTarget), when: step(0)},
	{key: "↑↓", label: "pick", modes: in(modeBuy), when: buyList},
	{key: "↑↓", label: "pick", modes: in(modeCart), when: cartHasLines},
	{key: "↑↓", label: "pick", modes: in(modeCard), when: step(0)},
	{key: "↑↓", label: "pick", modes: in(modeBribe), when: step(0)},
	{key: "1-2", label: "choose", modes: in(modeBribe), when: step(0)},
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
	{key: "1-2", label: "pick", modes: in(modeCard), when: cardOf(2)}, // the card's own count (#426); a digit picks, enter decides (#461)
	{key: "1-3", label: "pick", modes: in(modeCard), when: cardOf(3)},
	{key: "m", label: "max", dialogs: true, when: numberStep},
	{key: "h", label: "half", dialogs: true, when: numberStep},
	{key: "↑↓", label: "±1", dialogs: true, when: numberStep},
	{key: "pgup pgdn", label: "±10", dialogs: true, when: numberStep},
	{key: "enter", label: "next", modes: in(modeSell, modeTarget, modePropose, modeFront), when: step(0)},
	{key: "enter", label: "next", modes: in(modeMove), when: moveList},
	{key: "enter", label: "next", modes: in(modeCut, modeCook), when: step(0)},
	{key: "enter", label: "cut", modes: in(modeCut), when: step(1)},
	{key: "enter", label: "cook", modes: in(modeCook), when: step(1)},
	{key: "enter", label: "next", modes: in(modeSell, modeTarget), when: step(1)},
	{key: "enter", label: "next", modes: in(modeBuy), when: buyNext},
	{key: "enter", label: "next", modes: in(modeSell), when: step(2)},
	{key: "enter", label: "select", modes: in(modeStart)},
	{key: "enter", label: "buy", modes: in(modeBuy), when: buyOnce},
	{key: "enter", label: "keep at", modes: in(modeBuy), when: buyKeep},
	{key: "enter", label: "buy", modes: in(modeFront), when: frontBuy},
	{key: "enter", label: "rent", modes: in(modeFront), when: houseRent},
	{key: "enter", label: "buy", modes: in(modeFront), when: assetBuy},
	{key: "enter", label: "move", modes: in(modeMove), when: step(3)},
	{key: "enter", label: "post", modes: in(modeGuard)},
	{key: "enter", label: "drive", modes: in(modeDriver)},
	{key: "enter", label: "sell", modes: in(modeSell), when: sellOnce},
	{key: "enter", label: "sell nightly", modes: in(modeSell), when: sellStanding},
	{key: "enter", label: "set", modes: in(modeTarget), when: step(2)},
	{key: "enter", label: "set", modes: in(modeExport)},
	{key: "enter", label: "quantity", modes: in(modeCart), when: cartHasLines},
	{key: "x", label: "remove", modes: in(modeCart), when: cartHasLines},
	{key: "enter", label: "set", modes: in(modeCart), when: step(1)},
	{key: "enter", label: "propose", modes: in(modePropose), when: step(1)},
	{key: "enter", label: "post", modes: in(modePost)},
	{key: "enter", label: "send", modes: in(modeStrike)},
	{key: "enter", label: "undercut", modes: in(modeUndercut)},
	{key: "enter", label: "assign", modes: in(modeAssign)},
	{key: "enter", label: "name", modes: in(modeCaptain)},
	{key: "enter", label: "next", modes: in(modeFund), when: fundNext},
	{key: "enter", label: "next", modes: in(modeBribe), when: step(0)},
	{key: "enter", label: "pay", modes: in(modeBribe), when: step(1)},
	{key: "enter", label: "give", modes: in(modeFund), when: fundLast},
	{key: "enter", label: "decide", modes: in(modeCard), when: cardPicked}, // once a choice is picked (#461)
	{key: "N", label: "new run", modes: in(modeOver)},
	{key: "esc", label: "close", modes: in(modeOver)},
	{key: "enter", label: "next", modes: in(modeNewRun), when: newRunNext},
	{key: "enter", label: "start", modes: in(modeNewRun), when: newRunStart},
	{key: "D", label: "delete", modes: in(modeStart)},
	{key: "y", label: "<verb>", modes: in(modeConfirm)}, // the payload's verb (#242): fire, buy, go, scout…
	{key: "y enter", label: "end day", modes: in(modeConfirmEnd)},
	{key: "[ ]", label: "alert", keys: []string{"[", "]"}, modes: in(modeConfirmEnd), when: previewHasAlerts}, // the day's preview (#353)
	{key: "o", label: "open alert", modes: in(modeConfirmEnd), when: previewHasAlerts},
	{key: "enter", label: "run", modes: in(modeConfirmFast)},
	{key: "enter", label: "pay", modes: in(modeConfirmBuyOff)},
	{key: "enter", label: "invest", modes: in(modeInvest)},
	{key: "enter", label: "reserve", modes: in(modeReserve)},
	{key: "enter", label: "cash out", modes: in(modeCashOut)},
	{key: "enter", label: "buy", modes: in(modeRestock)},
	{key: "↑↓", label: "pick", modes: in(modePresets), when: step(0)},
	{key: "1-9", label: "choose", modes: in(modePresets), when: step(0)},
	{key: "enter", label: "review", modes: in(modePresets), when: step(0)},
	{key: "s", label: "save current", modes: in(modePresets), when: step(0)},
	{key: "x", label: "delete", modes: in(modePresets), when: presetOnSaved},
	{key: "enter", label: "apply", modes: in(modePresets), when: step(1)},
	{key: "enter", label: "pay", modes: in(modePayCop)},
	{key: "enter", label: "next", modes: in(modeSpy), when: spyOnFactions},
	{key: "enter", label: "plant", modes: in(modeSpy), when: spyOnCrew},
	{key: "enter", label: "next", modes: in(modeExit), when: step(0)},
	{key: "a", label: "ambitions", modes: in(modeExit), when: step(0)}, // the plans toward the ways out (#347)
	{key: "y", label: "retire", modes: in(modeExit), when: exitRetiring},
	{key: "y", label: "vanish", modes: in(modeExit), when: exitVanishing},
	{key: "y", label: "crown", modes: in(modeExit), when: exitCrowning},
	{key: "enter", label: "pin", modes: in(modeAmbitions), when: ambitionUnpinned},
	{key: "enter", label: "unpin", modes: in(modeAmbitions), when: ambitionPinned},
	{key: "q", label: "quit", modes: in(modeStart, modeOver)},
	// The trade's other side (#168): listed on the product step (and a
	// buy's connect step) alone, where the toggle is live.
	{key: "s", label: "sell", modes: in(modeBuy), when: buyList},
	{key: "b", label: "buy", modes: in(modeSell), when: step(0)},
	{key: "⇧tab", label: "back", keys: []string{"shift+tab"}, dialogs: true, when: pastFirstStep},
	{key: "esc", label: "close", dialogs: true, modes: in(modeConfirm, modeConfirmEnd, modeAmbitions)},
	{key: "a", label: "ambitions", modes: in(modeStage)},
	{key: "1-3", label: "open", modes: in(modeReport), when: hasLead},            // the lead's lines (#354)
	{key: "o", label: "open alert", modes: in(modeReport), when: stoppedOnAlert}, // the fast-forward's stop line (#352)
	{key: "enter esc", label: "close", modes: in(modeReport, modeHelp, modeStage)},
	{key: "enter esc", label: "close", modes: in(modeCard), when: step(1)},
	{key: "␣ esc", label: "close", modes: in(modeDetails)},
}
