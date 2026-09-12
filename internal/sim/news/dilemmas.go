package news

import (
	"fmt"
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
		slots game.CardSlots
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
		sl, ok := game.Eligible(w, c.cfg)
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
		Member: sl.MemberID,
		Corner: sl.CornerID,
		Amount: sl.Sum,
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

func renderSlots(t *template.Template, s game.CardSlots) string {
	var b strings.Builder
	if err := t.Execute(&b, s); err != nil {
		return t.Name()
	}
	return b.String()
}
