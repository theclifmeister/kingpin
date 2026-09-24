package game

import "strings"

// The kinds of target an investigation names (#343): the source of a
// city's heat the police can put a name to. A corner is the street
// sales that moved on it, a product a buyer's handoffs of it, a house
// the stock moved into it.
const (
	LeadCorner  = "corner"
	LeadProduct = "product"
	LeadHouse   = "house"
)

// LeadKinds are the kinds in the order a tie between two leads is broken.
var LeadKinds = []string{LeadCorner, LeadProduct, LeadHouse}

// Investigation is the police working one named source of a city's heat
// (#343): opened the night the sting would have come, it lands on the
// target alone on the night Due. The zero value is none open.
type Investigation struct {
	City   string
	Kind   string // LeadCorner, LeadProduct or LeadHouse
	Target string // the corner's, the product's or the house's id
	Opened int    // the night it opened
	Due    int    // the night the hit comes
}

// Open reports whether an investigation is running.
func (i Investigation) Open() bool { return i.Due > 0 }

// DaysLeft is the nights before the hit as of the morning of day: 1 is
// tonight.
func (i Investigation) DaysLeft(day int) int { return max(0, i.Due-day) }

// LeadKey is how HeatState.Trail is keyed: city, kind and target.
func LeadKey(city, kind, target string) string { return city + "/" + kind + "/" + target }

// SplitLeadKey is a Trail key's city, kind and target.
func SplitLeadKey(key string) (city, kind, target string) {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) != 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}

// LeadName is what an investigation's target is called: the corner's,
// the product's or the house's name, or the id when it is gone.
func (w *World) LeadName(kind, target string) string {
	switch kind {
	case LeadCorner:
		if c := w.Corner(target); c != nil {
			return c.Name
		}
	case LeadProduct:
		return w.ProductName(target)
	case LeadHouse:
		if h := w.House(target); h != nil {
			return h.Name
		}
	}
	return target
}
