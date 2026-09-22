package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// dialog is the state of the buy or sell modal. A buy is from a connect
// where you are (#72), into the stash there; a sale is in the city
// shown, out of the stash there, by whoever works corners there. The
// last step of either is whether the line is for today or stands: a
// buy's `once` / `keep at` (#113, a supply contract at the quantity),
// after the quantity, with `cash` / `credit` beside it; a sale's `once`
// / `standing` (#114, a standing order at the quantity and the dial),
// after the dial. A buy opens on a connect step before the product
// where more than one connect where you stand sells you something
// (paged); with one, that connect is the buy's and the step is skipped.
// The dialog turns from one side to the other in place (#168): b and s
// on the product step (or the connect step) switch it, the product
// under the cursor kept, and where the turn changed the city the dialog
// is about (a buy is where you stand, a sale in the shown city) the
// first body line says so.
type dialog struct {
	stepper
	pick     bool
	paged    bool // the buy has a connect step before the product
	supplier int  // the connect picked, an index into connectsHere, while paged
	credit   bool // the buy goes on the connect's book
	qty      numberField
	dial     events.Dial
	repeat   repeat
	turned   bool // the dialog was turned to this side from a city other than this one (#168)
}

// page is the dialog's page for the key table (#243): the connect
// step, where there is one, is a page before the product's, so back has
// it to go to. fieldAt is the quantity on its step; field the one on
// this page.
func (d *dialog) page() int {
	if d.pick {
		return 0
	}
	if d.paged {
		return d.step + 1
	}
	return d.step
}

func (d *dialog) fieldAt(step int) *numberField {
	if step == 1 && !d.pick {
		return &d.qty
	}
	return nil
}

func (d *dialog) field() *numberField { return d.fieldAt(d.step) }

// payNames are the buy's last-step notches for how it is paid, in the
// dial convention.
var payNames = []string{"cash", "credit"}

// repeat is the last step of the buy dialog (#113) and of the sell
// dialog (#114): whether the line is for today (once) or stands (keep
// at: a supply contract at the quantity; standing: a standing order at
// the quantity and the dial). The second notch is one value under two
// names, one dialog reading it at a time.
type repeat int

const (
	repeatOnce repeat = iota
	repeatKeep
	repeatStanding = repeatKeep
)

// repeatNames and sellRepeatNames are the notches as the buy and the
// sell dialog draw them, through the dial convention (dialCells).
var (
	repeatNames     = []string{"once", "keep at"}
	sellRepeatNames = []string{"once", "standing"}
)

// stepRepeat is the key handling of a repeat step: ←→ (h, l) turn it, a
// digit picks a notch. It reports whether the key was one of those.
func stepRepeat(key string, on *repeat, n int) bool {
	switch key {
	case "left", "h":
		if *on > 0 {
			*on--
		}
	case "right", "l":
		if int(*on) < n-1 {
			*on++
		}
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if i := int(key[0] - '1'); i < n {
			*on = repeat(i)
		}
	default:
		return false
	}
	return true
}

// dialogCity is the city a buy or sell dialog is about: a buy is where
// you are, or the city a lieutenant runs for you when the market is
// turned to it (buyCity, #174); a sale is in the city the market or
// map is turned to, and where you are from any other screen.
func (m *Model) dialogCity() string {
	if m.mode == modeBuy {
		return m.buyCity()
	}
	return m.actionCity()
}

// buyCity is the city a buy is in: where you stand, or, on the market
// turned to a city a lieutenant runs for you (#174), that city, the
// buy going through them at the contract markup. The dashboard's b is
// always where you stand.
func (m *Model) buyCity() string {
	if m.screen == screenMarket {
		if city := m.shown().ID; m.w.CanBuyIn(city) {
			return city
		}
	}
	return m.w.Player.Location
}

// buyThrough is the lieutenant a buy in the dialog's city goes through
// (#174): the one running it, where it is not where you stand; nil
// where you stand.
func (m *Model) buyThrough() *game.CrewMember {
	if city := m.buyCity(); city != m.w.Player.Location {
		return m.w.Crew.Lieutenant(city)
	}
	return nil
}

// buyPointer is b refused on the market turned to a city you are not
// in and nobody runs for you (#174): where a buy there would come
// from, as the status bar says it; blank where the buy can open.
func (m *Model) buyPointer() string {
	if m.screen != screenMarket {
		return ""
	}
	city := m.shown().ID
	if m.w.CanBuyIn(city) {
		return ""
	}
	return fmt.Sprintf("Can't buy in %s: you are in %s and nobody runs it for you. Go there %s, or give a lieutenant the city %s.",
		m.w.CityName(city), m.w.CityName(m.w.Player.Location), screenPointer(screenMap), screenPointer(screenCrew))
}

// actionCity is the city a sale or a cancelled order is about: the one
// shown on the market and map screens, where you are everywhere else.
func (m *Model) actionCity() string {
	if m.screen == screenMarket || m.screen == screenMap {
		return m.shown().ID
	}
	return m.w.Player.Location
}

func (m *Model) openDialog(mode mode) {
	if m.w.Over != nil {
		return
	}
	if mode == modeBuy {
		if why := m.buyPointer(); why != "" {
			m.refuse(why)
			return
		}
	}
	if why := m.cannotOpen(mode); why != "" {
		m.refuse(why)
		return
	}
	m.dlg = dialog{qty: newNumberField("blank = max"), dial: events.DialNormal}
	if mode == modeBuy {
		m.seedBuy()
	}
	if mode == modeSell {
		city := m.actionCity()
		// Land on something you actually hold there, or that the
		// contract brings.
		if m.sellable(city, m.w.Products[m.cursor]) == 0 {
			for i, id := range m.w.Products {
				if m.sellable(city, id) > 0 {
					m.cursor = i
					break
				}
			}
		}
		m.seedSell()
	}
	m.mode = mode
}

// cannotOpen is why the buy or sell dialog cannot open on its side
// today, as the status bar and the other side's error line say it, or
// blank: a buy needs a connect where you stand that is dealing, a sale
// a day you are not lying low and something to sell in its city.
func (m *Model) cannotOpen(mode mode) string {
	if mode == modeBuy {
		if open, _ := m.openConnects(); len(open) == 0 {
			return "Can't buy: " + m.whyNobodySells() + "."
		}
		return ""
	}
	if m.w.Today.LieLow {
		return "Can't sell: you are lying low today, nothing sells."
	}
	if city := m.actionCity(); m.sellableIn(city) == 0 {
		if m.w.Stashed() == 0 {
			return "Nothing to sell: buy from the supplier first."
		}
		return fmt.Sprintf("Nothing to sell in %s: turn to the other city, or run a route into it %s.", m.w.CityName(city), screenPointer(screenMap))
	}
	return ""
}

// openConnects is the connects where you stand that are dealing today
// (#72), as indexes into connectsHere, and the one the connect step
// lands on: the cheapest for the product under the cursor.
func (m *Model) openConnects() (open []int, land int) {
	for i, sup := range m.connectsHere() {
		if m.dealing(sup) {
			open = append(open, i)
		}
	}
	if len(open) == 0 {
		return nil, 0
	}
	land = open[0]
	if best := m.w.BestSupplier(m.buyCity(), m.w.Products[m.cursor]); best != nil {
		for _, i := range open {
			if m.connectsHere()[i].ID == best.ID {
				land = i
			}
		}
	}
	return open, land
}

// seedBuy sets the buy dialog's connect (#72): none dealing, and there
// is nothing to open on (cannotOpen says so first); one, and it is the
// buy's, the product step saying what you cannot afford; more, and the
// first step picks.
func (m *Model) seedBuy() {
	open, land := m.openConnects()
	switch len(open) {
	case 0:
	case 1:
		m.dlg.supplier = open[0]
	default:
		m.dlg.paged, m.dlg.pick, m.dlg.supplier = true, true, land
	}
}

// seedSell opens the sell dialog's dial on the order's or the standing
// order's for the product under the cursor, where one stands.
func (m *Model) seedSell() {
	city := m.actionCity()
	if o, ok := m.w.Order(city, m.w.Products[m.cursor]); ok {
		m.dlg.dial = o.Dial
	} else if o, ok := m.w.YourStanding(city, m.w.Products[m.cursor]); ok {
		m.dlg.dial = o.Dial
	}
}

// switchSide is b on the sell dialog or s on the buy dialog (#168): the
// dialog turned to the other side in place, on the product step, the
// product under the cursor kept and the cart on screen. What the
// earlier side held is cleared as shift+tab clears it (the quantity,
// the repeat, the pay), a buy's connect step is re-entered only where
// it has one, and the sale's dial is seeded as opening it would seed
// it. The other side refusing to open is the error line, not a close.
// Where the turn changes the city (a buy is where you stand, a sale in
// the shown city) the first body line says so.
func (m *Model) switchSide() {
	d := &m.dlg
	to := modeSell
	if m.mode == modeSell {
		to = modeBuy
	}
	if why := m.cannotOpen(to); why != "" {
		d.err = why
		return
	}
	from := m.dialogCity()
	m.mode = to
	d.step, d.pick, d.paged, d.supplier, d.credit = 0, false, false, 0, false
	d.qty.SetValue("")
	d.qty.Blur()
	d.repeat = repeatOnce
	d.dial = events.DialNormal
	d.turned = m.dialogCity() != from
	if to == modeBuy {
		m.seedBuy()
	} else {
		m.seedSell()
	}
}

// keyDialog is the buy and sell dialogs' key handler. Back is one key
// and close is one key (#110): esc closes the dialog from any step,
// shift+tab goes back a step keeping what the earlier steps hold (the
// product stays under the cursor; the quantity is cleared on leaving
// its step and kept on coming back to it from the last step) and is
// silent on the first, tab goes forward once the step is complete (a
// product you can buy or sell, a quantity that reads) and is silent
// otherwise; enter is next until the last step, where it commits: a
// buy's `once` / `keep at` (#113, ←→ or 1-2; enter buys, or sets the
// contract), a sale's `once` / `standing` after its dial (#114; enter
// queues the order, or sets it standing). The quantity step is a
// numberField (#112): m, h, ↑↓ and pgup pgdn move the number within
// what the field can take, the stash here (and what the contract
// brings) for a sale and maxBuy for a buy.
func (m *Model) keyDialog(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.dlg
	d.err = ""
	if closes(key) {
		m.mode = modePlay
		return m, nil
	}
	switch key {
	case "shift+tab":
		return m.dialogBack()
	case "tab":
		return m.dialogForward()
	}
	if d.pick {
		// The connect step (#72): a list of the connects where you
		// stand; enter takes one that sells you something.
		n := len(m.connectsHere())
		switch key {
		case "up", "k":
			stepCursor(&d.supplier, -1, n)
		case "down", "j":
			stepCursor(&d.supplier, 1, n)
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if i := int(key[0] - '1'); i < n {
				d.supplier = i
			}
		case "enter", "right", "l":
			return m.dialogForward()
		case "s":
			m.switchSide()
		}
		return m, nil
	}
	switch d.step {
	case 0:
		switch key {
		case "b", "s":
			// The other side, in place (#168); the key of the side
			// the dialog is on does nothing.
			if (key == "b") == (m.mode == modeSell) {
				m.switchSide()
			}
		case "up", "k":
			stepCursor(&m.cursor, -1, len(m.w.Products))
		case "down", "j":
			stepCursor(&m.cursor, 1, len(m.w.Products))
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if i := int(key[0] - '1'); i < len(m.w.Products) {
				m.cursor = i
			}
		case "enter", "right", "l":
			if err := m.productErr(); err != "" {
				d.err = err
				return m, nil
			}
			return m.dialogForward()
		}
		return m, nil
	case 1:
		switch key {
		case "enter":
			d.step = 2
			d.qty.Blur()
			return m, nil
		}
		d.qty.max = m.qtyMax()
		return m, d.qty.Update(k)
	case 2:
		if m.mode == modeBuy {
			switch key {
			case "enter":
				if d.repeat == repeatKeep {
					return m.confirmKeep()
				}
				return m.confirmBuy()
			case "c":
				// The pay notch (#72): credit where the connect gives
				// it and a contract is not what is being set.
				if d.repeat == repeatOnce && m.creditOffered() {
					d.credit = !d.credit
				}
				return m, nil
			}
			stepRepeat(key, &d.repeat, len(repeatNames))
			if d.repeat == repeatKeep {
				d.credit = false
			}
			return m, nil
		}
		switch key {
		case "left", "h":
			if d.dial > events.DialQuiet {
				d.dial--
			}
		case "right", "l":
			if d.dial < events.DialAggressive {
				d.dial++
			}
		case "1":
			d.dial = events.DialQuiet
		case "2":
			d.dial = events.DialNormal
		case "3":
			d.dial = events.DialAggressive
		case "enter":
			d.step = 3
		}
	case 3:
		if key == "enter" {
			if d.repeat == repeatStanding {
				return m.confirmStanding()
			}
			return m.confirmSell()
		}
		stepRepeat(key, &d.repeat, len(sellRepeatNames))
	}
	return m, nil
}

// qtyMax is what the quantity step can take: the stash of the product
// in the dialog's city plus what the supply contract there brings in
// the morning (#113: the contract fills before the orders resolve) for
// a sale, what the supplier will sell you for a buy (maxBuy). Blank has
// always meant it; m fills it in.
func (m *Model) qtyMax() int {
	id := m.w.Products[m.cursor]
	if m.mode == modeBuy {
		return m.maxBuy(id)
	}
	return m.sellable(m.dialogCity(), id)
}

// sellable is what an order for a product in a city may be for: the
// stash there and what its supply contract will buy in the morning by
// the market sim's plan.
func (m *Model) sellable(city, id string) int {
	return m.w.Stock(city, id) + m.set.Market.Due(m.w, city, id)
}

// sellableIn is sellable over every product in a city: whether the sell
// dialog has anything to open on there.
func (m *Model) sellableIn(city string) int {
	n := 0
	for _, id := range m.w.Products {
		n += m.sellable(city, id)
	}
	return n
}

// productErr is why the product under the cursor cannot go to the
// quantity step: none of it here to sell, or none affordable to buy;
// empty when it can.
func (m *Model) productErr() string {
	id := m.w.Products[m.cursor]
	if m.mode == modeSell && m.sellable(m.dialogCity(), id) == 0 {
		return "You have none of that here."
	}
	if m.mode == modeBuy && m.maxBuy(id) == 0 && m.maxBuyBy(id, true) == 0 {
		sup := m.buySupplier(id)
		switch {
		case sup == nil:
			return "Nobody sells that here today."
		case !sup.Sells(id) || sup.Price[id] <= 0:
			return sup.Name + " does not deal in that."
		case sup.Left() == 0:
			return sup.Name + " has nothing left today."
		}
		return "Can't afford or hold any."
	}
	return ""
}

// dialogForward is tab on the buy or sell dialog: the next step once
// this one is complete, silent otherwise, and silent on the last (enter
// is what buys or sells).
func (m *Model) dialogForward() (tea.Model, tea.Cmd) {
	d := &m.dlg
	if d.pick {
		cs := m.connectsHere()
		if d.supplier < 0 || d.supplier >= len(cs) {
			return m, nil
		}
		if sup := cs[d.supplier]; !m.sellsYou(sup) {
			switch {
			case sup.Frozen(m.w.Day):
				d.err = sup.Name + " is not taking your calls."
			case sup.Locked(m.w):
				d.err = sup.Name + " will not deal with you yet."
			case sup.Left() == 0:
				d.err = sup.Name + " has nothing left today."
			default:
				d.err = "Can't afford or hold anything " + sup.Name + " sells."
			}
			return m, nil
		}
		d.pick = false
		// Land on something they sell you.
		if m.productErr() != "" {
			for i, id := range m.w.Products {
				if m.maxBuy(id) > 0 || m.maxBuyBy(id, true) > 0 {
					m.cursor = i
					break
				}
			}
		}
		return m, nil
	}
	switch d.step {
	case 0:
		if m.productErr() != "" {
			return m, nil
		}
		d.step = 1
		// A product kept by contract opens on its level, at keep at,
		// the way the target dialog opens on the target (#113); one
		// with a standing order opens on its units, at standing (#114).
		if m.mode == modeBuy {
			if c, ok := m.w.Supplied(m.dialogCity(), m.w.Products[m.cursor]); ok {
				d.qty.Set(c.Units)
				d.repeat = repeatKeep
			}
		} else if o, ok := m.w.YourStanding(m.dialogCity(), m.w.Products[m.cursor]); ok {
			d.qty.Set(o.Qty)
			d.repeat = repeatStanding
		}
		return m, d.qty.Focus()
	case 1:
		if _, err := m.parseQty(m.qtyMax()); err != nil {
			return m, nil
		}
		d.step = 2
		d.qty.Blur()
	case 2:
		// The dial is always complete; the sale has one more step.
		if m.mode == modeSell {
			d.step = 3
		}
	}
	return m, nil
}

// dialogBack is shift+tab on the buy or sell dialog: the step before,
// silent on the first.
func (m *Model) dialogBack() (tea.Model, tea.Cmd) {
	d := &m.dlg
	if d.pick {
		return m, nil
	}
	if d.step == 0 {
		if d.paged {
			d.pick = true
		}
		return m, nil
	}
	return m, d.back(d.fieldAt) // the one back rule (#243): the quantity is cleared on leaving its step, kept on coming back to it
}

func (m *Model) parseQty(maxQty int) (int, error) { return readQty(m.dlg.qty, maxQty) }

// dialogError is a game error as a dialog shows it: in the register of
// the status bar's refusals, sentence case with a full stop (`Enter a
// whole number above zero.`, `Only 3 Weed in Eastside.`).
func dialogError(err error) string { return sentence(capitalize(err.Error())) }

// readQty reads a quantity field: blank means the most allowed, and
// nothing to do where that is none.
func readQty(f numberField, maxQty int) (int, error) {
	if strings.TrimSpace(f.Value()) == "" && maxQty <= 0 {
		return 0, fmt.Errorf("nothing to do")
	}
	return f.Read(maxQty)
}

// maxBuy is the most of a product the buy dialog's connect will sell
// you today for cash (#72): what you can pay for, what the stash here
// can hold and what they have left. It is the quantity field's max;
// what their book covers instead is maxBuyBy with credit, which is
// what a blank quantity means once the pay notch is on credit.
func (m *Model) maxBuy(id string) int { return m.maxBuyBy(id, false) }

// maxBuyBy is maxBuy paid one way: cash, or the connect's credit.
func (m *Model) maxBuyBy(id string, credit bool) int {
	return m.maxBuyFrom(m.buySupplier(id), id, credit)
}

// confirmBuy is enter on the buy dialog's last step at once: the buy
// itself, from the dialog's connect, for cash or on their book. A
// quantity that does not read, or that the connect refuses, puts the
// dialog back on the quantity step with the reason.
func (m *Model) confirmBuy() (tea.Model, tea.Cmd) {
	id := m.w.Products[m.cursor]
	qty, err := m.parseQty(m.maxBuyBy(id, m.dlg.credit))
	if err != nil {
		return m.quantityAgain(err)
	}
	sup := m.buySupplier(id)
	if sup == nil {
		return m.quantityAgain(fmt.Errorf("nobody sells %s here today", m.w.ProductName(id)))
	}
	p, err := m.w.Buy(sup.ID, id, qty, m.dlg.credit, m.set.Market.BuyPressure(m.w))
	if err != nil {
		return m.quantityAgain(err)
	}
	from := sup.Name
	if p.Lieutenant != "" {
		from += " through " + p.Lieutenant
	}
	if p.Credit {
		m.say(fmt.Sprintf("Bought %d %s from %s on credit: %s on the book, due day %d.", p.Qty, m.w.ProductName(id), from, money(p.Cost), sup.DebtDue))
	} else {
		m.say(fmt.Sprintf("Bought %d %s from %s for %s.", p.Qty, m.w.ProductName(id), from, money(p.Cost)))
	}
	m.nextLine()
	return m, nil
}

// confirmKeep is enter on the buy dialog's last step at keep at (#113):
// a supply contract for the product where you stand at the quantity,
// bought each morning at the contract price. The quantity may be over
// what you can buy today: the contract fills as far as the room and
// the cash allow.
func (m *Model) confirmKeep() (tea.Model, tea.Cmd) {
	id := m.w.Products[m.cursor]
	city := m.dialogCity()
	qty, err := m.parseQty(m.maxBuy(id))
	if err != nil {
		return m.quantityAgain(err)
	}
	if err := m.w.SetSupply(city, id, qty); err != nil {
		return m.quantityAgain(err)
	}
	m.say(fmt.Sprintf("Keeping %d %s in %s: bought each morning at %s the supplier's price.", qty, m.w.ProductName(id), m.w.CityName(city), format.Times(m.set.Market.Markup(), 2)))
	m.nextLine()
	return m, nil
}

// quantityAgain is a buy or sell dialog refused on its last step: back
// on the quantity step, the reason shown, the field focused.
func (m *Model) quantityAgain(err error) (tea.Model, tea.Cmd) {
	m.dlg.err = dialogError(err)
	m.dlg.step = 1
	return m, m.dlg.qty.Focus()
}

// nextLine is the dialog after a buy or an order (#103): back on the
// product step with the cart under the table, so the next product can
// be picked without reopening; esc there closes it.
func (m *Model) nextLine() {
	m.dlg.step = 0
	m.dlg.qty.SetValue("")
	m.dlg.qty.Blur()
	m.dlg.repeat = repeatOnce
	m.dlg.credit = false
}

// confirmSell is enter on the sell dialog's last step at once: the
// order for tonight.
func (m *Model) confirmSell() (tea.Model, tea.Cmd) {
	id := m.w.Products[m.cursor]
	city := m.dialogCity()
	qty, err := m.parseQty(m.sellable(city, id))
	if err != nil {
		return m.quantityAgain(err)
	}
	if err := m.w.PlaceSell(city, id, qty, m.dlg.dial); err != nil {
		return m.quantityAgain(err)
	}
	m.say(fmt.Sprintf("Queued %d %s in %s, %s. It sells at the end of the day.", qty, m.w.ProductName(id), m.w.CityName(city), m.dlg.dial))
	m.nextLine()
	return m, nil
}

// confirmStanding is enter on the sell dialog's last step at standing
// (#114): a standing order for the product in the dialog's city at the
// quantity and the dial, sold every night until cancelled at the crew's
// cut, wherever you place no order of your own that day.
func (m *Model) confirmStanding() (tea.Model, tea.Cmd) {
	id := m.w.Products[m.cursor]
	city := m.dialogCity()
	qty, err := m.parseQty(m.sellable(city, id))
	if err != nil {
		return m.quantityAgain(err)
	}
	if err := m.w.PlaceStanding(city, id, qty, m.dlg.dial); err != nil {
		return m.quantityAgain(err)
	}
	m.say(fmt.Sprintf("Standing: %d %s in %s, %s, every night until you cancel it; the crew keep %s.", qty, m.w.ProductName(id), m.w.CityName(city), m.dlg.dial, format.Pct(m.set.Market.Cut(), 0)))
	m.nextLine()
	return m, nil
}

// estHeat is what the heat sim will charge for this order in a city, plus
// the sloppy crew premium on the units it expects to move.
func (m *Model) estHeat(city, id string, qty int, dial events.Dial) float64 {
	if m.w.Product(city, id) == nil {
		return 0
	}
	moved := min(qty, m.set.Market.Capacity(m.w, city, id, dial))
	return m.set.Heat.SaleHeat(m.w, city, id, qty, dial) + m.set.Heat.SloppyHeat(m.w, city, moved)
}

// viewDialog draws the buy or sell modal: the title, the connect step
// of a paged buy, the product table and, below it, one block a step
// reached (#275: each step's block is its own method, in the order the
// dialog reads).
func (m *Model) viewDialog() string {
	w := m.w
	d := m.dlg
	id := w.Products[m.cursor]
	city := m.dialogCity()
	p := w.Product(city, id)
	buy := m.mode == modeBuy
	var body []string
	var sup *game.Supplier
	if buy {
		sup = m.buySupplier(id)
	}
	title := "SELL · " + w.CityName(city)
	if buy {
		title = "BUY · " + w.CityName(city)
		if sup != nil && !d.pick {
			title += " · " + sup.Name
		}
		if lt := m.buyThrough(); lt != nil {
			title += " · through " + lt.Name // the buy goes through the lieutenant who runs the city (#174)
		}
	}

	// The dialog turned to this side from the other city (#168): the
	// first line says where this side is, the way the pane's NOTES say
	// where you are.
	if d.turned {
		body = append(body, theme.Subtle.Render(m.sideLine()), "")
	}

	if d.pick {
		return m.modal(title, append(body, m.connectStepRows(d, id)...), m.modalFooter())
	}

	// Step 0: product list, with the market table's Δ and spark after
	// the price (#138): a buy reads the connect's price (#72), the
	// street's and the margin over it; a sale the street price and the
	// demand your corners there serve.
	cols, rows, cursor := m.dialogRows(city, buy)
	if cursor >= 0 {
		m.modalFollow(len(body) + 1 + cursor) // under the header
	}
	body = append(body, table(cols, rows, cursor, m.modalInner())...)
	body = append(body, "")

	body = append(body, m.quantityRows(d, city, id, buy, sup)...)
	if buy && d.step >= 2 && sup != nil {
		body = append(body, m.buyTermsRows(d, city, id, sup)...)
	}
	if !buy && d.step >= 2 {
		body = append(body, m.sellDialRows(d, city, id, p)...)
	}
	if !buy && d.step >= 3 {
		body = append(body, m.sellRepeatRows(d, city, id)...)
	}

	if d.err != "" {
		body = append(body, "", theme.Bad.Render(d.err))
	}
	if cb := m.cartBlock(); cb != nil {
		if body[len(body)-1] != "" {
			body = append(body, "")
		}
		body = append(body, cb...)
	}
	return m.modal(title, body, m.modalFooter())
}

// connectStepRows is the connect step of a buy (#72): the connects where
// you stand, the price of the product under the cursor, the lot, what
// they have left today, the relationship and the credit they give.
func (m *Model) connectStepRows(d dialog, id string) []string {
	var body []string
	body = append(body, m.connectTable(id, d.supplier)...)
	body = append(body, "")
	if cs := m.connectsHere(); d.supplier >= 0 && d.supplier < len(cs) {
		body = append(body, m.connectBlurb(cs[d.supplier], id)...)
	}
	if d.err != "" {
		body = append(body, "", theme.Bad.Render(d.err))
	}
	if cb := m.cartBlock(); cb != nil {
		body = append(body, "")
		body = append(body, cb...)
	}
	return body
}

// quantityRows is step 1: quantity, the number field with what it can
// take after it; before it, the nudge to pick a product on a day with
// nothing in the cart.
func (m *Model) quantityRows(d dialog, city, id string, buy bool, sup *game.Supplier) []string {
	w := m.w
	var body []string
	if d.step >= 1 {
		d.qty.max = m.qtyMax()
		if buy {
			body = append(body, m.inHand())
		}
		body = append(body, row("quantity", d.qty.View()))
		body = append(body, m.priceRows(city, id, buy)...)
		if buy && sup != nil {
			if qty, err := m.parseQty(m.maxBuyBy(id, d.credit)); err == nil {
				cost := w.Quote(sup, id, qty, d.credit)
				style := theme.Gold
				have := w.Player.DirtyCash
				if d.credit {
					have = sup.Credit()
				}
				if cost > have {
					style = theme.Bad
				}
				total := style.Render(money(cost))
				if d.credit {
					total += "   " + theme.Subtle.Render("credit "+cash(have))
				}
				body = append(body, row("total", total))
				if qty < sup.Lot && sup.SmallLot > 1 {
					body = append(body, theme.Warning.Render(fmt.Sprintf("Under %s's lot of %d: %s a unit.", sup.Name, sup.Lot, format.TimesSig(sup.SmallLot, 2))))
				}
			}
			note := fmt.Sprintf("%s has %d left today", sup.Name, sup.Left())
			if n := m.maxBuyBy(id, true); n > 0 {
				note += fmt.Sprintf("; their book covers %d", n)
			}
			body = append(body, theme.Subtle.Render(note+"."))
		} else if due := m.set.Market.Due(w, city, id); due > 0 {
			body = append(body, theme.Subtle.Render(fmt.Sprintf("%d stashed and %d the contract brings in the morning.", w.Stock(city, id), due)))
		}
	} else if len(w.Today.Buys)+len(w.Today.Orders) == 0 {
		body = append(body, theme.Subtle.Render("Pick a product."))
	}
	return body
}

// buyTermsRows is step 2 of a buy: once or keep at (#113) and cash or
// credit (#72), in the dial convention, and what the notches mean.
func (m *Model) buyTermsRows(d dialog, city, id string, sup *game.Supplier) []string {
	w := m.w
	var body []string
	qty, _ := m.parseQty(m.maxBuyBy(id, d.credit))
	body = append(body, "", row("repeat", dialCells(repeatNames, int(d.repeat))))
	pay := dialCells(payNames, 0)
	if d.credit {
		pay = dialCells(payNames, 1)
	} else if !m.creditOffered() || d.repeat == repeatKeep {
		pay = theme.Dial(true).Render("[cash]") + "  " + theme.Subtle.Render("credit")
	}
	body = append(body, row("pay", pay))
	switch {
	case d.repeat == repeatKeep:
		body = append(body, row("contract", fmt.Sprintf("keep %d here, the shortfall bought each morning at %s (%s)", qty, price(m.set.Market.SupplyPrice(w, city, id)), format.Times(m.set.Market.Markup(), 2))))
		if c, ok := w.Supplied(city, id); ok {
			body = append(body, theme.Subtle.Render(fmt.Sprintf("Kept at %d since day %d; this replaces it.", c.Units, c.Since)))
		} else {
			body = append(body, theme.Subtle.Render("A contract buys for cash from the cheapest connect here."))
		}
	case d.credit:
		due := w.Day + sup.CreditDays
		if sup.Debt > 0 {
			due = sup.DebtDue
		}
		body = append(body, row("credit", fmt.Sprintf("%s at %s · due day %d · %s of the book left", money(w.Quote(sup, id, qty, true)), format.Times(sup.CreditRatio, 2), due, cash(sup.Credit()))))
		if sup.Debt > 0 {
			body = append(body, theme.Warning.Render(fmt.Sprintf("You owe them %s already, due day %d.", money(sup.Debt), sup.DebtDue)))
		} else {
			body = append(body, theme.Subtle.Render("Miss the day and "+temperWords(sup.Temper)+"."))
		}
	default:
		body = append(body, theme.Subtle.Render("Bought now, once. Keep at is a supply contract: the same each morning."))
	}
	return body
}

// sellDialRows is step 2 of a sale: the dial preview. The dial row is
// the dial convention: the chosen notch in brackets and the accent.
func (m *Model) sellDialRows(d dialog, city, id string, p *game.ProductMarket) []string {
	w := m.w
	var body []string
	qty, _ := m.parseQty(m.sellable(city, id))
	body = append(body, "", row("dial", dialRow(d.dial)))
	dc := m.set.Market.Dial(d.dial)
	est := min(qty, m.set.Market.Capacity(w, city, id, d.dial))
	body = append(body, row("expect", fmt.Sprintf("~%d of %d at ~%s = ~%s", est, qty, price(p.Price*dc.Price), theme.Gold.Render(money(int(float64(est)*p.Price*dc.Price))))))
	h := m.estHeat(city, id, qty, d.dial)
	body = append(body, row("heat", heatStyle(w.City(city).Heat+h*4).Render(fmt.Sprintf("+%.1f", h))+"   "+theme.Subtle.Render(dialBlurb(d.dial))))
	if w.WorkedIn(city) == 0 {
		body = append(body, theme.Bad.Render(fmt.Sprintf("You work no corner in %s: nothing will sell.", w.CityName(city))), theme.Bad.Render("Post somebody "+screenPointer(screenMap)+"."))
	}
	return body
}

// sellRepeatRows is step 3 of a sale: once or standing (#114), in the
// dial convention, and what a standing order means.
func (m *Model) sellRepeatRows(d dialog, city, id string) []string {
	w := m.w
	var body []string
	qty, _ := m.parseQty(m.sellable(city, id))
	body = append(body, "", row("repeat", dialCells(sellRepeatNames, int(d.repeat))))
	if d.repeat == repeatStanding {
		body = append(body, row("standing", fmt.Sprintf("%d at %s nightly until cancelled; the crew keep %s", qty, d.dial, format.Pct(m.set.Market.Cut(), 0))))
		if o, ok := w.YourStanding(city, id); ok {
			body = append(body, theme.Subtle.Render(fmt.Sprintf("Standing at %d %s now; this replaces it.", o.Qty, o.Dial)))
		} else {
			body = append(body, theme.Subtle.Render("An order by hand wins its day; the standing one is back the next."))
		}
	} else {
		body = append(body, theme.Subtle.Render("Tonight, once. Standing is the same order every night, at a cut."))
	}
	return body
}

// sideLine is the first body line of a dialog turned to a side in
// another city (#168): `Selling in Bayport.` / `Buying in Eastside.`.
func (m *Model) sideLine() string {
	if m.mode == modeBuy {
		return "Buying in " + m.w.CityName(m.dialogCity()) + "."
	}
	return "Selling in " + m.w.CityName(m.dialogCity()) + "."
}

// dialogRows is the product step's table (#138): a buy's `product
// price/unit street margin Δ Nd have`, a sale's `product price/unit Δ
// Nd have demand/day`, the Δ and the spark the market table's cells
// (priceFacts) and the spark sized to what the modal leaves, as the
// market sizes it. A column the modal has no room for is dropped from
// the right (`have`, then the spark), never wrapped; at 80 columns
// every one fits. The cursor is the row of the product selected.
func (m *Model) dialogRows(city string, buy bool) (cols []col, rows [][]any, cursor int) {
	w := m.w
	cols = []col{{"product", kText, 0}, {"price", kPrice, 0}}
	if buy {
		cols = append(cols, col{"street", kPrice, 0}, col{"margin", kPct, 0})
	}
	cols = append(cols, col{"Δ", kPct, 0}, col{"", kBar, 0}, col{"stash", kInt, 0})
	if !buy {
		cols = append(cols, col{"demand/day", kInt, 0})
	}
	cursor = -1
	for i, pid := range w.Products {
		pm := w.Product(city, pid)
		if pm == nil {
			continue
		}
		if i == m.cursor {
			cursor = len(rows)
		}
		f := facts(pm)
		var row []any
		if buy {
			// The connect's price and margin (#72), `-` for what they
			// do not deal in.
			f = m.connectFacts(city, pid)
			var unit any
			if f.unit > 0 {
				unit = f.unit
			}
			row = []any{pm.Name, unit, pm.Price, f.marginCell()}
		} else {
			row = []any{pm.Name, pm.Price}
		}
		row = append(row, f.deltaCell(), f.sparkCell(), w.Stock(city, pid))
		if !buy {
			row = append(row, approx{w.Demand(city, pid)})
		}
		rows = append(rows, row)
	}
	width := m.modalInner()
	for len(cols) > 3 && tableWidth(cols, rows) > width {
		cols = cols[:len(cols)-1]
		for i := range rows {
			rows[i] = rows[i][:len(cols)]
		}
	}
	sparkCol(cols, rows, max(3, min(30, width-tableWidth(cols, rows))))
	return cols, rows, cursor
}

// connectFacts is a product's price facts read against the buy
// dialog's connect (#72): their price where they sell it to you today,
// none where they do not; the market's facts outside the buy dialog.
func (m *Model) connectFacts(city, id string) priceFacts {
	pm := m.w.Product(city, id)
	if m.mode != modeBuy {
		return facts(pm)
	}
	unit := 0.0
	if sup := m.buySupplier(id); sup != nil && m.w.Available(sup, id) {
		unit = sup.Price[id]
	}
	// Through a lieutenant (#174) the buy pays the markup: the facts
	// are read against that, the connect's own price kept for the line.
	f := factsAt(pm, unit*m.w.BuyMarkup(city))
	if lt := m.buyThrough(); lt != nil && unit > 0 {
		f.base, f.markup, f.through = unit, m.w.BuyMarkup(city), lt.Name
	}
	return f
}

// priceRows is the line under the quantity on the dialogs' later steps
// and the cart's for its selected line (#138): `price      $38 · +6%
// today · range 30d $30 – $45` for a sale, `supplier   $21 · street
// $38 · margin +82%` for a buy, the pane's numbers through priceLine,
// in Subtle so the number is under the eye while the quantity is
// typed; and, while one runs, the shock or slump on a row of its own,
// named as the pane names it (`shock      ×1.40, 3 days more`).
func (m *Model) priceRows(city, id string, buy bool) []string {
	f := m.priceFacts(city, id)
	if f == nil {
		return nil
	}
	label := "price"
	if buy {
		label = "supplier"
	}
	rows := []string{theme.Subtle.Render(fit(label, dialogLabelW)) + m.priceLine(city, id, buy)}
	if l, v := f.shockRow(); l != "" {
		rows = append(rows, theme.Subtle.Render(fit(l, dialogLabelW))+v)
	}
	return rows
}

// dialogLabelW is the label column of the dialogs' rows (`quantity   `).
const dialogLabelW = 11
