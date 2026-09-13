package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/theclifmeister/kingpin/internal/content"
)

// The profile (#50, docs/profile.md): the game around the runs. A
// second file under SaveDir, profile.json, its own schema, written by
// the UI when a run ends and never read by a sim or the harness: what
// it remembers (the runs, the unlocks, the dailies) only changes what a
// run starts as (Start), so a seed with the same start plays the same
// under any profile (TestProfileNeverTouchesTheRun).

// ProfileSchema is bumped whenever Profile changes shape incompatibly.
const ProfileSchema = 1

// ProfileFile is the profile's name under SaveDir.
const ProfileFile = "profile.json"

var (
	// ErrCorruptProfile means the file did not read: it has been set
	// aside as ProfileFile.corrupt-<date> and a fresh one starts.
	ErrCorruptProfile = errors.New("profile is corrupt")
	// ErrNewerProfile means a newer build wrote the profile; it is left
	// as it is and this run is not recorded.
	ErrNewerProfile = errors.New("profile is from a newer version of kingpin")
)

// Start is what a run began as (#50): the character (characters.toml;
// "" is the default, the run as it is), whether the hard DA was on, the
// daily's date where the run is one (DailyKey) and whether it was a
// practice attempt (a second on the date). Stamped on the world at
// NewWorld, read by the summary and the profile, never by a sim; the
// zero value is every run before the feature.
type Start struct {
	Character string
	HardDA    bool
	Daily     string
	Practice  bool
}

// Profile is what the game remembers between runs.
type Profile struct {
	SchemaVersion int
	Runs          []RunRecord      // every run that ended, oldest first
	Unlocks       map[string]bool  // character and toggle ids opened
	Best          map[string]Score // the daily's score by date: the first attempt's, never replaced
	Attempts      map[string]int   // dailies started by date; the second and later are practice
}

// RunRecord is one run that ended.
type RunRecord struct {
	Seed      uint64
	Character string // the character id, "" the default
	HardDA    bool
	Ending    string // the cause
	Score     int
	Days      int
	PeakCash  int
	Stage     string // the highest stage reached, by id
	Date      string // the day it ended, DailyKey's form
	Daily     string // the daily's date where the run was one
	Practice  bool   // a daily attempt after the first: recorded, never scored
}

// Score is a daily's score: what it was and the run it came from.
type Score struct {
	Score     int
	Seed      uint64
	Character string
	Ending    string
	Days      int
}

// NewProfile is an empty profile at the current schema.
func NewProfile() *Profile {
	return &Profile{SchemaVersion: ProfileSchema, Unlocks: map[string]bool{}, Best: map[string]Score{}, Attempts: map[string]int{}}
}

// ProfilePath is the profile's file under SaveDir.
func ProfilePath() (string, error) {
	dir, err := SaveDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ProfileFile), nil
}

// LoadProfile reads the profile: a missing one is empty, a corrupt one
// is set aside as profile.json.corrupt-<date> (kept, never written
// over; -2, -3 where the day has one already) and returned empty with
// ErrCorruptProfile, so the caller writes a fresh one beside it; one
// from a newer build is returned empty with ErrNewerProfile and left
// alone. now is the caller's date: time.Now is read in the UI alone.
func LoadProfile(now time.Time) (*Profile, error) {
	p, err := ProfilePath()
	if err != nil {
		return NewProfile(), err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return NewProfile(), nil
	}
	if err != nil {
		return NewProfile(), err
	}
	var prof Profile
	if err := json.Unmarshal(b, &prof); err != nil || prof.SchemaVersion < 1 {
		aside := setAside(p, now)
		if err == nil {
			err = fmt.Errorf("schema %d", prof.SchemaVersion)
		}
		return NewProfile(), fmt.Errorf("%w (%v): kept as %s", ErrCorruptProfile, err, filepath.Base(aside))
	}
	if prof.SchemaVersion > ProfileSchema {
		return NewProfile(), fmt.Errorf("%w (schema %d, this build reads %d)", ErrNewerProfile, prof.SchemaVersion, ProfileSchema)
	}
	fill(&prof)
	return &prof, nil
}

// setAside renames a corrupt profile to profile.json.corrupt-<date>,
// the first name free that day, and returns it.
func setAside(p string, now time.Time) string {
	base := p + ".corrupt-" + DailyKey(now)
	aside := base
	for i := 2; ; i++ {
		if _, err := os.Stat(aside); err != nil {
			break
		}
		aside = base + "-" + strconv.Itoa(i)
	}
	_ = os.Rename(p, aside)
	return aside
}

// fill gives a decoded profile its maps.
func fill(p *Profile) {
	if p.Unlocks == nil {
		p.Unlocks = map[string]bool{}
	}
	if p.Best == nil {
		p.Best = map[string]Score{}
	}
	if p.Attempts == nil {
		p.Attempts = map[string]int{}
	}
}

// SaveProfile writes the profile atomically (temp file then rename), as
// Save writes a world.
func SaveProfile(prof *Profile) error {
	p, err := ProfilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	prof.SchemaVersion = ProfileSchema
	b, err := json.MarshalIndent(prof, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profile: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Record files a run that ended: the record, the unlocks its ending
// and stage earn (Unlocks) and, for a daily's first attempt, its score
// in Best. A record already filed (the same seed, start, cause, day
// and score: an ended save opened again) is not filed twice. It returns the ids
// newly unlocked, in the file's order.
func (p *Profile) Record(chars content.CharactersConfig, prog content.ProgressionConfig, rec RunRecord) []string {
	fill(p)
	for _, r := range p.Runs {
		if r.Seed == rec.Seed && r.Character == rec.Character && r.Ending == rec.Ending && r.Days == rec.Days && r.Daily == rec.Daily && r.Practice == rec.Practice && r.Score == rec.Score {
			return nil
		}
	}
	p.Runs = append(p.Runs, rec)
	if rec.Daily != "" && !rec.Practice {
		if _, done := p.Best[rec.Daily]; !done {
			p.Best[rec.Daily] = Score{Score: rec.Score, Seed: rec.Seed, Character: rec.Character, Ending: rec.Ending, Days: rec.Days}
		}
	}
	var fresh []string
	for _, id := range Unlocks(chars, prog, rec.Ending, rec.Stage) {
		if !p.Unlocks[id] {
			p.Unlocks[id] = true
			fresh = append(fresh, id)
		}
	}
	return fresh
}

// Unlocked reports whether a character or toggle is open: its rule is
// always, or the profile has earned it.
func (p *Profile) Unlocked(rule content.UnlockRule, id string) bool {
	if rule.Free() {
		return true
	}
	return p != nil && p.Unlocks[id]
}

// Unlocks is the ids a run's ending and stage open (#50): every
// character and the toggle whose rule the run met. cause is the
// Ending's, stage the id of the highest stage the run reached.
func Unlocks(chars content.CharactersConfig, prog content.ProgressionConfig, cause, stage string) []string {
	var out []string
	for _, ch := range chars.Characters {
		if Met(ch.Unlock, prog, cause, stage) {
			out = append(out, ch.ID)
		}
	}
	if Met(chars.HardDA.Unlock, prog, cause, stage) {
		out = append(out, HardDAID)
	}
	return out
}

// HardDAID is the toggle's id in the profile's unlocks.
const HardDAID = "hard_da"

// Met reports whether a run's ending and stage meet a rule: always
// never does (it needs no unlock), an ending rule on its cause, a stage
// rule on that stage or one past it in the ladder.
func Met(rule content.UnlockRule, prog content.ProgressionConfig, cause, stage string) bool {
	switch {
	case rule.Ending != "":
		return cause == rule.Ending
	case rule.Stage != "":
		want, got := prog.Index(rule.Stage), prog.Index(stage)
		return want >= 0 && got >= want
	}
	return false
}

// Stage is the id of the highest stage the world has reached, for the
// record and the rules.
func (w *World) Stage(prog content.ProgressionConfig) string {
	if t := prog.Tier(w.Tier()); t != nil {
		return t.ID
	}
	return ""
}

// StartAttempt counts a daily started on the date and reports whether
// it is practice: every attempt after the first.
func (p *Profile) StartAttempt(date string) (practice bool) {
	fill(p)
	p.Attempts[date]++
	return p.Attempts[date] > 1
}

// Rank is where a score stands among the runs recorded: 1 for the best,
// and how many runs there are, the practice dailies included. A score
// equal to another's shares its rank.
func (p *Profile) Rank(score int) (rank, of int) {
	if p == nil {
		return 1, 0
	}
	rank = 1
	for _, r := range p.Runs {
		if r.Score > score {
			rank++
		}
	}
	return rank, len(p.Runs)
}

// BestRun is the highest-scoring run recorded, or nil with none.
func (p *Profile) BestRun() *RunRecord {
	if p == nil {
		return nil
	}
	var best *RunRecord
	for i := range p.Runs {
		if best == nil || p.Runs[i].Score > best.Score {
			best = &p.Runs[i]
		}
	}
	return best
}

// Endings is how many distinct causes the runs have reached.
func (p *Profile) Endings() int {
	if p == nil {
		return 0
	}
	seen := map[string]bool{}
	for _, r := range p.Runs {
		seen[r.Ending] = true
	}
	return len(seen)
}

// Dates is every daily date scored, oldest first.
func (p *Profile) Dates() []string {
	if p == nil {
		return nil
	}
	out := make([]string, 0, len(p.Best))
	for d := range p.Best {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// DailyKey is a date as the daily names it: the UTC date as YYYYMMDD.
func DailyKey(date time.Time) string { return date.UTC().Format("20060102") }

// DailySeed is the daily's seed (#50): the UTC date as YYYYMMDD, the
// number itself, folded through one step of SplitMix64 so the seeds of
// two days share nothing. The same date is the same seed in every
// process and on every machine; the UI passes the date in.
func DailySeed(date time.Time) uint64 {
	n, _ := strconv.ParseUint(DailyKey(date), 10, 64)
	z := n + 0x9E3779B97F4A7C15
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
