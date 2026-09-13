package anim

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The hold (#203): a Hold player ticks on past Done and says it is
// holding, Skip jumps its clock to the scene's end (an interstitial's
// too, over on its next tick), Replay rests then plays again on its
// period, Pulse turns the text the colour for its moment, and the three
// report scenes hold: the bust's title pulses and its level jitters,
// the morning's wipe replays, the incident prints and the beams pass,
// every held frame the scene's rows and never wider than asked.

func TestPlayerHolds(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	held := &Player{Scene: Decrypt(NewText("A"), theme.Money, 100*time.Millisecond, Seed(1, 0, "t")), Hold: true}
	for at := time.Duration(0); at < 3*time.Second; at += Frame {
		if held.Tick(now.Add(at)) {
			t.Fatalf("a hold was over at %v", at)
		}
		if held.Holding() != (at >= 100*time.Millisecond) {
			t.Fatalf("at %v Holding is %v", at, held.Holding())
		}
	}
	if len(held.Frame(10, 1)) != 1 {
		t.Error("Frame is not the scene's")
	}
	// Skip before the first tick: the first tick lands past the end.
	skipped := &Player{Scene: Decrypt(NewText("A"), theme.Money, 100*time.Millisecond, Seed(1, 0, "t")), Hold: true}
	skipped.Skip()
	if skipped.Tick(now) || !skipped.Holding() || skipped.T() < 100*time.Millisecond {
		t.Errorf("skipped before the first tick: holding %v T %v", skipped.Holding(), skipped.T())
	}
	if end := skipped.T(); end >= 100*time.Millisecond+Frame {
		t.Errorf("a skip landed %v past the end", end-100*time.Millisecond)
	} else if skipped.Tick(now.Add(Frame)); skipped.T() != end+Frame {
		t.Errorf("the clock after a skip runs from the end: T %v", skipped.T())
	}
	// Skip mid-scene, and an interstitial skipped is over on its next tick.
	p := &Player{Scene: Decrypt(NewText("A"), theme.Money, time.Second, Seed(1, 0, "t"))}
	p.Tick(now)
	p.Tick(now.Add(200 * time.Millisecond))
	p.Skip()
	if p.T() < time.Second || !p.Tick(now.Add(200*time.Millisecond+Frame)) {
		t.Errorf("an interstitial skipped at 200 ms: T %v, not over on the next tick", p.T())
	}
	// A skip on a scene that is done already changes nothing.
	at := p.T()
	p.Skip()
	if p.T() != at {
		t.Errorf("a skip past the end moved the clock %v → %v", at, p.T())
	}
}

func TestReplayPulse(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(profile)
	text := NewText("KINGPIN")
	over, rest := 300*time.Millisecond, time.Second
	r := Replay(WipeFrom(Right)(text, theme.Money, over, nil), over, rest)
	still := plain(Still(text, theme.Money).Frame(0, 10, 1))
	if !r.Done(0) {
		t.Error("a replay is not done from t = 0, as a Held is")
	}
	// The rest is the resolved frame; the pass starts from nothing at
	// the rest's end and is the wipe's frame on its own clock; then the
	// rest again.
	for _, at := range []time.Duration{0, rest / 2, rest - Frame, rest + over, rest + over + rest/2, 2 * (rest + over)} {
		if got := plain(r.Frame(at, 10, 1)); got != still {
			t.Errorf("at %v the replay rests on %q, not the resolved frame %q", at, got, still)
		}
	}
	if got := plain(r.Frame(rest, 10, 1)); got == still || strings.TrimSpace(got) != "" {
		t.Errorf("at the rest's end the wipe starts on %q", got)
	}
	w := WipeFrom(Right)(text, theme.Money, over, nil)
	for at := time.Duration(0); at < over; at += Frame {
		if a, b := strings.Join(r.Frame(rest+at, 10, 1), ""), strings.Join(w.Frame(at, 10, 1), ""); a != b {
			t.Errorf("the pass at %v is %q, the wipe's %q", at, a, b)
		}
		if a, b := strings.Join(r.Frame(2*rest+over+at, 10, 1), ""), strings.Join(w.Frame(at, 10, 1), ""); a != b {
			t.Errorf("the second pass at %v is %q, the wipe's %q", at, a, b)
		}
	}
	gold, red := theme.Fg(theme.Money).Render("KINGPIN"), theme.Fg(theme.Heat).Render("KINGPIN")
	p := Pulse(text, theme.Money, theme.Heat, 300*time.Millisecond, time.Second)
	if !p.Done(0) {
		t.Error("a pulse is not done from t = 0")
	}
	for _, at := range []time.Duration{0, 699 * time.Millisecond, time.Second, 1699 * time.Millisecond} {
		if f := p.Frame(at, 10, 1)[0]; !strings.Contains(f, gold) {
			t.Errorf("at %v the pulse is %q, not gold", at, f)
		}
	}
	for _, at := range []time.Duration{700 * time.Millisecond, 999 * time.Millisecond, 1700 * time.Millisecond} {
		if f := p.Frame(at, 10, 1)[0]; !strings.Contains(f, red) {
			t.Errorf("at %v the pulse is %q, not red", at, f)
		}
	}
}

// TestBustHolds: past BustLength the bust's title is gold with the
// strobe colour once a second, the level jitters on its tape once a
// HoldRest and stands in red between, the loss stands, and every held
// frame is two rows within the width.
func TestBustHolds(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(profile)
	const title, level, loss = "MORNING REPORT · DAY 42", "RAID", ": lost 40 Weed and $2,000 in Eastside"
	s := Bust(title, level, loss, false, Seed(1, 0, "bust"))
	red, gold := theme.Fg(theme.Heat).Render("MORNING"), theme.Fg(theme.Money).Render("MORNING")
	last := s.Frame(BustLength, 72, 2)
	reds, jitters := 0, 0
	for at := time.Duration(0); at < 2*HoldRest+bustJitter; at += Frame {
		f := s.Frame(BustLength+at, 72, 2)
		if len(f) != 2 || lipgloss.Width(f[0]) > 72 || lipgloss.Width(f[1]) > 72 {
			t.Fatalf("at %v the held frame is %d rows, %d and %d wide", at, len(f), lipgloss.Width(f[0]), lipgloss.Width(f[1]))
		}
		switch {
		case strings.Contains(f[0], red):
			reds++
		case !strings.Contains(f[0], gold):
			t.Errorf("at %v the title is %q, neither gold nor red", at, f[0])
		}
		if f[1] != last[1] {
			jitters++
			if at < HoldRest {
				t.Errorf("at %v the level moved before the first rest was up: %q", at, stripANSI(f[1]))
			}
		}
	}
	passes := int((2*HoldRest + bustJitter) / HoldPulse)
	if reds < passes*frames(holdPulseOn)-passes || reds > passes*frames(holdPulseOn)+passes {
		t.Errorf("the title was red on %d frames over %d pulses", reds, passes)
	}
	if jitters < 2 || jitters > 2*frames(bustJitter) {
		t.Errorf("the level jittered on %d frames over two rests", jitters)
	}
	if got := stripANSI(s.Frame(BustLength+HoldRest/2, 72, 2)[1]); got != "  "+level+loss {
		t.Errorf("at rest the line is %q", got)
	}
}

// TestMorningHolds: past MorningLength the counter stays today's and
// the title wipes in again once a HoldRest, whole between.
func TestMorningHolds(t *testing.T) {
	s := Morning(41, 42, "MORNING REPORT · DAY 42", Seed(1, 0, "morning"))
	whole := stripANSI(s.Frame(MorningLength, 40, 2)[1])
	wipes := 0
	for at := time.Duration(0); at < 2*(HoldRest+MorningLength); at += Frame {
		f := s.Frame(MorningLength+at, 40, 2)
		if got := stripANSI(f[0]); got != "42" {
			t.Errorf("at %v the counter reads %q", at, got)
		}
		if got := stripANSI(f[1]); got != whole {
			wipes++
			if !strings.HasPrefix(whole, strings.TrimRight(got, " ")) {
				t.Errorf("at %v the title is %q, not a wipe of %q", at, got, whole)
			}
		}
	}
	if wipes < 2 || wipes > 2*frames(MorningLength) {
		t.Errorf("the title wiped on %d frames over two rests", wipes)
	}
	if got := stripANSI(s.Frame(MorningLength+HoldRest/2, 40, 2)[1]); got != whole {
		t.Errorf("at rest the title is %q", got)
	}
}

// TestIncident: the line prints in at the report's indent in
// theme.World over IncidentLength, Done there, and holds: the beams
// pass once a HoldRest and the line stands whole between; the frame is
// h rows whatever h; the dice (the beams') tell two seeds apart in the
// hold and the print is the same on both.
func TestIncident(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(profile)
	const line = "A hurricane shuts the boat routes out of Bayport for 4 nights."
	s := Incident(line, Seed(1, 0, "incident"))
	if s.Done(IncidentLength-Frame) || !s.Done(IncidentLength) {
		t.Errorf("the scene is not %v long", IncidentLength)
	}
	first := stripANSI(s.Frame(0, 72, 1)[0])
	if strings.Contains(first, "hurricane") {
		t.Errorf("the line is up at the first frame: %q", first)
	}
	last := s.Frame(IncidentLength, 72, 1)
	if got := stripANSI(last[0]); got != "  "+line {
		t.Errorf("the last frame is %q", got)
	}
	if !strings.Contains(last[0], theme.Fg(theme.World).Render("A")) {
		t.Errorf("the line does not settle in theme.World: %q", last[0])
	}
	if mid := stripANSI(s.Frame(IncidentLength/2, 72, 1)[0]); len(strings.TrimSpace(mid)) == 0 || mid == "  "+line {
		t.Errorf("halfway the print is %q", mid)
	}
	for _, h := range []int{0, 1, 3} {
		if n := len(s.Frame(IncidentLength/2, 72, h)); n != h {
			t.Errorf("%d rows asked, %d given", h, n)
		}
	}
	moved := 0
	for at := time.Duration(0); at < 2*(HoldRest+incidentBeams); at += Frame {
		f := s.Frame(IncidentLength+at, 72, 1)
		if len(f) != 1 || lipgloss.Width(f[0]) > 72 {
			t.Fatalf("at %v the held frame is %d rows, %d wide", at, len(f), lipgloss.Width(f[0]))
		}
		if f[0] != last[0] {
			moved++
			if at < HoldRest {
				t.Errorf("at %v the beams passed before the first rest was up", at)
			}
		}
	}
	if moved < 10 || moved > 2*frames(incidentBeams) {
		t.Errorf("the line moved on %d frames over two rests", moved)
	}
	if got := stripANSI(s.Frame(IncidentLength+HoldRest/2, 72, 1)[0]); got != "  "+line {
		t.Errorf("at rest the line is %q", got)
	}
	a, b := incidentScene(3), incidentScene(4)
	same := true
	for at := time.Duration(0); at < IncidentLength; at += Frame {
		if strings.Join(a.Frame(at, 72, 1), "") != strings.Join(b.Frame(at, 72, 1), "") {
			t.Fatalf("at %v the print differs between seeds", at)
		}
	}
	for at := IncidentLength + HoldRest; at < IncidentLength+HoldRest+incidentBeams; at += Frame {
		same = same && strings.Join(a.Frame(at, 72, 1), "") == strings.Join(b.Frame(at, 72, 1), "")
	}
	if same {
		t.Error("two seeds rendered the same beams")
	}
}

// BenchmarkHeld: a held frame of each scene that holds, at the modal's
// width, through its loop's pass; the budget is BenchmarkEffects'
// millisecond.
func BenchmarkHeld(b *testing.B) {
	for _, sc := range Scenes() {
		if !sc.Hold {
			continue
		}
		s := sc.New(1)
		b.Run(sc.Name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				at := sc.Length + HoldRest + time.Duration(i%60)*Frame
				s.Frame(at, 76, 2)
			}
		})
	}
}
