// Package heat simulates law-enforcement pressure. Heat rises with what the
// player sold today and how loudly, decays over time, and triggers
// escalating responses at thresholds.
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
}

// New builds a heat sim. It needs the market config for per-product and
// per-dial heat multipliers.
func New(cfg content.HeatConfig, market content.MarketConfig) *Sim {
	return &Sim{cfg: cfg, market: market}
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

// SaleHeat is the heat drawn by trying to move wanted units of a product at
// a dial. Heat follows volume: every unit is a transaction somebody could
// see, weighted by how much the product itself draws attention and how
// loud the dial is. Units the crew moves beyond what the player could
// serve alone count at a discount: they are on the corners, you are not.
// The UI's dial preview uses it too, so the estimate is always honest.
func (s *Sim) SaleHeat(w *game.World, product string, wanted int, dial events.Dial) float64 {
	tun := s.cfg.Heat
	m := w.Market[product]
	pc := s.market.Product(product)
	if m == nil || pc == nil || tun.StreetUnits <= 0 {
		return 0
	}
	fill := s.dialFill(dial)
	own := math.Min(float64(wanted), math.Round(m.Demand*fill))
	total := math.Min(float64(wanted), math.Round(m.Demand*w.Reach()*fill))
	attempted := own + (total-own)*tun.CrewHeat
	return tun.SaleHeat * attempted * pc.Heat / tun.StreetUnits * s.dialHeat(dial)
}

// SloppyHeat is the premium low-skill runners add for moving units today.
func (s *Sim) SloppyHeat(w *game.World, units int) float64 {
	return s.Sloppiness(w) * s.cfg.Heat.SloppyHeat * float64(units)
}

// Sloppiness sums how far each runner falls below the sloppy-skill line, as
// a fraction: a skill-0 runner counts 1, a skilled one 0.
func (s *Sim) Sloppiness(w *game.World) float64 {
	line := float64(s.cfg.Heat.SloppySkill)
	if line <= 0 {
		return 0
	}
	total := 0.0
	for _, m := range w.Crew.Members {
		if m.Role == "runner" && float64(m.Skill) < line {
			total += (line - float64(m.Skill)) / line
		}
	}
	return total
}

// Step applies today's heat sources, decays, then checks thresholds.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Heat
	h := &w.Heat
	from := h.Value
	var reasons []string

	// Sales from the market sim, earlier in this tick. Heat follows the
	// volume you tried to move at that dial, not what a patrol cap let
	// through: standing on a corner shouting is the exposure. Units the crew
	// moves beyond what you could serve alone count at a discount: they are
	// on the corners, you are not.
	units := 0
	for _, e := range t.Events() {
		ps, ok := e.(events.PlayerSold)
		if !ok || ps.Wanted == 0 {
			continue
		}
		add := s.SaleHeat(w, ps.Product, ps.Wanted, ps.Dial)
		h.Value += add
		units += ps.Sold
		reasons = append(reasons, fmt.Sprintf("moved %d %s %s (+%.1f)", ps.Sold, w.ProductName(ps.Product), ps.Dial, add))
	}

	// Sloppy runners get noticed: every unit moved with a low-skill crew
	// on the corners adds a premium.
	if add := s.SloppyHeat(w, units); add > 0 {
		h.Value += add
		reasons = append(reasons, fmt.Sprintf("sloppy crew (+%.1f)", add))
	}

	// Sitting on a pile of dirty cash is its own tell.
	if tun.DirtyCashThreshold > 0 && w.Player.DirtyCash > tun.DirtyCashThreshold {
		mult := float64(w.Player.DirtyCash-tun.DirtyCashThreshold) / float64(tun.DirtyCashThreshold)
		add := tun.DirtyCashHeat * mult
		h.Value += add
		reasons = append(reasons, fmt.Sprintf("dirty cash (+%.1f)", add))
	}

	// Decay.
	decay := tun.Decay
	if w.LieLow {
		decay *= tun.LieLowMultiplier
		t.Emit(events.LaidLow{Day: t.Day})
		reasons = append(reasons, "lay low")
	}
	h.Value -= h.Value * decay
	h.Value = math.Max(0, math.Min(100, h.Value))

	if h.SellCapDays > 0 {
		h.SellCapDays--
		if h.SellCapDays == 0 {
			h.SellCap = 0
		}
	}

	// Threshold responses, highest first, one per day.
	if h.LastResponse == nil {
		h.LastResponse = map[string]int{}
	}
	if h.Responses == nil {
		h.Responses = map[string]int{}
	}
	resp := s.Thresholds()
	for i := len(resp) - 1; i >= 0; i-- {
		r := resp[i]
		if h.Value < r.Threshold {
			continue
		}
		if last, ok := h.LastResponse[r.Level]; ok && t.Day-last < tun.CooldownDays && r.Level != "arrest" {
			continue
		}
		h.Responses[r.Level]++
		s.fire(w, t, r)
		h.LastResponse[r.Level] = t.Day
		break
	}

	// Every sting and raid goes in a file. A thick enough file is a case.
	if w.Over == nil && tun.EvidenceArrest > 0 && h.Evidence >= tun.EvidenceArrest {
		w.Over = &game.Ending{Day: t.Day, Cause: "indicted", PeakCash: w.Stats.PeakCash}
		t.Emit(events.Enforcement{Day: t.Day, Level: "arrest", StockLost: map[string]int{}})
		t.Emit(events.GameOver{Day: t.Day, Cause: "indicted"})
	}

	if h.Value > h.Peak {
		h.Peak = h.Value
	}
	h.Value = math.Max(0, math.Min(100, h.Value))
	t.Emit(events.HeatChanged{Day: t.Day, From: from, To: h.Value, Reasons: reasons})
}

func (s *Sim) fire(w *game.World, t *game.Tick, r content.ResponseConfig) {
	ev := events.Enforcement{Day: t.Day, Level: r.Level, StockLost: map[string]int{}}
	switch r.Level {
	case "patrol":
		w.Heat.SellCapDays = r.CapDays
		w.Heat.SellCap = r.Cap
	case "arrest":
		w.Over = &game.Ending{Day: t.Day, Cause: "arrested", PeakCash: w.Stats.PeakCash}
		t.Emit(ev)
		t.Emit(events.GameOver{Day: t.Day, Cause: "arrested"})
		return
	default: // sting, raid
		for id, q := range w.Player.Stock {
			lost := int(math.Round(float64(q) * r.StockLoss))
			if lost > 0 {
				w.Player.Stock[id] -= lost
				ev.StockLost[id] = lost
			}
		}
		ev.CashLost = int(math.Round(float64(w.Player.DirtyCash) * r.CashLoss))
		w.Player.DirtyCash -= ev.CashLost
		if r.Level == "raid" {
			w.Stats.Raids++
		} else {
			w.Stats.Stings++
		}
	}
	w.Heat.Evidence += r.Evidence
	// The first raid convinces them they got you. The second one does not.
	// Each repeat of the same response cools things down less.
	n := float64(max(1, w.Heat.Responses[r.Level]))
	w.Heat.Value -= r.HeatDrop / n
	t.Emit(ev)
}
