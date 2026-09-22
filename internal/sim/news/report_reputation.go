package news

import "github.com/theclifmeister/kingpin/internal/events"

// reportReputation writes the reputation sim's events into the morning: the
// headlines and the report's lines. It says whether e was one of them.
func (r *reporter) reportReputation(e events.Event) bool {
	base := r.base
	switch ev := e.(type) {
	case events.ReputationShifted:
		key := "Reputation" + capitalize(ev.Axis) + "Down"
		if ev.Up() {
			key = "Reputation" + capitalize(ev.Axis) + "Up"
		}
		r.add("reputation", key, base)
	default:
		return false
	}
	return true
}
