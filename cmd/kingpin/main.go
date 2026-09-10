// Command kingpin is a terminal game about building a drug empire one day
// at a time while the heat closes in.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/ui"
)

func main() {
	cfg, err := content.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kingpin: bad content:", err)
		os.Exit(1)
	}
	m, err := ui.New(cfg)
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
