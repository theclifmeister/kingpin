package events

import "testing"

// TestDialsReadTheirTable (#275): every dial's String is its names
// table, a parse of a name is its position, and a value off the table
// reads as the default the hand-written switches gave, the middle of a
// three-way dial and off for a route.
func TestDialsReadTheirTable(t *testing.T) {
	for i, n := range DialNames() {
		if got := Dial(i).String(); got != n {
			t.Errorf("Dial(%d) = %q, want %q", i, got, n)
		}
		if d, ok := ParseDial(n); !ok || d != Dial(i) {
			t.Errorf("ParseDial(%q) = %v, %v", n, d, ok)
		}
	}
	for i, n := range ForceNames() {
		if f, ok := ParseForce(n); !ok || f != Force(i) || f.String() != n {
			t.Errorf("ParseForce(%q) = %v, %v", n, f, ok)
		}
	}
	for _, bad := range []string{"", "Quiet", "loud"} {
		if _, ok := ParseDial(bad); ok {
			t.Errorf("ParseDial(%q) parsed", bad)
		}
		if _, ok := ParseForce(bad); ok {
			t.Errorf("ParseForce(%q) parsed", bad)
		}
	}
	off := []struct{ got, want string }{
		{Dial(-1).String(), "normal"}, {Dial(3).String(), "normal"},
		{Pay(-1).String(), "fair"}, {Pay(3).String(), "fair"},
		{Force(-1).String(), "push"}, {Force(3).String(), "push"},
		{Launder(-1).String(), "normal"}, {Launder(3).String(), "normal"},
		{Ship(-1).String(), "normal"}, {Ship(3).String(), "normal"},
		{RouteDial(-1).String(), "off"}, {RouteDial(4).String(), "off"},
		{RouteFast.String(), "fast"}, {PayGenerous.String(), "generous"},
		{LaunderGreedy.String(), "greedy"}, {ShipSlow.String(), "slow"},
	}
	for i, c := range off {
		if c.got != c.want {
			t.Errorf("case %d: %q, want %q", i, c.got, c.want)
		}
	}
}
