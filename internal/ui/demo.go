package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// cmd/anim's window on the model (#161): a run to review every
// registered scene on, and one way to put a scene up in the mode that
// plays it in the game. The two exports below are the whole of it,
// and they are the review tool's, never the game's: in the game every
// scene starts inside Update, on the morning or the key that shows it
// (docs/animation.md), and nothing here changes that. The demo's world
// is a run of its own, saved under KINGPIN_HOME as any run is, which
// the command points at a directory of its own so no real slot is
// touched.

// DemoModel is the model cmd/anim reviews the scenes on: a fresh run on
// the seed at the size, animation on with the morning's, a few days in
// so the report has lines, the rival on its first corner (the one the
// strike's scene wins) and a crew on the payroll for the ending's
// figures. The run is saved to slot 1
// under KINGPIN_HOME, as a run is.
func DemoModel(cfg *content.Config, seed uint64, w, h int) (*Model, error) {
	m, err := wire(cfg, Options{Anim: true, MorningAnim: true})
	if err != nil {
		return nil, err
	}
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.slot = 1
	m.startRun(seed)
	world := m.w
	for i := 0; i < 3; i++ {
		world.SetStock(world.Player.Location, world.Products[0], 40)
		for _, k := range []tea.KeyMsg{demoKey("s"), demoKey("enter"), demoKey("enter"), demoKey("3"), demoKey("enter"), demoKey("enter"), demoKey("esc")} {
			m.Update(k) // sell it all, aggressive, once, as the fixture does
		}
		m.endDay()
		m.stop() // the morning's scene, or a card's
		world.Dilemmas.Pending = nil
		if m.mode == modeStage {
			world.SeeStage(m.stage)
		}
		m.mode = modePlay
	}
	world.Player.DirtyCash = 48_000
	world.Stats.PeakCash = 48_000
	world.Crew.Members = append(world.Crew.Members,
		game.CrewMember{ID: 1, Name: "Dre", Role: "runner", Skill: 60, Units: 120, Loyalty: 80, Nerve: 50, Wage: 50},
		game.CrewMember{ID: 2, Name: "Moose", Role: "enforcer", Skill: 70, Loyalty: 70, Nerve: 60, Wage: 65},
	)
	world.Crew.NextID = 2
	home := world.Home()
	home.Corners[0].Owner, home.Corners[0].Runner, home.Corners[0].Enforcer = game.OwnerRival, 0, 0
	world.Rival.Arrived, world.Rival.Muscle, world.Rival.Observed = 1, 3, true
	m.save()
	m.status = ""
	return m, nil
}

// DemoScene puts the registry's scene of the name up, in the mode that
// plays it in the game and through the game's own start (titleLoop,
// stageScene, showCard, playOver, morningScene, bustScene,
// mapSceneStart), on the demo's world, and returns the command that
// starts its ticks; nil for a name the registry lacks. effect pins the
// title's effect (anim.TitleEffects; "" cycles the set as the game
// does) and the rest ignore it. The scene up before comes down first,
// and what the scene before put on the world for its own sake (an
// ending, a card, a corner's new owner) is taken off, so every scene
// starts on the same run.
func (m *Model) DemoScene(name, effect string) tea.Cmd {
	m.stop()
	w := m.w
	w.Over, w.Dilemmas.Pending = nil, nil
	m.mode, m.screen, m.modalScroll, m.status = modePlay, screenDashboard, 0, ""
	m.mapScene, m.mapPlaying, m.opts.Effect = nil, nil, ""
	m.city = w.Player.Location
	w.Home().Corners[0].Owner = game.OwnerRival // the fixture's, the strike's to win
	switch name {
	case "title":
		m.mode = modeStart
		m.opts.Effect = effect
		m.titleLoop()
	case "stage":
		m.stage, m.mode = 3, modeStage
		m.stageScene(3)
	case "card":
		w.Dilemmas.Pending = demoCard(w.Day)
		m.showCard()
	case "over:indicted", "over:arrested", "over:broke":
		w.Over = &game.Ending{Day: w.Day, Cause: strings.TrimPrefix(name, "over:"), PeakCash: w.Stats.PeakCash}
		w.Heat.Evidence = max(w.Heat.Evidence, 7) // the file's pages
		m.mode = modeOver
		m.playOver()
	case "morning":
		m.mode = modeReport
		m.morningScene()
	case "bust":
		ev := events.Enforcement{Day: w.Day, City: w.Player.Location, Level: "raid", StockLost: map[string]int{w.Products[0]: 40}, CashLost: 2000}
		// The report's line, as the news writes it, so the loss has
		// something to burn along; once.
		line := fmt.Sprintf("RAID: lost 40 %s and %s in %s", w.ProductName(w.Products[0]), money(ev.CashLost), w.CityName(ev.City))
		if m.bustLoss("RAID") == "" {
			w.Report.Heat = append([]string{line}, w.Report.Heat...)
		}
		m.mode = modeReport
		m.bustScene(ev)
	case "strike":
		// Your enforcers took the rival's corner overnight: purple to
		// blue, and its name slides into the inspector.
		c := &w.Home().Corners[0]
		c.Owner = game.OwnerPlayer
		m.mapScene = []mapFlip{{c.ID, game.OwnerRival}}
		m.screen = screenMap
		m.mapSceneStart()
	default:
		return nil
	}
	return m.tick()
}

// demoCard is the card the demo deals: a sample in the file's shape.
func demoCard(day int) *game.Card {
	return &game.Card{ID: "demo", Day: day, Title: "The Wire", Text: "A man with a badge buys you a coffee and does not want anything. Not yet. He mentions the corner by name, and your runner by theirs, and leaves the receipt.",
		Choices: []game.Choice{
			{Label: "Pay him", Outcome: "He takes the envelope. There will be another coffee."},
			{Label: "Move the runner", Outcome: "The corner goes quiet for a week."},
			{Label: "Say nothing", Outcome: "He finishes the coffee."},
		}}
}

// demoKey is a key as the demo presses it, the way ui_test's key does.
func demoKey(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
