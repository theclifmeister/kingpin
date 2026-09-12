package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/game"
)

// hireOne puts a runner on the payroll by hand, the way the fixtures
// do: crew_min = 1 is tier 2's line, so the next morning enters Crew
// and opens its stage (#149).
func hireOne(m *Model) {
	m.w.Crew.Members = append(m.w.Crew.Members, game.CrewMember{ID: 800, Name: "Dre", Role: "runner", Skill: 60, Units: 120, Loyalty: 80, Nerve: 50, Wage: 50})
	m.w.Crew.NextID = 800
	m.w.Player.DirtyCash += 10_000 // the wages
}

// The morning a tier is entered and a card is dealt opens the stage,
// then the card, then the report (#149), and enter at each step never
// advances the day.
func TestStageBeforeCard(t *testing.T) {
	m := newTestModel(t, 80, 24)
	hireOne(m)
	m.w.Dilemmas.Pending = testCard(m.w.Day + 1)
	day := m.w.Day
	m.Update(key("n"))
	if m.mode != modeStage || m.stage != 2 || m.w.Day != day+1 || m.w.Tier() != 2 {
		t.Fatalf("after n: mode %v stage %d day %d tier %d", m.mode, m.stage, m.w.Day, m.w.Tier())
	}
	view := stripANSI(m.View())
	assertFits(t, m.View(), 80, 24, "stage")
	if !strings.Contains(view, "STAGE 2 · CREW") || !strings.Contains(view, "OPENED") || !strings.Contains(view, "NEXT") {
		t.Fatalf("the stage does not read as one:\n%s", view)
	}
	if strings.Contains(view, "A TEST") {
		t.Fatal("the card is up with the stage")
	}
	m.Update(key("enter"))
	if m.mode != modeCard || m.w.Day != day+1 || m.w.Dilemmas.Pending == nil {
		t.Fatalf("after the stage: mode %v day %d pending %v", m.mode, m.w.Day, m.w.Dilemmas.Pending)
	}
	m.Update(key("enter")) // decide
	m.Update(key("enter")) // the outcome
	if m.mode != modeReport || m.w.Day != day+1 {
		t.Fatalf("after the card: mode %v day %d", m.mode, m.w.Day)
	}
	m.Update(key("enter"))
	if m.mode != modePlay || m.w.Day != day+1 {
		t.Fatalf("after the report: mode %v day %d", m.mode, m.w.Day)
	}
	// Esc closes it too, onto the report when there is no card.
	m.w.Progression.Seen = nil
	m.showStage()
	if m.mode != modeStage {
		t.Fatalf("showStage: mode %v", m.mode)
	}
	m.Update(key("esc"))
	if m.mode != modeReport || m.w.Day != day+1 {
		t.Fatalf("esc on the stage: mode %v day %d", m.mode, m.w.Day)
	}
}

// The stage shows on the morning a tier is entered and never again on
// the same run: a save on the modal reopens it when continued, a save
// past it does not, and the dashboard's tier fact carries `· new` only
// while it waits.
func TestStageOnce(t *testing.T) {
	m := newTestModel(t, 80, 24)
	hireOne(m)
	m.Update(key("n"))
	if m.mode != modeStage {
		t.Fatalf("after n: mode %v", m.mode)
	}
	// Saved on the modal (the day's save is made before the morning
	// opens): a fresh model continuing the slot reopens it.
	c := newModelIn(t, m, 80, 24)
	if err := c.continueRun(m.slot); err != nil {
		t.Fatal(err)
	}
	if c.mode != modeStage || c.stage != 2 {
		t.Fatalf("continued on the stage: mode %v stage %d", c.mode, c.stage)
	}
	if view := stripANSI(c.View()); !strings.Contains(view, "STAGE 2 · CREW") {
		t.Fatalf("the continued stage does not read as one:\n%s", view)
	}
	c.Update(key("enter"))
	if c.mode != modeReport || c.w.StagePending() != 0 || !c.w.Progression.Seen[2] {
		t.Fatalf("closing the stage: mode %v pending %d seen %v", c.mode, c.w.StagePending(), c.w.Progression.Seen)
	}
	c.Update(key("enter"))
	if view := stripANSI(c.View()); strings.Contains(view, "tier Crew · new") {
		t.Fatalf("the tier fact still reads new after the stage:\n%s", view)
	}
	// Closing it saved: a continued save past it opens on the play
	// screen, and the mornings after open on the report.
	d := newModelIn(t, m, 80, 24)
	if err := d.continueRun(m.slot); err != nil {
		t.Fatal(err)
	}
	if d.mode != modePlay || d.w.StagePending() != 0 {
		t.Fatalf("continued past the stage: mode %v pending %d", d.mode, d.w.StagePending())
	}
	for i := 0; i < 3; i++ {
		endDay(t, d)
		if d.mode == modeStage {
			t.Fatalf("day %d: the stage shows again", d.w.Day)
		}
		d.Update(key("enter"))
	}
	// The mark: the tier fact reads `· new` while the stage waits, at
	// both widths, and not after.
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		r := richModel(t, size[0], size[1])
		delete(r.w.Progression.Seen, 4)
		if view := stripANSI(r.View()); !strings.Contains(view, "tier Distribution · new") {
			t.Errorf("%dx%d: the tier fact is not marked new:\n%s", size[0], size[1], view)
		}
		assertFits(t, r.View(), size[0], size[1], "dashboard with a stage pending")
		r.showStage()
		r.Update(key("enter"))
		r.Update(key("enter"))
		if view := stripANSI(r.View()); !strings.Contains(view, "tier Distribution") || strings.Contains(view, "tier Distribution · new") {
			t.Errorf("%dx%d: the tier fact after the stage:\n%s", size[0], size[1], view)
		}
	}
}

// newModelIn is a second model on the same save dir as m, for continuing
// m's slot the way a fresh start of the game would.
func newModelIn(t *testing.T, m *Model, w, h int) *Model {
	t.Helper()
	c, err := New(m.cfg)
	if err != nil {
		t.Fatal(err)
	}
	c.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return c
}

// F stops on the morning a tier is entered, before the report, with the
// stage up and the report opening with the reason.
func TestFastForwardStopsOnAStage(t *testing.T) {
	m := newTestModel(t, 80, 24)
	hireOne(m)
	day := m.w.Day
	fast(t, m, 30)
	if m.mode != modeStage || m.w.Day != day+1 {
		t.Fatalf("after F: mode %v day %d -> %d", m.mode, day, m.w.Day)
	}
	if m.fastStop != "Stopped after 1 day: a new stage." {
		t.Fatalf("stop line %q", m.fastStop)
	}
	if m.status != "Ran 1 day." {
		t.Fatalf("status %q", m.status)
	}
	assertFits(t, m.View(), 80, 24, "stage after F")
	m.Update(key("enter"))
	if got := reportLine(t, m); got != m.fastStop {
		t.Fatalf("the report opens with %q, not %q", got, m.fastStop)
	}
	// Seen, F runs on: the next stop is something else.
	closeMorning(t, m)
	fast(t, m, 3)
	if m.mode == modeStage || strings.Contains(m.fastStop, "a new stage") {
		t.Fatalf("F stopped on the stage again: mode %v %q", m.mode, m.fastStop)
	}
}

// Every tier's stage renders whole from the file's copy at 80x24 and
// 120x40: no line cut, no scrolling, the title, the blurb, every opens
// line and the next line (the closing line at the top of the ladder)
// on the screen. The copy is held to the width here.
func TestStageFits(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		m := newTestModel(t, size[0], size[1])
		m.startRun(1)
		tiers := m.cfg.Progression.Tiers
		for n := 2; n <= len(tiers); n++ {
			tier := tiers[n-1]
			m.w.Progression.Seen = map[int]bool{}
			for k := 2; k <= n; k++ {
				m.w.Reach(k, m.w.Day)
				m.w.Progression.Seen[k] = k < n
			}
			m.showStage()
			if m.mode != modeStage || m.stage != n {
				t.Fatalf("tier %d: mode %v stage %d", n, m.mode, m.stage)
			}
			what := fmt.Sprintf("%dx%d stage %d", size[0], size[1], n)
			view := m.View()
			assertFits(t, view, size[0], size[1], what)
			if body := m.stageLines(n); len(body) > m.modalRoom() {
				t.Errorf("%s: %d body lines in a room of %d: the copy needs cutting", what, len(body), m.modalRoom())
			}
			_, _, box := modalBox(t, view)
			plain := stripANSI(strings.Join(box, "\n"))
			if strings.Contains(plain, "…") || strings.Contains(plain, "more") {
				t.Errorf("%s: a line cut or a scroll mark:\n%s", what, plain)
			}
			for _, want := range append([]string{fmt.Sprintf("STAGE %d · %s", n, strings.ToUpper(tier.Name)), tier.Blurb, "OPENED", "NEXT"}, tier.Opens...) {
				if !strings.Contains(plain, want) {
					t.Errorf("%s: %q is not on the screen:\n%s", what, want, plain)
				}
			}
			next := tier.Next
			if n == len(tiers) {
				next = tier.Closing
			}
			// The next line wraps: its first words are on the screen.
			if head := strings.Join(strings.Fields(next)[:4], " "); !strings.Contains(plain, head) {
				t.Errorf("%s: the next line %q is not on the screen:\n%s", what, next, plain)
			}
			m.Update(key("enter"))
		}
	}
}

// A save from before the stage (#147's world) with tiers reached and
// none seen shows the highest once and marks the lower ones seen, so an
// old save is not walked through every modal.
func TestOldSaveSeesOneStage(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.w.Reach(2, 1)
	m.w.Reach(3, 5)
	m.w.Progression.Seen = nil
	if err := game.Save(1, m.w); err != nil {
		t.Fatal(err)
	}
	c := newModelIn(t, m, 80, 24)
	if err := c.continueRun(1); err != nil {
		t.Fatal(err)
	}
	if c.mode != modeStage || c.stage != 3 {
		t.Fatalf("continued: mode %v stage %d", c.mode, c.stage)
	}
	if view := stripANSI(c.View()); !strings.Contains(view, "STAGE 3 · TERRITORY") {
		t.Fatalf("the stage shown:\n%s", view)
	}
	c.Update(key("enter"))
	if c.mode != modePlay { // no report on a fresh save
		t.Fatalf("after the stage: mode %v", c.mode)
	}
	if seen := c.w.Progression.Seen; !seen[2] || !seen[3] || c.w.StagePending() != 0 {
		t.Fatalf("seen %v pending %d", seen, c.w.StagePending())
	}
	d := newModelIn(t, m, 80, 24)
	if err := d.continueRun(1); err != nil {
		t.Fatal(err)
	}
	if d.mode != modePlay || d.w.StagePending() != 0 {
		t.Fatalf("continued again: mode %v pending %d", d.mode, d.w.StagePending())
	}
}
