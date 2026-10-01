package ui

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
)

// AlertWords is each alert as the dashboard's ALERTS words it, plain,
// on the session's run: cmd/kingpin-web's parity test (#558) holds the
// web's alerts.js to these words, clause by clause, so a clause the TUI
// gains reaches the web or is listed as left out. It is the test's
// window on the model, as ReadmeKeys is cmd/keys', never the game's.
func AlertWords(cfg *content.Config, s *engine.Session, alerts []engine.Alert) []string {
	m := &Model{cfg: cfg, sess: s, rules: s.Rules(), w: s.World()}
	out := make([]string, len(alerts))
	for i, a := range alerts {
		out[i] = ansi.Strip(m.alertOf(a).text)
	}
	return out
}
