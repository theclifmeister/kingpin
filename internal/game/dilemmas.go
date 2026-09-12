package game

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/format"
)

var (
	ErrNoCard    = errors.New("no card to answer")
	ErrBadChoice = errors.New("no such choice")
)

// DilemmaState is the deck as it stands in this run: the card waiting for
// an answer, when the last one came up, and how often each has been drawn.
// Answered is per-day scratch the clock clears: the news sim reads it the
// next morning for the follow-up headline.
type DilemmaState struct {
	Pending  *Card          // drawn overnight; shown before the morning report
	LastCard int            // day the last card was drawn; paces the next
	Drawn    map[string]int // card id -> times drawn this run
	Answered *Answer
}

// Card is a dilemma as drawn: its text already rendered with the world's
// names in it, the choices and what each does. It is self-contained so a
// save on a card restores it and Choose needs no content to resolve it.
type Card struct {
	ID      string
	Day     int
	Title   string
	Text    string
	Choices []Choice
	Member  int    // crew id the card is about; 0 nobody
	Corner  string // corner id the card is about; "" none
	Amount  int    // the sum the card is about; 0 none
}

// Choice is one answer on a card. Effects are deltas keyed by name; the
// legal names are EffectKeys.
type Choice struct {
	Label    string
	Outcome  string             // what happened, for the journal
	Headline string             // optional follow-up, the next morning
	Effects  map[string]float64 // keyed by EffectKeys
}

// Answer is the choice the player made on a card, for the news sim to
// follow up on.
type Answer struct {
	Day      int
	Card     string
	Title    string
	Choice   string
	Outcome  string
	Headline string
}

// EffectKeys are the effects a choice may carry. Adding one is a code
// change here and in applyEffect; the news sim refuses a card whose
// choices use any other key, so a typo in the deck is a load failure, not
// a silent no-op. Cash deltas are clamped at zero, heat, loyalty and war at
// 0..100. The *_amount keys are multiples of the card's Amount, so a card
// can name the sum it is about and then take or pay it. stock_share is the
// fraction of every product lost (negative) or found (positive, capped by
// what the operation can hold). fear, respect and notoriety move the
// reputation axes (#14), clamped to 0..100; the sum cap is applied by the
// reputation sim at its next step.
var EffectKeys = []string{
	"dirty_cash", "clean_cash", "dirty_amount", "clean_amount",
	"heat",
	"loyalty", "crew_loyalty",
	"war", "grudge", "rival_muscle", "rival_cash",
	"stock_share",
	"fear", "respect", "notoriety",
}

// KnownEffect reports whether key is one of EffectKeys.
func KnownEffect(key string) bool {
	for _, k := range EffectKeys {
		if k == key {
			return true
		}
	}
	return false
}

// Choose answers the pending card with choice i: every effect applies at
// once, the outcome goes in the journal and the card is gone. It is the
// one place card effects touch the world.
func (w *World) Choose(i int) (Answer, error) {
	if w.Over != nil {
		return Answer{}, ErrGameOver
	}
	c := w.Dilemmas.Pending
	if c == nil {
		return Answer{}, ErrNoCard
	}
	if i < 0 || i >= len(c.Choices) {
		return Answer{}, ErrBadChoice
	}
	ch := c.Choices[i]
	// Sorted so clamps land the same way whatever order the map iterates.
	keys := make([]string, 0, len(ch.Effects))
	for k := range ch.Effects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := w.applyEffect(c, k, ch.Effects[k]); err != nil {
			return Answer{}, err
		}
	}
	a := Answer{Day: w.Day, Card: c.ID, Title: c.Title, Choice: ch.Label, Outcome: ch.Outcome, Headline: ch.Headline}
	w.Dilemmas.Pending = nil
	w.Dilemmas.Answered = &a
	w.Journal = append(w.Journal, Headline{Day: w.Day, Source: "dilemma", Text: ch.Outcome})
	return a, nil
}

// applyEffect is the one function that knows what every effect key does.
func (w *World) applyEffect(c *Card, key string, v float64) error {
	switch key {
	case "dirty_cash":
		w.Player.DirtyCash = max(0, w.Player.DirtyCash+int(v))
	case "clean_cash":
		w.Player.CleanCash = max(0, w.Player.CleanCash+int(v))
	case "dirty_amount":
		w.Player.DirtyCash = max(0, w.Player.DirtyCash+int(math.Round(v*float64(c.Amount))))
	case "clean_amount":
		w.Player.CleanCash = max(0, w.Player.CleanCash+int(math.Round(v*float64(c.Amount))))
	case "heat":
		// Where you are: the card is about tonight, and you are here.
		if c := w.Here(); c != nil {
			c.Heat = clamp(c.Heat + v)
			w.Heat.Peak = math.Max(w.Heat.Peak, c.Heat)
		}
	case "loyalty":
		if m := w.Crew.Member(c.Member); m != nil {
			m.Loyalty = clamp(m.Loyalty + v)
		}
	case "crew_loyalty":
		for i := range w.Crew.Members {
			w.Crew.Members[i].Loyalty = clamp(w.Crew.Members[i].Loyalty + v)
		}
	case "war":
		w.Rival.War = clamp(w.Rival.War + v)
	case "grudge":
		w.Rival.Grudge = max(0, w.Rival.Grudge+int(v))
	case "rival_muscle":
		w.Rival.Muscle = max(0, w.Rival.Muscle+int(v))
	case "rival_cash":
		w.Rival.Cash = max(0, w.Rival.Cash+int(v))
	case "stock_share":
		for _, cid := range w.CityOrder {
			free := w.Free(cid)
			for _, id := range w.Products {
				d := int(math.Round(float64(w.Stock(cid, id)) * v))
				if d > 0 {
					d = min(d, free)
					free -= d
				}
				w.AddStock(cid, id, d) // a negative share is a take, clamped at nothing
			}
		}
	case "fear", "respect", "notoriety":
		// The axis moves at once, clamped like the sim clamps it; the
		// street's total attention (the sum cap) is the reputation sim's
		// to enforce, and it does so on every source when it steps tonight.
		a := w.Player.Reputation.Axis(key)
		*a = clamp(*a + v)
	default:
		return fmt.Errorf("card %s: unknown effect %q", c.ID, key)
	}
	return nil
}

func clamp(v float64) float64 { return math.Max(0, math.Min(100, v)) }

// CardSlots are what a card's templates can name, filled from the world
// when it is drawn (Slots is the save slots' list). The three ids are
// what the drawn Card keeps beside its rendered text.
type CardSlots struct {
	Name     string // the crew member the card is about
	Role     string
	Corner   string // a corner of yours
	Theirs   string // the rival corner it borders, when contested
	Rival    string // the rival's leader
	City     string
	Front    string
	Product  string // the product you hold most of
	Amount   string // the sum the card is about, formatted
	MemberID int    // crew id the card is about; 0 nobody
	CornerID string // corner id the card is about; "" none
	Sum      int    // the sum the card is about, unformatted; 0 none
}

// Eligible reports whether a card's trigger holds in w and, if so, the
// slots it names: the one checker for the trigger vocabulary a dilemma
// card, a buyer (sim/market) and a progression tier (sim/news) share,
// a query over the world and content with no sim behind it, which is
// why it lives here beside FoldEffects and not in a sim (#144). Every
// set field must hold. Who fills a slot is fixed by the world, not the
// dice: the least loyal member under a loyalty line, the most loyal over
// one, the biggest corner, so the same world always names the same
// people.
func Eligible(w *World, c content.CardConfig) (CardSlots, bool) {
	t := c.Trigger
	s := CardSlots{City: w.Here().Name, Rival: w.Rival.Leader}
	if t.DayMin > 0 && w.Day < t.DayMin {
		return s, false
	}
	if t.DayMax > 0 && w.Day > t.DayMax {
		return s, false
	}
	// Heat where you are: the card finds you there.
	if w.HeatHere() < t.HeatMin {
		return s, false
	}
	if t.HeatMax > 0 && w.HeatHere() > t.HeatMax {
		return s, false
	}
	if w.Cash() < t.CashMin {
		return s, false
	}
	if t.StockMin > 0 {
		if w.Player.TotalStock() < t.StockMin {
			return s, false
		}
	}
	if len(w.Crew.Members) < t.CrewMin {
		return s, false
	}
	if t.Role != "" || t.LoyaltyBelow > 0 || t.LoyaltyAbove > 0 {
		var pick *CrewMember
		for i := range w.Crew.Members {
			m := &w.Crew.Members[i]
			if t.Role != "" && m.Role != t.Role {
				continue
			}
			if t.LoyaltyBelow > 0 && m.Loyalty >= t.LoyaltyBelow {
				continue
			}
			if t.LoyaltyAbove > 0 && m.Loyalty <= t.LoyaltyAbove {
				continue
			}
			switch {
			case pick == nil:
				pick = m
			case t.LoyaltyBelow > 0 && m.Loyalty < pick.Loyalty:
				pick = m
			case t.LoyaltyBelow == 0 && m.Loyalty > pick.Loyalty:
				pick = m
			}
		}
		if pick == nil {
			return s, false
		}
		s.Name, s.Role, s.MemberID = pick.Name, pick.Role, pick.ID
	}
	if t.Corners > 0 || t.Contested {
		var mine, theirs *Corner
		corners := w.Corners()
		for i := range corners {
			c := &corners[i]
			if !c.Worked() {
				continue
			}
			if t.Contested {
				var o *Corner
				for j := range corners {
					r := &corners[j]
					if r.Owner == OwnerRival && r.Borders(*c) && (o == nil || r.Demand > o.Demand) {
						o = r
					}
				}
				if o == nil {
					continue
				}
				if mine == nil || c.Demand > mine.Demand {
					mine, theirs = c, o
				}
				continue
			}
			if mine == nil || c.Demand > mine.Demand {
				mine = c
			}
		}
		if mine == nil || w.Worked() < t.Corners {
			return s, false
		}
		s.Corner, s.CornerID = mine.Name, mine.ID
		if theirs != nil {
			s.Theirs = theirs.Name
		}
	}
	if t.Rival && w.RivalHeld() == 0 {
		return s, false
	}
	if t.Personality != "" && (w.RivalHeld() == 0 || w.Rival.Personality != t.Personality) {
		return s, false
	}
	if t.WarMin > 0 && (w.RivalHeld() == 0 || w.Rival.War < t.WarMin) {
		return s, false
	}
	if t.Fronts {
		if len(w.Fronts) == 0 {
			return s, false
		}
		s.Front = w.Fronts[0].Name
	}
	// The progression's two (#147): the high-water mark every unlock
	// reads, and the lieutenant gate's count of cities with a held corner.
	if w.Stats.PeakCash < t.PeakCashMin {
		return s, false
	}
	if t.CitiesHeld > 0 && w.CitiesHeld() < t.CitiesHeld {
		return s, false
	}
	most := -1
	for _, id := range w.Products {
		q := 0
		for _, cid := range w.CityOrder {
			q += w.Stock(cid, id)
		}
		if q > most {
			most, s.Product = q, w.ProductName(id)
		}
	}
	s.Sum = nice(max(c.Amount, int(c.AmountShare*float64(w.Player.DirtyCash))))
	s.Amount = format.Money(s.Sum)
	return s, true
}

// nice rounds a sum to two significant figures, the way somebody names a
// price out loud: $2,347 is "twenty-three hundred".
func nice(n int) int {
	if n < 100 {
		return n
	}
	p := math.Pow(10, math.Floor(math.Log10(float64(n)))-1)
	return int(math.Round(float64(n)/p) * p)
}
