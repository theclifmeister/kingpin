// Command anim plays every registered scene (anim.Scenes) on a demo run
// at the terminal's size, for review under tmux and by eye (#161): the
// title, the stage, the card, the three endings, the morning, the bust
// and the strike, each inside the mode that plays it in the game.
// ← → move between scenes, r replays, e cycles the effect where a scene
// takes one (the title), 1-9 pick the seed, q quits; -scene starts on a
// scene, -seed on a seed, -list prints the registry and exits. A command
// of its own rather than a flag on kingpin, which keeps the game's flags
// clean; it never touches a real save (KINGPIN_HOME is a directory of
// its own for the run).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/ui"
	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

func main() {
	seed := flag.Uint64("seed", 3, "the run's seed (1-9 change it while playing)")
	start := flag.String("scene", "", "the scene to start on, by its registry name")
	list := flag.Bool("list", false, "print the registry and exit")
	flag.Parse()
	scenes := anim.Scenes()
	if *list {
		for _, sc := range scenes {
			fmt.Printf("%-14s %-6s %-24s %s\n", sc.Name, length(sc), strings.Join(sc.Effects, ","), sc.Starts)
		}
		return
	}
	at := 0
	if *start != "" {
		at = -1
		for i, sc := range scenes {
			if sc.Name == *start {
				at = i
			}
		}
		if at < 0 {
			fmt.Fprintf(os.Stderr, "anim: no scene %q; the registry is %s\n", *start, names(scenes))
			os.Exit(1)
		}
	}
	cfg, err := content.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "anim: bad content:", err)
		os.Exit(1)
	}
	home, err := os.MkdirTemp("", "kingpin-anim-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "anim:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(home)
	os.Setenv("KINGPIN_HOME", home)
	d := &demo{cfg: cfg, scenes: scenes, at: at, seed: *seed, effect: -1}
	if _, err := tea.NewProgram(d, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "anim:", err)
		os.Exit(1)
	}
}

// demo is the program: the model the scenes play on, which scene is up
// and on what seed and effect, and the bar under the game's frame that
// says so.
type demo struct {
	cfg    *content.Config
	scenes []anim.Named
	m      *ui.Model
	w, h   int
	at     int    // the scene up, an index into scenes
	seed   uint64 // the run's seed
	effect int    // the title's effect, an index into its Effects; -1 cycles the set as the game does
}

func (d *demo) Init() tea.Cmd { return nil }

// Update: the demo's own keys move between scenes, replay, cycle the
// effect, pick the seed and quit; a resize and a frame's tick go to
// the model, which draws the scene; every other key is dropped, so no
// key of the demo's lands on the game (where any key would end the
// scene).
func (d *demo) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.w, d.h = msg.Width, msg.Height
		if d.m == nil {
			return d, d.rebuild()
		}
		_, cmd := d.m.Update(msg)
		return d, cmd
	case tea.KeyMsg:
		if d.m == nil {
			return d, nil
		}
		switch k := msg.String(); k {
		case "q", "ctrl+c", "esc":
			return d, tea.Quit
		case "right", "l", "n", " ", "enter":
			d.at = (d.at + 1) % len(d.scenes)
			return d, d.play()
		case "left", "h", "p":
			d.at = (d.at + len(d.scenes) - 1) % len(d.scenes)
			return d, d.play()
		case "r":
			return d, d.play()
		case "e":
			if sc := d.scenes[d.at]; sc.With != nil {
				d.effect = (d.effect+2)%(len(sc.Effects)+1) - 1
				return d, d.play()
			}
			return d, nil
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			d.seed = uint64(k[0] - '0')
			return d, d.rebuild()
		}
		return d, nil
	}
	if d.m == nil {
		return d, nil
	}
	_, cmd := d.m.Update(msg)
	return d, cmd
}

// rebuild makes the run on the seed at the size and plays the scene.
func (d *demo) rebuild() tea.Cmd {
	m, err := ui.DemoModel(d.cfg, d.seed, d.w, d.h)
	if err != nil {
		return tea.Sequence(tea.Printf("anim: %v", err), tea.Quit)
	}
	d.m = m
	return d.play()
}

// play puts the scene up on the effect picked and starts its ticks.
func (d *demo) play() tea.Cmd {
	sc := d.scenes[d.at]
	effect := ""
	if d.effect >= 0 && sc.With != nil {
		effect = sc.Effects[d.effect]
	}
	return d.m.DemoScene(sc.Name, effect)
}

// View is the game's own render with the demo's bar on its last row:
// the scene, what starts it, its effects and length, the seed, and the
// keys.
func (d *demo) View() string {
	if d.m == nil {
		return "loading…"
	}
	lines := strings.Split(d.m.View(), "\n")
	for len(lines) < d.h {
		lines = append(lines, "")
	}
	lines[len(lines)-1] = d.bar()
	return strings.Join(lines, "\n")
}

// bar is the demo's status row: the scene on the left, the keys on the
// right, the left cut to what the width leaves.
func (d *demo) bar() string {
	sc := d.scenes[d.at]
	effects := strings.Join(sc.Effects, ", ")
	if sc.With != nil {
		effects = "cycling the set"
		if d.effect >= 0 {
			effects = sc.Effects[d.effect]
		}
	}
	where := fmt.Sprintf("%d/%d %s", d.at+1, len(d.scenes), sc.Name)
	rest := fmt.Sprintf("%s · seed %d", length(sc), d.seed)
	keys := []string{"←→ scene", "r replay", "1-9 seed", "q quit"}
	if sc.With != nil {
		keys = append(keys[:2], append([]string{"e effect"}, keys[2:]...)...)
	}
	var right strings.Builder
	for i, k := range keys {
		if i > 0 {
			right.WriteString("  ")
		}
		right.WriteString(theme.Key.Render(k[:strings.IndexByte(k, ' ')]) + k[strings.IndexByte(k, ' '):])
	}
	room := d.w - lipgloss.Width(right.String()) - 3
	// What starts the scene and its effects go when the width has room
	// for them, the effects outlasting the start; the scene, its length
	// and the seed always.
	left := where + " · " + rest
	for _, more := range []string{where + " · " + sc.Starts + " · " + effects + " · " + rest, where + " · " + effects + " · " + rest} {
		if lipgloss.Width(more) <= room {
			left = more
			break
		}
	}
	if room < 8 {
		return ansi.Truncate(" "+left, d.w, "…")
	}
	return " " + theme.Subtle.Render(ansi.Truncate(left, room, "…")) + strings.Repeat(" ", room-min(room, lipgloss.Width(left))+2) + right.String()
}

// length is a scene's length as the bar writes it: `1.5 s`, `250 ms`.
func length(sc anim.Named) string {
	if sc.Length%(100*1e6) == 0 {
		return fmt.Sprintf("%.1f s", sc.Length.Seconds())
	}
	return fmt.Sprintf("%d ms", sc.Length.Milliseconds())
}

func names(scenes []anim.Named) string {
	var ns []string
	for _, sc := range scenes {
		ns = append(ns, sc.Name)
	}
	return strings.Join(ns, ", ")
}
