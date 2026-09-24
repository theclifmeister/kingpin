package engine

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// StopKind is why a fast-forward (#116) stops on a day.
type StopKind string

// The stops, in the order Stop weighs them.
const (
	StopNone  StopKind = ""
	StopStage StopKind = "stage" // a new stage entered this morning (#149)
	StopCard  StopKind = "card"  // a dilemma card dealt
	StopAlert StopKind = "alert" // an alert the morning before did not have: Alert
	StopEvent StopKind = "event" // one of the day's events that needs you: Event
	StopOver  StopKind = "over"  // the run ended
	StopCap   StopKind = "cap"   // the days asked for ran out
)

// Stop is why the day that just ended needs you, or StopNone.
type Stop struct {
	Kind  StopKind
	Alert Alert        // StopAlert
	Event events.Event // StopEvent
}

// Stop weighs the day that just ended: a new stage first, then a card
// dealt, then an alert the morning before did not have (by its key, so
// a contract due tomorrow stops once and again when it is due today,
// and heat over the patrol line once until it drops under and comes
// back), then the first of the day's events that StopsOn names. before
// is Alerts as they stood the morning the day began; evs the day's
// events.
func (s *Session) Stop(evs []events.Event, before []Alert) Stop {
	w := s.w
	if w.StagePending() > 0 {
		return Stop{Kind: StopStage}
	}
	if w.Dilemmas.Pending != nil {
		return Stop{Kind: StopCard}
	}
	was := map[string]bool{}
	for _, a := range before {
		was[a.Key] = true
	}
	for _, a := range s.Alerts() {
		if !was[a.Key] {
			return Stop{Kind: StopAlert, Alert: a}
		}
	}
	for _, e := range evs {
		if StopsOn(e) {
			return Stop{Kind: StopEvent, Event: e}
		}
	}
	return Stop{}
}

// FastForward ends up to days days and stops on the first that needs
// you (Stop), the run's end or the cap. after is called with each day's
// events as it ends, before the day is weighed: the TUI saves there.
// It returns the days run, why it stopped and the stopping day's
// events. A run already over runs nothing and stops StopOver.
func (s *Session) FastForward(days int, after func([]events.Event)) (int, Stop, []events.Event) {
	if s.w == nil || s.w.Over != nil {
		return 0, Stop{Kind: StopOver}, nil
	}
	var evs []events.Event
	for ran := 1; ran <= days; ran++ {
		before := s.Alerts()
		evs = s.EndDay()
		if after != nil {
			after(evs)
		}
		if s.w.Over != nil {
			return ran, Stop{Kind: StopOver}, evs
		}
		if st := s.Stop(evs, before); st.Kind != StopNone {
			return ran, st, evs
		}
	}
	return days, Stop{Kind: StopCap}, evs
}

// StopsOn reports whether an event stops a fast-forward: the police past
// a patrol, the task force, an investigation opened (#343), an asset seized or the tunnel found, a gate
// crossed, the reign begun or broken, the rival moving in or eyeing a
// corner, a strike (bar a war night that held, #229), the war over, a
// boost that failed (#70), the police raiding a rival corner, a corner
// taken off you, a corner of yours nobody worked gone back to the street
// (#345), a corner the rival gave up, the crew quitting,
// defecting, arrested (#46), shot dead or retiring, a spy found or a lie
// that bit (#45), a lieutenant walking, an audit, a seizure, a deal
// offered or broken, a buyer asking, pressure or a reputation axis up a
// band, a new chief or an election, an envelope back, a raid that fell
// through (#228), the DA's file on the envelopes, the officials cold, a
// contract or a standing order short, and a stash house robbed, hit or
// lost (#73).
func StopsOn(e events.Event) bool {
	switch ev := e.(type) {
	case events.Enforcement:
		return ev.Level != content.Patrol
	case events.CornerStruck:
		return !ev.War || ev.Taken
	case events.RivalBoosted:
		return !ev.Taken
	case events.CornerTaken:
		return ev.From == game.OwnerPlayer
	case events.CornerLost:
		return ev.Reason == "idle" && ev.Owner == game.OwnerPlayer
	case events.CrewShot:
		return ev.Dead && !ev.Theirs
	case events.PressureShifted:
		return ev.To > ev.From
	case events.ReputationShifted:
		return ev.To > ev.From
	case events.TaskForceFormed, events.InvestigationOpened, events.AssetSeized, events.TunnelFound, events.Unlocked,
		events.ReignBegan, events.ReignBroken, events.RivalMovedIn, events.RivalEyeing,
		events.WarEnded, events.RivalRaided, events.RivalAbandoned, events.CrewQuit,
		events.CrewDefected, events.CrewArrested, events.CrewRetired, events.SpyFound,
		events.IntelFalse, events.LieutenantWalked, events.FrontAudited, events.ShipmentSeized,
		events.DealOffered, events.DealBroken, events.ContractOffered, events.ChiefReplaced,
		events.DAElected, events.BribeBackfired, events.RaidFellThrough, events.LeadsFiled,
		events.OfficialsCold, events.SupplyShort, events.StandingShort, events.HouseRobbed,
		events.HouseRaided, events.HouseLost:
		return true
	}
	return false
}
