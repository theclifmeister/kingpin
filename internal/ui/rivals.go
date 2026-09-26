package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// forces are the strike picker's rows, in dial order.
var forces = []events.Force{events.ForceWarn, events.ForcePush, events.ForceHit}

// rivalName is how a faction is referred to everywhere: `Big Sal's
// crew`, or "the rival" for one with no leader yet.
func (m *Model) rivalName(r *game.RivalState) string {
	if r == nil || r.Leader == "" {
		return "the rival"
	}
	return r.Leader + "'s crew"
}

// personalityWord is what the player knows about a faction's
// temperament.
func (m *Model) personalityWord(r *game.RivalState) string {
	if word := m.known().Personality(r.Faction()); word != game.Unknown {
		return word
	}
	return "unknown"
}

// eyeingWord is the tell (#69) for a panel line, `eyeing Riverside` in
// the faction's colour, or "" while no corner is spoken for.
func (m *Model) eyeingWord(r *game.RivalState) string {
	c := m.rules.Rivals.EyeingBy(m.w, r)
	if c == nil || c.Owner != game.OwnerNone {
		return ""
	}
	return m.factionStyle(r.Faction()).Render("eyeing " + c.Name)
}

// downWords is what keeps a faction from counting toward the crown's
// "crews down" and for how long, in the rule's own terms (#472,
// rivals.Sim.Down): `run out 3d ago; gone in 27d unless it claims
// again`, `holds 2 corners; no clock while it holds one`; "" for one
// that counts (gone, or paying you homage). It describes and pushes
// nothing (#413).
func (m *Model) downWords(r *game.RivalState) string {
	d := m.rules.Rivals.Down(m.w, r)
	day := m.w.Day
	in := func(on int) string {
		if on <= day+1 {
			return "tonight"
		}
		return fmt.Sprintf("in %dd", on-day)
	}
	ago := func(since int) string {
		if since > day {
			return "run out"
		}
		if since == day {
			return "run out today"
		}
		return fmt.Sprintf("run out %dd ago", day-since)
	}
	switch {
	case d.Counts:
		return ""
	case d.Scouting:
		return "on its way; it counts once it is here and down"
	case d.Due > day+1:
		return fmt.Sprintf("not here yet: may move in from day %d", d.Due)
	case d.Due > 0 && d.GoneOn > 0:
		return "waiting for room to move in; stands down " + in(d.GoneOn) + " if it finds none"
	case d.Due > 0:
		return "waiting for room to move in"
	case d.Corners > 0:
		return "holds " + plural(d.Corners, "corner") + "; no clock while it holds one"
	case d.Rich && d.GoneOn > 0:
		return ago(d.Since) + ", can afford a claim; gone " + in(d.GoneOn) + " unless it claims again, sooner if broke"
	case d.Rich:
		return ago(d.Since) + ", can afford a claim; gone once it cannot"
	}
	return ago(d.Since) + "; gone " + in(d.GoneOn) + " unless it claims again"
}

// factionCols are the rivals screen's FACTIONS table: who, where, what
// they hold, their muscle and where you stand.
var factionCols = []col{{"faction", kText, 0}, {"city", kText, 0}, {"corners", kInt, 0}, {"muscle", kText, 0}, {"stance", kText, 0}, {"trust", kBar, 6}}

// factionRows are the FACTIONS table's rows, one a faction in the order
// of the table, the leader in the faction's colour.
func (m *Model) factionRows() [][]any {
	w := m.w
	tun := m.rules.Rivals.Tuning()
	var rows [][]any
	for _, r := range w.Rivals {
		name := m.factionStyle(r.Faction()).Render(truncate(r.Leader, 14))
		var corners, muscle, trust any
		if r.Arrived > 0 && !r.Gone() {
			corners, muscle, trust = w.RivalHeldBy(r.Faction()), m.muscleWord(r), styled{m.trustStyle(r), gauge{frac: r.Trust / 100, n: r.Trust}} // the muscle as the file holds it (#45); trust a bar, as the pane draws it
		}
		rows = append(rows, []any{name, w.CityOf(r).Name, corners, muscle, w.Stance(r, tun.WarThreshold), trust})
	}
	return rows
}

// strikeRows are the picker's choices: the three forces for the corner,
// the three for its till (a boost, #70), plus calling off a strike or a
// boost already queued.
func (m *Model) strikeRows() []string {
	rows := []string{"warn", "push", "hit", "boost warn", "boost push", "boost hit"}
	if m.w.Today.Strike != nil {
		rows = append(rows, "stop")
	}
	return rows
}

// askStrike opens the force picker for the selected corner.
func (m *Model) askStrike() {
	c := m.mapSelected()
	if c == nil {
		return
	}
	if c.Owner != game.OwnerRival {
		m.refuse("Can't send them there: enforcers go against a corner a rival holds.")
		return
	}
	if m.w.Crew.Role(game.RoleEnforcer) == 0 {
		m.refuse("Nothing to send: no enforcers. Hire one " + screenPointer(screenCrew) + ".")
		return
	}
	m.pick.cursor = 1
	m.mode = modeStrike
}

func (m *Model) confirmStrike() {
	c := m.mapSelected()
	rows := m.strikeRows()
	m.mode = modePlay
	if c == nil {
		return
	}
	i := max(0, min(m.pick.cursor, len(rows)-1))
	if i >= 2*len(forces) {
		m.sess.CallOff()
		m.say("Called off. The enforcers stay home tonight.")
		return
	}
	if i >= len(forces) {
		// A boost confirms first (#70): the picker's cursor carries the
		// force into the confirmation.
		m.ask("boost", (*Model).boostConfirm, (*Model).confirmBoost)
		return
	}
	if err := m.sess.SendEnforcers(c.ID, forces[i]); err != nil {
		m.refuse("Can't send them: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Enforcers go to %s tonight: %s. Odds %s, heat +%.0f.", c.Name, forces[i], m.oddsWord(m.factionOf(c), c, forces[i]), m.rules.Rivals.StrikeHeat(c, forces[i])))
}

func (m *Model) viewStrike() string {
	c := m.mapSelected()
	if c == nil {
		return m.modal("SEND ENFORCERS", []string{"Nowhere to send them."}, m.modalFooter())
	}
	rows := m.strikeRows()
	m.pick.cursor = max(0, min(m.pick.cursor, len(rows)-1))
	fac := m.factionOf(c)
	body := []string{theme.Subtle.Render(fmt.Sprintf("%s vs %s on %s, muscle %s", plural(m.w.Crew.Role(game.RoleEnforcer), "enforcer"), m.rivalName(fac), c.Name, m.defenceWord(fac))), ""}
	var cells [][]any
	b := m.rules.Rivals.BoostTuning()
	for i, r := range rows {
		switch {
		case i < len(forces):
			f := forces[i]
			cells = append(cells, []any{r, m.oddsWord(fac, c, f), fmt.Sprintf("%+.0f", m.rules.Rivals.StrikeHeat(c, f)), signed{m.cfg.Rivals.ForceFor(f).War}, "the corner"})
		case i < 2*len(forces):
			// A boost (#70): the same odds at the force, for the till.
			f := forces[i-len(forces)]
			cells = append(cells, []any{r, m.oddsWord(fac, c, f), fmt.Sprintf("%+.0f", m.rules.Rivals.BoostHeat(c)), signed{b.War}, "~" + cash(m.rules.Rivals.BoostTake(m.w, *c))})
		default:
			cells = append(cells, []any{r, nil, nil, nil, nil})
		}
	}
	notes := []string{
		"Harder flips faster, draws more heat on you, adds to the war",
		"and costs the enforcers' nerve. A loud enough war brings a",
		"crackdown on both sides. A boost takes the till, not the corner.",
	}
	if _, _, _, ok := m.known().Muscle(fac.Faction()); !ok {
		notes = append(notes, "You do not know their muscle: the odds read blind until you do.")
	}
	for i, l := range notes {
		notes[i] = theme.Subtle.Render(l)
	}
	return m.pickerModal("SEND ENFORCERS", body, []col{{"force", kText, 0}, {"lands", kText, 0}, {"heat", kText, 0}, {"war", kInt, 0}, {"for", kText, 0}}, cells, m.pick.cursor, notes...)
}

// dials are the undercut picker's rows, in dial order (#68).
var dials = []events.Dial{events.DialQuiet, events.DialNormal, events.DialAggressive}

// undercutRows are the picker's choices: the three dials, plus calling
// off an undercut already queued on the corner.
func (m *Model) undercutRows() []string {
	rows := []string{"quiet", "normal", "aggressive"}
	if c := m.mapSelected(); c != nil {
		if _, ok := m.w.Undercutting(c.ID); ok {
			rows = append(rows, "stop")
		}
	}
	return rows
}

// askUndercut opens the dial picker for a price war on the selected
// corner, or says why there is none to fight there.
func (m *Model) askUndercut() {
	c := m.mapSelected()
	if c == nil {
		return
	}
	if c.Owner != game.OwnerRival {
		m.refuse("Can't undercut there: a price war is fought on a corner a rival holds.")
		return
	}
	if err := m.w.CanUndercut(c.ID); err != nil {
		m.refuse("Can't undercut: " + err.Error())
		return
	}
	m.pick.cursor = 1
	if d, ok := m.w.Undercutting(c.ID); ok {
		m.pick.cursor = int(d)
	}
	m.mode = modeUndercut
}

func (m *Model) confirmUndercut() {
	c := m.mapSelected()
	rows := m.undercutRows()
	m.mode = modePlay
	if c == nil {
		return
	}
	i := max(0, min(m.pick.cursor, len(rows)-1))
	if i >= len(dials) {
		m.sess.CancelUndercut(c.ID)
		m.say("Called off. " + c.Name + " sells at their price tonight.")
		return
	}
	d := dials[i]
	if err := m.sess.Undercut(c.ID, d); err != nil {
		m.refuse("Can't undercut: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Undercutting %s tonight at %s: ~%.0f units a day at %s off, %s loses ~%s a day.",
		c.Name, d, m.rules.Market.UndercutUnits(m.w, *c, d), format.Pct(m.rules.Market.PriceCut(), 0), m.factionOf(c).Leader, cash(m.undercutLoss(*c, d))))
}

// undercutLoss is what a price war on a corner at the dial costs the
// rival a day: the share taken of what the corner earns it.
func (m *Model) undercutLoss(c game.Corner, d events.Dial) int {
	return int(math.Round(m.rules.Market.Steal(m.w, c, d) * float64(m.rules.Rivals.CornerIncome(m.w, c))))
}

// undercutPriceCut is what a unit moved off the rival's corner at the
// dial makes against the street price, as a signed share: the dial's
// price less price_cut.
func (m *Model) undercutPriceCut(d events.Dial) float64 {
	return (m.rules.Market.Dial(d).Price*(1-m.rules.Market.PriceCut()) - 1) * 100
}

func (m *Model) viewUndercut() string {
	c := m.mapSelected()
	if c == nil {
		return m.modal("UNDERCUT", []string{"Nothing to undercut."}, m.modalFooter())
	}
	rows := m.undercutRows()
	m.pick.cursor = max(0, min(m.pick.cursor, len(rows)-1))
	body := []string{
		theme.Subtle.Render(fmt.Sprintf("%s on %s: worth ~%s a day to them", m.rivalName(m.factionOf(c)), c.Name, cash(m.rules.Rivals.CornerIncome(m.w, *c)))),
		theme.Subtle.Render("your corners next door " + format.Times(m.w.NextDoor(*c), 1) + ": the share taken scales with them"),
		"",
	}
	var cells [][]any
	for i, r := range rows {
		if i < len(dials) {
			d := dials[i]
			cells = append(cells, []any{r, approx{m.rules.Market.Steal(m.w, *c, d) * 100}, approx{m.rules.Market.UndercutUnits(m.w, *c, d)}, signed{m.undercutPriceCut(d)}, m.undercutLoss(*c, d)})
		} else {
			cells = append(cells, []any{r, nil, nil, nil, nil})
		}
	}
	notes := []string{
		"Tonight's orders here serve their customers too, cheap, on top",
		"of your own corners; the extra volume gluts the product. No",
		"heat, a little war and a grudge; starve a corner long enough",
		"and they push back or give it up, by temper.",
	}
	for i, l := range notes {
		notes[i] = theme.Subtle.Render(l)
	}
	return m.pickerModal("UNDERCUT", body, []col{{"dial", kDial, 0}, {"takes", kPct, 0}, {"units/day", kInt, 0}, {"off", kPct, 0}, {"they lose", kCash, 0}}, cells, m.pick.cursor, notes...)
}

// dealRules are the three lines on what a deal does and what breaks it,
// the rivals pane's RULES section.
var dealRules = []string{
	"Truce, tribute: off your corners. Split: off your side.",
	"A push or a hit under a deal breaks it: trust hits the floor and they make a call.",
	"So does a missed tribute, or walking off a split corner; the others hear of it.",
}

// dealDoes is what a deal of the kind does while it holds.
func dealDoes(kind string) string {
	switch kind {
	case game.DealTruce:
		return "Neither side pushes; no undercutting, no tips, for the term."
	case game.DealTribute:
		return "You pay the cut each night; they leave every corner of yours alone until you stop."
	case game.DealSplit:
		return "They neither claim nor push on your side of the line, and you post nobody past it."
	case game.DealHomage:
		return "They pay you the cut each night out of their chest and stay off your corners; it ends when they cannot pay."
	}
	return "Half the cost of a run, half the loss."
}

// dealBreaks is what breaks a deal of the kind and what that costs.
func dealBreaks(kind string) string {
	how := "A push or a hit under it"
	switch kind {
	case game.DealTribute:
		how = "A missed night, a push or a hit"
	case game.DealSplit:
		how = "Walking off a split corner, a push or a hit"
	}
	return how + " breaks it: trust hits the floor and they call the police."
}

// moodLine is where you stand with the rival: what trust buys you, or,
// after a betrayal, how long they are not taking your calls (bad).
func (m *Model) moodLine(r *game.RivalState) (line string, bad bool) {
	w := m.w
	switch {
	case r.Absorbed > 0 && r.AbsorbedBy == "":
		return fmt.Sprintf("Scattered on day %d. There is nobody left to talk to.", r.Absorbed), false
	case r.Absorbed > 0:
		return fmt.Sprintf("Absorbed on day %d. There is nobody left to talk to.", r.Absorbed), false
	case r.Fragmented > 0:
		return fmt.Sprintf("Leaderless since day %d. Their corners go back to the street.", r.Fragmented), false
	case m.rules.Rivals.Distrusted(r, w.Day+1):
		return fmt.Sprintf("You broke a deal. They take nothing for %s.", plural(r.Betrayed+m.rules.Rivals.Diplomacy().DistrustDays-w.Day-1, "more day")), true
	case r.Trust >= 60:
		return "They take you at your word. A deal is cheap to strike.", false
	case r.Trust < 20:
		return "They do not trust you. Keep a deal a while and that changes.", false
	default:
		return "Trust grows a little every day a deal holds and falls with every strike.", false
	}
}

// rivalsDetails is the rivals screen's pane: the offer under the cursor
// or, with none, the deal that holds or where you stand with the rival;
// then RULES, LIFETIME and the keys.
func (m *Model) rivalsDetails() []section {
	w := m.w
	r := m.faction()
	if r.Scouting() {
		e := m.rules.Rivals.Expansion()
		lines := wrapped(theme.Subtle, fmt.Sprintf("Your take in %s drew %s. Taking the free corners there leaves them less room; a tribute keeps them off yours; a take under %s over %s before they recruit sends them home; once they recruit they come anyway.", w.CityName(r.ScoutingCity), m.rivalName(r), money(e.TakeMin), plural(e.WindowDays, "day")))
		if r.ScoutsHit == 0 {
			lines = append(lines, keyRow("h", fmt.Sprintf("hit the scouts: +%s, a grudge", plural(e.SetbackDays, "day"))))
		}
		return []section{{"ON THE WAY", lines}}
	}
	if r.Arrived == 0 {
		return []section{{"NO RIVAL", wrapped(theme.Subtle, "Nobody is contesting the city yet. When somebody does, this is where you talk to them.")}}
	}
	mood, bad := m.moodLine(r)
	moodStyle := theme.Subtle
	if bad {
		moodStyle = theme.Bad
	}
	var sel section
	switch {
	case len(w.Offers) > 0:
		o := w.Offers[max(0, min(m.dealCursor, len(w.Offers)-1))]
		who := "theirs"
		if f := w.Faction(o.With()); f != nil && f != r {
			who = f.Leader + "'s"
		}
		lines := []string{theme.Subtle.Render(fmt.Sprintf("%s · %s to answer", who, plural(o.Expires-w.Day+1, "day")))}
		if o.Deal.Kind == game.DealTribute {
			lines = append(lines, m.tributeRows(w.Faction(o.With()), o.Deal)...)
		}
		lines = append(lines, wrapped(theme.Body, dealDoes(o.Deal.Kind))...)
		lines = append(lines, wrapped(theme.Subtle, dealBreaks(o.Deal.Kind))...)
		lines = append(lines, keyRow("a", "accept it"), keyRow("x", "turn it down"))
		sel = section{m.dealTitle(o.Deal), lines}
	case len(r.Deals) > 0:
		d := r.Deals[0]
		who, term := "yours", "until broken"
		if d.Offered {
			who = "theirs"
		}
		if d.Until > 0 {
			term = plural(d.Left(w.Day), "day") + " left"
		}
		lines := []string{row("who", fmt.Sprintf("%s, since day %d", who, d.Since)), row("term", term)}
		if d.Kind == game.DealTribute {
			lines = append(lines, m.tributeRows(r, d)...)
		}
		lines = append(lines, wrapped(theme.Body, dealDoes(d.Kind))...)
		lines = append(lines, wrapped(theme.Subtle, dealBreaks(d.Kind))...)
		sel = section{m.dealTitle(d), lines}
	default:
		sel = section{strings.ToUpper(m.rivalName(r)), wrapped(moodStyle, mood)}
	}
	// Who stands with you (#43): the defensive factions the expansionist
	// pushed toward you.
	if allies := m.rules.Rivals.Allies(w); len(allies) > 0 {
		var names []string
		for _, a := range allies {
			names = append(names, m.factionStyle(a.Faction()).Render(a.Leader))
		}
		sel.lines = append(sel.lines, row("allies", strings.Join(names, ", ")))
	}
	// The war you declared on them, short of the muscle to lose it
	// (#478): the last corner gone ends the run; with nobody to send it
	// sends nobody (#506).
	if w.War == r.Faction() {
		if line := m.warNobodyWords(); line != "" {
			sel.lines = append(sel.lines, wrapped(theme.Warning, "War: "+line)...)
		}
		if line := m.warMuscleLine(); line != "" {
			sel.lines = append(sel.lines, wrapped(theme.Bad, line)...)
		}
	}
	// A betrayal's clock is worth a line whatever is selected.
	if bad && len(w.Offers)+len(r.Deals) > 0 {
		sel.lines = append(sel.lines, wrapped(moodStyle, mood)...)
	}
	var rules []string
	for _, l := range dealRules {
		rules = append(rules, wrapped(theme.Subtle, l)...)
	}
	s := w.Stats
	life := []string{
		row("struck", fmt.Sprintf("%d · refused %d", s.Deals, s.DealsRefused)),
		row("broken", fmt.Sprintf("by you %d, by them %d", s.Betrayals, s.BetrayedBy)),
		row("tribute", cash(s.Tribute)+" paid"),
	}
	if s.Homage > 0 {
		life = append(life, row("homage", cash(s.Homage)+" to you"))
	}
	return []section{sel, {"RULES", rules}, m.booksSection(), {"LIFETIME", life}}
}

// The war order (#229, docs/rival.md): w on the rivals screen declares
// war on the faction under the cursor, after asking; while a war is on
// the same key calls it off, after asking. The rivals sim sends the
// hand's strike every night the hand leaves empty, at the file's dial
// on the faction's corner nearest your front line, and ends the war
// the night the faction folds, bows or has nothing left where you hold
// ground.

// atWar and notAtWar are the key's two labels.
func atWar(m *Model) bool    { return m.w.War != "" }
func notAtWar(m *Model) bool { return m.w.War == "" }

// warWord is the faction the war stands against, for a line: `Big Sal's
// crew`.
func (m *Model) warWord() string { return m.rivalName(m.w.Faction(m.w.War)) }

// askWar opens the declaration, or refuses with why.
func (m *Model) askWar() {
	w := m.w
	r := m.faction()
	if w.Over != nil {
		return
	}
	switch {
	case w.War != "":
		m.refuse(fmt.Sprintf("One war at a time: the enforcers are on %s. Call it off first.", m.warWord()))
	case r.Arrived == 0:
		m.refuse("Nothing to fight: nobody is contesting the city yet.")
	case r.Gone():
		m.refuse("Nothing to fight: " + m.rivalName(r) + " is no more. Turn to another faction.")
	case w.Crew.OnPayroll(game.RoleEnforcer) == 0:
		m.refuse("Nobody to send: no enforcers. Hire one " + screenPointer(screenCrew) + ".")
	case !w.WarHasGround(r):
		m.refuse(fmt.Sprintf("Nowhere to go: %s holds no corner in a city you hold ground in.", m.rivalName(r)))
	default:
		m.ask("declare war", (*Model).warConfirm, (*Model).confirmWar)
	}
}

// warConfirm is the declaration's body: the faction, the first corner,
// the dial and the odds, and what a war under a deal is.
func (m *Model) warConfirm() string {
	w := m.w
	r := m.faction()
	force, _ := m.cfg.Rivals.War.Force()
	c := m.rules.Rivals.WarTarget(w, r)
	body := m.wrapLines(fmt.Sprintf("Every night you send nobody yourself, the enforcers %s %s's nearest corner: %s tonight, odds %s, heat +%.0f. The same roll, the same toll on the crew, until they fold, bow or hold nothing left where you do.", force, m.rivalName(r), c.Name, m.oddsWord(r, c, force), m.rules.Rivals.StrikeHeat(c, force)))
	if w.AtPeaceWith(r.Faction()) {
		body = append(body, "", theme.Bad.Render("You have a deal with them: the first night breaks it, and the table remembers."))
	}
	if line := m.warMuscleLine(); line != "" {
		body = append(body, "")
		for _, l := range m.wrapLines(line) {
			body = append(body, theme.Bad.Render(l))
		}
	}
	body = append(body, "", theme.Subtle.Render("The war ends on its own when there is nothing left to take, or when you call it off here."))
	return m.modal("WAR ON "+strings.ToUpper(m.rivalName(r))+"?", body, m.modalFooter())
}

// warNobodyWords is what a war does with no enforcer at work to send
// (#506): the war order sends nobody (rivals.Sim's warOrder takes no
// enforcer, no order), so tonight takes nothing, and the faction's own
// pushes go on. "" with an enforcer at work or no war on.
func (m *Model) warNobodyWords() string {
	if m.w.War == "" || m.w.Crew.Role(game.RoleEnforcer) > 0 {
		return ""
	}
	return "no enforcer at work, so nobody goes in tonight and nothing is taken; they can still push you. Hire one " + screenPointer(screenCrew) + "."
}

// warMuscleLine is the taken-out warning (#478): a war you declare is
// open whatever its noise, and with fewer enforcers on the payroll than
// rivals.toml [endings] taken_out_muscle the push that takes your last
// corner ends the run. "" with muscle enough or the ending boxed.
func (m *Model) warMuscleLine() string {
	need := m.cfg.Rivals.Endings.TakenOutMuscle
	if need <= 0 || m.w.Crew.OnPayroll(game.RoleEnforcer) >= need {
		return ""
	}
	return fmt.Sprintf("With fewer than %s a war you lose ends the run.", plural(need, "enforcer"))
}

// confirmWar declares it.
func (m *Model) confirmWar() {
	m.mode = modePlay
	r := m.faction()
	if err := m.sess.DeclareWar(r.Faction()); err != nil {
		m.refuse("Can't declare war: " + err.Error() + ".")
		return
	}
	m.say(fmt.Sprintf("War on %s: the enforcers go in every night from tonight.", m.rivalName(r)))
}

// askCallOffWar opens the stand-down, or refuses with why.
func (m *Model) askCallOffWar() {
	if m.w.Over != nil {
		return
	}
	if m.w.War == "" {
		m.refuse("There is no war on.")
		return
	}
	m.ask("call off war", (*Model).callOffWarConfirm, (*Model).confirmCallOffWar)
}

// callOffWarConfirm is the stand-down's body.
func (m *Model) callOffWarConfirm() string {
	body := m.wrapLines(fmt.Sprintf("Call off the war on %s: the enforcers stand down from tonight. What was taken stays taken; the grudge stays too.", m.warWord()))
	return m.modal("CALL OFF THE WAR?", body, m.modalFooter())
}

// confirmCallOffWar ends it.
func (m *Model) confirmCallOffWar() {
	m.mode = modePlay
	who := m.warWord()
	if err := m.sess.CallOffWar(); err != nil {
		m.refuse("Can't: " + err.Error() + ".")
		return
	}
	m.say(fmt.Sprintf("The war on %s is off. The enforcers stay home tonight.", who))
}

// keyStrike is the strike picker's keys.
func (m *Model) keyStrike(key string) { m.pickerKey(key, len(m.strikeRows()), m.confirmStrike) }

// keyUndercut is the undercut picker's keys. The dial turns with ←→ as
// the sale's does (#241); the rows are the notches, so the picker's
// cursor is the dial.
func (m *Model) keyUndercut(key string) {
	switch key {
	case "left", "h":
		key = "up"
	case "right", "l":
		key = "down"
	}
	m.pickerKey(key, len(m.undercutRows()), m.confirmUndercut)
}

// rivalsMove turns the rivals screen to another faction (#43) with ←→
// and walks the offers with ↑↓.
func (m *Model) rivalsMove(dx, dy int) {
	switch {
	case dx != 0:
		m.cycleFaction(dx)
	case dy < 0 && m.dealCursor > 0:
		m.dealCursor--
	case dy > 0 && m.dealCursor < len(m.w.Offers)-1:
		m.dealCursor++
	}
}

// The scouts (#341, docs/rival.md): a faction moving on a city where
// you earn and nobody lives is telegraphed in stages, and h on the
// rivals screen, with it under the cursor, sends the enforcers after
// its scouts, once, after asking: a setback and a grudge.

// onScouts is the h key's condition: the faction under the cursor is on
// its way to a city.
func onScouts(m *Model) bool { r := m.faction(); return r != nil && r.Scouting() && !r.Gone() }

// scoutsWord is a faction's move for a line: `scouting Bayport · in
// d59`, `recruiting in Bayport · in d59`; "" for one not on its way.
func (m *Model) scoutsWord(r *game.RivalState) string {
	if r == nil || !r.Scouting() {
		return ""
	}
	stage := "scouting "
	if r.Recruited > 0 {
		stage = "recruiting in "
	}
	return stage + m.w.CityName(r.ScoutingCity) + " · in " + fmt.Sprintf("d%d", m.rules.Rivals.ArriveDay(m.w, r))
}

// scoutsLines are the rivals screen's lines for a faction on its way:
// where, the stages' days, and the answers.
func (m *Model) scoutsLines(r *game.RivalState) []string {
	e := m.rules.Rivals.Expansion()
	city := m.w.CityName(r.ScoutingCity)
	arrive := m.rules.Rivals.ArriveDay(m.w, r)
	lines := []string{m.factionStyle(r.Faction()).Render(m.rivalName(r)) + theme.Subtle.Render(" · "+m.scoutsWord(r))}
	if r.Recruited == 0 {
		lines = append(lines, fmt.Sprintf("Your take in %s drew them. They recruit from day %d and move in on day %d, unless your take there falls under %s over %s first.", city, r.ScoutDay+e.ScoutDays, arrive, money(e.TakeMin), plural(e.WindowDays, "day")))
	} else {
		lines = append(lines, fmt.Sprintf("They are hiring in %s and asking your people there. They move in on day %d whatever the money does now.", city, arrive))
	}
	return append(lines, "Take the free corners there first, pay them, or hit the scouts.")
}

// offScouts is h's refusal with the cursor on a faction not on its way
// (#506): said, never silent, and pointing at one that is.
func offScouts(m *Model) string {
	for _, r := range m.w.Rivals {
		if r != nil && r.Scouting() && !r.Gone() {
			return fmt.Sprintf("Nobody to hit here: %s is not moving on a city. %s's scouts are: [ ] turns to them.", m.rivalName(m.faction()), m.rivalName(r))
		}
	}
	return "Nobody to hit: no faction is moving on a city."
}

// askHitScouts opens the hit, or refuses with why.
func (m *Model) askHitScouts() {
	w := m.w
	r := m.faction()
	switch {
	case w.Over != nil:
		return
	case !onScouts(m):
		m.refuse("Nobody to hit: " + m.rivalName(r) + " is not moving on a city.")
	case r.ScoutsHit > 0:
		m.refuse("Their scouts have been hit already: it will not work twice.")
	case w.Today.HitScouts != "":
		m.refuse("The enforcers are already out after scouts tonight.")
	case w.Crew.OnPayroll(game.RoleEnforcer) == 0:
		m.refuse("Nobody to send: no enforcers. Hire one " + screenPointer(screenCrew) + ".")
	default:
		m.ask("hit the scouts", (*Model).hitScoutsConfirm, (*Model).confirmHitScouts)
	}
}

// hitScoutsConfirm is the hit's body: the setback, the grudge, and a
// deal it breaks.
func (m *Model) hitScoutsConfirm() string {
	r := m.faction()
	e := m.rules.Rivals.Expansion()
	body := m.wrapLines(fmt.Sprintf("The enforcers run %s's scouts out of %s tonight: they are set back %s, to move in on day %d, and they will hold it against you. It works once.", m.rivalName(r), m.w.CityName(r.ScoutingCity), plural(e.SetbackDays, "day"), m.rules.Rivals.ArriveDay(m.w, r)+e.SetbackDays))
	if m.w.AtPeaceWith(r.Faction()) {
		body = append(body, "", theme.Bad.Render("You have a deal with them: this breaks it, and the table remembers."))
	}
	return m.modal("HIT "+strings.ToUpper(m.rivalName(r))+"'S SCOUTS?", body, m.modalFooter())
}

// confirmHitScouts queues it.
func (m *Model) confirmHitScouts() {
	m.mode = modePlay
	r := m.faction()
	if err := m.sess.HitScouts(r.Faction()); err != nil {
		m.refuse("Can't: " + err.Error() + ".")
		return
	}
	m.say(fmt.Sprintf("The enforcers go after %s's scouts tonight.", m.rivalName(r)))
}
