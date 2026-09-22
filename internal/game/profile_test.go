package game

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/theclifmeister/kingpin/internal/content"
)

var day = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

// TestProfileRoundTrip (#50): a missing profile is an empty one, a
// saved one reads back as written, and the file is the world's
// neighbour under KINGPIN_HOME.
func TestProfileRoundTrip(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	p, err := LoadProfile(day)
	if err != nil || len(p.Runs) != 0 || len(p.Unlocks) != 0 || len(p.Best) != 0 || p.SchemaVersion != ProfileSchema {
		t.Fatalf("a missing profile read as %+v, %v", p, err)
	}
	cfg := content.MustLoad()
	rec := RunRecord{Seed: 42, Character: "cook", Ending: content.CauseRetired, Score: 765_314, Days: 120, PeakCash: 2_000_000, Stage: "territory", Date: "20260913"}
	p.Record(cfg.Characters, cfg.Progression, rec)
	p.Record(cfg.Characters, cfg.Progression, RunRecord{Seed: 7, Ending: content.CauseVanished, Score: 10, Days: 30, Stage: "crew", Date: "20260913", Daily: "20260913"})
	if err := SaveProfile(p); err != nil {
		t.Fatal(err)
	}
	again, err := LoadProfile(day)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, again) {
		t.Fatalf("round trip:\n%+v\n%+v", p, again)
	}
	if again.Runs[0] != rec || !again.Unlocks["excop"] || again.Best["20260913"].Score != 10 {
		t.Fatalf("the record did not survive: %+v", again)
	}
	path, _ := ProfilePath()
	if filepath.Base(path) != ProfileFile || filepath.Dir(path) != os.Getenv("KINGPIN_HOME") {
		t.Fatalf("the profile lives at %s", path)
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Fatal("the temp file was left behind")
	}
}

// TestCorruptProfileIsKept (#50): a profile that does not read is
// refused with a message, set aside as profile.json.corrupt-<date>
// (a second the same day as -2) and never written over; the fresh one
// saves beside it. One from a newer build is left where it is.
func TestCorruptProfileIsKept(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	path, _ := ProfilePath()
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfile(day)
	if !errors.Is(err, ErrCorruptProfile) || p == nil || len(p.Runs) != 0 {
		t.Fatalf("a corrupt profile read as %+v, %v", p, err)
	}
	aside := path + ".corrupt-20260913"
	if b, err := os.ReadFile(aside); err != nil || string(b) != "{not json" {
		t.Fatalf("the corrupt file was not kept at %s: %q %v", aside, b, err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("the corrupt file is still in place")
	}
	if err := SaveProfile(p); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(aside); string(b) != "{not json" {
		t.Fatal("the fresh profile wrote over the corrupt one")
	}
	if err := os.WriteFile(path, []byte(`{"SchemaVersion": 0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProfile(day); !errors.Is(err, ErrCorruptProfile) {
		t.Fatalf("a profile with no schema read: %v", err)
	}
	if _, err := os.Stat(aside + "-2"); err != nil {
		t.Fatalf("the second corrupt file of the day was not kept: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"SchemaVersion": 99}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, err := LoadProfile(day); !errors.Is(err, ErrNewerProfile) || len(p.Runs) != 0 {
		t.Fatalf("a newer profile read as %+v, %v", p, err)
	}
	if b, _ := os.ReadFile(path); string(b) != `{"SchemaVersion": 99}` {
		t.Fatal("the newer profile was moved")
	}
}

// TestEveryUnlockRuleFromAnEnding (#50): every character's rule and
// the toggle's, tabled from an Ending's cause and the stage reached.
func TestEveryUnlockRuleFromAnEnding(t *testing.T) {
	cfg := content.MustLoad()
	chars, prog := cfg.Characters, cfg.Progression
	cases := []struct {
		cause, stage string
		want         []string
	}{
		{content.CauseIndicted, "corner", nil},
		{content.CauseArrested, "crew", nil},
		{content.CauseBroke, "territory", nil},
		{content.CauseRetired, "territory", nil},
		{content.CauseVanished, "corner", []string{"excop"}},
		{content.CauseBusinessman, "territory", []string{"bookkeeper"}},
		{content.CauseKingpin, "territory", []string{"heir", HardDAID}}, // the Heir (#232) and the toggle
		{content.CauseIndicted, "distribution", []string{"dockhand"}},
		{content.CauseTakenOut, "cartel", []string{"dockhand"}},
		{content.CauseKingpin, "cartel", []string{"dockhand", "heir", HardDAID}},
		{content.CauseVanished, "distribution", []string{"excop", "dockhand"}},
		{"struck_by_lightning", "", nil},
	}
	for _, c := range cases {
		if got := Unlocks(chars, prog, c.cause, c.stage); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s at %s: unlocks %v, want %v", c.cause, c.stage, got, c.want)
		}
	}
	for _, ch := range chars.Characters {
		if ch.Unlock.Free() && Met(ch.Unlock, prog, content.CauseKingpin, "cartel") {
			t.Errorf("%s: an always rule is never met, it needs no unlock", ch.ID)
		}
	}
	// The profile applies them once, reports the fresh ones, and the
	// default and the always rows are open with nothing earned.
	p := NewProfile()
	for _, ch := range chars.Characters {
		if p.Unlocked(ch.Unlock, ch.ID) != ch.Unlock.Free() {
			t.Errorf("%s: open %v on an empty profile", ch.ID, p.Unlocked(ch.Unlock, ch.ID))
		}
	}
	rec := RunRecord{Seed: 1, Ending: content.CauseKingpin, Days: 150, Stage: "cartel", Date: "20260913"}
	if fresh := p.Record(chars, prog, rec); !reflect.DeepEqual(fresh, []string{"dockhand", "heir", HardDAID}) {
		t.Fatalf("fresh %v", fresh)
	}
	if fresh := p.Record(chars, prog, rec); fresh != nil || len(p.Runs) != 1 {
		t.Fatalf("filed twice: %v, %d runs", fresh, len(p.Runs))
	}
	if !p.Unlocked(chars.HardDA.Unlock, HardDAID) || !p.Unlocked(chars.Character("dockhand").Unlock, "dockhand") || p.Unlocked(chars.Character("excop").Unlock, "excop") {
		t.Fatalf("unlocks %v", p.Unlocks)
	}
}

// TestDailyIsScoredOnce (#50): the first attempt on a date is the one
// in Best, a later one is practice and never replaces it, and the
// summary's rank counts every run.
func TestDailyIsScoredOnce(t *testing.T) {
	cfg := content.MustLoad()
	p := NewProfile()
	if p.StartAttempt("20260913") {
		t.Fatal("the first attempt is practice")
	}
	second, third, other := p.StartAttempt("20260913"), p.StartAttempt("20260913"), p.StartAttempt("20260914")
	if !second || !third || other {
		t.Fatalf("the attempts do not count: %v %v %v", second, third, other)
	}
	p.Record(cfg.Characters, cfg.Progression, RunRecord{Seed: 1, Ending: content.CauseArrested, Score: 100, Days: 20, Stage: "corner", Date: "20260913", Daily: "20260913"})
	p.Record(cfg.Characters, cfg.Progression, RunRecord{Seed: 1, Ending: content.CauseRetired, Score: 900, Days: 90, Stage: "crew", Date: "20260913", Daily: "20260913", Practice: true})
	if best := p.Best["20260913"]; best.Score != 100 || best.Ending != content.CauseArrested {
		t.Fatalf("the practice run took Best: %+v", best)
	}
	if len(p.Runs) != 2 || !p.Runs[1].Practice {
		t.Fatalf("runs %+v", p.Runs)
	}
	if rank, of := p.Rank(500); rank != 2 || of != 2 {
		t.Fatalf("rank %d of %d", rank, of)
	}
	if rank, _ := p.Rank(900); rank != 1 {
		t.Fatalf("the best ranks %d", rank)
	}
	if p.BestRun().Score != 900 || p.Endings() != 2 || !reflect.DeepEqual(p.Dates(), []string{"20260913"}) {
		t.Fatalf("best %+v endings %d dates %v", p.BestRun(), p.Endings(), p.Dates())
	}
}
