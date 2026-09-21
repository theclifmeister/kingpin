package content

import (
	"strings"
	"testing"
)

// TestEveryEndingHasAnEpilogue (#49): every cause of game.Ending has a
// row in endings.toml with a title, an epilogue that renders over a
// full Epilogue with nothing left unfilled, and a scene line; the
// summary asks for lines; and the kingpin's reign is a function of the
// heat and the pressure, a year at least.
func TestEveryEndingHasAnEpilogue(t *testing.T) {
	cfg := MustLoad()
	e := cfg.Endings
	if len(e.Endings) != len(Causes) {
		t.Errorf("%d rows for %d causes", len(e.Endings), len(Causes))
	}
	d := Epilogue{Days: 120, City: "Eastside", Here: "Bayport", Offshore: "$750,000", Left: "$1,200", Bodies: 2, Fronts: "3 businesses", Corners: 6, Name: "Ziggy", Leader: "Sal", DA: "Marsh", Chief: "Kerr", Pages: 7, Years: "4 years", Reign: "31 days", Hot: true}
	for _, c := range Causes {
		row := e.Ending(c)
		if row == nil {
			t.Errorf("%s: no row", c)
			continue
		}
		if row.Title == "" || row.Scene == "" || e.Title(c) != row.Title {
			t.Errorf("%s: title %q scene %q", c, row.Title, row.Scene)
		}
		text, err := e.Render(c, d)
		if err != nil || text == "" || strings.Contains(text, "{{") || strings.Contains(text, "<no value>") {
			t.Errorf("%s: the epilogue did not render: %q %v", c, text, err)
		}
		d.Hot = !d.Hot
		if again, _ := e.Render(c, d); c == CauseKingpin && again == text {
			t.Errorf("%s: the epilogue reads the same hot and cool", c)
		}
	}
	if e.Render("struck_by_lightning", d); e.Title("struck_by_lightning") != "STRUCK BY LIGHTNING" || e.Won("struck_by_lightning") {
		t.Errorf("an unknown cause: title %q won %v", e.Title("struck_by_lightning"), e.Won("struck_by_lightning"))
	}
	if e.Summary.Lines < 1 || len(e.Summary.Weight) == 0 {
		t.Errorf("summary: %+v", e.Summary)
	}
	k := e.Kingpin
	if k.Reign(0, 0) != k.ReignYears || k.Reign(100, 100) != 1 || k.Reign(50, 50) != max(1, (k.ReignYears+1)/2) {
		t.Errorf("reign: %d at 0/0, %d at 100/100, %d at 50/50 of %d", k.Reign(0, 0), k.Reign(100, 100), k.Reign(50, 50), k.ReignYears)
	}
	// The file is validated at load: a cause the game lacks, a cause
	// twice, a missing cause and a template that will not parse fail.
	for name, bad := range map[string]EndingsConfig{
		"unknown": {Kingpin: k, Summary: e.Summary, Endings: append(append([]EndingConfig(nil), e.Endings...), EndingConfig{Cause: "struck_by_lightning", Title: "X", Epilogue: "x", Scene: "x"})},
		"twice":   {Kingpin: k, Summary: e.Summary, Endings: append(append([]EndingConfig(nil), e.Endings...), e.Endings[0])},
		"missing": {Kingpin: k, Summary: e.Summary, Endings: e.Endings[1:]},
		"parse":   {Kingpin: k, Summary: e.Summary, Endings: append(append([]EndingConfig(nil), e.Endings[1:]...), EndingConfig{Cause: e.Endings[0].Cause, Title: "X", Epilogue: "{{.Days", Scene: "x"})},
	} {
		if err := bad.validate(); err == nil {
			t.Errorf("%s: validate passed", name)
		}
	}
}
