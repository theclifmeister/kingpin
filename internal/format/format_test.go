package format

import "testing"

func TestCash(t *testing.T) {
	cases := map[int]string{
		0: "$0", 500: "$500", 9_999: "$9,999", -2_500: "-$2,500",
		10_000: "$10K", 45_000: "$45K", 123_456: "$123K", 999_499: "$999K", 999_600: "$1.0M",
		1_234_567: "$1.2M", 12_345_678: "$12M", 3_400_000_000: "$3.4B", -1_500_000: "-$1.5M",
		1_234_567_890_123: "$1.2T",
	}
	for n, want := range cases {
		if got := Cash(n); got != want {
			t.Errorf("Cash(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestMoney(t *testing.T) {
	for n, want := range map[int]string{0: "$0", 999: "$999", 1000: "$1,000", 25_000: "$25,000", -25_000: "-$25,000", 1_234_567: "$1,234,567"} {
		if got := Money(n); got != want {
			t.Errorf("Money(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestPrice(t *testing.T) {
	for v, want := range map[float64]string{19.5: "$19.50", 999.99: "$999.99", 2500: "$2,500", 10000: "$10,000", 2168.6: "$2,169"} {
		if got := Price(v); got != want {
			t.Errorf("Price(%v) = %q, want %q", v, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	for _, c := range []struct {
		n    int
		want string
	}{{0, "0 corners"}, {1, "1 corner"}, {2, "2 corners"}} {
		if got := Plural(c.n, "corner"); got != c.want {
			t.Errorf("Plural(%d) = %q, want %q", c.n, got, c.want)
		}
	}
	// The irregulars the game counts, and a two-word noun on its last.
	for noun, want := range map[string]string{
		"day": "days", "front": "fronts", "offer": "offers", "run": "runs", "election": "elections",
		"enforcer": "enforcers", "accountant": "accountants", "route": "routes", "headline": "headlines",
		"unit": "units", "lot": "lots", "city": "cities", "more day": "more days",
		"person": "people", "box": "boxes", "bus": "buses", "key": "keys", "crew": "crew",
	} {
		if got := Plural(2, noun); got != "2 "+want {
			t.Errorf("Plural(2, %q) = %q, want %q", noun, got, "2 "+want)
		}
		if got := Plural(1, noun); got != "1 "+noun {
			t.Errorf("Plural(1, %q) = %q", noun, got)
		}
	}
	for noun, want := range map[string]string{"runner": "a runner", "enforcer": "an enforcer", "accountant": "an accountant"} {
		if got := A(noun); got != want {
			t.Errorf("A(%q) = %q, want %q", noun, got, want)
		}
	}
	if Arrow != "→" {
		t.Errorf("Arrow = %q", Arrow)
	}
}
