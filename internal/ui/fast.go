package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
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

// askFast opens the confirmation on the default cap: an amountDialog
// (#275), the cap and the error under it.
func (m *Model) askFast() {
	if m.w.Over != nil {
		return
	}
	m.openAmount(modeConfirmFast, fmt.Sprintf("blank = %d", fastDays), fastDaysMax, false, "")
}

// fastCap is the cap the field reads: fastDays for a blank, an error for
// a number that does not read or is over fastDaysMax.
func (m *Model) fastCap() (int, error) {
	n, err := m.amt.Read(fastDays)
	if err != nil {
		return 0, err
	}
	if n > fastDaysMax {
		return 0, fmt.Errorf("up to %d days at a time", fastDaysMax)
	}
	return n, nil
}

// keyFast is the confirmation: enter runs the days, esc closes, and
// the rest goes to the number field (digits, backspace and the field's
// shortcuts; a letter never lands in it).
func (m *Model) keyFast(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	return m.keyAmount(k, func() int { return fastDaysMax }, m.confirmFast)
}

// confirmFast runs the days the field reads, or shows why it cannot.
func (m *Model) confirmFast() {
	n, err := m.fastCap()
	if err != nil {
		m.amt.err = dialogError(err)
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
	days := m.amountField(fastDaysMax)
	body := []string{
		fmt.Sprintf("Run up to %s, stopping when something needs you.", plural(n, "day")),
		"",
		row("days", days.View()),
		"",
		theme.Subtle.Render("Stops for a card, the police, the rival, the crew, the road, a buyer,"),
		theme.Subtle.Render("the law, a routine that ran short, a gate crossed or a new alert."),
	}
	if m.amt.err != "" {
		body = append(body, "", theme.Bad.Render(m.amt.err))
	}
	return m.modal("FAST-FORWARD", body, m.modalFooter())
}

// fastForward ends up to days days through the engine
// (Session.FastForward, #298), each saved and journalled as n's is
// (dayEnded), and stops on the first that needs you: it opens that
// morning as endDay does (the run over, the card, then the report), the
// report's first line naming the reason.
func (m *Model) fastForward(days int) {
	m.mode = modePlay
	if m.w.Over != nil {
		m.finish(false)
		return
	}
	ran, stop, evs := m.sess.FastForward(days, m.dayEnded)
	if m.w.Over == nil {
		m.fastStop = fmt.Sprintf("Stopped after %s: %s.", plural(ran, "day"), m.stopWhy(stop))
		if stop.Kind == engine.StopAlert {
			a := stop.Alert
			m.fastAlert = &a // the report offers the jump (#352)
		}
	}
	m.say(fmt.Sprintf("Ran %s.", plural(ran, "day")))
	m.morning(evs) // the stopping day's events: an earlier day's strike is the journal's
}

// stopWhy is why a fast-forward stopped, as the report's first line
// names it: a new stage, a card, a new alert's reason, the event's
// (stopEvent) or the cap.
func (m *Model) stopWhy(st engine.Stop) string {
	switch st.Kind {
	case engine.StopStage:
		return "a new stage"
	case engine.StopCard:
		return "a card to answer"
	case engine.StopAlert:
		return m.alertOf(st.Alert).why
	case engine.StopEvent:
		return m.stopEvent(st.Event)
	}
	return "the cap"
}

// stopEvent is the reason an event stops a fast-forward, as the report
// names it, or "" for one that does not. Which events stop is the
// engine's (engine.StopsOn, #298); this is only the words, and an event
// the engine stops on that has no words here is named by its kind
// (TestEveryStopHasWords).
func (m *Model) stopEvent(e events.Event) string {
	if !engine.StopsOn(e) {
		return ""
	}
	w := m.w
	switch ev := e.(type) {
	case events.Enforcement:
		if ev.Level == content.TaskForce {
			return "the task force in " + w.CityName(ev.City)
		}
		return format.A(ev.Level) + " in " + w.CityName(ev.City)
	case events.TaskForceFormed:
		return "a task force formed in " + w.CityName(ev.City)
	case events.InvestigationOpened:
		return "the police are working " + ev.Name
	case events.AssetSeized:
		return "the feds took " + ev.Name
	case events.TunnelFound:
		return "the tunnel was found"
	case events.Unlocked:
		return unlockStop(ev)
	case events.ReignBegan:
		return "the city is yours" // the reign (#227)
	case events.ReignBroken:
		return "the reign is over: " + ev.Why
	case events.StraightOpened:
		return "you could go straight" // #398
	case events.StraightLapsed:
		return "going straight is off again"
	case events.RivalMovedIn:
		return ev.Rival + " moved in on " + ev.Name
	case events.RivalEyeing:
		return ev.Rival + " is eyeing " + ev.Name
	case events.RivalScouting:
		return ev.Rival + " has scouts in " + m.w.CityName(ev.City)
	case events.RivalRecruiting:
		return ev.Rival + " is recruiting in " + m.w.CityName(ev.City)
	case events.CornerStruck:
		return "the strike on " + ev.Name
	case events.WarEnded:
		return "the war on " + ev.Rival + "'s crew is over"
	case events.RivalBoosted:
		return "the boost on " + ev.Name + " failed"
	case events.RivalRaided:
		return "the police raided " + ev.Name
	case events.CornerTaken:
		return ev.Rival + " took " + ev.Name
	case events.CornerLost:
		return ev.Name + " went back to the street" // nobody worked it (#345)
	case events.RivalAbandoned:
		return ev.Rival + " gave up " + ev.Name
	case events.CrewQuit:
		return ev.Name + " quit"
	case events.CrewDefected:
		return ev.Name + " defected"
	case events.CrewArrested:
		return ev.Name + " was arrested" // #46
	case events.CrewShot:
		return ev.Name + " was shot dead"
	case events.CrewRetired:
		return ev.Name + " retired"
	case events.SpyFound: // #45: a spy found, shot or home, and a lie that bit
		if ev.Dead {
			return ev.Name + " was found and shot"
		}
		return ev.Name + " came home"
	case events.IntelFalse:
		return "the word on " + ev.Name + " was " + ev.Rival + "'s"
	case events.LieutenantWalked:
		return ev.Name + " walked with " + ev.CityName
	case events.FrontAudited:
		return "an audit at " + ev.Name
	case events.ShipmentSeized:
		return "a shipment seized"
	case events.ExportSeized:
		return "a load seized abroad"
	case events.TrophySeized:
		return "the feds took " + ev.Name
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
		return "pressure up in " + w.CityName(ev.City)
	case events.ReputationShifted:
		return ev.Axis + " up"
	case events.ChiefReplaced:
		return "a new chief"
	case events.DAElected:
		return "the election"
	// The bought law (#42): an envelope that came back, the DA's file on
	// the envelopes, and the day the phones stop.
	case events.BribeBackfired:
		return "the envelope came back"
	case events.RaidFellThrough:
		return "the " + favourWord(ev.Level) + " fell through" // the favour (#228)
	case events.LeadsFiled:
		return "the DA's file on your envelopes"
	case events.OfficialsCold:
		return "the officials going cold"
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
	return e.Kind()
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
	case "asset":
		return ev.Name + " is for sale"
	}
	return ev.Name + " is open to you"
}
