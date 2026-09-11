package market

import (
	"fmt"
	"math"
	"strings"
	"text/template"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// buyer is a deck entry with its pitch parsed once.
type buyer struct {
	cfg   content.BuyerConfig
	pitch *template.Template
}

// parseBuyers parses every buyer's pitch, so a bad template fails at
// construction, not the first time the buyer comes looking.
func parseBuyers(cfg content.BuyersConfig) ([]buyer, error) {
	var deck []buyer
	for _, c := range cfg.Deck {
		t, err := template.New(c.ID + ".pitch").Parse(c.Pitch)
		if err != nil {
			return nil, fmt.Errorf("buyer %s pitch: %w", c.ID, err)
		}
		deck = append(deck, buyer{cfg: c, pitch: t})
	}
	return deck, nil
}

// Buyers lists the buyers the sim knows, in file order.
func (s *Sim) Buyers() []content.BuyerConfig {
	out := make([]content.BuyerConfig, 0, len(s.buyers))
	for _, b := range s.buyers {
		out = append(out, b.cfg)
	}
	return out
}

// BuyersTuning exposes the deck's pacing and penalties, for the UI and
// the harness.
func (s *Sim) BuyersTuning() content.BuyersTuning { return s.bcfg.Buyers }

// ContractPrice is what a contract pays per unit if it is handed over
// today: the street price in its city times the premium. The market
// screen shows it and the handoff pays it.
func (s *Sim) ContractPrice(w *game.World, c game.Contract) float64 {
	if m := w.Product(c.City, c.Product); m != nil {
		return m.Price * c.Premium
	}
	return 0
}

// BuyerEligible reports whether a buyer could come looking today: not
// blacklisted, not spent if they come once, past their unlock, and their
// trigger, if they have one, holding. It is the dilemma deck's check
// (news.Eligible) over the same trigger vocabulary, so a buyer who wants
// three corners or a front is gated the way a card is.
func (s *Sim) BuyerEligible(w *game.World, b content.BuyerConfig, day int) bool {
	if w.Stats.PeakCash < b.UnlockCash {
		return false
	}
	if until, ok := w.Buyers.Blacklist[b.ID]; ok && day < until {
		return false
	}
	if b.Once && w.Buyers.Drawn[b.ID] > 0 {
		return false
	}
	if b.Trigger.Set() {
		if _, ok := news.Eligible(w, content.CardConfig{Trigger: b.Trigger}); !ok {
			return false
		}
	}
	return true
}

// wants lists the (city, product) pairs a buyer would deal in today, in
// city then ladder order: every city they work, every product listed
// there that they take.
func (s *Sim) wants(w *game.World, b content.BuyerConfig) (cities []string, products map[string][]string) {
	products = map[string][]string{}
	for _, cid := range w.CityOrder {
		if !b.In(cid) {
			continue
		}
		for _, id := range w.Products {
			pc := s.cfg.Product(id)
			if pc == nil || w.Product(cid, id) == nil || !b.Wants(*pc) {
				continue
			}
			products[cid] = append(products[cid], id)
		}
		if len(products[cid]) > 0 {
			cities = append(cities, cid)
		}
	}
	return cities, products
}

// deal puts at most one offer on the table: none while one is open or
// for MinGap days after the last, then a chance rising each day so one
// is certain by MaxGap, if any buyer is eligible. Every roll comes off
// the buyers' side stream, one a day whatever happens, so the deck never
// moves the home city's dice and a run with the deck boxed draws nothing.
func (s *Sim) deal(w *game.World, t *game.Tick) {
	if len(s.buyers) == 0 {
		return
	}
	pace := s.bcfg.Buyers
	rng := t.Sub("buyers")
	roll := rng.Float64()
	since := t.Day - w.Buyers.LastOffer
	if w.OpenOffer() || since < pace.MinGap {
		return
	}
	if roll >= float64(since-pace.MinGap+1)/float64(pace.MaxGap-pace.MinGap+1) {
		return
	}
	// Weighted pick over who is eligible, ones already seen thinned so the
	// deck does not repeat while it still has fresh faces.
	type pick struct {
		b        *buyer
		cities   []string
		products map[string][]string
		w        float64
	}
	var picks []pick
	total := 0.0
	for i := range s.buyers {
		b := &s.buyers[i]
		if !s.BuyerEligible(w, b.cfg, t.Day) {
			continue
		}
		cities, products := s.wants(w, b.cfg)
		if len(cities) == 0 {
			continue
		}
		wt := b.cfg.Weight
		if wt <= 0 {
			wt = 1
		}
		wt /= float64(1 + w.Buyers.Drawn[b.cfg.ID])
		picks = append(picks, pick{b, cities, products, wt})
		total += wt
	}
	if len(picks) == 0 {
		return
	}
	r := rng.Float64() * total
	chosen := picks[len(picks)-1]
	for _, p := range picks {
		if r < p.w {
			chosen = p
			break
		}
		r -= p.w
	}
	b := chosen.b.cfg
	city := chosen.cities[0]
	if len(chosen.cities) > 1 {
		city = chosen.cities[rng.IntN(len(chosen.cities))]
	}
	ids := chosen.products[city]
	product := ids[0]
	if len(ids) > 1 {
		product = ids[rng.IntN(len(ids))]
	}
	pc := s.cfg.Product(product)
	m := w.Product(city, product)
	// A day of the product's demand there: what the corners you work
	// serve, and never less than one standard corner's, so a buyer scales
	// with the operation and a cornerless player still gets an order.
	base := pc.Demand * s.cityProduct(city, product).Demand * math.Max(1, w.HeldShare(city, product))
	size := b.Size[0] + rng.Float64()*(b.Size[1]-b.Size[0])
	days := b.Days[0] + rng.IntN(b.Days[1]-b.Days[0]+1)
	premium := b.Premium[0] + rng.Float64()*(b.Premium[1]-b.Premium[0])
	premium = math.Round(premium*20) / 20 // to the nearest 0.05, the way a price is named out loud
	units := max(1, int(math.Round(size*base)))
	c := game.Contract{
		Buyer:       b.ID,
		Name:        b.Name,
		City:        city,
		Product:     product,
		Units:       units,
		Premium:     premium,
		Penalty:     b.Penalty,
		PenaltyCash: b.PenaltyCash,
		HeatMul:     b.Heat,
		Street:      m.Price,
		Since:       t.Day,
		Due:         t.Day + days,
	}
	c.Expires = min(t.Day+pace.OfferDays-1, c.Due)
	c.Pitch = renderPitch(chosen.b.pitch, content.BuyerSlots{
		Name:    capitalize(b.Name),
		Units:   units,
		Product: w.ProductName(product),
		City:    w.CityName(city),
		Days:    days,
		Premium: fmt.Sprintf("%.3gx", premium),
	})
	c = w.OfferContract(c)
	t.Emit(events.ContractOffered{
		Day: t.Day, ID: c.ID, Buyer: c.Buyer, Name: c.Name, Pitch: c.Pitch, City: c.City, Product: c.Product,
		Units: c.Units, Premium: c.Premium, Due: c.Due, Expires: c.Expires,
	})
}

// deliver hands over what was queued against every live contract in a
// city, at today's opening street price times the premium, out of the
// stash there: a bulk handoff, not a day on a corner, so it consumes no
// demand, takes no dial and is not capped by a patrol. Lying low is
// everyone's day off. It runs before the street sales so a contract has
// first call on the stash.
func (s *Sim) deliver(w *game.World, t *game.Tick, city string) {
	if w.LieLow || len(w.Deliveries) == 0 {
		return
	}
	for i := range w.Contracts {
		c := &w.Contracts[i]
		if c.City != city || !c.Live(t.Day-1) {
			continue
		}
		m := w.Product(city, c.Product)
		units := min(w.Deliveries[c.ID], c.Owed(), w.Stock(city, c.Product))
		if m == nil || units <= 0 {
			continue
		}
		price := m.Price * c.Premium
		revenue := int(math.Round(price * float64(units)))
		w.Stash(city)[c.Product] -= units
		w.Player.DirtyCash += revenue
		w.Stats.TotalRevenue += revenue
		w.Stats.ContractUnits += units
		w.Stats.ContractCash += revenue
		c.Delivered += units
		c.Paid += revenue
		ev := events.ContractDelivered{
			Day: t.Day, ID: c.ID, Buyer: c.Buyer, Name: c.Name, City: city, Product: c.Product,
			Units: units, Owed: c.Owed(), Total: c.Units, Price: price, Street: m.Price, Signed: c.Street,
			Revenue: revenue, HeatMul: c.HeatMul,
		}
		if c.Owed() == 0 {
			c.Status = game.ContractDelivered
			c.Resolved = t.Day
			w.Stats.Contracts++
			ev.Complete = true
			ev.Respect = s.bcfg.Buyers.Respect
		}
		t.Emit(ev)
	}
}

// settle reports yesterday's acceptances, fails every contract short at
// its due day (the rest of its value is owed in cash, respect goes,
// notoriety comes, and the buyer stays away), lapses every offer past
// its day, and drops what has been off the books long enough. No dice.
func (s *Sim) settle(w *game.World, t *game.Tick) {
	pace := s.bcfg.Buyers
	kept := w.Contracts[:0]
	for i := range w.Contracts {
		c := w.Contracts[i]
		if c.Accepted == t.Day-1 {
			t.Emit(events.ContractAccepted{Day: t.Day, ID: c.ID, Name: c.Name, City: c.City, Product: c.Product, Units: c.Units, Due: c.Due})
		}
		switch {
		case c.Status == game.ContractAccepted && c.Due < t.Day:
			street := 0.0
			if m := w.Product(c.City, c.Product); m != nil {
				street = m.Price
			}
			cash := w.TakeCash(int(math.Round(c.PenaltyCash * float64(c.Owed()) * street)))
			c.Status = game.ContractFailed
			c.Resolved = t.Day
			w.Stats.ContractsFailed++
			if w.Buyers.Blacklist == nil {
				w.Buyers.Blacklist = map[string]int{}
			}
			w.Buyers.Blacklist[c.Buyer] = t.Day + pace.BlacklistDays
			t.Emit(events.ContractFailed{
				Day: t.Day, ID: c.ID, Buyer: c.Buyer, Name: c.Name, City: c.City, Product: c.Product,
				Units: c.Units, Delivered: c.Delivered, Cash: cash, Respect: c.Penalty, Notoriety: pace.NotorietyPenalty,
				Blacklisted: w.Buyers.Blacklist[c.Buyer],
			})
		case c.Status == game.ContractOffered && c.Expires < t.Day:
			c.Status = game.ContractExpired
			c.Resolved = t.Day
			t.Emit(events.ContractExpired{Day: t.Day, ID: c.ID, Name: c.Name, City: c.City, Product: c.Product, Units: c.Units})
		}
		if c.Done() && c.Resolved+pace.KeepDays < t.Day {
			continue
		}
		kept = append(kept, c)
	}
	w.Contracts = kept
	if len(w.Contracts) == 0 {
		w.Contracts = nil
	}
}

func renderPitch(t *template.Template, s content.BuyerSlots) string {
	var b strings.Builder
	if err := t.Execute(&b, s); err != nil {
		return t.Name()
	}
	return b.String()
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
