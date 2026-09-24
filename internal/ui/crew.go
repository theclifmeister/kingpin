package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// crewRows is the list the crew cursor moves over: the roster first, then
// the people looking for work.
func (m *Model) crewRows() []game.CrewMember {
	c := m.w.Crew
	rows := append([]game.CrewMember(nil), c.Members...)
	return append(rows, c.Candidates...)
}

// crewSelected returns the member under the cursor and whether they are on
// the payroll (as opposed to a candidate).
func (m *Model) crewSelected() (game.CrewMember, bool, bool) {
	rows := m.crewRows()
	if len(rows) == 0 {
		return game.CrewMember{}, false, false
	}
	return rows[clamp(&m.crewCursor, len(rows))], m.crewCursor < len(m.w.Crew.Members), true
}

func (m *Model) hireSelected() {
	c, onPayroll, ok := m.crewSelected()
	if !ok || onPayroll {
		m.refuse("Can't hire: put the cursor on somebody looking for work.")
		return
	}
	got, err := m.sess.Hire(c.ID)
	if err != nil {
		m.refuse("Can't hire: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("%s hired for %s.", got.Name, money(got.Fee)))
	m.crewCursor = len(m.w.Crew.Members) - 1
}

func (m *Model) askFire() {
	c, onPayroll, ok := m.crewSelected()
	if !ok || !onPayroll {
		m.refuse("Can't fire: put the cursor on somebody on the payroll.")
		return
	}
	m.subjectID = c.ID
	m.ask("fire", (*Model).fireConfirm, (*Model).confirmFire)
}

// fireConfirm is the confirmation's modal, naming who goes.
func (m *Model) fireConfirm() string {
	name := "them"
	if c := m.w.Crew.Member(m.subjectID); c != nil {
		name = c.Name
	}
	return m.modal("FIRE "+name+"?", []string{"No severance in this business. The rest of the crew", "will take it personally."}, m.modalFooter())
}

func (m *Model) confirmFire() {
	m.mode = modePlay
	got, err := m.sess.Fire(m.subjectID)
	if err != nil {
		m.refuse("Can't fire: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("%s is gone. The rest noticed.", got.Name))
}

// talking reports whether the report's tell has shown enough for the
// screens to hint at it: the DA's file grew without a bust twice running.
func (m *Model) talking() bool { return m.w.Heat.Leaks >= 2 }

func (m *Model) askInvestigate() {
	if len(m.w.Crew.Members) == 0 {
		m.refuse("Nothing to ask: nobody on the payroll.")
		return
	}
	if m.w.Today.Investigation != nil {
		m.refuse("Can't ask twice: somebody is already asking around tonight.")
		return
	}
	m.ask("ask", (*Model).investigateConfirm, (*Model).confirmInvestigate)
}

func (m *Model) confirmInvestigate() {
	m.mode = modePlay
	if err := m.sess.Investigate(); err != nil {
		m.refuse("Can't investigate: " + err.Error())
		return
	}
	m.say("Questions get asked tonight. Odds of a name ~" + format.Pct(m.rules.Crew.InvestigateOdds(m.w), 0) + ".")
}

func (m *Model) investigateConfirm() string {
	w := m.w
	odds := m.rules.Crew.InvestigateOdds(w)
	// The best enforcer on the books, at work or not, as the odds read
	// them (CrewState.Strongest, #275); one at skill 0 asks nothing.
	best := 0
	if e := w.Crew.Strongest(game.RoleEnforcer); e != nil {
		best = max(0, e.Skill)
	}
	who := "With no enforcer on the payroll you are asking yourself."
	if best > 0 {
		who = fmt.Sprintf("Your best enforcer (skill %d) does the asking.", best)
	}
	body := []string{
		fmt.Sprintf("Somebody goes through the crew tonight for %s.", money(m.rules.Crew.InvestigateCost())),
		who,
		"If somebody is talking to the police, ~" + format.Pct(odds, 0) + " it names them.",
	}
	if w.Crew.Investigated > 0 {
		body = append(body, theme.Subtle.Render(fmt.Sprintf("Every empty night so far (%d) narrows it down.", w.Crew.Investigated)))
	}
	body = append(body, theme.Warning.Render(fmt.Sprintf("Naming nobody costs everyone %.0f loyalty.", m.cfg.Crew.Informant.InvestigateLoyalty)))
	return m.modal("INVESTIGATE?", body, m.modalFooter())
}

func (m *Model) askPayOff() {
	c, onPayroll, ok := m.crewSelected()
	if !ok || !onPayroll {
		m.refuse("Can't pay off: put the cursor on somebody on the payroll.")
		return
	}
	m.subjectID = c.ID
	m.ask("pay", (*Model).payOffConfirm, (*Model).confirmPayOff)
}

func (m *Model) confirmPayOff() {
	m.mode = modePlay
	c := m.w.Crew.Member(m.subjectID)
	if c == nil {
		return
	}
	got, err := m.sess.PayOff(c.ID)
	if err != nil {
		m.refuse("Can't pay them off: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("%s pocketed it. Loyalty %.0f.", got.Name, got.Loyalty))
}

func (m *Model) payOffConfirm() string {
	c := m.w.Crew.Member(m.subjectID)
	if c == nil {
		return m.modal("PAY OFF", []string{"They are gone."}, m.modalFooter())
	}
	body := []string{
		fmt.Sprintf("%s for %s: loyalty %.0f %s %.0f.", money(m.rules.Crew.PayoffCost(*c)), c.Name, c.Loyalty, format.Arrow, min(100, c.Loyalty+m.rules.Crew.PayoffLoyalty())),
		theme.Subtle.Render("It buys loyalty, not silence: somebody already talking keeps talking."),
	}
	return m.modal("PAY OFF "+c.Name+"?", body, m.modalFooter())
}

func (m *Model) cyclePay() {
	p := (m.w.Crew.Pay + 1) % 3
	if err := m.sess.SetPay(p); err != nil {
		m.refuse("Can't set the pay: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Pay %s, %s/day. %s", p, money(m.rules.Crew.Wages(m.w, p)), payBlurb(p)))
}

func payBlurb(p events.Pay) string {
	switch p {
	case events.PayStingy:
		return "Loyalty bleeds."
	case events.PayGenerous:
		return "Loyalty climbs."
	default:
		return "The greedy drift."
	}
}

func loyaltyStyle(l, threshold float64) lipgloss.Style {
	switch {
	case l < threshold:
		return theme.Bad
	case l < threshold+15:
		return theme.Warning
	default:
		return theme.Good
	}
}

// crewWarning is the one warning line the crew screen carries under the
// pay dial, when there is one: the tell that somebody is talking, or a
// skim on record. MAIN truncates it; the pane's CREW section carries the
// whole of it.
func (m *Model) crewWarning() string {
	w := m.w
	tun := m.rules.Crew.Tuning()
	switch {
	case m.talking():
		return "▲ Somebody is talking. The file grew without a bust. Ask around or fire your suspect."
	case w.Crew.LastSkim > 0 && w.Day-w.Crew.LastSkim < tun.SuspectDays:
		who := fmt.Sprintf("Somebody's loyalty is under %.0f.", tun.SkimThreshold)
		if w.Crew.Role(game.RoleLieutenant) > 0 {
			who = fmt.Sprintf("Somebody's loyalty is under %.0f, or a lieutenant is greedy.", tun.SkimThreshold)
		}
		return fmt.Sprintf("▲ Skimming suspected. Money went missing on day %d. %s", w.Crew.LastSkim, who)
	}
	return ""
}

// payRow draws the pay dial as `stingy  [fair]  generous`.
func payRow(p events.Pay) string {
	return dialCells(events.PayNames(), int(p-events.PayStingy))
}

// post is the roster's status column: where the member is and, when they
// are nowhere, what that means for their role. An accountant has no
// post, they work every front you own; an enforcer without a corner
// guards nothing; a runner without one is idle.
func (m *Model) post(c game.CrewMember) any {
	w := m.w
	if tag := m.crewTag(c); tag != "" && tag != "retiring" {
		return styled{theme.Bad, tag} // in a cell or laid up (#46): nowhere
	}
	switch {
	case c.Role == game.RoleDriver:
		// The driver (#46): the route they ride, or none.
		if rid := w.DrivenRoute(c.ID); rid != "" {
			return styled{theme.CrewText, "drives " + m.routeName(rid)}
		}
		return styled{theme.Warning, "no route"}
	case c.Lieutenant():
		if c.City != "" {
			return styled{theme.CrewText, "runs " + w.CityName(c.City)}
		}
		return styled{theme.Warning, "no city"}
	case c.Role == game.RoleAccountant:
		if n := len(w.Fronts); n > 0 {
			return styled{theme.CrewText, plural(n, "front")}
		}
		return styled{theme.Warning, "no front"}
	case c.Role == game.RoleChemist:
		// The chemist (#47): the lab, or the batch on the way.
		if n := len(w.Crew.Cooks); n > 0 {
			return styled{theme.CrewText, "cooking"}
		}
		return styled{theme.CrewText, "the lab"}
	}
	if p := w.PostOf(c.ID); p != nil {
		return p.Name
	}
	if h := w.GuardOf(c.ID); h != nil {
		return styled{theme.CrewText, "guards " + h.Name}
	}
	if c.Role == game.RoleEnforcer {
		return styled{theme.Warning, "unposted"}
	}
	return styled{theme.Warning, "idle"}
}

// crewLine is the line a member skims (or, for a lieutenant, turns)
// under, which the loyalty bar is coloured against.
func (m *Model) crewLine(c game.CrewMember) float64 {
	if c.Lieutenant() {
		return m.rules.Crew.FlipLine()
	}
	return m.rules.Crew.Tuning().SkimThreshold
}

// viewCrew is the crew screen's MAIN (#86): the title with the count,
// the pay dial, the warning line when there is one, and the two tables.
// Everything about the person under the cursor and about the crew as a
// whole is the pane's.
func (m *Model) viewCrew() string {
	w := m.w
	tun := m.rules.Crew.Tuning()
	pay := w.Crew.Pay
	width := m.mainWidth()
	var b strings.Builder

	b.WriteString(truncate(theme.PanelTitle.Render("CREW")+theme.Subtle.Render(fmt.Sprintf(" · %d of %d on the payroll", len(w.Crew.Members), m.rules.Crew.MaxCrew(w))), width) + "\n")
	b.WriteString(truncate(theme.Subtle.Render("pay  ")+payRow(pay)+theme.Gold.Render(fmt.Sprintf("   %s/day", money(m.rules.Crew.Wages(w, pay)))), width) + "\n")
	if warn := m.crewWarning(); warn != "" {
		b.WriteString(truncate(theme.Bad.Render(warn), width) + "\n")
	}

	marks := []float64{tun.SkimThreshold / 100}
	// row is the cells every member and candidate shares: the loyalty
	// bar is coloured against the line they skim (or, for a lieutenant,
	// turn) under, and carry is what a runner adds to the stash.
	row := func(c game.CrewMember) []any {
		var carry any
		if c.Units > 0 {
			carry = c.Units
		}
		text := c.Name
		if len(c.Kin) > 0 {
			text += " " + kinGlyph // kin on the payroll or in the pool (#46)
		}
		var name any = text
		if c.ID == w.Crew.Exposed {
			name = styled{theme.Bad.Bold(true), text}
		}
		var age any
		if c.Age > 0 {
			age = c.Age
		}
		return []any{name, c.Role, c.Skill, age, styled{loyaltyStyle(c.Loyalty, m.crewLine(c)), gauge{c.Loyalty / 100, marks, c.Loyalty}}, m.rules.Crew.WageAt(w, c, pay), carry}
	}
	shared := []col{{"name", kText, 0}, {"role", kText, 0}, {"skill", kInt, 0}, {"age", kInt, 0}, {"loyalty", kBar, 10}, {"wage", kMoney, 0}, {"carry", kInt, 0}}

	b.WriteString(sectionTitle("ON THE PAYROLL", m.accent()) + "\n")
	if len(w.Crew.Members) == 0 {
		b.WriteString(emptyState("Nobody on the payroll. Pick a face below and press ", "h", ".") + "\n")
	} else {
		// The veterans' traits (#346) are a column once anybody has
		// shown one.
		traits := false
		for _, c := range w.Crew.Members {
			traits = traits || c.Trait != ""
		}
		var rows [][]any
		for _, c := range w.Crew.Members {
			r := row(c)
			if traits {
				r = append(r, m.traitCell(c))
			}
			rows = append(rows, append(r, m.post(c), day(c.Hired)))
		}
		cols := append([]col(nil), shared...)
		if traits {
			cols = append(cols, col{"trait", kText, 0})
		}
		cols = append(cols, col{"where", kText, 0}, col{"hired", kDays, 0})
		// Where MAIN is too narrow for the post to read whole (64
		// columns beside the pane at 100), the columns the pane carries
		// go first: carry, then the hire day, then the age (#46), then
		// the trait (#346).
		cols, rows = dropCols(cols, rows, width, "carry", "hired", "age", "trait")
		for _, l := range table(cols, rows, m.crewCursor, width) {
			b.WriteString(l + "\n")
		}
	}
	b.WriteString("\n")

	next := max(1, m.rules.Crew.PoolDays(w)-(w.Day-w.Crew.PoolDay))
	title := sectionTitle("LOOKING FOR WORK", m.accent()) + theme.Subtle.Render(" · new faces in "+plural(next, "day"))
	if !m.rules.Crew.LieutenantsWanted(w) {
		// The line named before it fires (#148): what the lieutenants
		// wait on, in the form the width has room for.
		title = firstFit(width,
			title+theme.Subtle.Render(" · lieutenants come looking once you hold corners in two cities"),
			title+theme.Subtle.Render(" · lieutenants once you hold two cities"),
			title)
	}
	b.WriteString(truncate(title, width) + "\n")
	if len(w.Crew.Candidates) == 0 {
		b.WriteString(emptyState("Nobody right now.") + "\n")
	} else {
		var rows [][]any
		for _, c := range w.Crew.Candidates {
			var fee any = c.Fee
			if c.Fee > w.Player.DirtyCash {
				fee = styled{theme.Bad, c.Fee}
			}
			rows = append(rows, append(row(c), fee))
		}
		cols := append(shared, col{"fee", kMoney, 0})
		cols, rows = dropCols(cols, rows, width, "carry", "age")
		for _, l := range table(cols, rows, m.crewCursor-len(w.Crew.Members), width) {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

// accountants is what the accountants on the payroll add to every front's
// daily wash at the current dial and the share of its audit risk they
// remove, by the laundering sim's rule: each counts by skill, and each
// takes its cut of the risk the ones before them left.
func (m *Model) accountants() (add int, cut float64) {
	tun := m.cfg.Laundering.Laundering
	through, risk := 0.0, 1.0
	for _, c := range m.w.Crew.Members {
		if c.Role != game.RoleAccountant {
			continue
		}
		skill := float64(c.Skill) / 100
		through += tun.AccountantThroughput * skill
		risk *= 1 - tun.AccountantRiskCut*skill
	}
	add = int(math.Round(through * m.rules.Laundering.Dial(m.w.Laundering.Dial).Mul))
	return add, 1 - math.Max(0, risk)
}

// crewDetails is the crew screen's pane (#86): the person under the
// cursor (their role, skill and hire day, loyalty against the lines, the
// wage at each pay dial, post, temper and whether they were named), what
// the keys would do to them with the numbers the confirmations use, then
// CREW, what the crew adds and needs as a whole, and the keys.
func (m *Model) crewDetails() []section {
	c, onPayroll, ok := m.crewSelected()
	if !ok {
		return []section{{"NOBODY", wrapped(theme.Subtle, "Nobody on the payroll and nobody looking for work. New faces come by every few days.")}, m.crewSection()}
	}
	return []section{{strings.ToUpper(c.Name), m.personLines(c, onPayroll)}, m.crewSection()}
}

// personLines is the selection section's body for one member or
// candidate.
func (m *Model) personLines(c game.CrewMember, onPayroll bool) []string {
	w := m.w
	tun := m.rules.Crew.Tuning()
	pay := w.Crew.Pay
	sub := theme.Subtle.Render

	// The first line is the person in a breath: the role, the skill and
	// the day they signed (or, for a candidate, their fee); the strip
	// reads it after the name.
	first := fmt.Sprintf("%s · skill %d · fee %s", c.Role, c.Skill, money(c.Fee))
	if onPayroll {
		first = fmt.Sprintf("%s · skill %d · hired d%d", c.Role, c.Skill, c.Hired)
		if lipgloss.Width(first) > paneTextW {
			first = fmt.Sprintf("%s · skill %d · d%d", c.Role, c.Skill, c.Hired)
		}
	}
	line := m.crewLine(c)
	lines := []string{
		first,
		row("loyalty", barText(c.Loyalty/100, 10, []float64{line / 100}, fmt.Sprintf(" %.0f", c.Loyalty), loyaltyStyle(c.Loyalty, line))),
	}
	if c.Lieutenant() {
		lines = append(lines, sub(fmt.Sprintf("  turns under %.0f · walks at %.0f", line, tun.QuitThreshold)))
	} else {
		lines = append(lines, sub(fmt.Sprintf("  skims under %.0f · walks at %.0f", line, tun.QuitThreshold)))
	}
	// The wage at the dial, and at the other two.
	var others []string
	for d := events.PayStingy; d <= events.PayGenerous; d++ {
		if d != pay {
			others = append(others, fmt.Sprintf("%s %s", d, money(m.rules.Crew.WageAt(w, c, d))))
		}
	}
	lines = append(lines, row("wage", fmt.Sprintf("%s/day %s", money(m.rules.Crew.WageAt(w, c, pay)), pay)), sub("  "+strings.Join(others, " · ")))
	if c.Units > 0 {
		lines = append(lines, row("carry", "+"+plural(c.Units, "unit")))
	}
	if a := m.ageLine(c); a != "" {
		lines = append(lines, row("age", a))
	}
	if kin := m.kinNames(c); len(kin) > 0 {
		lines = append(lines, row("kin", strings.Join(kin, ", ")))
	}
	if onPayroll {
		lines = append(lines, m.veteranLines(c)...)
	}
	if !onPayroll {
		if len(c.Kin) > 0 {
			lines = append(lines, row("", sub("came with the kin: fee at the discount")))
		}
		if w.Player.Reputation.Notoriety > 50 {
			lines = append(lines, sub("  asked to work for you")) // your name is in the paper (#233)
		}
		lines = append(lines, row("would", hireBlurb(c.Role)))
		if c.Role == game.RoleChemist {
			// What their hand would be worth (#47).
			lines = append(lines, row("cooks", fmt.Sprintf("q %.0f · %d a batch", m.rules.Crew.QualityOf(c.Skill), m.rules.Crew.BatchOf(c.Skill))))
		}
		hire := fmt.Sprintf("hire for %s", money(c.Fee))
		if c.Fee > w.Player.DirtyCash {
			hire = theme.Bad.Render(hire + " · can't afford")
		}
		return append(lines, keyRow("h", hire), m.askAroundRow())
	}
	switch {
	case c.Jailed(w.Day) && c.Bailed:
		lines = append(lines, row("post", theme.Warning.Render("jailed · out tomorrow")))
	case c.Jailed(w.Day):
		lines = append(lines, row("post", theme.Bad.Render(fmt.Sprintf("jailed · %dd to go", c.JailedUntil-w.Day))))
	case c.Wounded(w.Day):
		lines = append(lines, row("post", theme.Bad.Render(fmt.Sprintf("laid up · %dd to go", c.WoundedUntil-w.Day))))
	case c.Role == game.RoleDriver && w.DrivenRoute(c.ID) != "":
		lines = append(lines, row("drives", m.routeName(w.DrivenRoute(c.ID))))
	case c.Role == game.RoleDriver:
		lines = append(lines, row("drives", theme.Warning.Render("no route yet")))
	case c.Lieutenant() && c.City != "":
		lines = append(lines, row("runs", w.CityName(c.City)))
	case c.Lieutenant():
		lines = append(lines, row("runs", theme.Warning.Render("no city yet")))
	case c.Role == game.RoleAccountant:
		if len(w.Fronts) == 0 {
			lines = append(lines, row("post", theme.Warning.Render("no front to work")))
		} else {
			lines = append(lines, row("post", "the books"))
		}
	case c.Role == game.RoleChemist:
		// What their hand is worth (#47): the quality a cook lands at
		// and what a cut keeps, the best chemist's; a lesser one waits.
		if best := w.Crew.Chemist(); best != nil && best.ID == c.ID {
			lines = append(lines, row("cooks", fmt.Sprintf("q %.0f · %d a batch", m.rules.Crew.ChemistQuality(w), m.rules.Crew.Batch(w))))
			lines = append(lines, row("cuts", fmt.Sprintf("keep %.0f points", m.rules.Crew.CutBonus(w))))
		} else {
			lines = append(lines, row("post", theme.Subtle.Render("second to the best chemist")))
		}
	default:
		if p := w.PostOf(c.ID); p != nil {
			lines = append(lines, row("post", p.Name))
		} else if c.Role == game.RoleEnforcer {
			lines = append(lines, row("post", theme.Warning.Render("unposted")))
		} else {
			lines = append(lines, row("post", theme.Warning.Render("idle")))
		}
	}
	if t := m.temper(c); t != "" {
		lines = append(lines, row("temper", t))
	}
	if c.ID == w.Crew.Exposed {
		lines = append(lines, theme.Bad.Bold(true).Render("SNITCH")+theme.Bad.Render(": talking to the police"))
	}
	// What the keys would do to them, with the numbers the
	// confirmations use.
	if c.ID == w.Crew.Exposed {
		lines = append(lines, keyRow("f", "fire: the file stops growing"))
	} else {
		lines = append(lines, keyRow("f", fmt.Sprintf("fire: the rest lose %.0f loyalty", tun.FireLoyalty)))
	}
	if c.Lieutenant() && c.City == "" {
		lines = append(lines, keyRow("l", "give them a city"))
	} else if c.Lieutenant() {
		lines = append(lines, keyRow("l", "move them or take the city"))
	}
	switch {
	case c.Captain != "":
		lines = append(lines, keyRow("c", "move the captaincy or take it off them"))
	case m.rules.Crew.CanCaptain(w, c):
		lines = append(lines, keyRow("c", "make them captain of a city"))
	}
	if c.Jailed(w.Day) && !c.Bailed {
		bail := fmt.Sprintf("bail for %s clean", money(m.rules.Crew.BailCost(c)))
		if m.rules.Crew.BailCost(c) > w.Player.CleanCash {
			bail = theme.Bad.Render(bail + " · can't")
		}
		lines = append(lines, keyRow("b", bail))
	}
	lines = append(lines, keyRow("$", fmt.Sprintf("pay off for %s: %.0f %s %.0f", money(m.rules.Crew.PayoffCost(c)), c.Loyalty, format.Arrow, min(100, c.Loyalty+m.rules.Crew.PayoffLoyalty()))))
	return append(lines, m.askAroundRow())
}

// askAroundRow is what i would do tonight, with the price and the odds
// the confirmation shows.
func (m *Model) askAroundRow() string {
	if len(m.w.Crew.Members) == 0 {
		return keyRow("i", theme.Subtle.Render("ask around: nobody to ask"))
	}
	if m.w.Today.Investigation != nil {
		return keyRow("i", theme.Subtle.Render("ask around: already asking"))
	}
	return keyRow("i", fmt.Sprintf("ask around %s, names ~%s", money(m.rules.Crew.InvestigateCost()), format.Pct(m.rules.Crew.InvestigateOdds(m.w), 0)))
}

// temper is a lieutenant's personality as the pane shows it: the word
// once you have seen enough of them, how long that is until then, and
// nothing for anyone else.
func (m *Model) temper(c game.CrewMember) string {
	switch {
	case !c.Lieutenant():
		return ""
	case c.Observed:
		return c.Personality
	case c.City == "":
		return theme.Subtle.Render("shows on the job")
	}
	left := max(1, m.rules.Crew.RevealDays()-(m.w.Day-c.Assigned))
	return theme.Subtle.Render("shows in " + plural(left, "day"))
}

// hireBlurb is what a candidate would do on the payroll, for the pane.
func hireBlurb(role string) string {
	switch role {
	case game.RoleAccountant:
		return "work the fronts"
	case game.RoleEnforcer:
		return "guard a corner"
	case game.RoleLieutenant:
		return "run a city"
	case game.RoleChemist:
		return "cook and cut"
	case game.RoleDriver:
		return "drive a route"
	default:
		return "hold a corner"
	}
}

// crewSection is the pane's CREW section: the warning in full, what the
// crew adds where you stand, who is off a corner, what the accountants
// do, who runs what, who is talking and what the sloppy ones cost.
func (m *Model) crewSection() section {
	w := m.w
	here := w.Here()
	sub := theme.Subtle.Render
	var lines []string
	if warn := m.crewWarning(); warn != "" {
		lines = append(lines, wrapped(theme.Bad, warn)...)
	}
	lines = append(lines,
		row("capacity", fmt.Sprintf("%d in %s", w.Capacity(here.ID), here.Name)),
		row("", sub(fmt.Sprintf("%d yours + %d crew", w.Player.CarryLimit, w.Capacity(here.ID)-w.Player.CarryLimit))),
		row("corners", fmt.Sprintf("%d worked", w.Worked())),
	)
	idle, unposted := 0, 0
	for _, c := range w.Crew.Members {
		if w.PostOf(c.ID) != nil || !c.Fit(w.Day) {
			continue // one in a cell or laid up (#46) is not idle, they are nowhere
		}
		switch c.Role {
		case game.RoleRunner:
			idle++
		case game.RoleEnforcer:
			unposted++
		}
	}
	if idle > 0 {
		lines = append(lines, row("idle", theme.Warning.Render(plural(idle, "runner"))))
	}
	if unposted > 0 {
		lines = append(lines, row("unposted", theme.Warning.Render(plural(unposted, "enforcer"))))
	}
	if n := m.jailed(); n > 0 {
		lines = append(lines, row("jailed", theme.Bad.Render(plural(n, "member"))))
	}
	if n := m.wounded(); n > 0 {
		lines = append(lines, row("laid up", theme.Bad.Render(plural(n, "member"))))
	}
	if idle+unposted > 0 {
		// The pointer is wrapped on its own so it never breaks.
		lines = append(lines, wrapped(theme.Warning, "A runner earns nothing and an enforcer guards nothing off a corner.")...)
		lines = append(lines, wrapped(theme.Warning, "Post them "+screenPointer(screenMap)+".")...)
	}
	if n := w.Crew.Role(game.RoleAccountant); n > 0 {
		if len(w.Fronts) == 0 {
			lines = append(lines, wrapped(theme.Warning, "An accountant with no front is a wage.")...)
			lines = append(lines, wrapped(theme.Warning, "Buy "+screenPointer(screenLedger)+".")...)
		} else {
			add, cut := m.accountants()
			lines = append(lines, row("accountant", fmt.Sprintf("+%s/day a front", money(add))), row("", sub("audit risk cut "+format.Pct(cut, 0))))
		}
	}
	for _, cid := range w.CityOrder {
		if lt := w.Crew.Lieutenant(cid); lt != nil {
			runs := lt.Name
			if lt.Observed {
				runs += " · " + lt.Personality
			}
			lines = append(lines, row(w.CityName(cid), runs))
		}
	}
	for _, c := range w.Crew.Members {
		if c.Lieutenant() && c.City == "" {
			lines = append(lines, row("no city", theme.Warning.Render(c.Name+" is a wage")))
		}
	}
	if c := w.Crew.Member(w.Crew.Exposed); c != nil {
		lines = append(lines, row("snitch", theme.Bad.Render(c.Name+", fire them")))
	} else if w.Today.Investigation != nil {
		lines = append(lines, row("tonight", "questions get asked"))
	}
	if sl := m.rules.Heat.Sloppiness(w, here.ID); sl > 0 {
		per := sl * m.cfg.Heat.Heat.SloppyHeat * 100
		lines = append(lines, row("sloppy", theme.Warning.Render(fmt.Sprintf("+%.1f heat/100 units", per))), row("", sub(fmt.Sprintf("runners under skill %d", m.cfg.Heat.Heat.SloppySkill))))
	}
	return section{"CREW", lines}
}

// jailed and wounded count the crew in a cell and laid up (#46).
func (m *Model) jailed() int {
	n := 0
	for _, c := range m.w.Crew.Members {
		if c.Jailed(m.w.Day) {
			n++
		}
	}
	return n
}

func (m *Model) wounded() int {
	n := 0
	for _, c := range m.w.Crew.Members {
		if c.Wounded(m.w.Day) {
			n++
		}
	}
	return n
}

// crewMove walks the crew screen's rows: the roster, then the
// candidates.
func (m *Model) crewMove(dy int) {
	if dy < 0 && m.crewCursor > 0 {
		m.crewCursor--
	} else if dy > 0 && m.crewCursor < len(m.crewRows())-1 {
		m.crewCursor++
	}
}
