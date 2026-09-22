package game

import "github.com/theclifmeister/kingpin/internal/events"

// StrikeOrder is the player's enforcers sent against a rival corner at a
// force, resolved by the rival sim at end of day. Boost sends them for
// the corner's takings rather than the ground (#70): one order a night
// either way, at the force's odds.
type StrikeOrder struct {
	Corner string
	Force  events.Force
	Boost  bool
	War    bool // the war order's strike (#229), not the hand's
}

// ScoutOrder is the player paying for a look at the rival's books
// tonight (#70), paid up front and resolved by the rivals sim.
type ScoutOrder struct {
	Cost    int
	Faction string // whose books (#43); "" is the rival at home
}

// TipOrder is the player tipping the police on a rival corner tonight
// (#70): free in cash, resolved by the rivals sim.
type TipOrder struct {
	Corner string
}

// PoachOrder is the player paying Units heads of the rival's muscle to
// go home tonight (#70), Cost paid up front, resolved by the rivals sim.
type PoachOrder struct {
	Units   int
	Cost    int
	Faction string // whose muscle (#43); "" is the rival at home
}

// RivalState is the faction competing for the city's corners. Leader is
// empty until the sim seeds it; Arrived is 0 until it holds its first
// corner. War is how loud the fight has got, 0..100: past the crackdown
// line the police clear both sides.
type RivalState struct {
	ID          string // faction id (#144): FactionRival today, seeded with the rival; read it through Faction, which resolves the zero id of an older save
	Leader      string
	Personality string  // expansionist, defensive, opportunist, chaotic
	Supplier    float64 // its supplier price as a fraction of street; a better connect undercuts harder
	Cash        int
	Muscle      int // enforcers on its side, abstract
	Arrived     int // day it moved in; 0 = not yet
	Routed      int // day it last lost its last corner; 0 = never
	Observed    bool
	Grudge      int     // losses it has not yet paid back
	War         float64 // 0..100
	Claims      int     // lifetime counters for the run summary
	Flips       int     // corners it took from the player
	Tips        int
	LastClaim   int    // day it last chose a free corner to set up on (the tell, #69); 0 never (#60: the pace's cooldown)
	LastStruck  int    // day the player's enforcers last went in; 0 never (#60: under attack it grows as fast as it can)
	Eyeing      string // corner id it sets up on next step, the tell (#69); "" none. Post somebody on it first and the claim fails.
	EyeingDay   int    // day the tell was given

	// Diplomacy (#32): what it thinks of you and what you have agreed.
	Trust     float64 // 0..100; seeded by personality, earned by kept deals, spent by force
	Deals     []Deal  // live deals, oldest first
	Betrayed  int     // day the player last broke a deal; 0 never. It takes nothing for a while after.
	LastFlip  int     // day it last took a corner from the player; 0 never
	NextOffer int     // id of the next offer it makes

	// The books (#139): wages the day's take did not cover, carried
	// forward; a surplus day pays them down and at a full wage a head
	// walks. Zero is a payroll the take covers, the pre-#139 state.
	Arrears float64

	// The player's moves against it (#70). Heat is the police's
	// attention on it, 0..100: your tips, and its own pushes while it is
	// over zero; past the notice line they take a corner off it. Scouted
	// counts the scouts that read nothing since the last that did (the
	// books a scout read are facts in World.Intel since #45, game.Books
	// the read as the file holds it). Zero values are the pre-#70 state.
	Heat     float64
	Scouted  int
	LastRaid int // day the police last took a corner off it on your tip; 0 never
	Away     int // heads bought off or arrested and not yet back: what it wants less, for a while
	AwayDay  int // day the last of them came back, or was sent away; the next returns away_days later

	// The table (#43): a faction among factions. Home is the city it
	// lives in ("" reads as home, the one rival's city before #43);
	// Grudges and Trusts are what it holds against and thinks of the
	// other factions, by id (Trust above is its trust in you); Ally and
	// Against name the faction it stands with and the one it stands
	// against (a defensive faction sides with whoever the expansionist
	// is pushing on; "you" is the player); LostToYou counts the corners
	// your enforcers took off it, what a homage offer waits for;
	// LastTakenBy is the faction that took its last corner ("" you or
	// the police), who absorbs it after absorb_days with none; Absorbed
	// and Fragmented are the day it stopped being a faction (absorbed
	// into AbsorbedBy, or its leader taken: Fragments are the corners
	// still to drift). Zero values are the duel, the pre-#43 state.
	Home        string
	Grudges     map[string]int
	Trusts      map[string]float64
	Ally        string
	Against     string
	LostToYou   int
	LastTakenBy string
	Absorbed    int
	AbsorbedBy  string
	Fragmented  int
	Fragments   []string
}

// Gone reports whether the faction is out of the game: absorbed by
// another or fragmented after its leader was taken (#43). A gone faction
// steps no more; its corners drift and its deals are over.
func (r RivalState) Gone() bool { return r.Absorbed > 0 || r.Fragmented > 0 }

// Alive reports whether the faction is in the game and on the ground:
// arrived and not gone.
func (r RivalState) Alive() bool { return r.Arrived > 0 && !r.Gone() }

// TrustIn is the faction's trust in another faction, by id (#43): what
// Trusts holds, or a neutral 50 for one it has never dealt with.
func (r RivalState) TrustIn(id string) float64 {
	if v, ok := r.Trusts[id]; ok {
		return v
	}
	return 50
}

// Faction is the rival's faction id (#144): ID, or FactionRival for a
// save from before ids carried one (the zero value), so no migration.
func (r RivalState) Faction() string {
	if r.ID == "" {
		return FactionRival
	}
	return r.ID
}

// Lead is a crew member who went over to the rival: their name and the
// corner they ran (empty if none), which they walk the rival onto. The
// crew sim queues them on CrewState.Leads; the rivals sim acts on them
// next step.
type Lead struct {
	Name    string
	Corner  string
	Faction string // the faction they went to (#43); "" is the rival at home
}
