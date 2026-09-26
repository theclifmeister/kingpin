package engine

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestRepeatedOfferRunsPast (#469): a faction's offer stops a
// fast-forward the first time; the same faction offering the same kind
// of deal again within OfferQuiet (the last let lapse or turned down)
// runs past, however often it asks; a different kind, another faction
// or an offer after a quiet spell stops. A load starts the memo over.
func TestRepeatedOfferRunsPast(t *testing.T) {
	t.Parallel()
	s, err := New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	w := s.NewRun(3, game.Start{})
	stops := func(day int, faction, deal string) bool {
		ev := events.DealOffered{Day: day, Faction: faction, Rival: "Rosalind", Deal: deal}
		s.remember([]events.Event{ev})
		w.Dilemmas.Pending = nil
		return s.Stop([]events.Event{ev}, s.Alerts()).Kind == StopEvent
	}
	if !stops(10, "f2", game.DealTribute) {
		t.Fatal("the first offer did not stop")
	}
	for day := 12; day < 40; day += 2 {
		if stops(day, "f2", game.DealTribute) {
			t.Fatalf("the tribute asked again on day %d stopped", day)
		}
	}
	if !stops(40, "f2", game.DealTruce) {
		t.Fatal("a truce after the tributes did not stop")
	}
	if !stops(41, "f3", game.DealTribute) {
		t.Fatal("another faction's tribute did not stop")
	}
	if !stops(38+OfferQuiet+1, "f2", game.DealTribute) {
		t.Fatal("a tribute after a quiet spell did not stop")
	}
	s.Attach(w)
	if !stops(38+OfferQuiet+2, "f2", game.DealTribute) {
		t.Fatal("a tribute after a fresh attach did not stop")
	}
}

// TestPagesWithNoBust (#492): the night's growth in the DA's file less
// every bust's pages is an alert only when there is some and it has a
// cause: an informant (the leak count grew), a sour retiree, a tip. A
// sting's pages are none; pages the player's own move filed (an
// envelope, lumps offshore) have their own lines and no alert.
func TestPagesWithNoBust(t *testing.T) {
	t.Parallel()
	s, err := New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	w := s.NewRun(3, game.Start{})
	for _, c := range []struct {
		name         string
		evs          []events.Event
		grew, leaked int
		cause        string
	}{
		{"a sting", []events.Event{events.Enforcement{Level: content.Sting, Evidence: 2}}, 2, 0, ""},
		{"an informant", nil, 1, 1, PagesInformant},
		{"an informant beside a sting", []events.Event{events.Enforcement{Level: content.Sting, Evidence: 2}}, 3, 1, PagesInformant},
		{"a sour retiree", []events.Event{events.CrewRetired{Sour: true}}, 1, 0, PagesRetiree},
		{"a tip that came back", []events.Event{events.PoliceTipped{}}, 1, 0, PagesTip},
		{"a tip that filed nothing", []events.Event{events.PoliceTipped{}}, 0, 0, ""},
		{"an envelope", []events.Event{events.BribeBackfired{}}, 1, 0, ""},
	} {
		file, leaks := w.Heat.Evidence, w.Heat.Leaks
		w.Heat.Evidence += c.grew
		w.Heat.Leaks += c.leaked
		s.filed(c.evs, file, leaks)
		var got []Alert
		for _, a := range s.Alerts() {
			if a.Kind == AlertPages {
				got = append(got, a)
			}
		}
		switch {
		case c.cause == "" && len(got) != 0:
			t.Errorf("%s: %+v", c.name, got)
		case c.cause != "" && (len(got) != 1 || got[0].Level != c.cause || got[0].Count != w.Heat.Evidence || !got[0].Danger()):
			t.Errorf("%s: %+v, want the cause %s", c.name, got, c.cause)
		}
		w.Day++ // the next morning: last night's pages are gone
		if len(s.pages()) != 0 {
			t.Errorf("%s: the pages stood a second morning", c.name)
		}
	}
}
