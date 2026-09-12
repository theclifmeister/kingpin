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
// ticket with by day 60) and again for #47 (Player.Quality, World.
// BaseQuality, Corner.Repeat, Shipment.Quality, Supplier.Quality,
// Crew.Cooks and NextCook, Today.Cuts and five Stats added to the walk,
// the chemist beside the pool's faces once meth lists; the move is on
// day 1, the day the default quality and repeat_start are stamped, and
// no money number moved: t1 and t2 stand to the dollar and the boss's
// rows stand under #193's figures).
var seedDigest = []string{
	"bd97411885cf06ee", "ced86f2ac8c3998d", "270166cd90753086", "cd0b1b2ae6509acc",
	"5f19ad38744778f5", "b681fe7dbf1bd503", "60445d0b105f74b3", "5494e3a2609997fb",
	"cb970dd418bcc539", "df5cf7ff7c6ccdde", "7cd9077e5e30e5af", "f78ac87e96d59bce",
	"87d87a27df79c5ab", "d02173a7e0c0014c", "393b682e2c8581ba", "d402e3f551672c7e",
	"ecf3bdb22d4ccf6d", "3ce0aa1fa00aade4", "dea69e0bda7da667", "0ef8909ac95a4bb6",
	"934c678c012729b3", "14357dd1e8461810", "7e3aeda5184c5ea4", "fca61e6802302b1f",
	"7f0bb354a692fdcf", "505d9e29628c3226", "397ac9df48df6a00", "a35d754f766b3f78",
	"51467a64e6059702", "287c86616a2147c4", "674dfb79067b0179", "727ec47dd241bb7d",
	"bb6309722e25a61b", "c223c14cdab7e6cf", "d098ee92dd6cf35c", "11a26c86d30eeca3",
	"cc2c6819da949061", "1a2160b516b14a55", "a2e341d0813d2bfc", "c75485707fbb5b69",
	"9004ad5e93d0fa68", "270230a93a9ed7ff", "5a18849cf02ec968", "a17d696abe9c2651",
	"766bacd5ed6187ee", "ff3e1edad6859158", "366196f6510d6068", "309e9761addc9f8c",
	"b741d88c8664063a", "ee9a625e5b40bd82", "8ffc8dab57430c1e", "aa3ab2c01279d5b6",
	"cafc99dd0f67bcd1", "cd5e0990334b60ef", "c50d816cff621ccd", "c7f5c390f4ed0935",
	"c67dff13a1495f3e", "e3b6d122a79267da", "03c4fe677a76d6ff", "61505db5f68c9026",
}
