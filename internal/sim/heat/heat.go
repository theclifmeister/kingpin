// Package heat simulates law-enforcement pressure. Every city has its own
// heat: it rises with what the player sold there today and how loudly,
// decays over time, and the hottest city's police answer at the
// thresholds. The case (the DA's file) and the response ladder are the
// player's, wherever they are.
package heat

import (
	"fmt"
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the heat simulation.
type Sim struct {
	cfg    content.HeatConfig
	market content.MarketConfig
	ship   content.ShippingTuning
	tree   content.UpgradesConfig
	rep    content.ReputationFX
	lt     content.LieutenantTuning
}

// New builds a heat sim. It needs the market config for per-product and
// per-dial heat multipliers, the shipping tuning for what a seizure on
// the road adds, the upgrade tree for what the Security and Legal
// branches take off, of the reputation effects the two that are its
// (fear puts a floor under heat, notoriety makes you the target), and
// the lieutenant tuning for what a temper does to a city's heat and what
// a flipped one feeds the DA.
func New(cfg content.HeatConfig, market content.MarketConfig, ship content.ShippingTuning, tree content.UpgradesConfig, rep content.ReputationFX, lt content.LieutenantTuning) *Sim {
	return &Sim{cfg: cfg, market: market, ship: ship, tree: tree, rep: rep, lt: lt}
}

// LieutenantHeat is what the temper of whoever runs a city does to the
// heat of every sale there: 1 for a city nobody runs.
func (s *Sim) LieutenantHeat(w *game.World, city string) float64 {
	if lt := w.Crew.Lieutenant(city); lt != nil {
		return s.lt.Temper(lt.Personality).Heat
	}
	return 1
}

// Floor is the heat a feared player never cools below: decay works on
// what is above it. A nobody's floor is zero.
func (s *Sim) Floor(w *game.World) float64 {
	return s.rep.FearHeatFloor * math.Max(0, math.Min(1, w.Player.Reputation.Fear/100))
}

// PersonalHeat is the weight of a unit you move yourself, relative to a
// unit a nobody moves: notoriety makes you the one they are watching.
func (s *Sim) PersonalHeat(w *game.World) float64 {
	return content.Scale(w.Player.Reputation.Notoriety, s.rep.NotorietyHeat)
}

// Effects is what the player's upgrades do to heat today.
func (s *Sim) Effects(w *game.World) game.Effects { return game.FoldEffects(w, s.tree) }

// EvidenceArrest is how thick the DA's file has to be for an indictment,
// after a retained lawyer has had his say.
func (s *Sim) EvidenceArrest(w *game.World) int {
	return max(s.cfg.Heat.EvidenceArrest, s.Effects(w).EvidenceArrest)
}

func (s *Sim) Name() string { return "heat" }

// Thresholds returns the response thresholds in ascending order, for the UI.
func (s *Sim) Thresholds() []content.ResponseConfig {
	out := append([]content.ResponseConfig(nil), s.cfg.Responses...)
	sort.Slice(out, func(i, j int) bool { return out[i].Threshold < out[j].Threshold })
	return out
}

func (s *Sim) dialHeat(d events.Dial) float64 {
	switch d {
	case events.DialQuiet:
		return s.market.Dial.Quiet.Heat
	case events.DialAggressive:
		return s.market.Dial.Aggressive.Heat
	default:
		return s.market.Dial.Normal.Heat
	}
}

func (s *Sim) dialFill(d events.Dial) float64 {
	switch d {
	case events.DialQuiet:
		return s.market.Dial.Quiet.Fill
	case events.DialAggressive:
		return s.market.Dial.Aggressive.Fill
	default:
		return s.market.Dial.Normal.Fill
	}
}

// SaleHeat is the heat drawn in a city by trying to move wanted units of
// a product there at a dial. Heat follows volume: every unit is a
// transaction somebody could see, weighted by how much the product itself
// draws attention, how loud the dial is, which corners it moves on
// (CornerWeight) and how closely the city's police look. The UI's dial
// preview uses it too, so the estimate is always honest.
func (s *Sim) SaleHeat(w *game.World, city, product string, wanted int, dial events.Dial) float64 {
	tun := s.cfg.Heat
	pc := s.market.Product(product)
	c := w.City(city)
	if pc == nil || c == nil || tun.StreetUnits <= 0 {
		return 0
	}
	fx := s.Effects(w)
	attempted := math.Min(float64(wanted), math.Round(w.Demand(city, product)*s.dialFill(dial)*fx.FillMul))
	return tun.SaleHeat * fx.SaleHeatMul * attempted * s.CornerWeight(w, city, product) * c.HeatMul * pc.Heat / tun.StreetUnits * s.dialHeat(dial) * s.LieutenantHeat(w, city)
}

// CornerWeight is the heat one unit of a product draws on average across
// the corners it moves on in a city, relative to a unit a nobody moves
// themselves on a standard corner. A sale spreads over the worked corners
// by their share; each corner has its own heat, a unit a runner moves
// counts at the crew discount (they are on the corner, you are not), and
// a unit you move yourself counts your notoriety.
func (s *Sim) CornerWeight(w *game.World, city, product string) float64 {
	total, weighted := 0.0, 0.0
	personal := s.PersonalHeat(w)
	c0 := w.City(city)
	if c0 == nil {
		return 0
	}
	for _, c := range c0.Corners {
		if !c.Worked() {
			continue
		}
		share := c.Share(product)
		total += share
		unit := c.Heat
		if c.Runner != game.You {
			unit *= s.cfg.Heat.CrewHeat
		} else {
			unit *= personal
		}
		weighted += share * unit
	}
	if total <= 0 {
		return 0
	}
	return weighted / total
}

// SloppyHeat is the premium low-skill runners add for moving units in a
// city today.
func (s *Sim) SloppyHeat(w *game.World, city string, units int) float64 {
	return s.Sloppiness(w, city) * s.cfg.Heat.SloppyHeat * float64(units)
}

// Sloppiness is how much of a city's volume moves through a sloppy
// runner's hands, as a fraction: each runner working a corner there
// counts how far they fall below the sloppy-skill line (a skill-0 runner
// 1, a skilled one 0) times that corner's share of the corners you work
// there. A runner without a corner is not on the street to be noticed.
func (s *Sim) Sloppiness(w *game.World, city string) float64 {
	line := float64(s.cfg.Heat.SloppySkill)
	c0 := w.City(city)
	if line <= 0 || c0 == nil {
		return 0
	}
	total, sloppy := 0.0, 0.0
	for _, c := range c0.Corners {
		if !c.Worked() {
			continue
		}
		total += c.Demand
		if c.Runner == game.You {
			continue
		}
		if m := w.Crew.Member(c.Runner); m != nil && float64(m.Skill) < line {
			sloppy += c.Demand * (line - float64(m.Skill)) / line
		}
	}
	if total <= 0 {
		return 0
	}
	return sloppy / total
}

// Step applies today's heat sources to every city, decays each, then has
// the hottest city's police check the thresholds.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Heat
	fx := s.Effects(w)
	h := &w.Heat
	here := w.Player.Location
	home := w.Home().ID
	from := map[string]float64{}
	reasons := map[string][]string{}
	units := map[string]int{}
	attempted := map[string]bool{}
	for _, cid := range w.CityOrder {
		from[cid] = w.Cities[cid].Heat
	}
	add := func(city string, v float64, why string) {
		c := w.Cities[city]
		if c == nil {
			return
		}
		c.Heat += v
		if why != "" {
			reasons[city] = append(reasons[city], fmt.Sprintf("%s (+%.1f)", why, v))
		}
	}

	// Sales from the market sim, earlier in this tick, city by city. Heat
	// follows the volume you tried to move at that dial, not what a
	// patrol cap let through: standing on a corner shouting is the
	// exposure. Which corners, and whether you or a runner stood there,
	// weight it.
	for _, e := range t.Events() {
		ps, ok := e.(events.PlayerSold)
		if !ok || ps.Wanted == 0 {
			continue
		}
		attempted[ps.City] = true
		why := fmt.Sprintf("moved %d %s %s", ps.Sold, w.ProductName(ps.Product), ps.Dial)
		if ps.Standing {
			why = fmt.Sprintf("%s moved %d %s %s", ps.LieutenantName, ps.Sold, w.ProductName(ps.Product), ps.Dial)
		}
		add(ps.City, s.SaleHeat(w, ps.City, ps.Product, ps.Wanted, ps.Dial), why)
		units[ps.City] += ps.Sold
	}

	// The war, from the rivals sim, in the rival's city: enforcers you
	// sent in, a rival's call to the precinct, the police clearing the
	// front line. A shipment seized on the road, from the logistics sim,
	// is heat in both cities it joined.
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CornerStruck:
			add(home, ev.Heat, fmt.Sprintf("enforcers %s %s", pastTense(ev.Force), ev.Name))
		case events.RivalTippedPolice:
			add(home, ev.Heat, "somebody tipped the police")
		case events.WarEscalated:
			if ev.Stage == "crackdown" {
				add(home, ev.Heat, "the crackdown")
			}
		case events.ShipmentSeized:
			why := fmt.Sprintf("%d %s seized on the %s", ev.Units, w.ProductName(ev.Product), ev.Mode)
			add(ev.From, s.ship.SeizureHeat, why)
			add(ev.To, s.ship.SeizureHeat, why)
			// Sent fast, it was asking to be looked at: a page in the
			// file (#27: a case is built from what you did). A seizure
			// is not a bust and never a reason to search the stash.
			if ev.Dial == events.ShipFast && s.ship.SeizureEvidence > 0 {
				h.Evidence += s.ship.SeizureEvidence
				h.EvidenceDay = t.Day
				reasons[ev.To] = append(reasons[ev.To], fmt.Sprintf("sent fast: the DA's file on you grows (%d)", h.Evidence))
			}
		}
	}

	// Sloppy runners get noticed: every unit moved with a low-skill crew
	// on the corners adds a premium.
	for _, cid := range w.CityOrder {
		if v := s.SloppyHeat(w, cid, units[cid]); v > 0 {
			add(cid, v, "sloppy crew")
		}
	}

	// An informant on the payroll. The crew sim's turn event starts the
	// clock (the laundering sim, stepping after this one, flips an
	// accountant without it: the clock then runs from today, the last day
	// nobody was talking); every informant_days after that the DA gets a
	// page whatever was sold, and the lawyer cannot thin a witness. The
	// heat it adds, where you are, is left out of the reasons on purpose:
	// a delta the dial does not explain, and a file that grew without a
	// bust, are the tells. Once nobody is talking the count that shows
	// them resets.
	// A flipped lieutenant is the same clock with thicker pages: they
	// know where everything is.
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CrewTurnedInformant:
			h.LeakDay = ev.Day
		case events.LieutenantFlipped:
			h.LeakDay = ev.Day
		}
	}
	if w.Crew.Informants() == 0 {
		h.Leaks = 0
		h.LeakDay = t.Day
	} else if tun.InformantDays > 0 && t.Day-h.LeakDay >= tun.InformantDays {
		h.LeakDay = t.Day
		h.Leaks++
		add(here, tun.InformantHeat, "")
		pages := tun.InformantEvidence
		for _, m := range w.Crew.Members {
			if m.Informant && m.Lieutenant() {
				pages = max(pages, s.lt.Evidence)
			}
		}
		if pages > 0 {
			h.Evidence += pages
			h.EvidenceDay = t.Day
			reasons[here] = append(reasons[here], fmt.Sprintf("the DA's file on you grows (%d)", h.Evidence))
		}
	}

	// Sitting on a pile of dirty cash is its own tell, wherever you sit.
	if tun.DirtyCashThreshold > 0 && w.Player.DirtyCash > tun.DirtyCashThreshold {
		mult := float64(w.Player.DirtyCash-tun.DirtyCashThreshold) / float64(tun.DirtyCashThreshold)
		add(here, tun.DirtyCashHeat*mult, "dirty cash")
	}

	// An audit at one of your fronts yesterday is a tell too: the books
	// were looked at. Only a front that was being run greedy gives the DA
	// something to file (#27: a case is built from what you did, not
	// what you have). Laundering steps after heat, so the front's record
	// is how yesterday's audit reaches today's heat.
	for _, f := range w.Fronts {
		if f.Audited == 0 || f.Audited != t.Day-1 {
			continue
		}
		if f.AuditDial == events.LaunderGreedy && tun.AuditEvidence > 0 {
			h.Evidence += tun.AuditEvidence
			add(here, tun.AuditHeat, fmt.Sprintf("audit at %s, run greedy: the DA's file grows", f.Name))
		} else {
			add(here, tun.AuditHeat, fmt.Sprintf("audit at %s", f.Name))
		}
	}

	// Decay, in every city. Cold contacts make both the base rate and
	// lying low better. A feared name never quite cools: decay works on
	// what is above the floor, and nothing takes heat under it.
	decay := math.Max(tun.Decay, fx.Decay)
	if w.LieLow {
		decay *= math.Max(tun.LieLowMultiplier, fx.LieLowMultiplier)
		t.Emit(events.LaidLow{Day: t.Day})
		for _, cid := range w.CityOrder {
			reasons[cid] = append(reasons[cid], "lay low")
		}
	}
	floor := s.Floor(w)
	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		if c.Heat > floor {
			c.Heat -= (c.Heat - floor) * decay
		}
		c.Heat = math.Max(floor, math.Min(100, c.Heat))
	}

	if h.SellCapDays > 0 {
		h.SellCapDays--
		if h.SellCapDays == 0 {
			h.SellCap = 0
		}
	}

	// Threshold responses, highest first, one per day, from the police
	// of the hottest city; what they take comes out of the stash there.
	if h.LastResponse == nil {
		h.LastResponse = map[string]int{}
	}
	if h.Responses == nil {
		h.Responses = map[string]int{}
	}
	hot := s.hottest(w)
	resp := s.Thresholds()
	for i := len(resp) - 1; i >= 0; i-- {
		r := resp[i]
		if hot.Heat < r.Threshold {
			continue
		}
		if last, ok := h.LastResponse[r.Level]; ok && t.Day-last < tun.CooldownDays+fx.CooldownBonus && r.Level != "arrest" {
			continue
		}
		h.Responses[r.Level]++
		s.fire(w, t, hot, r, attempted[hot.ID], fx)
		h.LastResponse[r.Level] = t.Day
		break
	}

	// A retained lawyer lets the file go cold: a page drops off after
	// enough days without a new one. Only dealing adds pages (#27), so
	// lying low is how a case is left to die.
	if w.Over == nil && fx.EvidenceDecayDays > 0 && h.Evidence > 0 && t.Day-h.EvidenceDay >= fx.EvidenceDecayDays {
		h.Evidence--
		h.EvidenceDay = t.Day
		reasons[here] = append(reasons[here], fmt.Sprintf("the case goes cold (file %d)", h.Evidence))
	}

	// Every sting and raid goes in a file. A thick enough file is a case.
	if arrest := s.EvidenceArrest(w); w.Over == nil && arrest > 0 && h.Evidence >= arrest {
		if !s.takeFall(w, t, fx) {
			w.Over = &game.Ending{Day: t.Day, Cause: "indicted", PeakCash: w.Stats.PeakCash}
			t.Emit(events.Enforcement{Day: t.Day, City: hot.ID, Level: "arrest", StockLost: map[string]int{}})
			t.Emit(events.GameOver{Day: t.Day, Cause: "indicted"})
		}
	}

	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		c.Heat = math.Max(floor, math.Min(100, c.Heat))
		if c.Heat > h.Peak {
			h.Peak = c.Heat
		}
		t.Emit(events.HeatChanged{Day: t.Day, City: cid, From: from[cid], To: c.Heat, Reasons: reasons[cid]})
	}
}

// hottest is the city whose police answer today: the hottest, and where
// the player is when it is a tie.
func (s *Sim) hottest(w *game.World) *game.City {
	best := w.Here()
	for _, cid := range w.CityOrder {
		if c := w.Cities[cid]; c.Heat > best.Heat {
			best = c
		}
	}
	return best
}

// fire applies one response in a city. attempted says whether the player
// tried to sell there today: a sting or raid that turns up on a day
// nothing moved still costs stock and cash and cools heat, but finds
// nothing worth a file. Dirty cash draws attention; only dealing builds a
// case. The Security branch softens what a response takes; a lawyer thins
// what goes in the file. A raid while an informant is on the payroll goes
// straight to the stash: every unit, whatever the safehouse would have
// saved.
func (s *Sim) fire(w *game.World, t *game.Tick, city *game.City, r content.ResponseConfig, attempted bool, fx game.Effects) {
	ev := events.Enforcement{Day: t.Day, City: city.ID, Level: r.Level, StockLost: map[string]int{}}
	switch r.Level {
	case "patrol":
		w.Heat.SellCapDays = r.CapDays
		w.Heat.SellCap = math.Max(r.Cap, fx.PatrolCap)
	case "arrest":
		if s.takeFall(w, t, fx) {
			return
		}
		w.Over = &game.Ending{Day: t.Day, Cause: "arrested", PeakCash: w.Stats.PeakCash}
		t.Emit(ev)
		t.Emit(events.GameOver{Day: t.Day, Cause: "arrested"})
		return
	default: // sting, raid
		stockLoss, cashLoss := r.StockLoss, r.CashLoss
		if r.Level == "raid" {
			stockLoss *= fx.RaidLossMul
			cashLoss *= fx.RaidLossMul
			if w.Crew.Informants() > 0 {
				stockLoss, ev.Stash = 1, true
			}
		} else {
			stockLoss *= fx.StingStockMul
		}
		stash := w.Stash(city.ID)
		for id, q := range stash {
			lost := int(math.Round(float64(q) * stockLoss))
			if lost > 0 {
				stash[id] -= lost
				ev.StockLost[id] = lost
			}
		}
		ev.CashLost = int(math.Round(float64(w.Player.DirtyCash) * cashLoss))
		w.Player.DirtyCash -= ev.CashLost
		if r.Level == "raid" {
			w.Stats.Raids++
		} else {
			w.Stats.Stings++
		}
	}
	if attempted {
		ev.Evidence = max(0, r.Evidence-fx.EvidenceCut)
		if ev.Evidence > 0 {
			w.Heat.Evidence += ev.Evidence
			w.Heat.EvidenceDay = t.Day
		}
	}
	// The first raid convinces them they got you. The second one does not.
	// Each repeat of the same response cools things down less.
	n := float64(max(1, w.Heat.Responses[r.Level]))
	city.Heat -= r.HeatDrop / n
	t.Emit(ev)
}

// pastTense is what the enforcers did, for the heat report.
func pastTense(f events.Force) string {
	switch f {
	case events.ForceWarn:
		return "warned"
	case events.ForceHit:
		return "hit"
	default:
		return "pushed"
	}
}

// takeFall is the fall guy's one job: if the player owns one and he has
// not been used, the case that would have ended the run closes on him
// instead. The file is wiped, heat drops to 50 everywhere and half of all
// cash goes on making it stick. It reports whether he took it.
func (s *Sim) takeFall(w *game.World, t *game.Tick, fx game.Effects) bool {
	if !fx.FallGuy || w.FallGuyUsed {
		return false
	}
	w.FallGuyUsed = true
	w.Heat.Evidence = 0
	w.Heat.EvidenceDay = t.Day
	for _, c := range w.Cities {
		c.Heat = math.Min(c.Heat, 50)
	}
	lost := w.Player.DirtyCash/2 + w.Player.CleanCash/2
	w.Player.DirtyCash -= w.Player.DirtyCash / 2
	w.Player.CleanCash -= w.Player.CleanCash / 2
	t.Emit(events.FallGuyBurned{Day: t.Day, CashLost: lost})
	return true
}
