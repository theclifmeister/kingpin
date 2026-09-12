package anim

import (
	"hash/fnv"
	"math/rand/v2"
)

// Seed is a scene's dice: a stream of its own from the run's seed, the
// day and the scene's name, so no scene ever draws from game.RNGFor's
// stream (the sims' and the harness's) and two scenes on one morning do
// not share a die. The same three arguments give the same stream.
func Seed(seed uint64, day int, name string) *rand.Rand {
	h := fnv.New64a()
	h.Write([]byte(name))
	return rand.New(rand.NewPCG(seed^h.Sum64(), uint64(day)*0x9E3779B97F4A7C15+0xA5A5))
}
