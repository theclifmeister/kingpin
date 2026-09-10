package game

import (
	"errors"
	"fmt"

	"github.com/theclifmeister/kingpin/internal/events"
)

// Owners of a corner.
const (
	OwnerNone   = "none"
	OwnerPlayer = "player"
	OwnerRival  = "rival"
)

// You is the worker id for the player standing on a corner in person.
const You = -1

var (
	ErrNoCorner    = errors.New("no such corner")
	ErrCornerTaken = errors.New("somebody else holds that corner")
	ErrNotPostable = errors.New("only runners and enforcers work corners")
	ErrNoEnforcers = errors.New("no enforcers on the payroll")
)

// TerritoryState is the city's corners and who works them.
type TerritoryState struct {
	Corners []Corner
}

// Corner is one block of the city: a demand pool the player has to hold to
// serve. Static tuning is copied in from content so the world never needs
// the city config to step.
type Corner struct {
	ID       string
	Name     string
	X, Y     int                // map cell
	Demand   float64            // size relative to one standard corner
	Taste    map[string]float64 // per-product demand multiplier; missing = 1
	Heat     float64            // sale-heat multiplier for units moved here
	Risk     float64            // robbery-chance multiplier
	Owner    string             // OwnerNone, OwnerPlayer, OwnerRival
	Runner   int                // crew id working it, You for the player, 0 nobody
	Enforcer int                // crew id guarding it, 0 nobody
	Since    int                // day the current owner took it
	Idle     int                // consecutive days held with nobody working it
	Squeeze  float64            // share of its demand a rival is undercutting away today, 0..1
}

// Share is the corner's share of the city's demand for a product, in
// standard corners, less what a rival is undercutting away.
func (c Corner) Share(product string) float64 {
	s := c.Demand
	if t, ok := c.Taste[product]; ok {
		s *= t
	}
	return s * (1 - c.Squeeze)
}

// Borders reports whether two corners are neighbours on the map.
func (c Corner) Borders(o Corner) bool {
	if c.ID == o.ID {
		return false
	}
	dx, dy := c.X-o.X, c.Y-o.Y
	return dx*dx+dy*dy == 1
}

// Held reports whether the player owns the corner.
func (c Corner) Held() bool { return c.Owner == OwnerPlayer }

// Worked reports whether the player owns the corner and somebody is on it
// selling: only worked corners serve demand.
func (c Corner) Worked() bool { return c.Held() && c.Runner != 0 }

// Corner returns the corner with id, or nil.
func (w *World) Corner(id string) *Corner {
	for i := range w.Territory.Corners {
		if w.Territory.Corners[i].ID == id {
			return &w.Territory.Corners[i]
		}
	}
	return nil
}

// RivalHeld counts the corners the rival owns.
func (w *World) RivalHeld() int {
	n := 0
	for _, c := range w.Territory.Corners {
		if c.Owner == OwnerRival {
			n++
		}
	}
	return n
}

// Contested reports whether a corner borders one the other side holds:
// a player corner next to a rival one, or the reverse.
func (w *World) Contested(c Corner) bool {
	var other string
	switch c.Owner {
	case OwnerPlayer:
		other = OwnerRival
	case OwnerRival:
		other = OwnerPlayer
	default:
		return false
	}
	for _, o := range w.Territory.Corners {
		if o.Owner == other && c.Borders(o) {
			return true
		}
	}
	return false
}

// Held counts the corners the player owns.
func (w *World) Held() int {
	n := 0
	for _, c := range w.Territory.Corners {
		if c.Held() {
			n++
		}
	}
	return n
}

// Worked counts the corners the player owns and has somebody on.
func (w *World) Worked() int {
	n := 0
	for _, c := range w.Territory.Corners {
		if c.Worked() {
			n++
		}
	}
	return n
}

// HeldShare is the demand share the player serves for a product: the sum
// over worked corners, in standard corners.
func (w *World) HeldShare(product string) float64 {
	s := 0.0
	for _, c := range w.Territory.Corners {
		if c.Worked() {
			s += c.Share(product)
		}
	}
	return s
}

// Demand is how many units of a product the street the player works
// absorbs today: the city's per-corner demand times the share held.
func (w *World) Demand(product string) float64 {
	m := w.Market[product]
	if m == nil {
		return 0
	}
	return m.Demand * w.HeldShare(product)
}

// PostOf returns the corner a crew member (or You) is posted on, or nil.
func (w *World) PostOf(id int) *Corner {
	if id == 0 {
		return nil
	}
	for i := range w.Territory.Corners {
		c := &w.Territory.Corners[i]
		if c.Runner == id || c.Enforcer == id {
			return c
		}
	}
	return nil
}

// Post puts a crew member, or You, on a corner: runners and You work it,
// enforcers guard it. A free corner is claimed; a rival's is refused.
// Somebody already posted elsewhere is moved, and whoever held that slot
// on the corner steps off.
func (w *World) Post(corner string, id int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	c := w.Corner(corner)
	if c == nil {
		return ErrNoCorner
	}
	if c.Owner == OwnerRival {
		return ErrCornerTaken
	}
	role := "runner"
	if id != You {
		m := w.Crew.Member(id)
		if m == nil {
			return ErrNoMember
		}
		role = m.Role
	}
	if role != "runner" && role != "enforcer" {
		return ErrNotPostable
	}
	w.Recall(id)
	if c.Owner != OwnerPlayer {
		c.Owner = OwnerPlayer
		c.Since = w.Day
	}
	if role == "enforcer" {
		c.Enforcer = id
	} else {
		c.Runner = id
		c.Idle = 0
	}
	return nil
}

// Recall takes a crew member, or You, off whatever corner they are on. The
// corner stays held until nobody has worked it for a few days.
func (w *World) Recall(id int) {
	c := w.PostOf(id)
	if c == nil {
		return
	}
	if c.Runner == id {
		c.Runner = 0
	}
	if c.Enforcer == id {
		c.Enforcer = 0
	}
}

// Abandon gives a held corner back to the street and recalls everyone on
// it.
func (w *World) Abandon(corner string) error {
	if w.Over != nil {
		return ErrGameOver
	}
	c := w.Corner(corner)
	if c == nil {
		return ErrNoCorner
	}
	if c.Owner != OwnerPlayer {
		return fmt.Errorf("you do not hold %s", c.Name)
	}
	c.Owner = OwnerNone
	c.Runner, c.Enforcer, c.Idle, c.Since = 0, 0, 0, w.Day
	return nil
}

// SendEnforcers queues the crew's enforcers against a rival corner at a
// force; the rival sim resolves it at end of day. One strike a day: sending
// again replaces it.
func (w *World) SendEnforcers(corner string, force events.Force) error {
	if w.Over != nil {
		return ErrGameOver
	}
	c := w.Corner(corner)
	if c == nil {
		return ErrNoCorner
	}
	if c.Owner != OwnerRival {
		return fmt.Errorf("%s is not the rival's", c.Name)
	}
	if w.Crew.Role("enforcer") == 0 {
		return ErrNoEnforcers
	}
	w.Strike = &StrikeOrder{Corner: corner, Force: force}
	return nil
}

// CallOff cancels tonight's strike.
func (w *World) CallOff() { w.Strike = nil }
