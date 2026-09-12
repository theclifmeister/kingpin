package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The strike's scene (#158), the one scene on a play screen, so the
// tightest rule: the morning after a strike you won (a CornerStruck
// that was Taken) or a corner taken off you (a CornerTaken From you),
// the first time that corner's map is shown, its cell burns from the
// colour it was to the colour it is (anim.Strike: purple to blue when
// won, blue to purple when taken) and its name slides into the pane's
// inspector, anim.StrikeLength in all; the rest of MAIN, the pane and
// the frame draw as they are, and the map is play mode again the
// moment it ends. Any key ends it and is consumed (skip), as every
// interstitial's is. The corners are read off the tick's events in
// morning (Model.mapScene, UI state: never saved, cleared when their
// map is shown, replaced the next morning, dropped by a new or a
// continued run), so a fast-forward plays the stopping morning's and
// the journal has the rest; a corner in the other city plays when
// that city's map is shown. The scene reads the corner's owner off
// the world when it starts, so the colour it settles on is the map's
// own. A View returns no command, so the scene starts in Update, on
// the key after which the map is what View draws (mapSceneStart), and
// viewMap draws it (mapBurning, mapHead). With animation off, or under
// 80x24, nothing starts and the map is today's, byte for byte.

// mapFlip is a corner that changed hands overnight: its id and whose it
// was, so the cell burns from that colour.
type mapFlip struct {
	corner string
	from   string
}

// mapFlips reads the tick's events for the corners that changed hands
// with you on one side: a strike of yours that took the corner, and a
// corner the rival took off you.
func mapFlips(evs []events.Event) []mapFlip {
	var flips []mapFlip
	for _, e := range evs {
		switch ev := e.(type) {
		case events.CornerStruck:
			if ev.Taken {
				flips = append(flips, mapFlip{ev.Corner, game.OwnerRival})
			}
		case events.CornerTaken:
			if ev.From == game.OwnerPlayer {
				flips = append(flips, mapFlip{ev.Corner, game.OwnerPlayer})
			}
		}
	}
	return flips
}

// ownerColour is the colour a corner's owner draws it in: ownerStyle's.
func ownerColour(owner string) lipgloss.Color {
	switch owner {
	case game.OwnerPlayer:
		return theme.Crew
	case game.OwnerRival:
		return theme.Rivals
	default:
		return theme.Dim
	}
}

// mapSceneStart starts the map's scene when the map is what View draws
// next and a corner of the shown city is waiting: animation on, play
// mode, the map, no scene up, the terminal 80x24 or more. The shown
// city's corners come off mapScene (the other city's stay for its
// map); the cursor goes to the first, so the inspector is its and its
// name is what slides in; the dice are anim.Seed(seed, day, "strike").
func (m *Model) mapSceneStart() {
	if !m.opts.Anim || m.scene != nil || m.mode != modePlay || m.screen != screenMap || !m.titleFits() || len(m.mapScene) == 0 {
		return
	}
	city := m.shown()
	var flips []anim.Flip
	var ids, keep []mapFlip
	cellW := m.mapCellW()
	for _, f := range m.mapScene {
		c := m.w.Corner(f.corner)
		if c == nil || c.City != city.ID {
			keep = append(keep, f)
			continue
		}
		flips = append(flips, anim.Flip{Cell: m.cellName(c, cellW), From: ownerColour(f.from), To: ownerColour(c.Owner)})
		ids = append(ids, f)
	}
	m.mapScene = keep
	if len(flips) == 0 {
		return
	}
	m.mapPlaying = nil
	for _, f := range ids {
		m.mapPlaying = append(m.mapPlaying, f.corner)
	}
	for i := range city.Corners {
		if city.Corners[i].ID == ids[0].corner {
			m.mapCursor, m.onRoutes = i, false
		}
	}
	name := strings.ToUpper(m.w.Corner(ids[0].corner).Name)
	m.play(&anim.Player{
		Scene:  anim.Strike(flips, name, theme.Rivals, anim.Seed(m.w.Seed, m.w.Day, "strike")),
		Accent: theme.Rivals,
	})
}

// mapOnScene reports whether the map draws the scene: one is up on the
// map in play mode and the terminal is 80x24 or more (a scene started
// and then resized under the floor runs out unseen).
func (m *Model) mapOnScene() bool {
	return m.scene != nil && !m.scene.Idle && m.mode == modePlay && m.screen == screenMap && len(m.mapPlaying) > 0 && m.titleFits()
}

// mapBurning is the scene's row for each corner it is over, by corner
// id, at the cell's width; nothing while no scene runs.
func (m *Model) mapBurning() map[string]string {
	if !m.mapOnScene() {
		return nil
	}
	frame := m.scene.Frame(m.mapCellW()-1, len(m.mapPlaying))
	rows := map[string]string{}
	for i, id := range m.mapPlaying {
		rows[id] = frame[i]
	}
	return rows
}

// mapHead is the inspector's title row mid-scene, the name sliding in
// at the pane's text width, for the corner the scene is over first.
func (m *Model) mapHead(sel *game.Corner) (string, bool) {
	if !m.mapOnScene() || sel.ID != m.mapPlaying[0] {
		return "", false
	}
	frame := m.scene.Frame(paneTextW, len(m.mapPlaying)+1)
	return frame[len(m.mapPlaying)], true
}
