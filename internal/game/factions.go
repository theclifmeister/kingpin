package game

import "fmt"

// The factions (#43): the one rival became a table of three to six.
// World.Rivals holds them in id order and the first is the rival at
// home, the one every reader that means "the rival" gets from Rival();
// the rest live where the seed put them and roll on their own streams,
// so a run with one faction is byte-for-byte the duel it was.

// FactionYou is the counterpart id a faction uses for the player in its
// Ally and Against slots.
const FactionYou = "you"

// Rival is the rival at home: the first faction, the one the dashboard,
// the harness policies and every reader that means "the rival" talk
// about. A world built by hand with none gets one, empty, so the sims
// and the screens read it as they always did.
func (w *World) Rival() *RivalState {
	if len(w.Rivals) == 0 {
		w.Rivals = []*RivalState{{ID: FactionRival}}
	}
	return w.Rivals[0]
}

// Faction returns the faction with id, or nil; "" is the rival at home,
// the id a save from before ids resolves to (RivalState.Faction).
func (w *World) Faction(id string) *RivalState {
	if id == "" || id == FactionRival {
		return w.Rival()
	}
	for _, r := range w.Rivals {
		if r != nil && r.Faction() == id {
			return r
		}
	}
	return nil
}

// FactionIndex is the faction's position in Rivals, the number the map
// colours it by and the screens list it under; -1 for an id nobody has.
func (w *World) FactionIndex(id string) int {
	if id == "" {
		return 0
	}
	for i, r := range w.Rivals {
		if r != nil && r.Faction() == id {
			return i
		}
	}
	return -1
}

// FactionName is the faction in words, `Big Sal's crew`, or "the rival"
// for one with no leader yet.
func (w *World) FactionName(id string) string {
	r := w.Faction(id)
	if r == nil || r.Leader == "" {
		return "the rival"
	}
	return r.Leader + "'s crew"
}

// CityOf is the city a faction lives in: its Home, or the home city for
// the rival at home and a save from before factions had one.
func (w *World) CityOf(r *RivalState) *City {
	if r != nil && r.Home != "" {
		if c := w.Cities[r.Home]; c != nil {
			return c
		}
	}
	return w.Home()
}

// RivalHeldBy counts the corners a faction holds.
func (w *World) RivalHeldBy(id string) int {
	n := 0
	for _, c := range w.Corners() {
		if c.FactionID() == id {
			n++
		}
	}
	return n
}

// HeldByIn counts the corners a faction holds in one city.
func (w *World) HeldByIn(id, city string) int {
	n := 0
	if c := w.Cities[city]; c != nil {
		for _, k := range c.Corners {
			if k.FactionID() == id {
				n++
			}
		}
	}
	return n
}

// StrongestFaction is the faction holding the most corners in a city,
// the one a defector or a lieutenant's walk hands ground to; the first
// by id on a tie; nil while nobody holds any there.
func (w *World) StrongestFaction(city string) *RivalState {
	var best *RivalState
	most := 0
	for _, r := range w.Rivals {
		if r == nil || r.Gone() {
			continue
		}
		if n := w.HeldByIn(r.Faction(), city); n > most {
			best, most = r, n
		}
	}
	return best
}

// side is who holds a corner for the purposes of a border: the player,
// a faction by id, or nobody.
func side(c Corner) string {
	switch c.Owner {
	case OwnerPlayer:
		return OwnerPlayer
	case OwnerRival:
		return OwnerRival + ":" + c.FactionID()
	}
	return ""
}

// Contested reports whether a corner borders one another side holds: a
// player corner next to any faction's, a faction's next to the
// player's or another faction's (#43). A free corner is contested by
// nobody.
func (w *World) Contested(c Corner) bool {
	mine := side(c)
	if mine == "" {
		return false
	}
	city := w.Cities[c.City]
	if city == nil {
		return false
	}
	for _, o := range city.Corners {
		if s := side(o); s != "" && s != mine && c.Borders(o) {
			return true
		}
	}
	return false
}

// ContestedBy reports whether a corner not the faction's borders one
// the faction holds: the corners a faction pushes on.
func (w *World) ContestedBy(c Corner, faction string) bool {
	if faction == "" {
		faction = FactionRival
	}
	if c.Owner == OwnerNone || c.FactionID() == faction {
		return false
	}
	city := w.Cities[c.City]
	if city == nil {
		return false
	}
	for _, o := range city.Corners {
		if o.FactionID() == faction && c.Borders(o) {
			return true
		}
	}
	return false
}

// Dominant reports whether the city is yours (#43, the ending's read):
// every faction is absorbed, fragmented or paying you homage. A faction
// that has not arrived is not beaten, and with no factions there is
// nobody to have beaten, so a run nobody fought is never dominant by
// accident.
func (w *World) Dominant() bool {
	if len(w.Rivals) == 0 {
		return false
	}
	for _, r := range w.Rivals {
		if r == nil || r.Gone() {
			continue
		}
		if r.Arrived == 0 || w.DealWith(r.Faction(), DealHomage) == nil {
			return false
		}
	}
	return true
}

// Stance is a faction's stance toward you in a word for a table:
// absorbed, fragmented, homage, truce, tribute, split, war (a war worth
// the name), or quiet.
func (w *World) Stance(r *RivalState, warLine float64) string {
	switch {
	case r.Absorbed > 0:
		return "absorbed"
	case r.Fragmented > 0:
		return "fragmented"
	case r.Arrived == 0:
		return "not yet"
	case w.DealWith(r.Faction(), DealHomage) != nil:
		return "homage"
	case w.DealWith(r.Faction(), DealTruce) != nil:
		return "truce"
	case w.DealWith(r.Faction(), DealTribute) != nil:
		return "tribute"
	case w.DealWith(r.Faction(), DealSplit) != nil:
		return "split"
	case r.War >= warLine:
		return "war"
	}
	return "quiet"
}

// ErrNoFaction is the refusal for an id nobody has.
var ErrNoFaction = fmt.Errorf("no such faction")
