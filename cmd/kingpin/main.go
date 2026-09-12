// Command kingpin is a terminal game about building a drug empire one day
// at a time while the heat closes in.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui"
	"github.com/theclifmeister/kingpin/internal/ui/anim"
)

func main() {
	slot := flag.Int("slot", 0, fmt.Sprintf("open this save slot (1 to %d) straight away, skipping the start menu", game.SlotCount))
	noAnim := flag.Bool("no-anim", false, "play without the animated scenes (KINGPIN_NO_ANIM=1 does the same)")
	flag.Parse()
	// The scenes are on unless the flag or the environment says
	// otherwise (#152): any value in KINGPIN_NO_ANIM turns them off.
	// KINGPIN_ANIM_EFFECT pins the title loop's effect by name (#153),
	// for review; an unknown name is refused with the set.
	opts := ui.Options{Anim: !*noAnim && os.Getenv("KINGPIN_NO_ANIM") == "", Effect: os.Getenv("KINGPIN_ANIM_EFFECT")}
	if e, ok := anim.Effects[opts.Effect]; opts.Effect != "" && (!ok || !e.Needs.Text) {
		fmt.Fprintf(os.Stderr, "kingpin: KINGPIN_ANIM_EFFECT=%q is not one of %s\n", opts.Effect, strings.Join(anim.TitleEffects(), ", "))
		os.Exit(1)
	}
	cfg, err := content.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kingpin: bad content:", err)
		os.Exit(1)
	}
	var m *ui.Model
	if *slot != 0 {
		m, err = ui.NewSlot(cfg, *slot, opts)
	} else {
		m, err = ui.New(cfg, opts)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kingpin:", err)
		os.Exit(1)
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "kingpin:", err)
		os.Exit(1)
	}
}
