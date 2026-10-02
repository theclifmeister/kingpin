package engine

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// TestEveryKindIsClassed (#541): every alert kind and every event kind
// has a class under the stop rule, once, and docs/engine.md's tables
// say each as the code has it. A kind added without a class fails here
// by name, the way events.All works, so what F stops for is decided on
// purpose and checked against the rule, not against the last playtest.
func TestEveryKindIsClassed(t *testing.T) {
	t.Parallel()
	all := map[string]bool{}
	for _, e := range events.All {
		all[e.Kind()] = true
		if _, ok := eventClasses[e.Kind()]; !ok {
			t.Errorf("event %s has no class under the stop rule (stoprule.go eventLists)", e.Kind())
		}
	}
	listed := 0
	for c, evs := range eventLists {
		listed += len(evs)
		for _, e := range evs {
			if !all[e.Kind()] {
				t.Errorf("event %s is classed %s and is not in events.All", e.Kind(), c)
			}
		}
	}
	if listed != len(eventClasses) {
		t.Errorf("%d events listed for %d kinds: a kind is classed twice", listed, len(eventClasses))
	}
	if len(alertClasses) != len(AlertKinds()) {
		t.Errorf("%d alert kinds classed of %d", len(alertClasses), len(AlertKinds()))
	}
	for _, k := range AlertKinds() {
		if _, ok := alertClasses[k]; !ok {
			t.Errorf("alert %s has no class under the stop rule (stoprule.go alertClasses)", k)
		}
	}

	b, err := os.ReadFile("../../docs/engine.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	from := strings.Index(doc, "**The stop rule (#541).**")
	to := strings.Index(doc[max(from, 0):], "**What pins it.**")
	if from < 0 || to < 0 {
		t.Fatal("docs/engine.md has no stop rule section before What pins it")
	}
	to += from
	rows := map[string]Class{}
	for _, m := range regexp.MustCompile("(?m)^\\| `(\\w+)` \\| (\\w+) \\|").FindAllStringSubmatch(doc[from:to], -1) {
		if _, ok := rows[m[1]]; ok {
			t.Errorf("docs/engine.md classes %s twice", m[1])
		}
		rows[m[1]] = Class(m[2])
	}
	for _, k := range AlertKinds() {
		if got, want := rows[string(k)], alertClasses[k]; got != want {
			t.Errorf("docs/engine.md classes alert %s %q, the code %q", k, got, want)
		}
		delete(rows, string(k))
	}
	for kind, want := range eventClasses {
		got, ok := rows[kind]
		if want == ClassNotice {
			if ok {
				t.Errorf("docs/engine.md lists %s, a notice, among the stops", kind)
			}
			continue
		}
		if got != want {
			t.Errorf("docs/engine.md classes event %s %q, the code %q", kind, got, want)
		}
		delete(rows, kind)
	}
	for kind := range rows {
		t.Errorf("docs/engine.md classes %s, which is no stopping kind", kind)
	}
}

// TestTheRuleChanges (#541): what applying the rule moved. A deed the
// DA took stops like an asset seized, a member a faction poached like a
// defection (one who stayed does not), and a landed load, the port, the
// lanes abroad and the plan are notices; the dangers and the endings
// are classed as the rule words them.
func TestTheRuleChanges(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		e    events.Event
		want Class
	}{
		{events.DeedSeized{}, ClassTurn},
		{events.CrewPoached{}, ClassTurn},
		{events.CrewPoached{Stayed: true}, ClassNotice},
		{events.WarrantSigned{}, ClassDanger},
		{events.QuietBroken{}, ClassEnding},
		{events.ReignBegan{Again: true}, ClassNotice},
		{events.DealOffered{}, ClassAnswer},
		{events.PressureShifted{}, ClassNotice},
		{events.Unlocked{}, ClassNotice},
	} {
		if got := EventClass(c.e); got != c.want {
			t.Errorf("%s %+v: class %q, want %q", c.e.Kind(), c.e, got, c.want)
		}
		if StopsOn(c.e) != (c.want != ClassNotice) {
			t.Errorf("%s %+v: StopsOn %v against its class %q", c.e.Kind(), c.e, StopsOn(c.e), c.want)
		}
	}
	for _, k := range []AlertKind{AlertLanded, AlertPort, AlertExports, AlertPlan} {
		if a := (Alert{Kind: k}); !a.Notice() || a.Danger() {
			t.Errorf("%s: notice %v danger %v, want a notice", k, a.Notice(), a.Danger())
		}
	}
	for _, a := range []Alert{{Kind: AlertRetire}, {Kind: AlertReign}, {Kind: AlertStraight}, {Kind: AlertVanish}} {
		if a.Class() != ClassEnding || a.Notice() || a.Danger() {
			t.Errorf("%s: class %q, want an ending", a.Kind, a.Class())
		}
	}
}
