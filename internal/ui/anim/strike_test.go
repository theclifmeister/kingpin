package anim

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// TestStrikeScene (#158): the frame is one row a flip and then the
// name; a cell stands in the colour it was at the start and is the
// colour it is now at the end, the glyphs its own throughout the burn's
// front; the name is in the accent at the end and not there at the
// start; two seeds burn differently and the same seed the same.
func TestStrikeScene(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(profile)
	flips := []Flip{
		{Cell: "▪ THE YARDS", From: theme.Rivals, To: theme.Crew},
		{Cell: "▴ DOCKSIDE", From: theme.Crew, To: theme.Rivals},
	}
	s := Strike(flips, "THE YARDS", theme.Rivals, Seed(1, 3, "strike"))
	if s.Done(StrikeLength-Frame) || !s.Done(StrikeLength) {
		t.Error("Done is not at StrikeLength")
	}
	h := len(flips) + 1
	start := s.Frame(0, 40, h)
	if len(start) != h {
		t.Fatalf("%d rows, want %d", len(start), h)
	}
	for i, f := range flips {
		if open, _ := styleOf(f.From); !strings.Contains(start[i], open) {
			t.Errorf("row %d at the start is not in the colour it was: %q", i, start[i])
		}
	}
	if strings.TrimSpace(stripANSI(start[h-1])) == "THE YARDS" {
		t.Errorf("the name is in place at the start: %q", start[h-1])
	}
	// The end is the still: every word in the colour it is now (a blank
	// is plain, so a run breaks at the spaces).
	end := s.Frame(StrikeLength, 40, h)
	for i, f := range flips {
		if want := coloured(f.Cell, f.To); end[i] != want {
			t.Errorf("row %d at the end is %q, want %q", i, end[i], want)
		}
	}
	if want := coloured("THE YARDS", theme.Rivals); end[h-1] != want {
		t.Errorf("the name at the end is %q, want %q", end[h-1], want)
	}
	// Every row is its cell's width or under, every frame; a cell mid-burn
	// shows the fire's glyphs somewhere on the way.
	burnt := false
	for at := time.Duration(0); at < StrikeLength; at += Frame {
		for i, l := range s.Frame(at, 40, h) {
			if lipgloss.Width(l) > 11 {
				t.Errorf("at %v row %d is %d wide", at, i, lipgloss.Width(l))
			}
			if i < len(flips) && strings.ContainsAny(stripANSI(l), "▙█▜▀▝") {
				burnt = true
			}
		}
	}
	if !burnt {
		t.Error("no cell ever burned")
	}
	// Cut and padded to the height asked; wider than the canvas, cut.
	if f := s.Frame(0, 40, 1); len(f) != 1 {
		t.Errorf("cut to 1 row: %q", f)
	}
	if f := s.Frame(0, 40, 6); len(f) != 6 || f[5] != "" {
		t.Errorf("padded to 6 rows: %q", f)
	}
	if f := s.Frame(StrikeLength, 5, h); lipgloss.Width(f[0]) > 5 {
		t.Errorf("a 5-wide canvas: %q", f[0])
	}
	same, differ := true, false
	for at := time.Duration(0); at < StrikeLength; at += Frame {
		a := strings.Join(Strike(flips, "THE YARDS", theme.Rivals, Seed(1, 3, "strike")).Frame(at, 40, h), "\n")
		b := strings.Join(Strike(flips, "THE YARDS", theme.Rivals, Seed(2, 3, "strike")).Frame(at, 40, h), "\n")
		same = same && a == strings.Join(s.Frame(at, 40, h), "\n")
		differ = differ || a != b
	}
	if !same || !differ {
		t.Errorf("same seed same frames %v, other seed other frames %v", same, differ)
	}
}

// coloured is a line as the canvas renders it settled in one colour:
// each word styled, the blanks plain.
func coloured(s string, c lipgloss.Color) string {
	words := strings.Split(s, " ")
	for i, w := range words {
		if w != "" {
			words[i] = theme.Fg(c).Render(w)
		}
	}
	return strings.Join(words, " ")
}
