package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The new-run dialog (#50, docs/profile.md): enter on an empty slot of
// the start menu opens it, one modal with three pages. The first is
// the character picker: every row of characters.toml, a locked one
// greyed with its rule, and the daily under them (the date's seed, the
// default character, one scored attempt a date); the second the seed,
// a number field (blank is random); the third, once the profile has
// earned it, the hard DA toggle. enter on the daily starts at once;
// enter on the last page starts the run in the slot. Nothing here
// touches a sim: what a character changes is on the world on day 0
// (sim.NewWorldWith).

// newRunDialog is the dialog's state.
type newRunDialog struct {
	stepper
	slot   int         // the slot the run starts in
	cursor int         // the row on the first page: the characters, then the daily
	seed   numberField // the typed seed; blank is random
	hard   bool        // the hard DA toggle
}

// field is the seed on its page (#243); back keeps it, a seed being
// nobody's step's.
func (d *newRunDialog) field() *numberField {
	if d.step == 1 {
		return &d.seed
	}
	return nil
}

// openNewRun opens the dialog for the slot on its first page, the
// cursor on the default character.
func (m *Model) openNewRun(slot int) {
	m.nr = newRunDialog{slot: slot, seed: newNumberField("random")}
	m.nr.seed.Focus()
	m.mode = modeNewRun
}

// newRunRows is the first page: one a character, then the daily.
func (m *Model) newRunRows() int { return len(m.cfg.Characters.Characters) + 1 }

// dailyRow is the index of the daily's row.
func (m *Model) dailyRow() int { return len(m.cfg.Characters.Characters) }

// hardDAOpen reports whether the toggle's page is in the dialog: the
// profile has earned it.
func (m *Model) hardDAOpen() bool {
	return m.profile.Unlocked(m.cfg.Characters.HardDA.Unlock, game.HardDAID)
}

// lastStep is the dialog's last page: the toggle's where it is open,
// the seed's otherwise.
func (m *Model) lastStep() int {
	if m.hardDAOpen() {
		return 2
	}
	return 1
}

// unlockRule is a rule in words, for a locked row: `end a run vanished`,
// `reach Distribution`.
func (m *Model) unlockRule(r content.UnlockRule) string {
	switch {
	case r.Ending != "":
		ending := strings.ToLower(m.cfg.Endings.Title(r.Ending))
		if strings.HasPrefix(ending, "a ") || r.Ending == content.CauseKingpin {
			ending = "as " + ending
		}
		return "end a run " + ending
	case r.Stage != "":
		if i := m.cfg.Progression.Index(r.Stage); i >= 0 {
			return "reach " + m.cfg.Progression.Tiers[i].Name
		}
	}
	return "always"
}

// characterOpen reports whether the character is unlocked.
func (m *Model) characterOpen(ch content.CharacterConfig) bool {
	return m.profile.Unlocked(ch.Unlock, ch.ID)
}

// keyNewRun is the dialog's keys: the cursor on the first page (a
// locked row is refused with its rule), the number field on the
// second, left and right on the toggle, enter forward and on the last
// page the start, shift+tab back, esc closes to the menu.
func (m *Model) keyNewRun(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.nr
	if closes(key) {
		m.mode = modeStart
		return m, nil
	}
	switch key {
	case "shift+tab":
		d.back(noField) // the seed stays: it is nobody's step's
		return m, nil
	case "tab":
		if d.step < m.lastStep() && (d.step > 0 || d.cursor != m.dailyRow()) {
			m.nextNewRun()
		}
		return m, nil
	case "enter":
		m.nextNewRun()
		return m, nil
	}
	switch d.step {
	case 0:
		rows := m.newRunRows()
		switch key {
		case "up", "k":
			d.cursor = (d.cursor + rows - 1) % rows
		case "down", "j":
			d.cursor = (d.cursor + 1) % rows
		default:
			if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
				if i := int(key[0] - '1'); i < rows {
					d.cursor = i
					m.nextNewRun() // a digit selects and commits, as in every picker (#241)
				}
			}
		}
	case 1:
		return m, d.seed.Update(k)
	case 2:
		switch key {
		case "left", "right", "h", "l", "space", " ":
			d.hard = !d.hard
		}
	}
	return m, nil
}

// nextNewRun takes enter: the first page's row (the daily starts at
// once, a locked character is refused), the seed page's number, and
// the last page starts the run.
func (m *Model) nextNewRun() {
	d := &m.nr
	switch {
	case d.step == 0 && d.cursor == m.dailyRow():
		m.startDaily()
		return
	case d.step == 0:
		ch := m.cfg.Characters.Characters[d.cursor]
		if !m.characterOpen(ch) {
			m.refuse(fmt.Sprintf("%s is locked: %s.", ch.Name, m.unlockRule(ch.Unlock)))
			return
		}
	case d.step == 1:
		if _, ok := d.seed.Number(); !ok {
			m.refuse("A seed is a whole number, or blank for a random one.")
			return
		}
	}
	if d.step < m.lastStep() {
		d.step++
		return
	}
	seed := m.freshSeed()
	if n, ok := d.seed.Number(); ok && strings.TrimSpace(d.seed.Value()) != "" {
		seed = uint64(n)
	}
	ch := m.cfg.Characters.Characters[d.cursor]
	m.slot = d.slot
	m.startRunWith(seed, game.Start{Character: ch.ID, HardDA: d.hard && m.hardDAOpen()})
}

// startDaily starts today's daily in the dialog's slot: the date's
// seed, the default character, the hard DA off, and the attempt
// counted in the profile: the first of the date is scored, a later one
// is practice.
func (m *Model) startDaily() {
	date := game.DailyKey(m.now())
	practice := m.profile.StartAttempt(date)
	m.saveProfile()
	m.slot = m.nr.slot
	m.startRunWith(game.DailySeed(m.now()), game.Start{Daily: date, Practice: practice})
	if practice {
		m.say(fmt.Sprintf("Daily %s, a practice run: today's first attempt is the one scored. Seed %d.", m.dailyDate(), m.w.Seed))
	} else {
		m.say(fmt.Sprintf("Daily %s: one attempt, scored against your history. Seed %d.", m.dailyDate(), m.w.Seed))
	}
}

// dailyDate is today's date as the dialog prints it.
func (m *Model) dailyDate() string { return m.now().UTC().Format("2 Jan 2006") }

// viewNewRun is the dialog: the picker, the seed, the toggle.
func (m *Model) viewNewRun() string {
	d := &m.nr
	var body []string
	switch d.step {
	case 0:
		chars := m.cfg.Characters.Characters
		d.cursor = max(0, min(d.cursor, len(chars)))
		for i, ch := range chars {
			open := m.characterOpen(ch)
			name, blurb := ch.Name, ch.Blurb
			if !open {
				blurb = "locked: " + m.unlockRule(ch.Unlock)
			}
			body = append(body, m.pickRow(i == d.cursor, name, blurb, open)...)
		}
		date := m.dailyDate()
		daily := "the default character on the date's seed, one scored attempt"
		if m.profile.Attempts[game.DailyKey(m.now())] > 0 {
			daily = "the date's seed again: a practice run, your first is the one scored"
		}
		body = append(body, m.pickRow(d.cursor == m.dailyRow(), "Daily · "+date, daily, true)...)
		return m.modal("NEW RUN", body, m.modalFooter())
	case 1:
		ch := m.cfg.Characters.Characters[max(0, min(d.cursor, len(m.cfg.Characters.Characters)-1))]
		body = append(body, theme.Bold.Render(ch.Name)+"  "+theme.Subtle.Render(truncate(ch.Blurb, m.modalInner()-len(ch.Name)-2)))
		body = append(body, "", row("seed", d.seed.View()))
		body = append(body, m.subtle("A seed replays a run: the same seed and the same start play the same day for day. Blank takes a random one.")...)
		if !m.hardDAOpen() {
			hd := m.cfg.Characters.HardDA
			body = append(body, "", theme.Subtle.Render(fmt.Sprintf("%s · locked: %s", hd.Name, m.unlockRule(hd.Unlock))))
		}
		return m.modal("NEW RUN · SEED", body, m.modalFooter())
	}
	hd := m.cfg.Characters.HardDA
	off, on := "off", "on"
	if d.hard {
		on = theme.DialOn.Render("[on]")
	} else {
		off = theme.DialOn.Render("[off]")
	}
	body = append(body, theme.Bold.Render(hd.Name)+"  "+off+"  "+on)
	body = append(body, "")
	body = append(body, m.wrapLines(hd.Blurb+" The stance and the temper are set the morning you start and never pinned: the elections and the chief's term run as they always do.")...)
	return m.modal("NEW RUN · HARD DA", body, m.modalFooter())
}

// pickRow is a picker row of two lines: the name, selected or not, and
// its blurb or rule under it, greyed where the row is locked.
func (m *Model) pickRow(selected bool, name, under string, open bool) []string {
	style := theme.Body
	if !open {
		style = theme.Subtle
	}
	head := "   " + style.Render(name)
	if selected {
		head = theme.Gold.Render("▸ ") + theme.Selected.Render(" "+name+" ")
	}
	return []string{truncate(head, m.modalInner()), truncate("     "+theme.Subtle.Render(truncate(under, m.modalInner()-5)), m.modalInner())}
}

// The profile (#50): loaded once with the model, written when a run
// ends and when a daily starts, never by a sim or the harness.

// loadProfile reads the profile with today's date: a corrupt one is
// set aside by game.LoadProfile and the fresh one written beside it,
// the message kept for the start menu; one from a newer build is left
// alone and the runs go unrecorded.
func (m *Model) loadProfile() {
	p, err := game.LoadProfile(m.now())
	m.profile = p
	m.profileErr = ""
	if err != nil {
		m.profileErr = "Profile: " + err.Error() + "."
		if !strings.Contains(err.Error(), game.ErrNewerProfile.Error()) {
			m.profileErr += " Starting a fresh one."
			m.saveProfile()
		}
	}
}

// saveProfile writes the profile; a failure is the status bar's.
func (m *Model) saveProfile() {
	if m.profile == nil {
		return
	}
	if err := game.SaveProfile(m.profile); err != nil {
		m.alarm("Profile not saved: " + err.Error())
	}
}

// finish opens the summary on a run that has ended: modeOver, the run
// filed in the profile once (a save of an ended run opened again is
// not filed twice: game.Profile.Record), and the ending's scene where
// the morning plays one (#156).
func (m *Model) finish(scene bool) {
	m.mode = modeOver
	m.modalScroll = 0
	m.recordRun()
	if scene {
		m.playOver()
	}
}

// recordRun files the run that ended in the profile and keeps what it
// unlocked for the summary.
func (m *Model) recordRun() {
	w := m.w
	if w == nil || w.Over == nil || m.profile == nil {
		return
	}
	rec := game.RunRecord{
		Seed: w.Seed, Character: w.Start.Character, HardDA: w.Start.HardDA, Ending: w.Over.Cause, Score: w.Stats.Score,
		Days: w.Over.Day, PeakCash: w.Stats.PeakCash, Stage: w.Stage(m.cfg.Progression), Date: game.DailyKey(m.now()),
		Daily: w.Start.Daily, Practice: w.Start.Practice,
	}
	before := len(m.profile.Runs)
	m.unlocked = m.profile.Record(m.cfg.Characters, m.cfg.Progression, rec)
	if len(m.profile.Runs) != before {
		m.saveProfile()
	}
}

// unlockNames are the names of what the run just unlocked.
func (m *Model) unlockNames() []string {
	var out []string
	for _, id := range m.unlocked {
		if ch := m.cfg.Characters.Character(id); ch != nil {
			out = append(out, ch.Name)
		} else if id == game.HardDAID {
			out = append(out, m.cfg.Characters.HardDA.Name)
		}
	}
	return out
}

// historyLine is the start menu's line on the profile: the runs, the
// best score and the endings reached; nothing with no run recorded.
func (m *Model) historyLine() string {
	p := m.profile
	if p == nil || len(p.Runs) == 0 {
		return ""
	}
	best := p.BestRun()
	return fmt.Sprintf("History · %s · best %s · %d of %d endings", plural(len(p.Runs), "run"), cash(best.Score), p.Endings(), len(content.Causes))
}

// rankLine is the summary's clause on where the run stands against
// your own history: its rank by score among the runs recorded, this
// one included (`1st of 1 run` on the first); `not on the record`
// where the profile could not be written.
func (m *Model) rankLine() string {
	rank, of := m.profile.Rank(m.w.Stats.Score)
	if of == 0 {
		return "not on the record"
	}
	return fmt.Sprintf("%s of %s", ordinal(rank), plural(of, "run"))
}

// unlockedLine is what the summary's first line adds (#50): the
// daily's date and whether the run was practice, and what it unlocked;
// empty for a run that was neither.
func (m *Model) unlockedLine() string {
	w := m.w
	var parts []string
	switch {
	case w.Start.Daily != "" && w.Start.Practice:
		parts = append(parts, "daily "+dailyWord(w.Start.Daily)+", practice")
	case w.Start.Daily != "":
		parts = append(parts, "daily "+dailyWord(w.Start.Daily))
	}
	if names := m.unlockNames(); len(names) > 0 {
		parts = append(parts, "unlocked "+strings.Join(names, ", "))
	}
	if len(parts) == 0 {
		return ""
	}
	return theme.Gold.Render(" · " + strings.Join(parts, " · "))
}

// dailyWord is a daily's date key as the summary prints it.
func dailyWord(key string) string {
	if t, err := time.Parse("20060102", key); err == nil {
		return t.Format("2 Jan 2006")
	}
	return key
}
