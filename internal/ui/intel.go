package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The intel screen (#45, tab 9): what you know against what is true.
// One table of the file's live facts (subject, fact, how sure, age,
// source), newest first, under one cursor; the pane the fact under it
// and its story, then SPIES; `$ pay cop` buys a cop's word (modePayCop:
// an amount, blank the price), `p plant spy` sends a crew member under
// (modeSpy: the faction, then who). `i` on the map with a rival corner
// under the cursor and on the dashboard jumps here to the subject. The
// rival, law and route panels read the file too (known, muscleWord,
// oddsWord, riskWord): never Rival.Personality, Rival.Muscle,
// Law.Chief.Personality or Route.Risk (TestPanelsReadTheFile).

// known is the file as it reads today.
func (m *Model) known() game.Knowledge { return game.Known(m.w) }

// intelCols are the INTEL table's columns.
var intelCols = []col{{"subject", kText, 0}, {"fact", kText, 0}, {"sure", kPct, 0}, {"learnt", kDays, 0}, {"source", kText, 0}}

// intelRows are the table's rows, one a live fact, the newest first.
func (m *Model) intelRows() [][]any {
	var rows [][]any
	for _, f := range m.known().Facts() {
		rows = append(rows, []any{m.subjectWord(f), m.factWord(f), f.Now(m.w.Day) * 100, f.Age(m.w.Day), m.sourceWord(f)})
	}
	return rows
}

// intelSelected is the fact under the cursor, or nil with an empty file.
func (m *Model) intelSelected() *game.Fact {
	facts := m.known().Facts()
	if len(facts) == 0 {
		return nil
	}
	return &facts[clamp(&m.intelCursor, len(facts))]
}

// intelMove moves the cursor down the file.
func (m *Model) intelMove(dy int) {
	n := len(m.known().Facts())
	if dy < 0 && m.intelCursor > 0 {
		m.intelCursor--
	} else if dy > 0 && m.intelCursor < n-1 {
		m.intelCursor++
	}
}

// jumpIntel opens the intel screen on the first fact about the subject
// (`i` on the map's rival corner, `i` on the dashboard for the chief),
// or on the top of the file with a word that nothing is known yet.
func (m *Model) jumpIntel(subject, name string) {
	m.switchScreen(screenIntel)
	for i, f := range m.known().Facts() {
		if f.Subject == subject {
			m.intelCursor = i
			return
		}
	}
	m.intelCursor = 0
	m.say(fmt.Sprintf("Nothing is known about %s yet. What happens to you is written here; a cop or a spy fills it in.", name))
}

// subjectWord names a fact's subject: a leader in the faction's colour,
// the chief, a city's police, a road.
func (m *Model) subjectWord(f game.Fact) any {
	w := m.w
	switch f.Kind {
	case game.FactResponse:
		return w.CityName(f.Subject) + " police"
	case game.FactRisk:
		return m.routeName(f.Subject)
	}
	if f.Subject == game.SubjectChief {
		return "Chief " + truncate(w.Law.Chief.Name, 12)
	}
	if r := w.Faction(f.Subject); r != nil && r.Leader != "" {
		return styled{m.factionStyle(r.Faction()), truncate(r.Leader, 14)}
	}
	return f.Subject
}

// factWord is the fact in words for the table: `expansionist`, `4–6
// heads`, `$48K chest`, `moves on Riverside`, `sting from d34`, `seized
// ~2%/day`.
func (m *Model) factWord(f game.Fact) any {
	w := m.w
	switch f.Kind {
	case game.FactPersonality:
		return f.Value
	case game.FactMuscle:
		return f.Value + " heads"
	case game.FactCash:
		return cash(int(f.Number)) + " chest"
	case game.FactIncome:
		return cash(int(f.Number)) + "/day take"
	case game.FactWages:
		return cash(int(f.Number)) + "/day wages"
	case game.FactMove:
		if c := w.Corner(f.Value); c != nil {
			return "moves on " + c.Name
		}
	case game.FactStash:
		if c := w.Corner(f.Value); c != nil {
			return "till on " + c.Name
		}
	case game.FactResponse:
		return fmt.Sprintf("%s from d%.0f", f.Value, f.Number)
	case game.FactRisk:
		return "seized " + f.Value
	}
	return f.Value
}

// sourceWord is how a fact was learnt: `seen`, `the books`, `a cop`,
// `your spy`, `a contact says`, or, once a lie has bitten, `fed by
// Sal` in red.
func (m *Model) sourceWord(f game.Fact) any {
	switch f.Source {
	case game.SourceSeen:
		return "seen"
	case game.SourceBooks:
		return "the books"
	case game.SourceCop:
		return "a cop"
	case game.SourceSpy:
		return "your spy"
	case game.SourceContact:
		return styled{theme.Warning, "a contact says"}
	}
	if r := m.w.Faction(f.Source); r != nil && r.Leader != "" {
		return styled{theme.Bad, "fed by " + truncate(r.Leader, 10)}
	}
	return f.Source
}

// story is the fact's paragraph for the pane: what it is, where it came
// from and how far to trust it.
func (m *Model) story(f game.Fact) string {
	w := m.w
	who := ""
	if r := w.Faction(f.Subject); r != nil && r.Leader != "" && f.Kind != game.FactResponse && f.Kind != game.FactRisk {
		who = r.Leader
	}
	var what string
	switch f.Kind {
	case game.FactPersonality:
		if f.Subject == game.SubjectChief {
			what = fmt.Sprintf("Chief %s runs a %s department: it sets the cooldown, the cap and the decay the heat gauge works by.", w.Law.Chief.Name, f.Value)
		} else {
			what = fmt.Sprintf("%s plays %s: how they claim, when they push, and what they give up.", who, f.Value)
		}
	case game.FactMuscle:
		what = fmt.Sprintf("%s heads on %s's side. The strike picker's odds are read at this count; unknown, they read blind.", f.Value, who)
	case game.FactCash:
		what = fmt.Sprintf("%s in %s's chest: what a hire, a claim and the wage bill come out of.", cash(int(f.Number)), who)
	case game.FactIncome:
		what = fmt.Sprintf("%s's corners bring in %s a day.", who, cash(int(f.Number)))
	case game.FactWages:
		what = fmt.Sprintf("%s pays %s a day in wages; a chest under a few days of it is a crew about to shed heads.", who, cash(int(f.Number)))
	case game.FactMove:
		if c := w.Corner(f.Value); c != nil {
			what = fmt.Sprintf("%s moves on %s next: post somebody there, or an enforcer.", who, c.Name)
		}
	case game.FactStash:
		if c := w.Corner(f.Value); c != nil {
			what = fmt.Sprintf("%s's till is fattest on %s: a boost there takes the most.", who, c.Name)
		}
	case game.FactResponse:
		what = fmt.Sprintf("The %s police stand at the %s rung and can move from day %.0f: before that the cooldown holds them, whatever the heat.", w.CityName(f.Subject), f.Value, f.Number)
	case game.FactRisk:
		what = fmt.Sprintf("%s is seized %s in transit at the normal dial; the map's odds fold it the way the dice would.", m.routeName(f.Subject), f.Value)
	}
	how := map[string]string{
		game.SourceSeen:    "You saw it yourself.",
		game.SourceBooks:   "Your scout read it in their books.",
		game.SourceCop:     "A cop told you for money; cops are right about as often as they are paid.",
		game.SourceSpy:     "Your spy sent it out; a good one is right more often.",
		game.SourceContact: "A contact passed it on. Nobody vouches for a contact.",
	}[f.Source]
	if how == "" {
		if r := w.Faction(f.Source); r != nil {
			how = fmt.Sprintf("It was a lie, and %s fed it to you.", r.Leader)
		}
	}
	return strings.TrimSpace(what + " " + how)
}

// learntWord is when a fact was learnt: `day 4 · today`, `day 4 · 3
// days ago`.
func learntWord(f game.Fact, day int) string {
	if f.Age(day) == 0 {
		return fmt.Sprintf("day %d · today", f.Day)
	}
	return fmt.Sprintf("day %d · %s ago", f.Day, plural(f.Age(day), "day"))
}

// sureBar is a fact's confidence as a bar: `████░░ 67%`.
func sureBar(f game.Fact, day int) string {
	c := f.Now(day)
	style := theme.Good
	switch {
	case c < 0.4:
		style = theme.Bad
	case c < 0.7:
		style = theme.Warning
	}
	return barText(c, 8, nil, fmt.Sprintf(" %.0f%%", c*100), style)
}

// viewIntel is MAIN on the intel screen: the file as a table over the
// spies under.
func (m *Model) viewIntel() string {
	w := m.w
	width := m.mainWidth()
	var ls []string
	line := func(s string) { ls = append(ls, truncate(s, width)) }
	facts := m.known().Facts()
	line(theme.PanelTitle.Render(fmt.Sprintf("INTEL · %s", plural(len(facts), "fact"))))
	if len(facts) == 0 {
		line(emptyState("Nothing known yet. Pay a cop with ", "$", ", or plant a spy with ", "p", "; what happens to you is written here."))
	} else {
		ls = append(ls, table(intelCols, m.intelRows(), clamp(&m.intelCursor, len(facts)), width)...)
	}
	ls = append(ls, "")
	line(sectionTitle("SPIES", m.accent()))
	spies := w.Crew.Spies()
	if len(spies) == 0 {
		line(emptyState("Nobody under."))
	} else {
		var rows [][]any
		for _, s := range spies {
			rows = append(rows, []any{s.Name, m.spyWith(s), s.Skill, w.Day - s.UndercoverDay, m.nextReport(s)})
		}
		ls = append(ls, table([]col{{"spy", kText, 0}, {"with", kText, 0}, {"skill", kInt, 0}, {"under", kDays, 0}, {"reports", kText, 0}}, rows, -1, width)...)
	}
	if o := w.Today.Spy; o != nil {
		if s := w.Crew.Member(o.Member); s != nil {
			line(theme.Gold.Render(fmt.Sprintf("Tonight  %s goes under with %s", s.Name, m.rivalName(w.Faction(o.Faction)))))
		}
	}
	if o := w.Today.Cop; o != nil {
		line(theme.Gold.Render(fmt.Sprintf("Tonight  a cop takes %s and talks: ~%.0f%% straight", money(o.Amount), m.intelTuning().Accuracy(o.Amount)*100)))
	}
	return strings.Join(ls, "\n")
}

// spyWith names the faction a spy is under with, in its colour.
func (m *Model) spyWith(s game.CrewMember) any {
	r := m.w.Faction(s.Undercover)
	if r == nil {
		return s.Undercover
	}
	return styled{m.factionStyle(r.Faction()), truncate(r.Leader, 12)}
}

// nextReport is when a spy next files, and how straight they are.
func (m *Model) nextReport(s game.CrewMember) string {
	tun := m.intelTuning()
	if tun.SpyDays <= 0 {
		return "never"
	}
	under := m.w.Day - s.UndercoverDay
	due := tun.SpyDays - under%tun.SpyDays
	return fmt.Sprintf("next in %dd · ~%.0f%% right", due, tun.SpyOdds(s.Skill)*100)
}

// intelTuning is intel.toml's table.
func (m *Model) intelTuning() content.IntelTuning { return m.cfg.Intel.Intel }

// intelDetails is the intel screen's pane: the fact under the cursor
// and its story, the two moves, then SPIES.
func (m *Model) intelDetails() []section {
	w := m.w
	tun := m.intelTuning()
	var sel section
	if f := m.intelSelected(); f != nil {
		title := fmt.Sprint(m.subjectWord(*f))
		if s, ok := m.subjectWord(*f).(styled); ok {
			title = fmt.Sprint(s.v)
		}
		lines := []string{
			row("fact", fmt.Sprint(m.factWord(*f))),
			row("sure", sureBar(*f, w.Day)),
			row("learnt", learntWord(*f, w.Day)),
			row("source", fmt.Sprint(m.sourceWord(*f))),
		}
		if f.Stale > 0 {
			lines = append(lines, row("fades", fmt.Sprintf("−%.0f%%/day · gone at %.0f%%", f.Stale*100, f.Forget*100)))
		}
		lines = append(lines, wrapped(theme.Subtle, m.story(*f))...)
		sel = section{strings.ToUpper(title), lines}
	} else {
		sel = section{"THE FILE", wrapped(theme.Subtle, "Nothing known yet. A push shows you their muscle, a raid the chief, a seizure the road; the rest is bought or sent out from under.")}
	}
	sel.lines = append(sel.lines,
		keyRow("$", fmt.Sprintf("pay a cop: %s, ~%.0f%% straight", money(tun.CopPrice), tun.CopAccuracy*100)),
		keyRow("p", fmt.Sprintf("plant a spy: a report every %s", plural(tun.SpyDays, "day"))))
	var spies []string
	for _, s := range w.Crew.Spies() {
		r := w.Faction(s.Undercover)
		spies = append(spies, row(truncate(s.Name, paneLabelW), fmt.Sprintf("with %s · %s", m.rivalName(r), m.nextReport(s))))
	}
	if len(spies) == 0 {
		spies = wrapped(theme.Subtle, fmt.Sprintf("Nobody under. A spy stops selling and reports every %s; found, %.0f%% come home talking and the rest are shot.", plural(tun.SpyDays, "day"), tun.TurnShare*100))
	}
	return []section{sel, {"SPIES", spies}}
}

// The cop dialog (modePayCop): one page, a money field whose blank is
// the price, what the money buys and how straight it is.
type copDialog struct {
	amt numberField
	err string
}

// askPayCop opens the cop dialog.
func (m *Model) askPayCop() {
	if m.w.Over != nil {
		return
	}
	if m.w.Today.Cop != nil {
		m.refuse("Can't pay a cop: you have paid one today. They talk in the morning's report.")
		return
	}
	if m.w.Player.DirtyCash <= 0 {
		m.refuse("Can't pay a cop: a cop takes dirty cash, and you have none.")
		return
	}
	m.cop = copDialog{amt: newNumberField("blank = price")}
	m.cop.amt.money = true
	m.cop.amt.max = m.w.Player.DirtyCash
	m.cop.amt.Focus()
	m.mode = modePayCop
}

// copAmount is the amount the field reads: blank is the price, or the
// dirty cash where that is less.
func (m *Model) copAmount() (int, error) {
	return parseQtyInput(m.cop.amt.Value(), min(m.intelTuning().CopPrice, m.w.Player.DirtyCash))
}

func (m *Model) keyPayCop(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	m.cop.err = ""
	switch key {
	case "esc", "q":
		m.mode = modePlay
		return m, nil
	case "enter":
		m.confirmPayCop()
		return m, nil
	}
	m.cop.amt.max = m.w.Player.DirtyCash
	return m, m.cop.amt.Update(k)
}

// confirmPayCop pays, or shows why it cannot.
func (m *Model) confirmPayCop() {
	amt, err := m.copAmount()
	if err != nil {
		m.cop.err = dialogError(err)
		return
	}
	if err := m.w.PayCop(amt); err != nil {
		m.cop.err = dialogError(err)
		return
	}
	m.mode = modePlay
	m.say(fmt.Sprintf("A cop takes %s. What they know of the chief and the police here is in the morning's report, ~%.0f%% straight.", money(amt), m.intelTuning().Accuracy(amt)*100))
}

// viewPayCop is the dialog.
func (m *Model) viewPayCop() string {
	w := m.w
	tun := m.intelTuning()
	amt, err := m.copAmount()
	if err != nil {
		amt = 0
	}
	field := m.cop.amt
	field.max = w.Player.DirtyCash
	body := []string{
		m.inHand(),
		row("amount", field.View()),
	}
	if amt > 0 {
		style := theme.Gold
		if amt > w.Player.DirtyCash {
			style = theme.Bad
		}
		body = append(body, row("straight", style.Render(fmt.Sprintf("~%.0f%%", tun.Accuracy(amt)*100))))
	}
	body = append(body, "")
	body = append(body, m.subtle(fmt.Sprintf("Buys what Chief %s is like and the %s police's next move: the rung they stand at and the first night they can fire. The price is %s for %.0f%%; less money, less often. A wrong word is off by a rung or a few days.", w.Law.Chief.Name, w.Here().Name, money(tun.CopPrice), tun.CopAccuracy*100))...)
	body = append(body, m.subtle("The word is in the morning's report and on the intel screen. A cop is not a bribe: nothing is filed.")...)
	if m.cop.err != "" {
		body = append(body, "", theme.Bad.Render(m.cop.err))
	}
	return m.modal("PAY A COP", body, m.modalFooter())
}

// The spy dialog (modeSpy): the faction, then who goes under.
type spyDialog struct {
	step    int
	faction int
	member  int
}

// spyFactions are the factions a spy can join: alive, in table order.
func (m *Model) spyFactions() []*game.RivalState {
	var out []*game.RivalState
	for _, r := range m.w.Rivals {
		if r != nil && r.Alive() {
			out = append(out, r)
		}
	}
	return out
}

// spyCandidates are the crew who can go under: at work and not running
// a city, in roster order.
func (m *Model) spyCandidates() []game.CrewMember {
	var out []game.CrewMember
	for _, c := range m.w.Crew.Members {
		if c.Working() && !c.Runs() {
			out = append(out, c)
		}
	}
	return out
}

// askSpy opens the spy dialog, or says why nobody can go.
func (m *Model) askSpy() {
	if m.w.Over != nil {
		return
	}
	if len(m.spyFactions()) == 0 {
		m.refuse("Nobody to spy on: no faction is on the ground.")
		return
	}
	if len(m.spyCandidates()) == 0 {
		m.refuse("Nobody to send: everyone is in a cell, laid up, under or running a city. Hire " + screenPointer(screenCrew) + ".")
		return
	}
	if m.w.Today.Spy != nil {
		m.refuse("Can't plant another: somebody is going under tonight.")
		return
	}
	m.spy = spyDialog{}
	if len(m.spyFactions()) == 1 {
		m.spy.step = 1
	}
	m.mode = modeSpy
}

func (m *Model) keySpy(key string) {
	switch key {
	case "esc", "q":
		m.mode = modePlay
	case "shift+tab":
		if m.spy.step > 0 && len(m.spyFactions()) > 1 {
			m.spy.step = 0
		}
	case "up", "k":
		if m.spy.step == 0 && m.spy.faction > 0 {
			m.spy.faction--
		} else if m.spy.step == 1 && m.spy.member > 0 {
			m.spy.member--
		}
	case "down", "j":
		if m.spy.step == 0 && m.spy.faction < len(m.spyFactions())-1 {
			m.spy.faction++
		} else if m.spy.step == 1 && m.spy.member < len(m.spyCandidates())-1 {
			m.spy.member++
		}
	case "enter", "tab":
		if m.spy.step == 0 {
			m.spy.step = 1
			return
		}
		m.confirmSpy()
	}
}

// confirmSpy sends them, or says why not.
func (m *Model) confirmSpy() {
	facs, cands := m.spyFactions(), m.spyCandidates()
	m.mode = modePlay
	if len(facs) == 0 || len(cands) == 0 {
		return
	}
	r := facs[max(0, min(m.spy.faction, len(facs)-1))]
	c := cands[max(0, min(m.spy.member, len(cands)-1))]
	if err := m.w.PlantSpy(r.Faction(), c.ID); err != nil {
		m.refuse("Can't send them: " + err.Error())
		return
	}
	tun := m.intelTuning()
	found := "the odds of being found are their temper's"
	if temper := m.known().Personality(r.Faction()); temper != game.Unknown {
		found = fmt.Sprintf("~%.0f%% a report they are found", tun.Found(temper)*100)
	}
	m.say(fmt.Sprintf("%s goes under with %s tonight: off the street, a report every %s, ~%.0f%% right, and %s.", c.Name, m.rivalName(r), plural(tun.SpyDays, "day"), tun.SpyOdds(c.Skill)*100, found))
}

// viewSpy is the dialog: the factions, then the crew.
func (m *Model) viewSpy() string {
	tun := m.intelTuning()
	if m.spy.step == 0 {
		facs := m.spyFactions()
		m.spy.faction = max(0, min(m.spy.faction, len(facs)-1))
		var cells [][]any
		for _, r := range facs {
			cells = append(cells, []any{styled{m.factionStyle(r.Faction()), truncate(r.Leader, 14)}, m.w.CityOf(r).Name, m.known().Personality(r.Faction()), m.muscleWord(r), factCount(m.known(), r)})
		}
		m.modalFollow(1 + m.spy.faction)
		body := table([]col{{"faction", kText, 0}, {"city", kText, 0}, {"temper", kText, 0}, {"muscle", kText, 0}, {"known", kText, 0}}, cells, m.spy.faction, m.modalInner())
		body = append(body, "")
		body = append(body, m.subtle("Whose crew? A spy under reports its muscle, its next move and where its till is; the odds of being found are its temper's.")...)
		return m.modal("PLANT A SPY", body, m.modalFooter())
	}
	facs := m.spyFactions()
	r := facs[max(0, min(m.spy.faction, len(facs)-1))]
	cands := m.spyCandidates()
	m.spy.member = max(0, min(m.spy.member, len(cands)-1))
	var cells [][]any
	for _, c := range cands {
		where := "idle"
		if p := m.w.PostOf(c.ID); p != nil {
			where = p.Name
		}
		cells = append(cells, []any{c.Name, c.Role, c.Skill, approx{tun.SpyOdds(c.Skill) * 100}, where})
	}
	m.modalFollow(1 + m.spy.member)
	body := table([]col{{"name", kText, 0}, {"role", kText, 0}, {"skill", kInt, 0}, {"right", kPct, 0}, {"where", kText, 0}}, cells, m.spy.member, m.modalInner())
	body = append(body, "")
	foundWord := "their temper's, which you do not know"
	if temper := m.known().Personality(r.Faction()); temper != game.Unknown {
		foundWord = fmt.Sprintf("%.0f%% a report", tun.Found(temper)*100)
	}
	body = append(body, m.subtle(fmt.Sprintf("Who goes under with %s? They sell nothing for you while they are there and report every %s. The odds of being found are %s; found, %.0f%% come home and the rest are shot.", m.rivalName(r), plural(tun.SpyDays, "day"), foundWord, tun.TurnShare*100))...)
	return m.modal("PLANT A SPY · "+strings.ToUpper(m.rivalName(r)), body, m.modalFooter())
}

// factCount counts the facts the file holds on a faction, for the picker.
func factCount(k game.Knowledge, r *game.RivalState) string {
	n := 0
	for _, f := range k.Facts() {
		if f.Subject == r.Faction() {
			n++
		}
	}
	return plural(n, "fact")
}

// What the other panels read of the file (#45).

// muscleWord is a faction's heads as the file holds them: `5`, `4–6`
// or `?`.
func (m *Model) muscleWord(r *game.RivalState) string {
	return m.known().MuscleWord(r.Faction())
}

// muscleAge is the muscle fact's age in words, `(3d)`, or "".
func (m *Model) muscleAge(r *game.RivalState) string {
	if _, _, f, ok := m.known().Muscle(r.Faction()); ok {
		return fmt.Sprintf("(%dd)", f.Age(m.w.Day))
	}
	return ""
}

// oddsWord is a strike's odds at a force on a corner (nil: any of the
// faction's; a deed on the block cuts their defence, #194) as the file
// lets you read them: `~40%` at a count, `~30–45%` over a band, `?`
// unknown.
func (m *Model) oddsWord(r *game.RivalState, c *game.Corner, force events.Force) string {
	lo, hi, _, ok := m.known().Muscle(r.Faction())
	if !ok {
		return game.Unknown
	}
	rv := m.set.Rivals
	a, b := rv.OddsOnAt(m.w, r, c, force, hi)*100, rv.OddsOnAt(m.w, r, c, force, lo)*100
	if lo == hi || fmt.Sprintf("%.0f", a) == fmt.Sprintf("%.0f", b) {
		return fmt.Sprintf("~%.0f%%", a)
	}
	return fmt.Sprintf("~%.0f–%.0f%%", a, b)
}

// oddsCell is oddsWord for a table's percent column: the odds at a
// count, the band's word over a band, nil unknown.
func (m *Model) oddsCell(r *game.RivalState, c *game.Corner, force events.Force) any {
	lo, hi, _, ok := m.known().Muscle(r.Faction())
	if !ok {
		return nil
	}
	if lo == hi {
		return approx{m.set.Rivals.OddsOnAt(m.w, r, c, force, lo) * 100}
	}
	return m.oddsWord(r, c, force)
}

// pushWord is a push's odds on a corner as the file lets you read them.
func (m *Model) pushWord(r *game.RivalState, c *game.Corner) string {
	lo, hi, _, ok := m.known().Muscle(r.Faction())
	if !ok {
		return game.Unknown
	}
	rv := m.set.Rivals
	a, b := rv.PushOddsAt(m.w, r, c, lo)*100, rv.PushOddsAt(m.w, r, c, hi)*100
	if lo == hi || fmt.Sprintf("%.0f", a) == fmt.Sprintf("%.0f", b) {
		return fmt.Sprintf("~%.0f%%", a)
	}
	return fmt.Sprintf("~%.0f–%.0f%%", a, b)
}

// defenceWord is what a faction puts on a struck corner as the file
// lets you read it.
func (m *Model) defenceWord(r *game.RivalState) string {
	lo, hi, _, ok := m.known().Muscle(r.Faction())
	if !ok {
		return game.Unknown
	}
	rv := m.set.Rivals
	if lo == hi {
		return fmt.Sprintf("~%.1f", rv.DefenceAt(m.w, r, lo))
	}
	return fmt.Sprintf("~%.1f–%.1f", rv.DefenceAt(m.w, r, lo), rv.DefenceAt(m.w, r, hi))
}

// seizedWord is a route's odds of a seizure over a run at a dial as
// the file lets you read them: `~12%`, or `?` on a road you have never
// lost a shipment on and nobody has told you about (#238: `riskWord`
// is the corner's band, `rough`).
func (m *Model) seizedWord(r content.RouteConfig, d events.Ship) string {
	base, ok := m.known().Risk(r.ID)
	if !ok {
		return game.Unknown
	}
	return fmt.Sprintf("~%.0f%%", m.set.Logistics.RiskFrom(m.w, r, d, base)*100)
}

// chiefWord is the chief's temper as the file holds it, or `?`.
func (m *Model) chiefWord() string { return m.known().Chief() }
