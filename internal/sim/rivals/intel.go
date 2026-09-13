package rivals

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Intel (#45): what the factions let you know, and what they feed you.
// The rivals sim owns the truth about a faction, so it writes the facts
// about one: its temper once it has shown its hand (Observed, the #12
// pattern, filed at full confidence and never fading), its muscle within
// a band the night your enforcers or its met (a push on you, a strike,
// a boost: observe_confidence, observe_band), and the books the scout
// read (#70: cash, income, wages and muscle at full confidence, fading
// at stale_rate). None of that rolls dice: the observation facts are
// written off the tick's own events, so a run that never uses the
// feature is byte-for-byte the run before it. The feed is the one roll,
// on Tick.Sub("intel"): a faction that trusts you under feed_trust
// plants a lie at feed_chance a night, a road you are not on claiming
// feed_risk a day (the logistics sim seizes the first shipment on it,
// no dice) or one of its own corners as where its till is (a boost there
// finds it empty). A lie is filed as "a contact says" and names the
// faction only once it has bitten (game.Fact.Planted, Expose).

// Intel exposes the intel tuning for the UI and the harness.
func (s *Sim) Intel() content.IntelTuning { return s.intel }

// observe files what tonight's fighting showed of a faction: the
// muscle you met, within a band, off the events this step emitted, and
// the temper of every faction that has shown its hand. No dice.
func (s *Sim) observe(w *game.World, t *game.Tick) {
	tun := s.intel
	seen := map[string]bool{}
	for _, e := range t.Events() {
		var id string
		switch ev := e.(type) {
		case events.CornerStruck:
			id = ev.Faction
		case events.RivalBoosted:
			id = ev.Faction
		case events.RivalPushed:
			id = ev.Faction
		case events.CornerTaken:
			if ev.From == game.OwnerPlayer {
				id = ev.Faction
			}
		}
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		r := w.Faction(id)
		if r == nil {
			continue
		}
		lo, hi := game.MuscleBand(r.Muscle, tun.ObserveBand)
		f := game.Fact{
			Subject: r.Faction(), Kind: game.FactMuscle, Value: fmt.Sprintf("%d–%d", lo, hi), Number: float64(lo+hi) / 2,
			Confidence: tun.ObserveConfidence, Day: t.Day, Source: game.SourceSeen, Stale: tun.StaleRate, Forget: tun.Forget,
		}
		// A count stands over a band: the scout's or a spy's exact read
		// is not replaced while it lives (the saboteur's books read as
		// #70 left them until they fade).
		if old, ok := game.Known(w).Fact(r.Faction(), game.FactMuscle); ok && old.Source != game.SourceSeen && old.Alive(t.Day) {
			continue
		}
		w.Learn(f)
		t.Emit(events.IntelGained{Day: t.Day, Subject: f.Subject, FactKind: f.Kind, Value: f.Value, Confidence: f.Confidence, Source: f.Source, Name: r.Leader})
	}
	for _, r := range w.Rivals {
		if r == nil || !r.Observed || r.Personality == "" {
			continue
		}
		if f, ok := game.Known(w).Fact(r.Faction(), game.FactPersonality); ok && f.Confidence >= 1 {
			continue
		}
		f := game.Fact{Subject: r.Faction(), Kind: game.FactPersonality, Value: r.Personality, Confidence: 1, Day: t.Day, Source: game.SourceSeen}
		w.Learn(f)
		t.Emit(events.IntelGained{Day: t.Day, Subject: f.Subject, FactKind: f.Kind, Value: f.Value, Confidence: 1, Source: f.Source, Name: r.Leader})
	}
}

// file writes the books a scout read (#70) as facts: the chest, the
// take, the wage bill and the muscle, tonight's numbers at full
// confidence, fading at stale_rate.
func (s *Sim) file(w *game.World, t *game.Tick, r *game.RivalState, b game.Books) {
	w.LearnBooks(r.Faction(), b, s.intel.StaleRate, s.intel.Forget)
	t.Emit(events.IntelGained{Day: t.Day, Subject: r.Faction(), FactKind: game.FactMuscle, Value: fmt.Sprint(b.Muscle), Confidence: 1, Source: game.SourceBooks, Name: r.Leader})
}

// feed is a faction that distrusts you planting a lie (#45): under
// feed_trust, at feed_chance a night off the intel stream, a road with
// its dial off and nothing on it claiming feed_risk a day, or one of
// its own corners named as where its till is, filed at feed_confidence
// as "a contact says". The roll is made only for a faction under the
// line, so a run nobody distrusts draws nothing.
func (s *Sim) feed(w *game.World, t *game.Tick, r *game.RivalState) {
	tun := s.intel
	if tun.FeedChance <= 0 || !r.Alive() || r.Trust >= tun.FeedTrust {
		return
	}
	rng := t.Sub("intel")
	if rng.Float64() >= tun.FeedChance {
		return
	}
	// The roads you are not on, and its corners.
	var roads []string
	for _, id := range s.routes {
		rs := w.Route(id)
		if rs.Dial != events.RouteOff {
			continue
		}
		if a := s.routeAsset[id]; a != "" && !w.AssetLive(a) {
			continue // a road an asset opens (#48) is no road until it does
		}
		busy := false
		for _, sh := range w.Shipments {
			if sh.Route == id {
				busy = true
			}
		}
		if busy || w.Lure(id, game.FactRisk) != nil {
			continue
		}
		roads = append(roads, id)
	}
	var corners []string
	for _, c := range s.corners(w, r) {
		if owns(c, r) {
			corners = append(corners, c.ID)
		}
	}
	if len(roads)+len(corners) == 0 {
		return
	}
	i := rng.IntN(len(roads) + len(corners))
	var f game.Fact
	name := ""
	if i < len(roads) {
		id := roads[i]
		f = game.Fact{Subject: id, Kind: game.FactRisk, Value: fmt.Sprintf("~%.0f%%/day", tun.FeedRisk*100), Number: tun.FeedRisk}
		name = s.routeName(id)
	} else {
		id := corners[i-len(roads)]
		f = game.Fact{Subject: r.Faction(), Kind: game.FactStash, Value: id, Number: 0}
		if c := w.Corner(id); c != nil {
			name = c.Name
		}
	}
	f.Confidence, f.Day, f.Source, f.Stale, f.Forget, f.Planted = tun.FeedConfidence, t.Day, game.SourceContact, tun.StaleRate, tun.Forget, r.Faction()
	w.Learn(f)
	w.Stats.Lures++
	t.Emit(events.IntelGained{Day: t.Day, Subject: f.Subject, FactKind: f.Kind, Value: f.Value, Confidence: f.Confidence, Source: f.Source, Name: name})
}

// routeName is a route's name for a line, or its id.
func (s *Sim) routeName(id string) string {
	if n, ok := s.routeNames[id]; ok {
		return n
	}
	return id
}

// lured reports whether a boost on the corner walks into a lie (#45):
// a fact a faction fed you naming this corner as where its till is.
// The plant bites: the till is empty, the fact names the faction, and
// the paper hears of it.
func (s *Sim) lured(w *game.World, t *game.Tick, r *game.RivalState, c *game.Corner) bool {
	f := w.Lure(r.Faction(), game.FactStash)
	if f == nil || f.Value != c.ID {
		return false
	}
	fed := w.Expose(r.Faction(), game.FactStash)
	w.Stats.Bitten++
	t.Emit(events.IntelFalse{Day: t.Day, Subject: r.Faction(), FactKind: game.FactStash, Name: c.Name, Rival: r.Leader, Faction: fed})
	return true
}
