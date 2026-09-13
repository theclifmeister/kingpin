package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// Character is the start a run takes as a character of characters.toml
// (#50, docs/profile.md): what sim.NewWorldWith is handed for
// `cmd/balance -character C`. The harness measures a character with
// no profile at all: a start is on the world on day 0 and nothing
// else, so the policies play it as they play any run. An empty id is
// the default character, the run as it is; an unknown one panics.
func Character(cfg *content.Config, id string) game.Start {
	if id != "" && cfg.Characters.Character(id) == nil {
		panic("harness.Character: no character " + id)
	}
	return game.Start{Character: id}
}

// RunAs plays Run as a character: a fresh world on the seed with the
// character's start, then days days under the policy.
func RunAs(cfg *content.Config, id string, seed uint64, days int, policy Policy) (Result, error) {
	return RunFrom(cfg, sim.NewWorldWith(cfg, seed, Character(cfg, id)), days, policy)
}
