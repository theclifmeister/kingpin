package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The run summary (#49, docs/endings.md): what modeOver shows once the
// ending's scene is down, rendered off World and Stats and the endings
// file, no sim. The one modal, titled by the cause, scrolling at 80x24:
// the epilogue, the story (the run's top headlines by the weight of
// their source, in the order they happened), the money (the account
// marked as the score, the pile left behind), the people (the bodies
// and #46's fallen, the betrayals, the best of the crew), the city (the
// reputation bars, the ground, the law), the days and the score.

// viewOver is the GAME OVER modal: the ending's scene while it runs
// (#156), then the summary.
func (m *Model) viewOver() string {
	if m.scene != nil && !m.scene.Idle {
		return m.modal("GAME OVER", m.overFrame(), m.modalFooter())
	}
	e := m.w.Over
	return m.modal(fmt.Sprintf("%s · DAY %d", m.cfg.Endings.Title(e.Cause), e.Day), m.summaryLines(), m.modalFooter())
}

// summaryLines is the summary's body.
func (m *Model) summaryLines() []string {
	w := m.w
	e := w.Over
	won := m.cfg.Endings.Won(e.Cause)
	style := theme.Bad
	if won {
		style = theme.Good
	}
	var out []string
	out = append(out, cut(style.Bold(true).Render(m.cfg.Endings.Title(e.Cause))+theme.Subtle.Render(fmt.Sprintf("  day %d · reached %s", e.Day, m.reachedLine())+m.unlockedLine()), m.modalInner()))
	if epilogue := m.epilogue(); epilogue != "" {
		out = append(out, m.wrapLines(epilogue)...)
	}
	heading := func(s string) { out = append(out, "", theme.PanelTitle.Render(s)) }

	// The story: the headlines that weighed most, in the order they
	// happened, each with its day.
	if lines := m.storyLines(); len(lines) > 0 {
		heading("THE STORY")
		out = append(out, lines...)
	}

	// The money: the account is the score and marked so; the pile is
	// what the ending left behind, printed and never scored.
	heading("THE MONEY")
	money := [][2]string{
		{"offshore", cash(w.Offshore) + " · the score"},
		{"left behind", fmt.Sprintf("%s dirty · %s clean · %s in stock", cash(w.Player.DirtyCash), cash(w.Player.CleanCash), plural(w.TotalStock(), "unit"))},
		{"peak wealth", fmt.Sprintf("%s · revenue %s off %s", cash(w.Stats.PeakCash), cash(w.Stats.TotalRevenue), plural(w.Stats.UnitsSold, "unit"))},
		{"washed", fmt.Sprintf("%s, %s seized · lost %s wages · %s skimmed · %s robbed", cash(w.Stats.Laundered), cash(w.Stats.Seized), cash(w.Stats.Wages), cash(w.Stats.Skimmed), cash(w.Stats.Robbed))},
	}
	if s := w.Stats; s.Earned+s.Invested > 0 {
		money = append(money, [2]string{"the fronts", fmt.Sprintf("%s in levels, %s earned", cash(s.Invested), cash(s.Earned))})
	}
	out = append(out, m.factLines(money)...)

	// The people: the bodies and the fallen, the betrayals, the best of
	// the crew.
	heading("THE PEOPLE")
	people := [][2]string{{"bodies", m.bodiesLine()}}
	if n := len(w.Crew.Fallen); n > 0 {
		people = append(people, [2]string{"fallen", m.fallenLine()})
	}
	people = append(people, [2]string{"betrayals", m.betrayalsLine()})
	if best := m.bestCrew(); best != "" {
		people = append(people, [2]string{"best of them", best})
	}
	out = append(out, m.factLines(people)...)

	// The city: the reputation bars on one row, the ground and the law.
	heading("THE CITY")
	rep := w.Player.Reputation
	out = append(out, strings.Join([]string{
		bar("fear", rep.Fear/100, fmt.Sprintf("%.0f", rep.Fear)),
		bar("respect", rep.Respect/100, fmt.Sprintf("%.0f", rep.Respect)),
		bar("notoriety", rep.Notoriety/100, fmt.Sprintf("%.0f", rep.Notoriety)),
	}, "  "))
	city := [][2]string{
		{"ground", fmt.Sprintf("%s held · %d won · %d lost · %s · %s", plural(w.Held(), "corner"), w.Stats.CornersWon, w.Stats.CornersLost, plural(w.Stats.Stings, "sting"), plural(w.Stats.Raids, "raid"))},
		{"the law", fmt.Sprintf("Chief %s (%s) · DA %s (%s) · peak heat %.0f", truncate(w.Law.Chief.Name, 10), m.chiefWord(), truncate(w.Law.DA.Name, 10), stanceWord(w.Law.DA.Stance), w.Heat.Peak)},
	}
	out = append(out, m.factLines(city)...)

	// The score: the account over one plus the bodies, and the days,
	// shown and never scored.
	// The score's line ends with where the run stands against your own
	// history (#50, rankLine); the daily's date and what the run
	// unlocked are on the first line (unlockedLine). A line, not a
	// table.
	out = append(out, "", cut(theme.Gold.Bold(true).Render("SCORE  "+cash(w.Stats.Score))+theme.Subtle.Render(fmt.Sprintf("  %s over 1 + %s · %s · %s", cash(w.Offshore), plural(w.Stats.Bodies, "body"), plural(e.Day, "day"), m.rankLine())), m.modalInner()))
	return out
}

// factLines is a block of facts, one a row: the label in Subtle padded
// to the widest, the value after it cut to the modal's width.
func (m *Model) factLines(facts [][2]string) []string {
	width := 0
	for _, f := range facts {
		width = max(width, len(f[0]))
	}
	var out []string
	for _, f := range facts {
		out = append(out, cut(theme.Subtle.Render(fit(f[0], width))+"  "+f[1], m.modalInner()))
	}
	return out
}

// epilogue is the ending's epilogue from endings.toml, filled from the
// world as it stood; empty for a cause the file lacks, and the cause
// for one whose template will not render (the file is validated at
// load, so that is a field a template names that the data lacks).
func (m *Model) epilogue() string {
	w := m.w
	e := w.Over
	home := w.Home()
	fronts := plural(len(w.Fronts), "business")
	if len(w.Fronts) != 1 {
		fronts = fmt.Sprintf("%d businesses", len(w.Fronts))
	}
	d := content.Epilogue{
		Days:     e.Day,
		City:     home.Name,
		Here:     w.Here().Name,
		Offshore: money(w.Offshore),
		Left:     money(w.Cash()),
		Bodies:   w.Stats.Bodies,
		Fronts:   fronts,
		Corners:  w.Held(),
		Name:     e.Who,
		Leader:   e.Who,
		DA:       w.Law.DA.Name,
		Chief:    w.Law.Chief.Name,
		Pages:    max(1, w.Heat.Evidence),
		Years:    plural(m.cfg.Endings.Kingpin.Reign(home.Heat, home.Pressure), "year"),
		Reign:    plural(max(1, w.ReignDay()), "day"),
		Hot:      home.Heat > home.Pressure,
	}
	if e.Who == "" {
		d.Name, d.Leader = "Somebody", w.Rival().Leader
		if d.Leader == "" {
			d.Leader = "The rival"
		}
	}
	text, err := m.cfg.Endings.Render(e.Cause, d)
	if err != nil {
		return strings.ToUpper(e.Cause)
	}
	return text
}

// storyLines is the summary's timeline: the top [summary] lines
// headlines of the run by the weight of their source, the latest first
// among equals, shown in the order they happened as `day 42  text`.
func (m *Model) storyLines() []string {
	cfg := m.cfg.Endings.Summary
	type entry struct {
		h      game.Headline
		weight float64
		i      int
	}
	var picked []entry
	for i, h := range m.w.Journal {
		if wt := cfg.Weight[h.Source]; wt > 0 {
			picked = append(picked, entry{h, wt, i})
		}
	}
	sort.SliceStable(picked, func(a, b int) bool {
		if picked[a].weight != picked[b].weight {
			return picked[a].weight > picked[b].weight
		}
		return picked[a].i > picked[b].i
	})
	if len(picked) > cfg.Lines {
		picked = picked[:cfg.Lines]
	}
	sort.Slice(picked, func(a, b int) bool { return picked[a].i < picked[b].i })
	var out []string
	for _, p := range picked {
		day := theme.Subtle.Render(fmt.Sprintf("day %-3d", p.h.Day))
		out = append(out, day+" "+truncate(p.h.Text, m.modalInner()-8))
	}
	return out
}

// bodiesLine is the bodies on both sides and yours (#46).
func (m *Model) bodiesLine() string {
	s := m.w.Stats
	if s.Bodies == 0 {
		return "none"
	}
	return fmt.Sprintf("%d, %d of them yours", s.Bodies, s.Fallen)
}

// fallenLine names the fallen (#46) as `Name (role · day N)`, cut to
// the row.
func (m *Model) fallenLine() string {
	var names []string
	for _, f := range m.w.Crew.Fallen {
		names = append(names, fmt.Sprintf("%s (%s · day %d)", f.Name, f.Role, f.Day)) // `(role, dN)` would read as a key hint to TestNoKeyHintsOutsideTheLegend
	}
	return truncate(strings.Join(names, ", "), m.modalInner()-14)
}

// betrayalsLine counts who turned on you: the informants, the crew who
// went over, the lieutenants who walked, and the deals the factions
// broke; `none` with nobody.
func (m *Model) betrayalsLine() string {
	s := m.w.Stats
	var parts []string
	if s.Informants > 0 {
		parts = append(parts, plural(s.Informants, "informant"))
	}
	if s.Defections > 0 {
		parts = append(parts, plural(s.Defections, "defector"))
	}
	if s.Walked > 0 {
		parts = append(parts, fmt.Sprintf("%s walked", plural(s.Walked, "lieutenant")))
	}
	if s.BetrayedBy > 0 {
		parts = append(parts, fmt.Sprintf("%s broken by them", plural(s.BetrayedBy, "deal")))
	}
	if s.Betrayals > 0 {
		parts = append(parts, fmt.Sprintf("%s broken by you", plural(s.Betrayals, "deal")))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " · ")
}

// bestCrew is the best of the crew: the first of the fallen, if anyone
// fell for you, else the member whose tenure times loyalty is highest;
// empty with nobody.
func (m *Model) bestCrew() string {
	w := m.w
	if len(w.Crew.Fallen) > 0 {
		f := w.Crew.Fallen[0]
		return fmt.Sprintf("%s, %s, fell on day %d", f.Name, f.Role, f.Day)
	}
	best, score := (*game.CrewMember)(nil), -1.0
	for i := range w.Crew.Members {
		c := &w.Crew.Members[i]
		if v := float64(w.Day-c.Hired+1) * c.Loyalty; v > score {
			best, score = c, v
		}
	}
	if best == nil {
		return ""
	}
	return fmt.Sprintf("%s, %s, %s on the payroll at loyalty %.0f", best.Name, best.Role, plural(w.Day-best.Hired+1, "day"), best.Loyalty)
}

// reachedLine is the summary's tier row (#147): the highest tier
// reached and the day it was entered; the first tier is day 0 and
// reads as the name alone.
func (m *Model) reachedLine() string {
	w := m.w
	if w.Over != nil && w.Over.Cause == content.CauseKingpin && w.Reign > 0 {
		return fmt.Sprintf("Kingpin on day %d", w.Reign) // the reign's first morning (#227)
	}
	name := w.TierName(m.cfg.Progression)
	if d := w.ReachedOn(w.Tier()); d > 0 {
		return fmt.Sprintf("%s on day %d", name, d)
	}
	return name
}
