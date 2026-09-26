package engine_test

import (
	"errors"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestALumpIsReadBeforeTheWalkAway (#494): a lump moved offshore one
// night and a walk away the next morning scored the whole lump free of
// its pages. The morning after the move every way out is refused while
// the pages are due; the day ended, the heat sim files them, and the
// walk away opens again. A lump whose pages fill the file ends the run
// indicted that night instead of on the player's terms.
func TestALumpIsReadBeforeTheWalkAway(t *testing.T) {
	t.Parallel()
	lump := func(t *testing.T, lots int) (*engine.Session, *game.World, int) {
		t.Helper()
		s, w := newSession(t)
		off := s.Rules().Laundering.Offshore()
		w.Offshore, w.QuietDays = off.RetireCash, off.RetireDays
		w.Player.CleanCash = off.Lot * lots
		s.SetLieLow(true) // nothing sold: the only pages are the move's
		if err := s.Reserve(off.Lot * lots); err != nil {
			t.Fatal(err)
		}
		if n := s.PagesDue(); n != 0 {
			t.Fatalf("pages due before the night: %d", n)
		}
		s.EndDay()
		if w.Over != nil {
			t.Fatalf("the move ended the run: %+v", w.Over)
		}
		return s, w, s.Rules().Laundering.Lots(off.Lot*lots) * s.Rules().Heat.StructureEvidence()
	}

	// Three lots: two over the line, two pages, short of the file.
	s, w, pages := lump(t, 3)
	if pages <= 0 || s.PagesDue() != pages {
		t.Fatalf("the morning after: pages due %d, want %d", s.PagesDue(), pages)
	}
	w.QuietDays = s.Rules().Laundering.Offshore().RetireDays
	for name, exit := range map[string]func() error{"retire": s.Retire, "vanish": s.Vanish, "crown": s.Crown, "straight": s.GoStraight} {
		if err := exit(); !errors.Is(err, game.ErrPagesDue) || w.Over != nil {
			t.Fatalf("%s with the pages due: %v, over %+v", name, err, w.Over)
		}
	}
	before := w.Heat.Evidence
	s.EndDay()
	if w.Heat.Evidence < before+pages {
		t.Fatalf("the night after: the file %d, want at least %d", w.Heat.Evidence, before+pages)
	}
	if s.PagesDue() != 0 {
		t.Fatalf("pages still due once filed: %d", s.PagesDue())
	}
	w.QuietDays = s.Rules().Laundering.Offshore().RetireDays
	if err := s.Retire(); err != nil || w.Over == nil || w.Over.Cause != content.CauseRetired {
		t.Fatalf("retire once the pages are filed: %v %+v", err, w.Over)
	}

	// Twenty lots: nineteen pages, past the file: the night the pages
	// are read ends the run indicted, and the crown the morning after the
	// move was never on offer.
	s, w, pages = lump(t, 20)
	if pages < s.Rules().Heat.EvidenceArrest(w) {
		t.Fatalf("%d pages do not fill a file of %d", pages, s.Rules().Heat.EvidenceArrest(w))
	}
	if err := s.Retire(); !errors.Is(err, game.ErrPagesDue) {
		t.Fatalf("retire on the lump: %v", err)
	}
	s.EndDay()
	if w.Over == nil || w.Over.Cause != content.CauseIndicted {
		t.Fatalf("the lump's pages did not reach the file: over %+v, file %d", w.Over, w.Heat.Evidence)
	}
}

// TestAReadyEndingAlertsOnce (#498): each ending the player takes
// raises its alert the morning it opens, keyed so a fast-forward stops
// once: retiring once the account and the quiet days are in hand (the
// alert was keyed the same short and ready, so it never stopped), and
// vanishing once an identity is owned.
func TestAReadyEndingAlertsOnce(t *testing.T) {
	t.Parallel()
	s, w := newSession(t)
	off := s.Rules().Laundering.Offshore()
	w.Offshore = 1000
	short := s.Alerts()
	w.Offshore, w.QuietDays = off.RetireCash, off.RetireDays
	st := s.Stop(nil, short)
	if st.Kind != engine.StopAlert || st.Alert.Kind != engine.AlertRetire || !st.Alert.Ready {
		t.Fatalf("retiring opened: stop %+v", st)
	}
	if st := s.Stop(nil, s.Alerts()); st.Kind == engine.StopAlert {
		t.Fatalf("retiring stopped twice: %+v", st)
	}
	before := s.Alerts()
	for _, a := range before {
		if a.Kind == engine.AlertVanish {
			t.Fatalf("vanishing with no identity: %+v", a)
		}
	}
	w.Upgrades["identity"] = true
	st = s.Stop(nil, before)
	if st.Kind != engine.StopAlert || st.Alert.Kind != engine.AlertVanish {
		t.Fatalf("vanishing opened: stop %+v", st)
	}
	if st := s.Stop(nil, s.Alerts()); st.Kind == engine.StopAlert {
		t.Fatalf("vanishing stopped twice: %+v", st)
	}
}
