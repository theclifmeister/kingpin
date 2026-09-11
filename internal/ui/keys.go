package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The key table (#80, the epic's "Key legend" convention): every key the
// game accepts in play mode, once, with the label the legend draws it
// with, the sentence help and the README explain it with, and where it
// applies. keyPlay dispatches through it and nothing else, the status
// bar's legend, the details pane's KEYS section, the help modal and the
// README's key table are rendered from it, and a key pressed on a screen
// that does not list it is refused with a pointer to the one that does,
// in the one spelling: `Hire on the crew screen (4).` The modals' footers
// are the second table, modeBindings, in the same shape.
//
// The table's order is the legend's: `n end day`, the cursor keys, the
// screen's own actions, the keys that work everywhere, `? help`, and
// last the frame's keys, which help and the README list and the legend
// and the pane leave out (quiet). A screen's own binding for a key wins
// over the global one there (`b` buys a front on the ledger, `r` turns a
// route's dial on the map), and the global one is not listed there.

// binding is one key and what it does.
type binding struct {
	key     string               // as the legend draws it: `↑↓`, `[ ]`, `␣`, `pgup pgdn`, `r`
	label   string               // one or two lowercase words, the same everywhere it appears; `<city>` is the other city
	help    string               // one sentence for help and the README
	keys    []string             // the tea.KeyMsg strings it answers to; the key alone when nil
	screens []screen             // the screens that list it; every screen when nil and global
	modes   []mode               // the modals whose footer lists it (modeBindings)
	global  bool                 // works on every screen; a screen's own binding for the same key wins there
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

// listed reports whether the legend lists the binding on the screen: one
// of its own, or global with none named ([ ] works everywhere and is
// listed where it matters, the market and the map).
func (b binding) listed(s screen) bool {
	if b.screens == nil {
		return b.global
	}
	return b.names(s)
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

// onBuyers is the market's cursor being on the buyers under the table.
func onBuyers(m *Model) bool { return m.screen == screenMarket && m.onBuyers }

// step is the dialog open being on its nth page.
func step(n int) func(*Model) bool { return func(m *Model) bool { return m.modalStep() == n } }

var bindings = []binding{
	{key: "n", label: "end day", help: "end the day: the sims step, the run autosaves", global: true,
		do: func(m *Model, _ string) { m.endDay() }},
	// The cursor keys. The map and the tree are walked in two dimensions,
	// the market's arrows turn it to the other city, the journal pages.
	{key: "↑↓", label: "pick", help: "move the cursor (j and k move it too)", keys: upDown, global: true,
		do: func(m *Model, key string) { m.moveCursor(0, dir(key)) }},
	{key: "↑↓←→", label: "pick", help: "walk the map's grid or the tree's columns", keys: arrows, screens: on(screenMap, screenUpgrades),
		do: func(m *Model, key string) {
			if key == "left" || key == "right" {
				m.moveCursor(dir(key), 0)
			} else {
				m.moveCursor(0, dir(key))
			}
		}},
	{key: "←→", label: "city", help: "turn the market to the other city", keys: leftRight, screens: on(screenMarket),
		do: func(m *Model, key string) { m.moveCursor(dir(key), 0) }},
	{key: "pgup pgdn", label: "page", help: "page through the journal", keys: []string{"pgup", "pgdown"}, screens: on(screenJournal),
		do: func(m *Model, key string) {
			if key == "pgup" {
				m.journalPage(-1)
			} else {
				m.journalPage(1)
			}
		}},
	{key: "[ ]", label: "city", help: "turn the market and the map to the other city", keys: []string{"[", "]"}, screens: on(screenMarket, screenMap), global: true,
		do: func(m *Model, key string) { m.cycleCity(dir(key)) }},
	// The market, with the cursor on the buyers under the table.
	{key: "a", label: "accept", help: "take the buyer's offer", screens: on(screenMarket), when: onBuyers,
		do: func(m *Model, _ string) { m.answerContract(true) }},
	{key: "x", label: "decline", help: "turn the buyer's offer down", screens: on(screenMarket), when: onBuyers,
		do: func(m *Model, _ string) { m.answerContract(false) }},
	{key: "d", label: "deliver", help: "hand the buyer what the stash here holds", screens: on(screenMarket), when: onBuyers,
		do: func(m *Model, _ string) { m.deliverSelected() }},
	// The crew.
	{key: "h", label: "hire", help: "hire the selected candidate", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.hireSelected() }},
	{key: "f", label: "fire", help: "fire the selected member, after asking", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.askFire() }},
	{key: "t", label: "assign", help: "give the selected lieutenant a city to run", screens: on(screenCrew),
		do: func(m *Model, _ string) { m.askAssign() }},
	{key: "i", label: "investigate", help: "ask who is talking to the police, for a price", screens: on(screenCrew),
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
	{key: "r", label: "route dial", help: "the selected route: off, slow, normal, fast", screens: on(screenMap),
		do: func(m *Model, _ string) { m.cycleRoute() }},
	{key: "R", label: "route target", help: "what the selected route keeps the far end at", screens: on(screenMap),
		do: func(m *Model, _ string) { m.openTarget() }},
	// The tree.
	{key: "u", label: "buy upgrade", help: "buy the node under the cursor (enter too)", keys: []string{"u", "enter"}, screens: on(screenUpgrades),
		do: func(m *Model, _ string) { m.askUpgrade() }},
	// The ledger.
	{key: "b", label: "buy front", help: "buy a front through the picker", screens: on(screenLedger),
		do: func(m *Model, _ string) { m.askFront() }},
	{key: "f", label: "fund city", help: "give a city clean cash for goodwill", screens: on(screenLedger),
		do: func(m *Model, _ string) { m.askFund() }},
	// The rivals.
	{key: "d", label: "propose", help: "offer the rival a truce, tribute or a split", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.askPropose() }},
	{key: "y", label: "accept", help: "take the selected offer", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.answerOffer(true) }},
	{key: "x", label: "decline", help: "turn the selected offer down", screens: on(screenRivals),
		do: func(m *Model, _ string) { m.answerOffer(false) }},
	// Everywhere.
	{key: "b", label: "buy", help: "buy from the supplier where you stand", global: true,
		do: func(m *Model, _ string) { m.openDialog(modeBuy) }},
	{key: "s", label: "sell", help: "queue a street sale in the city shown", global: true,
		do: func(m *Model, _ string) { m.openDialog(modeSell) }},
	{key: "x", label: "cancel order", help: "cancel the order on the selected product", global: true,
		do: func(m *Model, _ string) { m.cancelSelected() }},
	{key: "l", label: "lie low", help: "lie low today: no sales, heat fades faster", global: true,
		do: func(m *Model, _ string) { m.toggleLieLow() }},
	{key: "p", label: "pay dial", help: "the pay dial: stingy, fair, generous", global: true,
		do: func(m *Model, _ string) { m.cyclePay() }},
	{key: "d", label: "launder dial", help: "the launder dial: careful, normal, greedy", global: true,
		do: func(m *Model, _ string) { m.cycleLaunder() }},
	{key: "g", label: "go to <city>", help: "go to the other city; the stock stays put", global: true,
		do: func(m *Model, _ string) { m.askTravel() }},
	{key: "r", label: "report", help: "reopen the morning report", global: true,
		do: func(m *Model, _ string) {
			if m.w.Report != nil {
				m.mode = modeReport
			}
		}},
	{key: "␣", label: "details", help: "show and hide the details", keys: []string{" "}, global: true,
		do: func(m *Model, _ string) { m.toggleDetails() }},
	{key: "?", label: "help", help: "this list", global: true,
		do: func(m *Model, _ string) { m.mode = modeHelp }},
	// The frame's keys: the title bar carries the screens, help the rest.
	{key: "enter", label: "end day", help: "end the day, after a confirmation", global: true, quiet: true,
		do: func(m *Model, _ string) { m.mode = modeConfirmEnd }},
	{key: "1-8", label: "switch screen", help: "the screens in the title bar's order", keys: []string{"1", "2", "3", "4", "5", "6", "7", "8"}, global: true, quiet: true,
		do: func(m *Model, key string) { m.switchScreen(screen(key[0] - '1')) }},
	{key: "tab", label: "next screen", help: "the next screen; shift+tab the one before", keys: []string{"tab", "shift+tab"}, global: true, quiet: true,
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
// order. Confirmations are `y <verb>` / `esc back` (any other key still
// declines); pickers and dialogs `enter <verb>` / `esc back` (q still
// closes, silently); the end of the day is the one that also takes enter
// and says so; the report, the card's outcome and help close on enter or
// esc. The keys themselves are handled by handleKey; the table is what
// the footer and the status bar say.
var modeBindings = []binding{
	{key: "↑↓", label: "pick", modes: in(modeStart, modePost, modeStrike, modeFront, modeAssign, modePropose)},
	{key: "↑↓", label: "pick", modes: in(modeBuy, modeSell, modeTarget), when: step(0)},
	{key: "↑↓", label: "pick", modes: in(modeCard), when: step(0)},
	{key: "←→", label: "dial", modes: in(modeSell), when: step(2)},
	{key: "←→", label: "city", modes: in(modeFund)},
	{key: "1-3", label: "dial", modes: in(modeSell), when: step(2)},
	{key: "1-3", label: "choose", modes: in(modeCard), when: step(0)},
	{key: "enter", label: "next", modes: in(modeBuy, modeSell, modeTarget, modePropose), when: step(0)},
	{key: "enter", label: "next", modes: in(modeSell), when: step(1)},
	{key: "enter", label: "select", modes: in(modeStart)},
	{key: "enter", label: "buy", modes: in(modeBuy), when: step(1)},
	{key: "enter", label: "buy", modes: in(modeFront)},
	{key: "enter", label: "sell", modes: in(modeSell), when: step(2)},
	{key: "enter", label: "set", modes: in(modeTarget), when: step(1)},
	{key: "enter", label: "propose", modes: in(modePropose), when: step(1)},
	{key: "enter", label: "post", modes: in(modePost)},
	{key: "enter", label: "send", modes: in(modeStrike)},
	{key: "enter", label: "assign", modes: in(modeAssign)},
	{key: "enter", label: "give", modes: in(modeFund)},
	{key: "enter", label: "decide", modes: in(modeCard), when: step(0)},
	{key: "enter", label: "new run", modes: in(modeOver)},
	{key: "c", label: "continue", modes: in(modeStart)},
	{key: "n", label: "new run", modes: in(modeStart)},
	{key: "y", label: "new run", modes: in(modeConfirmNew)},
	{key: "y", label: "fire", modes: in(modeConfirmFire)},
	{key: "y enter", label: "end day", modes: in(modeConfirmEnd)},
	{key: "y", label: "buy", modes: in(modeConfirmUpgrade)},
	{key: "y", label: "ask", modes: in(modeConfirmInvestigate)},
	{key: "y", label: "pay", modes: in(modeConfirmPayOff)},
	{key: "y", label: "go", modes: in(modeConfirmTravel)},
	{key: "q", label: "quit", modes: in(modeStart, modeOver)},
	{key: "esc", label: "back", modes: in(modeBuy, modeSell, modeTarget, modePropose, modePost, modeStrike, modeFront, modeAssign, modeFund,
		modeConfirmNew, modeConfirmFire, modeConfirmEnd, modeConfirmUpgrade, modeConfirmInvestigate, modeConfirmPayOff, modeConfirmTravel)},
	{key: "enter esc", label: "close", modes: in(modeReport, modeHelp)},
	{key: "enter esc", label: "close", modes: in(modeCard), when: step(1)},
	{key: "␣ esc", label: "close", modes: in(modeDetails)},
}

// modalStep is the page the open dialog is on: the buy, sell and target
// dialogs and the propose dialog have pages, and the card's outcome is
// its second.
func (m *Model) modalStep() int {
	switch m.mode {
	case modeBuy, modeSell:
		return m.dlg.step
	case modeTarget:
		return m.tgt.step
	case modePropose:
		return m.proposeStep
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

// keysFor is what the legend and the pane list for a screen, in table
// order: the screen's own live bindings and the global ones none of
// them shadows, the quiet ones left out.
func (m *Model) keysFor(s screen) []binding {
	var own, out []binding
	for _, b := range bindings {
		if !b.quiet && b.names(s) && b.live(m) {
			own = append(own, b)
		}
	}
	for _, b := range bindings {
		if b.quiet || !b.listed(s) || !b.live(m) {
			continue
		}
		shadowed := false
		if !b.names(s) {
			for _, o := range own {
				for _, k := range rawKeys(b) {
					if o.accepts(k) {
						shadowed = true
					}
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

// labelOf is the label as the legend draws it, the other city named.
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
		for _, s := range b.screens {
			where = append(where, screenPointer(s))
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

// helpGroups are the help modal's and the README's groups: the keys that
// work everywhere first, then each screen's own, every binding once
// under the first screen it names.
type helpGroup struct {
	title string
	keys  []binding
}

func helpGroups() []helpGroup {
	groups := []helpGroup{{title: "EVERYWHERE"}}
	for s := screen(0); s < screenCount; s++ {
		groups = append(groups, helpGroup{title: strings.ToUpper(screenOf[s])})
	}
	for _, b := range bindings {
		i := 0 // a global one is everyone's, wherever the legend lists it
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

// helpKeyW and helpLabelW are the help modal's columns: the key, the
// label, and the sentence in what is left.
const (
	helpKeyW   = 9
	helpLabelW = 14
)

// helpLines is the help modal's body: every binding, grouped, one a
// line as `key  label  help`.
func (m *Model) helpLines() []string {
	var body []string
	for i, g := range helpGroups() {
		if i > 0 {
			body = append(body, "")
		}
		body = append(body, theme.PanelTitle.Render(g.title))
		for _, b := range g.keys {
			body = append(body, theme.Key.Render(fit(b.key, helpKeyW))+"  "+theme.Subtle.Render(fit(b.label, helpLabelW))+"  "+b.help)
		}
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
// first thing a new player reads, its two keys drawn the legend's way.
// Every other key the player is told about is in the legend, the pane
// or a modal's footer.
func tutorialLine() string {
	sub := theme.Subtle.Render
	return sub("No sales queued. Press ") + theme.Key.Render("s") + sub(" to sell, ") + theme.Key.Render("n") + sub(" to end the day.")
}

// The README's key table sits between these markers; cmd/keys writes it
// there and TestReadmeMatchesKeys reads it back.
const (
	ReadmeKeysBegin = "<!-- keys:begin -->"
	ReadmeKeysEnd   = "<!-- keys:end -->"
)

// SpliceReadmeKeys replaces the table between the README's markers with
// table; it reports false when the markers are missing.
func SpliceReadmeKeys(readme, table string) (string, bool) {
	i := strings.Index(readme, ReadmeKeysBegin)
	j := strings.Index(readme, ReadmeKeysEnd)
	if i < 0 || j < i {
		return readme, false
	}
	return readme[:i+len(ReadmeKeysBegin)] + "\n" + table + readme[j:], true
}

// ReadmeKeysSection is the table as the README carries it, markers
// included: what cmd/keys writes and what the README must hold.
func ReadmeKeysSection(readme string) (string, bool) {
	i := strings.Index(readme, ReadmeKeysBegin)
	j := strings.Index(readme, ReadmeKeysEnd)
	if i < 0 || j < i {
		return "", false
	}
	return readme[i+len(ReadmeKeysBegin)+1 : j], true
}
