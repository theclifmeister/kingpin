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
	if w.Market == nil || w.Player.Stock == nil {
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
	return &w, nil
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
