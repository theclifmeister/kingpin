package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The assets (#48) on the ledger and in the buy picker. The ledger's
// ASSETS block sits under STASH: the assets owned (what each does,
// its city, its upkeep, whether it stands and when the task force last
// looked) and then the ones on offer, locked ones with the distance to
// their line of peak clean cash, under the one cursor; enter on an
// offer opens the picker on its asset page. The picker's third kind,
// `asset`, is where they are bought: clean cash only. The dashboard's
// gauge marks the task force's line once a task force can form
// (heat.Sim.Ladder) and the ALERTS say the morning one is announced.

// assetRows lists the assets on offer the player does not own, cheapest
// first, locked ones included so the ladder is visible; one the police
// found (the tunnel) is gone for good and not listed.
func (m *Model) assetRows() []game.AssetOffer { return m.sess.AssetOffers() }

// assetsShown reports whether the ledger carries the ASSETS block: once
// an asset is owned, one has been lost, or the first line is within
// reach (engine.GateNear of it to go on peak clean cash), so a run that
// never gets near the cartel reads the ledger it always did.
func (m *Model) assetsShown() bool {
	w := m.w
	if len(w.Assets) > 0 || len(w.AssetsLost) > 0 {
		return true
	}
	for _, o := range m.assetRows() {
		if float64(o.UnlockCash-w.Stats.PeakClean) < float64(o.UnlockCash)*engine.GateNear {
			return true
		}
	}
	return false
}

// assetCols are the ASSETS table's columns: the asset, what it is, its
// city, its upkeep and its status; where MAIN is too narrow for the row
// whole the city goes, then the kind (the pane carries both).
var assetCols = []col{{"asset", kText, 0}, {"what", kText, 0}, {"city", kText, 0}, {"upkeep/day", kMoney, 0}, {"status", kText, 0}}

// assetKind is the one-word column for an asset's effect.
func assetKind(effect string) string {
	switch effect {
	case content.AssetSupplier:
		return "the connect"
	case content.AssetPort:
		return "the port"
	case content.AssetAirstrip:
		return "the plane"
	case content.AssetLab:
		return "the lab"
	case content.AssetTunnel:
		return "the tunnel"
	}
	return effect
}

// assetBlurb is what an asset does, for the pane, in the file's terms.
func (m *Model) assetBlurb(id string) string {
	a := m.cfg.Assets.Asset(id)
	if a == nil {
		return ""
	}
	city := m.w.CityName(a.City)
	switch a.Effect {
	case content.AssetSupplier:
		return fmt.Sprintf("The wholesaler in %s sells at %s of street, without limit, and a buy never nudges their price.", city, format.Pct(a.OwnRatio, 0))
	case content.AssetPort:
		return fmt.Sprintf("Every boat through %s carries %s and clears customs for nothing.", city, format.TimesSig(a.CapacityMul, 3))
	case content.AssetAirstrip:
		return "The plane route is open: a day's flight, the priciest fare, and a risk only the task force's watch touches."
	case content.AssetLab:
		return fmt.Sprintf("A cook in %s is %s the batch at quality %.0f, precursors at %s of the cost.", city, format.TimesSig(a.LabMul, 3), a.LabQuality, format.Pct(a.LabCostMul, 0))
	case content.AssetTunnel:
		return "The tunnel route is open: cheap, slow and nearly never seen, until it is found once."
	}
	return ""
}

// assetStatus is an asset's state for the status column: `shut, back
// in Nd` for unpaid upkeep (the front's word, #238: `idle` is the
// crew's), else standing, with the day the task force last came.
func (m *Model) assetStatus(a game.Asset) any {
	w := m.w
	if a.Frozen(w.Day + 1) {
		return styled{theme.Warning, fmt.Sprintf("shut, back in %dd", a.FrozenUntil-w.Day-1)}
	}
	if last, ok := w.Heat.LastResponse[content.TaskForce]; ok && last > 0 {
		return styled{theme.Good, fmt.Sprintf("standing · feds looked %dd ago", w.Day-last)}
	}
	return styled{theme.Good, "standing"}
}

// assetTable is the ASSETS block's rows: the assets owned, then the
// offers, each with its status.
func (m *Model) assetTable(owned []game.Asset, offers []game.AssetOffer, long bool) [][]any {
	w := m.w
	var out [][]any
	for _, a := range owned {
		out = append(out, []any{a.Name, assetKind(a.Effect), w.CityName(a.City), a.Upkeep, m.assetStatus(a)})
	}
	for _, o := range offers {
		var status any
		switch {
		case o.Locked(w):
			toGo := cash(o.UnlockCash-w.Stats.PeakClean) + " clean to go"
			if long {
				toGo = "locked · " + toGo
			}
			status = styled{theme.Subtle, toGo}
		case o.Cost > w.Player.CleanCash:
			status = styled{theme.Bad, "short " + money(o.Cost-w.Player.CleanCash) + " clean"}
		default:
			status = "open to you · " + money(o.Cost) + " clean"
		}
		out = append(out, []any{o.Name, assetKind(o.Effect), w.CityName(o.City), o.Upkeep, status})
	}
	return out
}

// assetSection is an owned asset's pane: what it does, where, what it
// costs to keep, the floor it puts under the heat, when it was bought,
// and the task force.
func (m *Model) assetSection(a game.Asset) section {
	w := m.w
	status, _ := cellText(kText, 0, m.assetStatus(a))
	st := m.assetStatus(a).(styled).st
	lines := []string{st.Render(status)}
	lines = append(lines, wrapped(theme.Subtle, m.assetBlurb(a.ID))...)
	lines = append(lines,
		row("city", w.CityName(a.City)),
		row("upkeep", money(a.Upkeep)+"/day clean"),
	)
	if o, ok := m.rules.Laundering.AssetOffer(a.ID); ok && o.HeatFloor > 0 {
		lines = append(lines, row("heat floor", fmt.Sprintf("%.0f in every city while it stands", o.HeatFloor)))
	}
	lines = append(lines, row("bought", fmt.Sprintf("day %d · %s clean", a.Bought, money(a.Cost))))
	lines = append(lines, m.taskForceLines()...)
	return section{strings.ToUpper(a.Name), lines}
}

// assetOfferSection is an asset on offer in the pane.
func (m *Model) assetOfferSection(o game.AssetOffer) section {
	w := m.w
	lines := wrapped(theme.Subtle, m.assetBlurb(o.ID))
	lines = append(lines,
		row("cost", money(o.Cost)+" clean"),
		row("upkeep", money(o.Upkeep)+"/day clean"),
		row("city", w.CityName(o.City)),
	)
	if o.HeatFloor > 0 {
		lines = append(lines, row("heat floor", fmt.Sprintf("%.0f in every city while it stands", o.HeatFloor)))
	}
	switch {
	case o.Locked(w):
		lines = append(lines, theme.Subtle.Render("locked until peak clean cash "+cash(o.UnlockCash)), theme.Subtle.Render(cash(o.UnlockCash-w.Stats.PeakClean)+" to go"))
	case o.Cost > w.Player.CleanCash:
		lines = append(lines, theme.Bad.Render("short "+money(o.Cost-w.Player.CleanCash)+" clean"))
	default:
		lines = append(lines, keyRow("b", "buy it through the picker"))
	}
	if lost := w.AssetLost(o.ID); lost != nil {
		lines = append(lines, wrapped(theme.Warning, fmt.Sprintf("The feds took it on day %d. It is for sale again at the price.", lost.Lost))...)
	}
	return section{strings.ToUpper(o.Name), lines}
}

// taskForceLines is the pane's word on the task force: forming this
// morning, the last time it came, or the line it forms at.
func (m *Model) taskForceLines() []string {
	w := m.w
	h := m.rules.Heat
	if h.TaskForceForming(w) {
		return wrapped(theme.Bad, "A task force formed this morning and comes tonight: it takes an asset with it. Lie low.")
	}
	var lines []string
	if last, ok := w.Heat.LastResponse[content.TaskForce]; ok && last > 0 {
		lines = append(lines, row("task force", fmt.Sprintf("last came day %d", last)))
	}
	for _, r := range h.ThresholdsIn(w, w.Here()) {
		if r.Level == content.TaskForce {
			lines = append(lines, row("", fmt.Sprintf("forms at heat %.0f here", r.Threshold)))
		}
	}
	return lines
}

// The picker's asset page (#48): the assets on offer, clean cash only.
var assetOfferCols = []col{{"asset", kText, 0}, {"what", kText, 0}, {"city", kText, 0}, {"cost", kMoney, 0}, {"upkeep/day", kMoney, 0}, {"floor", kInt, 0}, {"status", kText, 0}}

func (m *Model) assetOfferRows(rows []game.AssetOffer) [][]any {
	w := m.w
	var out [][]any
	for _, o := range rows {
		var status any
		switch {
		case o.Locked(w):
			status = styled{theme.Subtle, "locked at " + cash(o.UnlockCash) + " clean"}
		case o.Cost > w.Player.CleanCash:
			status = styled{theme.Bad, "short " + money(o.Cost-w.Player.CleanCash)}
		default:
			status = "open to you"
		}
		out = append(out, []any{o.Name, assetKind(o.Effect), w.CityName(o.City), o.Cost, o.Upkeep, int(o.HeatFloor), status})
	}
	return out
}

// viewAssets is the picker's asset page.
func (m *Model) viewAssets() string {
	rows := m.assetRows()
	if len(rows) == 0 {
		return m.modal("BUY AN ASSET", []string{"Nothing for sale."}, m.modalFooter())
	}
	m.front.cursor = max(0, min(m.front.cursor, len(rows)-1))
	m.modalFollow(1 + m.front.cursor) // under the header
	cols := append([]col(nil), assetOfferCols...)
	cells := m.assetOfferRows(rows)
	// Where the modal is too narrow for the row whole the floor goes,
	// then the city, then the kind: the blurb under the table carries
	// what the selected one does.
	for _, drop := range []int{5, 2, 1} {
		if tableWidth(cols, cells) <= m.modalInner() {
			break
		}
		cols = append(cols[:drop:drop], cols[drop+1:]...)
		for i := range cells {
			cells[i] = append(cells[i][:drop:drop], cells[i][drop+1:]...)
		}
	}
	body := table(cols, cells, m.front.cursor, m.modalInner())
	o := rows[m.front.cursor]
	body = append(body, "")
	body = append(body, m.inHand())
	body = append(body, m.subtle(fmt.Sprintf("%s The upkeep is clean cash, and a task force can take it.", m.assetBlurb(o.ID)))...)
	return m.modal("BUY AN ASSET", body, m.modalFooter())
}

// confirmAsset buys the asset under the cursor.
func (m *Model) confirmAsset() {
	rows := m.assetRows()
	m.mode = modePlay
	if len(rows) == 0 {
		return
	}
	o := rows[max(0, min(m.front.cursor, len(rows)-1))]
	a, err := m.sess.BuyAsset(o.ID)
	if err != nil {
		m.refuse("Can't buy: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Bought %s for %s clean. It stands from tonight, %s/day clean to keep; the feds will notice.", a.Name, money(o.Cost), money(o.Upkeep)))
}
