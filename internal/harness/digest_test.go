package harness

import (
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// TestSeedDigest (#144): the boss on one pinned seed, sixty days, the
// whole world hashed at the end of every one of them. A change that
// moves any number, any day, fails here and names the first day it
// moved, so a stream shift (a draw added to the home stream, a sim
// slotted between two others, a seed roll moved) or a balance change
// reads as one diff and not as a dozen medians, and a shape change (a
// field moved, a constructor reshaped) that claims to be byte-for-byte
// the old run has a test that says so. The failure prints the new
// list to paste in: a PR that moves a number pins the new one and
// names the day and the reason.
//
// The digest walks the world by reflection in a fixed order (struct
// fields in declaration order, map keys sorted), hashing every
// exported value; a float is rounded to six decimals first, so the
// fused multiply-add an arm64 build is allowed to use (Go's spec
// permits it; amd64 does not) cannot move the hash between a laptop
// and CI while a real change to any number does.
func TestSeedDigest(t *testing.T) {
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, seedDigestSeed)
	_, sims, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	clock := game.NewClock(nil, sims...)
	policy := Boss(cfg, 40, "")
	var got []string
	for day := 1; day <= seedDigestDays; day++ {
		if w.Over != nil {
			break
		}
		policy(w)
		clock.EndDay(w)
		got = append(got, digest(w))
	}
	if len(seedDigest) != seedDigestDays {
		t.Fatalf("seed %d: %d days pinned, want %d; paste this in:\n\nvar seedDigest = []string{\n%s\n}", seedDigestSeed, len(seedDigest), seedDigestDays, list(got))
	}
	moved := -1
	for i := range seedDigest {
		if i >= len(got) || got[i] != seedDigest[i] {
			moved = i
			break
		}
	}
	if moved < 0 {
		return
	}
	was := "(the run ended)"
	if moved < len(got) {
		was = got[moved]
	}
	t.Errorf("seed %d: the world moved on day %d (digest %s, was %s); every later day follows. If the move is meant, name the day and the reason in the PR and paste this in:\n\nvar seedDigest = []string{\n%s\n}", seedDigestSeed, moved+1, was, seedDigest[moved], list(got))
}

// list renders the digests as the Go literal the test pins.
func list(ds []string) string {
	var b strings.Builder
	for i, d := range ds {
		if i%4 == 0 {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("\t")
		} else {
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "%q,", d)
	}
	return b.String()
}

// digest is the world's hash: FNV-1a over a walk of every exported
// value in a fixed order, floats to six decimals.
func digest(w *game.World) string {
	h := fnv.New64a()
	walk(h, reflect.ValueOf(w).Elem())
	return fmt.Sprintf("%016x", h.Sum64())
}

func walk(h interface{ Write([]byte) (int, error) }, v reflect.Value) {
	put := func(s string) { _, _ = h.Write([]byte(s)); _, _ = h.Write([]byte{0}) }
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			put("nil")
			return
		}
		walk(h, v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if f := v.Type().Field(i); f.IsExported() {
				put(f.Name)
				walk(h, v.Field(i))
			}
		}
	case reflect.Map:
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface()) })
		put(fmt.Sprintf("map%d", len(keys)))
		for _, k := range keys {
			put(fmt.Sprint(k.Interface()))
			walk(h, v.MapIndex(k))
		}
	case reflect.Slice, reflect.Array:
		put(fmt.Sprintf("len%d", v.Len()))
		for i := 0; i < v.Len(); i++ {
			walk(h, v.Index(i))
		}
	case reflect.Float32, reflect.Float64:
		put(fmt.Sprintf("%.6f", v.Float()))
	default:
		put(fmt.Sprint(v.Interface()))
	}
}

// seedDigestSeed is the run TestSeedDigest plays, seedDigestDays how
// far: sixty days is the boss into tier 3 with every sim awake (the
// crew, the road, the fronts, the rival's war and books, the law's
// first election), short enough to run in a second.
const (
	seedDigestSeed = 7
	seedDigestDays = 60
)

// seedDigest is the world after each of the boss's first sixty days on
// seedDigestSeed, pinned at 75bd4d0 (#144 PR 4: the shape changes of
// the issue, no number moved) and again for #193 (City.Campaign,
// DA.Backed, Law.CampaignOpen, Today.Backed and three Stats added to
// the walk; no number moved: the digest with the new fields skipped was
// the old one on every day, the boss having no clean cash to back a
// ticket with by day 60), again for #192 (Front.Level, Invested and
// Grew, Stats.Earned and Invested, Today.Invested; no number moved: the
// boss's first level comes after day 60 on this seed), and again for
// #195 (World.Offshore and QuietDays, Today.Reserved,
// Laundering.Structured, Stats.Reserved and Fees; one number moves
// from day 1: QuietDays, the count of quiet days in a row, which every
// run keeps and nothing but Retire reads; the boss reserves nothing by
// day 60 on this seed, holding under a campaign's worth).
var seedDigest = []string{
	"c37aa5813e26fb45", "4c7ae28d8664812f", "24e7d581125a75dd", "24957b066365a732",
	"93077bbeed4abab7", "7e08754ec15dfdb2", "3d869efe8b4ce69e", "11f9f9d1fd033abc",
	"b64cd38070adede3", "e59144dc6d5f0770", "499399402ab266d0", "42f52ded23e6a285",
	"4fae3c16180982e9", "fe44e9ac14340d30", "df47404a307b9db2", "6dbae93935de623f",
	"2b8a5c97bfb69d5e", "5d096c4497cced17", "49e3449db950d36c", "ff577fb8bb7f279b",
	"fa6c297e560709e6", "0c255bd8c4aa3f21", "2a4aeb1d9827e4c5", "b8afed6392a8af44",
	"11f8e07ce8acb58c", "ac9d11cb63b91dc4", "b58d1849f530dc5f", "0cffef6902ff83d8",
	"32def690fabf15df", "07d41ad173b94f9d", "cd2144d4ad1c0191", "4f8c0416c164dc50",
	"73dea43da22152ec", "1c8c6d688068cb46", "706a11fb5b8c434c", "df9584e1efd13a9d",
	"909948d09c54dd68", "8f4501863938c8a4", "ea66ebaccceb26c6", "4df38abf07238043",
	"596267eec6e6d9e0", "7ed400733b859a31", "6cef077a5bb3dffe", "3858b1a8b1398bff",
	"1588c787f19a6614", "97760d3a257de248", "0af94567b4bf2da9", "f3aedea5eecb12a2",
	"69b596e75c2a378c", "d7cfe79f01a602c9", "0aaf5ae3ecd9ecd9", "500a1266fb56c183",
	"eef0786138d9e570", "0fe60a781ccf06d0", "fe8ab3de97af03f4", "4f83d60646152608",
	"9c310e23b49db989", "be96813115edd72b", "e5453a1ac93445b4", "3ac10555511aad25",
}
