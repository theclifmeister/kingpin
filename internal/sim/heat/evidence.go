package heat

import (
	"fmt"
	"math"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// ForfeitEvidence is the pages a deed the DA seized files the morning
// after (#194): what the property dialog warns with.
func (s *Sim) ForfeitEvidence() int { return s.deed.ForfeitEvidence }

// StructureEvidence is the pages a lot of clean cash moved offshore
// over the line files the morning after (#195): what the reserve dialog
// warns with.
func (s *Sim) StructureEvidence() int { return s.cfg.Heat.StructureEvidence }

// EvidenceArrest is how thick the DA's file has to be for an indictment,
// after a retained lawyer has had his say and for the DA in office: a
// law-and-order DA needs fewer pages, a reformer more, a bought one
// (#42) more again, never under one.
func (s *Sim) EvidenceArrest(w *game.World) int {
	base := max(s.cfg.Heat.EvidenceArrest, s.Effects(w).EvidenceArrest)
	if base <= 0 {
		return 0
	}
	v := float64(base) * s.DA(w).EvidenceArrest
	// A bought DA sits on the file (#42): more pages before it is a case.
	if mul := s.law.Effects.BribedDAEvidenceMul; mul > 0 && w.Law.DABoughtOn(w.Day) {
		v *= mul
	}
	return max(1, int(math.Round(v)))
}

// file puts pages in the DA's file and stamps the day it last grew (the
// retainer's clock, cold); with a why it writes "why (n)" in a city's
// report, n the file's thickness after (#275: the one shape every page
// Step files takes). An audit's pages pass no why: its line rides the
// audit's heat instead. The case going cold is not this shape (a page
// off, its own line) and stays by hand in cold, as a response's pages
// do in fire.
func (d *day) file(city string, pages int, why string) {
	d.h.Evidence += pages
	d.h.EvidenceDay = d.t.Day
	if why != "" {
		d.reasons[city] = append(d.reasons[city], fmt.Sprintf("%s (%d)", why, d.h.Evidence))
	}
}

// informants is an informant on the payroll. The crew sim's turn event
// starts the clock (the laundering sim, stepping after this one, flips
// an accountant without it: the clock then runs from today, the last day
// nobody was talking); every informant_days after that the DA gets a
// page whatever was sold, and the lawyer cannot thin a witness. The heat
// it adds, where you are, is left out of the reasons on purpose: a delta
// the dial does not explain, and a file that grew without a bust, are
// the tells. Once nobody is talking the count that shows them resets.
// A flipped lieutenant is the same clock with thicker pages: they know
// where everything is.
func (s *Sim) informants(d *day) {
	w, t, tun, h := d.w, d.t, d.tun, d.h
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CrewTurnedInformant:
			h.LeakDay = ev.Day
		case events.LieutenantFlipped:
			h.LeakDay = ev.Day
		case events.CrewRetired:
			// A sour retiree talks on the way out (#46): one page, the
			// informant's, whatever was sold, the exception applied
			// once. Somebody on your payroll did something.
			if ev.Sour && tun.InformantEvidence > 0 {
				d.file(d.here, tun.InformantEvidence, fmt.Sprintf("%s talked on the way out: the DA's file on you grows", ev.Name))
			}
		}
	}
	if w.Crew.Informants() == 0 {
		h.Leaks = 0
		h.LeakDay = t.Day
	} else if tun.InformantDays > 0 && t.Day-h.LeakDay >= tun.InformantDays {
		h.LeakDay = t.Day
		h.Leaks++
		d.add(d.here, tun.InformantHeat, "")
		pages := tun.InformantEvidence
		for _, m := range w.Crew.Members {
			if m.Informant && m.Lieutenant() {
				pages = max(pages, s.lt.Evidence)
			}
		}
		if pages > 0 {
			d.file(d.here, pages, "the DA's file on you grows")
		}
	}
}

// envelopes is the bribes' pages. An envelope that blew up last night
// (#42, w.Law.Backfired: the law sim steps after this one, so its night
// is our morning) is heat where you are and pages in the file whatever
// was sold: bribing is something you did, the second bend in #27 besides
// the informant's. The DA's file on your envelopes (w.Law.Filed) is
// pages the same way.
func (s *Sim) envelopes(d *day) {
	w, t, here := d.w, d.t, d.here
	if b := s.law.Bribes; w.Law.Backfired > 0 && w.Law.Backfired == t.Day-1 {
		d.add(here, b.BackfireHeat, "the envelope came back")
		if b.BackfireEvidence > 0 {
			d.file(here, b.BackfireEvidence, "the bribe backfired: the DA's file on you grows")
		}
	}
	// The favour called in this morning (#228, w.Law.FavourOwed, the
	// night it is owed on, this tick's): the chief's name is in your ledger now, and the
	// DA's file gains favour_evidence pages whatever was sold, the
	// third bend in #27 beside the informant's and the backfire's,
	// because calling it was something you did. The response it stops
	// is in respond, with the ladder.
	if b := s.law.Bribes; w.Law.FavourOwed == t.Day && b.FavourEvidence > 0 {
		d.file(here, b.FavourEvidence, "the favour: the chief's name is in your ledger, and the DA's file on you grows")
	}
	if b := s.law.Bribes; w.Law.Filed > 0 && w.Law.Filed == t.Day-1 && b.LeadEvidence > 0 {
		d.file(here, b.LeadEvidence, "the DA opened a file on your envelopes: the DA's file on you grows")
	}
}

// forfeiture is a deed the DA seized last night (#194, w.Law.Forfeited,
// the same way as the envelopes): the money had no story, and buying the
// block was something you did. Pages where you are, whatever was sold.
func (s *Sim) forfeiture(d *day) {
	w, t := d.w, d.t
	if w.Law.Forfeited > 0 && w.Law.Forfeited == t.Day-1 && s.deed.ForfeitEvidence > 0 {
		d.file(d.here, s.deed.ForfeitEvidence, "the forfeiture: the DA's file on you grows")
	}
}

// audits is an audit at one of your fronts yesterday, a tell too: the
// books were looked at. Only a front that was being run greedy gives the
// DA something to file (#27: a case is built from what you did, not what
// you have). Laundering steps after heat, so the front's record is how
// yesterday's audit reaches today's heat. The pages go in without a line
// of their own: the audit's heat line says the file grew.
func (s *Sim) audits(d *day) {
	w, t, tun, here := d.w, d.t, d.tun, d.here
	for _, f := range w.Fronts {
		if f.Audited == 0 || f.Audited != t.Day-1 {
			continue
		}
		if pages := max(0, tun.AuditEvidence-d.fx.AuditEvidenceCut); f.AuditDial == events.LaunderGreedy && pages > 0 {
			d.file(here, pages, "")
			d.add(here, tun.AuditHeat, fmt.Sprintf("audit at %s, run greedy: the DA's file grows", f.Name))
		} else {
			d.add(here, tun.AuditHeat, fmt.Sprintf("audit at %s", f.Name))
		}
	}
}

// structuring is clean cash moved offshore yesterday over the lot
// (#195): the DA reads the transfers, a page a lot over the line,
// wherever you are (structuring is something you did, #27; a move under
// the lot, or none, files nothing). Laundering steps after heat, so its
// record is how last night's move reaches today's file.
func (s *Sim) structuring(d *day) {
	tun := d.tun
	if st := d.w.Laundering.Structured; st.Day != 0 && st.Day == d.t.Day-1 && st.Lots > 0 && tun.StructureEvidence > 0 {
		d.file(d.here, st.Lots*tun.StructureEvidence, "money moved offshore in lumps: the DA's file grows")
	}
}

// cold is a retained lawyer letting the file go cold: a page drops off
// after enough days without a new one. Only dealing adds pages (#27), so
// lying low is how a case is left to die.
func (s *Sim) cold(d *day) {
	h, t, fx := d.h, d.t, d.fx
	if d.w.Over == nil && fx.EvidenceDecayDays > 0 && h.Evidence > 0 && t.Day-h.EvidenceDay >= fx.EvidenceDecayDays {
		h.Evidence--
		h.EvidenceDay = t.Day
		d.reasons[d.here] = append(d.reasons[d.here], fmt.Sprintf("the case goes cold (file %d)", h.Evidence))
	}
}
