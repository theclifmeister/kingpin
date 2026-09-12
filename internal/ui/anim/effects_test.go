package anim

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The effect set's guards (#153). Every effect is walked over two
// texts, the title's six by fifty art and a one by twenty line, at the
// lengths the scenes play them: the title's and a card's.
var effectTexts = []struct {
	name string
	text Text
	over time.Duration
}{
	{"6x50", NewText(Kingpin), TitleLength},
	{"1x20", NewText("DAY 42 · SHIPMENT DUE"), 500 * time.Millisecond},
}

// plain is a frame's text with the colours stripped, one string.
func plain(ls []string) string {
	var b strings.Builder
	for _, l := range ls {
		b.WriteString(stripANSI(l))
		b.WriteByte('\n')
	}
	return b.String()
}

// walk calls f with every frame of the scene from 0 to its end.
func walk(s Scene, over time.Duration, f func(at time.Duration, frame []string)) {
	for at := time.Duration(0); at <= over; at += Frame {
		f(at, s.Frame(at, 80, 24))
	}
}

// TestEffectsAreDeterministic: every effect, the same seed, the same
// frames twice; and another seed other frames for the ones that throw
// dice (wipe, slide and print do not: the same on every seed).
func TestEffectsAreDeterministic(t *testing.T) {
	for _, name := range Names() {
		e := Effects[name]
		for _, tx := range effectTexts {
			a := e.New(tx.text, theme.Money, tx.over, Seed(3, 1, name))
			b := e.New(tx.text, theme.Money, tx.over, Seed(3, 1, name))
			c := e.New(tx.text, theme.Money, tx.over, Seed(4, 1, name))
			same, differ := true, false
			walk(a, tx.over, func(at time.Duration, fa []string) {
				fb, fc := b.Frame(at, 80, 24), c.Frame(at, 80, 24)
				same = same && strings.Join(fa, "\n") == strings.Join(fb, "\n")
				differ = differ || strings.Join(fa, "\n") != strings.Join(fc, "\n")
			})
			if !same {
				t.Errorf("%s over %s: the same seed rendered different frames", name, tx.name)
			}
			if differ != e.Dice {
				t.Errorf("%s over %s: another seed rendered other frames: %v, Dice says %v", name, tx.name, differ, e.Dice)
			}
		}
	}
}

// TestEffectsResolve: every effect's last frame is the text, in the
// accent, and nothing else on the canvas, and so is every frame after;
// the curtain's is the colour over the whole canvas.
func TestEffectsResolve(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(profile)
	for _, name := range Names() {
		e := Effects[name]
		for _, tx := range effectTexts {
			s := e.New(tx.text, theme.Money, tx.over, Seed(1, 0, name))
			if s.Done(tx.over-Frame) || !s.Done(tx.over) {
				t.Errorf("%s: Done is not at the length", name)
			}
			want := Still(tx.text, theme.Money).Frame(0, 80, 24)
			if !e.Needs.Text {
				want = Curtain(tx.text, theme.Money, tx.over, nil).Frame(tx.over, 80, 24)
				if p := plain(want); strings.Count(p, "█") != 80*24 {
					t.Errorf("%s: the curtain is not the whole canvas:\n%s", name, p)
				}
			}
			for _, at := range []time.Duration{tx.over, tx.over + Frame, 3 * tx.over} {
				got := s.Frame(at, 80, 24)
				if strings.Join(got, "\n") != strings.Join(want, "\n") {
					t.Errorf("%s over %s at %v is not the resolved text:\n%s", name, tx.name, at, plain(got))
					break
				}
			}
			// Every colour on the resolved frame is the accent.
			joined := strings.Join(s.Frame(tx.over, 80, 24), "\n")
			open := strings.Split(theme.Fg(theme.Money).Render("x"), "x")[0]
			if n := strings.Count(joined, open); n == 0 || n != strings.Count(joined, "\x1b[38;2;") {
				t.Errorf("%s over %s: the resolved frame is not all in the accent: %q", name, tx.name, joined)
			}
		}
	}
}

// TestEffectsFit: every effect at 80x24 and 120x40 over both texts,
// every frame the canvas's height and within its width, and a canvas
// too small for the text cuts it rather than wrapping.
func TestEffectsFit(t *testing.T) {
	for _, name := range Names() {
		e := Effects[name]
		for _, tx := range effectTexts {
			s := e.New(tx.text, theme.Money, tx.over, Seed(1, 0, name))
			for _, sz := range [][2]int{{80, 24}, {120, 40}, {10, 3}} {
				for at := time.Duration(0); at <= tx.over; at += Frame {
					frame := s.Frame(at, sz[0], sz[1])
					if len(frame) != sz[1] {
						t.Fatalf("%s over %s at %v: %d lines at %dx%d", name, tx.name, at, len(frame), sz[0], sz[1])
					}
					for i, l := range frame {
						if lipgloss.Width(l) > sz[0] {
							t.Fatalf("%s over %s at %v: line %d is %d wide at %dx%d: %q", name, tx.name, at, i, lipgloss.Width(l), sz[0], sz[1], stripANSI(l))
						}
					}
				}
			}
		}
	}
}

// TestEffectsReadInSixteenColours: rendered under the ANSI profile,
// every frame's uncoloured text differs from the previous frame's for
// at least the first half of the run, so the motion is in the
// characters and not the colour. The curtain has no text and is the
// colour by design; it is held to moving instead.
func TestEffectsReadInSixteenColours(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(profile)
	for _, name := range Names() {
		e := Effects[name]
		for _, tx := range effectTexts {
			s := e.New(tx.text, theme.Money, tx.over, Seed(2, 0, name))
			prev, first := "", ""
			for at := time.Duration(0); at <= tx.over/2; at += Frame {
				frame := s.Frame(at, 80, 24)
				for _, l := range frame {
					if strings.Contains(l, "38;2;") {
						t.Fatalf("%s: a truecolor sequence under the ANSI profile: %q", name, l)
					}
				}
				p := plain(frame)
				if at == 0 {
					first = p
				} else if p == prev && e.Needs.Text {
					t.Errorf("%s over %s: the frame at %v reads as the one before:\n%s", name, tx.name, at, p)
					break
				}
				prev = p
			}
			if prev == first {
				t.Errorf("%s over %s: the frame at the half is the first", name, tx.name)
			}
		}
	}
}

// Still is the text at once and done; Reverse plays an effect from its
// end; Needs reads the text and the canvas.
func TestStillReverseNeeds(t *testing.T) {
	tx := NewText("AB\nCD")
	st := Still(tx, theme.Money)
	if !st.Done(0) || plain(st.Frame(0, 4, 2)) != " AB\n CD\n" {
		t.Errorf("still: %q done %v", plain(st.Frame(0, 4, 2)), st.Done(0))
	}
	r := Reverse(Pour)(tx, theme.Money, time.Second, Seed(1, 0, "r"))
	f := Pour(tx, theme.Money, time.Second, Seed(1, 0, "r"))
	if r.Done(time.Second-Frame) || !r.Done(time.Second) {
		t.Error("reverse: Done is not at the length")
	}
	if plain(r.Frame(0, 10, 6)) != plain(f.Frame(time.Second, 10, 6)) || plain(r.Frame(time.Second, 10, 6)) != plain(f.Frame(0, 10, 6)) {
		t.Error("reverse does not play the effect from its end")
	}
	if plain(r.Frame(0, 10, 6)) != plain(st.Frame(0, 10, 6)) || strings.TrimSpace(plain(r.Frame(time.Second, 10, 6))) != "" {
		t.Errorf("a reversed pour does not open on the text and end on nothing:\n%s\n%s", plain(r.Frame(0, 10, 6)), plain(r.Frame(time.Second, 10, 6)))
	}
	n := Needs{Text: true, MinW: 10, MinH: 3}
	if !n.Met(tx, 10, 3) || n.Met(tx, 9, 3) || n.Met(tx, 10, 2) || n.Met(Text{}, 80, 24) || !(Needs{}).Met(Text{}, 0, 0) {
		t.Error("Needs.Met")
	}
	for _, name := range Names() {
		if e := Effects[name]; e.Name != name || e.New == nil || !e.Needs.Met(NewText(Kingpin), 80, 24) {
			t.Errorf("%s: bad entry, or the title does not meet its needs", name)
		}
	}
	if Effects["curtain"].Needs.Text || !Effects["decrypt"].Needs.Text {
		t.Error("the curtain needs no text; decrypt does")
	}
}

// Every effect's frame at 120x40 over the title's art, for the budget
// of a millisecond a frame: go test -bench Effects -run xxx ./internal/ui/anim
func BenchmarkEffects(b *testing.B) {
	art := NewText(Kingpin)
	for _, name := range Names() {
		s := Effects[name].New(art, theme.Money, TitleLength, Seed(1, 0, name))
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				at := time.Duration(i%45) * Frame
				s.Frame(at, 120, 40)
			}
		})
	}
}

// The title cycles the set: a pass picks off its own seed, never the
// effect of the pass before, every effect that resolves a text comes
// up over enough passes, a pinned name plays every pass, an unknown
// one cycles, and the curtain is never picked.
func TestTitleCyclesTheSet(t *testing.T) {
	seen := map[string]int{}
	prev := ""
	for pass := 0; pass < 60; pass++ {
		s, name := TitlePass(0, pass, prev, "")
		if name == prev || !Effects[name].Needs.Text || s == nil {
			t.Fatalf("pass %d: %q after %q", pass, name, prev)
		}
		seen[name]++
		prev = name
	}
	for _, n := range TitleEffects() {
		if seen[n] == 0 {
			t.Errorf("%s never came up in sixty passes", n)
		}
	}
	if seen["curtain"] > 0 || len(TitleEffects()) != len(Effects)-1 {
		t.Error("the curtain is in the title's cycle")
	}
	for pass := 0; pass < 5; pass++ {
		if _, name := TitlePass(0, pass, "wipe", "wipe"); name != "wipe" {
			t.Errorf("pinned to wipe, pass %d played %s", pass, name)
		}
		if _, name := TitlePass(0, pass, "", "curtain"); name == "curtain" {
			t.Error("pinned to the curtain, which has no text")
		}
	}
	_, a := TitlePass(0, 3, "", "nosuch")
	_, b := TitlePass(0, 3, "", "")
	if a != b {
		t.Error("an unknown pin does not cycle")
	}
	// The same pass on the same seed is the same scene.
	x, _ := TitlePass(7, 2, "", "")
	y, _ := TitlePass(7, 2, "", "")
	if strings.Join(x.Frame(time.Second, 80, 6), "\n") != strings.Join(y.Frame(time.Second, 80, 6), "\n") {
		t.Error("a pass is not deterministic")
	}
}
