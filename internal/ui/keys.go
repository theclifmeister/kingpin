package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/game"
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
// there. The two tables are in bindings.go, the README's sections they
// generate in readme.go (#275).

// binding is one key and what it does.
type binding struct {
	key     string               // as the pane draws it: `↑↓`, `[ ]`, `␣`, `pgup pgdn`, `r`
	label   string               // one or two lowercase words, the same everywhere it appears; `<city>` is the other city
	help    string               // one sentence for help and the README
	keys    []string             // the tea.KeyMsg strings it answers to; the key alone when nil
	screens []screen             // the screens whose pane lists it; nil lists it nowhere
	modes   []mode               // the modals whose footer lists it (modeBindings)
	global  bool                 // works on every screen, listed or not; a screen's own binding for the same key wins there
	dialogs bool                 // listed for whatever dialog is open (#243: openPaged), so no mode list
	quiet   bool                 // in help and the README only: the frame's keys, which the title bar and help carry
	when    func(*Model) bool    // live only while this holds; nil is always
	listed  func(*Model) bool    // listed in the pane only while this holds, live or not; nil lists it wherever it is live
	off     func(*Model) string  // the refusal while the key is the screen's own but not live (#460): said, never silent, never a global's
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

func on(ss ...screen) []screen { return ss }
func in(ms ...mode) []mode     { return ms }

// everywhere lists a binding on every screen: `n end day`, `? help`
// and, under paneMinWidth, `␣ more`.
var everywhere = on(screenDashboard, screenMarket, screenJournal, screenCrew, screenMap, screenUpgrades, screenLedger, screenRivals, screenIntel)

// listScreens are the screens whose cursor is the plain up-and-down
// one: the map walks two dimensions and lists its own; the tree's
// branch is a list, and its arrows turn the branch (#120).
var listScreens = on(screenDashboard, screenMarket, screenJournal, screenCrew, screenUpgrades, screenLedger, screenRivals, screenIntel)

// hasChemist is a chemist being on the payroll (#47): the cook is
// listed only then, and live on the product table either way, where it
// says to hire one (#460).
func hasChemist(m *Model) bool { return m.w.Crew.Chemist() != nil }

// onBuyers is the market's cursor being on the buyers under the table.
func onBuyers(m *Model) bool { return m.screen == screenMarket && m.onBuyers }

// onProducts is the market's cursor being on the product table (#239):
// where the cut and the cook act, and are listed (the cook with a
// chemist on the payroll, hasChemist).
func onProducts(m *Model) bool { return m.screen == screenMarket && !m.onBuyers && !m.onSuppliers }

// marketRegion is where the market's cursor is, in prose: the product
// table, the buyers or the connects.
func (m *Model) marketRegion() string {
	switch {
	case m.onBuyers:
		return "the buyers"
	case m.onSuppliers:
		return "the connects"
	}
	return "the product table"
}

// offBuyers and offProducts are the market's refusals for a region's
// key pressed on another region (#460): what to pick, and where the
// cursor is, rather than nothing.
func offBuyers(verb string) func(*Model) string {
	return func(m *Model) string {
		if len(m.buyerRows()) == 0 {
			return fmt.Sprintf("Nobody to %s in %s: no buyer is asking.", verb, m.shown().Name)
		}
		return fmt.Sprintf("Pick a buyer to %s: the cursor is on %s.", verb, m.marketRegion())
	}
}

func offProducts(verb string) func(*Model) string {
	return func(m *Model) string {
		return fmt.Sprintf("Pick a product to %s: the cursor is on %s.", verb, m.marketRegion())
	}
}

// offHouse is the ledger's x off a house row (#469, after #460): what
// to pick, where x fell through to the global cancel order and acted on
// the market's cursor, which the ledger does not show.
func offHouse(m *Model) string {
	if len(m.w.Houses) == 0 {
		return "No house to drop: you rent none."
	}
	return "Pick a house to drop: x on the ledger drops the house under the cursor."
}

// offAlerts is the dashboard's [ ] with no alert to pick (#469, after
// #460): it said nothing and turned the market's and the map's city,
// which the dashboard does not show.
func offAlerts(m *Model) string {
	if m.onPolice && len(m.sess.Alerts()) > 0 {
		return "Pick an alert from the product table: the arrows are on the police panels."
	}
	return "No alert to pick: ALERTS is empty. [ ] turns the city on the market and the map."
}

// step is the dialog open being on its nth page.
func step(n int) func(*Model) bool { return func(m *Model) bool { return m.modalStep() == n } }

// cardOf is a dilemma card open on its choices with n of them: the
// footer's `1-2` or `1-3` is the card's own count (#426).
func cardOf(n int) func(*Model) bool {
	return func(m *Model) bool {
		c := m.w.Dilemmas.Pending
		return m.modalStep() == 0 && !m.cardDone && c != nil && len(c.Choices) == n
	}
}

// pastFirstStep is the dialog open being on a page past its first: where
// shift+tab has a page to go back to.
func pastFirstStep(m *Model) bool { return m.modalStep() > 0 }

// numberStep is the open dialog being on a number field (#112, #243:
// what its state says through paged). The field's shortcuts are listed
// there and nowhere else.
func numberStep(m *Model) bool {
	d := m.openPaged()
	return d != nil && d.field() != nil && m.mode != modeNewRun // the seed is no quantity: no max, no half (#473)
}

// newRunNext is the new-run dialog (#50) having a page after this one:
// enter goes forward. newRunStart is its last page, or the daily's row
// on the first: enter starts the run.
func newRunNext(m *Model) bool {
	return m.mode == modeNewRun && !newRunStart(m)
}

func newRunStart(m *Model) bool {
	if m.mode != modeNewRun {
		return false
	}
	return m.nr.step == m.lastStep() || (m.nr.step == 0 && m.nr.cursor == m.dailyRow())
}

// moveList is the move dialog (#73) being on a list page: from, to or
// the product; its fourth page is the quantity.
func moveList(m *Model) bool { return m.modalStep() < 3 }

// mapOnRoutes is the map's routes cursor being on an edge (#42): where
// the checkpoint is for sale.
func mapOnRoutes(m *Model) bool {
	return m.screen == screenMap && m.onRoutes && m.selectedRoute() != nil
}

// spyOnFactions and spyOnCrew are the spy dialog's two pages (#45).
func spyOnFactions(m *Model) bool { return m.spy.step == 0 }
func spyOnCrew(m *Model) bool     { return m.spy.step == 1 }

// mapOnRivalCorner is the map's cursor being on a corner a faction
// holds (#45): where `i` has a subject to jump to.
func mapOnRivalCorner(m *Model) bool {
	c := m.mapSelected()
	return m.screen == screenMap && !m.onRoutes && c != nil && c.Owner == game.OwnerRival
}

// fundLast and fundNext are the fund dialog's last page and the page
// before it: the goodwill page is the last unless a campaign is open
// (#193), when the campaign page is.
func fundLast(m *Model) bool { return !m.campaignOpen() || m.fnd.step == 1 }
func fundNext(m *Model) bool { return !fundLast(m) }

// frontBuy and houseRent are the buy picker's second page, on the
// fronts and on the houses (#73).
func frontBuy(m *Model) bool { return m.front.step == 1 && m.front.kind == pickFront }

func houseRent(m *Model) bool { return m.front.step == 1 && m.front.kind == pickHouse }

// assetBuy is the picker's second page on the assets (#48).
func assetBuy(m *Model) bool { return m.front.step == 1 && m.front.kind == pickAsset }

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

// offPolice is the dashboard's arrows being off the police (#355): the
// ambitions' key is listed then, and gives its line to POLICE, which
// the pane holds whole at 100x30, while they are on it (it works
// either way).
func offPolice(m *Model) bool { return !m.onPolice }

// modalStep is the page the open dialog is on, as its state says
// (#243); the card's outcome is its second page.
func (m *Model) modalStep() int {
	if d := m.openPaged(); d != nil {
		return d.page()
	}
	if m.mode == modeCard && m.cardDone {
		return 1
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
		if b.quiet || !b.names(s) || !b.live(m) || (b.listed != nil && !b.listed(m)) {
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
		listed := b.dialogs && m.openPaged() != nil
		for _, x := range b.modes {
			listed = listed || x == md
		}
		if listed && b.live(m) { // the mode first: a predicate may read the run, and the start menu has none
			out = append(out, b)
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

// labelOf is the label as the pane draws it, the other city named and
// the confirmation's verb read off its payload (#242).
func (m *Model) labelOf(b binding) string {
	if strings.Contains(b.label, "<city>") && m.w != nil {
		return strings.ReplaceAll(b.label, "<city>", m.w.CityName(m.travelTo()))
	}
	if b.label == "<verb>" {
		return m.cfm.verb
	}
	return b.label
}

// lookup is the binding a key pressed on the current screen goes to: the
// screen's own first, then a global one. It also reports whether the
// screen has a binding for the key at all: one of its own that is not
// live now (the market's buyer keys with the cursor on the table) takes
// the key, so it is not pointed elsewhere; one with an off refusal says
// it (#460) and is never let through to a global that means something
// else (the market's d and the launder dial); one without falls to a
// global that takes the key, or to nothing.
func (m *Model) lookup(key string) (b binding, found, own bool) {
	refusal := ""
	for _, x := range bindings {
		if x.accepts(key) && x.names(m.screen) {
			own = true
			if x.live(m) {
				return x, true, true
			}
			if x.off != nil && refusal == "" {
				refusal = x.off(m)
			}
		}
	}
	// A key with a refusal is the screen's whatever the cursor is on
	// (#460): the market's d never turns the launder dial off a buyer.
	if refusal != "" {
		return binding{do: func(m *Model, _ string) { m.refuse(refusal) }}, true, true
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
	return fmt.Sprintf("on the %s screen (%d)", screens[s].word, int(s)+1)
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
		groups = append(groups, helpGroup{title: strings.ToUpper(screens[s].word)})
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
	{"till", "the float in hand: the wash and the road spend only over it"},
	{"target", "what a route keeps the far end at: units or days of demand"},
	{"heat", "a city's police eye, 0-100: sales raise it, days fade it"},
	{"patrol", "the first rung: caps what sells for a few days, files nothing"},
	{"sting", "a rung: takes some stock and cash, a page if you sold"},
	{"raid", "a big rung: much of the stock and cash, two pages if you sold"},
	{"arrest", "the top rung: the run ends in a cell"},
	{"file", "the DA's pages: a bust on a day you sold adds; enough indicts"},
	{"pressure", "a city's mood, 0-100: lowers police lines, tightens patrols"},
	{"goodwill", "bought with clean cash (f on the ledger): wears pressure down"},
	{"cover", "what your fronts explain of the dirty pile: it draws no heat"},
	{"estimate", "a forecast, such as a cop's word: shown with how sure it is"},
	{"drift", "a held corner nobody works goes back to the street in days"},
	{"undercut", "sell cheap on a rival corner next door: they lose, no heat"},
	{"keep at", "a supply contract: the stash bought back to a level daily"},
	{"through", "a buy into a city a lieutenant runs for you, at a markup"},
	{"standing", "a sell order that stands nightly until cancelled, at a cut"},
	{"connect", "who sells you product: a price, a lot, a temper, a rel"},
	{"credit", "a connect's book: take now, pay in days, or they answer"},
	{"unlock", "a line crossed: a product, front, connect or role opens"},
	{"house", "a rented stash off the street: rent in clean; a raid hits one"},
	{"deed", "the block a corner is on, bought clean: rent; the DA asks"},
	{"pane", "the details beside MAIN from 100 columns, always open"},
	{"strip", "the pane's one line under 100 columns; ␣ opens it over MAIN"},
	{"tier", "the stage a run is in, shown once: Corner to Cartel"},
	{"scout", "a paid look at the rival's books: a snapshot that goes stale"},
	{"boost", "the enforcers rob a rival corner's till, not the corner"},
	{"scene", "a short animation on a morning that matters; any key skips it"},
	{"world", "the weather: an incident that lands on the world, not on you"},
	{"quality", "a lot's grade, 0-100: the price pays it, corners remember it"},
	{"cut", "add units at nothing: more today, fewer customers tomorrow"},
	{"cook", "a chemist's batch of meth or designer, from precursors"},
	{"repeat", "the share of a corner's customers who come back"},
	{"overdose", "bad hard product on your corner: pressure and news, no page"},
	{"jailed", "in a cell after a bust, working nothing; bail is clean cash"},
	{"kin", "a cousin, partner or friend on the payroll: they remember"},
	{"driver", "rides a route's shipments and cuts the risk; seized, jailed"},
	{"trait", "what a veteran showed after their days of service; some bad"},
	{"captain", "a trusted veteran who looks after a city's crew each night"},
	{"lieutenant", "runs a city for a cut: sells it, stocks it; l on the crew"},
	{"temper", "a lieutenant's way: violent, greedy, careful or steady"},
	{"asset", "the supply side bought clean: a connect, port, plane, lab"},
	{"feds", "the task force above the raid: a day's notice, takes an asset"},
	{"intel", "what you know, with how sure: seen, bought, sent out, or fed"},
	{"spy", "a crew member under with a faction: reports, sells nothing"},
	{"ending", "how a run ends: nine ways, each a summary and a score"},
	{"score", "the offshore account over one plus the bodies; days shown"},
	{"quiet day", "all heat under {retire_heat}; no strike, push, bust or buyer's order"},
	{"run out", "a faction with no corner left: the rivals screen counts it"},
	{"absorbed", "run out long enough: it joins the faction that took its last"},
	{"scattered", "run out too long, or broke: it stands down, nobody's"},
	{"gone", "absorbed, scattered or leaderless: a crew down for the crown"},
	{"walk away", "retire on the account, vanish, or take the crown: asked twice"},
	{"reign", "the city yours: every crew gone or bowing, most of home held"},
	{"favour", "a bought chief owes you one; call it in, no raid"},
	{"war", "enforcers hit one faction every night until it folds"},
	{"character", "a start and nothing more: what is on the world on day 0"},
	{"daily", "the date's seed, the default character; the first go scores"},
	{"profile", "the runs, the unlocks and the dailies; a second file, no sim"},
	{"preset", "the routine's dials in one bundle: reviewed, then applied"},
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
	// A word that names a line the file sets reads it (#472: the quiet
	// day's heat is laundering.toml's retire_heat).
	lines := strings.NewReplacer("{retire_heat}", fmt.Sprintf("%.0f", m.cfg.Laundering.Offshore.RetireHeat))
	for _, w := range words {
		// The word takes the key column and one of the two spaces after
		// it, so `lieutenant`, ten, is never cut (#463) and no line
		// grows.
		body = append(body, theme.Key.Render(fit(w[0], helpKeyW+1))+" "+lines.Replace(w[1]))
	}
	return body
}

// arrowsListed is whether `↑↓ pick` is listed on the screen (#462): on
// the rivals screen the arrows walk the offers alone ([ ] turn the
// faction), so they are listed only where there are offers to walk.
func arrowsListed(m *Model) bool { return m.screen != screenRivals || len(m.w.Offers) > 1 }
