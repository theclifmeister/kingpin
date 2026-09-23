// The dashboard's panels past the street: heat and its thresholds, cash,
// the law and the rival, each a block of lines the narrow and the wide
// layout stack (#275: out of dashboard.go).

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// otherHeat is the other cities' heat, `Bayport heat 1`, joined, or "".
func (m *Model) otherHeat() string {
	w := m.w
	var elsewhere []string
	for _, cid := range w.CityOrder {
		if c := w.Cities[cid]; c != w.Here() {
			elsewhere = append(elsewhere, theme.Subtle.Render(c.Name+" heat ")+heatStyle(c.Heat).Render(fmt.Sprintf("%.0f", c.Heat)))
		}
	}
	return strings.Join(elsewhere, sep)
}

// heatLines is the HEAT panel's content for a panel innerW cells wide:
// the gauge with the thresholds marked, the numbers, the thresholds on
// one line where they fit and two a line where they do not, and, wide,
// the three reputation bars on one line (narrow has four lines with
// the thresholds and leaves the bars to the wide layout).
func (m *Model) heatLines(innerW int, narrow bool) []string {
	w := m.w
	here := w.Here()
	var marks []float64
	var thr []string
	// The ladder as the player faces it (#48, heat.Sim.Ladder): the
	// task force's line is marked only once one can form, so a run
	// with no asset and a small pile reads the four rungs it always did.
	for _, r := range m.rules.Heat.Ladder(w, here) {
		marks = append(marks, r.Threshold/100)
		name := r.Level
		if r.Level == content.TaskForce {
			name = "task force"
		}
		thr = append(thr, fmt.Sprintf("%s %.0f", name, r.Threshold))
	}
	lines := []string{heatStyle(here.Heat).Render(sparkline.Bar(here.Heat/100, innerW, marks))}
	numbers := []string{heatStyle(here.Heat).Render(fmt.Sprintf("%.0f", here.Heat)) + theme.Subtle.Render("/100"), theme.Subtle.Render(fmt.Sprintf("peak %.0f", w.Heat.Peak))}
	if ev := m.rules.Heat.EvidenceArrest(w); ev > 0 {
		style := theme.Subtle
		if w.Heat.Evidence >= ev-2 {
			style = theme.Bad
		}
		numbers = append(numbers, style.Render(fmt.Sprintf("file %d/%d", w.Heat.Evidence, ev)))
	}
	if line := strings.Join(numbers, sep); lipgloss.Width(line) <= innerW {
		lines = append(lines, line)
	} else {
		lines = append(lines, strings.Join(numbers, " "))
	}
	lines = append(lines, thresholdLines(thr, innerW)...)
	if !narrow {
		lines = append(lines, m.reputationLine(innerW))
	}
	return lines
}

// thresholdLines lays the ladder's lines out under the gauge: on one
// line where they fit, else two a line. With the task force on the
// ladder (#48, five rungs) the two lines take three and two, or two and
// three, whichever fits the panel; where neither does the four rungs
// the panel always had are listed and the gauge's mark alone carries
// the fifth, so HEAT keeps its four lines at 80x24.
func thresholdLines(thr []string, innerW int) []string {
	fits := func(ls []string) bool {
		for _, l := range ls {
			if lipgloss.Width(l) > innerW {
				return false
			}
		}
		return true
	}
	join := func(parts []string) string { return strings.Join(parts, " · ") }
	if all := join(thr); lipgloss.Width(all) <= innerW {
		return []string{theme.Subtle.Render(all)}
	}
	var splits [][]string
	if len(thr) > 4 {
		for _, cut := range []int{3, 2} {
			splits = append(splits, []string{join(thr[:cut]), join(thr[cut:])})
		}
		var four []string
		for _, l := range thr {
			if !strings.HasPrefix(l, "task force") {
				four = append(four, l)
			}
		}
		thr = four
	}
	var pairs []string
	for i := 0; i < len(thr); i += 2 {
		pairs = append(pairs, join(thr[i:min(i+2, len(thr))]))
	}
	splits = append(splits, pairs)
	for _, ls := range splits {
		if fits(ls) {
			var out []string
			for _, l := range ls {
				out = append(out, theme.Subtle.Render(l))
			}
			return out
		}
	}
	var out []string
	for _, l := range pairs {
		out = append(out, theme.Subtle.Render(l))
	}
	return out
}

// exposure is the line past which the dirty pile draws heat
// (heat.Sim.ExposureLine: the threshold plus what the fronts cover) and
// whether the pile is past it. The dashboard's CASH panel and the
// ledger both warn off it, so the two screens cannot disagree (#350).
func (m *Model) exposure() (line int, past bool) {
	line = m.rules.Heat.ExposureLine(m.w)
	return line, line > 0 && m.w.Player.DirtyCash > line
}

// exposureWarning is the ledger's sentence for a pile past the line,
// naming the cover when the fronts give any; empty under the line.
func (m *Model) exposureWarning() string {
	line, past := m.exposure()
	if !past {
		return ""
	}
	if cover := m.rules.Heat.Cover(m.w); cover > 0 {
		return fmt.Sprintf("Dirty cash over %s draws heat every day: %s, plus %s your fronts cover.", cash(line), cash(line-cover), cash(cover))
	}
	return fmt.Sprintf("Dirty cash over %s draws heat every day it sits there.", cash(line))
}

// cashLines is the CASH panel's content, four lines: the dirty pile
// with the warning on its row once it is past what the fronts cover,
// the clean cash with the day's wash, the peak, and the other city's
// heat (the layout keeps HEAT to four lines by putting it here).
// Narrow has the pools alone, the wash as a total, and the warning as
// the last line, where the other city's heat goes without it.
func (m *Model) cashLines(innerW int, narrow bool) []string {
	w := m.w
	over := ""
	if line, past := m.exposure(); past {
		over = theme.Warning.Render(fmt.Sprintf("over %s: heat", cash(line)))
	}
	dirty := theme.Gold.Render("dirty  " + cash(w.Player.DirtyCash))
	clean := theme.Subtle.Render("clean  " + cash(w.Player.CleanCash))
	if len(w.Fronts) > 0 {
		wash := cash(m.rules.Laundering.Capacity(w))
		clean = firstFit(innerW, clean+theme.Subtle.Render(fmt.Sprintf("  +%s/day %s", wash, w.Laundering.Dial)), clean+theme.Subtle.Render(" +"+wash))
	}
	peak := theme.Subtle.Render("peak   " + cash(w.Stats.PeakCash))
	last := m.otherHeat()
	// The warning rides the dirty row where the row has the room, and
	// is the last line where it does not (narrow, or a middling width).
	if over != "" && !narrow && lipgloss.Width(dirty)+2+lipgloss.Width(over) <= innerW {
		dirty = fit(dirty, lipgloss.Width(dirty)+2) + over
		over = ""
	}
	if over != "" {
		last = over
	}
	return []string{dirty, clean, peak, last}
}

// bar is a labelled gauge: `war ████░░░░ 58/80 loud`.
func bar(label string, frac float64, after string) string {
	return label + " " + sparkline.Bar(frac, dashBarW, nil) + " " + after
}

// lawLines is the LAW panel's content, four lines: the chief and what
// they are like once you have seen them work with the days they have
// left, the DA and their ticket with the days to the election, the
// pressure where you are with what you have bought the city, and the
// other city's pressure. Narrow drops the days, shortens the ticket
// and folds the rival's line into the last row.
func (m *Model) lawLines(innerW int, narrow bool) []string {
	w := m.w
	l := w.Law
	here := w.Here()
	chief := "Chief " + l.Chief.Name + sep
	if word := m.chiefWord(); word != game.Unknown {
		chief += theme.Subtle.Render(word)
	} else {
		chief += theme.Subtle.Render("new")
	}
	// The favour (#228) before the days left: the debt is what you act
	// on, the term what you wait for.
	owes := ""
	if l.Favours > 0 && !w.Cold() {
		owes = sep + theme.Good.Render("owes one")
	}
	if end := m.rules.Law.ChiefTermEnds(w); end > 0 && !narrow {
		left := sep + theme.Subtle.Render(fmt.Sprintf("%dd left", max(0, end-w.Day)))
		chief = firstFit(innerW, chief+owes+left, chief+owes, chief+left, chief)
	} else {
		chief = firstFit(innerW, chief+owes, chief)
	}
	// The ticket is spelt out where the line has the room for it and
	// the election, and law-order where it does not; the election goes
	// before the ticket does.
	da := "DA " + l.DA.Name + sep
	long, short := stanceWord(l.DA.Stance), stanceWord(l.DA.Stance)
	if l.DA.Stance == "law_and_order" {
		short = "law-order"
	}
	election, tight := "", ""
	if next := m.rules.Law.NextElection(w); next > 0 && !narrow {
		election = sep + theme.Subtle.Render(fmt.Sprintf("election in %dd", max(0, next-w.Day)))
		tight = sep + theme.Subtle.Render(fmt.Sprintf("election %dd", max(0, next-w.Day)))
	}
	da = firstFit(innerW,
		da+theme.Subtle.Render(long)+election,
		da+theme.Subtle.Render(short)+election,
		da+theme.Subtle.Render(short)+tight,
		da+theme.Subtle.Render(long),
		da+theme.Subtle.Render(short))
	// The pressure bar, in heat's red, with the goodwill after it: what
	// you have bought the city is always shown once you have bought
	// some, the bar giving way to the number where the room is short.
	pressure := theme.Bad.Render(bar("pressure", here.Pressure/100, fmt.Sprintf("%.0f", here.Pressure)))
	number := theme.Bad.Render(fmt.Sprintf("pressure %.0f", here.Pressure))
	switch goodwill := theme.Good.Render(fmt.Sprintf("goodwill %.0f", here.Goodwill)); {
	case here.Goodwill > 0:
		pressure = firstFit(innerW, pressure+sep+goodwill, number+sep+goodwill, pressure)
	case !narrow:
		pressure = firstFit(innerW, pressure+sep+theme.Subtle.Render("goodwill 0"), pressure)
	}
	// A campaign you have money in (#193) sits under the election
	// countdown while the tickets take it: the ticket and the total,
	// today's included, the bar giving way to the number for it.
	if camp := w.Campaigning(here.ID); l.CampaignOpen && camp.Cash > 0 {
		backing := theme.Gold.Render("backing " + stanceWord(camp.Ticket) + " " + cash(camp.Cash))
		if camp.Hedged {
			backing = theme.Bad.Render("backing both " + cash(camp.Cash))
		}
		pressure = firstFit(innerW, pressure+sep+backing, number+sep+backing, pressure)
	}
	last := ""
	if narrow {
		last = m.rivalShort(innerW)
	} else {
		var others []string
		for _, cid := range w.CityOrder {
			if c := w.Cities[cid]; c != here {
				others = append(others, theme.Subtle.Render(fmt.Sprintf("%s pressure %.0f", c.Name, c.Pressure)))
			}
		}
		last = strings.Join(others, sep)
	}
	return []string{chief, da, pressure, last}
}

// rivalShort is the rival in one line for the narrow layout's LAW
// panel: who they are and what they hold.
func (m *Model) rivalShort(innerW int) string {
	w := m.w
	r := w.Rival()
	if r.Arrived == 0 && w.RivalHeld() == 0 {
		return theme.Subtle.Render("no rival yet")
	}
	leader := theme.RivalText.Render(r.Leader)
	short := leader + sep + theme.Subtle.Render(plural(w.RivalHeldBy(r.Faction()), "corner"))
	if eye := m.eyeingWord(r); eye != "" {
		return firstFit(innerW, short+sep+eye, leader+sep+eye, eye, short)
	}
	// The table (#43): how many more sit at it, where the line has room.
	if n := len(w.Rivals); n > 1 {
		more := theme.Subtle.Render(fmt.Sprintf("+%d more", n-1))
		return firstFit(innerW, short+sep+more, leader+sep+more, short)
	}
	return short
}

// rivalLines is the RIVALS panel's content, four lines: who they are,
// what they hold and what they are like, the war as a bar against the
// line the police crack down at, the trust as a bar, and whatever
// holds or waits at the table.
func (m *Model) rivalLines(innerW int) []string {
	w := m.w
	r := w.Rival()
	tun := m.rules.Rivals.Tuning()
	if r.Arrived == 0 && w.RivalHeld() == 0 {
		var ls []string
		for _, l := range wrap("Nobody is contesting the city. Yet.", innerW) {
			ls = append(ls, theme.Subtle.Render(l))
		}
		return ls
	}
	leader := theme.RivalText.Render(r.Leader)
	who := leader + sep + theme.Subtle.Render(plural(w.RivalHeldBy(r.Faction()), "corner"))
	temper := who + sep + theme.Subtle.Render(m.personalityWord(r))
	// The books (#70): the muscle as last read, where the line has room
	// for it after the rest.
	books := temper
	if word := m.muscleWord(r); word != game.Unknown {
		books = temper + sep + theme.Subtle.Render("muscle "+word+" "+m.muscleAge(r))
	}
	if eye := m.eyeingWord(r); eye != "" {
		// The tell (#69) outranks the temper, the count and the name
		// where the line has room for one of them: it needs you.
		who = firstFit(innerW, temper+sep+eye, who+sep+eye, leader+sep+eye, eye, temper, who)
	} else {
		who = firstFit(innerW, books, temper, who)
	}
	// The table (#43): the other factions and what they hold, each in
	// its colour, where the panel has a line for them; the war and the
	// trust lines are the rival at home's.
	var others []string
	for i, f := range w.Rivals {
		if i == 0 {
			continue
		}
		n := w.RivalHeldBy(f.Faction())
		word := fmt.Sprint(n)
		if f.Gone() {
			word = "gone"
		} else if f.Arrived == 0 {
			word = "-"
		}
		others = append(others, theme.FactionText(i).Render(f.Leader)+" "+theme.Subtle.Render(word))
	}
	war := bar("war", r.War/tun.CrackdownThreshold, fmt.Sprintf("%.0f/%.0f", r.War, tun.CrackdownThreshold))
	switch {
	case r.War >= tun.WarThreshold:
		war = theme.Bad.Render(war + " loud")
	case r.War > 0:
		war = theme.Warning.Render(war)
	default:
		war = theme.Subtle.Render(war)
	}
	trust := theme.Subtle.Render(bar("trust", r.Trust/100, fmt.Sprintf("%.0f", r.Trust)))
	var table string
	switch {
	case len(w.Offers) > 0:
		offers := plural(len(w.Offers), "offer")
		table = theme.Gold.Render(firstFit(innerW, offers+" "+screenPointer(screenRivals), offers+" waits"))
	case len(r.Deals) > 0:
		var ds []string
		for _, d := range r.Deals {
			if d.Until > 0 {
				ds = append(ds, fmt.Sprintf("%s %dd", d.Kind, d.Left(w.Day)))
			} else {
				ds = append(ds, string(d.Kind))
			}
		}
		table = theme.Good.Render(strings.Join(ds, ", "))
	case w.Today.Proposal != nil:
		table = theme.Gold.Render("proposal tonight")
	default:
		table = emptyState("Nothing on the table.")
	}
	if len(others) > 0 {
		// Four lines: the factions' line takes the trust's when there
		// is a table, the trust reading on the rivals screen.
		trust = truncate(strings.Join(others, sep), innerW)
	}
	return []string{who, war, trust, table}
}
