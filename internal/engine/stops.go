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
// back), then the first of the day's events that StopsOn names, the
// world says you could answer (serves) and is news (news: a shortfall
// the morning it starts, an offer not repeated, #469). before
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
		if StopsOn(e) && s.serves(e) && s.news(e) {
			return Stop{Kind: StopEvent, Event: e}
		}
	}
	return Stop{}
}

// serves filters StopsOn by the world (#442): a buyer's offer stops the
// fast-forward only in a city you could answer it from, where you stand,
// work a corner or hold stock; one anywhere else is in the report and on
// the market (#440) and runs past. A playtest's fast-forward stopped on
// four Bayport offers in twelve mornings with nothing and nobody there.
func (s *Session) serves(e events.Event) bool {
	c, ok := e.(events.ContractOffered)
	if !ok {
		return true
	}
	w := s.w
	return c.City == w.Player.Location || w.WorkedIn(c.City) > 0 || w.StockIn(c.City) > 0
}

// OfferQuiet is how many days a faction's offer of a kind of deal
// stays news (#469): the same faction putting the same kind on the
// table again within it (one let lapse or turned down, and asked
// again) is in the report and on the rivals screen and runs past a
// fast-forward. A playtest's stopped on "Rosalind offers a tribute"
// every one to three days for five hundred days.
const OfferQuiet = 30

// memo is what the session remembers of the nights it ended (#469), so
// a fast-forward stops on a trouble the morning it starts and not every
// morning it goes on. It is the front end's reading, never saved and
// never read by a sim, and it draws nothing: a load, a new run and an
// attach start it over, so the worst a reload costs is one stop more.
type memo struct {
	short, shortBefore map[string]bool // the shortfalls left uncovered by the last night and by the night before it (shortKey)
	offered            map[string]int  // the day each faction last offered each kind of deal (offerKey)
	again              map[string]bool // the last night's offers that repeat one within OfferQuiet
}

// forget starts the memo over: a new run, a load, an attach.
func (s *Session) forget() { s.memo = memo{} }

// remember reads a night's events into the memo, after the night: the
// shortfalls it left uncovered and the offers it repeated.
func (s *Session) remember(evs []events.Event) {
	m := &s.memo
	m.shortBefore, m.short, m.again = m.short, map[string]bool{}, map[string]bool{}
	if m.offered == nil {
		m.offered = map[string]int{}
	}
	for _, e := range evs {
		switch ev := e.(type) {
		case events.StandingShort, events.SupplyShort:
			if !s.covered(e) {
				m.short[shortKey(e)] = true
			}
		case events.DealOffered:
			k := offerKey(ev)
			if last, ok := m.offered[k]; ok && ev.Day-last <= OfferQuiet {
				m.again[k] = true
			}
			m.offered[k] = ev.Day
		}
	}
}

// shortKey is a shortfall's identity night to night: the standing
// order's or the supply contract's city and product.
func shortKey(e events.Event) string {
	switch ev := e.(type) {
	case events.StandingShort:
		return "standing " + ev.City + " " + ev.Product
	case events.SupplyShort:
		return "supply " + ev.City + " " + ev.Product
	}
	return ""
}

// offerKey is an offer's identity from one to the next: the faction and
// the kind of deal.
func offerKey(ev events.DealOffered) string { return ev.Faction + " " + ev.Deal }

// covered reports whether a shortfall is made good by the morning
// (#469): a standing order whose stash a route or a batch landed enough
// into the same night, a supply contract a landing brought up to its
// level. It reads the world after the night and draws nothing.
func (s *Session) covered(e events.Event) bool {
	w := s.w
	switch ev := e.(type) {
	case events.StandingShort:
		return w.Stock(ev.City, ev.Product) >= ev.Units
	case events.SupplyShort:
		c, ok := w.StandingSupply(ev.City, ev.Product)
		return !ok || s.set.Market.Shortfall(w, c) <= 0
	}
	return false
}

// news filters StopsOn by what the session remembers (#469): a
// shortfall stops the morning it starts, not every morning it goes on,
// and not at all when a landing covered it the same night; a supply
// contract short of room never stops (the stash is full of stock, held
// for a buyer or not, and stash_full says so once); an offer stops
// unless the same faction offered the same kind of deal within
// OfferQuiet. Every other event is news.
func (s *Session) news(e events.Event) bool {
	switch ev := e.(type) {
	case events.SupplyShort:
		if ev.Why == "room" {
			return false
		}
		return !s.covered(e) && !s.memo.shortBefore[shortKey(e)]
	case events.StandingShort:
		return !s.covered(e) && !s.memo.shortBefore[shortKey(e)]
	case events.DealOffered:
		return !s.memo.again[offerKey(ev)]
	}
	return true
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
// corner, a faction scouting or recruiting where you earn (#341), a strike (bar a war night that held, #229), the war over, a
// boost that failed (#70), the police raiding a rival corner, a corner
// taken off you, a corner of yours lost to the street, nobody working it
// (#345) or the police clearing it (#469), a corner the rival gave up,
// the crew quitting, defecting, arrested (#46), shot dead or retiring,
// a spy found or a lie that bit (#45), a lieutenant walking, an audit, a
// seizure, a deal offered or broken (an offer the faction repeated runs
// past: Stop's news, #469), a buyer asking (where you could answer it:
// Stop's serves, #442), pressure or a reputation axis up a band, a new
// chief or an election, an envelope back, a raid that fell through
// (#228), the DA's file on the envelopes, the officials cold, a contract
// or a standing order short (the morning it starts: news, #469), and a
// stash house robbed, hit or lost (#73).
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
		return ev.Owner == game.OwnerPlayer // nobody worked it (#345), or the police cleared it (#469)
	case events.CrewShot:
		return ev.Dead && !ev.Theirs
	case events.PressureShifted:
		return ev.To > ev.From
	case events.ReignBegan:
		return !ev.Again // the first reign of the run (#399); one begun again runs past
	case events.ReputationShifted:
		return ev.To > ev.From
	case events.TaskForceFormed, events.InvestigationOpened, events.AssetSeized, events.TrophySeized, events.TunnelFound, events.Unlocked,
		events.ReignBroken, events.StraightOpened, events.StraightLapsed, events.RivalMovedIn, events.RivalEyeing,
		events.WarEnded, events.RivalRaided, events.RivalAbandoned, events.CrewQuit,
		events.CrewDefected, events.CrewArrested, events.CrewRetired, events.SpyFound,
		events.IntelFalse, events.LieutenantWalked, events.FrontAudited, events.ShipmentSeized, events.ExportSeized,
		events.DealOffered, events.DealBroken, events.ContractOffered, events.ChiefReplaced,
		events.DAElected, events.BribeBackfired, events.RaidFellThrough, events.LeadsFiled,
		events.OfficialsCold, events.SupplyShort, events.StandingShort, events.HouseRobbed,
		events.HouseRaided, events.HouseLost, events.RivalScouting, events.RivalRecruiting:
		return true
	}
	return false
}
