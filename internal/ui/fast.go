package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Fast-forward (#116): once the contracts, the standing orders and the
// route targets run the day, F ends days until something needs you. The
// confirmation takes a cap (a numberField, fastDays by default, fastDaysMax
// at most), and fastForward then ends days in a loop on the same
// goroutine, inside the one Update (no timer, no command: Init still
// starts nothing), each an ordinary endDay (the same clock, the same
// save, the same dice), and stops before the report of the first day on
// which something fires: the run over, a stage entered (#149: the modal
// opens before the card and the report), a card dealt (it is answered
// before the report, as always), an event of the kinds stopEvent names,
// an alert the dashboard did not carry the morning before (alerts: a
// contract due, heat over the patrol line, somebody talking, dirty cash
// under the float, wages short), or the cap. The report of that day
// opens with `Stopped after 3 days: contract due today.`, the first
// reason that fired, and keeps the line until the next day ends, so r
// reopens it as it was; the status bar says `Ran 3 days.`; the journal
// has every day's headlines, and the title bar's unread count grows.

const (
	fastDays    = 7  // the cap the dialog opens on
	fastDaysMax = 30 // the most one F runs
)

// fastDialog is the state of the fast-forward confirmation: the cap and
// the error under it.
type fastDialog struct {
	days numberField
	err  string
}

// askFast opens the confirmation on the default cap.
func (m *Model) askFast() {
	if m.w.Over != nil {
		return
	}
	m.fst = fastDialog{days: newNumberField(fmt.Sprintf("blank = %d", fastDays))}
	m.fst.days.max = fastDaysMax
	m.fst.days.Focus()
	m.mode = modeConfirmFast
}

// fastCap is the cap the field reads: fastDays for a blank, an error for
// a number that does not read or is over fastDaysMax.
func (m *Model) fastCap() (int, error) {
	s := strings.TrimSpace(m.fst.days.Value())
	if s == "" {
		return fastDays, nil
	}
	n, ok := m.fst.days.Number()
	if !ok || n <= 0 {
		return 0, fmt.Errorf("enter a whole number above zero")
	}
	if n > fastDaysMax {
		return 0, fmt.Errorf("up to %d days at a time", fastDaysMax)
	}
	return n, nil
}

// keyFast is the confirmation: y or enter runs the days, esc closes, and
// the rest goes to the number field (digits, backspace and the field's
// shortcuts; a letter never lands in it).
func (m *Model) keyFast(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	m.fst.err = ""
	switch key {
	case "esc", "q":
		m.mode = modePlay
		return m, nil
	case "y", "Y", "enter":
		m.confirmFast()
		return m, nil
	}
	m.fst.days.max = fastDaysMax
	return m, m.fst.days.Update(k)
}

// confirmFast runs the days the field reads, or shows why it cannot.
func (m *Model) confirmFast() {
	n, err := m.fastCap()
	if err != nil {
		m.fst.err = dialogError(err)
		return
	}
	m.fastForward(n)
}

// viewFast is the confirmation: the sentence with the cap the field
// reads, the field, and what stops the run.
func (m *Model) viewFast() string {
	n, err := m.fastCap()
	if err != nil {
		n = fastDays
	}
	days := m.fst.days
	days.max = fastDaysMax
	body := []string{
		fmt.Sprintf("Run up to %s, stopping when something needs you.", plural(n, "day")),
		"",
		"Days      " + days.View(),
		"",
		theme.Subtle.Render("Stops for a card, the police, the rival, the crew, the road, a buyer,"),
		theme.Subtle.Render("the law, a routine that ran short, a gate crossed or a new alert."),
	}
	if m.fst.err != "" {
		body = append(body, "", theme.Bad.Render(m.fst.err))
	}
	return m.modal("FAST-FORWARD", body, m.modalFooter())
}

// fastForward ends up to days days, each through stepDay as n ends one,
// and stops before the report of the first that needs you: it opens
// that morning as endDay does (the run over, the card, then the
// report), the report's first line naming the reason.
func (m *Model) fastForward(days int) {
	m.mode = modePlay
	if m.w.Over != nil {
		m.mode = modeOver
		return
	}
	ran := 0
	var reason string
	var evs []events.Event
	for ran < days {
		before := m.alerts()
		evs = m.stepDay()
		ran++
		if m.w.Over != nil {
			break
		}
		if reason = m.stopReason(evs, before); reason != "" {
			break
		}
	}
	if m.w.Over == nil {
		if reason == "" {
			reason = "the cap"
		}
		m.fastStop = fmt.Sprintf("Stopped after %s: %s.", plural(ran, "day"), reason)
	}
	m.say(fmt.Sprintf("Ran %s.", plural(ran, "day")))
	m.morning(evs) // the stopping day's events: an earlier day's strike is the journal's
}

// stopReason is why the day that just ended needs you, or "" when it
// does not: a new stage (#149, first: the tier entered this morning),
// a card dealt, then an alert that was not on the dashboard
// the morning before (by its key, so a contract due tomorrow stops once
// and again when it is due today, and heat over the patrol line once
// until it drops under and comes back), then the first of the day's
// events that stopEvent names.
func (m *Model) stopReason(evs []events.Event, before []alert) string {
	if m.w.StagePending() > 0 {
		return "a new stage"
	}
	if m.w.Dilemmas.Pending != nil {
		return "a card to answer"
	}
	was := map[string]bool{}
	for _, a := range before {
		was[a.key] = true
	}
	for _, a := range m.alerts() {
		if !was[a.key] {
			return a.why
		}
	}
	for _, e := range evs {
		if why := m.stopEvent(e); why != "" {
			return why
		}
	}
	return ""
}

// stopEvent is the reason an event stops the run, or "" for one that
// does not: the police past a patrol, a corner struck or taken off you,
// the crew walking, an audit, a seizure, the rival's offer or a deal
// broken, a corner the rival gave up to a price war (free: the tell's
// kind of stop, a corner to post on), the police raiding a rival corner
// on your tip (free too) or a boost that failed (#70), a buyer asking,
// pressure or a
// reputation axis up a band, a new chief or an election, a contract
// or a standing order that ran short (the routine broke), a gate crossed
// (#148: the Laundromat open to you, Heroin on offer, the Dutchman
// dealing, lieutenants wanting work), the rival moving in, and a stash
// house robbed, hit or lost (#73).
func (m *Model) stopEvent(e events.Event) string {
	w := m.w
	switch ev := e.(type) {
	case events.Enforcement:
		if ev.Level != content.Patrol {
			return format.A(ev.Level) + " in " + w.CityName(ev.City)
		}
	case events.Unlocked:
		return unlockStop(ev)
	case events.RivalMovedIn:
		return ev.Rival + " moved in on " + ev.Name
	case events.RivalEyeing:
		return ev.Rival + " is eyeing " + ev.Name
	case events.CornerStruck:
		return "the strike on " + ev.Name
	case events.RivalBoosted:
		if !ev.Taken {
			return "the boost on " + ev.Name + " failed"
		}
	case events.RivalRaided:
		return "the police raided " + ev.Name
	case events.CornerTaken:
		if ev.From == game.OwnerPlayer {
			return ev.Rival + " took " + ev.Name
		}
	case events.RivalAbandoned:
		return ev.Rival + " gave up " + ev.Name
	case events.CrewQuit:
		return ev.Name + " quit"
	case events.CrewDefected:
		return ev.Name + " defected"
	case events.LieutenantWalked:
		return ev.Name + " walked with " + ev.CityName
	case events.FrontAudited:
		return "an audit at " + ev.Name
	case events.ShipmentSeized:
		return "a shipment seized"
	case events.DealOffered:
		return ev.Rival + " offers " + format.A(ev.Deal)
	case events.DealBroken:
		if ev.By == "you" {
			return "you broke " + format.A(ev.Deal)
		}
		return ev.Rival + " broke " + format.A(ev.Deal)
	case events.ContractOffered:
		return ev.Name + " is asking"
	case events.PressureShifted:
		if ev.To > ev.From {
			return "pressure up in " + w.CityName(ev.City)
		}
	case events.ReputationShifted:
		if ev.To > ev.From {
			return ev.Axis + " up"
		}
	case events.ChiefReplaced:
		return "a new chief"
	case events.DAElected:
		return "the election"
	case events.SupplyShort:
		return fmt.Sprintf("the %s contract in %s short of %s", w.ProductName(ev.Product), w.CityName(ev.City), ev.Why)
	case events.StandingShort:
		return fmt.Sprintf("the standing order for %s in %s short of stock", w.ProductName(ev.Product), w.CityName(ev.City))
	// The stash houses (#73).
	case events.HouseRobbed:
		return ev.Name + " robbed"
	case events.HouseRaided:
		return format.A(ev.Level) + " at " + ev.Name
	case events.HouseLost:
		return "the landlord threw you out of " + ev.Name
	}
	return ""
}

// stopLine is the report's first line after a fast-forward, or "".
func (m *Model) stopLine() string { return m.fastStop }

// unlockStop is why a gate crossed stops a fast-forward, by its gate:
// `the Laundromat is open to you`, `Heroin is on offer`, `the Dutchman
// deals with you`, `lieutenants want work`.
func unlockStop(ev events.Unlocked) string {
	switch ev.Gate {
	case "front":
		return "the " + ev.Name + " is open to you"
	case "product":
		return ev.Name + " is on offer"
	case "connect":
		return ev.Name + " deals with you"
	case "role":
		return strings.ToLower(ev.Name) + " want work"
	}
	return ev.Name + " is open to you"
}
