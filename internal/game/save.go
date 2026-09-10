package game

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	// ErrNoSave means there is nothing to continue.
	ErrNoSave = errors.New("no saved run")
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

// SavePath is the single save slot.
func SavePath() (string, error) {
	dir, err := SaveDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "save.gob"), nil
}

// HasSave reports whether a save slot exists.
func HasSave() bool {
	p, err := SavePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Save writes w to the save slot atomically (temp file then rename).
func Save(w *World) error {
	p, err := SavePath()
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

// Load reads the save slot, upgrading an older save one schema version at a
// time with the given migrations. A save newer than this build, or older
// with no migration path, is refused.
func Load(migrations ...Migration) (*World, error) {
	p, err := SavePath()
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

// DeleteSave removes the save slot; missing is not an error.
func DeleteSave() error {
	p, err := SavePath()
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
