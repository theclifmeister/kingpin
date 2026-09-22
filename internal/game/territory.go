package game

import (
	"errors"
	"fmt"
	"math"

	"github.com/theclifmeister/kingpin/internal/events"
)

// Owners of a corner.
const (
	OwnerNone   = "none"
	OwnerPlayer = "player"
	OwnerRival  = "rival"
)

// FactionRival is the id of the one faction there is until #43: the
// rival. RivalState.ID is seeded with it and a zero id (a save from
// before ids) resolves to it through RivalState.Faction, so an old save
// needs no migration.
const FactionRival = "rival"

// You is the worker id for the player standing on a corner in person.
const You = -1

var (
	ErrNoCorner    = errors.New("no such corner")
	ErrCornerTaken = errors.New("somebody else holds that corner")
	ErrNotPostable = errors.New("only runners and enforcers work corners")
	ErrNoEnforcers = errors.New("no enforcers on the payroll")
	ErrElsewhere   = errors.New("you are not in that city")
	ErrAtPeace     = errors.New("a truce, a tribute or a homage holds; a price war is not on while the peace is")
	ErrNotNextDoor = errors.New("you work no corner next to it")
	ErrDeeded      = errors.New("you hold the deed to that block already")
	ErrNoDeeds     = errors.New("nobody is selling the block")
)

// Corner is one block of a city: a demand pool the player has to hold to
// serve. Static tuning is copied in from content so the world never needs
// the city config to step. Corner ids are unique across cities.
type Corner struct {
	ID         string
	City       string // city id
	Name       string
	X, Y       int                // map cell
	Demand     float64            // size relative to one standard corner
	Taste      map[string]float64 // per-product demand multiplier; missing = 1
	Heat       float64            // sale-heat multiplier for units moved here
	Risk       float64            // robbery-chance multiplier
	Owner      string             // OwnerNone, OwnerPlayer, OwnerRival
	Faction    string             // which faction holds it while Owner is OwnerRival (RivalState.Faction, #144); "" otherwise. Every hand-over to the rival stamps it and every hand-over off it clears it.
	Runner     int                // crew id working it, You for the player, 0 nobody
	Enforcer   int                // crew id guarding it, 0 nobody
	Since      int                // day the current owner took it
	Idle       int                // consecutive days held with nobody working it
	Squeeze    float64            // share of its demand a rival is undercutting away today, 0..1
	Robbed     int                // stick-ups on your watch; a lieutenant gives up on a corner at two. Not the holder's: a hand-over keeps it, and the territory sim forgets it once the corner has been off your street for the drift days
	Yours      bool               // you have held it at some time; the rival's grace period leaves those alone (#60)
	Starved    int                // days a price war has cut the rival's trade here (#68), counted off Squeeze by the rivals sim, which answers at pricewar_days; a rest that long forgets them
	StarvedDay int                // the last day it was cut; 0 never
	Repeat     float64            // the share of its customers who come back (#47), 0..1: the market sim's, off what you sold here; zero reads as all of them, the pre-#47 corner
	Deed       *Deed              // the block bought with clean cash (#194); nil is the corner as it was, whoever holds it
}

// Deed is the block a corner is on, bought (#194): the day and what it
// cost in clean cash. Everything a deed does is read off the corner by
// the sim that owns the number (the territory sim's rent and robbery,
// the rival's push, the raid's weight, the law's pressure and the
// forfeiture), each through city.toml [deed]; the deed itself is two
// numbers.
type Deed struct {
	Bought int
	Price  int
}

// Deeded reports whether the block the corner is on is yours.
func (c Corner) Deeded() bool { return c.Deed != nil }

// Repeats is the corner's repeat business (#47), 0..1: Repeat, or all of
// it for a corner that has never been stamped.
func (c Corner) Repeats() float64 {
	if c.Repeat > 0 {
		return c.Repeat
	}
	return 1
}

// Share is the corner's share of the city's demand for a product, in
// standard corners, less what a rival is undercutting away and less
// the customers bad product has cost it (#47: Repeat; a corner that
// was never sold anything under the floor serves its whole share, so
// the number is what it was).
func (c Corner) Share(product string) float64 {
	s := c.Demand
	if t, ok := c.Taste[product]; ok {
		s *= t
	}
	s *= 1 - c.Squeeze
	if c.Repeat > 0 && c.Repeat < 1 {
		s *= c.Repeat
	}
	return s
}

// Full is the corner's share of the city's demand for a product with no
// squeeze on it: what a price war (#68) takes its cut of.
func (c Corner) Full(product string) float64 {
	s := c.Demand
	if t, ok := c.Taste[product]; ok {
		s *= t
	}
	return s
}

// Trade is the street value a corner moves in a day, its squeeze off,
// in the products the street of its city sells (#139: the port's
// product, no_supply there, comes by the road): what a rival corner
// earns before its margin, the till a boost takes from (#70) and the
// truth a spy's stash report is right about (#45). It lives here, the
// rivals sim's arithmetic and the crew sim's, because the two share it
// (game.Eligible's rule).
func (w *World) Trade(c Corner) float64 {
	v := 0.0
	for _, id := range w.Products {
		if m := w.Product(c.City, id); m != nil && !m.NoSupply {
			v += m.Demand * c.Share(id) * m.Price
		}
	}
	return v
}

// Fattest is the corner where a faction's till is fattest today: the
// one of its corners moving the most trade, nil for one holding none.
func (w *World) Fattest(faction string) *Corner {
	var best *Corner
	most := -1.0
	for _, cid := range w.CityOrder {
		cs := w.Cities[cid].Corners
		for i := range cs {
			c := &cs[i]
			if c.Owner != OwnerRival || c.FactionID() != faction {
				continue
			}
			if v := w.Trade(*c); v > most {
				most, best = v, c
			}
		}
	}
	return best
}

// Borders reports whether two corners are neighbours on the same map.
func (c Corner) Borders(o Corner) bool {
	if c.ID == o.ID || c.City != o.City {
		return false
	}
	dx, dy := c.X-o.X, c.Y-o.Y
	return dx*dx+dy*dy == 1
}

// Held reports whether the player owns the corner.
func (c Corner) Held() bool { return c.Owner == OwnerPlayer }

// FactionID is the faction holding the corner while Owner is OwnerRival
// (#43): Faction, or FactionRival for a corner stamped before ids (a
// save from before #144, a world built by hand), the way
// RivalState.Faction resolves a zero id; "" for a corner nobody's.
func (c Corner) FactionID() string {
	if c.Owner != OwnerRival {
		return ""
	}
	if c.Faction == "" {
		return FactionRival
	}
	return c.Faction
}

// Worked reports whether the player owns the corner and somebody is on it
// selling: only worked corners serve demand.
func (c Corner) Worked() bool { return c.Held() && c.Runner != 0 }

// Corners lists every corner of every city, in city order. The slice is
// fresh but the corners are copies: mutate through Corner.
func (w *World) Corners() []Corner {
	var out []Corner
	for _, cid := range w.CityOrder {
		out = append(out, w.Cities[cid].Corners...)
	}
	return out
}

// Corner returns the corner with id in any city, or nil.
func (w *World) Corner(id string) *Corner {
	for _, cid := range w.CityOrder {
		if c := find(w.Cities[cid].Corners, func(e *Corner) bool { return e.ID == id }); c != nil {
			return c
		}
	}
	return nil
}

// RivalHeld counts the corners every faction owns, in every city.
func (w *World) RivalHeld() int {
	n := 0
	for _, cid := range w.CityOrder {
		for _, c := range w.Cities[cid].Corners {
			if c.Owner == OwnerRival {
				n++
			}
		}
	}
	return n
}

// Held counts the corners the player owns, in every city.
func (w *World) Held() int {
	n := 0
	for _, c := range w.Corners() {
		if c.Held() {
			n++
		}
	}
	return n
}

// HeldIn counts the corners the player owns in one city.
func (w *World) HeldIn(city string) int {
	n := 0
	if c := w.Cities[city]; c != nil {
		for _, k := range c.Corners {
			if k.Held() {
				n++
			}
		}
	}
	return n
}

// Worked counts the corners the player owns and has somebody on, in every
// city.
func (w *World) Worked() int {
	n := 0
	for _, c := range w.Corners() {
		if c.Worked() {
			n++
		}
	}
	return n
}

// WorkedIn counts the corners the player works in one city.
func (w *World) WorkedIn(city string) int {
	n := 0
	if c := w.Cities[city]; c != nil {
		for _, k := range c.Corners {
			if k.Worked() {
				n++
			}
		}
	}
	return n
}

// HeldShare is the demand share the player serves for a product in a city:
// the sum over the corners worked there, in standard corners.
func (w *World) HeldShare(city, product string) float64 {
	s := 0.0
	if c := w.Cities[city]; c != nil {
		for _, k := range c.Corners {
			if k.Worked() {
				s += k.Share(product)
			}
		}
	}
	return s
}

// Demand is how many units of a product the street the player works in a
// city absorbs today: the city's per-corner demand times the share held.
func (w *World) Demand(city, product string) float64 {
	m := w.Product(city, product)
	if m == nil {
		return 0
	}
	return m.Demand * w.HeldShare(city, product)
}

// PostOf returns the corner a crew member (or You) is posted on, in any
// city, or nil.
func (w *World) PostOf(id int) *Corner {
	if id == 0 {
		return nil
	}
	for _, cid := range w.CityOrder {
		if c := find(w.Cities[cid].Corners, func(e *Corner) bool { return e.Runner == id || e.Enforcer == id }); c != nil {
			return c
		}
	}
	return nil
}

// Post puts a crew member, or You, on a corner: runners and You work it,
// enforcers guard it. A free corner is claimed; a rival's is refused; You
// can only stand on a corner of the city you are in (the crew go where
// they are sent). Somebody already posted elsewhere is moved, and whoever
// held that slot on the corner steps off.
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
	if id == You && c.City != w.Player.Location {
		return ErrElsewhere
	}
	role := RoleRunner
	if id != You {
		m := w.Crew.Member(id)
		if m == nil {
			return ErrNoMember
		}
		role = m.Role
		// A member in a cell or laid up (#46) works nothing until
		// they are back.
		if m.Jailed(w.Day) {
			return ErrJailed
		}
		if m.Wounded(w.Day) {
			return ErrWounded
		}
		if m.Undercover != "" {
			return ErrUndercover
		}
	}
	if role != RoleRunner && role != RoleEnforcer {
		return ErrNotPostable
	}
	if c.Owner != OwnerPlayer {
		for _, r := range w.Rivals {
			if d := w.DealWith(r.Faction(), DealSplit); d != nil && !d.Covers(c.ID) && w.Cities[c.City] == w.CityOf(r) {
				return fmt.Errorf("the split gives %s to %s; break it first", c.Name, r.Leader)
			}
		}
	}
	w.Recall(id)
	if c.Owner != OwnerPlayer {
		c.Owner, c.Faction = OwnerPlayer, ""
		c.Since = w.Day
	}
	if role == RoleEnforcer {
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
	if h := w.GuardOf(id); h != nil {
		h.Guard = 0 // one enforcer, one job (#73)
	}
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

// Hand gives the corner to owner (faction is the faction's id for
// OwnerRival, "" otherwise) on day, sending whoever worked it home
// (#281). Everything that belongs to the holder starts again: the runner
// and the enforcer, the idle count, the squeeze, the price war's starve
// count and Since. Robbed does not: it is your stick-ups on the corner,
// not the holder's, and the territory sim forgets it on its own clock.
// Every hand-over goes through here, so no two can disagree on what a
// new holder inherits.
func (c *Corner) Hand(owner, faction string, day int) {
	c.Owner, c.Faction = owner, faction
	c.Runner, c.Enforcer, c.Idle, c.Squeeze, c.Since = 0, 0, 0, 0, day
	c.Starved, c.StarvedDay = 0, 0
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
	c.Hand(OwnerNone, "", w.Day)
	w.Today.Abandoned = append(w.Today.Abandoned, c.ID)
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
		return fmt.Errorf("%s is not a rival's", c.Name)
	}
	if w.Crew.Role(RoleEnforcer) == 0 {
		return ErrNoEnforcers
	}
	w.Today.Strike = &StrikeOrder{Corner: corner, Force: force}
	return nil
}

// Boost queues the crew's enforcers against a rival corner for its
// takings rather than the ground (#70), at a force: the same odds as a
// strike and the same one order a night, so a boost queued replaces a
// strike and a strike a boost. The rivals sim resolves it.
func (w *World) Boost(corner string, force events.Force) error {
	if err := w.SendEnforcers(corner, force); err != nil {
		return err
	}
	w.Today.Strike.Boost = true
	return nil
}

// CallOff cancels tonight's strike, or boost.
func (w *World) CallOff() { w.Today.Strike = nil }

// NextDoor is the worked share you cut a rival corner from (#68): the
// demand, in standard corners, of your worked corners bordering it,
// capped at NextDoorMax. It is what scales the share a price war takes
// off it: one standard corner next door takes the full steal, two of
// your corners next to it cut deeper than one, and a corner surrounded
// is cut no deeper than the cap. Zero on a corner that is not the
// rival's or that none of your worked corners borders.
func (w *World) NextDoor(c Corner) float64 {
	city := w.Cities[c.City]
	if city == nil || c.Owner != OwnerRival {
		return 0
	}
	yours := 0.0
	for _, o := range city.Corners {
		if o.Worked() && c.Borders(o) {
			yours += o.Demand
		}
	}
	return math.Min(NextDoorMax, yours)
}

// NextDoorMax is the most standard corners' worth of your trade a price
// war counts next door to a rival corner: at the file's steal and the
// aggressive dial it keeps the share taken well under the whole.
const NextDoorMax = 2

// CanUndercut reports why a price war cannot be fought on a corner
// tonight, or nil: it must be the rival's and border a corner you work
// (you have to be next door to cut in on it, the mirror of Contested),
// and no deal may cover it: a truce or a tribute keeps you off every
// corner of theirs, a split off their side of the line, the way the
// rival's own undercutting stays off yours under a deal. The market sim
// asks again when it resolves the night, so an undercut queued in the
// morning and made illegal by the day (the runner recalled, an offer
// taken) moves nothing.
func (w *World) CanUndercut(corner string) error {
	c := w.Corner(corner)
	if c == nil {
		return ErrNoCorner
	}
	if c.Owner != OwnerRival {
		return fmt.Errorf("%s is not a rival's", c.Name)
	}
	if w.AtPeaceWith(c.Faction) {
		return ErrAtPeace
	}
	if d := w.DealWith(c.Faction, DealSplit); d != nil && !d.Covers(c.ID) {
		return fmt.Errorf("the split gives %s to %s; break it first", c.Name, w.FactionName(c.Faction))
	}
	if w.NextDoor(*c) <= 0 {
		return ErrNotNextDoor
	}
	return nil
}

// Undercut queues a price war on a rival corner for tonight (#68): the
// orders you place at home serve a share of its demand on top of your
// own corners', cheap, at the dial. The market sim resolves it; queuing
// again replaces the dial. It is business, not a betrayal: refused
// under a deal that covers the corner (CanUndercut) rather than
// breaking it.
func (w *World) Undercut(corner string, dial events.Dial) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if err := w.CanUndercut(corner); err != nil {
		return err
	}
	if w.Today.Undercuts == nil {
		w.Today.Undercuts = map[string]events.Dial{}
	}
	w.Today.Undercuts[corner] = dial
	return nil
}

// CancelUndercut calls off tonight's price war on a corner.
func (w *World) CancelUndercut(corner string) {
	delete(w.Today.Undercuts, corner)
	if len(w.Today.Undercuts) == 0 {
		w.Today.Undercuts = nil
	}
}

// Undercutting reports the dial a rival corner is undercut at tonight,
// and whether it is.
func (w *World) Undercutting(corner string) (events.Dial, bool) {
	d, ok := w.Today.Undercuts[corner]
	return d, ok
}

// CornerTrade is the street value a corner moves in a day (#194): every
// product the city sells at its price today, the corner's full share of
// the city's demand for it, summed over the ladder as unlocked. It is
// what a deed is days of.
func (w *World) CornerTrade(c Corner) float64 {
	v := 0.0
	for _, id := range w.Products {
		if m := w.Product(c.City, id); m != nil {
			v += m.Demand * c.Full(id) * m.Price
		}
	}
	return v
}

// BuyDeed buys the block a corner is on (#194) for price in clean cash,
// whoever holds the corner: a deed on a rival's block is legal, pays
// rent and cuts their defence of it. Clean cash only (ErrNoCleanCash,
// Fund's rule), one deed a block (ErrDeeded), and none where the file
// puts none on sale (ErrNoDeeds: a price of 0). The money goes at once;
// the territory sim reports the purchase tonight and pays the rent from
// tonight on.
func (w *World) BuyDeed(corner string, price int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	c := w.Corner(corner)
	if c == nil {
		return ErrNoCorner
	}
	if c.Deed != nil {
		return ErrDeeded
	}
	if price <= 0 {
		return ErrNoDeeds
	}
	if price > w.Player.CleanCash && w.Player.CleanCash <= 0 {
		return ErrNoCleanCash
	}
	if err := w.payClean(price); err != nil {
		return err
	}
	c.Deed = &Deed{Bought: w.Day, Price: price}
	w.Stats.Deeds++
	w.Stats.DeedCash += price
	w.Today.DeedsBought = append(w.Today.DeedsBought, c.ID)
	return nil
}

// SeizeDeed takes the deed off a corner and returns it: the forfeiture's
// (the law sim, #194). Nil for a corner with none.
func (w *World) SeizeDeed(corner string) *Deed {
	c := w.Corner(corner)
	if c == nil || c.Deed == nil {
		return nil
	}
	d := c.Deed
	c.Deed = nil
	w.Stats.DeedsSeized++
	return d
}

// Deeds lists every corner whose block is yours, in city order; the
// corners are copies.
func (w *World) Deeds() []Corner {
	var out []Corner
	for _, c := range w.Corners() {
		if c.Deed != nil {
			out = append(out, c)
		}
	}
	return out
}

// DeedsIn counts the blocks you hold the deed to in a city.
func (w *World) DeedsIn(city string) int {
	n := 0
	if c := w.Cities[city]; c != nil {
		for _, k := range c.Corners {
			if k.Deed != nil {
				n++
			}
		}
	}
	return n
}

// DeedValue is what the deeds you hold cost between them, clean: what
// the DA weighs against what the fronts have washed (the forfeiture).
func (w *World) DeedValue() int {
	v := 0
	for _, c := range w.Deeds() {
		v += c.Deed.Price
	}
	return v
}

// NewestDeed is the corner whose deed was bought last, the last in city
// order of the ones bought that day; nil with none. It is what the
// forfeiture takes.
func (w *World) NewestDeed() *Corner {
	var newest *Corner
	for _, cid := range w.CityOrder {
		cs := w.Cities[cid].Corners
		for i := range cs {
			if cs[i].Deed != nil && (newest == nil || cs[i].Deed.Bought >= newest.Deed.Bought) {
				newest = &cs[i]
			}
		}
	}
	return newest
}
