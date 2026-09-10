package game

import (
	"errors"
	"fmt"
)

// LawState is the law's actors (#41): the police chief and the district
// attorney. Public pressure and goodwill are per city (City.Pressure,
// City.Goodwill). The law sim owns all of it; the heat sim and the rival
// sim read it through their own tuning.
type LawState struct {
	Chief Chief
	DA    DA
}

// Chief is the police chief: a name, a personality (corrupt, zealous,
// lazy) that the heat sim multiplies its tuning by, and the day they took
// office. Observed says the player has seen enough of them for the report
// to name the personality.
type Chief struct {
	Name        string
	Personality string
	Since       int
	Observed    bool
}

// DA is the district attorney: a name, the ticket they ran on
// (law_and_order, moderate, reform) and the day they were elected. The
// stance does not move between elections.
type DA struct {
	Name       string
	Stance     string
	ElectedDay int
}

// Funding is clean cash the player gave a city today, per-day scratch
// the law sim turns into goodwill and reports.
type Funding struct {
	City   string
	Amount int
}

var (
	// ErrNoCleanCash means the player tried to fund a city with money the
	// fronts have not washed yet: goodwill is only ever bought clean.
	ErrNoCleanCash = errors.New("goodwill is bought with clean cash only")
)

// Fund gives a city amount in clean cash, at once: a community centre, a
// councillor's campaign, a police benevolent fund. Only clean cash pays;
// dirty cash is refused however much of it there is. The law sim turns
// it into goodwill at end of day and reports it.
func (w *World) Fund(city string, amount int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Cities[city] == nil {
		return ErrNoCity
	}
	if amount <= 0 {
		return ErrBadQuantity
	}
	if amount > w.Player.CleanCash {
		if w.Player.CleanCash <= 0 {
			return ErrNoCleanCash
		}
		return fmt.Errorf("need $%d clean, only have $%d clean", amount, w.Player.CleanCash)
	}
	w.Player.CleanCash -= amount
	w.Stats.Funded += amount
	w.Funded = append(w.Funded, Funding{City: city, Amount: amount})
	return nil
}

// FundedToday is what the player has given a city today, in clean cash.
func (w *World) FundedToday(city string) int {
	n := 0
	for _, f := range w.Funded {
		if f.City == city {
			n += f.Amount
		}
	}
	return n
}

// MeanPressure is the average pressure across the cities: what an
// election is decided on.
func (w *World) MeanPressure() float64 {
	if len(w.CityOrder) == 0 {
		return 0
	}
	sum := 0.0
	for _, cid := range w.CityOrder {
		sum += w.Cities[cid].Pressure
	}
	return sum / float64(len(w.CityOrder))
}
