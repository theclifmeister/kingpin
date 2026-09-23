package game

// The side streams of the day's randomness (Tick.Sub, SubRNG): every
// name a sim or the news draws on, in one place, so a typo cannot open a
// fresh stream and silently change a run (#274, TestStreamsAreNamed).
// The string is the stream: renaming one changes every draw on it and
// moves pinned numbers. The home city's dice are Tick.RNG, not a Sub.
//
// A stream two sims draw on is one sequence shared in step order: a draw
// by the earlier sim shifts what the later one gets that day. The shared
// ones are marked; docs/day-loop.md has the table.
const (
	StreamMarketOf    = "market:"    // + city: a city's market away from home (market; the connects' restock)
	StreamLogistics   = "logistics"  // interception on the road (logistics)
	StreamTerritoryOf = "territory:" // + city: robbery and drift away from home (territory)
	StreamTaxOf       = "tax:"       // + city: the tax's jitter (#231, territory)
	StreamHousesOf    = "houses:"    // + city: the landlord (#73, territory)
	StreamHouses      = "houses"     // the raid on a house (#73, heat)
	StreamFactions    = "factions"   // the table's seating on day 0 and what factions do to each other (#43, rivals)
	StreamFactionOf   = "faction:"   // + faction: a seat past the first's own day (#43, rivals)
	StreamHomage      = "homage"     // homage offered (#43, rivals)
	StreamPoach       = "poach"      // a faction poaching another's muscle (#43, rivals)
	StreamFragment    = "fragment"   // a faction breaking up takes crew with it (#43, crew)
	StreamLife        = "life"       // kin, arrests, bail, the shot (#46, crew)
	StreamAges        = "ages"       // the pool's ages (#46, crew)
	StreamChemist     = "chemist"    // the chemist looking for work (#47, crew)
	StreamDriver      = "driver"     // the driver looking for work (#46, crew)
	StreamFixer       = "fixer"      // the fixer looking for work (crew)
	StreamLaw         = "law"        // the chief, the DA, the elections and campaigns (law)
	StreamBribes      = "bribes"     // a bribe backfiring (#42, law)
	StreamIncidents   = "incidents"  // the world's incident table (#44, world)
	StreamOverdose    = "overdose"   // an overdose on a cut product (#47, market)
	StreamSwagger     = "swagger"    // the boss's headline (#233, news)
	StreamCharacter   = "character"  // a character's start on day 0 (#50)

	// Shared streams.
	StreamBooks     = "books"     // shared: the rival's scout and poach (#70, rivals) and a tip's evidence (heat)
	StreamIntel     = "intel"     // shared: the spy (crew), the feed (rivals), the cop (heat) and the law's facts (#45)
	StreamSuppliers = "suppliers" // shared: the connects' credit (#72, market) and their headlines (news)
	StreamBuyers    = "buyers"    // shared: the buyers' deck (#71, market) and their headlines (news)

	// The news sim's own template streams, so a feature's headlines
	// never move the home stream.
	StreamProgression       = "progression"        // the tier's headline (#147)
	StreamUnlocks           = "unlocks"            // an unlock's headline
	StreamAssetsNews        = "assets:news"        // the assets' headlines (#48)
	StreamDeedsNews         = "deeds:news"         // the deeds' headlines (#194)
	StreamHousesNews        = "houses:news"        // the houses' headlines (#73)
	StreamIncidentsNews     = "incidents:news"     // an incident's headline (#44)
	StreamIntelNews         = "intel:news"         // the intel headlines (#45)
	StreamOverdoseNews      = "overdose:news"      // an overdose's headline (#47)
	StreamInvestigationNews = "investigation:news" // an investigation opened or gone nowhere (#343)
)

// Rand is the subset of *math/rand/v2.Rand the sims draw on (#275): the
// home stream (Tick.RNG) and every Sub are one, so a helper takes
// whichever the caller rolls on. One interface for every sim that passes
// its dice down.
type Rand interface {
	IntN(int) int
	Float64() float64
}
