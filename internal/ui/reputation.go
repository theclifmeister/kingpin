package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// What your name buys (#233, docs/reputation.md): the reputation
// effects in words, from content.ReputationFX scaled by the axis as
// the sims read them (content.Cut and Scale, the same arithmetic
// rivals.Sim.ClaimPace, PushPace, market.Sim.SupplierRatio,
// crew.Sim.LoyaltyLoss, HireFee and heat.Sim.PersonalHeat, Floor do),
// one line an effect over zero; TestEffectsInWordsAgreeWithTheSims
// reads the pane's numbers against the sims'. The dashboard's pane
// carries the section and the rivals screen's header one line of it.

// nameEffect is one effect in words: the axis, its value and what it
// does now.
type nameEffect struct {
	axis  string
	value float64
	lines []string
}

// nameEffects are the effects over zero (an axis that rounds to at
// least 1), fear, respect, notoriety.
func (m *Model) nameEffects() []nameEffect {
	rep := m.w.Player.Reputation
	fx := m.cfg.Reputation.Effects
	pct := func(v float64) string { return fmt.Sprintf("%.0f%%", math.Round(v*100)) }
	cut := func(axis, full float64) string { return pct(1 - content.Cut(axis, full)) }
	var out []nameEffect
	if math.Round(rep.Fear) >= 1 {
		e := nameEffect{axis: "fear", value: rep.Fear}
		e.lines = append(e.lines, fmt.Sprintf("rivals set up on a free corner %s less often, push you %s less", cut(rep.Fear, fx.RivalClaimCut), cut(rep.Fear, fx.FearPushCut)))
		if fx.FearHeatFloor > 0 {
			e.lines = append(e.lines, fmt.Sprintf("heat never falls under %.0f", fx.FearHeatFloor*math.Min(1, rep.Fear/100)))
		}
		out = append(out, e)
	}
	if math.Round(rep.Respect) >= 1 {
		e := nameEffect{axis: "respect", value: rep.Respect}
		e.lines = append(e.lines, fmt.Sprintf("the connect knocks %s off, the crew lose loyalty %s slower", cut(rep.Respect, fx.RespectSupplierCut), cut(rep.Respect, fx.RespectLoyaltyCut)))
		out = append(out, e)
	}
	if math.Round(rep.Notoriety) >= 1 {
		e := nameEffect{axis: "notoriety", value: rep.Notoriety}
		e.lines = append(e.lines, fmt.Sprintf("hire fees %s off, every unit you move yourself draws %s more heat", cut(rep.Notoriety, fx.NotorietyHireCut), pct(content.Scale(rep.Notoriety, fx.NotorietyHeat)-1)))
		out = append(out, e)
	}
	return out
}

// nameSection is the pane's YOUR NAME section: an axis a row, its
// effects wrapped under it; nil with nothing over zero.
func (m *Model) nameSection() []section {
	effects := m.nameEffects()
	if len(effects) == 0 {
		return nil
	}
	var lines []string
	for _, e := range effects {
		lines = append(lines, theme.Bold.Render(fmt.Sprintf("%s %.0f", e.axis, e.value)))
		for _, l := range e.lines {
			lines = append(lines, wrapWidth(l, m.textW()-2, "  ")...)
		}
	}
	return []section{{"YOUR NAME", lines}}
}

// nameLine is the rivals screen's one line of it: `your name: fear 62,
// they push 37% less · notoriety 70`, "" with nothing over zero.
func (m *Model) nameLine() string {
	rep := m.w.Player.Reputation
	fx := m.cfg.Reputation.Effects
	var parts []string
	if math.Round(rep.Fear) >= 1 {
		parts = append(parts, fmt.Sprintf("fear %.0f: they set up %.0f%% less, push %.0f%% less", rep.Fear, math.Round((1-content.Cut(rep.Fear, fx.RivalClaimCut))*100), math.Round((1-content.Cut(rep.Fear, fx.FearPushCut))*100)))
	}
	if math.Round(rep.Respect) >= 1 {
		parts = append(parts, fmt.Sprintf("respect %.0f: a kept deal earns %.0f%% more trust", rep.Respect, math.Round((content.Scale(rep.Respect, fx.RespectTrust)-1)*100)))
	}
	if math.Round(rep.Notoriety) >= 1 {
		parts = append(parts, fmt.Sprintf("notoriety %.0f", rep.Notoriety))
	}
	if len(parts) == 0 {
		return ""
	}
	return theme.Subtle.Render("your name: " + strings.Join(parts, " · "))
}

// wrapWidth wraps s to width with indent on every line.
func wrapWidth(s string, width int, indent string) []string {
	var out []string
	line := indent
	for _, word := range strings.Fields(s) {
		if line != indent && len(line)+1+len(word) > width {
			out = append(out, line)
			line = indent
		}
		if line != indent {
			line += " "
		}
		line += word
	}
	if line != indent {
		out = append(out, line)
	}
	return out
}
