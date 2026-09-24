package engine_test

import (
	"errors"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestViewCarriesTheAmbitions (#347): sixty days of the informed
// player, a plan pinned. The view lists every plan in the file's order,
// each the session's reading of it (the bar, the next step, the steps
// labelled from the file), marks the one pinned and names it under you;
// an id the game lacks is refused and "" unpins.
func TestViewCarriesTheAmbitions(t *testing.T) {
	t.Parallel()
	s, w := playedSession(t, 60)
	if err := s.PinAmbition("nope"); !errors.Is(err, game.ErrNoAmbition) {
		t.Fatalf("pinning an unknown plan: %v", err)
	}
	if err := s.PinAmbition(content.AmbitionLegit); err != nil {
		t.Fatal(err)
	}
	v := s.View()
	if v.You.Ambition != content.AmbitionLegit {
		t.Errorf("you.ambition %q", v.You.Ambition)
	}
	plans := s.Ambitions()
	if len(v.Ambitions) != len(content.AmbitionIDs) || len(plans) != len(v.Ambitions) {
		t.Fatalf("%d ambitions in the view, %d read, want %d", len(v.Ambitions), len(plans), len(content.AmbitionIDs))
	}
	for i, a := range v.Ambitions {
		p := plans[i]
		if a.ID != content.AmbitionIDs[i] || a.Progress != p.Progress() || a.Done != p.Done || len(a.Steps) != len(p.Steps) {
			t.Errorf("ambition %d: view %+v, read %+v", i, a, p)
		}
		if a.Pinned != (a.ID == w.Ambition) {
			t.Errorf("%s pinned %v with %q the plan", a.ID, a.Pinned, w.Ambition)
		}
		if a.Name == a.ID {
			t.Errorf("%s is not named from the file", a.ID)
		}
		for _, st := range a.Steps {
			if st.Label == "" || st.Label == st.ID {
				t.Errorf("%s: step %s is not labelled from the file", a.ID, st.ID)
			}
		}
	}
	if err := s.PinAmbition(""); err != nil || w.Ambition != "" {
		t.Fatalf("unpinning: %v, %q", err, w.Ambition)
	}
}
