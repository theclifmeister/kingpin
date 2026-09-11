package game

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SlotCount is how many runs live on disk: save1.gob to save3.gob under
// SaveDir. A slot is a whole world; the start menu picks one.
const SlotCount = 3

var (
	// ErrNoSave means there is nothing to continue.
	ErrNoSave = errors.New("no saved run")
	// ErrBadSlot is a slot number outside 1..SlotCount.
	ErrBadSlot = errors.New("no such save slot")
	// ErrNewerSchema means the save was written by a newer build.
	ErrNewerSchema = errors.New("save file is from a newer version of kingpin")
	// ErrOldSchema means the save predates a world change that has no
	// migration path. Start a new run.
	ErrOldSchema = errors.New("save file is from an older version of kingpin and cannot be upgraded")
)

// Migration upgrades a world from schema From to From+1. The package that
// owns the new state supplies it, so game never needs content to load.
type Migration struct {
	From  int
	Apply func(w *World)
}

// SaveDir returns the directory saves live in. KINGPIN_HOME overrides the
// platform config directory.
func SaveDir() (string, error) {
	if h := os.Getenv("KINGPIN_HOME"); h != "" {
		return h, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "kingpin"), nil
}

// SavePath is the file a slot lives in, save<N>.gob under SaveDir. The
// single save.gob of builds before the slots is slot 1: the first time
// slot 1 is looked at it is renamed, so an old run carries on where it
// was.
func SavePath(slot int) (string, error) {
	if slot < 1 || slot > SlotCount {
		return "", fmt.Errorf("%w: %d", ErrBadSlot, slot)
	}
	dir, err := SaveDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, fmt.Sprintf("save%d.gob", slot))
	if slot == 1 {
		adopt(filepath.Join(dir, "save.gob"), p)
	}
	return p, nil
}

// adopt renames the pre-slot save.gob to slot 1's file, once, and only
// where slot 1 is empty: a run saved into slot 1 since is the newer one.
func adopt(old, p string) {
	if _, err := os.Stat(old); err != nil {
		return
	}
	if _, err := os.Stat(p); err == nil {
		return
	}
	_ = os.Rename(old, p)
}

// HasSave reports whether a slot holds a run.
func HasSave(slot int) bool {
	p, err := SavePath(slot)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// SlotInfo is what the start menu says about a slot without loading it:
// the day, the cash in hand, the city the player stands in and when the
// file was written. Empty is a slot with no run in it; a file that does
// not read leaves the rest zero and Load says why.
type SlotInfo struct {
	Slot  int
	Day   int
	Cash  int
	City  string
	Saved time.Time
	Empty bool
}

// Slots describes every slot in order, 1 to SlotCount.
func Slots() []SlotInfo {
	infos := make([]SlotInfo, 0, SlotCount)
	for slot := 1; slot <= SlotCount; slot++ {
		infos = append(infos, slotInfo(slot))
	}
	return infos
}

func slotInfo(slot int) SlotInfo {
	info := SlotInfo{Slot: slot, Empty: true}
	p, err := SavePath(slot)
	if err != nil {
		return info
	}
	st, err := os.Stat(p)
	if err != nil {
		return info
	}
	info.Empty = false
	info.Saved = st.ModTime()
	b, err := os.ReadFile(p)
	if err != nil {
		return info
	}
	var w World
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&w); err != nil {
		return info
	}
	info.Day = w.Day
	info.Cash = w.Cash()
	if c := w.City(w.Player.Location); c != nil {
		info.City = c.Name
	}
	return info
}

// Save writes w to the slot atomically (temp file then rename).
func Save(slot int, w *World) error {
	p, err := SavePath(slot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(w); err != nil {
		return fmt.Errorf("encode save: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Load reads the slot, upgrading an older save one schema version at a
// time with the given migrations. A save newer than this build, or older
// with no migration path, is refused.
func Load(slot int, migrations ...Migration) (*World, error) {
	p, err := SavePath(slot)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoSave
	}
	if err != nil {
		return nil, err
	}
	var w World
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&w); err != nil {
		return nil, fmt.Errorf("save file is corrupt: %w", err)
	}
	if w.SchemaVersion > SchemaVersion {
		return nil, ErrNewerSchema
	}
	if w.SchemaVersion < 7 {
		// The one city there was lived on World itself. Read those fields
		// off the stream a second time (gob matches by name and ignores
		// the rest) for MigrateCities to wrap.
		var old v6
		if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&old); err != nil {
			return nil, fmt.Errorf("save file is corrupt: %w", err)
		}
		w.legacy = &old
	}
	if w.SchemaVersion < 10 {
		// The fall guy was a flag (FallGuyUsed) before fall_guys became a
		// count (#117); gob will not read a bool into an int, so the old
		// field is read off the stream a second time for MigrateFallGuys.
		var old v9
		if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&old); err != nil {
			return nil, fmt.Errorf("save file is corrupt: %w", err)
		}
		w.fell = old.FallGuyUsed
	}
	if len(w.Cities) == 0 && (w.legacy == nil || w.legacy.Market == nil) {
		return nil, fmt.Errorf("save file is corrupt: missing world state")
	}
	for w.SchemaVersion < SchemaVersion {
		var step *Migration
		for i := range migrations {
			if migrations[i].From == w.SchemaVersion {
				step = &migrations[i]
			}
		}
		if step == nil {
			return nil, fmt.Errorf("%w (schema %d, this build reads %d)", ErrOldSchema, w.SchemaVersion, SchemaVersion)
		}
		step.Apply(&w)
		w.SchemaVersion++
	}
	if w.Orders == nil {
		w.Orders = map[string]SellOrder{}
	}
	if w.Heat.LastResponse == nil {
		w.Heat.LastResponse = map[string]int{}
	}
	if w.Heat.Responses == nil {
		w.Heat.Responses = map[string]int{}
	}
	if w.Upgrades == nil {
		w.Upgrades = map[string]bool{}
	}
	if w.Player.Stash == nil {
		w.Player.Stash = map[string]map[string]int{}
	}
	w.legacy = nil
	return &w, nil
}

// v6 is what a pre-7 save carried for the one city there was, in the
// shape it had then: the market, the corners, the player's stock and the
// heat on World, Player and HeatState themselves.
type v6 struct {
	Market    map[string]*ProductMarket
	Territory struct{ Corners []Corner }
	Player    struct{ Stock map[string]int }
	Heat      struct{ Value float64 }
}

// v9 is what a pre-10 save carried for the fall guy: one flag on World.
// The schema is read beside it because gob leaves a false flag out of
// the stream and refuses a record none of whose fields it finds.
type v9 struct {
	SchemaVersion int
	FallGuyUsed   bool
}

// MigrateFallGuys is the 9 -> 10 step: the fall guy who had taken his
// fall is counted as one, so a save owning fallguy reads as it did.
func MigrateFallGuys(w *World) {
	if w.fell {
		w.FallsTaken = max(w.FallsTaken, 1)
	}
	w.fell = false
}

// MigrateCities is the 6 -> 7 step: the single city becomes the home
// entry of Cities, keeping its market, corners, stock and heat, and the
// player is standing in it. A save that already has cities (or nothing
// to wrap) is left alone; the caller lays out whatever cities are
// missing from the config, as a migration seeds any new state.
func (w *World) MigrateCities(home StartingCity) {
	w.Player.Location = home.ID
	if w.Cities[home.ID] != nil {
		return
	}
	old := w.legacy
	if old == nil || old.Market == nil {
		w.AddCity(home)
		return
	}
	c := w.AddCity(StartingCity{ID: home.ID, Name: home.Name, HeatMul: home.HeatMul, Wholesale: home.Wholesale})
	c.Market = old.Market
	c.Corners = old.Territory.Corners
	for i := range c.Corners {
		c.Corners[i].City = home.ID
	}
	c.Heat = old.Heat.Value
	stash := w.Stash(home.ID)
	for id, q := range old.Player.Stock {
		stash[id] = q
	}
	w.legacy = nil
}

// DeleteSave empties the slot; an empty one is not an error.
func DeleteSave(slot int) error {
	p, err := SavePath(slot)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
