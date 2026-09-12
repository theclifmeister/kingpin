package news

import (
	"fmt"
	"math"
	"strings"
	"text/template"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// card is a deck entry with its templates parsed once.
type card struct {
	cfg     content.CardConfig
	title   *template.Template
	text    *template.Template
	choices []choiceTmpl
}

type choiceTmpl struct {
	label, outcome, headline *template.Template
}

// Slots are what a card's templates can name, filled from the world when
// it is drawn.
type Slots struct {
	Name    string // the crew member the card is about
	Role    string
	Corner  string // a corner of yours
	Theirs  string // the rival corner it borders, when contested
	Rival   string // the rival's leader
	City    string
	Front   string
	Product string // the product you hold most of
	Amount  string // the sum the card is about, formatted
	member  int
	corner  string // corner id
	amount  int
}

// parseDeck parses every card's templates and checks each effect key is
// one the world applies, so a typo in the deck fails at construction.
func parseDeck(cfg content.DilemmasConfig) ([]card, error) {
	var deck []card
	for _, c := range cfg.Cards {
		k := card{cfg: c}
		var err error
		if k.title, err = template.New(c.ID + ".title").Parse(c.Title); err != nil {
			return nil, fmt.Errorf("card %s title: %w", c.ID, err)
		}
		if k.text, err = template.New(c.ID + ".text").Parse(c.Text); err != nil {
			return nil, fmt.Errorf("card %s text: %w", c.ID, err)
		}
		for i, ch := range c.Choices {
			var t choiceTmpl
			if t.label, err = template.New(fmt.Sprintf("%s.%d.label", c.ID, i)).Parse(ch.Label); err != nil {
				return nil, fmt.Errorf("card %s choice %d label: %w", c.ID, i, err)
			}
			if t.outcome, err = template.New(fmt.Sprintf("%s.%d.outcome", c.ID, i)).Parse(ch.Outcome); err != nil {
				return nil, fmt.Errorf("card %s choice %d outcome: %w", c.ID, i, err)
			}
			if t.headline, err = template.New(fmt.Sprintf("%s.%d.headline", c.ID, i)).Parse(ch.Headline); err != nil {
				return nil, fmt.Errorf("card %s choice %d headline: %w", c.ID, i, err)
			}
			for key := range ch.Effects {
				if !game.KnownEffect(key) {
					return nil, fmt.Errorf("card %s choice %d: unknown effect %q", c.ID, i, key)
				}
			}
			if ch.Effects["loyalty"] != 0 && c.Trigger.Role == "" && c.Trigger.LoyaltyBelow == 0 && c.Trigger.LoyaltyAbove == 0 {
				return nil, fmt.Errorf("card %s choice %d: loyalty needs a trigger that names a member", c.ID, i)
			}
			k.choices = append(k.choices, t)
		}
		deck = append(deck, k)
	}
	return deck, nil
}

// Deck lists the cards the sim knows, in file order.
func (s *Sim) Deck() []content.CardConfig {
	out := make([]content.CardConfig, 0, len(s.deck))
	for _, c := range s.deck {
		out = append(out, c.cfg)
	}
	return out
}

// Eligible reports whether a card's trigger holds in w and, if so, the
// slots it names. Every set field must hold. Who fills a slot is fixed by
// the world, not the dice: the least loyal member under a loyalty line,
// the most loyal over one, the biggest corner, so the same world always
// names the same people.
func Eligible(w *game.World, c content.CardConfig) (Slots, bool) {
	t := c.Trigger
	s := Slots{City: w.Here().Name, Rival: w.Rival.Leader}
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
		var pick *game.CrewMember
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
		s.Name, s.Role, s.member = pick.Name, pick.Role, pick.ID
	}
	if t.Corners > 0 || t.Contested {
		var mine, theirs *game.Corner
		corners := w.Corners()
		for i := range corners {
			c := &corners[i]
			if !c.Worked() {
				continue
			}
			if t.Contested {
				var o *game.Corner
				for j := range corners {
					r := &corners[j]
					if r.Owner == game.OwnerRival && r.Borders(*c) && (o == nil || r.Demand > o.Demand) {
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
		s.Corner, s.corner = mine.Name, mine.ID
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
	s.amount = nice(max(c.Amount, int(c.AmountShare*float64(w.Player.DirtyCash))))
	s.Amount = dollarsInt(s.amount)
	return s, true
}

// drawCard puts at most one eligible card in front of the player: none
// while one waits for an answer or for MinGap days after the last, then a
// chance rising each day so one is certain by MaxGap, if any is eligible.
// One roll a day whatever happens keeps the RNG stream fixed for the seed.
func (s *Sim) drawCard(w *game.World, t *game.Tick) {
	pace := s.dcfg.Dilemmas
	since := t.Day - w.Dilemmas.LastCard
	roll := t.RNG.Float64()
	if len(s.deck) == 0 || w.Dilemmas.Pending != nil || since < pace.MinGap {
		return
	}
	if roll >= float64(since-pace.MinGap+1)/float64(pace.MaxGap-pace.MinGap+1) {
		return
	}
	if w.Dilemmas.Drawn == nil {
		w.Dilemmas.Drawn = map[string]int{}
	}
	// Weighted pick over what is eligible, cards already seen thinned so
	// the deck does not repeat while it still has fresh ones.
	type pick struct {
		card  *card
		slots Slots
		w     float64
	}
	var picks []pick
	total := 0.0
	for i := range s.deck {
		c := &s.deck[i]
		n := w.Dilemmas.Drawn[c.cfg.ID]
		if c.cfg.Once && n > 0 {
			continue
		}
		sl, ok := Eligible(w, c.cfg)
		if !ok {
			continue
		}
		wt := c.cfg.Weight
		if wt <= 0 {
			wt = 1
		}
		wt /= float64(1 + n)
		picks = append(picks, pick{c, sl, wt})
		total += wt
	}
	if len(picks) == 0 {
		return
	}
	r := t.RNG.Float64() * total
	chosen := picks[len(picks)-1]
	for _, p := range picks {
		if r < p.w {
			chosen = p
			break
		}
		r -= p.w
	}
	c, sl := chosen.card, chosen.slots
	pending := &game.Card{
		ID:     c.cfg.ID,
		Day:    t.Day,
		Title:  renderSlots(c.title, sl),
		Text:   renderSlots(c.text, sl),
		Member: sl.member,
		Corner: sl.corner,
		Amount: sl.amount,
	}
	for i, ch := range c.choices {
		pending.Choices = append(pending.Choices, game.Choice{
			Label:    renderSlots(ch.label, sl),
			Outcome:  renderSlots(ch.outcome, sl),
			Headline: renderSlots(ch.headline, sl),
			Effects:  c.cfg.Choices[i].Effects,
		})
	}
	w.Dilemmas.Pending = pending
	w.Dilemmas.LastCard = t.Day
	w.Dilemmas.Drawn[c.cfg.ID]++
	t.Emit(events.DilemmaDrawn{Day: t.Day, Card: c.cfg.ID, Title: pending.Title})
}

func renderSlots(t *template.Template, s Slots) string {
	var b strings.Builder
	if err := t.Execute(&b, s); err != nil {
		return t.Name()
	}
	return b.String()
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

// dollarsInt formats a whole-dollar sum with separators.
func dollarsInt(n int) string {
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return "$" + b.String()
}
