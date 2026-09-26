package ui

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// The new-run dialog, the profile and the daily (#50, docs/profile.md).

var sept13 = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

// pickerModel is the rich fixture on the start menu with slot 2 empty
// and the clock stopped on 13 Sep 2026.
func pickerModel(t *testing.T) *Model {
	t.Helper()
	m := richModel(t, 80, 24)
	m.now = func() time.Time { return sept13 }
	m.mode, m.startChoice = modeStart, 1
	return m
}

// TestPickerListsEveryCharacter: the picker renders every row of
// characters.toml at 80x24 with a locked one greyed under its rule,
// the daily under them, enter on a locked row is refused with the
// rule, and the unlock in the profile opens it.
func TestPickerListsEveryCharacter(t *testing.T) {
	m := pickerModel(t)
	m.Update(key("enter"))
	if m.mode != modeNewRun || m.nr.slot != 2 || m.nr.step != 0 {
		t.Fatalf("mode %v slot %d step %d", m.mode, m.nr.slot, m.nr.step)
	}
	view := stripANSI(m.View())
	assertFits(t, m.View(), 80, 24, "picker")
	for _, ch := range m.cfg.Characters.Characters {
		if !strings.Contains(view, ch.Name) {
			t.Errorf("%s is not on the picker:\n%s", ch.Name, view)
		}
	}
	if !strings.Contains(view, "locked: end a run as a businessman") || !strings.Contains(view, "locked: end a run vanished") || !strings.Contains(view, "locked: reach Distribution") {
		t.Errorf("the rules are not on the locked rows:\n%s", view)
	}
	if !strings.Contains(view, "Daily · 13 Sep 2026") {
		t.Errorf("the daily is not on the picker:\n%s", view)
	}
	for i, ch := range m.cfg.Characters.Characters {
		if ch.Unlock.Free() {
			continue
		}
		m.nr.cursor = i
		m.Update(key("enter"))
		// Refused on the dialog, with the unlock (#500): the status bar
		// is under the modal, so enter read as doing nothing.
		rule := m.unlockRule(ch.Unlock)
		if m.nr.step != 0 || m.nr.err != ch.Name+" is locked: "+rule+" to play them." {
			t.Fatalf("%s: step %d err %q", ch.ID, m.nr.step, m.nr.err)
		}
		if view := stripANSI(m.View()); !strings.Contains(view, ch.Name+" is locked: "+rule) {
			t.Fatalf("%s: the refusal is not on the dialog:\n%s", ch.ID, view)
		}
		m.profile.Unlocks[ch.ID] = true
		m.Update(key("enter"))
		if m.nr.step != 1 {
			t.Fatalf("%s unlocked: step %d status %q", ch.ID, m.nr.step, m.status)
		}
		m.Update(key("shift+tab"))
		delete(m.profile.Unlocks, ch.ID)
	}
	// The hard DA is a line on the seed page while locked, its own
	// page once earned.
	m.nr.cursor = 0
	m.Update(key("enter"))
	if view := stripANSI(m.View()); !strings.Contains(view, "Hard DA · locked: end a run as kingpin") {
		t.Errorf("the locked toggle is not named on the seed page:\n%s", view)
	}
	if m.lastStep() != 1 {
		t.Fatalf("last step %d with the toggle locked", m.lastStep())
	}
	m.profile.Unlocks[game.HardDAID] = true
	if m.lastStep() != 2 {
		t.Fatalf("last step %d with the toggle open", m.lastStep())
	}
}

// TestNewRunStartsTheCharacter: a typed seed and a character start
// that run in the slot, the world stamped with the start; the hard DA
// page's toggle seats the law; esc on any page is the menu with
// nothing started.
func TestNewRunStartsTheCharacter(t *testing.T) {
	m := pickerModel(t)
	m.profile.Unlocks[game.HardDAID] = true
	m.Update(key("enter"))
	m.Update(key("2")) // the cook
	m.Update(key("esc"))
	if m.mode != modeStart || !game.Slots()[1].Empty {
		t.Fatalf("esc: mode %v, slot 2 empty %v", m.mode, game.Slots()[1].Empty)
	}
	m.Update(key("enter"))
	m.Update(key("2")) // a digit moves (#500): the cook
	if m.nr.step != 0 || m.nr.cursor != 1 {
		t.Fatalf("2 acted: step %d cursor %d", m.nr.step, m.nr.cursor)
	}
	m.Update(key("enter")) // the seed page
	for _, k := range []string{"4", "2"} {
		m.Update(key(k))
	}
	m.Update(key("enter"))
	if m.nr.step != 2 {
		t.Fatalf("step %d", m.nr.step)
	}
	m.Update(key("right"))
	m.Update(key("enter"))
	w := m.w
	if m.mode != modePlay || m.slot != 2 || w.Seed != 42 || w.Start.Character != "cook" || !w.Start.HardDA || w.Start.Daily != "" {
		t.Fatalf("mode %v slot %d seed %d start %+v", m.mode, m.slot, w.Seed, w.Start)
	}
	if w.Law.DA.Stance != "law_and_order" || w.Law.Chief.Personality != "zealous" || w.Crew.Chemist() == nil || w.Product(w.Home().ID, "meth") == nil {
		t.Fatalf("the start is not on the world: law %+v chemist %v", w.Law, w.Crew.Chemist())
	}
	if !strings.Contains(m.status, "The Cook") || !strings.Contains(m.status, "Seed 42") {
		t.Fatalf("status %q", m.status)
	}
	want := sim.NewWorldWith(m.cfg, 42, game.Start{Character: "cook", HardDA: true})
	if len(want.Crew.Members) != len(w.Crew.Members) || want.Crew.Members[0].Name != w.Crew.Members[0].Name {
		t.Fatal("the run is not the seed's as the character")
	}
	// N starts over as the same character; the daily's is not a daily.
	m.Update(key("N"))
	m.Update(key("y"))
	if m.w.Start.Character != "cook" || !m.w.Start.HardDA || m.w.Seed == 42 {
		t.Fatalf("N: start %+v seed %d", m.w.Start, m.w.Seed)
	}
}

// TestDailyIsTheDate: the daily starts the default character on the
// date's seed, the attempt counted in the profile on disk; a second on
// the date is practice and says so; the first's score is Best and the
// practice run never replaces it; n on a daily's summary is a run as
// the default character, not the daily again.
func TestDailyIsTheDate(t *testing.T) {
	m := pickerModel(t)
	m.Update(key("enter"))
	// The daily is the row after the six characters: 7 reaches it, and
	// enter turns to its confirmation (#500), which a second enter
	// starts.
	m.Update(key("7"))
	if m.nr.cursor != m.dailyRow() || m.nr.step != 0 {
		t.Fatalf("7: cursor %d step %d, not on the daily", m.nr.cursor, m.nr.step)
	}
	if foot := stripANSI(legend(m.modalFooter())); !strings.Contains(foot, fmtDigits(m.newRunRows())+" pick") {
		t.Fatalf("the footer's digits do not reach the daily: %q", foot)
	}
	m.Update(key("enter"))
	view := stripANSI(m.View())
	assertFits(t, m.View(), 80, 24, "daily confirmation")
	for _, want := range []string{"NEW RUN · DAILY", "Daily · 13 Sep 2026", "yes, today's first attempt", "enter start", "⇧tab back"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the confirmation lacks %q:\n%s", want, view)
		}
	}
	if m.mode != modeNewRun || m.w.Start.Daily != "" {
		t.Fatalf("the daily started without its confirm: mode %v", m.mode)
	}
	m.Update(key("shift+tab"))
	if m.nr.step != 0 || m.nr.cursor != m.dailyRow() {
		t.Fatalf("back from the confirmation: step %d cursor %d", m.nr.step, m.nr.cursor)
	}
	m.Update(key("enter"))
	m.Update(key("enter"))
	w := m.w
	if m.mode != modePlay || w.Seed != game.DailySeed(sept13) || w.Start.Character != "" || w.Start.Daily != "20260913" || w.Start.Practice {
		t.Fatalf("mode %v seed %d start %+v", m.mode, w.Seed, w.Start)
	}
	if !strings.Contains(m.status, "Daily 13 Sep 2026") || strings.Contains(m.status, "practice") {
		t.Fatalf("status %q", m.status)
	}
	p, err := game.LoadProfile(sept13)
	if err != nil || p.Attempts["20260913"] != 1 {
		t.Fatalf("the attempt is not on disk: %+v %v", p, err)
	}
	// It ends, and is scored.
	w.Offshore = 900
	w.Over = w.End(content.CauseRetired, w.Day, "")
	m.finish(false)
	view = stripANSI(strings.Join(m.summaryLines(), "\n"))
	if !strings.Contains(view, "daily 13 Sep 2026") || !strings.Contains(view, "1st of 1 run") {
		t.Errorf("the summary does not name the daily:\n%s", view)
	}
	p, _ = game.LoadProfile(sept13)
	if p.Best["20260913"].Score != 900 || len(p.Runs) != 1 || p.Runs[0].Daily != "20260913" || p.Runs[0].Practice {
		t.Fatalf("profile %+v", p)
	}
	// The second attempt is practice.
	m.Update(key("esc"))
	m.startChoice = 2
	m.Update(key("enter"))
	if view := stripANSI(m.View()); !strings.Contains(view, "a practice run") {
		t.Errorf("the picker does not say practice:\n%s", view)
	}
	m.Update(key("7")) // the daily row, after the six characters
	m.Update(key("enter"))
	if view := stripANSI(m.View()); !strings.Contains(view, "no: a practice run") {
		t.Errorf("the confirmation does not say practice:\n%s", view)
	}
	m.Update(key("enter"))
	if !m.w.Start.Practice || m.w.Seed != game.DailySeed(sept13) || !strings.Contains(m.status, "practice") {
		t.Fatalf("second attempt: start %+v status %q", m.w.Start, m.status)
	}
	m.w.Offshore = 5000
	m.w.Over = m.w.End(content.CauseRetired, m.w.Day, "")
	m.finish(false)
	if view := stripANSI(strings.Join(m.summaryLines(), "\n")); !strings.Contains(view, "daily 13 Sep 2026, practice") || !strings.Contains(view, "1st of 2 runs") {
		t.Errorf("the summary does not say practice and the rank:\n%s", view)
	}
	p, _ = game.LoadProfile(sept13)
	if p.Best["20260913"].Score != 900 || len(p.Runs) != 2 || !p.Runs[1].Practice || p.Attempts["20260913"] != 2 {
		t.Fatalf("the practice run took Best: %+v", p)
	}
	m.Update(key("N"))
	if m.mode != modePlay || m.w.Start.Daily != "" || m.w.Start.Character != "" || m.w.Seed == game.DailySeed(sept13) {
		t.Fatalf("n on the daily's summary: %+v seed %d", m.w.Start, m.w.Seed)
	}
}

// TestRunEndRecordsAndUnlocks: a run that ends is filed once (an
// ended save opened again is not filed twice), what it unlocked is on
// the summary and open on the picker, and n on the summary starts the
// same character.
func TestRunEndRecordsAndUnlocks(t *testing.T) {
	m := pickerModel(t)
	m.Update(key("enter"))
	m.Update(key("4")) // the ex-cop, locked
	m.profile.Unlocks["excop"] = true
	m.Update(key("enter"))
	m.Update(key("enter"))
	w := m.w
	if w.Start.Character != "excop" || !w.Owns("scanner") || game.Known(w).Chief() != w.Law.Chief.Personality {
		t.Fatalf("start %+v owns %v chief %q", w.Start, w.Upgrades, game.Known(w).Chief())
	}
	w.Over = w.End(content.CauseKingpin, w.Day, "")
	m.save()
	m.finish(true)
	if m.mode != modeOver || len(m.unlocked) != 2 || m.unlocked[0] != "heir" || m.unlocked[1] != game.HardDAID { // the kingpin ending opens the Heir (#232) and the toggle
		t.Fatalf("mode %v unlocked %v", m.mode, m.unlocked)
	}
	if view := stripANSI(strings.Join(m.summaryLines(), "\n")); !strings.Contains(view, "unlocked The Heir, Hard DA") || !strings.Contains(view, "1st of 1 run") {
		t.Errorf("the summary does not say what it unlocked:\n%s", view)
	}
	p, _ := game.LoadProfile(sept13)
	if len(p.Runs) != 1 || p.Runs[0].Character != "excop" || p.Runs[0].Ending != content.CauseKingpin || !p.Unlocks[game.HardDAID] {
		t.Fatalf("profile %+v", p)
	}
	// Opened again from its save: filed once.
	again := sizedModel(t, duel(), Options{Anim: false}, 80, 24) // the same home: the menu, slot 1 being full
	again.now = m.now
	if err := again.continueRun(2); err != nil {
		t.Fatal(err)
	}
	if again.mode != modeOver || len(again.unlocked) != 0 {
		t.Fatalf("mode %v unlocked %v", again.mode, again.unlocked)
	}
	p, _ = game.LoadProfile(sept13)
	if len(p.Runs) != 1 {
		t.Fatalf("filed twice: %+v", p.Runs)
	}
	if view := stripANSI(strings.Join(again.summaryLines(), "\n")); !strings.Contains(view, "1st of 1 run") {
		t.Errorf("the summary does not rank the run:\n%s", view)
	}
	again.Update(key("N"))
	if again.mode != modePlay || again.w.Start.Character != "excop" || again.w.Over != nil {
		t.Fatalf("n: mode %v start %+v", again.mode, again.w.Start)
	}
	// The menu's history line, and the toggle's page now open.
	again.Update(key("esc"))
	again.mode = modeStart
	if view := stripANSI(again.View()); !strings.Contains(view, "History · 1 run · best score $0 · 1 of 9 endings") {
		t.Errorf("the menu has no history line:\n%s", view)
	}
	again.startChoice = 2
	again.Update(key("enter"))
	if again.lastStep() != 2 {
		t.Fatalf("the hard DA is not open after the kingpin ending: %v", again.profile.Unlocks)
	}
}

// TestCorruptProfileIsSetAside: a profile that does not read is kept
// beside a fresh one and the start menu says so; a save is untouched.
func TestCorruptProfileIsSetAside(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	path, _ := game.ProfilePath()
	if err := os.WriteFile(path, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := sizedModel(t, duel(), Options{Anim: false}, 80, 24)
	if !strings.Contains(m.profileErr, "profile is corrupt") || !strings.Contains(m.profileErr, "profile.json.corrupt-") {
		t.Fatalf("err %q", m.profileErr)
	}
	m.mode = modeStart // a fresh install starts a run in slot 1; the menu is where the message shows
	view := stripANSI(m.View())
	assertFits(t, m.View(), 80, 24, "menu with the profile error")
	if !strings.Contains(view, "Profile: profile is corrupt") {
		t.Errorf("the menu does not say:\n%s", view)
	}
	if b, err := os.ReadFile(path); err != nil || !strings.Contains(string(b), `"SchemaVersion": 1`) {
		t.Fatalf("the fresh profile was not written: %q %v", b, err)
	}
	if _, err := game.LoadProfile(sept13); !errors.Is(err, nil) {
		t.Fatalf("the fresh profile does not read: %v", err)
	}
}

// fmtDigits is a footer's digit key over n rows: `1-7`.
func fmtDigits(n int) string { return "1-" + strconv.Itoa(n) }

// TestSeedTakesEveryPrintedSeed (#491): the seed field takes every seed
// the game prints (a uint64, up to 20 digits), shows all of it at 80
// columns, and a printed seed typed a key at a time replays that run; a
// number past the uint64 top is refused with the reason, never cut.
func TestSeedTakesEveryPrintedSeed(t *testing.T) {
	for _, seed := range []string{"1790406073553998979", "18446744073709551615"} {
		m := pickerModel(t)
		m.Update(key("enter"))
		m.Update(key("enter")) // the default character: the seed page
		for _, r := range seed {
			m.Update(key(string(r)))
		}
		view := stripANSI(m.View())
		assertFits(t, m.View(), 80, 24, "seed page")
		if !strings.Contains(view, "> "+seed) {
			t.Fatalf("%s is not shown whole at 80 columns:\n%s", seed, view)
		}
		m.Update(key("enter"))
		want, _ := strconv.ParseUint(seed, 10, 64)
		if m.mode != modePlay || m.w.Seed != want || !strings.Contains(m.status, "Seed "+seed) {
			t.Fatalf("typed %s: mode %v seed %d status %q err %q", seed, m.mode, m.w.Seed, m.status, m.nr.err)
		}
	}
	// Past the top: refused on the page, with why.
	m := pickerModel(t)
	m.Update(key("enter"))
	m.Update(key("enter"))
	for _, r := range "18446744073709551616" {
		m.Update(key(string(r)))
	}
	m.Update(key("enter"))
	if m.mode != modeNewRun || m.nr.step != 1 || !strings.Contains(m.nr.err, "at most 18446744073709551615") {
		t.Fatalf("over the top: mode %v step %d err %q", m.mode, m.nr.step, m.nr.err)
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "That is past the top: a seed is at most 18446744073709551615.") {
		t.Fatalf("the refusal is not on the page:\n%s", view)
	}
}
