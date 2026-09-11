// Command kingpin is a terminal game about building a drug empire one day
// at a time while the heat closes in.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui"
)

func main() {
	slot := flag.Int("slot", 0, fmt.Sprintf("open this save slot (1 to %d) straight away, skipping the start menu", game.SlotCount))
	flag.Parse()
	cfg, err := content.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kingpin: bad content:", err)
		os.Exit(1)
	}
	var m *ui.Model
	if *slot != 0 {
		m, err = ui.NewSlot(cfg, *slot)
	} else {
		m, err = ui.New(cfg)
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
