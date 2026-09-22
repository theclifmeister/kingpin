package ui

import (
	"strings"
	"testing"
)

// TestModeTableIsComplete: every mode has a row in modes (#274), named
// once, with a key handler, and a view unless it is the play screen,
// whose view is the frame; a mode whose keys walk pages has the dialog
// state they live in. A mode added to the constants without its row
// fails here rather than falling through a switch that forgot it, the
// way hasPages forgot the spy dialog (#272).
func TestModeTableIsComplete(t *testing.T) {
	seen := map[string]mode{}
	for md := mode(0); md < modeCount; md++ {
		r := modes[md]
		if r.name == "" {
			t.Errorf("mode %d has no row", md)
			continue
		}
		if prev, ok := seen[r.name]; ok {
			t.Errorf("modes %d and %d are both %q", prev, md, r.name)
		}
		seen[r.name] = md
		if r.key == nil {
			t.Errorf("%s: no key handler", r.name)
		}
		if (r.view == nil) != (md == modePlay) {
			t.Errorf("%s: view %v, want one on every modal and none on the play screen", r.name, r.view != nil)
		}
		if r.pages != nil && r.paged == nil {
			t.Errorf("%s: its keys walk pages but it has no dialog state", r.name)
		}
	}
}

// TestScreenTableIsComplete: every screen has a row in screens (#274)
// with its names, its MAIN, its pane, its accent and its cursor.
func TestScreenTableIsComplete(t *testing.T) {
	seen := map[string]bool{}
	for s := screen(0); s < screenCount; s++ {
		r := screens[s]
		if r.name == "" || r.short == "" || r.word == "" {
			t.Errorf("screen %d: names %q %q %q", s, r.name, r.short, r.word)
			continue
		}
		if r.word != strings.ToLower(r.name) {
			t.Errorf("%s: the word is %q", r.name, r.word)
		}
		if seen[r.name] {
			t.Errorf("%s is two screens", r.name)
		}
		seen[r.name] = true
		if r.view == nil || r.details == nil || r.move == nil || r.accent == "" {
			t.Errorf("%s: view %v, details %v, move %v, accent %q", r.name, r.view != nil, r.details != nil, r.move != nil, r.accent)
		}
	}
}
