package engine_test

import (
	"reflect"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestViewCarriesTheTableAndTheEnding (#554): view 18 carries what
// Street Edition's rivals' table and ending screen read: each faction's
// city, stance and split lines as the world has them, the reign and its
// slip, the lifetime counters, the fallen, and the ending's title,
// epilogue and story, the session's own.
func TestViewCarriesTheTableAndTheEnding(t *testing.T) {
	t.Parallel()
	s, w := freshSession(t)
	warLine := s.Rules().Rivals.Tuning().WarThreshold
	v := s.View()
	if len(v.Factions) != len(w.Rivals) {
		t.Fatalf("%d factions in the view, %d in the world", len(v.Factions), len(w.Rivals))
	}
	for i, r := range w.Rivals {
		f := v.Factions[i]
		if want := w.CityOf(r); want == nil || f.City != want.ID {
			t.Errorf("%s: city %q", f.ID, f.City)
		}
		if want := w.Stance(r, warLine); f.Stance != want {
			t.Errorf("%s: stance %q, want %q", f.ID, f.Stance, want)
		}
		want := w.SplitLinesWith(r.Faction())
		if len(f.SplitLines) != len(want) {
			t.Fatalf("%s: %d split lines, want %d", f.ID, len(f.SplitLines), len(want))
		}
		for j := range want {
			if len(want[j]) > 0 && !reflect.DeepEqual(f.SplitLines[j], want[j]) {
				t.Errorf("%s: split line %d = %v, want %v", f.ID, j, f.SplitLines[j], want[j])
			}
		}
	}
	if v.Over != nil || v.You.Reign != 0 || len(v.Fallen) != 0 {
		t.Fatalf("a fresh run: over %v, reign %d, fallen %v", v.Over, v.You.Reign, v.Fallen)
	}

	w.Reign, w.ReignSlip = 3, 1
	w.Stats.Deals, w.Stats.DealsRefused, w.Stats.Tribute, w.Stats.Homage, w.Stats.Informants, w.Stats.CrewPoached = 4, 2, 9000, 7000, 1, 3
	w.Crew.Fallen = append(w.Crew.Fallen, game.Fallen{ID: 77, Name: "Ziggy", Role: "runner", Day: 3})
	w.Rivals[0].Betrayed = 2
	w.Journal = append(w.Journal, game.Headline{Day: 1, Source: "rivals", Text: "A crew moves in."})
	w.Over = w.End(content.CauseKingpin, w.Day, "")
	v = s.View()
	if v.You.Reign != 3 || v.You.ReignSlip != 1 {
		t.Errorf("reign %d slip %d", v.You.Reign, v.You.ReignSlip)
	}
	st := v.Stats
	if st.Deals != 4 || st.DealsRefused != 2 || st.Tribute != 9000 || st.Homage != 7000 || st.Informants != 1 || st.CrewPoached != 3 {
		t.Errorf("the lifetime counters: %+v", st)
	}
	if len(v.Fallen) != 1 || v.Fallen[0].Name != "Ziggy" || v.Fallen[0].Role != "runner" || v.Fallen[0].Day != 3 {
		t.Errorf("fallen %+v", v.Fallen)
	}
	if v.Factions[0].Betrayed != 2 {
		t.Errorf("betrayed %d", v.Factions[0].Betrayed)
	}
	o := v.Over
	if o.Title != s.Config().Endings.Title(content.CauseKingpin) || !o.Won {
		t.Errorf("title %q won %v", o.Title, o.Won)
	}
	if o.Epilogue == "" || o.Epilogue != s.Epilogue() {
		t.Errorf("epilogue %q, the session's %q", o.Epilogue, s.Epilogue())
	}
	if o.Reached != 3 {
		t.Errorf("a kingpin's reached day is the reign's first morning: %d", o.Reached)
	}
	story := s.Story()
	if len(o.Story) != len(story) {
		t.Fatalf("%d story lines, the session's %d", len(o.Story), len(story))
	}
	for i, h := range story {
		if o.Story[i].Day != h.Day || o.Story[i].Text != h.Text {
			t.Errorf("story %d = %+v, want %+v", i, o.Story[i], h)
		}
	}
}
