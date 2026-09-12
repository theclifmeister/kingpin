package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The stash houses on the ledger (#73): the STASH table under the one
// cursor with FRONTS, LOGISTICS and ON OFFER, the picker's house step
// (b, then front or house), the move dialog (m: from, to, product,
// quantity), the guard picker (e) and the drop confirmation (x).

// houseRows lists the houses on offer that the player does not rent
// yet, in file order (a city's cheapest first), locked ones included so
// the ladder is visible.
func (m *Model) houseRows() []game.HouseOffer {
	var rows []game.HouseOffer
	for _, o := range m.cfg.Houses.Offers {
		if m.w.House(o.ID) == nil {
			rows = append(rows, game.HouseOffer{ID: o.ID, Name: o.Name, City: o.City, Corner: o.Corner, Capacity: o.Capacity, Price: o.Price, Rent: o.Rent, UnlockCash: o.UnlockCash})
		}
	}
	return rows
}

// The picker's kinds: a front, then a house.
const (
	pickFront = iota
	pickHouse
)

// pickNames are the picker's first page, in the dial convention.
var pickNames = []string{"front", "house"}

// houseOfferCols and houseOfferRows are the houses on offer in the
// picker: where, how much it holds, what it costs and keeps, the block's
// heat and risk (the raid's weight and the robbers'), and whether it is
// open to you.
var houseOfferCols = []col{{"house", kText, 0}, {"city", kText, 0}, {"holds", kInt, 0}, {"price", kMoney, 0}, {"rent/day", kMoney, 0}, {"heat", kText, 0}, {"risk", kText, 0}, {"status", kText, 0}}

func (m *Model) houseOfferRows(rows []game.HouseOffer) [][]any {
	var out [][]any
	for _, o := range rows {
		var status any
		switch {
		case o.Locked(m.w):
			status = styled{theme.Subtle, "locked at " + cash(o.UnlockCash)}
		case o.Price > m.w.Player.DirtyCash:
			status = styled{theme.Bad, "short " + money(o.Price-m.w.Player.DirtyCash)}
		default:
			status = "open to you"
		}
		heat, risk := "-", "-"
		if c := m.w.Corner(o.Corner); c != nil {
			heat, risk = times(c.Heat), times(c.Risk)
		}
		out = append(out, []any{o.Name, m.w.CityName(o.City), o.Capacity, o.Price, o.Rent, heat, risk, status})
	}
	return out
}

// keyFront is the buy picker (#73: two pages, the kind then the offer):
// esc closes from either, shift+tab leaves the offers for the kinds (the
// kind kept under the cursor), tab opens the offers for the kind under
// the cursor, enter is next on the first page and buy on the second, and
// a digit picks a row.
func (m *Model) keyFront(key string) {
	switch key {
	case "esc", "q":
		m.mode = modePlay
	case "shift+tab":
		if m.frontStep == 1 {
			m.frontStep = 0
		}
	case "tab":
		if m.frontStep == 0 {
			m.openOffers()
		}
	case "up", "k":
		if m.frontStep == 0 {
			m.frontKind = max(0, m.frontKind-1)
		} else if m.frontCursor > 0 {
			m.frontCursor--
		}
	case "down", "j":
		if m.frontStep == 0 {
			m.frontKind = min(len(pickNames)-1, m.frontKind+1)
		} else if m.frontCursor < len(m.offerCount())-1 {
			m.frontCursor++
		}
	case "enter":
		if m.frontStep == 0 {
			m.openOffers()
		} else {
			m.confirmFront()
		}
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			i := int(key[0] - '1')
			if m.frontStep == 0 {
				if i < len(pickNames) {
					m.frontKind = i
					m.openOffers()
				}
			} else if i < len(m.offerCount()) {
				m.frontCursor = i
				m.confirmFront()
			}
		}
	}
}

// offerCount is one entry an offer of the kind the picker is on, for
// the cursor's range.
func (m *Model) offerCount() []struct{} {
	if m.frontKind == pickHouse {
		return make([]struct{}, len(m.houseRows()))
	}
	return make([]struct{}, len(m.frontRows()))
}

// openOffers turns the picker to the offers of the kind under the
// cursor, or says why there are none.
func (m *Model) openOffers() {
	if len(m.offerCount()) == 0 {
		m.mode = modePlay
		if m.frontKind == pickHouse {
			m.refuse("Nothing to rent: you have every house there is.")
		} else {
			m.refuse("Nothing to buy: you own every front there is.")
		}
		return
	}
	m.frontStep, m.frontCursor = 1, 0
}

// confirmHouse takes the lease on the house under the cursor.
func (m *Model) confirmHouse() {
	rows := m.houseRows()
	m.mode = modePlay
	if len(rows) == 0 {
		return
	}
	o := rows[max(0, min(m.frontCursor, len(rows)-1))]
	h, err := m.w.BuyHouse(o)
	if err != nil {
		m.refuse("Can't rent: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Took the lease on %s for %s: holds %d, rent %s/day clean. What you buy in %s goes there first.", h.Name, money(o.Price), h.Capacity, money(o.Rent), m.w.CityName(h.City)))
}

// viewKind is the picker's first page: front or house, what each is.
func (m *Model) viewKind() string {
	m.frontKind = max(0, min(m.frontKind, len(pickNames)-1))
	rows := [][]any{
		{"front", fmt.Sprintf("washes dirty cash clean · %d on offer", len(m.frontRows()))},
		{"house", fmt.Sprintf("keeps stock off the street · %d on offer", len(m.houseRows()))},
	}
	body := table([]col{{"buy", kText, 0}, {"what", kText, 0}}, rows, m.frontKind, m.modalInner())
	body = append(body, "")
	body = append(body, m.subtle(fmt.Sprintf("Dirty cash %s. A house takes what arrives in its city first, and a raid hits one place, not the operation.", cash(m.w.Player.DirtyCash)))...)
	return m.modal("BUY", body, m.modalFooter())
}

// viewHouses is the picker's house page.
func (m *Model) viewHouses() string {
	rows := m.houseRows()
	if len(rows) == 0 {
		return m.modal("RENT A HOUSE", []string{"Nothing to rent."}, m.modalFooter())
	}
	m.frontCursor = max(0, min(m.frontCursor, len(rows)-1))
	m.modalFollow(1 + m.frontCursor) // under the header
	cols := append([]col(nil), houseOfferCols...)
	cells := m.houseOfferRows(rows)
	// Where the modal is too narrow for the row whole, the block's heat
	// and risk go, then the city: the pane carries them once the house
	// is rented, and the blurb under the table names the block.
	for _, drop := range []int{6, 5, 1} {
		if tableWidth(cols, cells) <= m.modalInner() {
			break
		}
		cols = append(cols[:drop:drop], cols[drop+1:]...)
		for i := range cells {
			cells[i] = append(cells[i][:drop:drop], cells[i][drop+1:]...)
		}
	}
	body := table(cols, cells, m.frontCursor, m.modalInner())
	o := rows[m.frontCursor]
	block := o.Corner
	if c := m.w.Corner(o.Corner); c != nil {
		block = c.Name
	}
	body = append(body, "")
	body = append(body, m.subtle(fmt.Sprintf("Dirty cash %s. %s is on %s in %s; the rent is clean cash, and unpaid %d days running the landlord throws you out.", cash(m.w.Player.DirtyCash), o.Name, block, m.w.CityName(o.City), m.set.Territory.RentDays()))...)
	return m.modal("RENT A HOUSE", body, m.modalFooter())
}

// houseCols is the ledger's STASH table: where each house is, what it
// holds of what it can, the rent, who is inside and its state.
var houseCols = []col{{"house", kText, 0}, {"city", kText, 0}, {"block", kText, 0}, {"stock", kText, 0}, {"rent/day", kMoney, 0}, {"guard", kText, 0}, {"status", kText, 0}}

// houseRow is a house's STASH row.
func (m *Model) houseRow(h game.House) []any {
	w := m.w
	block := h.Corner
	if c := w.Corner(h.Corner); c != nil {
		block = c.Name
	}
	var guard any = styled{theme.Subtle, "nobody"}
	if g := w.Crew.Member(h.Guard); g != nil {
		guard = g.Name
	}
	return []any{h.Name, w.CityName(h.City), block, fmt.Sprintf("%d/%d", h.Units(), h.Capacity), h.Rent, guard, m.houseStatus(h)}
}

// houseStatus is a house's state for the status column, in the one
// lowercase vocabulary: known (the police have it), rent unpaid 2d,
// quiet.
func (m *Model) houseStatus(h game.House) any {
	switch {
	case h.Known:
		return styled{theme.Bad, "known"}
	case h.Unpaid > 0:
		return styled{theme.Warning, fmt.Sprintf("rent unpaid %dd", h.Unpaid)}
	default:
		return styled{theme.Good, "quiet"}
	}
}

// houseSection is a house's detail in the pane: the block and what it
// means (the raid's weight, the robbers' risk), what it holds, the
// rent, the guard and the robbery odds, whether the police know it, and
// what the keys do to it.
func (m *Model) houseSection(h game.House) section {
	w := m.w
	status, _ := cellText(kText, 0, m.houseStatus(h))
	st := m.houseStatus(h).(styled).st
	lines := []string{st.Render(status)}
	if c := w.Corner(h.Corner); c != nil {
		lines = append(lines, row("block", c.Name), row("", fmt.Sprintf("heat %s · risk %s", times(c.Heat), times(c.Risk))))
	}
	lines = append(lines, row("holds", fmt.Sprintf("%d of %d", h.Units(), h.Capacity)))
	for _, id := range w.Products {
		if q := h.Stock[id]; q > 0 {
			lines = append(lines, row("", fmt.Sprintf("%d %s", q, w.ProductName(id))))
		}
	}
	rent := money(h.Rent) + "/day clean"
	if h.Unpaid > 0 {
		rent += fmt.Sprintf(" · unpaid %d of %d days", h.Unpaid, m.set.Territory.RentDays())
	}
	lines = append(lines, row("rent", rent))
	guard := theme.Subtle.Render("nobody")
	if g := w.Crew.Member(h.Guard); g != nil {
		guard = fmt.Sprintf("%s · skill %d", g.Name, g.Skill)
	}
	lines = append(lines, row("guard", guard))
	lines = append(lines, row("robbery", fmt.Sprintf("%.1f%%/day", m.set.Territory.HouseRobberyChance(w, &h)*100)))
	lines = append(lines, row("since", fmt.Sprintf("day %d · %s", h.Bought, money(h.Price))))
	if h.Known {
		lines = append(lines, wrapped(theme.Bad, "The police know this house: it is the one the raid finds. Move the stock out and drop it.")...)
	}
	if h.Units() > 0 {
		lines = append(lines, keyRow("m", "move stock"))
	}
	lines = append(lines, keyRow("e", "post a guard inside"), keyRow("x", "drop the lease"))
	return section{strings.ToUpper(h.Name), lines}
}

// ledgerOnHouse is the ledger's cursor being on a house: where m, e and
// x are the house's.
func ledgerOnHouse(m *Model) bool {
	return m.screen == screenLedger && m.ledgerSelected().kind == ledgerHouse
}

// ledgerHouse is the house under the ledger's cursor, or nil.
func (m *Model) ledgerHouseSelected() *game.House {
	sel := m.ledgerSelected()
	if sel.kind != ledgerHouse || sel.i >= len(m.w.Houses) {
		return nil
	}
	return &m.w.Houses[sel.i]
}

// The move dialog (#73): from, to, product, quantity.
type moveDialog struct {
	step    int
	city    string
	from    string // a house id, or game.Street
	to      string
	product string
	cursor  int
	qty     numberField
	err     string
}

// places lists the places in the move's city a step offers: every
// place holding something for from, every other place for to. The
// street is game.Street.
func (m *Model) places(from bool) []string {
	var out []string
	if !from || m.w.Player.StockIn(m.mv.city) > 0 {
		if !from && m.mv.from != game.Street || from {
			out = append(out, game.Street)
		}
	}
	for _, h := range m.w.HousesIn(m.mv.city) {
		if from && h.Units() == 0 || !from && h.ID == m.mv.from {
			continue
		}
		out = append(out, h.ID)
	}
	return out
}

// placeLabel names a place for the dialog.
func (m *Model) placeLabel(id string) string {
	if h := m.w.House(id); h != nil {
		return h.Name
	}
	return "the street"
}

// placeHolds is what a place holds and can hold.
func (m *Model) placeHolds(id string) (units, capacity int) {
	if h := m.w.House(id); h != nil {
		return h.Units(), h.Capacity
	}
	return m.w.Player.StockIn(m.mv.city), m.w.StreetCapacity(m.mv.city)
}

// askMove opens the move dialog for the city of the house under the
// cursor, or the one you stand in.
func (m *Model) askMove() {
	if m.w.Over != nil {
		return
	}
	city := m.w.Player.Location
	if h := m.ledgerHouseSelected(); h != nil {
		city = h.City
	}
	if len(m.w.HousesIn(city)) == 0 {
		m.refuse("Nothing to move to: no house in " + m.w.CityName(city) + ". Rent one with b.")
		return
	}
	if m.w.StockIn(city) == 0 {
		m.refuse("Nothing to move: no stock in " + m.w.CityName(city) + ".")
		return
	}
	m.mv = moveDialog{city: city, qty: newNumberField("blank = all")}
	m.mv.qty.Focus()
	m.mode = modeMove
}

// moveMax is what the quantity step can move: what the source holds of
// the product, up to the destination's room.
func (m *Model) moveMax() int {
	d := &m.mv
	have := m.w.Street(d.city, d.product)
	if h := m.w.House(d.from); h != nil {
		have = h.Stock[d.product]
	}
	if h := m.w.House(d.to); h != nil {
		have = min(have, h.Room())
	}
	return have
}

// moveProducts are the products the source holds.
func (m *Model) moveProducts() []string {
	var out []string
	for _, id := range m.w.Products {
		q := m.w.Street(m.mv.city, id)
		if h := m.w.House(m.mv.from); h != nil {
			q = h.Stock[id]
		}
		if q > 0 {
			out = append(out, id)
		}
	}
	return out
}

// moveRows is the list the dialog's current step picks from.
func (m *Model) moveRows() int {
	switch m.mv.step {
	case 0:
		return len(m.places(true))
	case 1:
		return len(m.places(false))
	case 2:
		return len(m.moveProducts())
	}
	return 0
}

func (m *Model) keyMove(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.mv
	d.err = ""
	switch key {
	case "esc":
		m.mode = modePlay
		return m, nil
	case "shift+tab":
		// Back keeps what the earlier step chose under the cursor (#110).
		if d.step > 0 {
			d.step--
			d.cursor = m.moveChosen()
			d.qty.SetValue("")
		}
		return m, nil
	case "tab":
		if d.step < 3 {
			m.moveNext()
		}
		return m, nil
	case "enter":
		if d.step < 3 {
			m.moveNext()
		} else {
			m.confirmMove()
		}
		return m, nil
	}
	if d.step < 3 {
		switch key {
		case "up", "k":
			if d.cursor > 0 {
				d.cursor--
			}
		case "down", "j":
			if d.cursor < m.moveRows()-1 {
				d.cursor++
			}
		default:
			if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
				if i := int(key[0] - '1'); i < m.moveRows() {
					d.cursor = i
					m.moveNext()
				}
			}
		}
		return m, nil
	}
	d.qty.max = m.moveMax()
	return m, d.qty.Update(k)
}

// moveChosen is the index of what the current step chose before, for
// the cursor on the way back: the source, the destination or the
// product.
func (m *Model) moveChosen() int {
	d := &m.mv
	var list []string
	var want string
	switch d.step {
	case 0:
		list, want = m.places(true), d.from
	case 1:
		list, want = m.places(false), d.to
	case 2:
		list, want = m.moveProducts(), d.product
	}
	for i, id := range list {
		if id == want {
			return i
		}
	}
	return 0
}

// moveNext takes the row under the cursor and opens the next step.
func (m *Model) moveNext() {
	d := &m.mv
	switch d.step {
	case 0:
		places := m.places(true)
		if len(places) == 0 {
			return
		}
		d.from = places[max(0, min(d.cursor, len(places)-1))]
	case 1:
		places := m.places(false)
		if len(places) == 0 {
			return
		}
		d.to = places[max(0, min(d.cursor, len(places)-1))]
	case 2:
		ids := m.moveProducts()
		if len(ids) == 0 {
			return
		}
		d.product = ids[max(0, min(d.cursor, len(ids)-1))]
	}
	d.step++
	d.cursor = 0
}

func (m *Model) confirmMove() {
	d := &m.mv
	qty, err := parseQtyInput(d.qty.Value(), m.moveMax())
	if err != nil {
		d.err = dialogError(err)
		return
	}
	n, err := m.w.Move(d.city, d.from, d.to, d.product, qty)
	if err != nil {
		d.err = dialogError(err)
		return
	}
	m.mode = modePlay
	heat := m.set.Heat.MoveHeat(m.w, d.city, d.product, n)
	m.say(fmt.Sprintf("Moved %d %s from %s to %s. The drive is +%.1f heat tonight.", n, m.w.ProductName(d.product), m.placeLabel(d.from), m.placeLabel(d.to), heat))
}

func (m *Model) viewMove() string {
	d := &m.mv
	w := m.w
	title := "MOVE STOCK · " + w.CityName(d.city)
	var body []string
	placeRows := func(ids []string) [][]any {
		var rows [][]any
		for _, id := range ids {
			units, capacity := m.placeHolds(id)
			rows = append(rows, []any{m.placeLabel(id), fmt.Sprintf("%d/%d", units, capacity)})
		}
		return rows
	}
	switch d.step {
	case 0, 1:
		ids := m.places(d.step == 0)
		d.cursor = max(0, min(d.cursor, max(0, len(ids)-1)))
		if len(ids) == 0 {
			body = []string{theme.Subtle.Render("Nowhere to move it.")}
			break
		}
		m.modalFollow(1 + d.cursor)
		body = table([]col{{"place", kText, 0}, {"holds", kText, 0}}, placeRows(ids), d.cursor, m.modalInner())
		what := "Move from where?"
		if d.step == 1 {
			what = "From " + m.placeLabel(d.from) + " to where?"
		}
		body = append(body, "", theme.Subtle.Render(what))
	case 2:
		ids := m.moveProducts()
		d.cursor = max(0, min(d.cursor, max(0, len(ids)-1)))
		var rows [][]any
		for _, id := range ids {
			q := w.Street(d.city, id)
			if h := w.House(d.from); h != nil {
				q = h.Stock[id]
			}
			rows = append(rows, []any{w.ProductName(id), q})
		}
		m.modalFollow(1 + d.cursor)
		body = table([]col{{"product", kText, 0}, {"have", kInt, 0}}, rows, d.cursor, m.modalInner())
		body = append(body, "", theme.Subtle.Render(fmt.Sprintf("From %s to %s: what?", m.placeLabel(d.from), m.placeLabel(d.to))))
	default:
		qty := d.qty
		qty.max = m.moveMax()
		_, room := m.placeHolds(d.to)
		toUnits, _ := m.placeHolds(d.to)
		body = []string{
			fmt.Sprintf("%s → %s: %s", m.placeLabel(d.from), m.placeLabel(d.to), w.ProductName(d.product)),
			"",
			"Quantity  " + qty.View(),
			"",
		}
		body = append(body, m.subtle(fmt.Sprintf("%s holds %d of %d. A move is free and instant; the units moved are exposure tonight, at %s of a unit sold.", m.placeLabel(d.to), toUnits, room, times(m.cfg.Houses.Houses.MoveHeat)))...)
		if n, err := parseQtyInput(d.qty.Value(), m.moveMax()); err == nil && n > 0 {
			body = append(body, theme.Subtle.Render(fmt.Sprintf("Heat      +%.1f tonight for %d units", m.set.Heat.MoveHeat(w, d.city, d.product, n), n)))
		}
	}
	if d.err != "" {
		body = append(body, "", theme.Bad.Render(d.err))
	}
	return m.modal(title, body, m.modalFooter())
}

// The guard picker (#73): an enforcer for the house under the cursor,
// or nobody.
func (m *Model) guardRows() []game.CrewMember {
	rows := []game.CrewMember{{ID: 0, Name: "Nobody", Role: "enforcer"}}
	for _, c := range m.w.Crew.Members {
		if c.Role == "enforcer" {
			rows = append(rows, c)
		}
	}
	return rows
}

func (m *Model) askGuard() {
	h := m.ledgerHouseSelected()
	if h == nil || m.w.Over != nil {
		return
	}
	if len(m.guardRows()) == 1 {
		m.refuse("Nothing to post: no enforcers. Hire one " + screenPointer(screenCrew) + ".")
		return
	}
	m.guardCursor = 0
	m.mode = modeGuard
}

func (m *Model) confirmGuard() {
	h := m.ledgerHouseSelected()
	rows := m.guardRows()
	m.mode = modePlay
	if h == nil || len(rows) == 0 {
		return
	}
	who := rows[max(0, min(m.guardCursor, len(rows)-1))]
	if err := m.w.Guard(h.ID, who.ID); err != nil {
		m.refuse("Can't post: " + err.Error())
		return
	}
	if who.ID == 0 {
		m.say("Nobody is guarding " + h.Name + " now.")
		return
	}
	m.say(fmt.Sprintf("%s is inside %s: robbery %.1f%%/day.", who.Name, h.Name, m.set.Territory.HouseRobberyChance(m.w, h)*100))
}

func (m *Model) keyGuard(key string) {
	switch key {
	case "esc", "q":
		m.mode = modePlay
	case "up", "k":
		if m.guardCursor > 0 {
			m.guardCursor--
		}
	case "down", "j":
		if m.guardCursor < len(m.guardRows())-1 {
			m.guardCursor++
		}
	case "enter":
		m.confirmGuard()
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			if i := int(key[0] - '1'); i < len(m.guardRows()) {
				m.guardCursor = i
				m.confirmGuard()
			}
		}
	}
}

func (m *Model) viewGuard() string {
	h := m.ledgerHouseSelected()
	rows := m.guardRows()
	if h == nil {
		return m.modal("GUARD", []string{"Nobody to post."}, m.modalFooter())
	}
	m.guardCursor = max(0, min(m.guardCursor, len(rows)-1))
	var cells [][]any
	for _, r := range rows {
		var where any = styled{theme.Subtle, "unposted"}
		var skill any = r.Skill
		switch {
		case r.ID == 0:
			where, skill = styled{theme.Subtle, "takes the guard off"}, nil
		case h.Guard == r.ID:
			where = styled{theme.Good, "already here"}
		case m.w.GuardOf(r.ID) != nil:
			where = styled{theme.Warning, "in " + m.w.GuardOf(r.ID).Name + ", will move"}
		case m.w.PostOf(r.ID) != nil:
			where = styled{theme.Warning, "on " + m.w.PostOf(r.ID).Name + ", will move"}
		}
		cells = append(cells, []any{r.Name, skill, where})
	}
	m.modalFollow(1 + m.guardCursor)
	body := table([]col{{"name", kText, 0}, {"skill", kInt, 0}, {"where", kText, 0}}, cells, m.guardCursor, m.modalInner())
	body = append(body, "")
	body = append(body, m.subtle(fmt.Sprintf("Who should guard %s? One enforcer, one job: the house or a corner.", h.Name))...)
	return m.modal("GUARD "+strings.ToUpper(h.Name), body, m.modalFooter())
}

// askDrop asks before walking away from the house under the cursor.
func (m *Model) askDrop() {
	if m.ledgerHouseSelected() == nil || m.w.Over != nil {
		return
	}
	m.mode = modeConfirmDrop
}

func (m *Model) confirmDrop() {
	h := m.ledgerHouseSelected()
	m.mode = modePlay
	if h == nil {
		return
	}
	units := h.Units()
	gone, err := m.w.Drop(h.ID)
	if err != nil {
		m.refuse("Can't drop: " + err.Error())
		return
	}
	if units > 0 {
		m.say(fmt.Sprintf("Dropped %s: %s went with it.", gone.Name, plural(units, "unit")))
		return
	}
	m.say("Dropped " + gone.Name + ".")
}

func (m *Model) dropConfirm() string {
	h := m.ledgerHouseSelected()
	if h == nil {
		return m.modal("DROP?", []string{"Nothing to drop."}, m.modalFooter())
	}
	body := []string{fmt.Sprintf("Walk away from %s?", h.Name)}
	if h.Units() > 0 {
		body = append(body, fmt.Sprintf("The %s in it go with it. Move them out first.", plural(h.Units(), "unit")))
	} else {
		body = append(body, "It is empty. The price is not refunded.")
	}
	return m.modal("DROP "+strings.ToUpper(h.Name)+"?", body, m.modalFooter())
}

// subtle wraps a modal's sentence to its width in Subtle, so nothing is
// cut.
func (m *Model) subtle(s string) []string {
	var out []string
	for _, l := range wrap(s, m.modalInner()) {
		out = append(out, theme.Subtle.Render(l))
	}
	return out
}

// stashedLine is the dashboard's street fact where you have houses in
// the city: what is on the street of what it holds, and what is in the
// houses (#73); short is the 80-column form, which leaves the corners
// room on the same line.
func (m *Model) stashedLine(city string, short bool) string {
	w := m.w
	houses := w.HousesIn(city)
	if len(houses) == 0 {
		return fmt.Sprintf("stash %d/%d", w.StockIn(city), w.Capacity(city))
	}
	housed := 0
	for _, h := range houses {
		housed += h.Units()
	}
	if short {
		return fmt.Sprintf("carrying %d/%d · %d in %s", w.Player.StockIn(city), w.StreetCapacity(city), housed, plural(len(houses), "house"))
	}
	return fmt.Sprintf("carrying %d/%d · stashed %d in %s", w.Player.StockIn(city), w.StreetCapacity(city), housed, plural(len(houses), "house"))
}

// houseAlerts are the houses the police know about: the one the raid
// finds, until it is dropped.
func (m *Model) houseAlerts() []alert {
	var out []alert
	for _, h := range m.w.Houses {
		if h.Known {
			out = append(out, alert{
				text: theme.Bad.Render(fmt.Sprintf("The police know about %s: move the stock out and drop it %s.", h.Name, screenPointer(screenLedger))),
				why:  "the police know about " + h.Name,
				key:  "known " + h.ID,
			})
		}
	}
	return out
}
