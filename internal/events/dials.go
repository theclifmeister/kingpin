package events

// The dials (#275). Every dial is an int in a fixed order, saved as that
// int (a World field in JSON), and named by one table here that drives
// its String, the notches the UI draws and, for the two a config file
// names (the sell dial and the force), its Parse, so a dial's words are
// written once. None of them implements encoding.TextUnmarshaler or
// TextMarshaler: encoding/json would then refuse the ints every save
// holds (and write strings in their place), so the config files that name
// a dial keep a string field and read it through the Parse function.
//
// A value outside the table reads as the dial's default, the middle notch
// of a three-way dial and off for a route, as the hand-written switches
// always did.

// named is v's name in names, or names[def] for a value off the table.
func named[T ~int](names []string, v T, def int) string {
	if v < 0 || int(v) >= len(names) {
		return names[def]
	}
	return names[v]
}

// parse is the position of s in names, and whether it is there.
func parse[T ~int](names []string, s string) (T, bool) {
	for i, n := range names {
		if n == s {
			return T(i), true
		}
	}
	return 0, false
}

// Dial is the risk dial attached to a sell order: every action trades money
// against heat.
type Dial int

const (
	DialQuiet Dial = iota
	DialNormal
	DialAggressive
)

var dialNames = [...]string{"quiet", "normal", "aggressive"}

func (d Dial) String() string { return named(dialNames[:], d, int(DialNormal)) }

// DialNames are the sell dial's notches in order.
func DialNames() []string { return append([]string(nil), dialNames[:]...) }

// ParseDial is the sell dial named s, and whether s names one.
func ParseDial(s string) (Dial, bool) { return parse[Dial](dialNames[:], s) }

// Pay is the crew pay dial: every day the whole crew is paid stingy, fair or
// generous, trading cash against loyalty.
type Pay int

const (
	PayStingy Pay = iota
	PayFair
	PayGenerous
)

var payNames = [...]string{"stingy", "fair", "generous"}

func (p Pay) String() string { return named(payNames[:], p, int(PayFair)) }

// PayNames are the pay dial's notches in order.
func PayNames() []string { return append([]string(nil), payNames[:]...) }

// Force is the dial on a strike against a rival corner: how hard the
// enforcers go in. Harder flips corners faster and draws more heat.
type Force int

const (
	ForceWarn Force = iota
	ForcePush
	ForceHit
)

var forceNames = [...]string{"warn", "push", "hit"}

func (f Force) String() string { return named(forceNames[:], f, int(ForcePush)) }

// ForceNames are the force dial's notches in order.
func ForceNames() []string { return append([]string(nil), forceNames[:]...) }

// ParseForce is the force named s, and whether s names one.
func ParseForce(s string) (Force, bool) { return parse[Force](forceNames[:], s) }

// Launder is the laundering dial: how hard every front is pushed, trading
// throughput against audits.
type Launder int

const (
	LaunderCareful Launder = iota
	LaunderNormal
	LaunderGreedy
)

var launderNames = [...]string{"careful", "normal", "greedy"}

func (l Launder) String() string { return named(launderNames[:], l, int(LaunderNormal)) }

// LaunderNames are the launder dial's notches in order.
func LaunderNames() []string { return append([]string(nil), launderNames[:]...) }

// Ship is the shipping dial: how fast a shipment is pushed over its route,
// trading days in transit against the chance of a seizure on each.
type Ship int

const (
	ShipSlow Ship = iota
	ShipNormal
	ShipFast
)

var shipNames = [...]string{"slow", "normal", "fast"}

func (s Ship) String() string { return named(shipNames[:], s, int(ShipNormal)) }

// RouteDial is the route dial (#61): a persistent setting per route, off or
// the ship dial the logistics sim runs the route at every day. Off is the
// zero value, so a route nobody has touched runs nothing. It is the one
// four-way dial, and a value off its table reads as off, not the middle.
type RouteDial int

const (
	RouteOff RouteDial = iota
	RouteSlow
	RouteNormal
	RouteFast
)

var routeNames = [...]string{"off", "slow", "normal", "fast"}

func (r RouteDial) String() string { return named(routeNames[:], r, int(RouteOff)) }

// On reports whether the route runs at all.
func (r RouteDial) On() bool { return r > RouteOff && r <= RouteFast }

// Ship is the ship dial a running route sends at; normal for one that is
// off, which never sends.
func (r RouteDial) Ship() Ship {
	switch r {
	case RouteSlow:
		return ShipSlow
	case RouteFast:
		return ShipFast
	default:
		return ShipNormal
	}
}
