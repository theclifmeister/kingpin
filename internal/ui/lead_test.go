package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// TestReportOpensWithTheLead (#354): the morning report opens with
// TODAY and the night's lead, numbered, over the sections, at 80x24 and
// up, its footer offering the lines; 1, 2 and 3 each land where the
// line is answered, the subject under the cursor, as the alert jump
// does; a night with no lead has no TODAY and no key for it. The
// journal's filter reads the lead's lines under their own source.
func TestReportOpensWithTheLead(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := richModel(t, size[0], size[1])
		w := m.w
		member, corner := w.Crew.Members[0], w.Here().Corners[1]
		w.Report.Lead = []game.Line{
			{Kind: "crew_lost", Text: "Lost " + member.Name + " (arrested).", Act: game.Act{Screen: game.ScreenCrew, Subject: game.OnMember}, Member: member.ID},
			{Kind: "corner_lost", Text: "Lost a corner: " + corner.Name + ".", Act: game.Act{Screen: game.ScreenMap, Subject: game.OnCorner}, Corner: corner.ID, City: corner.City},
			{Kind: "flow", Text: "Profit fell 40% on the week: +$6,000 against +$10K a night.", Act: game.Act{Screen: game.ScreenLedger}},
		}
		m.mode, m.fastStop, m.fastAlert = modeReport, "", nil
		view := stripANSI(m.View())
		today := strings.Index(view, "TODAY")
		first := len(view)
		for _, h := range []string{"PRICES", "SALES", "CREW", "MONEY", "NEWS", "TIER", "INCIDENT"} {
			if i := strings.Index(view, h); i >= 0 && i < first {
				first = i
			}
		}
		if today < 0 || today > first {
			t.Fatalf("%dx%d: the report does not open with TODAY:\n%s", size[0], size[1], view)
		}
		for i, l := range w.Report.Lead {
			if !strings.Contains(view, string(rune('1'+i))+" "+l.Text[:20]) {
				t.Errorf("%dx%d: line %d is not on the report:\n%s", size[0], size[1], i+1, view)
			}
		}
		if !strings.Contains(view, "1-3 open") {
			t.Errorf("%dx%d: the footer does not offer the lead:\n%s", size[0], size[1], view)
		}

		m.Update(key("1"))
		if c, _, _ := m.crewSelected(); m.mode != modePlay || m.screen != screenCrew || c.ID != member.ID {
			t.Errorf("%dx%d: 1 landed on screen %v mode %v", size[0], size[1], m.screen, m.mode)
		}
		m.mode = modeReport
		m.Update(key("2"))
		if m.mode != modePlay || m.screen != screenMap || m.mapSelected().ID != corner.ID {
			t.Errorf("%dx%d: 2 landed on screen %v mode %v", size[0], size[1], m.screen, m.mode)
		}
		m.mode = modeReport
		m.Update(key("3"))
		if m.mode != modePlay || m.screen != screenLedger {
			t.Errorf("%dx%d: 3 landed on screen %v mode %v", size[0], size[1], m.screen, m.mode)
		}

		w.Report.Lead = nil
		m.mode = modeReport
		if view := stripANSI(m.View()); strings.Contains(view, "TODAY") || strings.Contains(view, "1-3 open") {
			t.Errorf("%dx%d: a night with no lead:\n%s", size[0], size[1], view)
		}
		m.Update(key("1"))
		if m.mode != modeReport {
			t.Errorf("%dx%d: 1 with no lead left the report", size[0], size[1])
		}
	}

	m := richModel(t, 120, 40)
	m.w.Journal = append(m.w.Journal, game.Headline{Day: m.w.Day, Source: "digest", Text: "Lost a corner: Oak."})
	m.switchScreen(screenJournal)
	for m.journalFilter != "digest" {
		m.Update(key("f"))
		if m.journalFilter == "" {
			t.Fatal("the journal's filter never reads the digest")
		}
	}
	if hs := m.headlines(); len(hs) == 0 || hs[0].Text != "Lost a corner: Oak." {
		t.Errorf("the digest's lines: %+v", hs)
	}
}
